package security

import "time"

// Test helper functions for GDPR consent management tests

// createTestConsentRecord creates a test ConsentRecord with default values
func createTestConsentRecord() *ConsentRecord {
	now := time.Now()
	expiryDate := now.Add(365 * 24 * time.Hour)
	return &ConsentRecord{
		ID:               "test-consent-123",
		DataSubjectID:    "test-user-456",
		DataSubjectEmail: "test@example.com",
		ConsentVersion:   "1.0",
		ConsentScope: []ConsentPurpose{
			{
				ID:          "analytics",
				Name:        "Analytics",
				Description: "Analytics tracking",
				LegalBasis:  "consent",
				ConsentDate: now,
				Required:    false,
				Consented:   true,
			},
			{
				ID:          "marketing",
				Name:        "Marketing",
				Description: "Marketing communications",
				LegalBasis:  "consent",
				ConsentDate: now,
				Required:    false,
				Consented:   true,
			},
		},
		ProcessingPurposes: []string{
			"analytics",
			"marketing",
			"personalization",
		},
		DataCategories: []string{
			"usage_data",
			"preferences",
			"device_info",
		},
		Recipients: []DataRecipient{
			{
				ID:         "proc-1",
				Name:       "Analytics Inc",
				Type:       "processor",
				Country:    "US",
				Purposes:   []string{"analytics"},
				Safeguards: []string{"SCC"},
			},
		},
		ConsentProof: &ConsentProof{
			Type:      "digital_signature",
			Evidence:  "User clicked consent button",
			IPAddress: "192.168.1.1",
			UserAgent: "Mozilla/5.0",
			Method:    "web_form",
			Timestamp: now,
			Verified:  true,
			Metadata:  make(map[string]any),
		},
		ConsentDate:   now,
		ExpiryDate:    &expiryDate,
		ConsentMethod: "web_form",
		Status:        "active",
		CreatedAt:     now,
		UpdatedAt:     now,
		Metadata:      make(map[string]any),
		RetentionPeriod: 365 * 24 * time.Hour,
	}
}

// createTestGDPRManager creates a test GDPRConsentManager with default configuration
func createTestGDPRManager() *GDPRConsentManager {
	return &GDPRConsentManager{
		config: GDPRConsentConfig{
			DataRetentionPolicies: map[string]time.Duration{
				"analytics": 90 * 24 * time.Hour,
				"marketing": 180 * 24 * time.Hour,
			},
			CrossBorderTransferRules: []CrossBorderRule{},
			ConsentRenewalDays:       365,
			BreachNotificationHours:  72,
			ConsentExpiryDays:        365,
			DataRetentionDays:        90,
			RequestProcessingDays:    30,
			Enabled:                  true,
			AutomaticConsentRenewal:  false,
			GranularConsentRequired:  true,
			ConsentWithdrawalEnabled: true,
			DataPortabilityEnabled:   true,
			RightToErasureEnabled:    true,
			PrivacyByDesignEnabled:   true,
			RequireExplicitConsent:   true,
			RequireConsentProof:      true,
			ConsentProofRequired:     true,
		},
		// The other fields will be initialized by the constructor or test setup
	}
}