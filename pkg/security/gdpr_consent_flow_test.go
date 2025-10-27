package security

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memoryConsentStore struct {
	mu         sync.Mutex
	consents   map[string]*ConsentRecord
	withdrawal []string
}

func newMemoryConsentStore() *memoryConsentStore {
	return &memoryConsentStore{
		consents: make(map[string]*ConsentRecord),
	}
}

func (m *memoryConsentStore) StoreConsent(_ context.Context, consent *ConsentRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.consents[consent.ID] = consent
	return nil
}

func (m *memoryConsentStore) GetConsent(_ context.Context, dataSubjectID, purpose string) (*ConsentRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, consent := range m.consents {
		if consent.DataSubjectID == dataSubjectID {
			if consent.Purpose == purpose {
				return consent, nil
			}
			for _, p := range consent.ProcessingPurposes {
				if p == purpose {
					return consent, nil
				}
			}
		}
	}
	return nil, ErrConsentNotFound
}

func (m *memoryConsentStore) GetAllConsents(_ context.Context, dataSubjectID string) ([]*ConsentRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	results := make([]*ConsentRecord, 0)
	for _, consent := range m.consents {
		if consent.DataSubjectID == dataSubjectID {
			results = append(results, consent)
		}
	}
	return results, nil
}

func (m *memoryConsentStore) UpdateConsent(_ context.Context, consentID string, updates *ConsentUpdates) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if consent, ok := m.consents[consentID]; ok {
		consent.Metadata = updates.Metadata
	}
	return nil
}

func (m *memoryConsentStore) WithdrawConsent(_ context.Context, consentID string, withdrawal *ConsentWithdrawal) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if consent, ok := m.consents[consentID]; ok {
		consent.Status = "withdrawn"
		consent.WithdrawalDate = &withdrawal.WithdrawalDate
		m.withdrawal = append(m.withdrawal, consentID)
		return nil
	}
	return ErrConsentNotFound
}

func (m *memoryConsentStore) GetExpiredConsents(_ context.Context) ([]*ConsentRecord, error) {
	return nil, nil
}

func (m *memoryConsentStore) GetConsentsForRenewal(_ context.Context) ([]*ConsentRecord, error) {
	return nil, nil
}

func (m *memoryConsentStore) RecordConsent(ctx context.Context, consent *ConsentRecord) error {
	return m.StoreConsent(ctx, consent)
}

func (m *memoryConsentStore) ListConsents(_ context.Context, dataSubjectID string) ([]*ConsentRecord, error) {
	return m.GetAllConsents(context.Background(), dataSubjectID)
}

func (m *memoryConsentStore) GetConsentHistory(_ context.Context, _ string) ([]*ConsentHistoryEntry, error) {
	return nil, nil
}

func (m *memoryConsentStore) CleanupExpiredConsents(_ context.Context) error {
	return nil
}

type stubGDPRAuditLogger struct {
	mu              sync.Mutex
	consentEvents   []*ConsentEvent
	requestLogs     []*DataSubjectRequestLog
	processingLogs  []*DataProcessingLog
	crossBorderLogs []*CrossBorderTransferLog
}

func (s *stubGDPRAuditLogger) LogConsentEvent(_ context.Context, event *ConsentEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.consentEvents = append(s.consentEvents, event)
	return nil
}

func (s *stubGDPRAuditLogger) LogDataSubjectRequest(_ context.Context, log *DataSubjectRequestLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requestLogs = append(s.requestLogs, log)
	return nil
}

func (s *stubGDPRAuditLogger) LogDataProcessingActivity(_ context.Context, activity *DataProcessingLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.processingLogs = append(s.processingLogs, activity)
	return nil
}

func (s *stubGDPRAuditLogger) LogCrossBorderTransfer(_ context.Context, transfer *CrossBorderTransferLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.crossBorderLogs = append(s.crossBorderLogs, transfer)
	return nil
}

func (s *stubGDPRAuditLogger) LogPrivacyBreach(_ context.Context, _ *PrivacyBreachLog) error {
	return nil
}

type stubDataSubjectRights struct {
	lastRequestType string
}

func (s *stubDataSubjectRights) HandleAccessRequest(_ context.Context, request *DataAccessRequest) (*DataAccessResponse, error) {
	s.lastRequestType = request.RequestType
	return &DataAccessResponse{
		RequestID: request.ID,
		Status:    "received",
	}, nil
}

func (s *stubDataSubjectRights) HandlePortabilityRequest(_ context.Context, request *DataPortabilityRequest) (*DataPortabilityResponse, error) {
	s.lastRequestType = request.RequestType
	return &DataPortabilityResponse{
		RequestID:      request.ID,
		Format:         request.Format,
		TransferMethod: "secure-download",
		StructuredData: request.StructuredData,
		ResponseDate:   time.Now(),
		Metadata:       map[string]any{"status": "received"},
	}, nil
}

func (s *stubDataSubjectRights) HandleErasureRequest(_ context.Context, request *DataErasureRequest) (*DataErasureResponse, error) {
	s.lastRequestType = request.RequestType
	return &DataErasureResponse{
		RequestID:    request.ID,
		Status:       "received",
		ResponseDate: time.Now(),
	}, nil
}

func (s *stubDataSubjectRights) HandleRectificationRequest(_ context.Context, request *DataRectificationRequest) (*DataRectificationResponse, error) {
	s.lastRequestType = request.RequestType
	return &DataRectificationResponse{
		RequestID:     request.ID,
		ResponseDate:  time.Now(),
		RectifiedData: request.CorrectedData,
		Metadata:      map[string]any{"status": "received"},
	}, nil
}

func (s *stubDataSubjectRights) HandleObjectionRequest(_ context.Context, request *DataObjectionRequest) (*DataObjectionResponse, error) {
	s.lastRequestType = request.RequestType
	return &DataObjectionResponse{
		RequestID:          request.ID,
		ResponseDate:       time.Now(),
		ProcessingStopped:  true,
		Metadata:           map[string]any{"status": "received"},
		LegalJustification: request.LegalGrounds,
	}, nil
}

func (s *stubDataSubjectRights) GetRequestStatus(_ context.Context, _ string) (*RequestStatus, error) {
	return &RequestStatus{Status: "processed"}, nil
}

func TestGDPRConsentManager_RecordConsentSetsExpiryAndLogs(t *testing.T) {
	manager := &GDPRConsentManager{
		config: GDPRConsentConfig{
			Enabled:                  true,
			ConsentExpiryDays:        30,
			ConsentWithdrawalEnabled: true,
			GranularConsentRequired:  true,
			ConsentProofRequired:     true,
		},
	}

	store := newMemoryConsentStore()
	manager.SetConsentStore(store)

	audit := &stubGDPRAuditLogger{}
	manager.SetAuditLogger(audit)

	now := time.Now()
	consent := &ConsentRecord{
		ID:                 "consent-1",
		DataSubjectID:      "user-1",
		ConsentMethod:      "web_form",
		LegalBasis:         "consent",
		ProcessingPurposes: []string{"analytics"},
		ConsentProof: &ConsentProof{
			Timestamp: now,
			Type:      "digital",
			Metadata:  map[string]any{},
		},
		Granular:     true,
		Specific:     true,
		Informed:     true,
		Unambiguous:  true,
		ConsentGiven: true,
	}

	err := manager.RecordConsent(context.Background(), consent)
	require.NoError(t, err)

	require.NotNil(t, consent.ExpiryDate)
	expectedExpiry := time.Now().AddDate(0, 0, 30)
	assert.WithinDuration(t, expectedExpiry, *consent.ExpiryDate, time.Hour)

	stored, err := store.GetConsent(context.Background(), "user-1", "analytics")
	require.NoError(t, err)
	assert.Equal(t, "consent-1", stored.ID)

	require.Len(t, audit.consentEvents, 1)
	assert.Equal(t, "consent_recorded", audit.consentEvents[0].EventType)
}

func TestGDPRConsentManager_WithdrawConsentUpdatesStoreAndLogs(t *testing.T) {
	manager := &GDPRConsentManager{
		config: GDPRConsentConfig{
			Enabled:                  true,
			ConsentWithdrawalEnabled: true,
			GranularConsentRequired:  true,
			ConsentProofRequired:     true,
		},
	}
	store := newMemoryConsentStore()
	manager.SetConsentStore(store)

	audit := &stubGDPRAuditLogger{}
	manager.SetAuditLogger(audit)

	record := &ConsentRecord{
		ID:                 "consent-2",
		DataSubjectID:      "user-2",
		ConsentMethod:      "web",
		LegalBasis:         "consent",
		ProcessingPurposes: []string{"marketing"},
		ConsentProof: &ConsentProof{
			Timestamp: time.Now(),
			Type:      "digital",
			Metadata:  map[string]any{},
		},
		Granular:    true,
		Specific:    true,
		Informed:    true,
		Unambiguous: true,
	}
	require.NoError(t, store.StoreConsent(context.Background(), record))

	withdrawal := &ConsentWithdrawal{
		WithdrawalDate:   time.Now(),
		WithdrawalMethod: "portal",
		RequestedBy:      "user-2",
	}

	err := manager.WithdrawConsent(context.Background(), "consent-2", withdrawal)
	require.NoError(t, err)

	stored, err := store.GetConsent(context.Background(), "user-2", "marketing")
	require.NoError(t, err)
	assert.Equal(t, "withdrawn", stored.Status)
	assert.NotNil(t, stored.WithdrawalDate)

	require.Len(t, audit.consentEvents, 1)
	assert.Equal(t, "consent_withdrawn", audit.consentEvents[0].EventType)
}

func TestGDPRConsentManager_ProcessDataSubjectRequestRoutesHandler(t *testing.T) {
	manager := &GDPRConsentManager{
		config: GDPRConsentConfig{Enabled: true},
	}

	rights := &stubDataSubjectRights{}
	manager.SetDataSubjectRightsHandler(rights)

	audit := &stubGDPRAuditLogger{}
	manager.SetAuditLogger(audit)

	request := &DataAccessRequest{
		ID:            "req-1",
		DataSubjectID: "user-3",
		RequestType:   "access",
	}

	err := manager.ProcessDataSubjectRequest(context.Background(), request)
	require.NoError(t, err)

	assert.Equal(t, "access", rights.lastRequestType)
	require.Len(t, audit.requestLogs, 1)
	assert.Equal(t, "req-1", audit.requestLogs[0].RequestID)
}
