package enterprise

import (
	"context"
	"time"
)

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
// GDPR ADDITIONAL TYPES
// ============================================================================

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

// AlertChannel represents an alert channel
type AlertChannel interface {
	Send(ctx context.Context, alert *ComplianceAlert) error
	Configure(config AlertChannelConfig) error
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
// FUNCTION TEMPLATE TYPES
// ============================================================================

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
// CHAOS ENGINEERING ADDITIONAL TYPES
// ============================================================================

// ExperimentHypothesis represents the hypothesis for a chaos experiment
type ExperimentHypothesis struct {
	ExpectedRecovery *RecoveryConfig `json:"expected_recovery,omitempty"`
	Metadata         map[string]any  `json:"metadata"`
	Description      string          `json:"description"`
	ExpectedBehavior string          `json:"expected_behavior"`
	SuccessCriteria  []string        `json:"success_criteria"`
}

// Chaos experiment types
const (
	NetworkChaos  FaultType = "network"
	ServiceChaos  FaultType = "service"
	ResourceChaos FaultType = "resource"
	DatabaseChaos FaultType = "database"
	StorageChaos  FaultType = "storage"
)

// Target types for chaos experiments
const (
	ServiceTarget        = "service"
	NetworkTarget        = "network"
	DatabaseTarget       = "database"
	InfrastructureTarget = "infrastructure"
)

// Target scopes (additional to existing ones)
const (
	SingleInstanceScope TargetScope = "single_instance"
)

// Observation types for chaos engineering
type ObservationType string

const (
	MetricObservation ObservationType = "metric"
	LogObservation    ObservationType = "log"
	EventObservation  ObservationType = "event"
	HealthObservation ObservationType = "health"
)

// Observation severity
type ObservationSeverity string

const (
	InfoObservationSeverity     ObservationSeverity = "info"
	WarningObservationSeverity  ObservationSeverity = "warning"
	ErrorObservationSeverity    ObservationSeverity = "error"
	CriticalObservationSeverity ObservationSeverity = "critical"
)

// RecoveryConfig represents recovery configuration for chaos experiments
type RecoveryConfig struct {
	HealthChecks  []string      `json:"health_checks"`
	Timeout       time.Duration `json:"timeout"`
	RetryAttempts int           `json:"retry_attempts"`
	RetryDelay    time.Duration `json:"retry_delay"`
	Automatic     bool          `json:"automatic"`
	Rollback      bool          `json:"rollback"`
}

// Observation represents an observation during chaos experiments
type Observation struct {
	Timestamp time.Time           `json:"timestamp"`
	Data      map[string]any      `json:"data"`
	Metadata  map[string]any      `json:"metadata"`
	ID        string              `json:"id"`
	Type      ObservationType     `json:"type"`
	Severity  ObservationSeverity `json:"severity"`
	Message   string              `json:"message"`
	Source    string              `json:"source"`
}

// ExperimentResults represents the results of a chaos experiment
type ExperimentResults struct {
	StartTime       time.Time           `json:"start_time"`
	EndTime         time.Time           `json:"end_time"`
	Metrics         map[string]any      `json:"metrics"`
	Recovery        *RecoveryResults    `json:"recovery,omitempty"`
	Metadata        map[string]any      `json:"metadata"`
	Status          ExperimentStatus    `json:"status"`
	ExperimentID    string              `json:"experiment_id"`
	Summary         string              `json:"summary"`
	Observations    []Observation       `json:"observations"`
	Failures        []ExperimentFailure `json:"failures"`
	Recommendations []string            `json:"recommendations"`
	Duration        time.Duration       `json:"duration"`
	HypothesisValid bool                `json:"hypothesis_valid"`
}

// ExperimentFailure represents a failure during chaos experiments
type ExperimentFailure struct {
	Timestamp time.Time      `json:"timestamp"`
	Metadata  map[string]any `json:"metadata"`
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Message   string         `json:"message"`
	Severity  Severity       `json:"severity"`
	Component string         `json:"component"`
}

// RecoveryResults represents recovery results
type RecoveryResults struct {
	Method     string        `json:"method"`
	Steps      []string      `json:"steps"`
	Errors     []string      `json:"errors"`
	Duration   time.Duration `json:"duration"`
	Successful bool          `json:"successful"`
	Attempted  bool          `json:"attempted"`
}

// ExperimentReport represents a chaos experiment report
type ExperimentReport struct {
	Timestamp    time.Time           `json:"timestamp"`
	Results      *ExperimentResults  `json:"results"`
	Analysis     *ExperimentAnalysis `json:"analysis"`
	Metadata     map[string]any      `json:"metadata"`
	ID           string              `json:"id"`
	ExperimentID string              `json:"experiment_id"`
	Type         ReportType          `json:"type"`
	Format       ReportFormat        `json:"format"`
}

// ExperimentAnalysis represents analysis of experiment results
type ExperimentAnalysis struct {
	Trends          map[string]any `json:"trends"`
	Comparisons     map[string]any `json:"comparisons"`
	Insights        []string       `json:"insights"`
	Recommendations []string       `json:"recommendations"`
	ResilienceScore float64        `json:"resilience_score"`
}

// ============================================================================
// CHAOS ENGINEERING FRAMEWORK TYPES
// ============================================================================

// ChaosEngineeringFramework represents the main chaos engineering framework
