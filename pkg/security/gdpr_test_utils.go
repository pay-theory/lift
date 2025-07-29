package security

import (
	"time"
)

// createTestConsentRecord creates a test consent record for use in tests
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

// createTestGDPRManager creates a test GDPR manager for use in tests
func createTestGDPRManager() *GDPRConsentManager {
	config := GDPRConsentConfig{
		Enabled:                  true,
		ConsentExpiryDays:        365,
		GranularConsentRequired:  true,
		ConsentProofRequired:     true,
		ConsentWithdrawalEnabled: true,
		DataPortabilityEnabled:   true,
		RightToErasureEnabled:    true,
		DataRetentionDays:        2555, // 7 years
		BreachNotificationHours:  72,
		PrivacyByDesignEnabled:   true,
	}
	
	manager := NewGDPRConsentManager(config)
	return manager
}