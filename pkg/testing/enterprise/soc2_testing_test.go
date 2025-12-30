package enterprise

import (
	"context"
	"testing"
	"time"
)

func TestSOC2ComplianceTester_ControlsAndEvidence(t *testing.T) {
	tester := NewSOC2ComplianceTester(nil)

	if err := tester.TestSecurityControls(context.Background()); err != nil {
		t.Fatalf("TestSecurityControls error: %v", err)
	}
	if err := tester.TestAvailabilityControls(context.Background()); err != nil {
		t.Fatalf("TestAvailabilityControls error: %v", err)
	}
	if err := tester.TestProcessingIntegrity(context.Background()); err != nil {
		t.Fatalf("TestProcessingIntegrity error: %v", err)
	}
	if err := tester.TestConfidentialityControls(context.Background()); err != nil {
		t.Fatalf("TestConfidentialityControls error: %v", err)
	}
	if err := tester.TestPrivacyControls(context.Background()); err != nil {
		t.Fatalf("TestPrivacyControls error: %v", err)
	}

	evidence, err := tester.CollectControlEvidence(context.Background())
	if err != nil || evidence == nil {
		t.Fatalf("CollectControlEvidence evidence=%#v err=%v", evidence, err)
	}
	if len(evidence.SecurityControls) == 0 {
		t.Fatalf("expected security controls evidence")
	}
}

func TestSOC2Reporter_GenerateComplianceReport(t *testing.T) {
	reporter := NewSOC2Reporter()

	report, err := reporter.GenerateComplianceReport([]SOC2TestResult{
		{Timestamp: time.Now(), Success: true},
		{Timestamp: time.Now(), Success: false},
	})
	if err != nil || report == nil {
		t.Fatalf("GenerateComplianceReport report=%#v err=%v", report, err)
	}
	if report.TotalTests != 2 || report.PassedTests != 1 || report.FailedTests != 1 {
		t.Fatalf("unexpected report counts: %#v", report)
	}
	if report.Summary == "" {
		t.Fatalf("expected non-empty summary")
	}

	report, err = reporter.GenerateComplianceReport([]SOC2TestResult{
		{Timestamp: time.Now(), Success: true},
	})
	if err != nil || report == nil || report.FailedTests != 0 || !report.OverallCompliance {
		t.Fatalf("GenerateComplianceReport(all pass) report=%#v err=%v", report, err)
	}
}
