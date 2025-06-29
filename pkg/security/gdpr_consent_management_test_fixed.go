package security

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock implementations for testing

// MockConsentStore implements ConsentStore interface for testing
type MockConsentStoreFixed struct {
	mock.Mock
}

func (m *MockConsentStoreFixed) RecordConsent(ctx context.Context, consent *ConsentRecord) error {
	args := m.Called(ctx, consent)
	return args.Error(0)
}

func (m *MockConsentStoreFixed) StoreConsent(ctx context.Context, consent *ConsentRecord) error {
	args := m.Called(ctx, consent)
	return args.Error(0)
}

func (m *MockConsentStoreFixed) GetConsent(ctx context.Context, dataSubjectID, purpose string) (*ConsentRecord, error) {
	args := m.Called(ctx, dataSubjectID, purpose)
	return args.Get(0).(*ConsentRecord), args.Error(1)
}

func (m *MockConsentStoreFixed) GetAllConsents(ctx context.Context, dataSubjectID string) ([]*ConsentRecord, error) {
	args := m.Called(ctx, dataSubjectID)
	return args.Get(0).([]*ConsentRecord), args.Error(1)
}

func (m *MockConsentStoreFixed) UpdateConsent(ctx context.Context, consentID string, updates *ConsentUpdates) error {
	args := m.Called(ctx, consentID, updates)
	return args.Error(0)
}

func (m *MockConsentStoreFixed) WithdrawConsent(ctx context.Context, consentID string, withdrawal *ConsentWithdrawal) error {
	args := m.Called(ctx, consentID, withdrawal)
	return args.Error(0)
}

func (m *MockConsentStoreFixed) GetExpiredConsents(ctx context.Context) ([]*ConsentRecord, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*ConsentRecord), args.Error(1)
}

func (m *MockConsentStoreFixed) GetConsentsForRenewal(ctx context.Context) ([]*ConsentRecord, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*ConsentRecord), args.Error(1)
}

func (m *MockConsentStoreFixed) ListConsents(ctx context.Context, dataSubjectID string) ([]*ConsentRecord, error) {
	args := m.Called(ctx, dataSubjectID)
	return args.Get(0).([]*ConsentRecord), args.Error(1)
}

func (m *MockConsentStoreFixed) GetConsentHistory(ctx context.Context, consentID string) ([]*ConsentHistoryEntry, error) {
	args := m.Called(ctx, consentID)
	return args.Get(0).([]*ConsentHistoryEntry), args.Error(1)
}

func (m *MockConsentStoreFixed) CleanupExpiredConsents(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// Test helper functions

func createTestConsentRecordFixed() *ConsentRecord {
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
			Metadata: map[string]any{
				"form_id": "consent-form-v1",
			},
		},
		Status:      "active",
		Granular:    true,
		Specific:    true,
		Informed:    true,
		Unambiguous: true,
		Metadata: map[string]any{
			"campaign_id": "summer-2024",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func createTestGDPRManagerFixed() *GDPRConsentManager {
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

func TestGDPRConsentManager_RecordConsent_Fixed(t *testing.T) {
	tests := []struct {
		name          string
		consent       *ConsentRecord
		setupMock     func(*MockConsentStoreFixed)
		expectedError string
	}{
		{
			name:    "successful consent recording",
			consent: createTestConsentRecordFixed(),
			setupMock: func(m *MockConsentStoreFixed) {
				m.On("StoreConsent", mock.Anything, mock.Anything).Return(nil)
			},
			expectedError: "",
		},
		{
			name: "missing data subject ID",
			consent: &ConsentRecord{
				ProcessingPurposes: []string{"marketing"},
				LegalBasis:         "consent",
				Specific:           true,
				Informed:           true,
				Unambiguous:        true,
			},
			setupMock:     func(m *MockConsentStoreFixed) {},
			expectedError: "data subject ID is required",
		},
		{
			name: "missing processing purposes",
			consent: &ConsentRecord{
				DataSubjectID: "user-456",
				LegalBasis:    "consent",
				Specific:      true,
				Informed:      true,
				Unambiguous:   true,
			},
			setupMock:     func(m *MockConsentStoreFixed) {},
			expectedError: "processing purposes are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &MockConsentStoreFixed{}
			tt.setupMock(mockStore)

			manager := createTestGDPRManagerFixed()
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

func TestGDPRConsentManager_GetConsent_Fixed(t *testing.T) {
	tests := []struct {
		name          string
		dataSubjectID string
		purpose       string
		setupMock     func(*MockConsentStoreFixed)
		expectedError string
	}{
		{
			name:          "successful consent retrieval",
			dataSubjectID: "user-456",
			purpose:       "marketing",
			setupMock: func(m *MockConsentStoreFixed) {
				consent := createTestConsentRecordFixed()
				m.On("GetConsent", mock.Anything, "user-456", "marketing").Return(consent, nil)
			},
			expectedError: "",
		},
		{
			name:          "consent not found",
			dataSubjectID: "user-456",
			purpose:       "analytics",
			setupMock: func(m *MockConsentStoreFixed) {
				m.On("GetConsent", mock.Anything, "user-456", "analytics").Return((*ConsentRecord)(nil), ErrConsentNotFound)
			},
			expectedError: "consent not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &MockConsentStoreFixed{}
			tt.setupMock(mockStore)

			manager := createTestGDPRManagerFixed()
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

func TestGDPRConsentManager_WithdrawConsent_Fixed(t *testing.T) {
	tests := []struct {
		name          string
		consentID     string
		withdrawal    *ConsentWithdrawal
		setupMock     func(*MockConsentStoreFixed)
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
			setupMock: func(m *MockConsentStoreFixed) {
				m.On("WithdrawConsent", mock.Anything, "consent-123", mock.Anything).Return(nil)
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &MockConsentStoreFixed{}
			tt.setupMock(mockStore)

			manager := createTestGDPRManagerFixed()
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

// Integration test
func TestGDPRConsentManager_Integration_ConsentLifecycle_Fixed(t *testing.T) {
	mockStore := &MockConsentStoreFixed{}
	manager := createTestGDPRManagerFixed()
	manager.SetConsentStore(mockStore)

	ctx := context.Background()
	consent := createTestConsentRecordFixed()

	// Record consent
	mockStore.On("StoreConsent", ctx, consent).Return(nil)
	err := manager.RecordConsent(ctx, consent)
	require.NoError(t, err)

	// Get consent
	mockStore.On("GetConsent", ctx, consent.DataSubjectID, "marketing").Return(consent, nil)
	retrievedConsent, err := manager.GetConsent(ctx, consent.DataSubjectID, "marketing")
	require.NoError(t, err)
	assert.Equal(t, consent.ID, retrievedConsent.ID)

	mockStore.AssertExpectations(t)
}

// Utility tests
func TestGDPRConsentManager_Utilities_Fixed(t *testing.T) {
	manager := createTestGDPRManagerFixed()

	t.Run("generateConsentID", func(t *testing.T) {
		id1 := manager.generateConsentID()
		id2 := manager.generateConsentID()

		assert.NotEmpty(t, id1)
		assert.NotEmpty(t, id2)
		assert.NotEqual(t, id1, id2)
		assert.Contains(t, id1, "consent-")
	})

	t.Run("isValidEmail", func(t *testing.T) {
		assert.True(t, manager.isValidEmail("user@example.com"))
		assert.True(t, manager.isValidEmail("test.email+tag@domain.co.uk"))
		assert.False(t, manager.isValidEmail("invalid-email"))
		assert.False(t, manager.isValidEmail("@domain.com"))
		assert.False(t, manager.isValidEmail("user@"))
	})

	t.Run("calculateExpiryDate", func(t *testing.T) {
		expiryDate := manager.calculateExpiryDate()
		assert.True(t, expiryDate.After(time.Now()))
	})
}
