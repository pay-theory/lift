package enterprise

import (
	"time"
)

// ============================================================================
// COMPLIANCE FRAMEWORK CATEGORIES
// ============================================================================

// SOC2Category represents the five SOC 2 trust service categories
type SOC2Category string

const (
	SOC2SecurityCategory        SOC2Category = "security"
	AvailabilityCategory        SOC2Category = "availability"
	ProcessingIntegrityCategory SOC2Category = "processing_integrity"
	ConfidentialityCategory     SOC2Category = "confidentiality"
	PrivacyCategory             SOC2Category = "privacy"
)

// GDPRCategory represents GDPR compliance categories
type GDPRCategory string

const (
	DataProtectionCategory     GDPRCategory = "data_protection"
	ConsentManagementCategory  GDPRCategory = "consent_management"
	DataSubjectRightsCategory  GDPRCategory = "data_subject_rights"
	DataProcessingCategory     GDPRCategory = "data_processing"
	DataTransferCategory       GDPRCategory = "data_transfer"
	BreachNotificationCategory GDPRCategory = "breach_notification"
	PrivacyByDesignCategory    GDPRCategory = "privacy_by_design"
)

// SecurityCategory represents general security categories
type SecurityCategory string

const (
	DataProtectionSecurity     SecurityCategory = "data_protection"
	AccessControlSecurity      SecurityCategory = "access_control"
	EncryptionSecurity         SecurityCategory = "encryption"
	NetworkSecurity            SecurityCategory = "network"
	ApplicationSecurity        SecurityCategory = "application"
	InfrastructureSecurity     SecurityCategory = "infrastructure"
	IdentityManagementSecurity SecurityCategory = "identity_management"
	IncidentResponseSecurity   SecurityCategory = "incident_response"
)

// ============================================================================
// EVIDENCE AND MONITORING TYPES
// ============================================================================

// EvidenceType represents different types of evidence
type EvidenceType string

const (
	LogEvidence        EvidenceType = "log"
	ScreenshotEvidence EvidenceType = "screenshot"
	DocumentEvidence   EvidenceType = "document"
	ConfigEvidence     EvidenceType = "configuration"
	MetricEvidence     EvidenceType = "metric"
	TestResultEvidence EvidenceType = "test_result"
	ConsentEvidence    EvidenceType = "consent"
	ProcessingEvidence EvidenceType = "processing"
	TransferEvidence   EvidenceType = "transfer"
	BreachEvidence     EvidenceType = "breach"
)

// PrivacyEvidenceType is an alias for EvidenceType for privacy contexts
type PrivacyEvidenceType = EvidenceType

// MonitorType represents different types of monitoring
type MonitorType string

const (
	PerformanceMonitor  MonitorType = "performance"
	SecurityMonitor     MonitorType = "security"
	ComplianceMonitor   MonitorType = "compliance"
	AvailabilityMonitor MonitorType = "availability"
	IntegrityMonitor    MonitorType = "integrity"
)

// ============================================================================
// TEST TYPES AND FREQUENCIES
// ============================================================================

// TestType represents different types of compliance tests
type TestType string

const (
	InquiryTest       TestType = "inquiry"
	ObservationTest   TestType = "observation"
	InspectionTest    TestType = "inspection"
	ReperformanceTest TestType = "reperformance"
	AnalyticalTest    TestType = "analytical"
)

// TestFrequency defines how often tests should be performed
type TestFrequency string

const (
	ContinuousFrequency TestFrequency = "continuous"
	DailyFrequency      TestFrequency = "daily"
	WeeklyFrequency     TestFrequency = "weekly"
	MonthlyFrequency    TestFrequency = "monthly"
	QuarterlyFrequency  TestFrequency = "quarterly"
	AnnualFrequency     TestFrequency = "annual"
)

// ============================================================================
// COMPLIANCE STATUS TYPES
// ============================================================================

// ComplianceStatus represents overall compliance status
type ComplianceStatus string

const (
	CompliantStatus    ComplianceStatus = "compliant"
	NonCompliantStatus ComplianceStatus = "non_compliant"
	PartiallyCompliant ComplianceStatus = "partially_compliant"
)

// ControlStatus represents the status of individual controls
type ControlStatus string

const (
	ControlPassing   ControlStatus = "passing"
	ControlFailing   ControlStatus = "failing"
	ControlNotTested ControlStatus = "not_tested"
	ControlException ControlStatus = "exception"
)

// ComplianceTestStatus represents the status of compliance tests
type ComplianceTestStatus string

const (
	ComplianceTestPassed    ComplianceTestStatus = "passed"
	ComplianceTestFailed    ComplianceTestStatus = "failed"
	ComplianceTestException ComplianceTestStatus = "exception"
	ComplianceTestSkipped   ComplianceTestStatus = "skipped"
)

// ContractTestStatus represents the status of contract tests
type ContractTestStatus string

const (
	ContractTestStatusPassed ContractTestStatus = "passed"
	ContractTestStatusFailed ContractTestStatus = "failed"
)

// ============================================================================
// REPORT STRUCTURE TYPES
// ============================================================================

// SectionType represents different types of report sections
type SectionType string

const (
	SummarySection        SectionType = "summary"
	DetailSection         SectionType = "detail"
	EvidenceSection       SectionType = "evidence"
	RecommendationSection SectionType = "recommendation"
)

// ReportChart represents a chart in a report
type ReportChart struct {
	Data  map[string]any `json:"data"`
	Type  string         `json:"type"`
	Title string         `json:"title"`
}

// ReportSection represents a section in a report
type ReportSection struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Type        SectionType    `json:"type"`
	Content     string         `json:"content"`
	Data        map[string]any `json:"data"`
	Charts      []ReportChart  `json:"charts"`
}

// ReportTemplate represents a template for generating reports
type ReportTemplate struct {
	Metadata    map[string]any  `json:"metadata"`
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Framework   string          `json:"framework"`
	Type        ReportType      `json:"type"`
	Format      ReportFormat    `json:"format"`
	Description string          `json:"description"`
	Sections    []ReportSection `json:"sections"`
}

// ContractReportTemplate represents a template for contract reports
type ContractReportTemplate struct {
	Metadata    map[string]any  `json:"metadata"`
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Type        ReportType      `json:"type"`
	Format      ReportFormat    `json:"format"`
	Description string          `json:"description"`
	Sections    []ReportSection `json:"sections"`
}

// ============================================================================
// CORE TEST RESULT STRUCTURES
// ============================================================================

// TestResult represents the result of a test execution
type TestResult struct {
	StartTime time.Time      `json:"start_time"`
	EndTime   time.Time      `json:"end_time"`
	Error     error          `json:"error,omitempty"`
	Metrics   map[string]any `json:"metrics"`
	Metadata  map[string]any `json:"metadata"`
	TestID    string         `json:"test_id"`
	Name      string         `json:"name"`
	Status    TestStatus     `json:"status"`
	Errors    []string       `json:"errors"`
	Warnings  []string       `json:"warnings"`
	Duration  time.Duration  `json:"duration"`
	Passed    bool           `json:"passed"`
}

// TestReport represents a comprehensive test report
type TestReport struct {
	StartTime    time.Time      `json:"start_time"`
	EndTime      time.Time      `json:"end_time"`
	GeneratedAt  time.Time      `json:"generated_at"`
	Metadata     map[string]any `json:"metadata"`
	Summary      *TestSummary   `json:"summary"`
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	SuiteName    string         `json:"suite_name"`
	Type         ReportType     `json:"type"`
	Format       ReportFormat   `json:"format"`
	TestResults  []*TestResult  `json:"test_results"`
	SkippedTests int            `json:"skipped_tests"`
	FailedTests  int            `json:"failed_tests"`
	PassedTests  int            `json:"passed_tests"`
	TotalTests   int            `json:"total_tests"`
	Duration     time.Duration  `json:"duration"`
}

// TestSummary provides a summary of test results
type TestSummary struct {
	TotalTests      int           `json:"total_tests"`
	PassedTests     int           `json:"passed_tests"`
	FailedTests     int           `json:"failed_tests"`
	SkippedTests    int           `json:"skipped_tests"`
	SuccessRate     float64       `json:"success_rate"`
	TotalDuration   time.Duration `json:"total_duration"`
	AverageDuration time.Duration `json:"average_duration"`
}

// ============================================================================
// COMPLIANCE STRUCTURES
// ============================================================================

// ComplianceReport represents a comprehensive compliance report
type ComplianceReport struct {
	StartTime     time.Time                 `json:"start_time"`
	EndTime       time.Time                 `json:"end_time"`
	Controls      map[string]*ControlResult `json:"controls"`
	Summary       *ComplianceSummary        `json:"summary"`
	Metadata      map[string]any            `json:"metadata"`
	ID            string                    `json:"id"`
	Framework     string                    `json:"framework"`
	OverallStatus ComplianceStatus          `json:"overall_status"`
	Duration      time.Duration             `json:"duration"`
	AuditPeriod   time.Duration             `json:"audit_period"`
}

// ControlResult represents the result of testing a control
type ControlResult struct {
	StartTime   time.Time                        `json:"start_time"`
	EndTime     time.Time                        `json:"end_time"`
	Category    any                              `json:"category"`
	TestResults map[string]*ComplianceTestResult `json:"test_results"`
	ControlID   string                           `json:"control_id"`
	Status      ControlStatus                    `json:"status"`
	Evidence    []Evidence                       `json:"evidence"`
	Duration    time.Duration                    `json:"duration"`
}

// ComplianceTestResult represents the result of a compliance test
type ComplianceTestResult struct {
	StartTime time.Time            `json:"start_time"`
	EndTime   time.Time            `json:"end_time"`
	Result    any                  `json:"result"`
	Expected  any                  `json:"expected"`
	TestID    string               `json:"test_id"`
	Type      TestType             `json:"type"`
	Status    ComplianceTestStatus `json:"status"`
	Duration  time.Duration        `json:"duration"`
}

// ComplianceSummary provides a summary of compliance results
type ComplianceSummary struct {
	ControlsByCategory map[SecurityCategory]int `json:"controls_by_category"`
	TotalControls      int                      `json:"total_controls"`
	PassingControls    int                      `json:"passing_controls"`
	FailingControls    int                      `json:"failing_controls"`
	ExceptionControls  int                      `json:"exception_controls"`
	ComplianceScore    float64                  `json:"compliance_score"`
}

// Evidence represents evidence collected for compliance
type Evidence struct {
	Timestamp   time.Time      `json:"timestamp"`
	Metadata    map[string]any `json:"metadata"`
	Type        EvidenceType   `json:"type"`
	Description string         `json:"description"`
	Location    string         `json:"location"`
	Hash        string         `json:"hash"`
}

// ============================================================================
type DataTransfer struct {
	Timestamp   time.Time          `json:"timestamp"`
	Metadata    map[string]any     `json:"metadata"`
	ID          string             `json:"id"`
	Source      string             `json:"source"`
	Destination string             `json:"destination"`
	Mechanism   TransferMechanism  `json:"mechanism"`
	LegalBasis  string             `json:"legal_basis"`
	Status      string             `json:"status"`
	DataTypes   []PersonalDataType `json:"data_types"`
	Safeguards  []string           `json:"safeguards"`
}

// ChaosExperiment represents a chaos engineering experiment
type SecurityConfig struct {
	MaxSeverity       string   `json:"max_severity"`
	ApprovedTargets   []string `json:"approved_targets"`
	ForbiddenTargets  []string `json:"forbidden_targets"`
	RequireApproval   bool     `json:"require_approval"`
	AuditLogging      bool     `json:"audit_logging"`
	EncryptionEnabled bool     `json:"encryption_enabled"`
}

// ============================================================================
// CONTRACT TESTING ADDITIONAL TYPES
// ============================================================================

// ContractRegistry manages service contracts
