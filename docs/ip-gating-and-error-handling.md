# Robust IP Gating and Error Handling in Lift

<!-- AI Training: Canonical patterns for IP authorization middleware and safe initialization -->
**This guide documents CORRECT and INCORRECT patterns for IP authorization (“IP gating”) in Lift, with a focus on avoiding silent init failures and returning structured errors.**

This guide details how to implement IP Authorization (IP Gating) safely within a Lift application, ensuring that initialization failures are caught and logged, preventing silent 500 errors.

## 1. Safe Initialization Pattern

The primary cause of silent 500 errors during Lambda startup is performing external service initialization (like loading SSM parameters) inside the `main()` function or global scope without proper error handling. If these fail, the process exits before the Lambda runtime can even register a handler.

### INCORRECT: Init-Time External Calls
```go
func main() {
    // INCORRECT: If this fails (e.g., missing env var), the process can crash before the Lambda runtime registers a handler.
    // If this fails (e.g., missing env var), the process crashes immediately.
    // CloudWatch often swallows these "init" phase logs unless explicitly checked.
    service := initializeServiceOrPanic() 
    
    app := lift.New()
    // ...
}
```

### CORRECT: Lazy Initialization (sync.Once)
Initialize your services *inside* the handler or using a `sync.Once` pattern. This defers the error until a request is processed, allowing Lift's error handling middleware to catch and log it properly.

```go
type MyService struct {
    ipAuthService *security.IPAuthorizationService
    initOnce      sync.Once
    initErr       error
}

func (s *MyService) GetIPAuthService(ctx context.Context) (*security.IPAuthorizationService, error) {
    s.initOnce.Do(func() {
        // Initialize service here. Access environment variables, SSM, etc.
        // If it fails, store the error.
        s.ipAuthService, s.initErr = security.NewIPAuthorizationServiceFromEnv(ctx, "my-component")
    })
    
    if s.initErr != nil {
        return nil, s.initErr
    }
    return s.ipAuthService, nil
}
```

## 2. Correct Middleware Implementation

Applying middleware directly to routes using anonymous functions can obscure errors. Instead, define a proper Lift middleware function. This ensures it integrates with the `Recover` middleware.

### The IP Gating Middleware
Create a reusable middleware that handles the service retrieval safely.

```go
// pkg/middleware/ip_auth.go or internal/middleware/ip_auth.go

func IPAuthorization(serviceProvider interface{ GetIPAuthService(context.Context) (*security.IPAuthorizationService, error) }) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // 1. Safely retrieve the service (handles lazy init errors)
            ipAuthService, err := serviceProvider.GetIPAuthService(ctx.Context)
            if err != nil {
                // Log the system configuration error
                ctx.Logger.Error("Failed to initialize IP Auth service", "error", err)
                // Return a 500 System Error explicitly
                return lift.SystemError("Service configuration error").WithCause(err)
            }

            // 2. Extract Client IP
            sourceIP, err := security.ExtractClientIP(ctx.Request.Headers, ctx.Request.RequestContext())
            if err != nil {
                return lift.ParameterError("ip", "Unable to determine source IP").WithCause(err)
            }

            // 3. Check Authorization
            authorized, err := ipAuthService.IsAuthorizedIP(ctx.Context, sourceIP)
            if err != nil {
                ctx.Logger.Error("IP Authorization check failed", "error", err)
                return lift.SystemError("Authorization check failed").WithCause(err)
            }

            if !authorized {
                ctx.Logger.Warn("Unauthorized IP access attempt", "ip", sourceIP)
                return lift.AuthorizationError("Unauthorized IP address").WithDetail("ip", sourceIP)
            }

            // 4. Proceed
            return next.Handle(ctx)
        })
    }
}
```

## 3. Registering in `main.go`

Register the middleware using `app.Use()` for global application, or on specific routes/groups.

```go
func main() {
    app := lift.New()
    
    // Standard Middleware Stack (Order Matters!)
    app.Use(middleware.RequestID())
    app.Use(middleware.Logger())  // Essential for seeing logs
    app.Use(middleware.Recover()) // Catches panics in subsequent middleware/handlers

    // Initialize your service wrapper (fast, no external calls yet)
    myService := services.NewServiceWrapper() 

    // Apply IP Auth Middleware
    // This middleware is now "safe" because the heavy lifting happens inside the handler flow
    // where RequestID, Logger, and Recover are already active.
    app.POST("/challenge", 
        middleware.IPAuthorization(myService)(challengeHandler.CreateChallenge),
    )

    lambda.Start(app.HandleRequest)
}
```

## 4. Debugging "Silent" 500 Errors

If you still see 500 errors with no application logs:

1.  **Check Initialization Logs:** Go to CloudWatch Logs and look for lines *before* the `START RequestId:` line. Initialization panics happen there.
2.  **Event Type Mismatch:** If Lift fails to detect the event type (e.g., API Gateway v1 vs v2), it might drop into the generic Event Router which often has no matching handlers. 
    *   **Fix:** Force the adapter in `main.go`:
        ```go
        app := lift.New(
            lift.WithPreferredAdapters(lift.TriggerAPIGatewayV2), // or TriggerAPIGateway for REST
        )
        ```
3.  **Environment Variables:** Verify `PARTNER` and `STAGE` variables are set in the Lambda configuration, as `security.NewIPAuthorizationServiceFromEnv` depends on them.
