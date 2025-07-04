# JWT Authentication: Production-Ready Auth with Lift

**This is the RECOMMENDED pattern for implementing JWT authentication in Lift applications.**

## What is This Example?

This example demonstrates the **STANDARD approach** for implementing production-ready JWT authentication. It shows the **preferred patterns** for multi-tenant auth, role-based access control, and secure token validation with Lift.

## Why Use This Authentication Pattern?

✅ **USE this pattern when:**
- Building production APIs requiring authentication
- Need multi-tenant authentication and authorization
- Require role-based access control (RBAC)
- Want consistent security across all endpoints
- Building microservices with shared authentication

❌ **DON'T USE when:**
- Building internal tools without authentication needs
- Using API keys or other authentication methods
- Single-tenant applications with simple auth needs
- Development/testing environments only

## Features Demonstrated

- **JWT Token Validation**: Support for HS256 and RS256 algorithms
- **Multi-tenant Authentication**: Tenant isolation and validation
- **Role-based Access Control (RBAC)**: Restrict access based on user roles
- **Scope-based Permissions**: Fine-grained access control using scopes
- **Optional Authentication**: Endpoints that work with or without authentication
- **Security Context Integration**: Access to authenticated user information

## Endpoints

### Public Endpoints (No Authentication Required)

- `GET /health` - Health check endpoint
- `GET /public` - Public information endpoint

### Protected Endpoints (JWT Required)

- `GET /api/profile` - User profile information
- `GET /api/users` - User management (requires `admin` or `manager` role)
- `GET /api/payments` - Payment data (requires `payments:read` scope)
- `GET /api/tenant/:id/data` - Tenant-specific data (validates tenant access)

### Optional Authentication Endpoints

- `GET /mixed/content` - Content that adapts based on authentication status

## Core Authentication Patterns

### 1. JWT Configuration (STANDARD Pattern)

**Purpose:** Configure secure JWT validation with multi-tenant support
**When to use:** All production APIs requiring authentication

```go
// CORRECT: Production-ready JWT configuration
jwtConfig := security.JWTConfig{
    SigningMethod:   "HS256",                    // PREFERRED: or "RS256" for production
    SecretKey:       "your-secret-key-here",     // REQUIRED: Load from AWS Secrets Manager
    Issuer:          "pay-theory",               // REQUIRED: Validate token issuer
    Audience:        []string{"lift-api"},       // REQUIRED: Validate intended audience
    MaxAge:          time.Hour,                  // REQUIRED: Short expiration for security
    RequireTenantID: true,                       // REQUIRED: Multi-tenant security
    ValidateTenant: func(tenantID string) error {
        // REQUIRED: Custom tenant validation
        return validateTenantAccess(tenantID)
    },
}

// INCORRECT: Insecure configuration
// jwtConfig := security.JWTConfig{
//     SecretKey: "weak-key",      // Weak secret - security risk
//     MaxAge:    24 * time.Hour,  // Too long - security risk
//     // Missing RequireTenantID - multi-tenant vulnerability
//     // Missing Issuer/Audience validation - token forgery risk
// }
```

### 2. Route Protection (PREFERRED Pattern)

**Purpose:** Apply consistent authentication across route groups
**When to use:** Protecting API endpoints with JWT

```go
// CORRECT: Group-based authentication
app := lift.New()

// Public routes (no authentication)
app.GET("/health", healthHandler)
app.GET("/public", publicHandler)

// Protected API routes
api := app.Group("/api")
api.Use(security.JWTAuth(jwtConfig))  // REQUIRED: Apply to all routes

// All routes under /api are now protected
api.GET("/profile", profileHandler)
api.GET("/users", usersHandler)
api.GET("/payments", paymentsHandler)

// INCORRECT: Route-by-route authentication
// app.GET("/api/profile", security.JWTAuth(jwtConfig), profileHandler)
// app.GET("/api/users", security.JWTAuth(jwtConfig), usersHandler)
// This is inconsistent and error-prone
```

### 3. Role-Based Access Control (STANDARD Pattern)

**Purpose:** Restrict access based on user roles
**When to use:** Endpoints requiring specific permissions

```go
// CORRECT: Role-based middleware
api.GET("/users", 
    security.RequireRoles("admin", "manager"),  // REQUIRED roles
    func(ctx *lift.Context) error {
        // Only admin or manager users can access
        return ctx.JSON(getAllUsers())
    })

api.GET("/payments", 
    security.RequireScopes("payments:read"),    // REQUIRED scope
    func(ctx *lift.Context) error {
        // Only users with payments:read scope can access
        return ctx.JSON(getPayments())
    })

// INCORRECT: Manual role checking in handlers
// api.GET("/users", func(ctx *lift.Context) error {
//     user := ctx.User()
//     if !hasRole(user, "admin") {  // Manual checking - error-prone
//         return ctx.JSON(403, "Forbidden")
//     }
//     return ctx.JSON(getAllUsers())
// })
```

### 4. Tenant Isolation (CRITICAL Pattern)

**Purpose:** Ensure users only access their tenant's data
**When to use:** All multi-tenant applications

```go
// CORRECT: Automatic tenant validation
api.GET("/tenant/:id/data", func(ctx *lift.Context) error {
    requestedTenant := ctx.Param("id")
    userTenant := ctx.TenantID()  // From JWT token
    
    // Lift automatically validates tenant access
    if requestedTenant != userTenant {
        return security.NewSecurityError("FORBIDDEN", "Cross-tenant access denied")
    }
    
    return ctx.JSON(getTenantData(userTenant))
})

// INCORRECT: Missing tenant validation
// api.GET("/tenant/:id/data", func(ctx *lift.Context) error {
//     tenantID := ctx.Param("id")
//     // No validation - users can access any tenant's data!
//     return ctx.JSON(getTenantData(tenantID))
// })
```

## JWT Token Format

The JWT tokens should include the following claims:

```json
{
  "sub": "user123",                    // User ID (required)
  "iss": "pay-theory",                 // Issuer (must match config)
  "aud": ["lift-api"],                 // Audience (must match config)
  "exp": 1640995200,                   // Expiration timestamp
  "iat": 1640991600,                   // Issued at timestamp
  "tenant_id": "tenant1",              // Tenant ID (required if RequireTenantID is true)
  "account_id": "account123",          // Account ID (optional)
  "roles": ["user", "manager"],        // User roles for RBAC
  "scopes": ["payments:read", "users:read"] // User scopes for permissions
}
```

## Testing with curl

### 1. Generate a Test JWT Token

You can use online tools like [jwt.io](https://jwt.io) to generate test tokens, or use the following Node.js script:

```javascript
const jwt = require('jsonwebtoken');

const payload = {
  sub: "user123",
  iss: "pay-theory",
  aud: ["lift-api"],
  exp: Math.floor(Date.now() / 1000) + (60 * 60), // 1 hour
  iat: Math.floor(Date.now() / 1000),
  tenant_id: "tenant1",
  account_id: "account123",
  roles: ["user", "manager"],
  scopes: ["payments:read", "users:read"]
};

const token = jwt.sign(payload, "your-secret-key-here");
console.log(token);
```

### 2. Test Public Endpoints

```bash
# Health check (no auth required)
curl http://localhost:8080/health

# Public endpoint (no auth required)
curl http://localhost:8080/public
```

### 3. Test Protected Endpoints

```bash
# Set your JWT token
TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."

# User profile (requires valid JWT)
curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/profile

# Users endpoint (requires admin or manager role)
curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/users

# Payments endpoint (requires payments:read scope)
curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/payments

# Tenant data (validates tenant access)
curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/tenant/tenant1/data
```

### 4. Test Optional Authentication

```bash
# Without token (anonymous access)
curl http://localhost:8080/mixed/content

# With token (authenticated access)
curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/mixed/content
```

## Error Responses

### Authentication Errors

```json
{
  "code": "UNAUTHORIZED",
  "message": "Missing or invalid authorization token",
  "status_code": 401
}
```

### Authorization Errors

```json
{
  "code": "FORBIDDEN", 
  "message": "Required roles: [admin, manager]",
  "status_code": 403
}
```

### Tenant Validation Errors

```json
{
  "code": "FORBIDDEN",
  "message": "Access denied for this tenant", 
  "status_code": 403
}
```

## Production Considerations

### 1. Secret Management

In production, load JWT secrets from AWS Secrets Manager:

```go
secretsProvider := security.NewSecretsManager("us-east-1")
secretKey, err := secretsProvider.GetSecret(ctx, "jwt-secret-key")
if err != nil {
    log.Fatal("Failed to load JWT secret:", err)
}

jwtConfig.SecretKey = secretKey
```

### 2. RSA Key Pairs

For RS256 algorithm, use RSA key pairs:

```go
jwtConfig := security.JWTConfig{
    SigningMethod:   "RS256",
    PublicKeyPath:   "/path/to/public.pem",
    PrivateKeyPath:  "/path/to/private.pem", // Only needed for token generation
    // ... other config
}
```

### 3. Token Caching

The JWT middleware includes built-in token validation caching for performance. Tokens are validated once and cached until expiration.

### 4. Multi-tenant Isolation

The example demonstrates strict tenant isolation:
- Each JWT must include a `tenant_id` claim
- Users can only access data for their own tenant
- Custom tenant validation logic can be implemented

### 5. Monitoring and Logging

All authentication events are automatically logged with structured data including:
- User ID and tenant ID
- Authentication method
- Request details
- Success/failure status

## What This Example Teaches

### ✅ Best Practices Demonstrated

1. **ALWAYS use route groups** for consistent authentication - Apply JWT middleware to groups, not individual routes
2. **ALWAYS validate tenant access** - Use `RequireTenantID: true` and custom validation
3. **ALWAYS use short token lifetimes** - 1 hour or less for security
4. **PREFER role-based middleware** over manual role checking - Use `RequireRoles()` and `RequireScopes()`
5. **ALWAYS load secrets from AWS Secrets Manager** - Never hardcode JWT secrets

### 🚫 Critical Anti-Patterns Avoided

1. **Manual authentication checking** - Inconsistent and error-prone
2. **Long token lifetimes** - Security vulnerability
3. **Missing tenant validation** - Cross-tenant data access
4. **Weak secret keys** - Token forgery risk
5. **Route-by-route auth** - Inconsistent protection

### 🔒 Security Requirements (MANDATORY)

1. **Strong Secrets**: Use cryptographically secure secret keys (256+ bits)
2. **HTTPS Only**: Never use HTTP in production environments
3. **Token Validation**: Always validate issuer, audience, and expiration
4. **Tenant Isolation**: Strict validation prevents cross-tenant access
5. **Audit Logging**: All auth events automatically logged by Lift
6. **Key Rotation**: Regular rotation of JWT signing keys

## Integration with Pay Theory Architecture

This JWT implementation is designed to work with Pay Theory's multi-tenant architecture:

- **Kernel Account**: Central authentication and user management
- **Partner Accounts**: Isolated tenant environments
- **Cross-account Communication**: Secure service-to-service authentication
- **Compliance**: PCI DSS and SOC 2 compliance patterns 