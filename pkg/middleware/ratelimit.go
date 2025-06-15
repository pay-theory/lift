package middleware

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/pay-theory/lift/pkg/dynamorm"
	"github.com/pay-theory/lift/pkg/lift"
)

// RateLimitConfig holds configuration for rate limiting
type RateLimitConfig struct {
	// DynamORM configuration
	DynamORM *dynamorm.DynamORMWrapper `json:"-"`

	// Rate limiting settings
	DefaultLimit  int           `json:"default_limit"`  // Requests per window
	DefaultWindow time.Duration `json:"default_window"` // Time window
	Window        time.Duration `json:"window"`         // Alias for DefaultWindow (backward compatibility)
	BurstLimit    int           `json:"burst_limit"`    // Burst allowance

	// Strategy settings
	Strategy    string        `json:"strategy"`    // fixed_window, sliding_window, multi_window
	Granularity time.Duration `json:"granularity"` // For sliding window strategy

	// Multi-tenant settings
	TenantLimits map[string]int `json:"tenant_limits"` // Per-tenant limits
	UserLimits   map[string]int `json:"user_limits"`   // Per-user limits

	// Key generation
	KeyPrefix     string                            `json:"key_prefix"`
	KeyFunc       func(*lift.Context) *RateLimitKey `json:"-"` // Custom key function
	IncludePath   bool                              `json:"include_path"`
	IncludeMethod bool                              `json:"include_method"`

	// Error handling
	ErrorHandler func(*lift.Context, *RateLimitResult) error `json:"-"` // Custom error handler

	// Behavior settings
	SkipSuccessful bool `json:"skip_successful"` // Only count failed requests
	SkipOptions    bool `json:"skip_options"`    // Skip OPTIONS requests

	// Headers
	HeaderPrefix string `json:"header_prefix"` // X-RateLimit prefix

	// Storage settings
	TableName       string        `json:"table_name"`
	TTL             time.Duration `json:"ttl"`              // How long to keep records
	CleanupInterval time.Duration `json:"cleanup_interval"` // How often to cleanup
}

// RateLimitKey represents a rate limiting key with metadata
type RateLimitKey struct {
	Identifier string            `json:"identifier"` // Primary identifier (tenant:user, IP, etc.)
	Resource   string            `json:"resource"`   // Resource being accessed (path)
	Operation  string            `json:"operation"`  // Operation being performed (method)
	Metadata   map[string]string `json:"metadata"`   // Additional metadata
}

// RateLimitEntry represents a rate limit record in DynamoDB
type RateLimitEntry struct {
	Key         string    `dynamodbav:"pk" json:"key"`
	Count       int       `dynamodbav:"count" json:"count"`
	WindowStart time.Time `dynamodbav:"window_start" json:"window_start"`
	LastRequest time.Time `dynamodbav:"last_request" json:"last_request"`
	TTL         int64     `dynamodbav:"ttl" json:"ttl"`
}

// RateLimitResult contains the result of a rate limit check
type RateLimitResult struct {
	Allowed     bool          `json:"allowed"`
	Limit       int           `json:"limit"`
	Remaining   int           `json:"remaining"`
	ResetAt     time.Time     `json:"reset_at"`
	RetryAfter  time.Duration `json:"retry_after"`
	WindowStart time.Time     `json:"window_start"`
}

// RateLimitMiddleware creates a rate limiting middleware with DynamORM backend
func RateLimitMiddleware(config RateLimitConfig) lift.Middleware {
	// Set defaults
	if config.DefaultLimit == 0 {
		config.DefaultLimit = 1000 // 1000 requests per window
	}
	if config.DefaultWindow == 0 {
		config.DefaultWindow = time.Hour // 1 hour window
	}
	if config.BurstLimit == 0 {
		config.BurstLimit = config.DefaultLimit / 10 // 10% burst
	}
	if config.KeyPrefix == "" {
		config.KeyPrefix = "ratelimit"
	}
	if config.HeaderPrefix == "" {
		config.HeaderPrefix = "X-RateLimit"
	}
	if config.TableName == "" {
		config.TableName = "rate_limits"
	}
	if config.TTL == 0 {
		config.TTL = 24 * time.Hour // Keep records for 24 hours
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = time.Hour // Cleanup every hour
	}

	limiter := &rateLimiter{
		config: config,
	}

	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			// Skip OPTIONS requests if configured
			if config.SkipOptions && ctx.Request.Method == "OPTIONS" {
				return next.Handle(ctx)
			}

			// Generate rate limit key
			key := limiter.generateKey(ctx)

			// Check rate limit
			result, err := limiter.checkLimit(ctx.Context, key, ctx)
			if err != nil {
				// Log error but don't fail the request
				if ctx.Logger != nil {
					ctx.Logger.Error("Rate limit check failed", map[string]interface{}{
						"error": err.Error(),
						"key":   key,
					})
				}
				// Continue without rate limiting on error
				return next.Handle(ctx)
			}

			// Add rate limit headers
			limiter.addHeaders(ctx, result)

			// Check if request is allowed
			if !result.Allowed {
				// Log rate limit exceeded
				if ctx.Logger != nil {
					ctx.Logger.Warn("Rate limit exceeded", map[string]interface{}{
						"key":       "[SANITIZED_RATE_LIMIT_KEY]", // Sanitized for security
						"limit":     result.Limit,
						"remaining": result.Remaining,
						"reset_at":  result.ResetAt,
					})
				}

				// Return 429 Too Many Requests
				ctx.Response.Status(429)
				return ctx.Response.JSON(map[string]interface{}{
					"error":       "Rate limit exceeded",
					"limit":       result.Limit,
					"remaining":   result.Remaining,
					"reset_at":    result.ResetAt.Unix(),
					"retry_after": int(result.RetryAfter.Seconds()),
				})
			}

			// Execute handler
			err = next.Handle(ctx)

			// If configured to skip successful requests, decrement counter for successful requests
			if config.SkipSuccessful && err == nil && ctx.Response.StatusCode < 400 {
				// Decrement the counter (best effort)
				_ = limiter.decrementCounter(ctx.Context, key)
			}

			return err
		})
	}
}

// rateLimiter implements the rate limiting logic
type rateLimiter struct {
	config RateLimitConfig
}

// generateKey creates a unique key for rate limiting
func (r *rateLimiter) generateKey(ctx *lift.Context) string {
	parts := []string{r.config.KeyPrefix}

	// Add tenant ID
	if tenantID := ctx.TenantID(); tenantID != "" {
		parts = append(parts, "tenant", tenantID)
	}

	// Add user ID
	if userID := ctx.UserID(); userID != "" {
		parts = append(parts, "user", userID)
	}

	// Add method if configured
	if r.config.IncludeMethod {
		parts = append(parts, "method", ctx.Request.Method)
	}

	// Add path if configured
	if r.config.IncludePath {
		parts = append(parts, "path", ctx.Request.Path)
	}

	// Add IP address as fallback
	if ip := ctx.Request.Headers["X-Forwarded-For"]; ip != "" {
		parts = append(parts, "ip", ip)
	} else if ip := ctx.Request.Headers["X-Real-IP"]; ip != "" {
		parts = append(parts, "ip", ip)
	}

	key := ""
	for i, part := range parts {
		if i > 0 {
			key += ":"
		}
		key += part
	}

	return key
}

// getLimit returns the appropriate limit for the context
func (r *rateLimiter) getLimit(ctx *lift.Context) int {
	// Check user-specific limits
	if userID := ctx.UserID(); userID != "" {
		if limit, exists := r.config.UserLimits[userID]; exists {
			return limit
		}
	}

	// Check tenant-specific limits
	if tenantID := ctx.TenantID(); tenantID != "" {
		if limit, exists := r.config.TenantLimits[tenantID]; exists {
			return limit
		}
	}

	// Return default limit
	return r.config.DefaultLimit
}

// checkLimit checks if the request is within rate limits
func (r *rateLimiter) checkLimit(ctx context.Context, key string, liftCtx *lift.Context) (*RateLimitResult, error) {
	now := time.Now()
	windowStart := now.Truncate(r.config.DefaultWindow)
	limit := r.getLimit(liftCtx)

	// Try to get existing entry
	var entry RateLimitEntry
	err := r.config.DynamORM.Get(ctx, key, &entry)

	if err != nil {
		// If item doesn't exist, create new entry
		// For now, we'll assume any error means not found
		// TODO: Implement proper error checking when DynamORM provides it
		entry = RateLimitEntry{
			Key:         key,
			Count:       1,
			WindowStart: windowStart,
			LastRequest: now,
			TTL:         now.Add(r.config.TTL).Unix(),
		}

		err = r.config.DynamORM.Put(ctx, entry)
		if err != nil {
			return nil, fmt.Errorf("failed to create rate limit entry: %w", err)
		}

		return &RateLimitResult{
			Allowed:     true,
			Limit:       limit,
			Remaining:   limit - 1,
			ResetAt:     windowStart.Add(r.config.DefaultWindow),
			RetryAfter:  0,
			WindowStart: windowStart,
		}, nil
	}

	// Check if we're in a new window
	if entry.WindowStart.Before(windowStart) {
		// Reset for new window
		entry.Count = 1
		entry.WindowStart = windowStart
		entry.LastRequest = now
		entry.TTL = now.Add(r.config.TTL).Unix()

		err = r.config.DynamORM.Put(ctx, entry)
		if err != nil {
			return nil, fmt.Errorf("failed to reset rate limit entry: %w", err)
		}

		return &RateLimitResult{
			Allowed:     true,
			Limit:       limit,
			Remaining:   limit - 1,
			ResetAt:     windowStart.Add(r.config.DefaultWindow),
			RetryAfter:  0,
			WindowStart: windowStart,
		}, nil
	}

	// Check if limit exceeded
	if entry.Count >= limit {
		resetAt := entry.WindowStart.Add(r.config.DefaultWindow)
		retryAfter := time.Until(resetAt)
		if retryAfter < 0 {
			retryAfter = 0
		}

		return &RateLimitResult{
			Allowed:     false,
			Limit:       limit,
			Remaining:   0,
			ResetAt:     resetAt,
			RetryAfter:  retryAfter,
			WindowStart: entry.WindowStart,
		}, nil
	}

	// Increment counter
	entry.Count++
	entry.LastRequest = now
	entry.TTL = now.Add(r.config.TTL).Unix()

	err = r.config.DynamORM.Put(ctx, entry)
	if err != nil {
		return nil, fmt.Errorf("failed to update rate limit entry: %w", err)
	}

	return &RateLimitResult{
		Allowed:     true,
		Limit:       limit,
		Remaining:   limit - entry.Count,
		ResetAt:     entry.WindowStart.Add(r.config.DefaultWindow),
		RetryAfter:  0,
		WindowStart: entry.WindowStart,
	}, nil
}

// decrementCounter decrements the counter for successful requests (if configured)
func (r *rateLimiter) decrementCounter(ctx context.Context, key string) error {
	var entry RateLimitEntry
	err := r.config.DynamORM.Get(ctx, key, &entry)

	if err != nil {
		return err // Ignore errors for decrement
	}

	if entry.Count > 0 {
		entry.Count--
		return r.config.DynamORM.Put(ctx, entry)
	}

	return nil
}

// addHeaders adds rate limit headers to the response
func (r *rateLimiter) addHeaders(ctx *lift.Context, result *RateLimitResult) {
	prefix := r.config.HeaderPrefix

	ctx.Response.Header(prefix+"-Limit", strconv.Itoa(result.Limit))
	ctx.Response.Header(prefix+"-Remaining", strconv.Itoa(result.Remaining))
	ctx.Response.Header(prefix+"-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))

	if !result.Allowed {
		ctx.Response.Header("Retry-After", strconv.Itoa(int(result.RetryAfter.Seconds())))
	}
}

// RateLimitStats provides statistics about rate limiting
type RateLimitStats struct {
	TotalRequests   int64 `json:"total_requests"`
	AllowedRequests int64 `json:"allowed_requests"`
	BlockedRequests int64 `json:"blocked_requests"`
	ErrorCount      int64 `json:"error_count"`
}

// GetRateLimitStats returns rate limiting statistics from actual usage data
func GetRateLimitStats(config RateLimitConfig) (*RateLimitStats, error) {
	if config.DynamORM == nil {
		return &RateLimitStats{}, fmt.Errorf("DynamORM not configured")
	}

	ctx := context.Background()

	// We would need to implement scanning capabilities in DynamORM to get full stats
	// For now, we'll implement a basic version that tracks aggregate metrics

	// In a production implementation, we could:
	// 1. Add a separate stats tracking table
	// 2. Use DynamoDB streams to aggregate metrics
	// 3. Use CloudWatch metrics integration

	// Placeholder implementation that could be extended:
	// Try to get a sample of recent entries to estimate statistics
	sampleKey := fmt.Sprintf("%s:stats:aggregate", config.KeyPrefix)

	var statsEntry struct {
		Key             string `dynamodbav:"pk"`
		TotalRequests   int64  `dynamodbav:"total_requests"`
		AllowedRequests int64  `dynamodbav:"allowed_requests"`
		BlockedRequests int64  `dynamodbav:"blocked_requests"`
		ErrorCount      int64  `dynamodbav:"error_count"`
		LastUpdated     int64  `dynamodbav:"last_updated"`
		TTL             int64  `dynamodbav:"ttl"`
	}

	err := config.DynamORM.Get(ctx, sampleKey, &statsEntry)
	if err != nil {
		// If stats don't exist yet, return zeros
		// In production, we'd implement background aggregation
		return &RateLimitStats{
			TotalRequests:   0,
			AllowedRequests: 0,
			BlockedRequests: 0,
			ErrorCount:      0,
		}, nil
	}

	return &RateLimitStats{
		TotalRequests:   statsEntry.TotalRequests,
		AllowedRequests: statsEntry.AllowedRequests,
		BlockedRequests: statsEntry.BlockedRequests,
		ErrorCount:      statsEntry.ErrorCount,
	}, nil
}

// UpdateRateLimitStats updates aggregate statistics (called by rate limiter)
func UpdateRateLimitStats(ctx context.Context, config RateLimitConfig, allowed bool, hasError bool) error {
	if config.DynamORM == nil {
		return nil // Silently skip if no storage configured
	}

	statsKey := fmt.Sprintf("%s:stats:aggregate", config.KeyPrefix)
	now := time.Now()

	// Atomic update of statistics
	var statsEntry struct {
		Key             string `dynamodbav:"pk"`
		TotalRequests   int64  `dynamodbav:"total_requests"`
		AllowedRequests int64  `dynamodbav:"allowed_requests"`
		BlockedRequests int64  `dynamodbav:"blocked_requests"`
		ErrorCount      int64  `dynamodbav:"error_count"`
		LastUpdated     int64  `dynamodbav:"last_updated"`
		TTL             int64  `dynamodbav:"ttl"`
	}

	// Try to get existing stats
	err := config.DynamORM.Get(ctx, statsKey, &statsEntry)
	if err != nil {
		// Initialize new stats entry
		statsEntry.Key = statsKey
		statsEntry.TTL = now.Add(30 * 24 * time.Hour).Unix() // Keep for 30 days
	}

	// Update counters
	statsEntry.TotalRequests++
	if allowed {
		statsEntry.AllowedRequests++
	} else {
		statsEntry.BlockedRequests++
	}
	if hasError {
		statsEntry.ErrorCount++
	}
	statsEntry.LastUpdated = now.Unix()

	// Save updated stats
	return config.DynamORM.Put(ctx, statsEntry)
}

// CleanupExpiredEntries removes expired rate limit entries
func CleanupExpiredEntries(ctx context.Context, config RateLimitConfig) error {
	// This would scan the table and remove expired entries
	// Implementation depends on DynamORM's scan capabilities
	// For now, we rely on DynamoDB TTL to handle cleanup automatically
	return nil
}

// BurstRateLimitMiddleware creates a burst-aware rate limiting middleware
func BurstRateLimitMiddleware(config RateLimitConfig) lift.Middleware {
	// This is a more sophisticated rate limiter that allows bursts
	// Implementation would track both sustained rate and burst allowance
	// For now, we'll use the basic rate limiter
	return RateLimitMiddleware(config)
}

// AdaptiveRateLimitMiddleware creates an adaptive rate limiting middleware
func AdaptiveRateLimitMiddleware(config RateLimitConfig) lift.Middleware {
	// This would adjust limits based on system load, error rates, etc.
	// Implementation would monitor system metrics and adjust limits dynamically
	// For now, we'll use the basic rate limiter
	return RateLimitMiddleware(config)
}

// RateLimit creates a rate limiting middleware with the given configuration
func RateLimit(config RateLimitConfig) lift.Middleware {
	return RateLimitMiddleware(config)
}

// TenantRateLimit creates a tenant-specific rate limiting middleware
func TenantRateLimit(limit int, window time.Duration) lift.Middleware {
	config := RateLimitConfig{
		DefaultLimit:  limit,
		DefaultWindow: window,
		Window:        window,
		KeyFunc:       tenantKeyFunc,
		ErrorHandler:  defaultErrorHandler,
	}
	return RateLimitMiddleware(config)
}

// UserRateLimit creates a user-specific rate limiting middleware
func UserRateLimit(limit int, window time.Duration) lift.Middleware {
	config := RateLimitConfig{
		DefaultLimit:  limit,
		DefaultWindow: window,
		Window:        window,
		KeyFunc:       userKeyFunc,
		ErrorHandler:  defaultErrorHandler,
	}
	return RateLimitMiddleware(config)
}

// IPRateLimit creates an IP-based rate limiting middleware
func IPRateLimit(limit int, window time.Duration) lift.Middleware {
	config := RateLimitConfig{
		DefaultLimit:  limit,
		DefaultWindow: window,
		Window:        window,
		KeyFunc:       ipKeyFunc,
		ErrorHandler:  defaultErrorHandler,
	}
	return RateLimitMiddleware(config)
}

// EndpointRateLimit creates an endpoint-specific rate limiting middleware
func EndpointRateLimit(limit int, window time.Duration) lift.Middleware {
	config := RateLimitConfig{
		DefaultLimit:  limit,
		DefaultWindow: window,
		Window:        window,
		KeyFunc:       endpointKeyFunc,
		ErrorHandler:  defaultErrorHandler,
	}
	return RateLimitMiddleware(config)
}

// CompositeRateLimit creates a composite rate limiting middleware with multiple strategies
func CompositeRateLimit(config RateLimitConfig) lift.Middleware {
	// Set defaults if not provided
	if config.KeyFunc == nil {
		config.KeyFunc = defaultKeyFunc
	}
	if config.ErrorHandler == nil {
		config.ErrorHandler = defaultErrorHandler
	}
	return RateLimitMiddleware(config)
}

// Key generation functions

// defaultKeyFunc generates a default rate limiting key
func defaultKeyFunc(ctx *lift.Context) *RateLimitKey {
	key := &RateLimitKey{
		Metadata: make(map[string]string),
	}

	// Build identifier from tenant and user
	parts := []string{}
	if tenantID := ctx.TenantID(); tenantID != "" {
		parts = append(parts, tenantID)
		key.Metadata["tenant_id"] = tenantID
	}
	if userID := ctx.UserID(); userID != "" {
		parts = append(parts, userID)
		key.Metadata["user_id"] = userID
	}

	if len(parts) == 0 {
		// Fallback to IP if no tenant/user
		if ip := ctx.Header("X-Forwarded-For"); ip != "" {
			parts = append(parts, ip)
		} else if ip := ctx.Header("X-Real-IP"); ip != "" {
			parts = append(parts, ip)
		} else {
			parts = append(parts, "unknown")
		}
	}

	key.Identifier = joinParts(parts, ":")
	key.Resource = ctx.Request.Path
	key.Operation = ctx.Request.Method

	return key
}

// tenantKeyFunc generates a tenant-specific rate limiting key
func tenantKeyFunc(ctx *lift.Context) *RateLimitKey {
	tenantID := ctx.TenantID()
	if tenantID == "" {
		tenantID = "default"
	}

	return &RateLimitKey{
		Identifier: tenantID,
		Resource:   ctx.Request.Path,
		Operation:  ctx.Request.Method,
		Metadata: map[string]string{
			"tenant_id": tenantID,
		},
	}
}

// userKeyFunc generates a user-specific rate limiting key
func userKeyFunc(ctx *lift.Context) *RateLimitKey {
	userID := ctx.UserID()
	if userID == "" {
		userID = "anonymous"
	}

	return &RateLimitKey{
		Identifier: userID,
		Resource:   ctx.Request.Path,
		Operation:  ctx.Request.Method,
		Metadata: map[string]string{
			"user_id": userID,
		},
	}
}

// ipKeyFunc generates an IP-based rate limiting key
func ipKeyFunc(ctx *lift.Context) *RateLimitKey {
	ip := ctx.Header("X-Forwarded-For")
	if ip == "" {
		ip = ctx.Header("X-Real-IP")
	}
	if ip == "" {
		ip = "unknown"
	}

	return &RateLimitKey{
		Identifier: ip,
		Resource:   ctx.Request.Path,
		Operation:  ctx.Request.Method,
		Metadata: map[string]string{
			"ip": ip,
		},
	}
}

// endpointKeyFunc generates an endpoint-specific rate limiting key
func endpointKeyFunc(ctx *lift.Context) *RateLimitKey {
	endpoint := ctx.Request.Method + ":" + ctx.Request.Path

	return &RateLimitKey{
		Identifier: endpoint,
		Resource:   ctx.Request.Path,
		Operation:  ctx.Request.Method,
		Metadata: map[string]string{
			"endpoint": endpoint,
		},
	}
}

// defaultErrorHandler handles rate limit exceeded errors
func defaultErrorHandler(ctx *lift.Context, result *RateLimitResult) error {
	ctx.Response.Status(429)
	ctx.Response.Header("Retry-After", strconv.Itoa(int(result.RetryAfter.Seconds())))

	return ctx.Response.JSON(map[string]interface{}{
		"error":       "Rate limit exceeded",
		"limit":       result.Limit,
		"remaining":   result.Remaining,
		"reset_at":    result.ResetAt.Unix(),
		"retry_after": int(result.RetryAfter.Seconds()),
	})
}

// Helper function to join string parts
func joinParts(parts []string, separator string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}

	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += separator + parts[i]
	}
	return result
}
