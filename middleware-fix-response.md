# Response to Mockery Team: Middleware Not Executing Issue

## The Problem

You've discovered a bug in the lift framework (confirmed in the current codebase). The middleware is not being transferred from the app to the router when using Lambda.

## Root Cause

The `app.HandleRequest()` method (used by Lambda) does NOT call `app.Start()`, which means middleware is never transferred to the router. This is why your middleware isn't executing.

Here's what's happening:

1. You register middleware with `app.Use()` → stored in `app.middleware`
2. You call `lambda.Start(app.HandleRequest)` 
3. `HandleRequest` processes the request but never calls `app.Start()`
4. Without `Start()`, middleware is never transferred to the router via `router.SetMiddleware()`
5. The router executes handlers directly without any middleware

## Why Tests Work

The test method `HandleTestRequest()` DOES call `app.Start()` first (line 297 in app.go), which is why middleware works in tests but not in production Lambda.

## Temporary Workaround

Until this is fixed in lift, you need to manually call `app.Start()` before starting Lambda:

```go
func main() {
    app := lift.New()
    
    // Add middleware
    app.Use(func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            ctx.Set("required_service", "some_service")
            return next.Handle(ctx)
        })
    })
    
    // Add routes
    app.GET("/test", func(ctx *lift.Context) error {
        service := ctx.Get("required_service").(string)
        return ctx.JSON(map[string]string{"service": service})
    })
    
    // CRITICAL: Call Start() to transfer middleware to router
    if err := app.Start(); err != nil {
        panic(err)
    }
    
    // Now start Lambda
    lambda.Start(app.HandleRequest)
}
```

## Why Our Initial Guidance Was Wrong

When we said "use `app.Start()` instead of `lambda.Start(app.HandleRequest)`", we were wrong. The `app.Start()` method only transfers middleware - it doesn't start any server or Lambda handler. That's why your Lambda exits immediately.

The correct pattern is:
1. Call `app.Start()` to initialize middleware
2. Then call `lambda.Start(app.HandleRequest)` to start Lambda

## Permanent Fix Required

The lift framework needs to be updated so that `HandleRequest` calls `Start()` internally, just like `HandleTestRequest` does. This would be a one-line fix in the framework:

```go
func (a *App) HandleRequest(ctx context.Context, event interface{}) (interface{}, error) {
    // Ensure the app is started (this line is missing in v1.0.15)
    if err := a.Start(); err != nil {
        return nil, err
    }
    
    // ... rest of the method
}
```

## Action Items

1. **Immediate**: Use the workaround above - call `app.Start()` before `lambda.Start()`
2. **Long-term**: File a bug report with the lift team to fix `HandleRequest`
3. **Testing**: Your tests work because `HandleTestRequest` already calls `Start()`

## Example Working Code

```go
package main

import (
    "log"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/pay-theory/lift/pkg/lift"
)

func main() {
    app := lift.New()
    
    // Recovery middleware
    app.Use(lift.Recover())
    
    // Logger middleware
    app.Use(lift.Logger())
    
    // Service injection middleware
    app.Use(func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Initialize your services
            customerService := initializeCustomerService()
            ctx.Set("customer_service", customerService)
            
            return next.Handle(ctx)
        })
    })
    
    // Routes
    app.GET("/customers/:id", getCustomer)
    
    // CRITICAL: Initialize middleware before Lambda starts
    if err := app.Start(); err != nil {
        log.Fatal("Failed to start app:", err)
    }
    
    // Start Lambda handler
    lambda.Start(app.HandleRequest)
}

func getCustomer(ctx *lift.Context) error {
    // This will now work because middleware ran
    service := ctx.Get("customer_service").(CustomerService)
    
    customerID := ctx.Param("id")
    customer, err := service.GetCustomer(customerID)
    if err != nil {
        return lift.NewError(404, "Customer not found", nil)
    }
    
    return ctx.JSON(customer)
}
```

This is a framework bug that needs to be fixed, but the workaround above will get your middleware working immediately.