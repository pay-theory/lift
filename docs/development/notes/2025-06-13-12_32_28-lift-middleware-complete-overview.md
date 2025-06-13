Let me clarify what **actually exists** in the current Lift codebase:

## ✅ **What Actually Exists:**

### 1. **Context Methods** ✅
```go
ctx.UserID()    // ✅ EXISTS - returns string from context values
ctx.TenantID()  // ✅ EXISTS - returns string from context values
ctx.Query()     // ✅ EXISTS - gets query parameters
ctx.Header()    // ✅ EXISTS - gets headers
```

### 2. **JWT Middleware** ✅
```go
middleware.JWT(config)         // ✅ EXISTS - JWT authentication middleware
middleware.JWTOptional(config) // ✅ EXISTS - optional JWT middleware
```

### 3. **App Configuration** ✅
```go
app.WithMetrics(metrics) // ✅ EXISTS - sets metrics collector
app.WithLogger(logger)   // ✅ EXISTS - sets logger
```

## ❌ **What I Hallucinated:**

### 1. **`ctx.Claims()`** ❌
This method **does NOT exist**. JWT claims are accessed through the security context:
```go
// WRONG (hallucinated):
claims := ctx.Claims()

// CORRECT (actual API):
secCtx := lift.WithSecurity(ctx)
principal := secCtx.GetPrincipal()
// Access user info through principal
```

### 2. **`lift.WithJWTAuth()`** ❌
This function **does NOT exist**. JWT auth is configured via middleware:
```go
// WRONG (hallucinated):
app := lift.New(lift.WithJWTAuth())

// CORRECT (actual API):
app := lift.New()
app.Use(middleware.JWT(jwtConfig))
```

## 🔧 **Correct Current API Usage:**

### JWT Authentication
```go
import (
    "github.com/pay-theory/lift/pkg/middleware"
    "github.com/pay-theory/lift/pkg/security"
)

// Configure JWT middleware
jwtConfig := security.JWTConfig{
    SigningMethod: "HS256",
    SecretKey:     "your-secret-key",
    Issuer:        "your-issuer",
}

app.Use(middleware.JWT(jwtConfig))
```

### Accessing User Information
```go
func handler(ctx *lift.Context) error {
    // These work:
    userID := ctx.UserID()     // ✅ Gets user ID from context values
    tenantID := ctx.TenantID() // ✅ Gets tenant ID from context values
    
    // For JWT claims, use security context:
    secCtx := lift.WithSecurity(ctx)
    principal := secCtx.GetPrincipal()
    if principal != nil {
        roles := principal.Roles
        scopes := principal.Scopes
    }
    
    return ctx.JSON(map[string]interface{}{
        "user_id":   userID,
        "tenant_id": tenantID,
    })
}
```

### App Setup
```go
app := lift.New()
app.WithLogger(logger)   // ✅ Works
app.WithMetrics(metrics) // ✅ Works
app.Use(middleware.JWT(jwtConfig)) // ✅ Works
```

## 🤦‍♂️ **My Mistake**

I mixed up:
1. **Planned/documented APIs** with **implemented APIs**
2. **Future roadmap features** with **current functionality**
3. **Conceptual examples** with **actual working code**

The Lift framework is still in active development, and I incorrectly presented some planned features as if they were already implemented. Thank you for catching this - it's exactly the kind of feedback that prevents wasted development time.

**The current working APIs are more limited but functional.** Always check the actual source code in `pkg/lift/` and `pkg/middleware/` for what's currently available.