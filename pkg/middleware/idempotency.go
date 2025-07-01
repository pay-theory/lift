package middleware

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
)

// IdempotencyStore defines the interface for storing idempotency keys and responses
type IdempotencyStore interface {
	// Get retrieves a stored response by key
	Get(ctx context.Context, key string) (*IdempotencyRecord, error)
	// Set stores a response with the given key
	Set(ctx context.Context, key string, record *IdempotencyRecord) error
	// SetProcessing marks a key as being processed (prevents concurrent duplicates)
	SetProcessing(ctx context.Context, key string, expiresAt time.Time) error
	// Delete removes a key from the store
	Delete(ctx context.Context, key string) error
}

// IdempotencyRecord represents a stored idempotent response
type IdempotencyRecord struct {
	Key            string    `json:"key"`
	Status         string    `json:"status"` // "processing", "completed", "error"
	Response       any       `json:"response,omitempty"`
	StatusCode     int       `json:"status_code,omitempty"`
	Error          string    `json:"error,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	RequestHash    string    `json:"request_hash,omitempty"`
}

// IdempotencyOptions configures the idempotency middleware
type IdempotencyOptions struct {
	// Store is the backend for storing idempotency records
	Store IdempotencyStore
	// HeaderName is the header to check for idempotency key (default: "Idempotency-Key")
	HeaderName string
	// TTL is how long to store successful responses (default: 24 hours)
	TTL time.Duration
	// ProcessingTimeout is how long to wait for in-flight requests (default: 30 seconds)
	ProcessingTimeout time.Duration
	// IncludeRequestHash includes request body hash for stricter validation
	IncludeRequestHash bool
	// OnDuplicate is called when a duplicate request is detected
	OnDuplicate func(ctx *lift.Context, record *IdempotencyRecord)
}


// Idempotency creates middleware that provides idempotent request handling
func Idempotency(opts IdempotencyOptions) Middleware {
	// Set defaults
	if opts.HeaderName == "" {
		opts.HeaderName = "Idempotency-Key"
	}
	if opts.TTL == 0 {
		opts.TTL = 24 * time.Hour
	}
	if opts.ProcessingTimeout == 0 {
		opts.ProcessingTimeout = 30 * time.Second
	}

	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			// Check for idempotency key
			idempotencyKey := ctx.Header(opts.HeaderName)
			if idempotencyKey == "" {
				// No idempotency key, process normally
				return next.Handle(ctx)
			}

			// Add account/tenant context to key for isolation
			accountID := ctx.Get("account_id")
			if accountID != nil {
				idempotencyKey = fmt.Sprintf("%v:%s", accountID, idempotencyKey)
			}

			// Check for existing record
			existing, err := opts.Store.Get(ctx.Request.Context(), idempotencyKey)
			if err == nil && existing != nil {
				switch existing.Status {
				case "completed":
					// Return cached successful response
					if opts.OnDuplicate != nil {
						opts.OnDuplicate(ctx, existing)
					}
					ctx.Response.StatusCode = existing.StatusCode
					ctx.Response.Header("X-Idempotent-Replay", "true")
					// Directly set the body and mark as written using proper API
					ctx.Response.Body = existing.Response
					ctx.Response.Header("Content-Type", "application/json")
					return nil
					
				case "error":
					// Return cached error
					if existing.Error != "" {
						return lift.NewLiftError("IDEMPOTENT_ERROR_REPLAY", existing.Error, existing.StatusCode)
					}
					return lift.NewLiftError("IDEMPOTENT_ERROR_REPLAY", "Previous request failed", 500)
					
				case "processing":
					// Check if processing timeout has elapsed
					if time.Now().After(existing.ExpiresAt) {
						// Timeout elapsed, allow retry
						if err := opts.Store.Delete(ctx.Request.Context(), idempotencyKey); err != nil {
							if ctx.Logger != nil {
								ctx.Logger.Error("Failed to delete expired idempotency key", map[string]any{
									"key": idempotencyKey,
									"error": err.Error(),
								})
							}
						}
					} else {
						// Still processing
						return lift.NewLiftError("IDEMPOTENCY_CONFLICT", "A request with this idempotency key is already being processed", 409)
					}
				}
			}

			// Mark as processing to prevent concurrent duplicates
			processingRecord := &IdempotencyRecord{
				Key:       idempotencyKey,
				Status:    "processing",
				CreatedAt: time.Now(),
				ExpiresAt: time.Now().Add(opts.ProcessingTimeout),
			}
			
			if err := opts.Store.SetProcessing(ctx.Request.Context(), idempotencyKey, processingRecord.ExpiresAt); err != nil {
				// Log but continue - idempotency is best-effort
				if ctx.Logger != nil {
					ctx.Logger.Warn("Failed to set idempotency processing lock", map[string]any{
						"key": idempotencyKey,
						"error": err.Error(),
					})
				}
			}

			// Enable response buffering to capture the response
			ctx.EnableResponseBuffering()

			// Execute handler
			handlerErr := next.Handle(ctx)
			
			// Capture response after handler execution
			var capturedResponse any
			var capturedStatus int
			
			// Try to get response from buffer first
			if buffer := ctx.GetResponseBuffer(); buffer != nil {
				capturedResponse = buffer.CapturedData
				capturedStatus = buffer.StatusCode
			} else {
				// Fallback to Response.Body (may not be reliable)
				capturedResponse = ctx.Response.Body
				capturedStatus = ctx.Response.StatusCode
			}
			
			if capturedStatus == 0 {
				capturedStatus = 200
			}

			// Prepare record for storage
			record := &IdempotencyRecord{
				Key:       idempotencyKey,
				CreatedAt: time.Now(),
				ExpiresAt: time.Now().Add(opts.TTL),
			}

			if handlerErr != nil {
				// Store error result
				record.Status = "error"
				record.Error = handlerErr.Error()
				if liftErr, ok := handlerErr.(*lift.LiftError); ok {
					record.StatusCode = liftErr.StatusCode
				} else {
					record.StatusCode = 500
				}
			} else {
				// Store successful result
				record.Status = "completed"
				record.Response = capturedResponse
				record.StatusCode = capturedStatus
				if record.StatusCode == 0 {
					record.StatusCode = 200
				}
			}

			// Store the result
			if storeErr := opts.Store.Set(ctx.Request.Context(), idempotencyKey, record); storeErr != nil {
				// Log but don't fail the request
				if ctx.Logger != nil {
					ctx.Logger.Error("Failed to store idempotency record", map[string]any{
						"key": idempotencyKey,
						"error": storeErr.Error(),
					})
				}
			}

			return handlerErr
		})
	}
}

// MemoryIdempotencyStore provides an in-memory implementation of IdempotencyStore
// This is suitable for single-instance applications or testing
type MemoryIdempotencyStore struct {
	mu      sync.RWMutex
	records map[string]*IdempotencyRecord
}

// NewMemoryIdempotencyStore creates a new in-memory idempotency store
func NewMemoryIdempotencyStore() *MemoryIdempotencyStore {
	return &MemoryIdempotencyStore{
		records: make(map[string]*IdempotencyRecord),
	}
}

// Get retrieves a record by key
func (m *MemoryIdempotencyStore) Get(ctx context.Context, key string) (*IdempotencyRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	record, exists := m.records[key]
	if !exists {
		return nil, nil
	}
	
	// Check if expired
	if time.Now().After(record.ExpiresAt) {
		return nil, nil
	}
	
	return record, nil
}

// Set stores a record
func (m *MemoryIdempotencyStore) Set(ctx context.Context, key string, record *IdempotencyRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.records[key] = record
	
	// Clean up expired records periodically
	m.cleanupExpired()
	
	return nil
}

// SetProcessing marks a key as being processed
func (m *MemoryIdempotencyStore) SetProcessing(ctx context.Context, key string, expiresAt time.Time) error {
	return m.Set(ctx, key, &IdempotencyRecord{
		Key:       key,
		Status:    "processing",
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
	})
}

// Delete removes a record
func (m *MemoryIdempotencyStore) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	delete(m.records, key)
	return nil
}

// cleanupExpired removes expired records (called with lock held)
func (m *MemoryIdempotencyStore) cleanupExpired() {
	now := time.Now()
	for key, record := range m.records {
		if now.After(record.ExpiresAt) {
			delete(m.records, key)
		}
	}
}