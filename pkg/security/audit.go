package security

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// LiftContext represents the minimal interface needed from lift.Context
type LiftContext interface {
	Set(key string, value interface{})
	Get(key string) interface{}
	UserID() string
	TenantID() string
	ClientIP() string
	Logger() Logger
	GetDataAccessLog() []string
}

// Logger represents the minimal logging interface needed
type Logger interface {
	Error(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
}

// AuditStorage defines the interface for audit log storage
type AuditStorage interface {
	Store(ctx context.Context, entry AuditLogEntry) error
	Query(ctx context.Context, filter AuditFilter) ([]AuditLogEntry, error)
	BatchStore(ctx context.Context, entries []AuditLogEntry) error
}

// AuditFilter defines filters for querying audit logs
type AuditFilter struct {
	UserID    string    `json:"user_id,omitempty"`
	TenantID  string    `json:"tenant_id,omitempty"`
	AuditID   string    `json:"audit_id,omitempty"`
	EntryType string    `json:"entry_type,omitempty"`
	Since     time.Time `json:"since,omitempty"`
	Until     time.Time `json:"until,omitempty"`
	Limit     int       `json:"limit,omitempty"`
}

// BufferedAuditLogger implements AuditLogger with buffering for performance
type BufferedAuditLogger struct {
	storage      AuditStorage
	bufferSize   int
	flushTimeout time.Duration
	buffer       []AuditLogEntry
	bufferMu     sync.Mutex
	flushTicker  *time.Ticker
	stopCh       chan struct{}
	wg           sync.WaitGroup
	metrics      AuditLoggerMetrics
	metricsMu    sync.RWMutex
}

// AuditLogEntry represents a complete audit log entry
type AuditLogEntry struct {
	ID            string                 `json:"id"`
	AuditID       string                 `json:"audit_id"`
	TenantID      string                 `json:"tenant_id"`
	UserID        string                 `json:"user_id"`
	EntryType     string                 `json:"entry_type"` // request, response, data_access, security_event
	Timestamp     time.Time              `json:"timestamp"`
	TTL           int64                  `json:"ttl"`
	Request       *AuditRequest          `json:"request,omitempty"`
	Response      *AuditResponse         `json:"response,omitempty"`
	DataAccess    *DataAccessLog         `json:"data_access,omitempty"`
	SecurityEvent *SecurityEvent         `json:"security_event,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Checksum      string                 `json:"checksum"`
}

// AuditQueryResult represents the result of an audit query
type AuditQueryResult struct {
	Entries    []AuditLogEntry `json:"entries"`
	NextToken  string          `json:"next_token,omitempty"`
	TotalCount int             `json:"total_count"`
}

// AuditLoggerMetrics tracks audit system performance
type AuditLoggerMetrics struct {
	TotalEntries      int64         `json:"total_entries"`
	BufferedEntries   int           `json:"buffered_entries"`
	FlushCount        int64         `json:"flush_count"`
	ErrorCount        int64         `json:"error_count"`
	AverageLatency    time.Duration `json:"average_latency"`
	LastFlush         time.Time     `json:"last_flush"`
	BufferUtilization float64       `json:"buffer_utilization"`
}

// NewBufferedAuditLogger creates a new buffered audit logger
func NewBufferedAuditLogger(storage AuditStorage, bufferSize int, flushTimeout time.Duration) *BufferedAuditLogger {
	logger := &BufferedAuditLogger{
		storage:      storage,
		bufferSize:   bufferSize,
		flushTimeout: flushTimeout,
		buffer:       make([]AuditLogEntry, 0, bufferSize),
		stopCh:       make(chan struct{}),
	}

	// Start background flusher
	logger.startFlusher()

	return logger
}

// StartAudit starts a new audit session and returns an audit ID
func (bal *BufferedAuditLogger) StartAudit(ctx LiftContext) string {
	auditID := bal.generateAuditID()

	// Store audit ID in context for correlation
	ctx.Set("audit_id", auditID)

	return auditID
}

// LogRequest logs an audit request
func (bal *BufferedAuditLogger) LogRequest(auditID string, request *AuditRequest) error {
	entry := AuditLogEntry{
		ID:        bal.generateEntryID(),
		AuditID:   auditID,
		TenantID:  request.TenantID,
		UserID:    request.UserID,
		EntryType: "request",
		Timestamp: request.Timestamp,
		TTL:       time.Now().Add(365 * 24 * time.Hour).Unix(), // 1 year retention
		Request:   request,
	}

	entry.Checksum = bal.calculateChecksum(entry)

	return bal.bufferEntry(entry)
}

// LogResponse logs an audit response
func (bal *BufferedAuditLogger) LogResponse(auditID string, response *AuditResponse) error {
	entry := AuditLogEntry{
		ID:        bal.generateEntryID(),
		AuditID:   auditID,
		EntryType: "response",
		Timestamp: time.Now(),
		TTL:       time.Now().Add(365 * 24 * time.Hour).Unix(),
		Response:  response,
	}

	entry.Checksum = bal.calculateChecksum(entry)

	return bal.bufferEntry(entry)
}

// LogDataAccess logs data access for audit trails
func (bal *BufferedAuditLogger) LogDataAccess(auditID string, access *DataAccessLog) error {
	entry := AuditLogEntry{
		ID:         bal.generateEntryID(),
		AuditID:    auditID,
		EntryType:  "data_access",
		Timestamp:  access.Timestamp,
		TTL:        time.Now().Add(365 * 24 * time.Hour).Unix(),
		DataAccess: access,
	}

	entry.Checksum = bal.calculateChecksum(entry)

	return bal.bufferEntry(entry)
}

// LogSecurityEvent logs a security event
func (bal *BufferedAuditLogger) LogSecurityEvent(auditID string, event *SecurityEvent) error {
	entry := AuditLogEntry{
		ID:            bal.generateEntryID(),
		AuditID:       auditID,
		EntryType:     "security_event",
		Timestamp:     event.Timestamp,
		TTL:           time.Now().Add(365 * 24 * time.Hour).Unix(),
		SecurityEvent: event,
	}

	entry.Checksum = bal.calculateChecksum(entry)

	return bal.bufferEntry(entry)
}

// bufferEntry adds an entry to the buffer
func (bal *BufferedAuditLogger) bufferEntry(entry AuditLogEntry) error {
	bal.bufferMu.Lock()
	defer bal.bufferMu.Unlock()

	bal.buffer = append(bal.buffer, entry)

	// Update metrics
	bal.metricsMu.Lock()
	bal.metrics.TotalEntries++
	bal.metrics.BufferedEntries = len(bal.buffer)
	bal.metrics.BufferUtilization = float64(len(bal.buffer)) / float64(bal.bufferSize)
	bal.metricsMu.Unlock()

	// Flush if buffer is full
	if len(bal.buffer) >= bal.bufferSize {
		return bal.flushBuffer()
	}

	return nil
}

// flushBuffer writes buffered entries to storage
func (bal *BufferedAuditLogger) flushBuffer() error {
	if len(bal.buffer) == 0 {
		return nil
	}

	start := time.Now()

	// Create batch write
	entries := make([]AuditLogEntry, len(bal.buffer))
	copy(entries, bal.buffer)

	// Batch write to storage
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := bal.storage.BatchStore(ctx, entries)

	// Update metrics
	bal.metricsMu.Lock()
	if err != nil {
		bal.metrics.ErrorCount++
	} else {
		bal.metrics.FlushCount++
		bal.metrics.LastFlush = time.Now()

		// Update average latency
		latency := time.Since(start)
		if bal.metrics.AverageLatency == 0 {
			bal.metrics.AverageLatency = latency
		} else {
			bal.metrics.AverageLatency = (bal.metrics.AverageLatency + latency) / 2
		}
	}
	bal.metrics.BufferedEntries = 0
	bal.metrics.BufferUtilization = 0
	bal.metricsMu.Unlock()

	if err != nil {
		return fmt.Errorf("failed to flush audit buffer: %w", err)
	}

	// Clear buffer
	bal.buffer = bal.buffer[:0]

	return nil
}

// startFlusher starts the background flusher goroutine
func (bal *BufferedAuditLogger) startFlusher() {
	bal.flushTicker = time.NewTicker(bal.flushTimeout)
	bal.wg.Add(1)

	go func() {
		defer bal.wg.Done()

		for {
			select {
			case <-bal.flushTicker.C:
				bal.bufferMu.Lock()
				if len(bal.buffer) > 0 {
					if err := bal.flushBuffer(); err != nil {
						// Log error but continue
						fmt.Printf("Error flushing audit buffer: %v\n", err)
					}
				}
				bal.bufferMu.Unlock()

			case <-bal.stopCh:
				// Final flush before stopping
				bal.bufferMu.Lock()
				bal.flushBuffer()
				bal.bufferMu.Unlock()
				return
			}
		}
	}()
}

// Stop stops the audit logger and flushes remaining entries
func (bal *BufferedAuditLogger) Stop() error {
	close(bal.stopCh)
	bal.flushTicker.Stop()
	bal.wg.Wait()

	return nil
}

// QueryAuditTrail queries the audit trail
func (bal *BufferedAuditLogger) QueryAuditTrail(ctx context.Context, filter AuditFilter) (*AuditQueryResult, error) {
	entries, err := bal.storage.Query(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit trail: %w", err)
	}

	return &AuditQueryResult{
		Entries:    entries,
		TotalCount: len(entries),
	}, nil
}

// GetAuditMetrics returns audit system metrics
func (bal *BufferedAuditLogger) GetAuditMetrics() AuditLoggerMetrics {
	bal.metricsMu.RLock()
	defer bal.metricsMu.RUnlock()

	metrics := bal.metrics

	// Update current buffer state
	bal.bufferMu.Lock()
	metrics.BufferedEntries = len(bal.buffer)
	metrics.BufferUtilization = float64(len(bal.buffer)) / float64(bal.bufferSize)
	bal.bufferMu.Unlock()

	return metrics
}

// VerifyIntegrity verifies the integrity of audit entries
func (bal *BufferedAuditLogger) VerifyIntegrity(ctx context.Context, auditID string) (bool, error) {
	filter := AuditFilter{
		AuditID: auditID,
	}

	entries, err := bal.storage.Query(ctx, filter)
	if err != nil {
		return false, fmt.Errorf("failed to query audit entries: %w", err)
	}

	// Verify checksums
	for _, entry := range entries {
		expectedChecksum := bal.calculateChecksum(entry)
		if entry.Checksum != expectedChecksum {
			return false, fmt.Errorf("checksum mismatch for entry %s", entry.ID)
		}
	}

	return true, nil
}

// generateAuditID generates a unique audit ID
func (bal *BufferedAuditLogger) generateAuditID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if random generation fails
		return fmt.Sprintf("audit_%d_fallback", time.Now().UnixNano())
	}
	return fmt.Sprintf("audit_%d_%s", time.Now().Unix(), hex.EncodeToString(bytes))
}

// generateEntryID generates a unique entry ID
func (bal *BufferedAuditLogger) generateEntryID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if random generation fails
		return fmt.Sprintf("entry_%d_fallback", time.Now().UnixNano())
	}
	return fmt.Sprintf("entry_%d_%s", time.Now().UnixNano(), hex.EncodeToString(bytes))
}

// calculateChecksum calculates a checksum for audit entry integrity
func (bal *BufferedAuditLogger) calculateChecksum(entry AuditLogEntry) string {
	// Create a copy without the checksum field
	entryCopy := entry
	entryCopy.Checksum = ""

	// Marshal to JSON for consistent hashing
	data, err := json.Marshal(entryCopy)
	if err != nil {
		return ""
	}

	// Simple checksum (in production, use a proper hash function)
	hash := uint32(0)
	for _, b := range data {
		hash = hash*31 + uint32(b)
	}

	return fmt.Sprintf("%08x", hash)
}

// InMemoryAuditStorage implements AuditStorage for testing and development
type InMemoryAuditStorage struct {
	entries map[string][]AuditLogEntry
	mu      sync.RWMutex
}

// NewInMemoryAuditStorage creates a new in-memory audit storage
func NewInMemoryAuditStorage() *InMemoryAuditStorage {
	return &InMemoryAuditStorage{
		entries: make(map[string][]AuditLogEntry),
	}
}

// Store stores a single audit entry
func (imas *InMemoryAuditStorage) Store(ctx context.Context, entry AuditLogEntry) error {
	imas.mu.Lock()
	defer imas.mu.Unlock()

	if _, exists := imas.entries[entry.AuditID]; !exists {
		imas.entries[entry.AuditID] = make([]AuditLogEntry, 0)
	}

	imas.entries[entry.AuditID] = append(imas.entries[entry.AuditID], entry)
	return nil
}

// BatchStore stores multiple audit entries
func (imas *InMemoryAuditStorage) BatchStore(ctx context.Context, entries []AuditLogEntry) error {
	imas.mu.Lock()
	defer imas.mu.Unlock()

	for _, entry := range entries {
		if _, exists := imas.entries[entry.AuditID]; !exists {
			imas.entries[entry.AuditID] = make([]AuditLogEntry, 0)
		}
		imas.entries[entry.AuditID] = append(imas.entries[entry.AuditID], entry)
	}

	return nil
}

// Query queries audit entries based on filter
func (imas *InMemoryAuditStorage) Query(ctx context.Context, filter AuditFilter) ([]AuditLogEntry, error) {
	imas.mu.RLock()
	defer imas.mu.RUnlock()

	var results []AuditLogEntry

	for auditID, entries := range imas.entries {
		if filter.AuditID != "" && auditID != filter.AuditID {
			continue
		}

		for _, entry := range entries {
			if filter.UserID != "" && entry.UserID != filter.UserID {
				continue
			}

			if filter.TenantID != "" && entry.TenantID != filter.TenantID {
				continue
			}

			if filter.EntryType != "" && entry.EntryType != filter.EntryType {
				continue
			}

			if !filter.Since.IsZero() && entry.Timestamp.Before(filter.Since) {
				continue
			}

			if !filter.Until.IsZero() && entry.Timestamp.After(filter.Until) {
				continue
			}

			results = append(results, entry)

			if filter.Limit > 0 && len(results) >= filter.Limit {
				return results, nil
			}
		}
	}

	return results, nil
}

// Clear clears all audit entries
func (imas *InMemoryAuditStorage) Clear() {
	imas.mu.Lock()
	defer imas.mu.Unlock()
	imas.entries = make(map[string][]AuditLogEntry)
}
