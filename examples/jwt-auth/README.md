# JWT Authentication Example

This example demonstrates how to use JWT authentication with the Lift framework, including multi-tenant support, role-based access control, and scope-based permissions.

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

## JWT Configuration

The example uses the following JWT configuration:

```go
jwtConfig := security.JWTConfig{
    SigningMethod:   "HS256",                    // or "RS256" for RSA
    SecretKey:       "your-secret-key-here",     // For HS256
    Issuer:          "pay-theory",
    Audience:        []string{"lift-api"},
    MaxAge:          time.Hour,
    RequireTenantID: true,
    ValidateTenant: func(tenantID string) error {
        // Custom tenant validation logic
        validTenants := []string{"tenant1", "tenant2", "premium-tenant"}
        for _, valid := range validTenants {
            if tenantID == valid {
                return nil
            }
        }
        return security.NewSecurityError("INVALID_TENANT", "Tenant not found")
    },
}
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

## Security Best Practices

1. **Use Strong Secrets**: Generate cryptographically secure secret keys
2. **Short Token Lifetimes**: Use short expiration times (1 hour or less)
3. **Validate All Claims**: Always validate issuer, audience, and expiration
4. **Tenant Isolation**: Implement strict tenant validation
5. **Role-based Access**: Use the principle of least privilege
6. **Audit Logging**: Log all authentication and authorization events
7. **HTTPS Only**: Always use HTTPS in production
8. **Token Rotation**: Implement regular key rotation

## Integration with Pay Theory Architecture

This JWT implementation is designed to work with Pay Theory's multi-tenant architecture:

- **Kernel Account**: Central authentication and user management
- **Partner Accounts**: Isolated tenant environments
- **Cross-account Communication**: Secure service-to-service authentication
- **Compliance**: PCI DSS and SOC 2 compliance patterns 