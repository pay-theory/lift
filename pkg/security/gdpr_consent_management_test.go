package security

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock implementations for testing

// MockConsentStore implements ConsentStore interface for testing
type MockConsentStore struct {
	mock.Mock
}

func (m *MockConsentStore) RecordConsent(ctx context.Context, consent *ConsentRecord) error {
	args := m.Called(ctx, consent)
	return args.Error(0)
}

func (m *MockConsentStore) StoreConsent(ctx context.Context, consent *ConsentRecord) error {
	args := m.Called(ctx, consent)
	return args.Error(0)
}

func (m *MockConsentStore) GetConsent(ctx context.Context, dataSubjectID, purpose string) (*ConsentRecord, error) {
	args := m.Called(ctx, dataSubjectID, purpose)
	return args.Get(0).(*ConsentRecord), args.Error(1)
}

func (m *MockConsentStore) GetAllConsents(ctx context.Context, dataSubjectID string) ([]*ConsentRecord, error) {
	args := m.Called(ctx, dataSubjectID)
	return args.Get(0).([]*ConsentRecord), args.Error(1)
}

func (m *MockConsentStore) UpdateConsent(ctx context.Context, consentID string, updates *ConsentUpdates) error {
	args := m.Called(ctx, consentID, updates)
	return args.Error(0)
}

func (m *MockConsentStore) WithdrawConsent(ctx context.Context, consentID string, withdrawal *ConsentWithdrawal) error {
	args := m.Called(ctx, consentID, withdrawal)
	return args.Error(0)
}

func (m *MockConsentStore) GetExpiredConsents(ctx context.Context) ([]*ConsentRecord, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*ConsentRecord), args.Error(1)
}

func (m *MockConsentStore) GetConsentsForRenewal(ctx context.Context) ([]*ConsentRecord, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*ConsentRecord), args.Error(1)
}

func (m *MockConsentStore) ListConsents(ctx context.Context, dataSubjectID string) ([]*ConsentRecord, error) {
	args := m.Called(ctx, dataSubjectID)
	return args.Get(0).([]*ConsentRecord), args.Error(1)
}

func (m *MockConsentStore) GetConsentHistory(ctx context.Context, consentID string) ([]*ConsentHistoryEntry, error) {
	args := m.Called(ctx, consentID)
	return args.Get(0).([]*ConsentHistoryEntry), args.Error(1)
}

func (m *MockConsentStore) CleanupExpiredConsents(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockDataSubjectRightsHandler implements DataSubjectRightsHandler interface for testing
type MockDataSubjectRightsHandler struct {
	mock.Mock
}

func (m *MockDataSubjectRightsHandler) HandleAccessRequest(ctx context.Context, request *DataAccessRequest) (*DataAccessResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(*DataAccessResponse), args.Error(1)
}

func (m *MockDataSubjectRightsHandler) HandlePortabilityRequest(ctx context.Context, request *DataPortabilityRequest) (*DataPortabilityResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(*DataPortabilityResponse), args.Error(1)
}

func (m *MockDataSubjectRightsHandler) HandleErasureRequest(ctx context.Context, request *DataErasureRequest) (*DataErasureResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(*DataErasureResponse), args.Error(1)
}

func (m *MockDataSubjectRightsHandler) HandleRectificationRequest(ctx context.Context, request *DataRectificationRequest) (*DataRectificationResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(*DataRectificationResponse), args.Error(1)
}

func (m *MockDataSubjectRightsHandler) HandleObjectionRequest(ctx context.Context, request *DataObjectionRequest) (*DataObjectionResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(*DataObjectionResponse), args.Error(1)
}

func (m *MockDataSubjectRightsHandler) GetRequestStatus(ctx context.Context, requestID string) (*RequestStatus, error) {
	args := m.Called(ctx, requestID)
	return args.Get(0).(*RequestStatus), args.Error(1)
}

// MockPrivacyImpactAssessment implements PrivacyImpactAssessment interface for testing
type MockPrivacyImpactAssessment struct {
	mock.Mock
}

func (m *MockPrivacyImpactAssessment) ConductPIA(ctx context.Context, request *PIARequest) (*PIAResult, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(*PIAResult), args.Error(1)
}

func (m *MockPrivacyImpactAssessment) UpdatePIA(ctx context.Context, piaID string, updates *PIAUpdate) error {
	args := m.Called(ctx, piaID, updates)
	return args.Error(0)
}

func (m *MockPrivacyImpactAssessment) GetPIA(ctx context.Context, piaID string) (*PIAResult, error) {
	args := m.Called(ctx, piaID)
	return args.Get(0).(*PIAResult), args.Error(1)
}

func (m *MockPrivacyImpactAssessment) ListPIAs(ctx context.Context, filters *PIAFilters) ([]*PIAResult, error) {
	args := m.Called(ctx, filters)
	return args.Get(0).([]*PIAResult), args.Error(1)
}

func (m *MockPrivacyImpactAssessment) GetPIATemplate(processingType string) (*PIATemplate, error) {
	args := m.Called(processingType)
	return args.Get(0).(*PIATemplate), args.Error(1)
}

func (m *MockPrivacyImpactAssessment) ValidateDataProcessing(ctx context.Context, processing *DataProcessingActivity) (*ProcessingValidation, error) {
	args := m.Called(ctx, processing)
	return args.Get(0).(*ProcessingValidation), args.Error(1)
}

func (m *MockPrivacyImpactAssessment) GetRiskAssessment(ctx context.Context, activityID string) (*RiskAssessment, error) {
	args := m.Called(ctx, activityID)
	return args.Get(0).(*RiskAssessment), args.Error(1)
}

// Test helper functions

func createTestConsentRecord() *ConsentRecord {
	now := time.Now()
	expiryDate := now.Add(365 * 24 * time.Hour)
	return &ConsentRecord{
		ID:                 "consent-123",
		DataSubjectID:      "user-456",
		DataSubjectEmail:   "user@example.com",
		ConsentVersion:     "1.0",
		ConsentDate:        now,
		ConsentMethod:      "explicit",
		LegalBasis:         "consent",
		ProcessingPurposes: []string{"marketing"},
		DataCategories:     []string{"contact_info", "preferences"},
		ExpiryDate:         &expiryDate,
		ConsentProof: &ConsentProof{
			Type:      "digital_signature",
			Method:    "web_form",
			Evidence:  "test-signature",
			Timestamp: now,
			IPAddress: "192.168.1.1",
			UserAgent: "Mozilla/5.0",
			Verified:  true,
			Metadata: map[string]interface{}{
				"form_id": "consent-form-v1",
			},
		},
		Status:      "active",
		Granular:    true,
		Specific:    true,
		Informed:    true,
		Unambiguous: true,
		Metadata: map[string]interface{}{
			"campaign_id": "summer-2024",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func createTestGDPRManager() *GDPRConsentManager {
	config := GDPRConsentConfig{
		Enabled:                  true,
		ConsentExpiryDays:        365,
		GranularConsentRequired:  true,
		ConsentProofRequired:     true,
		ConsentWithdrawalEnabled: true,
		DataPortabilityEnabled:   true,
		RightToErasureEnabled:    true,
		BreachNotificationHours:  72,
		PrivacyByDesignEnabled:   true,
	}

	return NewGDPRConsentManager(config)
}

// Unit Tests

func TestGDPRConsentManager_RecordConsent(t *testing.T) {
	tests := []struct {
		name          string
		consent       *ConsentRecord
		setupMock     func(*MockConsentStore)
		expectedError string
	}{
		{
			name:    "successful consent recording",
			consent: createTestConsentRecord(),
			setupMock: func(m *MockConsentStore) {
				m.On("RecordConsent", mock.Anything, mock.Anything).Return(nil)
			},
			expectedError: "",
		},
		{
			name: "missing data subject ID",
			consent: &ConsentRecord{
				ProcessingPurposes: []string{"marketing"},
				LegalBasis:         "consent",
			},
			setupMock:     func(m *MockConsentStore) {},
			expectedError: "data subject ID is required",
		},
		{
			name: "missing processing purposes",
			consent: &ConsentRecord{
				DataSubjectID: "user-456",
				LegalBasis:    "consent",
			},
			setupMock:     func(m *MockConsentStore) {},
			expectedError: "processing purposes are required",
		},
		{
			name:    "store error",
			consent: createTestConsentRecord(),
			setupMock: func(m *MockConsentStore) {
				m.On("RecordConsent", mock.Anything, mock.Anything).Return(assert.AnError)
			},
			expectedError: "failed to record consent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &MockConsentStore{}
			tt.setupMock(mockStore)

			manager := createTestGDPRManager()
			manager.SetConsentStore(mockStore)

			err := manager.RecordConsent(context.Background(), tt.consent)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			mockStore.AssertExpectations(t)
		})
	}
}

func TestGDPRConsentManager_GetConsent(t *testing.T) {
	tests := []struct {
		name          string
		dataSubjectID string
		purpose       string
		setupMock     func(*MockConsentStore)
		expectedError string
	}{
		{
			name:          "successful consent retrieval",
			dataSubjectID: "user-456",
			purpose:       "marketing",
			setupMock: func(m *MockConsentStore) {
				consent := createTestConsentRecord()
				m.On("GetConsent", mock.Anything, "user-456", "marketing").Return(consent, nil)
			},
			expectedError: "",
		},
		{
			name:          "consent not found",
			dataSubjectID: "user-456",
			purpose:       "analytics",
			setupMock: func(m *MockConsentStore) {
				m.On("GetConsent", mock.Anything, "user-456", "analytics").Return((*ConsentRecord)(nil), ErrConsentNotFound)
			},
			expectedError: "consent not found",
		},
		{
			name:          "empty data subject ID",
			dataSubjectID: "",
			purpose:       "marketing",
			setupMock:     func(m *MockConsentStore) {},
			expectedError: "data subject ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &MockConsentStore{}
			tt.setupMock(mockStore)

			manager := createTestGDPRManager()
			manager.SetConsentStore(mockStore)

			consent, err := manager.GetConsent(context.Background(), tt.dataSubjectID, tt.purpose)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, consent)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, consent)
			}

			mockStore.AssertExpectations(t)
		})
	}
}

func TestGDPRConsentManager_WithdrawConsent(t *testing.T) {
	tests := []struct {
		name          string
		consentID     string
		withdrawal    *ConsentWithdrawal
		setupMock     func(*MockConsentStore)
		expectedError string
	}{
		{
			name:      "successful consent withdrawal",
			consentID: "consent-123",
			withdrawal: &ConsentWithdrawal{
				Reason:           "user_request",
				WithdrawalDate:   time.Now(),
				WithdrawalMethod: "web_form",
				RequestedBy:      "user-456",
				Verified:         true,
			},
			setupMock: func(m *MockConsentStore) {
				m.On("WithdrawConsent", mock.Anything, "consent-123", mock.Anything).Return(nil)
			},
			expectedError: "",
		},
		{
			name:      "empty consent ID",
			consentID: "",
			withdrawal: &ConsentWithdrawal{
				Reason: "user_request",
			},
			setupMock:     func(m *MockConsentStore) {},
			expectedError: "consent ID is required",
		},
		{
			name:          "nil withdrawal",
			consentID:     "consent-123",
			withdrawal:    nil,
			setupMock:     func(m *MockConsentStore) {},
			expectedError: "withdrawal information is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &MockConsentStore{}
			tt.setupMock(mockStore)

			manager := createTestGDPRManager()
			manager.SetConsentStore(mockStore)

			err := manager.WithdrawConsent(context.Background(), tt.consentID, tt.withdrawal)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			mockStore.AssertExpectations(t)
		})
	}
}

func TestGDPRConsentManager_HandleAccessRequest(t *testing.T) {
	tests := []struct {
		name          string
		request       *DataAccessRequest
		setupMock     func(*MockDataSubjectRightsHandler)
		expectedError string
	}{
		{
			name: "successful access request",
			request: &DataAccessRequest{
				ID:            "req-123",
				DataSubjectID: "user-456",
				RequestType:   "access",
				Timestamp:     time.Now(),
				ContactInfo:   "user@example.com",
			},
			setupMock: func(m *MockDataSubjectRightsHandler) {
				response := &DataAccessResponse{
					RequestID: "req-123",
					Status:    "completed",
					Data:      map[string]interface{}{"name": "John Doe"},
				}
				m.On("HandleAccessRequest", mock.Anything, mock.Anything).Return(response, nil)
			},
			expectedError: "",
		},
		{
			name: "missing data subject ID",
			request: &DataAccessRequest{
				ID:          "req-123",
				RequestType: "access",
			},
			setupMock:     func(m *MockDataSubjectRightsHandler) {},
			expectedError: "data subject ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockHandler := &MockDataSubjectRightsHandler{}
			tt.setupMock(mockHandler)

			manager := createTestGDPRManager()
			manager.SetDataSubjectRightsHandler(mockHandler)

			response, err := manager.HandleAccessRequest(context.Background(), tt.request)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, response)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, response)
			}

			mockHandler.AssertExpectations(t)
		})
	}
}

func TestGDPRConsentManager_ConductPIA(t *testing.T) {
	tests := []struct {
		name          string
		request       *PIARequest
		setupMock     func(*MockPrivacyImpactAssessment)
		expectedError string
	}{
		{
			name: "successful PIA",
			request: &PIARequest{
				ID:          "pia-123",
				ProjectName: "New Marketing Campaign",
				DataTypes:   []string{"email", "preferences"},
				Purpose:     "marketing",
				LegalBasis:  "consent",
			},
			setupMock: func(m *MockPrivacyImpactAssessment) {
				result := &PIAResult{
					ID:        "pia-123",
					RiskScore: 0.3,
					RiskLevel: "low",
					Status:    "approved",
					Timestamp: time.Now(),
				}
				m.On("ConductPIA", mock.Anything, mock.Anything).Return(result, nil)
			},
			expectedError: "",
		},
		{
			name: "missing project name",
			request: &PIARequest{
				ID:        "pia-123",
				DataTypes: []string{"email"},
				Purpose:   "marketing",
			},
			setupMock:     func(m *MockPrivacyImpactAssessment) {},
			expectedError: "project name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPIA := &MockPrivacyImpactAssessment{}
			tt.setupMock(mockPIA)

			manager := createTestGDPRManager()
			manager.SetPrivacyImpactAssessment(mockPIA)

			result, err := manager.ConductPIA(context.Background(), tt.request)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}

			mockPIA.AssertExpectations(t)
		})
	}
}

// Integration Tests

func TestGDPRConsentManager_Integration_ConsentLifecycle(t *testing.T) {
	// This test verifies the complete consent lifecycle
	mockStore := &MockConsentStore{}
	manager := createTestGDPRManager()
	manager.SetConsentStore(mockStore)

	ctx := context.Background()
	consent := createTestConsentRecord()

	// Record consent
	mockStore.On("RecordConsent", ctx, consent).Return(nil)
	err := manager.RecordConsent(ctx, consent)
	require.NoError(t, err)

	// Get consent
	mockStore.On("GetConsent", ctx, consent.DataSubjectID, consent.Purpose).Return(consent, nil)
	retrievedConsent, err := manager.GetConsent(ctx, consent.DataSubjectID, consent.Purpose)
	require.NoError(t, err)
	assert.Equal(t, consent.ID, retrievedConsent.ID)

	// Update consent
	update := &ConsentUpdate{
		ConsentGiven: false,
		Timestamp:    time.Now(),
		Reason:       "user_preference_change",
	}
	mockStore.On("UpdateConsent", ctx, consent.ID, update).Return(nil)
	err = manager.UpdateConsent(ctx, consent.ID, update)
	require.NoError(t, err)

	// Withdraw consent
	withdrawal := &ConsentWithdrawal{
		Reason:    "user_request",
		Timestamp: time.Now(),
		Method:    "web_form",
	}
	mockStore.On("WithdrawConsent", ctx, consent.ID, withdrawal).Return(nil)
	err = manager.WithdrawConsent(ctx, consent.ID, withdrawal)
	require.NoError(t, err)

	mockStore.AssertExpectations(t)
}

func TestGDPRConsentManager_Integration_DataSubjectRights(t *testing.T) {
	// This test verifies data subject rights handling
	mockHandler := &MockDataSubjectRightsHandler{}
	manager := createTestGDPRManager()
	manager.SetDataSubjectRightsHandler(mockHandler)

	ctx := context.Background()
	dataSubjectID := "user-456"

	// Access request
	accessRequest := &DataAccessRequest{
		ID:            "access-123",
		DataSubjectID: dataSubjectID,
		RequestType:   "access",
		Timestamp:     time.Now(),
		ContactInfo:   "user@example.com",
	}
	accessResponse := &DataAccessResponse{
		RequestID: "access-123",
		Status:    "completed",
		Data:      map[string]interface{}{"name": "John Doe"},
	}
	mockHandler.On("HandleAccessRequest", ctx, accessRequest).Return(accessResponse, nil)

	response, err := manager.HandleAccessRequest(ctx, accessRequest)
	require.NoError(t, err)
	assert.Equal(t, "completed", response.Status)

	// Erasure request
	erasureRequest := &DataErasureRequest{
		DataAccessRequest: DataAccessRequest{
			ID:            "erasure-123",
			DataSubjectID: dataSubjectID,
			RequestType:   "erasure",
			Timestamp:     time.Now(),
		},
		Reason: "withdrawal_of_consent",
	}
	erasureResponse := &DataErasureResponse{
		RequestID:    "erasure-123",
		Status:       "completed",
		DataDeleted:  []string{"profile", "preferences"},
		DeletedCount: 2,
	}
	mockHandler.On("HandleErasureRequest", ctx, erasureRequest).Return(erasureResponse, nil)

	erasureResp, err := manager.HandleErasureRequest(ctx, erasureRequest)
	require.NoError(t, err)
	assert.Equal(t, "completed", erasureResp.Status)
	assert.Equal(t, 2, erasureResp.DeletedCount)

	mockHandler.AssertExpectations(t)
}

// Performance Tests

func BenchmarkGDPRConsentManager_RecordConsent(b *testing.B) {
	mockStore := &MockConsentStore{}
	manager := createTestGDPRManager()
	manager.SetConsentStore(mockStore)

	mockStore.On("RecordConsent", mock.Anything, mock.Anything).Return(nil)

	ctx := context.Background()
	consent := createTestConsentRecord()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		consent.ID = fmt.Sprintf("consent-%d", i)
		manager.RecordConsent(ctx, consent)
	}
}

func BenchmarkGDPRConsentManager_GetConsent(b *testing.B) {
	mockStore := &MockConsentStore{}
	manager := createTestGDPRManager()
	manager.SetConsentStore(mockStore)

	consent := createTestConsentRecord()
	mockStore.On("GetConsent", mock.Anything, mock.Anything, mock.Anything).Return(consent, nil)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.GetConsent(ctx, "user-456", "marketing")
	}
}

func BenchmarkGDPRConsentManager_HandleAccessRequest(b *testing.B) {
	mockHandler := &MockDataSubjectRightsHandler{}
	manager := createTestGDPRManager()
	manager.SetDataSubjectRightsHandler(mockHandler)

	response := &DataAccessResponse{
		RequestID: "req-123",
		Status:    "completed",
		Data:      map[string]interface{}{"name": "John Doe"},
	}
	mockHandler.On("HandleAccessRequest", mock.Anything, mock.Anything).Return(response, nil)

	ctx := context.Background()
	request := &DataAccessRequest{
		ID:            "req-123",
		DataSubjectID: "user-456",
		RequestType:   "access",
		Timestamp:     time.Now(),
		ContactInfo:   "user@example.com",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		request.ID = fmt.Sprintf("req-%d", i)
		manager.HandleAccessRequest(ctx, request)
	}
}

// Concurrent Tests

func TestGDPRConsentManager_ConcurrentOperations(t *testing.T) {
	mockStore := &MockConsentStore{}
	manager := createTestGDPRManager()
	manager.SetConsentStore(mockStore)

	// Setup mocks for concurrent operations
	mockStore.On("RecordConsent", mock.Anything, mock.Anything).Return(nil)
	mockStore.On("GetConsent", mock.Anything, mock.Anything, mock.Anything).Return(createTestConsentRecord(), nil)
	mockStore.On("UpdateConsent", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	ctx := context.Background()
	numGoroutines := 10
	numOperations := 100

	// Test concurrent consent recording
	t.Run("concurrent_record_consent", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					consent := createTestConsentRecord()
					consent.ID = fmt.Sprintf("consent-%d-%d", id, j)
					err := manager.RecordConsent(ctx, consent)
					assert.NoError(t, err)
				}
			}(i)
		}

		wg.Wait()
	})

	// Test concurrent consent retrieval
	t.Run("concurrent_get_consent", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					_, err := manager.GetConsent(ctx, fmt.Sprintf("user-%d", id), "marketing")
					assert.NoError(t, err)
				}
			}(i)
		}

		wg.Wait()
	})
}

// Error Handling Tests

func TestGDPRConsentManager_ErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		setupManager  func() *GDPRConsentManager
		operation     func(*GDPRConsentManager) error
		expectedError string
	}{
		{
			name: "consent store not set",
			setupManager: func() *GDPRConsentManager {
				return createTestGDPRManager()
			},
			operation: func(m *GDPRConsentManager) error {
				return m.RecordConsent(context.Background(), createTestConsentRecord())
			},
			expectedError: "consent store not configured",
		},
		{
			name: "data subject rights handler not set",
			setupManager: func() *GDPRConsentManager {
				return createTestGDPRManager()
			},
			operation: func(m *GDPRConsentManager) error {
				request := &DataAccessRequest{
					ID:            "req-123",
					DataSubjectID: "user-456",
					RequestType:   "access",
				}
				_, err := m.HandleAccessRequest(context.Background(), request)
				return err
			},
			expectedError: "data subject rights handler not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := tt.setupManager()
			err := tt.operation(manager)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

// Validation Tests

func TestGDPRConsentManager_Validation(t *testing.T) {
	manager := createTestGDPRManager()

	tests := []struct {
		name          string
		consent       *ConsentRecord
		expectedError string
	}{
		{
			name:          "nil consent",
			consent:       nil,
			expectedError: "consent record is required",
		},
		{
			name: "empty data subject ID",
			consent: &ConsentRecord{
				Purpose:      "marketing",
				ConsentGiven: true,
			},
			expectedError: "data subject ID is required",
		},
		{
			name: "empty purpose",
			consent: &ConsentRecord{
				DataSubjectID: "user-456",
				ConsentGiven:  true,
			},
			expectedError: "purpose is required",
		},
		{
			name: "invalid legal basis",
			consent: &ConsentRecord{
				DataSubjectID: "user-456",
				Purpose:       "marketing",
				LegalBasis:    "invalid",
				ConsentGiven:  true,
			},
			expectedError: "invalid legal basis",
		},
		{
			name: "expired consent",
			consent: &ConsentRecord{
				DataSubjectID: "user-456",
				Purpose:       "marketing",
				LegalBasis:    "consent",
				ConsentGiven:  true,
				ExpiryDate:    &[]time.Time{time.Now().Add(-24 * time.Hour)}[0],
			},
			expectedError: "consent has expired",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.validateConsentRecord(tt.consent)
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test utilities

func TestGDPRConsentManager_Utilities(t *testing.T) {
	manager := createTestGDPRManager()

	t.Run("generate_consent_id", func(t *testing.T) {
		id1 := manager.generateConsentID()
		id2 := manager.generateConsentID()

		assert.NotEmpty(t, id1)
		assert.NotEmpty(t, id2)
		assert.NotEqual(t, id1, id2)
	})

	t.Run("validate_email", func(t *testing.T) {
		validEmails := []string{
			"user@example.com",
			"test.email+tag@domain.co.uk",
			"user123@test-domain.org",
		}

		invalidEmails := []string{
			"invalid-email",
			"@domain.com",
			"user@",
			"",
		}

		for _, email := range validEmails {
			assert.True(t, manager.isValidEmail(email), "Expected %s to be valid", email)
		}

		for _, email := range invalidEmails {
			assert.False(t, manager.isValidEmail(email), "Expected %s to be invalid", email)
		}
	})

	t.Run("calculate_expiry_date", func(t *testing.T) {
		now := time.Now()
		expiry := manager.calculateExpiryDate()

		expectedExpiry := now.Add(time.Duration(manager.config.ConsentExpiryDays) * 24 * time.Hour)
		assert.WithinDuration(t, expectedExpiry, expiry, time.Second)
	})
}

// Test configuration validation

func TestGDPRConfig_Validation(t *testing.T) {
	tests := []struct {
		name          string
		config        GDPRConfig
		expectedError string
	}{
		{
			name: "valid config",
			config: GDPRConfig{
				Enabled:                 true,
				BreachNotificationHours: 72,
			},
			expectedError: "",
		},
		{
			name: "invalid breach notification hours",
			config: GDPRConfig{
				Enabled:                 true,
				BreachNotificationHours: 0,
			},
			expectedError: "breach notification hours must be positive",
		},
		{
			name: "duplicate breach notification test",
			config: GDPRConfig{
				Enabled:                 true,
				BreachNotificationHours: 0,
			},
			expectedError: "breach notification hours must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGDPRConfig(tt.config)
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Helper function for config validation (would be implemented in the main file)
func validateGDPRConfig(config GDPRConfig) error {
	if config.BreachNotificationHours <= 0 {
		return fmt.Errorf("breach notification hours must be positive")
	}
	return nil
}

// Test data generators for load testing

func generateTestConsents(count int) []*ConsentRecord {
	consents := make([]*ConsentRecord, count)
	purposes := []string{"marketing", "analytics", "personalization", "communication"}

	for i := 0; i < count; i++ {
		timestamp := time.Now().Add(-time.Duration(i) * time.Minute)
		expiryDate := time.Now().Add(365 * 24 * time.Hour)
		consents[i] = &ConsentRecord{
			ID:            fmt.Sprintf("consent-%d", i),
			DataSubjectID: fmt.Sprintf("user-%d", i%1000), // 1000 unique users
			Purpose:       purposes[i%len(purposes)],
			LegalBasis:    "consent",
			ConsentGiven:  i%10 != 0, // 90% consent given
			Timestamp:     &timestamp,
			ExpiryDate:    &expiryDate,
			Source:        "web_form",
			IPAddress:     fmt.Sprintf("192.168.1.%d", i%255),
			UserAgent:     "Mozilla/5.0",
			ConsentProof: &ConsentProof{
				Method:    "digital_signature",
				Signature: fmt.Sprintf("signature-%d", i),
				Timestamp: time.Now(),
			},
		}
	}

	return consents
}

// Load test

func TestGDPRConsentManager_LoadTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	mockStore := &MockConsentStore{}
	manager := createTestGDPRManager()
	manager.SetConsentStore(mockStore)

	// Setup mocks for load test
	mockStore.On("RecordConsent", mock.Anything, mock.Anything).Return(nil)
	mockStore.On("GetConsent", mock.Anything, mock.Anything, mock.Anything).Return(createTestConsentRecord(), nil)

	ctx := context.Background()
	consents := generateTestConsents(10000)

	start := time.Now()

	// Record all consents
	for _, consent := range consents {
		err := manager.RecordConsent(ctx, consent)
		require.NoError(t, err)
	}

	duration := time.Since(start)
	throughput := float64(len(consents)) / duration.Seconds()

	t.Logf("Recorded %d consents in %v (%.2f consents/sec)", len(consents), duration, throughput)

	// Verify minimum throughput (should handle at least 1000 consents/sec)
	assert.Greater(t, throughput, 1000.0, "Throughput should be at least 1000 consents/sec")
}
