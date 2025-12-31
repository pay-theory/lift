package security

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingConsentStore struct {
	inner *memoryConsentStore

	storeErr    error
	withdrawErr error
	updateErr   error
}

func (f *failingConsentStore) StoreConsent(ctx context.Context, consent *ConsentRecord) error {
	if f.storeErr != nil {
		return f.storeErr
	}
	return f.inner.StoreConsent(ctx, consent)
}

func (f *failingConsentStore) GetConsent(ctx context.Context, dataSubjectID, purpose string) (*ConsentRecord, error) {
	return f.inner.GetConsent(ctx, dataSubjectID, purpose)
}

func (f *failingConsentStore) GetAllConsents(ctx context.Context, dataSubjectID string) ([]*ConsentRecord, error) {
	return f.inner.GetAllConsents(ctx, dataSubjectID)
}

func (f *failingConsentStore) UpdateConsent(ctx context.Context, consentID string, updates *ConsentUpdates) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	return f.inner.UpdateConsent(ctx, consentID, updates)
}

func (f *failingConsentStore) WithdrawConsent(ctx context.Context, consentID string, withdrawal *ConsentWithdrawal) error {
	if f.withdrawErr != nil {
		return f.withdrawErr
	}
	return f.inner.WithdrawConsent(ctx, consentID, withdrawal)
}

func (f *failingConsentStore) GetExpiredConsents(context.Context) ([]*ConsentRecord, error)         { return nil, nil }
func (f *failingConsentStore) GetConsentsForRenewal(context.Context) ([]*ConsentRecord, error)     { return nil, nil }
func (f *failingConsentStore) RecordConsent(ctx context.Context, consent *ConsentRecord) error     { return f.StoreConsent(ctx, consent) }
func (f *failingConsentStore) ListConsents(ctx context.Context, dataSubjectID string) ([]*ConsentRecord, error) {
	return f.GetAllConsents(ctx, dataSubjectID)
}
func (f *failingConsentStore) GetConsentHistory(context.Context, string) ([]*ConsentHistoryEntry, error) {
	return nil, nil
}
func (f *failingConsentStore) CleanupExpiredConsents(context.Context) error { return nil }

type failingGDPRAuditLogger struct {
	consentErr error
	requestErr error
}

func (f *failingGDPRAuditLogger) LogConsentEvent(context.Context, *ConsentEvent) error          { return f.consentErr }
func (f *failingGDPRAuditLogger) LogDataSubjectRequest(context.Context, *DataSubjectRequestLog) error {
	return f.requestErr
}
func (f *failingGDPRAuditLogger) LogDataProcessingActivity(context.Context, *DataProcessingLog) error {
	return nil
}
func (f *failingGDPRAuditLogger) LogCrossBorderTransfer(context.Context, *CrossBorderTransferLog) error {
	return nil
}
func (f *failingGDPRAuditLogger) LogPrivacyBreach(context.Context, *PrivacyBreachLog) error { return nil }

type stubPIA struct {
	lastRequest *PIARequest
	result      *PIAResult
	err         error
}

func (s *stubPIA) ConductPIA(_ context.Context, assessment *PIARequest) (*PIAResult, error) {
	s.lastRequest = assessment
	return s.result, s.err
}

func (s *stubPIA) GetPIATemplate(string) (*PIATemplate, error)                       { return nil, nil }
func (s *stubPIA) ValidateDataProcessing(context.Context, *DataProcessingActivity) (*ProcessingValidation, error) {
	return nil, nil
}
func (s *stubPIA) GetRiskAssessment(context.Context, string) (*RiskAssessment, error) { return nil, nil }
func (s *stubPIA) UpdatePIA(context.Context, string, *PIAUpdate) error                { return nil }
func (s *stubPIA) GetPIA(context.Context, string) (*PIAResult, error)                 { return nil, nil }
func (s *stubPIA) ListPIAs(context.Context, *PIAFilters) ([]*PIAResult, error)        { return nil, nil }

func TestGDPRConsentManager_ErrorPathsAndRouting(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("record consent requires enabled and store", func(t *testing.T) {
		t.Parallel()

		manager := NewGDPRConsentManager(GDPRConsentConfig{Enabled: false})
		err := manager.RecordConsent(ctx, &ConsentRecord{})
		require.Error(t, err)

		manager = NewGDPRConsentManager(GDPRConsentConfig{Enabled: true})
		err = manager.RecordConsent(ctx, &ConsentRecord{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "consent store not configured")
	})

	t.Run("record consent validates and stores", func(t *testing.T) {
		t.Parallel()

		store := &failingConsentStore{inner: newMemoryConsentStore(), storeErr: errors.New("store failed")}
		manager := NewGDPRConsentManager(GDPRConsentConfig{
			Enabled:                 true,
			ConsentExpiryDays:       1,
			GranularConsentRequired: true,
			ConsentProofRequired:    true,
		})
		manager.SetConsentStore(store)
		manager.SetAuditLogger(&failingGDPRAuditLogger{consentErr: errors.New("audit failed")})

		consent := &ConsentRecord{
			ID:                 "c1",
			DataSubjectID:      "u1",
			ConsentMethod:      "web",
			LegalBasis:         "consent",
			ProcessingPurposes: []string{"analytics"},
			ConsentProof:       &ConsentProof{Timestamp: time.Now(), Type: "digital", Metadata: map[string]any{}},
			Granular:           true,
			Specific:           true,
			Informed:           true,
			Unambiguous:        true,
		}

		err := manager.RecordConsent(ctx, consent)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to store consent")

		store.storeErr = nil
		require.NoError(t, manager.RecordConsent(ctx, consent))
		require.NotNil(t, consent.ExpiryDate)
	})

	t.Run("get consent validates input", func(t *testing.T) {
		t.Parallel()

		manager := NewGDPRConsentManager(GDPRConsentConfig{Enabled: true})
		_, err := manager.GetConsent(ctx, "", "p")
		require.Error(t, err)

		_, err = manager.GetConsent(ctx, "u1", "p")
		require.Error(t, err)
	})

	t.Run("withdraw consent validates and routes", func(t *testing.T) {
		t.Parallel()

		manager := NewGDPRConsentManager(GDPRConsentConfig{Enabled: true, ConsentWithdrawalEnabled: true})
		err := manager.WithdrawConsent(ctx, "", &ConsentWithdrawal{})
		require.Error(t, err)

		err = manager.WithdrawConsent(ctx, "id", nil)
		require.Error(t, err)

		manager = NewGDPRConsentManager(GDPRConsentConfig{Enabled: true, ConsentWithdrawalEnabled: false})
		err = manager.WithdrawConsent(ctx, "id", &ConsentWithdrawal{})
		require.Error(t, err)

		store := &failingConsentStore{inner: newMemoryConsentStore(), withdrawErr: errors.New("withdraw failed")}
		manager = NewGDPRConsentManager(GDPRConsentConfig{Enabled: true, ConsentWithdrawalEnabled: true})
		manager.SetConsentStore(store)
		manager.SetAuditLogger(&failingGDPRAuditLogger{consentErr: errors.New("audit failed")})

		err = manager.WithdrawConsent(ctx, "id", &ConsentWithdrawal{WithdrawalDate: time.Now(), WithdrawalMethod: "portal", RequestedBy: "u"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to withdraw consent")
	})

	t.Run("process data subject request routes all types and validates", func(t *testing.T) {
		t.Parallel()

		manager := NewGDPRConsentManager(GDPRConsentConfig{Enabled: true})
		err := manager.ProcessDataSubjectRequest(ctx, &DataAccessRequest{ID: "r1", RequestType: "access"})
		require.Error(t, err)

		rights := &stubDataSubjectRights{}
		manager.SetDataSubjectRightsHandler(rights)
		manager.SetAuditLogger(&failingGDPRAuditLogger{requestErr: errors.New("audit failed")})

		for _, rt := range []string{"access", "portability", "erasure", "rectification", "objection"} {
			req := &DataAccessRequest{ID: "r-" + rt, DataSubjectID: "u", RequestType: rt}
			require.NoError(t, manager.ProcessDataSubjectRequest(ctx, req))
			assert.Equal(t, rt, rights.lastRequestType)
		}

		err = manager.ProcessDataSubjectRequest(ctx, &DataAccessRequest{ID: "rX", DataSubjectID: "u", RequestType: "unknown"})
		require.Error(t, err)
	})
}

func TestGDPRConsentValidation_ErrorCases(t *testing.T) {
	t.Parallel()

	manager := NewGDPRConsentManager(GDPRConsentConfig{Enabled: true, GranularConsentRequired: true, ConsentProofRequired: true})

	require.Error(t, manager.validateConsent(nil))
	require.Error(t, manager.validateConsent(&ConsentRecord{}))
	require.Error(t, manager.validateConsent(&ConsentRecord{DataSubjectID: "u"}))
	require.Error(t, manager.validateConsent(&ConsentRecord{DataSubjectID: "u", Purpose: "p"}))
	require.Error(t, manager.validateConsent(&ConsentRecord{DataSubjectID: "u", Purpose: "p", LegalBasis: "invalid"}))

	expired := time.Now().Add(-time.Hour)
	require.Error(t, manager.validateConsent(&ConsentRecord{
		DataSubjectID: "u",
		Purpose:       "p",
		LegalBasis:    "consent",
		ExpiryDate:    &expired,
		Granular:      true,
		ConsentProof:  &ConsentProof{Timestamp: time.Now(), Type: "digital", Metadata: map[string]any{}},
		Specific:      true,
		Informed:      true,
		Unambiguous:   true,
	}))

	require.Error(t, manager.validateConsent(&ConsentRecord{
		DataSubjectID: "u",
		Purpose:       "p",
		LegalBasis:    "consent",
		Granular:      false,
		ConsentProof:  &ConsentProof{Timestamp: time.Now(), Type: "digital", Metadata: map[string]any{}},
		Specific:      true,
		Informed:      true,
		Unambiguous:   true,
	}))

	require.Error(t, manager.validateConsent(&ConsentRecord{
		DataSubjectID: "u",
		Purpose:       "p",
		LegalBasis:    "consent",
		Granular:      true,
		ConsentProof:  nil,
		Specific:      true,
		Informed:      true,
		Unambiguous:   true,
	}))

	require.Error(t, manager.validateConsent(&ConsentRecord{
		DataSubjectID: "u",
		Purpose:       "p",
		LegalBasis:    "consent",
		Granular:      true,
		ConsentProof:  &ConsentProof{Timestamp: time.Now(), Type: "digital", Metadata: map[string]any{}},
		Specific:      false,
		Informed:      true,
		Unambiguous:   true,
	}))
	require.Error(t, manager.validateConsent(&ConsentRecord{
		DataSubjectID: "u",
		Purpose:       "p",
		LegalBasis:    "consent",
		Granular:      true,
		ConsentProof:  &ConsentProof{Timestamp: time.Now(), Type: "digital", Metadata: map[string]any{}},
		Specific:      true,
		Informed:      false,
		Unambiguous:   true,
	}))
	require.Error(t, manager.validateConsent(&ConsentRecord{
		DataSubjectID: "u",
		Purpose:       "p",
		LegalBasis:    "consent",
		Granular:      true,
		ConsentProof:  &ConsentProof{Timestamp: time.Now(), Type: "digital", Metadata: map[string]any{}},
		Specific:      true,
		Informed:      true,
		Unambiguous:   false,
	}))
}

func TestGDPRConsentManager_HandlerHelpers(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rights := &stubDataSubjectRights{}
	pia := &stubPIA{result: &PIAResult{AssessmentID: "pia1"}}

	manager := NewGDPRConsentManager(GDPRConsentConfig{Enabled: true})
	manager.SetDataSubjectRightsHandler(rights)
	manager.SetPrivacyImpactAssessment(pia)
	manager.SetConsentStore(&failingConsentStore{inner: newMemoryConsentStore()})

	_, err := manager.HandleAccessRequest(ctx, nil)
	require.Error(t, err)
	_, err = manager.HandleAccessRequest(ctx, &DataAccessRequest{})
	require.Error(t, err)
	_, err = manager.HandleAccessRequest(ctx, &DataAccessRequest{DataSubjectID: "u", RequestType: "access"})
	require.NoError(t, err)

	_, err = manager.ConductPIA(ctx, nil)
	require.Error(t, err)
	_, err = manager.ConductPIA(ctx, &PIARequest{})
	require.Error(t, err)
	_, err = manager.ConductPIA(ctx, &PIARequest{ProjectName: "p"})
	require.NoError(t, err)

	require.NoError(t, manager.UpdateConsent(ctx, "id", &ConsentUpdates{UpdatedBy: "u", UpdateReason: "r"}))

	_, err = manager.HandleErasureRequest(ctx, &DataErasureRequest{DataAccessRequest: DataAccessRequest{RequestType: "erasure"}})
	require.NoError(t, err)

	manager2 := NewGDPRConsentManager(GDPRConsentConfig{Enabled: true})
	_, err = manager2.HandleErasureRequest(ctx, &DataErasureRequest{})
	require.Error(t, err)

	manager3 := NewGDPRConsentManager(GDPRConsentConfig{Enabled: true})
	_, err = manager3.ConductPIA(ctx, &PIARequest{ProjectName: "p"})
	require.Error(t, err)

	manager4 := NewGDPRConsentManager(GDPRConsentConfig{Enabled: true})
	err = manager4.UpdateConsent(ctx, "id", &ConsentUpdates{UpdatedBy: "u", UpdateReason: "r"})
	require.Error(t, err)
}
