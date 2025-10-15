package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/observability"
)

const (
	// Kernel account IDs
	QAKernelAccountID = "058264189048"
	KernelAccountID   = "075149869707"

	// Default timeouts
	DefaultConnectTimeout = 30 * time.Second
	DefaultReadTimeout    = 30 * time.Second
)

// LoggerFunc is a function that returns the singleton logger from the calling service
type LoggerFunc func() observability.StructuredLogger

// Client provides authenticated cross-account calls to kernel services
type Client struct {
	partner        string
	stage          string
	region         string
	externalID     string
	loggerFunc     LoggerFunc
	httpClient     *http.Client
	connectTimeout time.Duration
	readTimeout    time.Duration
	stsClient      *sts.Client
}

// CallOptions configures a kernel service call
type CallOptions struct {
	ServicePrefix  string         // Service name (e.g., "k3", "paze-wallet-key-service")
	Endpoint       string         // API endpoint path
	Method         string         // HTTP method (GET, POST, etc.)
	Body           any            // Request payload (will be JSON marshaled)
	IncludeRegion  bool           // Whether to include region in URL
	Subsystem      string         // Logging subsystem name (optional)
	ConnectTimeout time.Duration  // Connection timeout (optional)
	ReadTimeout    time.Duration  // Read timeout (optional)
	Headers        map[string]string // Additional headers (optional)
}

// Response represents a kernel service response
type Response struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
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

	// Set defaults
	if opts.Method == "" {
		opts.Method = "POST"
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

	// Build URL
	baseURL := c.getKernelBaseURL(opts.ServicePrefix, opts.IncludeRegion)
	var fullURL string
	if opts.IncludeRegion {
		fullURL = fmt.Sprintf("%s%s", baseURL, opts.Endpoint)
	} else {
		fullURL = fmt.Sprintf("%s/%s", baseURL, opts.Endpoint)
	}

	// Marshal request body
	var bodyBytes []byte
	if opts.Body != nil {
		var err error
		bodyBytes, err = json.Marshal(opts.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	// Get cross-account credentials
	credentials, err := c.getCrossAccountCredentials(ctx)
	if err != nil {
		if c.loggerFunc != nil {
			c.loggerFunc().Error("Failed to get cross-account credentials", map[string]any{
				"error":     err.Error(),
				"subsystem": opts.Subsystem,
			})
		}
		return nil, fmt.Errorf("failed to get credentials: %w", err)
	}

	// Create HTTP request
	var bodyReader io.Reader
	if len(bodyBytes) > 0 {
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, opts.Method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	if len(bodyBytes) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	// Add custom headers
	for key, value := range opts.Headers {
		req.Header.Set(key, value)
	}

	// Sign request with SigV4
	signer := v4.NewSigner()

	// Create payload hash
	payloadHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" // Empty hash
	if len(bodyBytes) > 0 {
		payloadHash = "" // Let the signer calculate it
	}

	err = signer.SignHTTP(ctx, credentials, req, payloadHash, "execute-api", c.region, time.Now())
	if err != nil {
		if c.loggerFunc != nil {
			c.loggerFunc().Error("Failed to sign request", map[string]any{
				"error":     err.Error(),
				"subsystem": opts.Subsystem,
			})
		}
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	// Log request
	if c.loggerFunc != nil {
		c.loggerFunc().Debug("Making kernel service call", map[string]any{
			"service":   opts.ServicePrefix,
			"endpoint":  opts.Endpoint,
			"method":    opts.Method,
			"url":       fullURL,
			"subsystem": opts.Subsystem,
		})
	}

	// Execute request
	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		if c.loggerFunc != nil {
			c.loggerFunc().Error("Kernel service call failed", map[string]any{
				"error":     err.Error(),
				"service":   opts.ServicePrefix,
				"subsystem": opts.Subsystem,
			})
		}
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer httpResp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	duration := time.Since(start)

	// Build response
	response := &Response{
		StatusCode: httpResp.StatusCode,
		Headers:    make(map[string]string),
		Body:       respBody,
		Duration:   duration,
	}

	// Copy response headers
	for key, values := range httpResp.Header {
		if len(values) > 0 {
			response.Headers[key] = values[0]
		}
	}

	// Log response
	if c.loggerFunc != nil {
		c.loggerFunc().Debug("Kernel service call completed", map[string]any{
			"service":     opts.ServicePrefix,
			"status_code": httpResp.StatusCode,
			"duration_ms": duration.Milliseconds(),
			"subsystem":   opts.Subsystem,
		})
	}

	// Check for errors
	if httpResp.StatusCode >= 400 {
		if c.loggerFunc != nil {
			c.loggerFunc().Error("Kernel service returned error", map[string]any{
				"status_code": httpResp.StatusCode,
				"response":    string(respBody),
				"subsystem":   opts.Subsystem,
			})
		}
		return response, fmt.Errorf("kernel service error: status %d", httpResp.StatusCode)
	}

	return response, nil
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

	// If we are not overriding the default kernel selection behavior, use QAKernel for innovate or austin partners
	if kernelEnvOverride == "" && (c.partner == "innovate" || c.partner == "austin") {
		return "qakernel"
	}

	// If we are overriding the default kernel selection behavior, use the override value
	if kernelEnvOverride != "" {
		return kernelEnvOverride
	}

	// Else use the kernel env
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

// Unmarshal unmarshals the response body into the provided struct
func (r *Response) Unmarshal(v any) error {
	return json.Unmarshal(r.Body, v)
}

// Convenience functions for specific services

// K3Call makes an authenticated call to K3 API Gateway
func (c *Client) K3Call(ctx context.Context, endpoint string, body any, method string) (*Response, error) {
	if method == "" {
		method = "POST"
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
		method = "GET"
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
		method = "POST"
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
		method = "POST"
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
		method = "POST"
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
		method = "GET"
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
