// Package kernel provides authenticated cross-account calls to kernel services.
//
// This package enables partner account services to make SigV4-authenticated API calls
// to services running in the kernel account.
//
// # Overview
//
// The kernel client handles:
//   - STS AssumeRole for cross-account authentication
//   - SigV4 request signing for API Gateway
//   - Integration with Lift observability (logger, metrics, tracing)
//
// # Authentication Modes
//
// Mode 1: Shared Role (Default for trusted partners in AWS Organization)
//   - Uses role: kernel-access
//   - No External ID required
//
// Mode 2: External Partner Role (For partners outside AWS Organization)
//   - Uses role: kernel-access-external
//   - Requires External ID in ClientConfig
//
// # Configuration
//
// The kernel client is configured via ClientConfig. All configuration including
// service URLs and account IDs should be passed by the calling application
// (typically via environment variables set by CDK).
//
// Required configuration:
//   - AccountID: AWS account ID for kernel services (for STS assume role)
//
// Optional configuration:
//   - Region: AWS region (defaults to "us-east-1")
//   - ExternalID: For external partner authentication
//   - ConnectTimeout: Connection timeout (defaults to 30s)
//   - ReadTimeout: Read timeout (defaults to 30s)
//
// # Usage Examples
//
// Basic client creation:
//
//	import (
//	    "context"
//	    "os"
//	    "github.com/pay-theory/lift/pkg/services/kernel"
//	    pkglogger "github.com/yourservice/internal/logger"
//	)
//
//	// Create kernel client with configuration from environment
//	ctx := context.Background()
//	client, err := kernel.NewClient(ctx, pkglogger.GetLiftLogger, kernel.ClientConfig{
//	    AccountID: os.Getenv("KERNEL_ACCOUNT_ID"),
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Making a call to a kernel service:
//
//	response, err := client.Call(ctx, &kernel.CallOptions{
//	    BaseURL:  os.Getenv("KERNEL_PAZE_WALLET_URL"),
//	    Endpoint: "decode-token",
//	    Method:   http.MethodPost,
//	    Body:     map[string]any{"token": "encrypted_paze_token..."},
//	})
//	if err != nil {
//	    log.Printf("Paze call failed: %v", err)
//	    return err
//	}
//
//	var result map[string]any
//	if err := response.Unmarshal(&result); err != nil {
//	    log.Printf("Failed to unmarshal response: %v", err)
//	    return err
//	}
//
// Using with Lift middleware:
//
//	import (
//	    "github.com/pay-theory/lift/pkg/lift"
//	    "github.com/pay-theory/lift/pkg/services/kernel"
//	    pkglogger "github.com/yourservice/internal/logger"
//	)
//
//	// In main.go Lambda setup
//	app := lift.New()
//
//	// Create kernel client
//	kernelClient, err := kernel.NewClient(ctx, pkglogger.GetLiftLogger, kernel.ClientConfig{
//	    AccountID: os.Getenv("KERNEL_ACCOUNT_ID"),
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Register middleware
//	app.Use(kernel.KernelClientMiddleware(kernelClient))
//
//	// In handler function
//	func MyHandler(ctx *lift.Context) error {
//	    kernelClient := kernel.GetKernelClient(ctx)
//	    if kernelClient == nil {
//	        return lift.SystemError("Kernel client not available")
//	    }
//
//	    response, err := kernelClient.Call(ctx, &kernel.CallOptions{
//	        BaseURL:  os.Getenv("KERNEL_PAZE_WALLET_URL"),
//	        Endpoint: "decode-token",
//	        Method:   http.MethodPost,
//	        Body:     data,
//	    })
//	    if err != nil {
//	        return lift.SystemError("Call failed").WithCause(err)
//	    }
//
//	    return ctx.OK(response)
//	}
//
// # Error Handling
//
// The client returns errors for:
//   - Missing required configuration (AccountID)
//   - STS AssumeRole failures (invalid credentials, role not found)
//   - HTTP request failures (network errors, timeouts)
//   - HTTP status codes >= 400 (includes error details in logs)
//
// # Logging
//
// All calls are logged with structured logging including:
//   - Request details (endpoint, method, URL)
//   - Response details (status code, duration)
//   - Error details (for failures)
//
// # Security
//
// The client implements AWS best practices:
//   - STS AssumeRole with least-privilege temporary credentials
//   - SigV4 request signing for authentication
//   - External ID support for cross-organization access
//   - Automatic credential expiration handling
//   - TLS encryption for all HTTP requests
//
// # Performance
//
// The client is designed for performance:
//   - Reusable HTTP client with connection pooling
//   - Configurable timeouts (connect, read)
//   - Efficient request signing
//   - Minimal memory allocations
//
// # Thread Safety
//
// The Client struct is safe for concurrent use. Multiple goroutines can
// call methods on the same Client instance simultaneously.
package kernel
