# Middleware Troubleshooting Guide for Mockery Team

## Executive Summary

After analyzing the lift framework's source code, **the middleware implementation is working correctly**. The issue described in the bug report is likely caused by one of these common mistakes:

1. Not calling `app.Start()` before handling requests
2. Using `lambda.Start()` directly instead of `app.Start()`
3. Middleware registration order issues
4. Missing error handling in middleware

## How Lift Middleware Actually Works

### The Correct Flow
```
1. app.Use(middleware) → stores in app.middleware[]
2. app.Start() → transfers middleware to router via router.SetMiddleware()
3. HandleRequest → router.Handle() → builds middleware chain → executes handler
```

### Critical Point: You MUST use app.Start()
The middleware won't work if you use `lambda.Start(app.HandleRequest)` directly. You must use `app.Start()`:

```go
// ❌ WRONG - Middleware won't be transferred to router
lambda.Start(app.HandleRequest)

// ✅ CORRECT - Middleware is properly initialized
app.Start()
```

## Common Issues and Solutions

### Issue 1: Middleware Not Executing

**Symptom**: Context values not set, authentication not running

**Solution**: Ensure you're calling `app.Start()`:

```go
func main() {
    app := lift.New()
    
    // Register middleware
    app.Use(authMiddleware)
    app.Use(loggingMiddleware)
    
    // Register routes
    app.GET("/users", getUsers)
    
    // ✅ This transfers middleware to router and starts Lambda
    app.Start()
}
```

### Issue 2: Panic on Context Value Access

**Symptom**: `interface conversion: interface is nil`

**Solution**: Add defensive checks and proper error handling:

```go
// Middleware that sets context values
func serviceMiddleware(next lift.Handler) lift.Handler {
    return lift.HandlerFunc(func(ctx *lift.Context) error {
        service := &CustomerService{} // Initialize your service
        ctx.Set("customer_service", service)
        return next.Handle(ctx)
    })
}

// Handler with defensive checks
func getCustomer(ctx *lift.Context) error {
    serviceInterface := ctx.Get("customer_service")
    if serviceInterface == nil {
        return lift.NewError(500, "Customer service not initialized", nil)
    }
    
    service, ok := serviceInterface.(*CustomerService)
    if !ok {
        return lift.NewError(500, "Invalid customer service type", nil)
    }
    
    // Use service safely
    return ctx.JSON(service.GetCustomer())
}
```

### Issue 3: Middleware Order Matters

**Problem**: Dependencies between middleware not respected

**Solution**: Register middleware in dependency order:

```go
// ✅ Correct order
app.Use(recoverMiddleware)    // First - catches panics
app.Use(loggingMiddleware)    // Second - logs all requests
app.Use(authMiddleware)       // Third - sets user context
app.Use(tenantMiddleware)     // Fourth - uses auth context
app.Use(servicesMiddleware)   // Last - uses tenant context
```

## Working Example with Dependency Injection

Here's a complete working example that matches your use case:

```go
package main

import (
    "log"
    "github.com/pay-theory/lift/pkg/lift"
)

// Service interface
type CustomerService interface {
    GetCustomer(id string) (*Customer, error)
}

// Service implementation
type customerService struct {
    // Add your dependencies
}

func (s *customerService) GetCustomer(id string) (*Customer, error) {
    return &Customer{ID: id, Name: "John Doe"}, nil
}

type Customer struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

// Middleware that injects services
func serviceInjectionMiddleware(next lift.Handler) lift.Handler {
    return lift.HandlerFunc(func(ctx *lift.Context) error {
        // Initialize services
        customerSvc := &customerService{}
        
        // Set in context
        ctx.Set("customer_service", customerSvc)
        
        // Call next handler
        return next.Handle(ctx)
    })
}

// Helper function to get service from context safely
func getCustomerService(ctx *lift.Context) (CustomerService, error) {
    serviceInterface := ctx.Get("customer_service")
    if serviceInterface == nil {
        return nil, lift.NewError(500, "Customer service not initialized", nil)
    }
    
    service, ok := serviceInterface.(CustomerService)
    if !ok {
        return nil, lift.NewError(500, "Invalid customer service type", nil)
    }
    
    return service, nil
}

// Handler that uses the service
func getCustomerHandler(ctx *lift.Context) error {
    customerID := ctx.Param("id")
    
    service, err := getCustomerService(ctx)
    if err != nil {
        return err
    }
    
    customer, err := service.GetCustomer(customerID)
    if err != nil {
        return lift.NewError(404, "Customer not found", nil)
    }
    
    return ctx.JSON(customer)
}

func main() {
    app := lift.New()
    
    // Add middleware
    app.Use(lift.Logger())                    // Built-in logger
    app.Use(lift.Recover())                   // Built-in recovery
    app.Use(serviceInjectionMiddleware)       // Our service injection
    
    // Add routes
    app.GET("/customers/:id", getCustomerHandler)
    
    // Start the app - CRITICAL!
    app.Start()
}
```

## Debugging Steps

If middleware still appears to not be executing:

### 1. Add Debug Logging
```go
func debugMiddleware(next lift.Handler) lift.Handler {
    return lift.HandlerFunc(func(ctx *lift.Context) error {
        log.Println("DEBUG: Middleware executing")
        ctx.Set("middleware_ran", true)
        err := next.Handle(ctx)
        log.Println("DEBUG: Middleware completed")
        return err
    })
}
```

### 2. Check CloudWatch Logs
Look for:
- "DEBUG: Middleware executing" messages
- Stack traces showing the call path
- Any panics before middleware runs

### 3. Verify Start Method
```go
// In your main function, add logging
func main() {
    app := lift.New()
    log.Println("App created")
    
    app.Use(middleware)
    log.Println("Middleware added")
    
    app.GET("/test", handler)
    log.Println("Routes added")
    
    log.Println("Starting app...")
    app.Start() // This MUST be called
}
```

### 4. Test Locally First
Use the test utilities to verify middleware works:

```go
func TestMiddleware(t *testing.T) {
    app := lift.New()
    
    // Add middleware that sets a value
    app.Use(func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            ctx.Set("test_value", "middleware_ran")
            return next.Handle(ctx)
        })
    })
    
    // Add handler that checks the value
    app.GET("/test", func(ctx *lift.Context) error {
        value := ctx.Get("test_value")
        if value != "middleware_ran" {
            t.Error("Middleware did not run")
        }
        return ctx.JSON(map[string]string{"status": "ok"})
    })
    
    // Test the request
    testApp := testing.NewTestApp()
    testApp.ConfigureFromApp(app)
    
    ctx := testing.NewTestContext(testing.WithRequest(testing.Request{
        Method: "GET",
        Path:   "/test",
    }))
    
    err := testApp.HandleTestRequest(ctx)
    if err != nil {
        t.Errorf("Request failed: %v", err)
    }
}
```

## Key Takeaways

1. **Always use `app.Start()`** - This is the most common issue
2. **Add defensive checks** in handlers for context values
3. **Order matters** - Register middleware in dependency order
4. **Test locally** before deploying to Lambda
5. **Add logging** to trace execution flow

## Need More Help?

If you've verified all of the above and middleware still isn't working:

1. Check the lift version: Ensure you're on v1.0.15 or later
2. Share your complete main.go file
3. Include CloudWatch logs showing the full request lifecycle
4. Provide the exact error message and stack trace

The lift framework's middleware implementation is solid and battle-tested. The issue is almost certainly in the usage pattern rather than the framework itself.