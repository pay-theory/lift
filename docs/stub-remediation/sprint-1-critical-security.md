# Sprint 1: Critical Security Fixes
**Duration**: 1 week  
**Priority**: CRITICAL  
**Goal**: Eliminate production crashes and security vulnerabilities

## Overview
This sprint focuses on replacing panic statements with proper error handling and removing debug print statements that expose sensitive information. All implementations must follow Lift's error handling patterns and use structured logging.

## Task 1: Replace Panic Statements in Security Modules

### Files to Fix:
- `pkg/security/jwt.go`
- `pkg/security/auth.go`
- `pkg/security/dataprotection.go`
- `pkg/middleware/auth.go`

### Implementation Pattern:
```go
// ❌ WRONG - Current pattern
func ValidateToken(token string) {
    if token == "" {
        panic("invalid token")
    }
}

// ✅ CORRECT - Lift pattern
func ValidateToken(token string) error {
    if token == "" {
        return lift.NewError(http.StatusUnauthorized, "Invalid token", map[string]interface{}{
            "error": "token_required",
            "detail": "Authorization token is missing or empty",
        })
    }
    return nil
}
```

### Lift Error Conventions:
- Use `lift.NewError()` for structured errors
- Include HTTP status code
- Provide user-friendly message
- Add detailed error context in metadata
- Never expose internal implementation details

## Task 2: Replace Panic in CDK Constructs

### Files to Fix:
- `pkg/cdk/constructs/*.go` (all construct files)
- `pkg/cdk/patterns/*.go` (all pattern files)

### Implementation Pattern:
```go
// ❌ WRONG - Current pattern
func NewDynamORMTable(scope constructs.Construct, id *string, props *DynamORMTableProps) *DynamORMTable {
    if props.TableName == nil {
        panic("TableName is required")
    }
}

// ✅ CORRECT - CDK pattern with validation
func NewDynamORMTable(scope constructs.Construct, id *string, props *DynamORMTableProps) *DynamORMTable {
    // Validate required properties
    if err := validateDynamORMTableProps(props); err != nil {
        // For CDK constructs, use jsii error handling
        panic(fmt.Sprintf("DynamORMTable validation failed: %v", err))
    }
    
    // Set defaults for optional properties
    props = applyDynamORMTableDefaults(props)
    
    // Continue with construction...
}

// Separate validation function
func validateDynamORMTableProps(props *DynamORMTableProps) error {
    if props.TableName == nil || *props.TableName == "" {
        return fmt.Errorf("TableName is required and cannot be empty")
    }
    if props.PartitionKey == nil || *props.PartitionKey == "" {
        return fmt.Errorf("PartitionKey is required for DynamORM tables")
    }
    return nil
}
```

### CDK Error Conventions:
- Validate all required properties early
- Use separate validation functions
- Apply sensible defaults after validation
- For CDK constructs, panic is acceptable ONLY during construction with clear error messages
- Never panic after construction is complete

## Task 3: Remove Debug Print Statements

### Files to Fix:
- All files in `pkg/security/`
- All files in `pkg/middleware/`
- All deployment scripts in `scripts/`

### Implementation Pattern:
```go
// ❌ WRONG - Current pattern
func AuthenticateUser(token string) (*User, error) {
    fmt.Printf("JWT token: %s", token)
    log.Println("Authenticating user with token:", token)
    // ... authentication logic
}

// ✅ CORRECT - Structured logging pattern
func AuthenticateUser(token string) (*User, error) {
    // Use context logger with sanitized information
    if ctx.Logger != nil {
        ctx.Logger.Debug("User authentication attempt", map[string]any{
            "token_type": "JWT",
            "token_length": len(token),
            "timestamp": time.Now().Unix(),
            // Never log the actual token value
        })
    }
    
    // ... authentication logic
    
    if err != nil {
        ctx.Logger.Error("Authentication failed", map[string]any{
            "error": err.Error(),
            "token_type": "JWT",
        })
        return nil, err
    }
    
    return user, nil
}
```

### Logging Conventions:
- Use structured logging via `ctx.Logger`
- Never log sensitive data (tokens, passwords, keys)
- Log metadata about operations, not values
- Use appropriate log levels (Debug, Info, Warn, Error)
- Include contextual information for debugging

## Task 4: Fix Event Handler Panics

### Files to Fix:
- `pkg/adapters/eventbridge.go`
- `pkg/adapters/sqs.go`
- `pkg/adapters/s3.go`
- `pkg/adapters/dynamodb.go`

### Implementation Pattern:
```go
// ❌ WRONG - Current pattern
func HandleEvent(event EventBridgeEvent) {
    if event.Detail == nil {
        panic("event detail is required")
    }
}

// ✅ CORRECT - Proper error handling
func HandleEvent(ctx *lift.Context, event EventBridgeEvent) error {
    // Validate event
    if err := validateEventBridgeEvent(event); err != nil {
        return lift.NewError(http.StatusBadRequest, "Invalid event", map[string]interface{}{
            "error": "invalid_event",
            "detail": err.Error(),
            "source": event.Source,
            "event_id": event.ID,
        })
    }
    
    // Process event with proper error handling
    if err := processEvent(ctx, event); err != nil {
        // Log error but return structured response
        ctx.Logger.Error("Event processing failed", map[string]any{
            "event_id": event.ID,
            "event_type": event.DetailType,
            "error": err.Error(),
        })
        
        // For async events, consider retry strategy
        if shouldRetry(err) {
            return lift.NewRetriableError(err)
        }
        
        return err
    }
    
    return nil
}
```

### Event Handler Conventions:
- Always validate events before processing
- Return errors, never panic
- Use structured error responses
- Consider retry strategies for transient failures
- Log sufficient context for debugging
- Use DLQ for permanent failures

## Task 5: Security Module Error Handling

### Special Considerations for Security:
```go
// Data Protection Module Pattern
func EncryptData(ctx *lift.Context, data []byte) ([]byte, error) {
    // Get encryption key from secure storage
    key, err := getEncryptionKey(ctx)
    if err != nil {
        // Don't expose key retrieval errors
        return nil, lift.NewError(http.StatusInternalServerError, 
            "Encryption service unavailable", nil)
    }
    
    // Perform encryption
    encrypted, err := performEncryption(data, key)
    if err != nil {
        // Log internal error but return generic message
        ctx.Logger.Error("Encryption failed", map[string]any{
            "data_size": len(data),
            "error": err.Error(),
        })
        return nil, lift.NewError(http.StatusInternalServerError,
            "Failed to encrypt data", nil)
    }
    
    return encrypted, nil
}
```

## Testing Requirements

### Unit Tests:
```go
func TestErrorHandling(t *testing.T) {
    // Test missing token
    err := ValidateToken("")
    assert.NotNil(t, err)
    assert.Contains(t, err.Error(), "token_required")
    
    // Ensure no panic
    assert.NotPanics(t, func() {
        _ = ValidateToken("")
    })
}
```

### Integration Tests:
- Verify error responses match API contracts
- Ensure no sensitive data in error messages
- Test error propagation through middleware chain
- Validate structured logging output

## Acceptance Criteria

1. **Zero panic statements** in production code paths
2. **No debug print statements** with sensitive data
3. **All errors use structured format** with appropriate HTTP codes
4. **Logging follows security best practices**
5. **All tests pass** including new error handling tests
6. **No performance degradation** from additional error handling

## Rollout Strategy

1. **Day 1-2**: Fix security modules (highest risk)
2. **Day 3-4**: Fix CDK constructs and event handlers
3. **Day 5**: Remove all debug prints and add structured logging
4. **Day 6-7**: Testing, validation, and deployment

## Post-Implementation Checklist

- [ ] Run security scanner to verify no sensitive data in logs
- [ ] Perform load testing to ensure no performance impact
- [ ] Update error handling documentation
- [ ] Add linting rules to prevent future panics
- [ ] Create security logging guidelines
- [ ] Set up alerts for any remaining panic occurrences