package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/observability"
)

// stsAssumeRoleClient defines the subset of STS functionality we rely on.
type stsAssumeRoleClient interface {
	AssumeRole(ctx context.Context, params *sts.AssumeRoleInput, optFns ...func(*sts.Options)) (*sts.AssumeRoleOutput, error)
}

const (
	// Kernel account IDs
	QAKernelAccountID = "058264189048"
	KernelAccountID   = "075149869707"

	// Default timeouts
	DefaultConnectTimeout = 30 * time.Second
	DefaultReadTimeout    = 30 * time.Second

	emptyPayloadHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

// LoggerFunc is a function that returns the singleton logger from the calling service
type LoggerFunc func() observability.StructuredLogger

// Client provides authenticated cross-account calls to kernel services
type Client struct {
	loggerFunc      LoggerFunc
	httpClient      *http.Client
	stsClient       stsAssumeRoleClient
	baseURLResolver func(servicePrefix string, includeRegion bool) string
	partner         string
	stage           string
	region          string
	externalID      string
	connectTimeout  time.Duration
	readTimeout     time.Duration
}

// CallOptions configures a kernel service call
type CallOptions struct {
	Body           any               // Request payload (will be JSON marshaled)
	Headers        map[string]string // Additional headers (optional)
	ServicePrefix  string            // Service name (e.g., "k3", "paze-wallet-key-service")
	Endpoint       string            // API endpoint path
	Method         string            // HTTP method (GET, POST, etc.)
	Subsystem      string            // Logging subsystem name (optional)
	ConnectTimeout time.Duration     // Connection timeout (optional)
	ReadTimeout    time.Duration     // Read timeout (optional)
	IncludeRegion  bool              // Whether to include region in URL
}

// Response represents a kernel service response
//
//nolint:govet // govet/fieldalignment would add noise for this small, short-lived struct.
type Response struct {
	Body       []byte
	Headers    map[string]string
	StatusCode int
	Duration   time.Duration
}

// NewClient creates a new kernel service client
// loggerFunc should return the calling service's singleton logger (e.g., logger.GetLiftLogger)
func NewClient(ctx context.Context, loggerFunc LoggerFunc) (*Client, error) {
	// Get environment variables
	partner := os.Getenv("PARTNER")
	stage := os.Getenv("STAGE")
	region := os.Getenv("TARGET_REGION")
	externalID := os.Getenv("K3_EXTERNAL_ID")

	if partner == "" {
		return nil, fmt.Errorf("PARTNER environment variable is required")
	}
	if stage == "" {
		return nil, fmt.Errorf("STAGE environment variable is required")
	}
	if region == "" {
		region = "us-east-1" // Default region
	}

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &Client{
		partner:        partner,
		stage:          stage,
		region:         region,
		externalID:     externalID,
		loggerFunc:     loggerFunc,
		httpClient:     &http.Client{},
		connectTimeout: DefaultConnectTimeout,
		readTimeout:    DefaultReadTimeout,
		stsClient:      sts.NewFromConfig(cfg),
	}, nil
}

// NewClientWithConfig creates a client with custom configuration
func NewClientWithConfig(
	ctx context.Context,
	loggerFunc LoggerFunc,
	partner, stage, region string,
	connectTimeout, readTimeout time.Duration,
) (*Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	if connectTimeout == 0 {
		connectTimeout = DefaultConnectTimeout
	}
	if readTimeout == 0 {
		readTimeout = DefaultReadTimeout
	}

	return &Client{
		partner:        partner,
		stage:          stage,
		region:         region,
		externalID:     os.Getenv("K3_EXTERNAL_ID"),
		loggerFunc:     loggerFunc,
		httpClient:     &http.Client{},
		connectTimeout: connectTimeout,
		readTimeout:    readTimeout,
		stsClient:      sts.NewFromConfig(cfg),
	}, nil
}

// Call makes an authenticated call to a kernel service
func (c *Client) Call(ctx context.Context, opts *CallOptions) (*Response, error) {
	start := time.Now()

	c.applyCallDefaults(opts)

	fullURL := c.buildFullURL(opts)
	bodyBytes, err := marshalRequestBody(opts.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	credentials, err := c.getCrossAccountCredentials(ctx)
	if err != nil {
		c.logError("Failed to get cross-account credentials", opts, err, nil)
		return nil, fmt.Errorf("failed to get credentials: %w", err)
	}

	req, err := c.createHTTPRequest(ctx, opts, fullURL, bodyBytes)
	if err != nil {
		return nil, err
	}

	if err = c.signHTTPRequest(ctx, req, credentials, len(bodyBytes) > 0, opts); err != nil {
		return nil, err
	}

	c.logRequest(opts, fullURL)

	response, err := c.executeHTTPRequest(req, opts, start)
	if err != nil {
		return nil, err
	}

	c.logResponse(opts, response)

	if response.StatusCode >= http.StatusBadRequest {
		c.logKernelError(opts, response)
		return response, fmt.Errorf("kernel service error: status %d", response.StatusCode)
	}

	return response, nil
}

func (c *Client) applyCallDefaults(opts *CallOptions) {
	if opts.Method == "" {
		opts.Method = http.MethodPost
	}
	if opts.ConnectTimeout == 0 {
		opts.ConnectTimeout = c.connectTimeout
	}
	if opts.ReadTimeout == 0 {
		opts.ReadTimeout = c.readTimeout
	}
	if opts.Subsystem == "" {
		opts.Subsystem = opts.ServicePrefix
	}
}

func (c *Client) buildFullURL(opts *CallOptions) string {
	baseURL := c.resolveBaseURL(opts.ServicePrefix, opts.IncludeRegion)
	if opts.IncludeRegion {
		return fmt.Sprintf("%s%s", baseURL, opts.Endpoint)
	}
	return fmt.Sprintf("%s/%s", baseURL, opts.Endpoint)
}

func marshalRequestBody(body any) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	return json.Marshal(body)
}

func (c *Client) createHTTPRequest(ctx context.Context, opts *CallOptions, fullURL string, bodyBytes []byte) (*http.Request, error) {
	var bodyReader io.Reader
	if len(bodyBytes) > 0 {
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, opts.Method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	if len(bodyBytes) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	for key, value := range opts.Headers {
		req.Header.Set(key, value)
	}

	return req, nil
}

func (c *Client) signHTTPRequest(ctx context.Context, req *http.Request, credentials aws.Credentials, hasBody bool, opts *CallOptions) error {
	signer := v4.NewSigner()
	payloadHash := emptyPayloadHash
	if hasBody {
		payloadHash = ""
	}

	if err := signer.SignHTTP(ctx, credentials, req, payloadHash, "execute-api", c.region, time.Now()); err != nil {
		c.logError("Failed to sign request", opts, err, nil)
		return fmt.Errorf("failed to sign request: %w", err)
	}
	return nil
}

func (c *Client) executeHTTPRequest(req *http.Request, opts *CallOptions, start time.Time) (*Response, error) {
	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		c.logError("Kernel service call failed", opts, err, map[string]any{"endpoint": opts.Endpoint})
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() {
		if closeErr := httpResp.Body.Close(); closeErr != nil {
			c.logWarn("Failed to close kernel response body", opts, closeErr)
		}
	}()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	response := &Response{
		StatusCode: httpResp.StatusCode,
		Duration:   time.Since(start),
		Headers:    make(map[string]string),
		Body:       respBody,
	}

	for key, values := range httpResp.Header {
		if len(values) > 0 {
			response.Headers[key] = values[0]
		}
	}

	return response, nil
}

func (c *Client) logRequest(opts *CallOptions, url string) {
	if logger := c.logger(); logger != nil {
		logger.Debug("Making kernel service call", map[string]any{
			"service":   opts.ServicePrefix,
			"endpoint":  opts.Endpoint,
			"method":    opts.Method,
			"url":       url,
			"subsystem": opts.Subsystem,
		})
	}
}

func (c *Client) logResponse(opts *CallOptions, response *Response) {
	if logger := c.logger(); logger != nil {
		logger.Debug("Kernel service call completed", map[string]any{
			"service":     opts.ServicePrefix,
			"status_code": response.StatusCode,
			"duration_ms": response.Duration.Milliseconds(),
			"subsystem":   opts.Subsystem,
		})
	}
}

func (c *Client) logKernelError(opts *CallOptions, response *Response) {
	if logger := c.logger(); logger != nil {
		logger.Error("Kernel service returned error", map[string]any{
			"status_code": response.StatusCode,
			"response":    string(response.Body),
			"subsystem":   opts.Subsystem,
		})
	}
}

func (c *Client) logError(message string, opts *CallOptions, err error, extra map[string]any) {
	if logger := c.logger(); logger != nil {
		fields := map[string]any{
			"error":     err.Error(),
			"service":   opts.ServicePrefix,
			"subsystem": opts.Subsystem,
		}
		for k, v := range extra {
			fields[k] = v
		}
		logger.Error(message, fields)
	}
}

func (c *Client) logWarn(message string, opts *CallOptions, err error) {
	if logger := c.logger(); logger != nil {
		logger.Warn(message, map[string]any{
			"error":     err.Error(),
			"service":   opts.ServicePrefix,
			"subsystem": opts.Subsystem,
		})
	}
}

func (c *Client) logger() observability.StructuredLogger {
	if c.loggerFunc == nil {
		return nil
	}
	return c.loggerFunc()
}

// getCrossAccountCredentials assumes the kernel-access role
func (c *Client) getCrossAccountCredentials(ctx context.Context) (aws.Credentials, error) {
	accountID := c.getKernelAccountID()

	// Determine role name based on External ID presence
	roleName := "kernel-access"
	if c.externalID != "" {
		roleName = "kernel-access-external"
	}

	roleARN := fmt.Sprintf("arn:aws:iam::%s:role/%s", accountID, roleName)

	// Build assume role input
	input := &sts.AssumeRoleInput{
		RoleArn:         aws.String(roleARN),
		RoleSessionName: aws.String("kernel-service-call"),
	}

	if c.externalID != "" {
		input.ExternalId = aws.String(c.externalID)
	}

	// Assume role
	result, err := c.stsClient.AssumeRole(ctx, input)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to assume role %s: %w", roleARN, err)
	}

	if result.Credentials == nil {
		return aws.Credentials{}, fmt.Errorf("no credentials returned from AssumeRole")
	}

	return aws.Credentials{
		AccessKeyID:     aws.ToString(result.Credentials.AccessKeyId),
		SecretAccessKey: aws.ToString(result.Credentials.SecretAccessKey),
		SessionToken:    aws.ToString(result.Credentials.SessionToken),
		Source:          "AssumeRole",
		CanExpire:       true,
		Expires:         aws.ToTime(result.Credentials.Expiration),
	}, nil
}

// getKernelEnvironment determines which kernel environment to use
func (c *Client) getKernelEnvironment() string {
	kernelEnvOverride := os.Getenv("KERNEL_ENV_OVERRIDE")

	if kernelEnvOverride != "" {
		return kernelEnvOverride
	}

	if strings.EqualFold(c.partner, "innovate") {
		return "qakernel"
	}

	return "kernel"
}

// getKernelAccountID returns the appropriate kernel account ID
func (c *Client) getKernelAccountID() string {
	kernelEnv := c.getKernelEnvironment()
	if kernelEnv == "qakernel" {
		return QAKernelAccountID
	}
	return KernelAccountID
}

// getKernelBaseURL constructs the kernel service base URL
func (c *Client) getKernelBaseURL(servicePrefix string, includeRegion bool) string {
	urlStage := c.stage
	urlPrefix := c.getKernelEnvironment()

	// If the partner is start live, use the study env
	if c.partner == "start" && c.stage == "paytheory" {
		urlStage = "paytheorystudy"
	}

	if includeRegion {
		return fmt.Sprintf("https://%s.%s.%s.com/%s/", c.region, urlPrefix, urlStage, servicePrefix)
	}
	return fmt.Sprintf("https://%s.%s.%s.com", servicePrefix, urlPrefix, urlStage)
}

func (c *Client) resolveBaseURL(servicePrefix string, includeRegion bool) string {
	if c.baseURLResolver != nil {
		return c.baseURLResolver(servicePrefix, includeRegion)
	}
	return c.getKernelBaseURL(servicePrefix, includeRegion)
}

// Unmarshal unmarshals the response body into the provided struct
func (r *Response) Unmarshal(v any) error {
	return json.Unmarshal(r.Body, v)
}

// Convenience functions for specific services

// K3Call makes an authenticated call to K3 API Gateway
func (c *Client) K3Call(ctx context.Context, endpoint string, body any, method string) (*Response, error) {
	if method == "" {
		method = http.MethodPost
	}

	return c.Call(ctx, &CallOptions{
		ServicePrefix: "k3",
		Endpoint:      endpoint,
		Method:        method,
		Body:          body,
		IncludeRegion: false,
		Subsystem:     "K3",
	})
}

// BinLookupCall makes an authenticated call to Bin Lookup Service
func (c *Client) BinLookupCall(ctx context.Context, endpoint string, method string, body any) (*Response, error) {
	if method == "" {
		method = http.MethodGet
	}

	return c.Call(ctx, &CallOptions{
		ServicePrefix: "bin-lookup-service",
		Endpoint:      endpoint,
		Method:        method,
		Body:          body,
		IncludeRegion: true,
		Subsystem:     "BIN LOOKUP SERVICE",
	})
}

// AppleWalletCall makes an authenticated call to Apple Wallet Key Service
func (c *Client) AppleWalletCall(ctx context.Context, endpoint string, body any, method string) (*Response, error) {
	if method == "" {
		method = http.MethodPost
	}

	return c.Call(ctx, &CallOptions{
		ServicePrefix: "apple-wallet-key-service",
		Endpoint:      endpoint,
		Method:        method,
		Body:          body,
		IncludeRegion: true,
		Subsystem:     "APPLE WALLET KEY SERVICE",
	})
}

// GoogleWalletCall makes an authenticated call to Google Wallet Key Service
func (c *Client) GoogleWalletCall(ctx context.Context, endpoint string, body any, method string) (*Response, error) {
	if method == "" {
		method = http.MethodPost
	}

	return c.Call(ctx, &CallOptions{
		ServicePrefix: "google-wallet-key-service",
		Endpoint:      endpoint,
		Method:        method,
		Body:          body,
		IncludeRegion: true,
		Subsystem:     "GOOGLE WALLET KEY SERVICE",
	})
}

// PazeWalletCall makes an authenticated call to Paze Wallet Key Service
func (c *Client) PazeWalletCall(ctx context.Context, endpoint string, body any, method string) (*Response, error) {
	if method == "" {
		method = http.MethodPost
	}

	return c.Call(ctx, &CallOptions{
		ServicePrefix: "paze-wallet-key-service",
		Endpoint:      endpoint,
		Method:        method,
		Body:          body,
		IncludeRegion: true,
		Subsystem:     "PAZE WALLET KEY SERVICE",
	})
}

// BankDataCall makes an authenticated call to Bank Data Service
func (c *Client) BankDataCall(ctx context.Context, endpoint string, method string, body any) (*Response, error) {
	if method == "" {
		method = http.MethodGet
	}

	return c.Call(ctx, &CallOptions{
		ServicePrefix: "bank-data-service",
		Endpoint:      endpoint,
		Method:        method,
		Body:          body,
		IncludeRegion: true,
		Subsystem:     "BANK DATA SERVICE",
	})
}

// KernelClientMiddleware creates middleware for kernel client integration
func KernelClientMiddleware(client *Client) lift.Middleware {
	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			// Add kernel client to context
			ctx.Set("kernel_client", client)
			return next.Handle(ctx)
		})
	}
}

// GetKernelClient retrieves the kernel client from Lift context
func GetKernelClient(ctx *lift.Context) *Client {
	if client, ok := ctx.Get("kernel_client").(*Client); ok {
		return client
	}
	return nil
}
