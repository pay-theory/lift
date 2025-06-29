# Issue Report: Middleware Not Executing Before Route Handlers in AWS Lambda

## Summary
When using the lift framework (v1.0.15) in AWS Lambda with API Gateway, middleware registered with `app.Use()` is not being executed before route handlers are called, causing runtime panics when handlers depend on context values set by middleware.

## Environment
- **lift version**: v1.0.15
- **Go version**: 1.21
- **AWS Lambda Runtime**: provided.al2
- **Deployment**: AWS Lambda + API Gateway (REST API)

## Expected Behavior
Middleware registered with `app.Use()` should execute in order before any route handler is called, allowing middleware to:
1. Set context values that handlers depend on
2. Perform authentication/authorization
3. Handle errors and panics
4. Log requests

## Actual Behavior
Route handlers are being called directly without middleware execution, causing:
- Panic when handlers try to access context values that should be set by middleware
- Authentication middleware not running, leaving requests unauthenticated
- No request logging from logger middleware
- No panic recovery from recovery middleware

## Reproduction Steps

1. Create a simple Lambda function with middleware and routes:

```go
package main

import (
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/pay-theory/lift/pkg/lift"
)

func main() {
    app := lift.New()
    
    // Add middleware that sets a required context value
    app.Use(func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            ctx.Set("required_service", "some_service")
            return next.Handle(ctx)
        })
    })
    
    // Add route that depends on the context value
    app.GET("/test", func(ctx *lift.Context) error {
        // This will panic because middleware didn't run
        service := ctx.Get("required_service").(string)
        return ctx.JSON(map[string]string{"service": service})
    })
    
    lambda.Start(app.HandleRequest)
}
```

2. Deploy to AWS Lambda with API Gateway
3. Call the endpoint: `GET /test`
4. Observe panic in CloudWatch logs

## Stack Trace
```
{
    "errorMessage": "interface conversion: interface is nil, not services.CustomerService",
    "errorType": "TypeAssertionError",
    "stackTrace": [
        {
            "path": "mockery/internal/handlers/customers/get.go",
            "line": 11,
            "label": "GetCustomer"
        },
        {
            "path": "github.com/pay-theory/lift@v1.0.15/pkg/lift/handler.go",
            "line": 13,
            "label": "HandlerFunc.Handle"
        },
        {
            "path": "github.com/pay-theory/lift@v1.0.15/pkg/lift/router.go",
            "line": 93,
            "label": "(*Router).Handle"
        },
        {
            "path": "github.com/pay-theory/lift@v1.0.15/pkg/lift/app.go",
            "line": 256,
            "label": "(*App).HandleRequest"
        }
    ]
}
```

## Root Cause Analysis
The stack trace shows that the request goes directly from `app.HandleRequest` to `router.Handle` to the route handler, bypassing the middleware chain entirely. This suggests that:

1. The middleware chain is not being properly constructed or invoked
2. The router is finding and calling handlers directly without going through the middleware pipeline
3. There may be an issue with how `app.HandleRequest` processes the middleware chain

## Workaround Attempts
1. **Moving middleware registration before routes**: No effect
2. **Using different middleware patterns**: No effect
3. **Simplifying to basic middleware**: Still not executed

## Comparison with Working Implementation
The challenge-service example in the same codebase works correctly, but it doesn't use complex middleware that sets context values required by handlers. The key difference is that working examples don't have handlers that depend on middleware-set context values.

## Impact
This issue makes it impossible to:
- Implement authentication/authorization middleware
- Use dependency injection patterns
- Share services or database connections via context
- Implement proper error handling and recovery
- Add request logging

## Request for Assistance
1. Is there a specific order or pattern required for middleware to work correctly in Lambda?
2. Are there known issues with middleware in the Lambda environment?
3. Is there a different way to register middleware that ensures execution?
4. Could this be related to how API Gateway proxy integration works with lift?

## Additional Context
- The same code pattern works in other Go web frameworks (gin, echo, etc.)
- Simple routes without middleware dependencies work fine
- The issue only manifests when handlers depend on context values set by middleware
- No middleware logs appear in CloudWatch, confirming they're not being executed