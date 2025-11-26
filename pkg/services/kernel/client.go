package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	// Default timeouts
	DefaultConnectTimeout = 30 * time.Second
	DefaultReadTimeout    = 30 * time.Second

	emptyPayloadHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

// LoggerFunc is a function that returns the singleton logger from the calling service
type LoggerFunc func() observability.StructuredLogger

// Client provides authenticated cross-account calls to kernel services
type Client struct {
	loggerFunc     LoggerFunc
	httpClient     *http.Client
	stsClient      stsAssumeRoleClient
	accountID      string // Kernel AWS account ID for STS assume role
	region         string
	externalID     string
	connectTimeout time.Duration
	readTimeout    time.Duration
}

// CallOptions configures a kernel service call
type CallOptions struct {
	Body           any               // Request payload (will be JSON marshaled)
	Headers        map[string]string // Additional headers (optional)
	BaseURL        string            // Full base URL (required)
	Endpoint       string            // API endpoint path
	Method         string            // HTTP method (GET, POST, etc.)
	ConnectTimeout time.Duration     // Connection timeout (optional)
	ReadTimeout    time.Duration     // Read timeout (optional)
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

// ClientConfig holds configuration for creating a kernel client
type ClientConfig struct {
	// AccountID is the AWS account ID for kernel services (required for STS assume role)
	AccountID string

	// Region is the AWS region (optional, defaults to "us-east-1")
	Region string

	// ExternalID is for external partner authentication (optional)
	ExternalID string

	// ConnectTimeout is the connection timeout (optional, defaults to 30s)
	ConnectTimeout time.Duration

	// ReadTimeout is the read timeout (optional, defaults to 30s)
	ReadTimeout time.Duration
}

// NewClient creates a new kernel service client with the given configuration
// loggerFunc should return the calling service's singleton logger (e.g., logger.GetLiftLogger)
func NewClient(ctx context.Context, loggerFunc LoggerFunc, cfg ClientConfig) (*Client, error) {
	if cfg.AccountID == "" {
		return nil, fmt.Errorf("AccountID is required")
	}

	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	if cfg.ConnectTimeout == 0 {
		cfg.ConnectTimeout = DefaultConnectTimeout
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = DefaultReadTimeout
	}

	// Load AWS config
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &Client{
		region:         cfg.Region,
		externalID:     cfg.ExternalID,
		accountID:      cfg.AccountID,
		loggerFunc:     loggerFunc,
		httpClient:     &http.Client{},
		connectTimeout: cfg.ConnectTimeout,
		readTimeout:    cfg.ReadTimeout,
		stsClient:      sts.NewFromConfig(awsCfg),
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
}

func (c *Client) buildFullURL(opts *CallOptions) string {
	baseURL := opts.BaseURL

	// Append endpoint - handle trailing slash in baseURL
	if len(baseURL) > 0 && baseURL[len(baseURL)-1] == '/' {
		return baseURL + opts.Endpoint
	}
	return baseURL + "/" + opts.Endpoint
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
			"endpoint": opts.Endpoint,
			"method":   opts.Method,
			"url":      url,
		})
	}
}

func (c *Client) logResponse(opts *CallOptions, response *Response) {
	if logger := c.logger(); logger != nil {
		logger.Debug("Kernel service call completed", map[string]any{
			"endpoint":    opts.Endpoint,
			"status_code": response.StatusCode,
			"duration_ms": response.Duration.Milliseconds(),
		})
	}
}

func (c *Client) logKernelError(opts *CallOptions, response *Response) {
	if logger := c.logger(); logger != nil {
		logger.Error("Kernel service returned error", map[string]any{
			"endpoint":    opts.Endpoint,
			"status_code": response.StatusCode,
			"response":    string(response.Body),
		})
	}
}

func (c *Client) logError(message string, opts *CallOptions, err error, extra map[string]any) {
	if logger := c.logger(); logger != nil {
		fields := map[string]any{
			"error":    err.Error(),
			"endpoint": opts.Endpoint,
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
			"error":    err.Error(),
			"endpoint": opts.Endpoint,
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

// getKernelAccountID returns the kernel account ID (from configuration)
func (c *Client) getKernelAccountID() string {
	return c.accountID
}

// Unmarshal unmarshals the response body into the provided struct
func (r *Response) Unmarshal(v any) error {
	return json.Unmarshal(r.Body, v)
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
