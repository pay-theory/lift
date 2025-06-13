# Lift Framework Security Architecture
*Version: 1.0*
*Classification: Internal*

## Overview

This document outlines the comprehensive security architecture for the Lift framework, ensuring protection of sensitive data, compliance with regulations, and secure multi-tenant operations across Pay Theory's infrastructure.

## Security Principles

1. **Defense in Depth**: Multiple layers of security controls
2. **Zero Trust**: Never trust, always verify
3. **Least Privilege**: Minimal permissions required
4. **Encryption Everywhere**: Data encrypted at rest and in transit
5. **Audit Everything**: Comprehensive logging and monitoring

## Architecture Layers

### 1. Authentication Layer

#### JWT Authentication
```go
// pkg/security/jwt.go
type JWTConfig struct {
    // Signing
    SigningMethod   string // RS256, HS256
    PublicKeyPath   string
    PrivateKeyPath  string
    
    // Validation
    Issuer          string
    Audience        []string
    MaxAge          time.Duration
    
    // Multi-tenant
    RequireTenantID bool
    ValidateTenant  func(tenantID string) error
    
    // Rotation
    KeyRotation     bool
    RotationPeriod  time.Duration
}

type Claims struct {
    jwt.StandardClaims
    UserID    string   `json:"user_id"`
    TenantID  string   `json:"tenant_id"`
    AccountID string   `json:"account_id"` // Partner or Kernel
    Roles     []string `json:"roles"`
    Scopes    []string `json:"scopes"`
}
```

#### API Key Authentication
```go
// pkg/security/apikey.go
type APIKeyConfig struct {
    // Storage
    Provider        string // "secrets-manager", "parameter-store"
    KeyPrefix       string
    
    // Validation
    MinLength       int
    RequireRotation bool
    MaxAge          time.Duration
    
    // Rate Limiting
    RateLimit       int
    RatePeriod      time.Duration
}

type APIKey struct {
    ID          string
    Key         string // Hashed
    TenantID    string
    Scopes      []string
    CreatedAt   time.Time
    LastUsedAt  time.Time
    ExpiresAt   time.Time
}
```

#### Multi-Factor Authentication
```go
// pkg/security/mfa.go
type MFAConfig struct {
    Required        bool
    Methods         []string // "totp", "sms", "email"
    GracePeriod     time.Duration
    BackupCodes     int
}
```

### 2. Authorization Layer

#### Role-Based Access Control (RBAC)
```go
// pkg/security/rbac.go
type RBACConfig struct {
    // Roles
    DefaultRoles    []string
    RoleHierarchy   map[string][]string // role -> inherited roles
    
    // Permissions
    Permissions     map[string][]string // role -> permissions
    
    // Dynamic
    DynamicRoles    bool
    RoleProvider    RoleProvider
}

type Permission struct {
    Resource    string // "users", "payments", "accounts"
    Action      string // "read", "write", "delete"
    Conditions  map[string]interface{} // Dynamic conditions
}

// Middleware
func RequirePermission(resource, action string) Middleware {
    return func(next Handler) Handler {
        return HandlerFunc(func(ctx *Context) error {
            if !ctx.HasPermission(resource, action) {
                return Forbidden("Insufficient permissions")
            }
            return next.Handle(ctx)
        })
    }
}
```

#### Attribute-Based Access Control (ABAC)
```go
// pkg/security/abac.go
type Policy struct {
    ID          string
    Effect      string // "allow", "deny"
    Principal   PolicyPrincipal
    Action      []string
    Resource    []string
    Conditions  []PolicyCondition
}

type PolicyCondition struct {
    Key         string
    Operator    string // "equals", "contains", "greater_than"
    Value       interface{}
}
```

### 3. Encryption Layer

#### Data Encryption
```go
// pkg/security/encryption.go
type EncryptionConfig struct {
    // At Rest
    KMSKeyID        string
    Algorithm       string // "AES-256-GCM"
    
    // In Transit
    TLSVersion      string // "1.3"
    CipherSuites    []string
    
    // Field Level
    FieldEncryption bool
    SensitiveFields []string // PII fields
}

// Field-level encryption for PII
func EncryptField(value string, keyID string) (string, error) {
    // Use AWS KMS for key management
    // Return base64 encoded encrypted value
}

func DecryptField(encrypted string, keyID string) (string, error) {
    // Decrypt using KMS
    // Audit access
}
```

#### Secrets Management
```go
// pkg/security/secrets.go
type SecretsProvider interface {
    GetSecret(ctx context.Context, name string) (string, error)
    PutSecret(ctx context.Context, name string, value string) error
    RotateSecret(ctx context.Context, name string) error
    DeleteSecret(ctx context.Context, name string) error
}

type AWSSecretsManager struct {
    client      *secretsmanager.Client
    cache       *SecretCache
    keyPrefix   string
}

type SecretCache struct {
    secrets     map[string]*CachedSecret
    mu          sync.RWMutex
    ttl         time.Duration
}
```

### 4. Network Security

#### Request Validation
```go
// pkg/security/validation.go
type RequestValidator struct {
    // Size Limits
    MaxBodySize     int64
    MaxHeaderSize   int
    
    // Content
    AllowedMethods  []string
    AllowedHeaders  []string
    
    // Rate Limiting
    RateLimiter     RateLimiter
    
    // IP Filtering
    AllowedIPs      []net.IPNet
    DeniedIPs       []net.IPNet
}

// Input sanitization
func SanitizeInput(input string) string {
    // Remove potentially dangerous characters
    // Prevent injection attacks
}
```

#### Cross-Origin Resource Sharing (CORS)
```go
// pkg/security/cors.go
type CORSConfig struct {
    AllowedOrigins      []string
    AllowedMethods      []string
    AllowedHeaders      []string
    ExposedHeaders      []string
    AllowCredentials    bool
    MaxAge              int
    
    // Dynamic origin validation
    ValidateOrigin      func(origin string) bool
}
```

### 5. Multi-Tenant Security

#### Tenant Isolation
```go
// pkg/security/tenant.go
type TenantConfig struct {
    // Isolation
    IsolationLevel  string // "strict", "shared"
    
    // Data
    DataPartitioning bool
    PartitionKey     string
    
    // Resources
    ResourceQuotas   map[string]int
    
    // Cross-tenant
    AllowCrossTenant bool
    TrustedTenants   []string
}

// Middleware for tenant isolation
func TenantIsolation() Middleware {
    return func(next Handler) Handler {
        return HandlerFunc(func(ctx *Context) error {
            tenantID := ctx.TenantID()
            if tenantID == "" {
                return Unauthorized("Tenant ID required")
            }
            
            // Set tenant context for all operations
            ctx.SetTenantContext(tenantID)
            
            return next.Handle(ctx)
        })
    }
}
```

#### Cross-Account Security
```go
// pkg/security/crossaccount.go
type CrossAccountConfig struct {
    // Kernel Account
    KernelAccountID     string
    KernelRoleARN       string
    
    // Partner Accounts
    PartnerAccounts     map[string]PartnerAccount
    
    // Trust Policy
    TrustPolicy         string
    ExternalID          string
    
    // Session
    SessionDuration     time.Duration
}

type PartnerAccount struct {
    AccountID       string
    RoleARN         string
    Region          string
    Environment     string // dev, staging, prod
}

// Assume role for cross-account access
func AssumePartnerRole(accountID string) (*sts.Credentials, error) {
    // Validate account is trusted
    // Assume role with external ID
    // Return temporary credentials
}
```

### 6. Audit & Compliance

#### Audit Logging
```go
// pkg/security/audit.go
type AuditLogger struct {
    // Destination
    LogGroup        string
    StreamPrefix    string
    
    // Content
    IncludePII      bool
    HashPII         bool
    
    // Retention
    RetentionDays   int
}

type AuditEvent struct {
    Timestamp       time.Time
    EventType       string
    UserID          string
    TenantID        string
    AccountID       string
    Resource        string
    Action          string
    Result          string // "success", "failure"
    Details         map[string]interface{}
    IPAddress       string
    UserAgent       string
    RequestID       string
}

func (a *AuditLogger) LogSecurityEvent(event AuditEvent) {
    // Log to CloudWatch
    // Send to SIEM if configured
    // Alert on suspicious patterns
}
```

#### Compliance Controls
```go
// pkg/security/compliance.go
type ComplianceConfig struct {
    // Standards
    PCIDSSEnabled   bool
    SOC2Enabled     bool
    HIPAAEnabled    bool
    
    // Data Residency
    DataRegions     []string
    
    // Privacy
    GDPREnabled     bool
    CCPAEnabled     bool
    
    // Retention
    DataRetention   map[string]time.Duration
}

// Compliance validation middleware
func ComplianceValidation(standards []string) Middleware {
    return func(next Handler) Handler {
        return HandlerFunc(func(ctx *Context) error {
            // Validate request meets compliance requirements
            // Check data residency
            // Enforce retention policies
            return next.Handle(ctx)
        })
    }
}
```

### 7. Security Monitoring

#### Threat Detection
```go
// pkg/security/threatdetection.go
type ThreatDetector struct {
    // Patterns
    SuspiciousPatterns  []Pattern
    
    // Thresholds
    FailedLoginLimit    int
    RateLimitExceeded   int
    
    // Actions
    BlockDuration       time.Duration
    AlertChannels       []string
}

type Pattern struct {
    Name        string
    Regex       string
    Severity    string // "low", "medium", "high", "critical"
    Action      string // "log", "block", "alert"
}

// Real-time threat detection
func (t *ThreatDetector) Analyze(ctx *Context) ThreatLevel {
    // Check for SQL injection
    // Check for XSS attempts
    // Check for unusual patterns
    // Return threat level
}
```

#### Security Metrics
```go
// pkg/security/metrics.go
type SecurityMetrics struct {
    // Authentication
    LoginAttempts       Counter
    FailedLogins        Counter
    TokensIssued        Counter
    TokensRevoked       Counter
    
    // Authorization
    PermissionDenied    Counter
    
    // Threats
    ThreatDetected      Counter
    RequestsBlocked     Counter
    
    // Compliance
    ComplianceViolations Counter
}
```

## Implementation Guidelines

### 1. Secure Defaults
- All communications encrypted (TLS 1.3)
- Authentication required by default
- Minimal permissions granted
- Audit logging enabled
- Input validation enforced

### 2. Security Headers
```go
func SecureHeaders() Middleware {
    return func(next Handler) Handler {
        return HandlerFunc(func(ctx *Context) error {
            headers := ctx.Response.Headers
            headers["X-Content-Type-Options"] = "nosniff"
            headers["X-Frame-Options"] = "DENY"
            headers["X-XSS-Protection"] = "1; mode=block"
            headers["Strict-Transport-Security"] = "max-age=31536000; includeSubDomains"
            headers["Content-Security-Policy"] = "default-src 'self'"
            return next.Handle(ctx)
        })
    }
}
```

### 3. Error Handling
- Never expose internal errors
- Log detailed errors internally
- Return generic messages to clients
- Include request IDs for tracking

### 4. Testing Requirements
- Security unit tests for all components
- Integration tests for auth flows
- Penetration testing before release
- Regular security audits

## Deployment Security

### 1. Lambda Security
```yaml
# Minimal IAM role for Lambda
Version: '2012-10-17'
Statement:
  - Effect: Allow
    Principal:
      Service: lambda.amazonaws.com
    Action: sts:AssumeRole
  - Effect: Allow
    Action:
      - logs:CreateLogGroup
      - logs:CreateLogStream
      - logs:PutLogEvents
    Resource: !Sub 'arn:aws:logs:${AWS::Region}:${AWS::AccountId}:*'
  - Effect: Allow
    Action:
      - kms:Decrypt
      - kms:GenerateDataKey
    Resource: !GetAtt KMSKey.Arn
```

### 2. VPC Configuration
- Deploy in private subnets
- Use VPC endpoints for AWS services
- Enable flow logs
- Configure NACLs and security groups

### 3. Secrets Rotation
- Automatic rotation every 90 days
- Zero-downtime rotation
- Audit trail for all access
- Encrypted storage

## Incident Response

### 1. Security Incidents
1. Detect: Automated monitoring
2. Contain: Automatic blocking
3. Investigate: Audit logs
4. Remediate: Fix vulnerabilities
5. Review: Post-mortem

### 2. Breach Protocol
1. Immediate notification
2. Preserve evidence
3. Assess impact
4. Notify affected parties
5. Implement fixes

## Compliance Matrix

| Requirement | PCI-DSS | SOC2 | HIPAA | Implementation |
|-------------|---------|------|--------|----------------|
| Encryption at Rest | ✓ | ✓ | ✓ | KMS encryption |
| Encryption in Transit | ✓ | ✓ | ✓ | TLS 1.3 |
| Access Control | ✓ | ✓ | ✓ | RBAC + ABAC |
| Audit Logging | ✓ | ✓ | ✓ | CloudWatch Logs |
| Data Retention | ✓ | ✓ | ✓ | Lifecycle policies |
| Vulnerability Scanning | ✓ | ✓ | ✓ | Weekly scans |

## Security Checklist

### Development Phase
- [ ] Code review for security issues
- [ ] Static analysis (SAST)
- [ ] Dependency scanning
- [ ] Secret scanning
- [ ] Unit tests for security controls

### Pre-Production
- [ ] Dynamic analysis (DAST)
- [ ] Penetration testing
- [ ] Security audit
- [ ] Compliance validation
- [ ] Performance impact assessment

### Production
- [ ] Real-time monitoring
- [ ] Regular audits
- [ ] Incident response drills
- [ ] Security updates
- [ ] Compliance reporting

## Conclusion

The Lift framework's security architecture provides comprehensive protection through multiple layers of security controls. By following these guidelines and implementing all security features, we ensure the protection of sensitive data and maintain compliance with industry standards. 