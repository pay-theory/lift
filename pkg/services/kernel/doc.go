// Package kernel provides authenticated cross-account calls to kernel services.
//
// This package enables partner account services to make SigV4-authenticated API calls
// to services running in the kernel account (qakernel or production kernel).
//
// # Overview
//
// The kernel client handles:
//   - STS AssumeRole for cross-account authentication
//   - SigV4 request signing for API Gateway
//   - Automatic kernel environment detection (qakernel vs kernel)
//   - URL construction with region/service prefix support
//   - Integration with Lift observability (logger, metrics, tracing)
//
// # Authentication Modes
//
// Mode 1: Shared Role (Default for trusted partners in AWS Organization)
//   - Uses role: kernel-access
//   - No External ID required
//   - For partners: qakernel, paytheory, etc.
//
// Mode 2: External Partner Role (For partners outside AWS Organization)
//   - Uses role: kernel-access-external
//   - Requires External ID via K3_EXTERNAL_ID environment variable
//   - For external partners with additional security requirements
//
// # Environment Variables
//
// Required:
//   - PARTNER: Partner identifier (e.g., "qakernel", "paytheory")
//   - STAGE: Deployment stage (e.g., "dev", "stage", "prod")
//
// Optional:
//   - TARGET_REGION: AWS region (default: "us-east-1")
//   - K3_EXTERNAL_ID: External ID for Mode 2 authentication
//   - KERNEL_ENV_OVERRIDE: Force kernel environment ("qakernel" or "kernel")
//
// # Kernel Environment Selection
//
// The client automatically selects the kernel environment based on partner:
//   - Partners "innovate" and "austin" → qakernel (account: 058264189048)
//   - All other partners → kernel (account: 075149869707)
//   - Override with KERNEL_ENV_OVERRIDE environment variable
//
// # URL Patterns
//
// K3 (without region):
//
//	https://k3.{kernel_env}.{stage}.com/{endpoint}
//
// Other services (with region):
//
//	https://{region}.{kernel_env}.{stage}.com/{service-prefix}/{endpoint}
//
// # Usage Examples
//
// Basic client creation:
//
//	import (
//	    "context"
//	    "github.com/pay-theory/lift/pkg/services/kernel"
//	    pkglogger "github.com/yourservice/internal/logger"
//	)
//
//	// Create kernel client using your service's singleton logger
//	ctx := context.Background()
//	client, err := kernel.NewClient(ctx, pkglogger.GetLiftLogger)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Call Paze wallet service:
//
//	response, err := client.PazeWalletCall(ctx, "decode-token", map[string]any{
//	    "token": "encrypted_paze_token...",
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
// Call K3 API:
//
//	response, err := client.K3Call(ctx, "v1/tokenize", map[string]any{
//	    "card_number": "4111111111111111",
//	    "exp_month":   "12",
//	    "exp_year":    "2025",
//	    "cvv":         "123",
//	}, "POST")
//	if err != nil {
//	    log.Printf("K3 call failed: %v", err)
//	    return err
//	}
//
// Call bin lookup service:
//
//	response, err := client.BinLookupCall(ctx, "?card_bin=411111", "GET", nil)
//	if err != nil {
//	    log.Printf("Bin lookup failed: %v", err)
//	    return err
//	}
//
//	var binData struct {
//	    CardBrand string `json:"card_brand"`
//	    CardType  string `json:"card_type"`
//	    BankName  string `json:"bank_name"`
//	}
//	if err := response.Unmarshal(&binData); err != nil {
//	    log.Printf("Failed to unmarshal bin data: %v", err)
//	    return err
//	}
//
// Generic call with custom options:
//
//	response, err := client.Call(ctx, &kernel.CallOptions{
//	    ServicePrefix:  "custom-service",
//	    Endpoint:       "api/v1/process",
//	    Method:         "POST",
//	    Body:           map[string]any{"data": "value"},
//	    IncludeRegion:  true,
//	    Subsystem:      "CUSTOM SERVICE",
//	    ConnectTimeout: 10 * time.Second,
//	    ReadTimeout:    30 * time.Second,
//	    Headers: map[string]string{
//	        "X-Custom-Header": "custom-value",
//	    },
//	})
//	if err != nil {
//	    log.Printf("Custom call failed: %v", err)
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
//	// Create kernel client with your singleton logger getter
//	kernelClient, err := kernel.NewClient(ctx, pkglogger.GetLiftLogger)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Register middleware
//	app.Use(kernel.KernelClientMiddleware(kernelClient))
//
//	// In handler function
//	func MyHandler(ctx *lift.Context) error {
//	    // Get kernel client from context
//	    kernelClient := kernel.GetKernelClient(ctx)
//	    if kernelClient == nil {
//	        return lift.SystemError("Kernel client not available")
//	    }
//
//	    // Use kernel client
//	    response, err := kernelClient.PazeWalletCall(ctx, "decode-token", data, "POST")
//	    if err != nil {
//	        return lift.SystemError("Paze call failed").WithCause(err)
//	    }
//
//	    return ctx.OK(response)
//	}
//
// # Convenience Functions
//
// The package provides service-specific convenience functions:
//
//   - K3Call: Call K3 API Gateway
//   - PazeWalletCall: Call Paze Wallet Key Service
//   - AppleWalletCall: Call Apple Wallet Key Service
//   - GoogleWalletCall: Call Google Wallet Key Service
//   - BinLookupCall: Call Bin Lookup Service
//   - BankDataCall: Call Bank Data Service
//
// All convenience functions use sensible defaults for method, region inclusion,
// and subsystem logging names.
//
// # Error Handling
//
// The client returns errors for:
//   - Missing required environment variables
//   - STS AssumeRole failures (invalid credentials, role not found)
//   - HTTP request failures (network errors, timeouts)
//   - HTTP status codes >= 400 (includes error details in logs)
//
// # Logging
//
// All calls are logged with structured logging including:
//   - Request details (service, endpoint, method)
//   - Response details (status code, duration)
//   - Error details (for failures)
//   - Subsystem tags (for filtering logs by service)
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
