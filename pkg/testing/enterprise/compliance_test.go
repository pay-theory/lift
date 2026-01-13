package enterprise

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
)

type failingEvidenceStorage struct {
	err error
}

func (s failingEvidenceStorage) Store(_ context.Context, _ *Evidence) error { return s.err }
func (s failingEvidenceStorage) Retrieve(_ context.Context, _ string) (*Evidence, error) {
	return nil, s.err
}
func (s failingEvidenceStorage) List(_ context.Context, _ EvidenceFilter) ([]*Evidence, error) {
	return nil, s.err
}
func (s failingEvidenceStorage) Delete(_ context.Context, _ string) error { return s.err }

func TestSOC2TypeIICompliance_ValidateCompliance_StoreReportError(t *testing.T) {
	framework := NewSOC2TypeIICompliance(24 * time.Hour)
	framework.evidenceStore.storage = failingEvidenceStorage{err: errors.New("store failed")}

	_, err := framework.ValidateCompliance(context.Background(), &lift.App{})
	if err == nil {
		t.Fatalf("ValidateCompliance expected error")
	}
}

func TestSOC2TypeIICompliance_ExecuteTest_CoversBranches(t *testing.T) {
	framework := NewSOC2TypeIICompliance(24 * time.Hour)

	control := SOC2Control{ID: "X"}
	app := &lift.App{}
	ctx := context.Background()

	tests := []ControlTest{
		{ID: "SEC-001-INQ", Type: InquiryTest},
		{ID: "AV-001-INQ", Type: InquiryTest},
		{ID: "SEC-002-OBS", Type: ObservationTest},
		{ID: "PI-001-OBS", Type: ObservationTest},
		{ID: "SEC-003-INS", Type: InspectionTest},
		{ID: "AV-002-INS", Type: InspectionTest},
		{ID: "SEC-004-REP", Type: ReperformanceTest},
		{ID: "PI-002-REP", Type: ReperformanceTest},
		{ID: "SEC-005-ANA", Type: AnalyticalTest},
		{ID: "AV-003-ANA", Type: AnalyticalTest},
	}

	for _, tc := range tests {
		if _, err := framework.executeTest(ctx, app, control, tc); err != nil {
			t.Fatalf("executeTest(%s/%s) error: %v", tc.Type, tc.ID, err)
		}
	}

	// Unsupported test type
	if _, err := framework.executeTest(ctx, app, control, ControlTest{ID: "x", Type: TestType("unknown")}); err == nil {
		t.Fatalf("executeTest(unknown type) expected error")
	}

	// Unsupported IDs per type
	if _, err := framework.executeTest(ctx, app, control, ControlTest{ID: "bad", Type: InquiryTest}); err == nil {
		t.Fatalf("executeTest(bad inquiry ID) expected error")
	}
	if _, err := framework.executeTest(ctx, app, control, ControlTest{ID: "bad", Type: ObservationTest}); err == nil {
		t.Fatalf("executeTest(bad observation ID) expected error")
	}
	if _, err := framework.executeTest(ctx, app, control, ControlTest{ID: "bad", Type: InspectionTest}); err == nil {
		t.Fatalf("executeTest(bad inspection ID) expected error")
	}
	if _, err := framework.executeTest(ctx, app, control, ControlTest{ID: "bad", Type: ReperformanceTest}); err == nil {
		t.Fatalf("executeTest(bad reperformance ID) expected error")
	}
	if _, err := framework.executeTest(ctx, app, control, ControlTest{ID: "bad", Type: AnalyticalTest}); err == nil {
		t.Fatalf("executeTest(bad analytical ID) expected error")
	}
}

func TestSOC2TypeIICompliance_StatusCalculations(t *testing.T) {
	framework := NewSOC2TypeIICompliance(24 * time.Hour)

	overall := framework.calculateOverallStatus(map[string]*ControlResult{
		"1": {Status: ControlPassing},
		"2": {Status: ControlPassing},
	})
	if overall != CompliantStatus {
		t.Fatalf("overall=%q, want %q", overall, CompliantStatus)
	}

	overall = framework.calculateOverallStatus(map[string]*ControlResult{
		"1": {Status: ControlPassing},
		"2": {Status: ControlFailing},
		"3": {Status: ControlPassing},
	})
	if overall != PartiallyCompliant {
		t.Fatalf("overall=%q, want %q", overall, PartiallyCompliant)
	}

	overall = framework.calculateOverallStatus(map[string]*ControlResult{
		"1": {Status: ControlFailing},
		"2": {Status: ControlFailing},
	})
	if overall != NonCompliantStatus {
		t.Fatalf("overall=%q, want %q", overall, NonCompliantStatus)
	}

	controlStatus := framework.calculateControlStatus(map[string]*ComplianceTestResult{
		"a": {Status: ComplianceTestPassed},
		"b": {Status: ComplianceTestPassed},
	})
	if controlStatus != ControlPassing {
		t.Fatalf("controlStatus=%q, want %q", controlStatus, ControlPassing)
	}

	controlStatus = framework.calculateControlStatus(map[string]*ComplianceTestResult{
		"a": {Status: ComplianceTestPassed},
		"b": {Status: ComplianceTestFailed},
	})
	if controlStatus != ControlFailing {
		t.Fatalf("controlStatus=%q, want %q", controlStatus, ControlFailing)
	}

	if got := framework.evaluateTestResult("x", "x"); got != ComplianceTestPassed {
		t.Fatalf("evaluateTestResult pass=%q, want %q", got, ComplianceTestPassed)
	}
	if got := framework.evaluateTestResult("x", "y"); got != ComplianceTestFailed {
		t.Fatalf("evaluateTestResult fail=%q, want %q", got, ComplianceTestFailed)
	}
}
