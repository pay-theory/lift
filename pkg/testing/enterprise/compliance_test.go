package enterprise

import (
	"context"
	"testing"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSOC2TypeIICompliance(t *testing.T) {
	// Create SOC 2 Type II compliance framework
	auditPeriod := 365 * 24 * time.Hour // 1 year
	compliance := NewSOC2TypeIICompliance(auditPeriod)

	require.NotNil(t, compliance)
	assert.Equal(t, auditPeriod, compliance.auditPeriod)
	assert.NotNil(t, compliance.controls)
	assert.NotNil(t, compliance.validator)
	assert.NotNil(t, compliance.reporter)
	assert.NotNil(t, compliance.monitor)
	assert.NotNil(t, compliance.evidenceStore)
}

func TestSOC2ComplianceValidation(t *testing.T) {
	// Create test context
	ctx := context.Background()

	// Create a test app
	app := lift.New()

	// Create SOC 2 Type II compliance framework
	auditPeriod := 30 * 24 * time.Hour // 30 days for testing
	compliance := NewSOC2TypeIICompliance(auditPeriod)

	// Run compliance validation
	report, err := compliance.ValidateCompliance(ctx, app)

	require.NoError(t, err)
	require.NotNil(t, report)

	// Validate report structure
	assert.Equal(t, "SOC 2 Type II", report.Framework)
	assert.Equal(t, auditPeriod, report.AuditPeriod)
	assert.NotZero(t, report.StartTime)
	assert.NotZero(t, report.EndTime)
	assert.True(t, report.EndTime.After(report.StartTime))
	assert.NotZero(t, report.Duration)
	assert.NotNil(t, report.Controls)

	// Validate controls were tested
	assert.Greater(t, len(report.Controls), 0, "Should have tested at least one control")

	// Validate control results
	for controlID, result := range report.Controls {
		assert.NotEmpty(t, controlID)
		assert.NotNil(t, result)
		assert.Equal(t, controlID, result.ControlID)
		assert.NotZero(t, result.StartTime)
		assert.NotZero(t, result.EndTime)
		assert.True(t, result.EndTime.After(result.StartTime))
		assert.NotZero(t, result.Duration)
		assert.NotNil(t, result.TestResults)
		assert.NotNil(t, result.Evidence)

		// Validate test results
		for testID, testResult := range result.TestResults {
			assert.NotEmpty(t, testID)
			assert.NotNil(t, testResult)
			assert.Equal(t, testID, testResult.TestID)
			assert.NotZero(t, testResult.StartTime)
			assert.NotZero(t, testResult.EndTime)
			assert.True(t, testResult.EndTime.After(testResult.StartTime))
			assert.NotZero(t, testResult.Duration)
			assert.NotNil(t, testResult.Result)
		}
	}
}

func TestSOC2ControlTesting(t *testing.T) {
	ctx := context.Background()
	app := lift.New()
	compliance := NewSOC2TypeIICompliance(30 * 24 * time.Hour)

	// Test individual control
	controls := getSOC2Controls()
	require.Greater(t, len(controls), 0, "Should have SOC 2 controls defined")

	control := controls[0] // Test first control
	result, err := compliance.testControl(ctx, app, control)

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, control.ID, result.ControlID)
	assert.Equal(t, control.Category, result.Category)
	assert.NotZero(t, result.StartTime)
	assert.NotZero(t, result.EndTime)
	assert.NotZero(t, result.Duration)
	assert.NotNil(t, result.TestResults)

	// Validate all tests were executed
	assert.Equal(t, len(control.Tests), len(result.TestResults))
}

func TestSOC2TestExecution(t *testing.T) {
	ctx := context.Background()
	app := lift.New()
	compliance := NewSOC2TypeIICompliance(30 * 24 * time.Hour)

	// Create test control and test
	control := SOC2Control{
		ID:       "TEST-001",
		Category: SOC2SecurityCategory,
		Tests: []ControlTest{
			{
				ID:        "TEST-001-INQ",
				Type:      InquiryTest,
				Procedure: "Test inquiry procedure",
				Frequency: MonthlyFrequency,
				Automated: true,
			},
		},
	}

	test := control.Tests[0]
	result, err := compliance.executeTest(ctx, app, control, test)

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, test.ID, result.TestID)
	assert.Equal(t, test.Type, result.Type)
	assert.NotZero(t, result.StartTime)
	assert.NotZero(t, result.EndTime)
	assert.NotZero(t, result.Duration)
	assert.NotNil(t, result.Result)
}

func TestSOC2InquiryTests(t *testing.T) {
	ctx := context.Background()
	app := lift.New()
	compliance := NewSOC2TypeIICompliance(30 * 24 * time.Hour)

	control := SOC2Control{ID: "TEST-001", Category: SOC2SecurityCategory}

	// Test security policy inquiry
	test := ControlTest{ID: "SEC-001-INQ", Type: InquiryTest}
	result, err := compliance.executeInquiryTest(ctx, app, control, test)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Validate result structure
	resultMap, ok := result.(map[string]any)
	require.True(t, ok, "Result should be a map")

	assert.Contains(t, resultMap, "policy_exists")
	assert.Contains(t, resultMap, "last_updated")
	assert.Contains(t, resultMap, "approval_status")
	assert.Contains(t, resultMap, "review_frequency")
}

func TestSOC2ObservationTests(t *testing.T) {
	ctx := context.Background()
	app := lift.New()
	compliance := NewSOC2TypeIICompliance(30 * 24 * time.Hour)

	control := SOC2Control{ID: "TEST-001", Category: SOC2SecurityCategory}

	// Test access control observation
	test := ControlTest{ID: "SEC-002-OBS", Type: ObservationTest}
	result, err := compliance.executeObservationTest(ctx, app, control, test)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Validate result structure
	resultMap, ok := result.(map[string]any)
	require.True(t, ok, "Result should be a map")

	assert.Contains(t, resultMap, "authentication_required")
	assert.Contains(t, resultMap, "authorization_enforced")
	assert.Contains(t, resultMap, "session_management")
	assert.Contains(t, resultMap, "failed_attempts_logged")
}

func TestSOC2AnalyticalTests(t *testing.T) {
	ctx := context.Background()
	app := lift.New()
	compliance := NewSOC2TypeIICompliance(30 * 24 * time.Hour)

	control := SOC2Control{ID: "TEST-001", Category: SOC2SecurityCategory}

	// Test security log analysis
	test := ControlTest{ID: "SEC-005-ANA", Type: AnalyticalTest}
	result, err := compliance.executeAnalyticalTest(ctx, app, control, test)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Validate result structure
	resultMap, ok := result.(map[string]any)
	require.True(t, ok, "Result should be a map")

	assert.Contains(t, resultMap, "total_events")
	assert.Contains(t, resultMap, "security_events")
	assert.Contains(t, resultMap, "anomalies_detected")
	assert.Contains(t, resultMap, "response_time_avg")
}

func TestSOC2EvidenceCollection(t *testing.T) {
	ctx := context.Background()
	compliance := NewSOC2TypeIICompliance(30 * 24 * time.Hour)

	// Create test control with evidence requirements
	control := SOC2Control{
		ID:       "TEST-001",
		Category: SOC2SecurityCategory,
		Evidence: []EvidenceRequirement{
			{
				Type:        LogEvidence,
				Description: "Test log evidence",
				Retention:   365 * 24 * time.Hour,
				Location:    "/var/log/test",
				Automated:   true,
			},
			{
				Type:        ConfigEvidence,
				Description: "Test configuration evidence",
				Retention:   30 * 24 * time.Hour,
				Location:    "/etc/test",
				Automated:   false,
			},
		},
	}

	result := &ControlResult{
		ControlID: control.ID,
		Category:  control.Category,
		Status:    ControlPassing,
	}

	evidence, err := compliance.collectEvidence(ctx, control, result)

	require.NoError(t, err)
	require.NotNil(t, evidence)
	assert.Equal(t, len(control.Evidence), len(evidence))

	// Validate evidence structure
	for i, ev := range evidence {
		req := control.Evidence[i]
		assert.Equal(t, req.Type, ev.Type)
		assert.Equal(t, req.Description, ev.Description)
		assert.Equal(t, req.Location, ev.Location)
		assert.NotZero(t, ev.Timestamp)
		assert.NotEmpty(t, ev.Hash)
		assert.NotNil(t, ev.Metadata)
		assert.Equal(t, control.ID, ev.Metadata["control_id"])
		assert.Equal(t, req.Automated, ev.Metadata["automated"])
	}
}

func TestSOC2ComplianceStatus(t *testing.T) {
	compliance := NewSOC2TypeIICompliance(30 * 24 * time.Hour)

	// Test all passing controls
	allPassing := map[string]*ControlResult{
		"CTRL-001": {Status: ControlPassing},
		"CTRL-002": {Status: ControlPassing},
		"CTRL-003": {Status: ControlPassing},
	}
	status := compliance.calculateOverallStatus(allPassing)
	assert.Equal(t, CompliantStatus, status)

	// Test all failing controls
	allFailing := map[string]*ControlResult{
		"CTRL-001": {Status: ControlFailing},
		"CTRL-002": {Status: ControlFailing},
		"CTRL-003": {Status: ControlFailing},
	}
	status = compliance.calculateOverallStatus(allFailing)
	assert.Equal(t, NonCompliantStatus, status)

	// Test mixed controls (majority passing)
	mixedPassing := map[string]*ControlResult{
		"CTRL-001": {Status: ControlPassing},
		"CTRL-002": {Status: ControlPassing},
		"CTRL-003": {Status: ControlFailing},
	}
	status = compliance.calculateOverallStatus(mixedPassing)
	assert.Equal(t, PartiallyCompliant, status)

	// Test mixed controls (majority failing)
	mixedFailing := map[string]*ControlResult{
		"CTRL-001": {Status: ControlPassing},
		"CTRL-002": {Status: ControlFailing},
		"CTRL-003": {Status: ControlFailing},
	}
	status = compliance.calculateOverallStatus(mixedFailing)
	assert.Equal(t, NonCompliantStatus, status)
}

func TestSOC2ControlStatus(t *testing.T) {
	compliance := NewSOC2TypeIICompliance(30 * 24 * time.Hour)

	// Test all passing tests
	allPassing := map[string]*ComplianceTestResult{
		"TEST-001": {Status: ComplianceTestPassed},
		"TEST-002": {Status: ComplianceTestPassed},
		"TEST-003": {Status: ComplianceTestPassed},
	}
	status := compliance.calculateControlStatus(allPassing)
	assert.Equal(t, ControlPassing, status)

	// Test some failing tests
	someFailing := map[string]*ComplianceTestResult{
		"TEST-001": {Status: ComplianceTestPassed},
		"TEST-002": {Status: ComplianceTestFailed},
		"TEST-003": {Status: ComplianceTestPassed},
	}
	status = compliance.calculateControlStatus(someFailing)
	assert.Equal(t, ControlFailing, status)
}

func TestSOC2TestResultEvaluation(t *testing.T) {
	compliance := NewSOC2TypeIICompliance(30 * 24 * time.Hour)

	// Test matching results
	status := compliance.evaluateTestResult("passed", "passed")
	assert.Equal(t, ComplianceTestPassed, status)

	status = compliance.evaluateTestResult(true, true)
	assert.Equal(t, ComplianceTestPassed, status)

	status = compliance.evaluateTestResult(100, 100)
	assert.Equal(t, ComplianceTestPassed, status)

	// Test non-matching results
	status = compliance.evaluateTestResult("passed", "failed")
	assert.Equal(t, ComplianceTestFailed, status)

	status = compliance.evaluateTestResult(true, false)
	assert.Equal(t, ComplianceTestFailed, status)

	status = compliance.evaluateTestResult(100, 200)
	assert.Equal(t, ComplianceTestFailed, status)
}

func TestSOC2Categories(t *testing.T) {
	// Test SOC 2 category constants
	assert.Equal(t, "security", string(SOC2SecurityCategory))
	assert.Equal(t, "availability", string(AvailabilityCategory))
	assert.Equal(t, "processing_integrity", string(ProcessingIntegrityCategory))
	assert.Equal(t, "confidentiality", string(ConfidentialityCategory))
	assert.Equal(t, "privacy", string(PrivacyCategory))
}

func TestSOC2TestTypes(t *testing.T) {
	// Test test type constants
	assert.Equal(t, "inquiry", string(InquiryTest))
	assert.Equal(t, "observation", string(ObservationTest))
	assert.Equal(t, "inspection", string(InspectionTest))
	assert.Equal(t, "reperformance", string(ReperformanceTest))
	assert.Equal(t, "analytical", string(AnalyticalTest))
}

func TestSOC2EvidenceTypes(t *testing.T) {
	// Test evidence type constants
	assert.Equal(t, "log", string(LogEvidence))
	assert.Equal(t, "screenshot", string(ScreenshotEvidence))
	assert.Equal(t, "document", string(DocumentEvidence))
	assert.Equal(t, "configuration", string(ConfigEvidence))
	assert.Equal(t, "metric", string(MetricEvidence))
	assert.Equal(t, "test_result", string(TestResultEvidence))
}

// Benchmark tests for performance validation
func BenchmarkSOC2ComplianceValidation(b *testing.B) {
	ctx := context.Background()
	app := lift.New()
	compliance := NewSOC2TypeIICompliance(30 * 24 * time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := compliance.ValidateCompliance(ctx, app)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSOC2ControlTesting(b *testing.B) {
	ctx := context.Background()
	app := lift.New()
	compliance := NewSOC2TypeIICompliance(30 * 24 * time.Hour)
	controls := getSOC2Controls()

	if len(controls) == 0 {
		b.Skip("No controls available for benchmarking")
	}

	control := controls[0]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := compliance.testControl(ctx, app, control)
		if err != nil {
			b.Fatal(err)
		}
	}
}
