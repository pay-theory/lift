package enterprise

import (
	"context"
	"errors"
	"testing"
	"time"
)

type articleValidatorApp struct {
	failArticle string
}

func (a articleValidatorApp) ValidateArticle(article string) error {
	if article == a.failArticle {
		return errors.New("article validation failed")
	}
	return nil
}

func TestGDPRCompliance_Basics(t *testing.T) {
	g := NewGDPRCompliance()

	// Ensure validateRule is exercised by adding at least one rule.
	g.validator.rules = append(g.validator.rules, ValidationRule{ID: "rule1"})

	result, err := g.ValidateCompliance(context.Background(), map[string]any{"x": "y"})
	if err != nil {
		t.Fatalf("ValidateCompliance error: %v", err)
	}
	if result == nil || result.Status == "" {
		t.Fatalf("expected validation result")
	}

	if _, err := g.TestDataSubjectRights(context.Background()); err != nil {
		t.Fatalf("TestDataSubjectRights error: %v", err)
	}

	report, err := g.GenerateComplianceReport(context.Background())
	if err != nil || report == nil || report.ID == "" {
		t.Fatalf("GenerateComplianceReport report=%#v err=%v", report, err)
	}

	indexer := NewBasicEvidenceIndexer()
	if err := indexer.IndexEvidence(context.Background(), &Evidence{}); err != nil {
		t.Fatalf("IndexEvidence error: %v", err)
	}
	if _, err := indexer.SearchEvidence(context.Background(), "q"); err != nil {
		t.Fatalf("SearchEvidence error: %v", err)
	}
	if _, err := indexer.GetEvidence(context.Background(), "id"); err != nil {
		t.Fatalf("GetEvidence error: %v", err)
	}

	storage := NewFileEvidenceStorage("/tmp", indexer)
	if err := storage.Store(context.Background(), &Evidence{}); err != nil {
		t.Fatalf("FileEvidenceStorage.Store error: %v", err)
	}
	if _, err := storage.Retrieve(context.Background(), "id"); err != nil {
		t.Fatalf("FileEvidenceStorage.Retrieve error: %v", err)
	}
	if _, err := storage.List(context.Background(), EvidenceFilter{}); err != nil {
		t.Fatalf("FileEvidenceStorage.List error: %v", err)
	}
	if err := storage.Delete(context.Background(), "id"); err != nil {
		t.Fatalf("FileEvidenceStorage.Delete error: %v", err)
	}
}

func TestGDPRPrivacyFramework_ValidateArticlesAndHelpers(t *testing.T) {
	f := NewGDPRPrivacyFramework(24 * time.Hour)

	report, err := f.ValidateGDPRCompliance(context.Background(), articleValidatorApp{failArticle: "Article 6"})
	if err != nil {
		t.Fatalf("ValidateGDPRCompliance error: %v", err)
	}
	if report == nil || len(report.Articles) == 0 {
		t.Fatalf("expected report with articles")
	}

	// Cover testArticle cancellation handling.
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	article := getGDPRArticles()[0]
	articleResult, err := f.testArticle(cancelCtx, nil, article)
	if err == nil || articleResult == nil {
		t.Fatalf("testArticle expected cancellation error")
	}
	if articleResult.Metadata["cancel_reason"] == "" {
		t.Fatalf("expected cancel_reason metadata")
	}

	status := f.calculateOverallStatus(map[string]*ArticleResult{
		"6": {Status: CompliantStatus},
		"7": {Status: NonCompliantStatus},
	})
	if status != NonCompliantStatus {
		t.Fatalf("calculateOverallStatus=%q, want %q", status, NonCompliantStatus)
	}

	// Exercise helper-based article validators.
	if r, err := f.validateRightToErasure(context.Background(), nil); err != nil || r == nil || len(r.TestResults) == 0 {
		t.Fatalf("validateRightToErasure r=%#v err=%v", r, err)
	}
	if r, err := f.validateDataPortability(context.Background(), nil); err != nil || r == nil || len(r.TestResults) == 0 {
		t.Fatalf("validateDataPortability r=%#v err=%v", r, err)
	}
	if r, err := f.validateBreachNotification(context.Background(), nil); err != nil || r == nil || len(r.TestResults) == 0 {
		t.Fatalf("validateBreachNotification r=%#v err=%v", r, err)
	}
	if r, err := f.validateConsentLawfulness(context.Background(), nil); err != nil || r == nil || len(r.TestResults) == 0 {
		t.Fatalf("validateConsentLawfulness r=%#v err=%v", r, err)
	}
	if r, err := f.validateTransferPrinciples(context.Background(), nil); err != nil || r == nil || len(r.TestResults) == 0 {
		t.Fatalf("validateTransferPrinciples r=%#v err=%v", r, err)
	}
	if r, err := f.validatePrivacyImpactAssessment(context.Background(), nil); err != nil || r == nil || len(r.TestResults) == 0 {
		t.Fatalf("validatePrivacyImpactAssessment r=%#v err=%v", r, err)
	}
}
