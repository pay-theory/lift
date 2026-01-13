package enterprise

// ============================================================================
// COMMON SEVERITY AND STATUS TYPES
// ============================================================================

// Severity represents the severity level of issues, alerts, and violations
type Severity string

const (
	CriticalSeverity Severity = "critical"
	HighSeverity     Severity = "high"
	MediumSeverity   Severity = "medium"
	LowSeverity      Severity = "low"
	InfoSeverity     Severity = "info"
)

// ValidationSeverity is an alias for Severity for validation contexts
type ValidationSeverity = Severity

// RuleSeverity is an alias for Severity for rule contexts
type RuleSeverity = Severity

// AlertSeverity is an alias for Severity for alert contexts
type AlertSeverity = Severity

// Test Status Types
type TestStatus string

const (
	PendingStatus TestStatus = "pending"
	RunningStatus TestStatus = "running"
	PassedStatus  TestStatus = "passed"
	FailedStatus  TestStatus = "failed"
	SkippedStatus TestStatus = "skipped"
	TimeoutStatus TestStatus = "timeout"
	ErrorStatus   TestStatus = "error"
	// Aliases for backward compatibility
	TestStatusPending   = PendingStatus
	TestStatusRunning   = RunningStatus
	TestStatusPassed    = PassedStatus
	TestStatusFailed    = FailedStatus
	TestStatusSkipped   = SkippedStatus
	TestStatusTimeout   = TimeoutStatus
	TestStatusError     = ErrorStatus
	TestStatusCompleted = PassedStatus
)

// ValidationStatus represents validation result status
type ValidationStatus string

const (
	ValidationPassed ValidationStatus = "passed"
	ValidationFailed ValidationStatus = "failed"
	ValidationError  ValidationStatus = "error"
	// Aliases for backward compatibility
	ValidationStatusPassed = ValidationPassed
	ValidationStatusFailed = ValidationFailed
	ValidationStatusError  = ValidationError
)

// ============================================================================
// REPORT FORMAT AND EXPORT TYPES
// ============================================================================

// ReportFormat represents the format for exporting reports
type ReportFormat string

const (
	JSONFormat ReportFormat = "json"
	PDFFormat  ReportFormat = "pdf"
	HTMLFormat ReportFormat = "html"
	CSVFormat  ReportFormat = "csv"
	XMLFormat  ReportFormat = "xml"
)

// ExportFormat is an alias for ReportFormat for export contexts
type ExportFormat = ReportFormat

// Report Types
type ReportType string

const (
	ComplianceReportType  ReportType = "compliance"
	SecurityReportType    ReportType = "security"
	PerformanceReportType ReportType = "performance"
	TestReportType        ReportType = "test"
	ContractReportType    ReportType = "contract"
	ChaosReportType       ReportType = "chaos"
)

type ValidationViolation struct {
	Value    any            `json:"value"`
	Expected any            `json:"expected"`
	Metadata map[string]any `json:"metadata"`
	RuleID   string         `json:"rule_id"`
	Severity Severity       `json:"severity"`
	Message  string         `json:"message"`
	Field    string         `json:"field"`
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	Metadata map[string]any `json:"metadata"`
	Message  string         `json:"message"`
	Field    string         `json:"field"`
}

// DataTransfer represents a data transfer record
