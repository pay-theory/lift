# Lift Middleware Patterns Guide

*Date: 2025-06-16-06_56_39*
*Author: Pay Theory Lift Team*

## Overview

This guide provides the correct Lift middleware patterns and answers common questions about JWT authentication, rate limiting, CORS, WebSocket routes, and route group middleware.

## 1. JWT Authentication Middleware

### Correct JWT Configuration

The correct way to configure JWT authentication in Lift is using the `middleware.JWT()` function from the `pkg/middleware/auth.go` package:

```go
import (
    "github.com/pay-theory/lift/pkg/middleware"
    "github.com/pay-theory/lift/pkg/security"
)

// Basic JWT middleware
app.Use(middleware.JWT(security.JWTConfig{
    SigningMethod: "HS256",
    SecretKey:     "your-secret-key",
    Issuer:        "your-app",
    Audience:      []string{"your-audience"},
}))

// Advanced JWT configuration
app.Use(middleware.JWT(security.JWTConfig{
    SigningMethod:   "RS256",
    PublicKeyPath:   "/path/to/public.pem",
    Issuer:          "your-auth-service",
    Audience:        []string{"api", "web"},
    MaxAge:          time.Hour,
    RequireTenantID: true,
    ValidateTenant: func(tenantID string) error {
        // Custom tenant validation
        return validateTenantAccess(tenantID)
    },
}))
```

### Alternative: App-Level JWT Configuration

You can also configure JWT at the app level using options:

```go
app := lift.New(
    lift.WithJWTAuth(lift.JWTAuthConfig{
        Secret:    "your-secret-key",
        Algorithm: "HS256",
        SkipPaths: []string{"/health", "/login"},
        Validator: func(claims jwt.MapClaims) error {
            // Custom validation logic
            return nil
        },
    }),
)
```

### Optional JWT Authentication

For endpoints that can work with or without authentication:

```go
app.Use(middleware.JWTOptional(security.JWTConfig{
    SigningMethod: "HS256",
    SecretKey:     "your-secret-key",
}))
```

## 2. Rate Limiting Middleware

### Basic Rate Limiting

```go
import "github.com/pay-theory/lift/pkg/middleware"

// Basic rate limiting
app.Use(middleware.RateLimit(middleware.RateLimitConfig{
    DefaultLimit:  100,
    DefaultWindow: time.Minute,
    DynamORM:      dynamormInstance, // Required for distributed rate limiting
}))
```

### Different Rate Limits for Different Route Groups

```go
// Global rate limiting (applied to all routes)
app.Use(middleware.RateLimit(middleware.RateLimitConfig{
    DefaultLimit:  1000,
    DefaultWindow: time.Hour,
    DynamORM:      dynamormInstance,
}))

// API routes with stricter limits
apiGroup := app.Group("/api/v1")
apiGroup.Use(middleware.RateLimit(middleware.RateLimitConfig{
    DefaultLimit:  100,
    DefaultWindow: time.Minute,
    DynamORM:      dynamormInstance,
}))

// OAuth routes with very strict limits
oauthGroup := apiGroup.Group("/oauth")
oauthGroup.Use(middleware.IPRateLimit(10, time.Minute)) // 10 requests per minute per IP
```

### Partner-Based Rate Limiting (from JWT Claims)

```go
app.Use(middleware.RateLimit(middleware.RateLimitConfig{
    DefaultLimit:  100,
    DefaultWindow: time.Minute,
    DynamORM:      dynamormInstance,
    TenantLimits: map[string]int{
        "premium":    1000, // Premium partners get higher limits
        "enterprise": 5000, // Enterprise partners get even higher limits
        "basic":      100,  // Basic partners get default limits
    },
    KeyFunc: func(ctx *lift.Context) *middleware.RateLimitKey {
        // Extract tenant from JWT claims
        tenantID := ctx.TenantID()
        userID := ctx.UserID()
        
        return &middleware.RateLimitKey{
            Identifier: fmt.Sprintf("tenant:%s:user:%s", tenantID, userID),
            Resource:   ctx.Request.Path,
            Operation:  ctx.Request.Method,
            Metadata: map[string]string{
                "tenant_id": tenantID,
                "user_id":   userID,
            },
        }
    },
}))
```

### IP-Based Rate Limiting for OAuth Flows

```go
// OAuth routes with IP-based rate limiting
oauthGroup := app.Group("/oauth")
oauthGroup.Use(middleware.IPRateLimit(5, time.Minute)) // 5 requests per minute per IP

// Or more sophisticated IP-based limiting
oauthGroup.Use(middleware.RateLimit(middleware.RateLimitConfig{
    DefaultLimit:  5,
    DefaultWindow: time.Minute,
    DynamORM:      dynamormInstance,
    KeyFunc: func(ctx *lift.Context) *middleware.RateLimitKey {
        ip := ctx.Header("X-Forwarded-For")
        if ip == "" {
            ip = ctx.Header("X-Real-IP")
        }
        
        return &middleware.RateLimitKey{
            Identifier: fmt.Sprintf("ip:%s", ip),
            Resource:   ctx.Request.Path,
            Operation:  ctx.Request.Method,
            Metadata: map[string]string{
                "ip": ip,
                "flow": "oauth",
            },
        }
    },
}))
```

## 3. CORS Middleware

### Correct CORS Configuration

The CORS middleware is available in the `pkg/middleware` package:

```go
import "github.com/pay-theory/lift/pkg/middleware"

// Basic CORS (allows all origins)
app.Use(middleware.CORS([]string{"*"}))

// Secure CORS configuration
app.Use(middleware.CORS([]string{
    "https://app.example.com",
    "https://admin.example.com",
}))
```

### Advanced CORS with Custom Configuration

For more advanced CORS configuration, you can create a custom middleware:

```go
func CORSMiddleware(config CORSConfig) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            origin := ctx.Header("Origin")
            
            // Check if origin is allowed
            allowed := false
            for _, allowedOrigin := range config.AllowOrigins {
                if allowedOrigin == "*" || allowedOrigin == origin {
                    allowed = true
                    break
                }
            }
            
            if allowed {
                ctx.Response.Header("Access-Control-Allow-Origin", origin)
                ctx.Response.Header("Access-Control-Allow-Methods", strings.Join(config.AllowMethods, ", "))
                ctx.Response.Header("Access-Control-Allow-Headers", strings.Join(config.AllowHeaders, ", "))
                ctx.Response.Header("Access-Control-Allow-Credentials", "true")
                ctx.Response.Header("Access-Control-Max-Age", "86400")
            }
            
            // Handle preflight requests
            if ctx.Request.Method == "OPTIONS" {
                return ctx.Status(204).JSON(nil)
            }
            
            return next.Handle(ctx)
        })
    }
}

type CORSConfig struct {
    AllowOrigins []string
    AllowMethods []string
    AllowHeaders []string
}

// Usage
app.Use(CORSMiddleware(CORSConfig{
    AllowOrigins: []string{"https://app.example.com"},
    AllowMethods: []string{"GET", "POST", "PUT", "DELETE"},
    AllowHeaders: []string{"Authorization", "Content-Type"},
}))
```

## 4. WebSocket Routes and Authentication

### Setting Up WebSocket Routes

WebSocket routes in Lift use a different pattern than HTTP routes:

```go
// Enable WebSocket support
app := lift.New(lift.WithWebSocketSupport())

// Register WebSocket routes
app.WebSocket("$connect", handleConnect)
app.WebSocket("$disconnect", handleDisconnect)
app.WebSocket("$default", handleDefault)
app.WebSocket("sendMessage", handleSendMessage)
app.WebSocket("joinRoom", handleJoinRoom)

// Use the WebSocket handler for Lambda
lambda.Start(app.WebSocketHandler())
```

### WebSocket Authentication Middleware

```go
import "github.com/pay-theory/lift/pkg/middleware"

// WebSocket JWT authentication
app.Use(middleware.WebSocketAuth(middleware.WebSocketAuthConfig{
    JWTConfig: security.JWTConfig{
        SigningMethod: "HS256",
        SecretKey:     "your-secret-key",
    },
    TokenExtractor: func(ctx *lift.Context) string {
        // Extract from query parameters (common for WebSocket)
        token := ctx.Query("Authorization")
        if token == "" {
            token = ctx.Query("token")
        }
        return strings.TrimPrefix(token, "Bearer ")
    },
    SkipRoutes: []string{"$connect"}, // Skip auth for connect if needed
    OnError: func(ctx *lift.Context, err error) error {
        return ctx.Status(401).JSON(map[string]string{
            "error": "Authentication failed",
        })
    },
}))
```

### WebSocket Handler Example

```go
func handleConnect(ctx *lift.Context) error {
    wsCtx, err := ctx.AsWebSocket()
    if err != nil {
        return err
    }
    
    connectionID := wsCtx.ConnectionID()
    userID := ctx.UserID() // Available after authentication
    
    // Store connection
    if err := storeConnection(connectionID, userID); err != nil {
        return ctx.Status(500).JSON(map[string]string{
            "error": "Failed to store connection",
        })
    }
    
    return ctx.Status(200).JSON(map[string]string{
        "status": "connected",
        "connectionId": connectionID,
    })
}

func handleSendMessage(ctx *lift.Context) error {
    wsCtx, err := ctx.AsWebSocket()
    if err != nil {
        return err
    }
    
    // Parse message from request body
    var message struct {
        Content string `json:"content"`
        RoomID  string `json:"roomId"`
    }
    
    if err := ctx.ParseRequest(&message); err != nil {
        return err
    }
    
    // Broadcast to room members
    connections, err := getConnectionsByRoom(message.RoomID)
    if err != nil {
        return err
    }
    
    return wsCtx.BroadcastMessage(ctx.Context(), connections, []byte(message.Content))
}
```

## 5. Route Group Middleware

### Applying Middleware to Route Groups

Route groups in Lift don't have a `Use()` method directly, but you can apply middleware in several ways:

#### Method 1: Apply Middleware Globally, Then Use Groups

```go
// Global middleware
app.Use(middleware.Logger())
app.Use(middleware.Recovery())

// Public routes (no additional middleware)
app.GET("/health", healthHandler)

// Protected API routes
apiGroup := app.Group("/api/v1")
// Apply JWT middleware to all API routes by wrapping handlers
apiGroup.GET("/users", middleware.JWT(jwtConfig)(getUsersHandler))
apiGroup.POST("/users", middleware.JWT(jwtConfig)(createUserHandler))
```

#### Method 2: Create Middleware Chains

```go
// Create middleware chains
jwtChain := func(handler lift.Handler) lift.Handler {
    return middleware.JWT(jwtConfig)(
        middleware.RateLimit(rateLimitConfig)(handler),
    )
}

// Apply to route groups
apiGroup := app.Group("/api/v1")
apiGroup.GET("/users", jwtChain(getUsersHandler))
apiGroup.POST("/users", jwtChain(createUserHandler))
```

#### Method 3: Enhanced Route Group Pattern

You can extend the route group functionality:

```go
type EnhancedRouteGroup struct {
    *lift.RouteGroup
    middleware []lift.Middleware
}

func (g *EnhancedRouteGroup) Use(middleware lift.Middleware) *EnhancedRouteGroup {
    g.middleware = append(g.middleware, middleware)
    return g
}

func (g *EnhancedRouteGroup) GET(path string, handler lift.Handler) {
    // Apply all middleware to the handler
    finalHandler := handler
    for i := len(g.middleware) - 1; i >= 0; i-- {
        finalHandler = g.middleware[i](finalHandler)
    }
    g.RouteGroup.GET(path, finalHandler)
}

// Usage
func NewEnhancedGroup(app *lift.App, prefix string) *EnhancedRouteGroup {
    return &EnhancedRouteGroup{
        RouteGroup: app.Group(prefix),
        middleware: make([]lift.Middleware, 0),
    }
}

// Example usage
apiGroup := NewEnhancedGroup(app, "/api/v1")
apiGroup.Use(middleware.JWT(jwtConfig))
apiGroup.Use(middleware.RateLimit(rateLimitConfig))
apiGroup.GET("/users", getUsersHandler)
```

## 6. Complete Example: Putting It All Together

```go
package main

import (
    "time"
    
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/pay-theory/lift/pkg/middleware"
    "github.com/pay-theory/lift/pkg/security"
)

func main() {
    // Create app with WebSocket support
    app := lift.New(lift.WithWebSocketSupport())
    
    // Global middleware
    app.Use(middleware.Logger())
    app.Use(middleware.Recovery())
    app.Use(middleware.CORS([]string{"https://app.example.com"}))
    
    // Public routes (no auth required)
    app.GET("/health", healthHandler)
    app.POST("/auth/login", loginHandler)
    
    // Protected API routes
    apiGroup := app.Group("/api/v1")
    
    // JWT middleware for API routes
    jwtConfig := security.JWTConfig{
        SigningMethod: "HS256",
        SecretKey:     "your-secret-key",
    }
    
    // Rate limiting for API routes
    apiRateLimit := middleware.RateLimitConfig{
        DefaultLimit:  100,
        DefaultWindow: time.Minute,
        DynamORM:      dynamormInstance,
    }
    
    // Apply middleware to API routes
    apiMiddleware := func(handler lift.Handler) lift.Handler {
        return middleware.JWT(jwtConfig)(
            middleware.RateLimit(apiRateLimit)(handler),
        )
    }
    
    apiGroup.GET("/users", apiMiddleware(getUsersHandler))
    apiGroup.POST("/users", apiMiddleware(createUserHandler))
    
    // OAuth routes with stricter rate limiting
    oauthGroup := apiGroup.Group("/oauth")
    oauthRateLimit := middleware.RateLimitConfig{
        DefaultLimit:  10,
        DefaultWindow: time.Minute,
        DynamORM:      dynamormInstance,
    }
    
    oauthGroup.POST("/token", middleware.RateLimit(oauthRateLimit)(tokenHandler))
    oauthGroup.POST("/refresh", middleware.RateLimit(oauthRateLimit)(refreshHandler))
    
    // WebSocket routes with authentication
    app.Use(middleware.WebSocketAuth(middleware.WebSocketAuthConfig{
        JWTConfig: jwtConfig,
        TokenExtractor: func(ctx *lift.Context) string {
            return ctx.Query("token")
        },
    }))
    
    app.WebSocket("$connect", handleConnect)
    app.WebSocket("$disconnect", handleDisconnect)
    app.WebSocket("sendMessage", handleSendMessage)
    
    // Start Lambda handler
    lambda.Start(app.WebSocketHandler())
}

func healthHandler(ctx *lift.Context) error {
    return ctx.OK(map[string]string{"status": "healthy"})
}

func getUsersHandler(ctx *lift.Context) error {
    // Handler implementation
    return ctx.OK([]string{"user1", "user2"})
}

// ... other handlers
```

## 7. Common Middleware Issues and Solutions

### Issue: `middleware.Logger()` type doesn't match `lift.Middleware`

**Solution**: Use the correct import and function signature:

```go
import "github.com/pay-theory/lift/pkg/middleware"

// Correct usage
app.Use(middleware.Logger())
```

### Issue: `middleware.Recovery` is undefined

**Solution**: Use the correct function name:

```go
// Correct
app.Use(middleware.Recover())

// Not middleware.Recovery
```

### Issue: Route groups don't have `Use()` method

**Solution**: Apply middleware using function composition or create enhanced route groups as shown above.

### Issue: WebSocket routes don't work with regular HTTP routing

**Solution**: Use the WebSocket-specific routing and handler:

```go
// For WebSocket
app.WebSocket("$connect", handler)
lambda.Start(app.WebSocketHandler())

// For HTTP
app.GET("/api/endpoint", handler)
lambda.Start(app.HandleRequest)
```

## 8. Best Practices

1. **Order Matters**: Apply middleware in the correct order (logging first, auth after, etc.)
2. **Use Specific Middleware**: Apply rate limiting and auth only where needed
3. **WebSocket Authentication**: Extract tokens from query parameters for WebSocket connections
4. **Error Handling**: Always provide custom error handlers for middleware
5. **Performance**: Use DynamORM for distributed rate limiting in production
6. **Security**: Never expose internal errors in production environments

This guide should resolve all the middleware configuration issues you've encountered. The key is using the correct import paths and understanding that Lift has specific patterns for different types of middleware and routing. 