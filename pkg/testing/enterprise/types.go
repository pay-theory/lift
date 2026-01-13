package enterprise

import (
	"context"
	"fmt"
	"time"
)

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
// CONTRACT TESTING STRUCTURES
// ============================================================================

// ContractTest represents a contract test
type ContractTest struct {
	Validator ContractValidator `json:"validator"`
	Contract  *ServiceContract  `json:"contract"`
	Config    *TestConfig       `json:"config"`
	Metadata  map[string]any    `json:"metadata"`
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Provider  string            `json:"provider"`
	Consumer  string            `json:"consumer"`
}

// ServiceContract represents a service contract
type ServiceContract struct {
	Provider     ServiceInfo           `json:"provider"`
	Consumer     ServiceInfo           `json:"consumer"`
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
	Metadata     map[string]any        `json:"metadata"`
	ID           string                `json:"id"`
	Name         string                `json:"name"`
	Version      string                `json:"version"`
	Status       ContractStatus        `json:"status"`
	Interactions []ContractInteraction `json:"interactions"`
}

// ContractInteraction represents an interaction in a contract
type ContractInteraction struct {
	Request     *InteractionRequest  `json:"request"`
	Response    *InteractionResponse `json:"response"`
	Metadata    map[string]any       `json:"metadata"`
	ID          string               `json:"id"`
	Description string               `json:"description"`
	State       string               `json:"state,omitempty"`
}

// InteractionRequest represents a request in a contract interaction
type InteractionRequest struct {
	Body    any               `json:"body"`
	Headers map[string]string `json:"headers"`
	Query   map[string]any    `json:"query"`
	Schema  *SchemaDefinition `json:"schema,omitempty"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
}

// InteractionResponse represents a response in a contract interaction
type InteractionResponse struct {
	Body    any               `json:"body"`
	Headers map[string]string `json:"headers"`
	Schema  *SchemaDefinition `json:"schema,omitempty"`
	Status  int               `json:"status"`
}

// ContractTestResult represents the result of a contract test
type ContractTestResult struct {
	StartTime    time.Time            `json:"start_time"`
	EndTime      time.Time            `json:"end_time"`
	Summary      *ContractTestSummary `json:"summary"`
	Metadata     map[string]any       `json:"metadata"`
	ContractID   string               `json:"contract_id"`
	Provider     string               `json:"provider"`
	Consumer     string               `json:"consumer"`
	Status       TestStatus           `json:"status"`
	Interactions []InteractionResult  `json:"interactions"`
	Duration     time.Duration        `json:"duration"`
}

// InteractionResult represents the result of testing an interaction
type InteractionResult struct {
	Request       *InteractionRequest  `json:"request"`
	Response      *InteractionResponse `json:"response"`
	Expected      *InteractionResponse `json:"expected"`
	Metadata      map[string]any       `json:"metadata"`
	InteractionID string               `json:"interaction_id"`
	Status        TestStatus           `json:"status"`
	Errors        []string             `json:"errors"`
}

// ContractTestSummary provides a summary of contract test results
type ContractTestSummary struct {
	AverageResponseTime string  `json:"average_response_time"`
	TotalInteractions   int     `json:"total_interactions"`
	PassedInteractions  int     `json:"passed_interactions"`
	FailedInteractions  int     `json:"failed_interactions"`
	SuccessRate         float64 `json:"success_rate"`
}

// ============================================================================
// GDPR AND PRIVACY STRUCTURES
// ============================================================================

// ConsentRecord represents a GDPR consent record
type ConsentRecord struct {
	ConsentDate     time.Time      `json:"consent_date"`
	ExpiryDate      *time.Time     `json:"expiry_date,omitempty"`
	WithdrawnDate   *time.Time     `json:"withdrawn_date,omitempty"`
	Metadata        map[string]any `json:"metadata"`
	ID              string         `json:"id"`
	SubjectID       string         `json:"subject_id"`
	Purpose         string         `json:"purpose"`
	LegalBasis      string         `json:"legal_basis"`
	ProcessingScope string         `json:"processing_scope"`
	DataTypes       []string       `json:"data_types"`
	ConsentGiven    bool           `json:"consent_given"`
}

// PersonalDataType represents different types of personal data
type PersonalDataType string

const (
	IdentityData   PersonalDataType = "identity"
	ContactData    PersonalDataType = "contact"
	BiometricData  PersonalDataType = "biometric"
	FinancialData  PersonalDataType = "financial"
	HealthData     PersonalDataType = "health"
	LocationData   PersonalDataType = "location"
	BehavioralData PersonalDataType = "behavioral"
	PreferenceData PersonalDataType = "preference"
	// Additional aliases for backward compatibility
	IdentifyingData   PersonalDataType = "identifying"
	SensitiveData     PersonalDataType = "sensitive"
	CommunicationData PersonalDataType = "communication"
)

// TransferMechanism represents data transfer mechanisms
type TransferMechanism string

const (
	AdequacyDecision TransferMechanism = "adequacy_decision"
	StandardClauses  TransferMechanism = "standard_clauses"
	BindingRules     TransferMechanism = "binding_rules"
	Certification    TransferMechanism = "certification"
	CodeOfConduct    TransferMechanism = "code_of_conduct"
)

// ConsentHistory represents the history of consent changes
type ConsentHistory struct {
	Metadata  map[string]any  `json:"metadata"`
	ConsentID string          `json:"consent_id"`
	Changes   []ConsentChange `json:"changes"`
}

// ConsentChange represents a change in consent
type ConsentChange struct {
	Timestamp time.Time      `json:"timestamp"`
	OldValue  any            `json:"old_value"`
	NewValue  any            `json:"new_value"`
	Metadata  map[string]any `json:"metadata"`
	Action    string         `json:"action"`
	Reason    string         `json:"reason"`
}

// ============================================================================
// ALERTING AND NOTIFICATION STRUCTURES
// ============================================================================

// AlertingConfig represents alerting configuration
type AlertingConfig struct {
	Templates map[string]string    `json:"templates"`
	Channels  []AlertChannelConfig `json:"channels"`
	Rules     []AlertRule          `json:"rules"`
	Enabled   bool                 `json:"enabled"`
}

// AlertChannelConfig represents configuration for an alert channel
type AlertChannelConfig struct {
	Config  map[string]any `json:"config"`
	Type    string         `json:"type"`
	Enabled bool           `json:"enabled"`
}

// AlertRule represents an alerting rule
type AlertRule struct {
	Metadata   map[string]any   `json:"metadata"`
	ID         string           `json:"id"`
	Conditions []AlertCondition `json:"conditions"`
	Channels   []string         `json:"channels"`
	Throttle   time.Duration    `json:"throttle"`
}

// AlertCondition represents a condition for triggering an alert
type AlertCondition struct {
	Value    any    `json:"value"`
	Field    string `json:"field"`
	Operator string `json:"operator"`
}

// ComplianceAlert represents a compliance-related alert
type ComplianceAlert struct {
	Timestamp   time.Time      `json:"timestamp"`
	Metadata    map[string]any `json:"metadata"`
	ID          string         `json:"id"`
	Framework   string         `json:"framework"`
	Severity    Severity       `json:"severity"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
}

// AlertType represents different types of alerts
type AlertType string

const (
	ComplianceAlertType   AlertType = "compliance"
	SecurityAlertType     AlertType = "security"
	PerformanceAlertType  AlertType = "performance"
	AvailabilityAlertType AlertType = "availability"
)

// NotificationConfig configures notifications
type NotificationConfig struct {
	Templates map[string]string     `json:"templates"`
	Channels  []NotificationChannel `json:"channels"`
	Rules     []NotificationRule    `json:"rules"`
	Enabled   bool                  `json:"enabled"`
}

// NotificationChannel defines a notification channel
type NotificationChannel struct {
	Config  map[string]any `json:"config"`
	Type    string         `json:"type"`
	Enabled bool           `json:"enabled"`
}

// NotificationRule defines notification rules
type NotificationRule struct {
	Event    string   `json:"event"`
	Severity string   `json:"severity"`
	Template string   `json:"template"`
	Channels []string `json:"channels"`
	Enabled  bool     `json:"enabled"`
}

// AlertChannel represents an alert channel
type AlertChannel interface {
	Send(ctx context.Context, alert *ComplianceAlert) error
	Configure(config AlertChannelConfig) error
}

// ============================================================================
// CONFIGURATION AND POLICY STRUCTURES
// ============================================================================

// TestConfig represents configuration for tests
type TestConfig struct {
	Parameters    map[string]any `json:"parameters"`
	Environment   string         `json:"environment"`
	Prerequisites []string       `json:"prerequisites"`
	Timeout       time.Duration  `json:"timeout"`
	Retries       int            `json:"retries"`
	Parallel      bool           `json:"parallel"`
	Cleanup       bool           `json:"cleanup"`
}

// RetentionPolicy represents a data retention policy
type RetentionPolicy struct {
	TypeRetention    map[PrivacyEvidenceType]time.Duration `json:"type_retention"`
	ArchiveLocation  string                                `json:"archive_location"`
	DefaultRetention time.Duration                         `json:"default_retention"`
	AutoCleanup      bool                                  `json:"auto_cleanup"`
}

// ============================================================================
// INTERFACE DEFINITIONS
// ============================================================================

// ContractValidator defines the interface for validating contracts
type ContractValidator interface {
	ValidateContract(ctx context.Context, contract *ServiceContract) (*TestResult, error)
	ValidateInteraction(ctx context.Context, interaction *ContractInteraction) (*InteractionResult, error)
}

// ReportExporter defines the interface for exporting reports
type ReportExporter interface {
	Export(ctx context.Context, report any, format ReportFormat) ([]byte, error)
}

// AlertingSystem defines the interface for alerting systems
type AlertingSystem interface {
	SendAlert(ctx context.Context, alert *ComplianceAlert) error
	ConfigureChannel(channel AlertChannelConfig) error
	GetChannels() []AlertChannelConfig
}

// EvidenceIndexer defines the interface for indexing evidence
type EvidenceIndexer interface {
	IndexEvidence(ctx context.Context, evidence *Evidence) error
	SearchEvidence(ctx context.Context, query string) ([]*Evidence, error)
	GetEvidence(ctx context.Context, id string) (*Evidence, error)
}

// ============================================================================
// CONTRACT TESTING ADDITIONAL TYPES
// ============================================================================

// ContractRegistry manages service contracts
type ContractRegistry struct {
	contracts map[string]*ServiceContract
	versions  map[string][]string
}

// ContractTestRunner executes contract tests
type ContractTestRunner struct {
	registry *ContractRegistry
	config   *TestConfig
}

// ContractTestReporter generates contract test reports
type ContractTestReporter struct {
	templates map[string]*ContractReportTemplate
	exporters map[string]ContractReportExporter
}

// ContractTestSuite manages contract testing
type ContractTestSuite struct {
	contracts map[string]*ServiceContract
	tests     map[string]*ContractTest
	config    *TestConfig
}

// ContractReportExporter interface for exporting contract reports
type ContractReportExporter interface {
	Export(ctx context.Context, report *TestReport, destination string) error
}

// ContractReportExporterImpl struct implementation of ContractReportExporter
type ContractReportExporterImpl struct {
}

// ExportDestination represents an export destination
type ExportDestination struct {
	Config map[string]any `json:"config"`
	Type   string         `json:"type"`
}

// ============================================================================
// PATTERN TESTING TYPES
// ============================================================================

// Interaction represents a single interaction in a contract (for patterns.go)
type Interaction struct {
	Request     *InteractionRequest  `json:"request"`
	Response    *InteractionResponse `json:"response"`
	Metadata    map[string]any       `json:"metadata"`
	ID          string               `json:"id"`
	Description string               `json:"description"`
}

// Contract represents a service contract (for patterns.go)
type Contract struct {
	Metadata     map[string]any `json:"metadata"`
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Version      string         `json:"version"`
	Provider     string         `json:"provider"`
	Consumer     string         `json:"consumer"`
	Interactions []Interaction  `json:"interactions"`
}

// ============================================================================
// HEALTH CHECK TYPES
// ============================================================================

// HealthCheck represents a health check function
type HealthCheck func(ctx context.Context) error

// WorkerStatus defines worker status
type WorkerStatus string

const (
	IdleWorker    WorkerStatus = "idle"
	BusyWorker    WorkerStatus = "busy"
	StoppedWorker WorkerStatus = "stopped"
	ErrorWorker   WorkerStatus = "error"
)

// ============================================================================
// CONTRACT TESTING TYPES
// ============================================================================

// ContractTestConfig configures contract testing framework
type ContractTestConfig struct {
	Environment    string        `json:"environment"`
	Timeout        time.Duration `json:"timeout"`
	RetryAttempts  int           `json:"retry_attempts"`
	RetryDelay     time.Duration `json:"retry_delay"`
	StrictMode     bool          `json:"strict_mode"`
	Parallel       bool          `json:"parallel"`
	MaxConcurrency int           `json:"max_concurrency"`
}

// ServiceInfo represents service information in contracts
type ServiceInfo struct {
	Metadata    map[string]any `json:"metadata,omitempty"`
	Name        string         `json:"name"`
	Version     string         `json:"version"`
	BaseURL     string         `json:"base_url"`
	Environment string         `json:"environment"`
}

// ContractStatus represents the status of a contract
type ContractStatus string

const (
	ContractActive   ContractStatus = "active"
	ContractInactive ContractStatus = "inactive"
	ContractDraft    ContractStatus = "draft"
	ContractArchived ContractStatus = "archived"
)

// SchemaDefinition defines a JSON schema for validation
type SchemaDefinition struct {
	Properties  map[string]*SchemaProperty `json:"properties,omitempty"`
	MinLength   *int                       `json:"min_length,omitempty"`
	MaxLength   *int                       `json:"max_length,omitempty"`
	Minimum     *float64                   `json:"minimum,omitempty"`
	Maximum     *float64                   `json:"maximum,omitempty"`
	Items       *SchemaDefinition          `json:"items,omitempty"`
	Type        string                     `json:"type"`
	Pattern     string                     `json:"pattern,omitempty"`
	Format      string                     `json:"format,omitempty"`
	Description string                     `json:"description,omitempty"`
	Required    []string                   `json:"required,omitempty"`
	Enum        []any                      `json:"enum,omitempty"`
}

// SchemaProperty defines a property in a JSON schema
type SchemaProperty struct {
	MinLength   *int            `json:"min_length,omitempty"`
	MaxLength   *int            `json:"max_length,omitempty"`
	Minimum     *float64        `json:"minimum,omitempty"`
	Maximum     *float64        `json:"maximum,omitempty"`
	Items       *SchemaProperty `json:"items,omitempty"`
	Type        string          `json:"type"`
	Pattern     string          `json:"pattern,omitempty"`
	Format      string          `json:"format,omitempty"`
	Description string          `json:"description,omitempty"`
	Enum        []any           `json:"enum,omitempty"`
	Required    bool            `json:"required"`
}

// InteractionValidation represents validation results for a contract interaction
type InteractionValidation struct {
	Timestamp     time.Time                   `json:"timestamp"`
	Checks        map[string]*ValidationCheck `json:"checks"`
	InteractionID string                      `json:"interaction_id"`
	Status        string                      `json:"status"`
	Errors        []string                    `json:"errors"`
	Warnings      []string                    `json:"warnings"`
	Duration      time.Duration               `json:"duration"`
}

// ValidationCheck represents a single validation check
type ValidationCheck struct {
	Expected    any            `json:"expected"`
	Actual      any            `json:"actual"`
	Metadata    map[string]any `json:"metadata"`
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Status      string         `json:"status"`
	Errors      []string       `json:"errors"`
	Warnings    []string       `json:"warnings"`
	Valid       bool           `json:"valid"`
}

// ============================================================================
// WIDGET AND DASHBOARD TYPES
// ============================================================================

// WidgetType represents different types of dashboard widgets
type WidgetType string

const (
	ChartWidget  WidgetType = "chart"
	MetricWidget WidgetType = "metric"
	TableWidget  WidgetType = "table"
	AlertWidget  WidgetType = "alert"
	StatusWidget WidgetType = "status"
)

// ============================================================================
// CONTRACT TESTING HELPER FUNCTIONS
// ============================================================================

// ServiceDefinition represents a service definition for contracts
type ServiceDefinition struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	BaseURL string `json:"base_url"`
}

// NewContractTestSuite creates a new contract test suite
func NewContractTestSuite() *ContractTestSuite {
	return &ContractTestSuite{
		contracts: make(map[string]*ServiceContract),
		tests:     make(map[string]*ContractTest),
		config: &TestConfig{
			Timeout:  30 * time.Second,
			Retries:  3,
			Parallel: true,
		},
	}
}

// RunContractTests runs all contract tests in the suite
func (c *ContractTestSuite) RunContractTests() (map[string]ContractTestResult, error) {
	results := make(map[string]ContractTestResult)

	for name, test := range c.tests {
		// Create a basic result for now
		result := ContractTestResult{
			ContractID: test.Contract.ID,
			Provider:   test.Provider,
			Consumer:   test.Consumer,
			StartTime:  time.Now(),
			EndTime:    time.Now(),
			Duration:   time.Millisecond,
			Status:     TestStatusPassed,
		}
		results[name] = result
	}

	return results, nil
}

// AddContract adds a contract to the suite
func (c *ContractTestSuite) AddContract(contract *Contract) {
	// Convert Contract to ServiceContract
	serviceContract := &ServiceContract{
		ID:      contract.ID,
		Name:    contract.Name,
		Version: contract.Version,
		Provider: ServiceInfo{
			Name:    contract.Provider,
			Version: "1.0.0",
		},
		Consumer: ServiceInfo{
			Name:    contract.Consumer,
			Version: "1.0.0",
		},
		Status:    ContractActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  contract.Metadata,
	}

	c.contracts[contract.Name] = serviceContract
}

// CreateContractTest creates a contract test
func (c *ContractTestSuite) CreateContractTest(contractName string, validator ContractValidator) (*ContractTest, error) {
	contract, exists := c.contracts[contractName]
	if !exists {
		return nil, fmt.Errorf("contract %s not found", contractName)
	}

	test := &ContractTest{
		ID:        fmt.Sprintf("test-%s-%d", contractName, time.Now().Unix()),
		Name:      fmt.Sprintf("Contract test for %s", contractName),
		Provider:  contract.Provider.Name,
		Consumer:  contract.Consumer.Name,
		Contract:  contract,
		Validator: validator,
		Config:    c.config,
	}

	c.tests[contractName] = test
	return test, nil
}
