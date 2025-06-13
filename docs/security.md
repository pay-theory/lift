# Security

Security is paramount in serverless applications. Lift provides comprehensive security features and follows AWS best practices to help you build secure applications. This guide covers authentication, authorization, data protection, and security best practices.

## Overview

Lift's security features include:

- **JWT Authentication**: Built-in JWT validation and parsing
- **API Key Management**: Secure API key validation
- **Role-Based Access Control (RBAC)**: Fine-grained permissions
- **Data Encryption**: At-rest and in-transit encryption
- **Security Headers**: Automatic security headers
- **Input Validation**: Request validation and sanitization
- **Rate Limiting**: DDoS and abuse protection

## Authentication

### JWT Authentication

Lift provides robust JWT token handling:

```go
// Configure JWT middleware
app.Use(middleware.JWT(middleware.JWTConfig{
    // Secret key for HS256
    SecretKey: []byte(os.Getenv("JWT_SECRET")),
    
    // Or public key for RS256
    PublicKey: loadPublicKey(),
    
    // Token location
    TokenLookup: "header:Authorization",
    AuthScheme:  "Bearer",
    
    // Claims validation
    Claims: jwt.MapClaims{},
    
    // Custom validation
    ValidateFunc: func(claims jwt.MapClaims) error {
        // Check expiration
        exp, ok := claims["exp"].(float64)
        if !ok || time.Now().Unix() > int64(exp) {
            return errors.New("token expired")
        }
        
        // Check issuer
        iss, ok := claims["iss"].(string)
        if !ok || iss != "my-auth-service" {
            return errors.New("invalid issuer")
        }
        
        return nil
    },
    
    // Skip certain paths
    SkipPaths: []string{
        "/health",
        "/login",
        "/register",
        "/forgot-password",
    },
}))

// Access claims in handlers
func protectedHandler(ctx *lift.Context) error {
    claims := ctx.Get("claims").(jwt.MapClaims)
    userID := claims["sub"].(string)
    roles := claims["roles"].([]string)
    
    ctx.Logger.Info("Authenticated request", map[string]interface{}{
        "user_id": userID,
        "roles":   roles,
    })
    
    return ctx.JSON(protectedData)
}
```

### Custom Authentication

```go
// Custom auth middleware
func CustomAuth(authService AuthService) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Extract token
            token := ctx.Header("X-Auth-Token")
            if token == "" {
                return lift.Unauthorized("Authentication required")
            }
            
            // Validate token
            user, err := authService.ValidateToken(token)
            if err != nil {
                ctx.Logger.Warn("Invalid token", map[string]interface{}{
                    "error": err.Error(),
                    "token": token[:10] + "...", // Log partial token
                })
                return lift.Unauthorized("Invalid token")
            }
            
            // Set user context
            ctx.Set("user", user)
            ctx.SetUserID(user.ID)
            ctx.SetTenantID(user.TenantID)
            
            // Add auth info to logs
            ctx.Logger = ctx.Logger.With(map[string]interface{}{
                "user_id":   user.ID,
                "tenant_id": user.TenantID,
            })
            
            return next.Handle(ctx)
        })
    }
}
```

### API Key Authentication

```go
// API key middleware
func APIKeyAuth(store APIKeyStore) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Get API key from header
            apiKey := ctx.Header("X-API-Key")
            if apiKey == "" {
                // Try query parameter as fallback
                apiKey = ctx.Query("api_key")
            }
            
            if apiKey == "" {
                return lift.Unauthorized("API key required")
            }
            
            // Validate API key
            keyInfo, err := store.Validate(apiKey)
            if err != nil {
                return lift.Unauthorized("Invalid API key")
            }
            
            // Check if key is active
            if !keyInfo.Active {
                return lift.Forbidden("API key is inactive")
            }
            
            // Check rate limits for this key
            if keyInfo.RateLimitExceeded() {
                return lift.TooManyRequests("Rate limit exceeded")
            }
            
            // Set context
            ctx.Set("api_key", keyInfo)
            ctx.SetTenantID(keyInfo.TenantID)
            
            return next.Handle(ctx)
        })
    }
}
```

### Multi-Factor Authentication

```go
// MFA verification
func MFAMiddleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            user := ctx.Get("user").(*User)
            
            // Check if MFA is required
            if user.MFAEnabled && !user.MFAVerified {
                // Check for MFA code
                mfaCode := ctx.Header("X-MFA-Code")
                if mfaCode == "" {
                    return lift.Forbidden("MFA verification required")
                }
                
                // Verify MFA code
                if err := verifyMFACode(user, mfaCode); err != nil {
                    return lift.Forbidden("Invalid MFA code")
                }
                
                // Mark as verified for this session
                user.MFAVerified = true
                ctx.Set("user", user)
            }
            
            return next.Handle(ctx)
        })
    }
}
```

## Authorization

### Role-Based Access Control (RBAC)

```go
// RBAC middleware
func RequireRole(roles ...string) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            user := ctx.Get("user").(*User)
            
            // Check if user has required role
            hasRole := false
            for _, requiredRole := range roles {
                for _, userRole := range user.Roles {
                    if userRole == requiredRole {
                        hasRole = true
                        break
                    }
                }
            }
            
            if !hasRole {
                ctx.Logger.Warn("Access denied", map[string]interface{}{
                    "user_id":       user.ID,
                    "user_roles":    user.Roles,
                    "required_roles": roles,
                })
                return lift.Forbidden("Insufficient permissions")
            }
            
            return next.Handle(ctx)
        })
    }
}

// Use in routes
adminGroup := app.Group("/admin", RequireRole("admin"))
adminGroup.GET("/users", listUsers)
adminGroup.DELETE("/users/:id", deleteUser)
```

### Permission-Based Access Control

```go
// Fine-grained permissions
type Permission string

const (
    PermUserRead   Permission = "user:read"
    PermUserWrite  Permission = "user:write"
    PermUserDelete Permission = "user:delete"
    PermOrderRead  Permission = "order:read"
    PermOrderWrite Permission = "order:write"
)

func RequirePermission(perms ...Permission) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            user := ctx.Get("user").(*User)
            
            // Check permissions
            for _, required := range perms {
                if !user.HasPermission(required) {
                    return lift.Forbidden(
                        fmt.Sprintf("Missing permission: %s", required),
                    )
                }
            }
            
            return next.Handle(ctx)
        })
    }
}

// Resource-based permissions
func CanAccessResource(ctx *lift.Context, resource Resource) error {
    user := ctx.Get("user").(*User)
    
    // Owner can always access
    if resource.OwnerID == user.ID {
        return nil
    }
    
    // Check team membership
    if resource.TeamID != "" && user.IsInTeam(resource.TeamID) {
        return nil
    }
    
    // Check explicit permissions
    if user.HasResourcePermission(resource.ID, "read") {
        return nil
    }
    
    return lift.Forbidden("Cannot access resource")
}
```

### Attribute-Based Access Control (ABAC)

```go
// ABAC policy engine
type Policy struct {
    Resource   string
    Action     string
    Conditions []Condition
}

type Condition struct {
    Attribute string
    Operator  string
    Value     interface{}
}

func ABACMiddleware(policies []Policy) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            user := ctx.Get("user").(*User)
            resource := ctx.Request.Path
            action := ctx.Request.Method
            
            // Evaluate policies
            allowed := false
            for _, policy := range policies {
                if policy.Matches(resource, action) {
                    if policy.Evaluate(user, ctx) {
                        allowed = true
                        break
                    }
                }
            }
            
            if !allowed {
                return lift.Forbidden("Access denied by policy")
            }
            
            return next.Handle(ctx)
        })
    }
}
```

## Data Protection

### Encryption at Rest

```go
// Encrypt sensitive data before storage
type EncryptedField struct {
    cipher cipher.AEAD
}

func (e *EncryptedField) Encrypt(plaintext string) (string, error) {
    nonce := make([]byte, e.cipher.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return "", err
    }
    
    ciphertext := e.cipher.Seal(nonce, nonce, []byte(plaintext), nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (e *EncryptedField) Decrypt(ciphertext string) (string, error) {
    data, err := base64.StdEncoding.DecodeString(ciphertext)
    if err != nil {
        return "", err
    }
    
    nonceSize := e.cipher.NonceSize()
    nonce, ciphertext := data[:nonceSize], data[nonceSize:]
    
    plaintext, err := e.cipher.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return "", err
    }
    
    return string(plaintext), nil
}

// Use in models
type User struct {
    ID        string `json:"id"`
    Email     string `json:"email"`
    SSN       string `json:"-" dynamodb:"ssn,encrypted"`
    CreditCard string `json:"-" dynamodb:"credit_card,encrypted"`
}
```

### Encryption in Transit

```go
// Ensure TLS for external calls
func SecureHTTPClient() *http.Client {
    return &http.Client{
        Transport: &http.Transport{
            TLSClientConfig: &tls.Config{
                MinVersion: tls.VersionTLS12,
                CipherSuites: []uint16{
                    tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
                    tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
                    tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
                },
            },
        },
        Timeout: 30 * time.Second,
    }
}

// Certificate pinning
func PinnedHTTPClient(pins []string) *http.Client {
    return &http.Client{
        Transport: &http.Transport{
            TLSClientConfig: &tls.Config{
                VerifyPeerCertificate: func(certs [][]byte, _ [][]*x509.Certificate) error {
                    for _, cert := range certs {
                        hash := sha256.Sum256(cert)
                        pin := base64.StdEncoding.EncodeToString(hash[:])
                        
                        for _, validPin := range pins {
                            if pin == validPin {
                                return nil
                            }
                        }
                    }
                    return errors.New("certificate pin validation failed")
                },
            },
        },
    }
}
```

### Data Masking

```go
// Mask sensitive data in logs and responses
func MaskSensitiveData() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Intercept response
            err := next.Handle(ctx)
            
            // Mask sensitive fields in response
            if ctx.Response.Body != nil {
                masked := maskFields(ctx.Response.Body, []string{
                    "ssn",
                    "creditCard",
                    "password",
                    "apiKey",
                })
                ctx.Response.Body = masked
            }
            
            return err
        })
    }
}

func maskFields(data interface{}, fields []string) interface{} {
    // Implementation to recursively mask fields
    // Replace sensitive values with "***"
    return maskedData
}
```

## Security Headers

### Default Security Headers

```go
// Security headers middleware
app.Use(middleware.SecurityHeaders(middleware.SecurityConfig{
    // Prevent XSS
    XSSProtection: "1; mode=block",
    
    // Prevent MIME sniffing
    ContentTypeNosniff: "nosniff",
    
    // Prevent clickjacking
    XFrameOptions: "DENY",
    
    // Force HTTPS
    HSTSMaxAge:            31536000, // 1 year
    HSTSIncludeSubdomains: true,
    HSTSPreload:          true,
    
    // Content Security Policy
    ContentSecurityPolicy: "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'",
    
    // Referrer Policy
    ReferrerPolicy: "strict-origin-when-cross-origin",
    
    // Permissions Policy
    PermissionsPolicy: "geolocation=(), microphone=(), camera=()",
}))
```

### CORS Configuration

```go
// Secure CORS configuration
app.Use(middleware.CORS(middleware.CORSConfig{
    // Specific allowed origins (not *)
    AllowOrigins: []string{
        "https://app.example.com",
        "https://admin.example.com",
    },
    
    // Or dynamic validation
    AllowOriginFunc: func(origin string) bool {
        // Validate against whitelist
        return isAllowedOrigin(origin)
    },
    
    // Allowed methods
    AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
    
    // Allowed headers
    AllowHeaders: []string{
        "Authorization",
        "Content-Type",
        "X-CSRF-Token",
    },
    
    // Exposed headers
    ExposeHeaders: []string{
        "X-Request-ID",
        "X-Rate-Limit-Remaining",
    },
    
    // Allow credentials
    AllowCredentials: true,
    
    // Max age for preflight
    MaxAge: 86400,
}))
```

## Input Validation

### Request Validation

```go
// Comprehensive input validation
type CreateUserRequest struct {
    Email    string `json:"email" validate:"required,email,max=255"`
    Password string `json:"password" validate:"required,min=8,max=128,password"`
    Name     string `json:"name" validate:"required,min=2,max=100,alphaspace"`
    Age      int    `json:"age" validate:"min=13,max=120"`
    Phone    string `json:"phone" validate:"omitempty,e164"`
    Website  string `json:"website" validate:"omitempty,url"`
}

// Custom validation rules
func init() {
    validate.RegisterValidation("password", func(fl validator.FieldLevel) bool {
        password := fl.Field().String()
        
        // Check complexity requirements
        var (
            hasMinLen  = len(password) >= 8
            hasUpper   = regexp.MustCompile(`[A-Z]`).MatchString(password)
            hasLower   = regexp.MustCompile(`[a-z]`).MatchString(password)
            hasNumber  = regexp.MustCompile(`[0-9]`).MatchString(password)
            hasSpecial = regexp.MustCompile(`[!@#$%^&*]`).MatchString(password)
        )
        
        return hasMinLen && hasUpper && hasLower && hasNumber && hasSpecial
    })
    
    validate.RegisterValidation("alphaspace", func(fl validator.FieldLevel) bool {
        return regexp.MustCompile(`^[a-zA-Z\s]+$`).MatchString(fl.Field().String())
    })
}
```

### SQL Injection Prevention

```go
// Use parameterized queries
func getUser(db *sql.DB, userID string) (*User, error) {
    // GOOD: Parameterized query
    query := "SELECT id, email, name FROM users WHERE id = ?"
    row := db.QueryRow(query, userID)
    
    // NEVER: String concatenation
    // query := fmt.Sprintf("SELECT * FROM users WHERE id = '%s'", userID)
    
    var user User
    err := row.Scan(&user.ID, &user.Email, &user.Name)
    return &user, err
}

// DynamoDB with DynamORM (automatically safe)
func getUserDynamo(ctx *lift.Context, userID string) (*User, error) {
    db := dynamorm.FromContext(ctx)
    
    var user User
    err := db.Query(&user).
        Where("id = ?", userID). // Safe parameterization
        Execute()
    
    return &user, err
}
```

### XSS Prevention

```go
// HTML sanitization
func SanitizeHTML(input string) string {
    // Remove dangerous tags and attributes
    p := bluemonday.UGCPolicy()
    return p.Sanitize(input)
}

// Template rendering with auto-escaping
func renderTemplate(ctx *lift.Context, data interface{}) error {
    tmpl := template.Must(template.ParseFiles("template.html"))
    
    // Auto-escapes HTML
    var buf bytes.Buffer
    err := tmpl.Execute(&buf, data)
    if err != nil {
        return err
    }
    
    return ctx.HTML(buf.String())
}
```

## Rate Limiting

### API Rate Limiting

```go
// Configure rate limiting
app.Use(middleware.RateLimit(middleware.RateLimitConfig{
    // Window configuration
    WindowSize:  time.Minute,
    MaxRequests: 100,
    
    // Key function - rate limit by user
    KeyFunc: func(ctx *lift.Context) string {
        if userID := ctx.UserID(); userID != "" {
            return "user:" + userID
        }
        return "ip:" + ctx.ClientIP()
    },
    
    // Different limits for different endpoints
    EndpointLimits: map[string]int{
        "/api/v1/auth/login":    5,   // 5 per minute
        "/api/v1/auth/register": 3,   // 3 per minute
        "/api/v1/export":        10,  // 10 per minute
    },
    
    // Store (DynamoDB for distributed)
    Store: dynamostore.New(dynamostore.Config{
        TableName: "rate_limits",
        TTL:       5 * time.Minute,
    }),
    
    // Custom response
    ExceededHandler: func(ctx *lift.Context) error {
        return ctx.Status(429).JSON(map[string]interface{}{
            "error": "Too many requests",
            "retry_after": 60,
        })
    },
}))
```

### DDoS Protection

```go
// Advanced rate limiting for DDoS protection
func DDoSProtection() lift.Middleware {
    // Track request patterns
    patterns := NewPatternTracker()
    
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            clientIP := ctx.ClientIP()
            
            // Check for suspicious patterns
            if patterns.IsSuspicious(clientIP) {
                ctx.Logger.Warn("Suspicious activity detected", map[string]interface{}{
                    "ip": clientIP,
                    "pattern": patterns.GetPattern(clientIP),
                })
                return lift.TooManyRequests("Suspicious activity detected")
            }
            
            // Track request
            patterns.Track(clientIP, ctx.Request.Path)
            
            return next.Handle(ctx)
        })
    }
}
```

## Secrets Management

### Environment Variables

```go
// Secure configuration loading
type Config struct {
    JWTSecret     string `env:"JWT_SECRET,required"`
    DatabaseURL   string `env:"DATABASE_URL,required"`
    EncryptionKey string `env:"ENCRYPTION_KEY,required"`
    APIKeys       string `env:"API_KEYS"` // Comma-separated
}

func LoadConfig() (*Config, error) {
    config := &Config{}
    
    // Load from environment
    if err := env.Parse(config); err != nil {
        return nil, err
    }
    
    // Validate sensitive values
    if len(config.JWTSecret) < 32 {
        return nil, errors.New("JWT secret too short")
    }
    
    if len(config.EncryptionKey) != 32 {
        return nil, errors.New("encryption key must be 32 bytes")
    }
    
    return config, nil
}
```

### AWS Secrets Manager

```go
// Retrieve secrets from AWS Secrets Manager
func GetSecret(secretName string) (map[string]string, error) {
    sess := session.Must(session.NewSession())
    svc := secretsmanager.New(sess)
    
    input := &secretsmanager.GetSecretValueInput{
        SecretId: aws.String(secretName),
    }
    
    result, err := svc.GetSecretValue(input)
    if err != nil {
        return nil, err
    }
    
    var secrets map[string]string
    if err := json.Unmarshal([]byte(*result.SecretString), &secrets); err != nil {
        return nil, err
    }
    
    return secrets, nil
}

// Use in application initialization
func InitializeSecrets() error {
    secrets, err := GetSecret("myapp/production/secrets")
    if err != nil {
        return err
    }
    
    // Set as environment variables
    for key, value := range secrets {
        os.Setenv(key, value)
    }
    
    return nil
}
```

## Security Monitoring

### Audit Logging

```go
// Audit trail middleware
func AuditLog(store AuditStore) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            start := time.Now()
            
            // Capture request details
            audit := AuditEntry{
                ID:        uuid.New().String(),
                Timestamp: start,
                UserID:    ctx.UserID(),
                TenantID:  ctx.TenantID(),
                IP:        ctx.ClientIP(),
                Method:    ctx.Request.Method,
                Path:      ctx.Request.Path,
                Headers:   sanitizeHeaders(ctx.Request.Headers),
            }
            
            // Execute handler
            err := next.Handle(ctx)
            
            // Complete audit entry
            audit.Duration = time.Since(start)
            audit.StatusCode = ctx.Response.StatusCode
            if err != nil {
                audit.Error = err.Error()
            }
            
            // Store audit entry
            if err := store.Save(audit); err != nil {
                ctx.Logger.Error("Failed to save audit log", map[string]interface{}{
                    "error": err.Error(),
                })
            }
            
            return err
        })
    }
}
```

### Security Alerts

```go
// Security event monitoring
func SecurityMonitor() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Monitor for security events
            defer func() {
                checkSecurityEvents(ctx)
            }()
            
            return next.Handle(ctx)
        })
    }
}

func checkSecurityEvents(ctx *lift.Context) {
    // Multiple failed auth attempts
    if failures := getAuthFailures(ctx.ClientIP()); failures > 5 {
        alert := SecurityAlert{
            Type:     "MULTIPLE_AUTH_FAILURES",
            Severity: "HIGH",
            IP:       ctx.ClientIP(),
            Details: map[string]interface{}{
                "failures": failures,
                "user_agent": ctx.Header("User-Agent"),
            },
        }
        sendSecurityAlert(alert)
    }
    
    // Suspicious patterns
    if isSQLInjectionAttempt(ctx.Request) {
        alert := SecurityAlert{
            Type:     "SQL_INJECTION_ATTEMPT",
            Severity: "CRITICAL",
            IP:       ctx.ClientIP(),
            Path:     ctx.Request.Path,
            Payload:  sanitizeForLogging(ctx.Request),
        }
        sendSecurityAlert(alert)
        
        // Block IP
        blockIP(ctx.ClientIP())
    }
}
```

## Security Best Practices

### 1. Principle of Least Privilege

```go
// GOOD: Minimal permissions
func getUserHandler(ctx *lift.Context) error {
    userID := ctx.Param("id")
    requestingUser := ctx.UserID()
    
    // Users can only access their own data
    if userID != requestingUser && !ctx.HasRole("admin") {
        return lift.Forbidden("Cannot access other users' data")
    }
    
    // Return limited fields for non-admin
    if !ctx.HasRole("admin") {
        return ctx.JSON(getPublicUserData(userID))
    }
    
    return ctx.JSON(getFullUserData(userID))
}
```

### 2. Defense in Depth

```go
// Multiple layers of security
app.Use(middleware.RateLimit())      // Layer 1: Rate limiting
app.Use(middleware.SecurityHeaders()) // Layer 2: Security headers
app.Use(middleware.JWT())            // Layer 3: Authentication
app.Use(middleware.Audit())          // Layer 4: Audit logging

// Additional validation in handler
func handler(ctx *lift.Context) error {
    // Layer 5: Input validation
    if err := validateInput(ctx); err != nil {
        return err
    }
    
    // Layer 6: Authorization
    if err := checkPermissions(ctx); err != nil {
        return err
    }
    
    // Layer 7: Business logic validation
    if err := validateBusinessRules(ctx); err != nil {
        return err
    }
    
    return ctx.JSON(response)
}
```

### 3. Secure Defaults

```go
// GOOD: Secure by default
type SecurityConfig struct {
    EnableTLS      bool   `default:"true"`
    MinTLSVersion  string `default:"1.2"`
    SessionTimeout int    `default:"3600"` // 1 hour
    MaxLoginAttempts int  `default:"5"`
    PasswordMinLength int `default:"12"`
}

// Require explicit opt-out for security features
if !config.DisableSecurityHeaders {
    app.Use(middleware.SecurityHeaders())
}
```

### 4. Regular Security Updates

```go
// Dependency scanning in CI/CD
// go.mod
module myapp

require (
    github.com/pay-theory/lift v1.0.0
    // Keep dependencies updated
)

// Security scanning script
#!/bin/bash
go list -json -m all | nancy sleuth
gosec ./...
```

### 5. Security Testing

```go
// Security-focused tests
func TestSQLInjection(t *testing.T) {
    payloads := []string{
        "1' OR '1'='1",
        "1; DROP TABLE users;--",
        "1' UNION SELECT * FROM users--",
    }
    
    for _, payload := range payloads {
        resp := app.TestRequest("GET", "/users?id="+payload, nil)
        assert.NotEqual(t, 200, resp.StatusCode, 
            "SQL injection payload should be rejected: %s", payload)
    }
}

func TestXSS(t *testing.T) {
    payload := map[string]string{
        "name": "<script>alert('XSS')</script>",
    }
    
    resp := app.TestRequest("POST", "/users", payload)
    assert.NotContains(t, resp.Body, "<script>",
        "XSS payload should be sanitized")
}
```

## Summary

Lift's security features provide:

- **Authentication**: JWT, API keys, MFA support
- **Authorization**: RBAC, ABAC, fine-grained permissions
- **Data Protection**: Encryption, masking, secure transmission
- **Input Validation**: Comprehensive validation and sanitization
- **Security Headers**: Automatic security headers and CORS
- **Monitoring**: Audit logging and security alerts

Security is a shared responsibility - use these features along with AWS security best practices to build secure serverless applications. 