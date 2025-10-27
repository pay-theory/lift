package security

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingAuditStorage struct {
	*InMemoryAuditStorage
	mu         sync.Mutex
	batchCalls [][]AuditLogEntry
}

func newRecordingAuditStorage() *recordingAuditStorage {
	return &recordingAuditStorage{
		InMemoryAuditStorage: NewInMemoryAuditStorage(),
		batchCalls:           make([][]AuditLogEntry, 0),
	}
}

func (ras *recordingAuditStorage) BatchStore(ctx context.Context, entries []AuditLogEntry) error {
	copied := make([]AuditLogEntry, len(entries))
	copy(copied, entries)

	ras.mu.Lock()
	ras.batchCalls = append(ras.batchCalls, copied)
	ras.mu.Unlock()

	return ras.InMemoryAuditStorage.BatchStore(ctx, entries)
}

func (ras *recordingAuditStorage) lastBatch() []AuditLogEntry {
	ras.mu.Lock()
	defer ras.mu.Unlock()

	if len(ras.batchCalls) == 0 {
		return nil
	}
	return ras.batchCalls[len(ras.batchCalls)-1]
}

func (ras *recordingAuditStorage) batchCount() int {
	ras.mu.Lock()
	defer ras.mu.Unlock()
	return len(ras.batchCalls)
}

func (ras *recordingAuditStorage) corruptChecksum(auditID string) {
	ras.InMemoryAuditStorage.mu.Lock()
	defer ras.InMemoryAuditStorage.mu.Unlock()

	entries := ras.InMemoryAuditStorage.entries[auditID]
	if len(entries) > 0 {
		entries[0].Checksum = "invalid"
		ras.InMemoryAuditStorage.entries[auditID] = entries
	}
}

type stubAuditLogger struct {
	infos []string
	mu    sync.Mutex
}

func (l *stubAuditLogger) Error(msg string, keysAndValues ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.infos = append(l.infos, msg)
	_ = keysAndValues
}

func (l *stubAuditLogger) Info(msg string, keysAndValues ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.infos = append(l.infos, msg)
	_ = keysAndValues
}

func (l *stubAuditLogger) Warn(msg string, keysAndValues ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.infos = append(l.infos, msg)
	_ = keysAndValues
}

type stubLiftContext struct {
	values        map[string]any
	userID        string
	tenantID      string
	clientIP      string
	logger        Logger
	dataAccessLog []string
	mu            sync.Mutex
}

func newStubLiftContext(userID, tenantID string) *stubLiftContext {
	return &stubLiftContext{
		values:        make(map[string]any),
		userID:        userID,
		tenantID:      tenantID,
		clientIP:      "127.0.0.1",
		logger:        &stubAuditLogger{},
		dataAccessLog: make([]string, 0),
	}
}

func (s *stubLiftContext) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[key] = value
}

func (s *stubLiftContext) Get(key string) any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.values[key]
}

func (s *stubLiftContext) UserID() string {
	return s.userID
}

func (s *stubLiftContext) TenantID() string {
	return s.tenantID
}

func (s *stubLiftContext) ClientIP() string {
	return s.clientIP
}

func (s *stubLiftContext) Logger() Logger {
	return s.logger
}

func (s *stubLiftContext) GetDataAccessLog() []string {
	return s.dataAccessLog
}

func TestBufferedAuditLogger_LogRequestFlushesAndUpdatesMetrics(t *testing.T) {
	storage := newRecordingAuditStorage()
	logger := NewBufferedAuditLogger(storage, 1, time.Hour)
	defer func() {
		require.NoError(t, logger.Stop())
	}()

	ctx := newStubLiftContext("user-1", "tenant-1")
	auditID := logger.StartAudit(ctx)
	require.Equal(t, auditID, ctx.Get("audit_id"))

	before := time.Now()
	request := &AuditRequest{
		UserID:    "user-1",
		TenantID:  "tenant-1",
		Action:    "create",
		Resource:  "resource",
		Timestamp: before,
	}

	require.NoError(t, logger.LogRequest(auditID, request))

	require.Equal(t, 1, storage.batchCount())

	batch := storage.lastBatch()
	require.Len(t, batch, 1)
	entry := batch[0]

	assert.Equal(t, "request", entry.EntryType)
	assert.Equal(t, request.UserID, entry.UserID)
	assert.Equal(t, request.TenantID, entry.TenantID)
	assert.NotEmpty(t, entry.Checksum)
	assert.Less(t, before.Unix(), entry.TTL)

	metrics := logger.GetAuditMetrics()
	assert.Equal(t, int64(1), metrics.TotalEntries)
	assert.Equal(t, int64(1), metrics.FlushCount)
	assert.Equal(t, 0, metrics.BufferedEntries)
}

func TestBufferedAuditLogger_StopFlushesBufferedEntries(t *testing.T) {
	storage := newRecordingAuditStorage()
	logger := NewBufferedAuditLogger(storage, 5, time.Hour)
	stopped := false
	t.Cleanup(func() {
		if !stopped {
			require.NoError(t, logger.Stop())
		}
	})

	auditID := "audit-123"
	response := &AuditResponse{
		StatusCode: 200,
		Duration:   time.Millisecond,
	}
	require.NoError(t, logger.LogResponse(auditID, response))
	require.NoError(t, logger.LogResponse(auditID, response))

	assert.Equal(t, 0, storage.batchCount(), "buffer should not flush until Stop is called")

	require.NoError(t, logger.Stop())
	stopped = true

	require.Equal(t, 1, storage.batchCount())
	batch := storage.lastBatch()
	require.Len(t, batch, 2)
}

func TestBufferedAuditLogger_QueryAuditTrailAppliesFilters(t *testing.T) {
	storage := newRecordingAuditStorage()
	logger := NewBufferedAuditLogger(storage, 10, time.Hour)
	defer func() {
		require.NoError(t, logger.Stop())
	}()

	now := time.Now()
	entries := []AuditLogEntry{
		{
			ID:        "entry1",
			AuditID:   "audit-1",
			TenantID:  "tenant-1",
			UserID:    "user-1",
			EntryType: "request",
			Timestamp: now.Add(-2 * time.Hour),
		},
		{
			ID:        "entry2",
			AuditID:   "audit-2",
			TenantID:  "tenant-2",
			UserID:    "user-2",
			EntryType: "response",
			Timestamp: now.Add(-1 * time.Hour),
		},
	}

	for i := range entries {
		entries[i].Checksum = logger.calculateChecksum(entries[i])
	}

	require.NoError(t, storage.BatchStore(context.Background(), entries))

	filter := AuditFilter{
		TenantID:  "tenant-1",
		EntryType: "request",
	}

	result, err := logger.QueryAuditTrail(context.Background(), filter)
	require.NoError(t, err)
	require.Len(t, result.Entries, 1)
	assert.Equal(t, "entry1", result.Entries[0].ID)
}

func TestBufferedAuditLogger_VerifyIntegrityDetectsTampering(t *testing.T) {
	storage := newRecordingAuditStorage()
	logger := NewBufferedAuditLogger(storage, 10, time.Hour)
	defer func() {
		require.NoError(t, logger.Stop())
	}()

	entry := AuditLogEntry{
		ID:        "entry-integrity",
		AuditID:   "audit-integrity",
		EntryType: "request",
		Timestamp: time.Now(),
	}
	entry.Checksum = logger.calculateChecksum(entry)

	require.NoError(t, storage.BatchStore(context.Background(), []AuditLogEntry{entry}))

	ok, err := logger.VerifyIntegrity(context.Background(), "audit-integrity")
	require.NoError(t, err)
	assert.True(t, ok)

	storage.corruptChecksum("audit-integrity")

	ok, err = logger.VerifyIntegrity(context.Background(), "audit-integrity")
	require.Error(t, err)
	assert.False(t, ok)
}
