package enterprise

import (
	"context"
	"testing"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
)

func TestGDPRPrivacyFramework(t *testing.T) {
	// Create GDPR framework
	framework := NewGDPRPrivacyFramework(30 * 24 * time.Hour) // 30 days audit period

	// Create test app
	app := &lift.App{}

	ctx := context.Background()

	// Test GDPR compliance validation
	t.Run("ValidateGDPRCompliance", func(t *testing.T) {
		report, err := framework.ValidateGDPRCompliance(ctx, app)
		if err != nil {
			t.Fatalf("Failed to validate GDPR compliance: %v", err)
		}

		if report == nil {
			t.Fatal("Expected compliance report, got nil")
		}

		if report.Framework != "GDPR" {
			t.Errorf("Expected framework 'GDPR', got %s", report.Framework)
		}

		if len(report.Articles) == 0 {
			t.Error("Expected articles in report, got none")
		}

		// Check that we have key GDPR articles
		expectedArticles := []string{"Article 6", "Article 17"}
		for _, article := range expectedArticles {
			if _, exists := report.Articles[article]; !exists {
				t.Errorf("Expected article %s in report", article)
			}
		}

		// Validate report structure
		if report.StartTime.IsZero() {
			t.Error("Expected start time to be set")
		}

		if report.EndTime.IsZero() {
			t.Error("Expected end time to be set")
		}

		if report.Duration <= 0 {
			t.Error("Expected positive duration")
		}

		if report.OverallStatus == "" {
			t.Error("Expected overall status to be set")
		}

		if report.RiskAssessment == nil {
			t.Error("Expected risk assessment to be present")
		}
	})

	t.Run("TestArticleValidation", func(t *testing.T) {
		// Test specific article validation
		articles := framework.articles
		if len(articles) == 0 {
			t.Fatal("Expected GDPR articles to be configured")
		}

		for _, article := range articles {
			t.Run(article.Number, func(t *testing.T) {
				result, err := framework.testArticle(ctx, app, article)
				if err != nil {
					t.Fatalf("Failed to test article %s: %v", article.Number, err)
				}

				if result == nil {
					t.Fatal("Expected article result, got nil")
				}

				if result.ArticleNumber != article.Number {
					t.Errorf("Expected article number %s, got %s", article.Number, result.ArticleNumber)
				}

				if result.Category != article.Category {
					t.Errorf("Expected category %s, got %s", article.Category, result.Category)
				}

				if len(result.TestResults) == 0 {
					t.Error("Expected test results for article")
				}

				// Validate test results
				for testID, testResult := range result.TestResults {
					if testResult.TestID != testID {
						t.Errorf("Test ID mismatch: expected %s, got %s", testID, testResult.TestID)
					}

					if testResult.StartTime.IsZero() {
						t.Error("Expected test start time to be set")
					}

					if testResult.EndTime.IsZero() {
						t.Error("Expected test end time to be set")
					}

					if testResult.Duration <= 0 {
						t.Error("Expected positive test duration")
					}

					if testResult.Status == "" {
						t.Error("Expected test status to be set")
					}
				}
			})
		}
	})

	t.Run("TestConsentValidation", func(t *testing.T) {
		// Test consent validation
		result, err := framework.validateConsentLawfulness(ctx, app)
		if err != nil {
			t.Fatalf("Failed to validate consent lawfulness: %v", err)
		}

		if result == nil {
			t.Fatal("Expected consent validation result, got nil")
		}

		// Check expected consent validation fields
		expectedFields := []string{
			"consent_mechanism_exists",
			"consent_freely_given",
			"consent_specific",
			"consent_informed",
			"consent_unambiguous",
			"consent_withdrawable",
			"consent_granular",
			"consent_documented",
			"legal_basis_documented",
		}

		resultMap, ok := result.(map[string]any)
		if !ok {
			t.Fatal("Expected result to be a map")
		}

		for _, field := range expectedFields {
			if _, exists := resultMap[field]; !exists {
				t.Errorf("Expected field %s in consent validation result", field)
			}
		}
	})

	t.Run("TestRightToErasureValidation", func(t *testing.T) {
		// Test right to erasure validation
		result, err := framework.validateRightToErasure(ctx, app)
		if err != nil {
			t.Fatalf("Failed to validate right to erasure: %v", err)
		}

		if result == nil {
			t.Fatal("Expected erasure validation result, got nil")
		}

		// Check expected erasure validation fields
		expectedFields := []string{
			"erasure_request_mechanism",
			"erasure_grounds_checked",
			"erasure_executed",
			"third_parties_notified",
			"response_within_30_days",
			"erasure_documented",
			"backup_erasure_included",
			"technical_erasure_complete",
		}

		resultMap, ok := result.(map[string]any)
		if !ok {
			t.Fatal("Expected result to be a map")
		}

		for _, field := range expectedFields {
			if _, exists := resultMap[field]; !exists {
				t.Errorf("Expected field %s in erasure validation result", field)
			}
		}
	})

	t.Run("TestDataPortabilityValidation", func(t *testing.T) {
		// Test data portability validation
		result, err := framework.validateDataPortability(ctx, app)
		if err != nil {
			t.Fatalf("Failed to validate data portability: %v", err)
		}

		if result == nil {
			t.Fatal("Expected portability validation result, got nil")
		}

		// Check expected portability validation fields
		expectedFields := []string{
			"portability_mechanism",
			"structured_format",
			"commonly_used_format",
			"machine_readable",
			"direct_transmission",
			"technical_feasibility",
			"response_within_30_days",
			"free_of_charge",
		}

		resultMap, ok := result.(map[string]any)
		if !ok {
			t.Fatal("Expected result to be a map")
		}

		for _, field := range expectedFields {
			if _, exists := resultMap[field]; !exists {
				t.Errorf("Expected field %s in portability validation result", field)
			}
		}
	})

	t.Run("TestTransferValidation", func(t *testing.T) {
		// Test transfer validation
		result, err := framework.validateTransferPrinciples(ctx, app)
		if err != nil {
			t.Fatalf("Failed to validate transfer principles: %v", err)
		}

		if result == nil {
			t.Fatal("Expected transfer validation result, got nil")
		}

		// Check expected transfer validation fields
		expectedFields := []string{
			"transfer_lawfulness",
			"adequate_protection",
			"transfer_documented",
			"data_subject_informed",
			"safeguards_implemented",
			"transfer_necessity",
			"proportionality_assessed",
		}

		resultMap, ok := result.(map[string]any)
		if !ok {
			t.Fatal("Expected result to be a map")
		}

		for _, field := range expectedFields {
			if _, exists := resultMap[field]; !exists {
				t.Errorf("Expected field %s in transfer validation result", field)
			}
		}
	})

	t.Run("TestBreachValidation", func(t *testing.T) {
		// Test breach notification validation
		result, err := framework.validateBreachNotification(ctx, app)
		if err != nil {
			t.Fatalf("Failed to validate breach notification: %v", err)
		}

		if result == nil {
			t.Fatal("Expected breach validation result, got nil")
		}

		// Check expected breach validation fields
		expectedFields := []string{
			"breach_detection_capability",
			"72_hour_notification",
			"supervisory_authority_notified",
			"breach_documented",
			"risk_assessment_conducted",
			"notification_complete",
			"follow_up_provided",
		}

		resultMap, ok := result.(map[string]any)
		if !ok {
			t.Fatal("Expected result to be a map")
		}

		for _, field := range expectedFields {
			if _, exists := resultMap[field]; !exists {
				t.Errorf("Expected field %s in breach validation result", field)
			}
		}
	})

	t.Run("TestPrivacyImpactAssessment", func(t *testing.T) {
		// Test PIA validation
		result, err := framework.validatePrivacyImpactAssessment(ctx, app)
		if err != nil {
			t.Fatalf("Failed to validate PIA: %v", err)
		}

		if result == nil {
			t.Fatal("Expected PIA validation result, got nil")
		}

		// Check expected PIA validation fields
		expectedFields := []string{
			"pia_conducted",
			"high_risk_processing",
			"systematic_assessment",
			"necessity_proportionality",
			"risks_identified",
			"mitigation_measures",
			"consultation_conducted",
			"pia_documented",
			"pia_updated",
		}

		resultMap, ok := result.(map[string]any)
		if !ok {
			t.Fatal("Expected result to be a map")
		}

		for _, field := range expectedFields {
			if _, exists := resultMap[field]; !exists {
				t.Errorf("Expected field %s in PIA validation result", field)
			}
		}
	})
}

func TestGDPRCategories(t *testing.T) {
	// Test GDPR categories
	categories := []GDPRCategory{
		DataProtectionCategory,
		ConsentManagementCategory,
		DataSubjectRightsCategory,
		DataTransferCategory,
		GDPRGovernanceCategory,
		BreachNotificationCategory,
	}

	for _, category := range categories {
		if string(category) == "" {
			t.Errorf("Category should not be empty: %v", category)
		}
	}
}

func TestPrivacyTestTypes(t *testing.T) {
	// Test privacy test types
	testTypes := []PrivacyTestType{
		ConsentTest,
		DataMappingTest,
		RightToAccessTest,
		RightToErasureTest,
		DataPortabilityTest,
		TransferValidationTest,
		BreachDetectionTest,
		PIATest,
	}

	for _, testType := range testTypes {
		if string(testType) == "" {
			t.Errorf("Test type should not be empty: %v", testType)
		}
	}
}

func TestPersonalDataTypes(t *testing.T) {
	// Test personal data types
	dataTypes := []PersonalDataType{
		IdentifyingData,
		SensitiveData,
		BiometricData,
		HealthData,
		FinancialData,
		LocationData,
		BehavioralData,
		CommunicationData,
	}

	for _, dataType := range dataTypes {
		if string(dataType) == "" {
			t.Errorf("Data type should not be empty: %v", dataType)
		}
	}
}

func TestGDPRArticleStructure(t *testing.T) {
	articles := getGDPRArticles()

	if len(articles) == 0 {
		t.Fatal("Expected GDPR articles to be configured")
	}

	for _, article := range articles {
		// Validate article structure
		if article.Number == "" {
			t.Error("Article number should not be empty")
		}

		if article.Title == "" {
			t.Error("Article title should not be empty")
		}

		if article.Category == "" {
			t.Error("Article category should not be empty")
		}

		if article.Description == "" {
			t.Error("Article description should not be empty")
		}

		if len(article.Tests) == 0 {
			t.Errorf("Article %s should have tests", article.Number)
		}

		// Validate tests
		for _, test := range article.Tests {
			if test.ID == "" {
				t.Error("Test ID should not be empty")
			}

			if test.Type == "" {
				t.Error("Test type should not be empty")
			}

			if test.Procedure == "" {
				t.Error("Test procedure should not be empty")
			}
		}

		// Validate evidence requirements
		for _, evidence := range article.Evidence {
			if evidence.Type == "" {
				t.Error("Evidence type should not be empty")
			}

			if evidence.Description == "" {
				t.Error("Evidence description should not be empty")
			}

			if evidence.Retention <= 0 {
				t.Error("Evidence retention should be positive")
			}
		}
	}
}

func BenchmarkGDPRValidation(b *testing.B) {
	framework := NewGDPRPrivacyFramework(30 * 24 * time.Hour)
	app := &lift.App{}
	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := framework.ValidateGDPRCompliance(ctx, app)
		if err != nil {
			b.Fatalf("Benchmark failed: %v", err)
		}
	}
}

func BenchmarkConsentValidation(b *testing.B) {
	framework := NewGDPRPrivacyFramework(30 * 24 * time.Hour)
	app := &lift.App{}
	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := framework.validateConsentLawfulness(ctx, app)
		if err != nil {
			b.Fatalf("Benchmark failed: %v", err)
		}
	}
}

func BenchmarkErasureValidation(b *testing.B) {
	framework := NewGDPRPrivacyFramework(30 * 24 * time.Hour)
	app := &lift.App{}
	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := framework.validateRightToErasure(ctx, app)
		if err != nil {
			b.Fatalf("Benchmark failed: %v", err)
		}
	}
}
