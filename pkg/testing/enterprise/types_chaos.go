package enterprise

import (
	"context"
	"fmt"
	"time"
)

// ============================================================================
// CHAOS ENGINEERING TYPES
// ============================================================================

// ChaosExperimentResult represents the result of a chaos experiment
type ChaosExperimentResult struct {
	StartTime    time.Time        `json:"start_time"`
	EndTime      time.Time        `json:"end_time"`
	Metrics      map[string]any   `json:"metrics"`
	Metadata     map[string]any   `json:"metadata"`
	ID           string           `json:"id"`
	ExperimentID string           `json:"experiment_id"`
	Status       ExperimentStatus `json:"status"`
	FaultType    FaultType        `json:"fault_type"`
	Target       string           `json:"target"`
	Errors       []string         `json:"errors"`
	Duration     time.Duration    `json:"duration"`
}

// ExperimentStatus represents the status of a chaos experiment
type ExperimentStatus string

const (
	ExperimentPending   ExperimentStatus = "pending"
	ExperimentRunning   ExperimentStatus = "running"
	ExperimentCompleted ExperimentStatus = "completed"
	ExperimentFailed    ExperimentStatus = "failed"
	ExperimentAborted   ExperimentStatus = "aborted"
)

// Additional experiment status constants
const (
	ExperimentStatusPending   = ExperimentPending
	ExperimentStatusRunning   = ExperimentRunning
	ExperimentStatusCompleted = ExperimentCompleted
	ExperimentStatusFailed    = ExperimentFailed
	ExperimentStatusAborted   = ExperimentAborted
	CompletedExperimentStatus = ExperimentCompleted
)

// FaultType represents different types of faults that can be injected
type FaultType string

const (
	LatencyFault       FaultType = "latency"
	NetworkPartition   FaultType = "network_partition"
	ServiceUnavailable FaultType = "service_unavailable"
	TimeoutFault       FaultType = "timeout"
	ErrorFault         FaultType = "error"
	ResourceExhaustion FaultType = "resource_exhaustion"
	CPUStressFault     FaultType = "cpu_stress"
	MemoryStressFault  FaultType = "memory_stress"
	DiskStressFault    FaultType = "disk_stress"
	PodKillFault       FaultType = "pod_kill"
	ContainerKillFault FaultType = "container_kill"
)

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

// FaultDefinition defines a fault injection configuration
type FaultDefinition struct {
	Parameters  map[string]any  `json:"parameters"`
	Recovery    *RecoveryConfig `json:"recovery,omitempty"`
	Metadata    map[string]any  `json:"metadata,omitempty"`
	ID          string          `json:"id"`
	Type        FaultType       `json:"type"`
	Target      string          `json:"target"`
	Severity    Severity        `json:"severity"`
	Duration    time.Duration   `json:"duration"`
	Probability float64         `json:"probability"`
	Enabled     bool            `json:"enabled"`
}

// ExperimentTarget represents a target for chaos experiments
type ExperimentTarget struct {
	Labels     map[string]string `json:"labels,omitempty"`
	Metadata   map[string]any    `json:"metadata,omitempty"`
	Type       string            `json:"type"`
	Name       string            `json:"name"`
	Identifier string            `json:"identifier"`
	Scope      TargetScope       `json:"scope"`
	Namespace  string            `json:"namespace,omitempty"`
	Selector   string            `json:"selector,omitempty"`
}

// TargetScope defines the scope of experiment targets
type TargetScope string

const (
	SingleScope   TargetScope = "single"
	MultipleScope TargetScope = "multiple"
	ClusterScope  TargetScope = "cluster"
	RegionScope   TargetScope = "region"
)

// Target scopes (additional to existing ones)
const (
	SingleInstanceScope TargetScope = "single_instance"
)

// FaultStatus represents the status of a fault injection
type FaultStatus struct {
	StartTime time.Time      `json:"start_time"`
	Impact    map[string]any `json:"impact"`
	Metadata  map[string]any `json:"metadata"`
	Duration  time.Duration  `json:"duration"`
	Active    bool           `json:"active"`
}

// FaultState represents the state of a fault injection
type FaultState string

const (
	FaultPending   FaultState = "pending"
	FaultActive    FaultState = "active"
	FaultCompleted FaultState = "completed"
	FaultFailed    FaultState = "failed"
	FaultAborted   FaultState = "aborted"
)

// ValidationViolation represents a validation violation
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
type ChaosExperiment struct {
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	StartTime   time.Time         `json:"start_time"`
	EndTime     time.Time         `json:"end_time"`
	Metadata    map[string]any    `json:"metadata"`
	Target      ExperimentTarget  `json:"target"`
	Type        FaultType         `json:"type"`
	Status      ExperimentStatus  `json:"status"`
	Description string            `json:"description"`
	Name        string            `json:"name"`
	ID          string            `json:"id"`
	Hypothesis  string            `json:"hypothesis"`
	Faults      []FaultDefinition `json:"faults"`
	Fault       FaultDefinition   `json:"fault"`
	Duration    time.Duration     `json:"duration"`
}

// LatencyFaultConfig represents a network latency fault configuration
type LatencyFaultConfig struct {
	Target   string        `json:"target"`
	Latency  time.Duration `json:"latency"`
	Jitter   time.Duration `json:"jitter"`
	Duration time.Duration `json:"duration"`
}

// NetworkPartitionConfig represents a network partition fault configuration
type NetworkPartitionConfig struct {
	Mode     string        `json:"mode"`
	Targets  []string      `json:"targets"`
	Duration time.Duration `json:"duration"`
}

// ErrorFaultConfig represents an error injection fault configuration
type ErrorFaultConfig struct {
	ErrorType string        `json:"error_type"`
	Target    string        `json:"target"`
	ErrorRate float64       `json:"error_rate"`
	Duration  time.Duration `json:"duration"`
}

// PendingExperiment represents an experiment in pending state
type PendingExperiment struct {
	ScheduledAt time.Time        `json:"scheduled_at"`
	Experiment  *ChaosExperiment `json:"experiment"`
	Metadata    map[string]any   `json:"metadata"`
	ID          string           `json:"id"`
	Priority    int              `json:"priority"`
}

// ChaosConfig configures chaos testing parameters (simple version for basic chaos testing)
type ChaosConfig struct {
	MaxDuration     time.Duration `json:"max_duration"`
	RecoveryTimeout time.Duration `json:"recovery_timeout"`
	FailureRate     float64       `json:"failure_rate"`
	Enabled         bool          `json:"enabled"`
}

// ChaosEngineeringConfig configures the comprehensive chaos engineering framework
type ChaosEngineeringConfig struct {
	Metadata                 map[string]any     `json:"metadata"`
	Environment              string             `json:"environment"`
	Security                 SecurityConfig     `json:"security"`
	Notifications            NotificationConfig `json:"notifications"`
	MaxConcurrentExperiments int                `json:"max_concurrent_experiments"`
	DefaultTimeout           time.Duration      `json:"default_timeout"`
	MonitoringInterval       time.Duration      `json:"monitoring_interval"`
	RetentionPeriod          time.Duration      `json:"retention_period"`
	SafetyMode               bool               `json:"safety_mode"`
}

// SecurityConfig configures security settings
type SecurityConfig struct {
	MaxSeverity       string   `json:"max_severity"`
	ApprovedTargets   []string `json:"approved_targets"`
	ForbiddenTargets  []string `json:"forbidden_targets"`
	RequireApproval   bool     `json:"require_approval"`
	AuditLogging      bool     `json:"audit_logging"`
	EncryptionEnabled bool     `json:"encryption_enabled"`
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
type ChaosEngineeringFramework struct {
	config      *ChaosEngineeringConfig
	experiments map[string]*ChaosExperiment
	injectors   map[string]FaultInjector
	monitors    map[string]any
	scheduler   *ChaosScheduler
	executor    *ExperimentExecutor
	reporter    *ChaosReporter
	metrics     *ResilienceMetrics
}

// ChaosScheduler handles experiment scheduling
type ChaosScheduler struct {
	experiments map[string]*PendingExperiment
	config      *ChaosEngineeringConfig
	executor    *ExperimentExecutor
}

// ExperimentExecutor executes chaos experiments
type ExperimentExecutor struct {
	config    *ChaosEngineeringConfig
	injectors map[string]FaultInjector
	workers   map[string]any
	results   map[string]*ExperimentResults
	queue     []any
}

// ChaosReporter generates chaos experiment reports
type ChaosReporter struct {
	templates  map[string]*ReportTemplate
	exporters  map[string]ReportExporter
	generators map[string]any
}

// FaultInjector interface for fault injection
type FaultInjector interface {
	InjectFault(ctx context.Context, fault *FaultDefinition) error
	RemoveFault(ctx context.Context, faultID string) error
	GetStatus(ctx context.Context, faultID string) (*FaultStatus, error)
}

// ============================================================================
// CONTRACT VALIDATION ADDITIONAL TYPES
// ============================================================================

// ContractValidationResult represents contract validation results
type ContractValidationResult struct {
	Timestamp   time.Time                         `json:"timestamp"`
	Validations map[string]*InteractionValidation `json:"validations"`
	Summary     *ValidationSummary                `json:"summary"`
	Metadata    map[string]any                    `json:"metadata"`
	ID          string                            `json:"id"`
	ContractID  string                            `json:"contract_id"`
	Status      TestStatus                        `json:"status"`
	Errors      []string                          `json:"errors"`
	Warnings    []string                          `json:"warnings"`
	Duration    time.Duration                     `json:"duration"`
}

// ValidationSummary represents a summary of validation results
type ValidationSummary struct {
	TotalInteractions   int     `json:"total_interactions"`
	ValidInteractions   int     `json:"valid_interactions"`
	InvalidInteractions int     `json:"invalid_interactions"`
	TotalChecks         int     `json:"total_checks"`
	PassedChecks        int     `json:"passed_checks"`
	FailedChecks        int     `json:"failed_checks"`
	SuccessRate         float64 `json:"success_rate"`
}

// ============================================================================
// HELPER FUNCTIONS AND CONSTRUCTORS
// ============================================================================

// NewChaosEngineeringFramework creates a new chaos engineering framework
func NewChaosEngineeringFramework(config *ChaosEngineeringConfig) *ChaosEngineeringFramework {
	return &ChaosEngineeringFramework{
		config:      config,
		experiments: make(map[string]*ChaosExperiment),
		injectors:   make(map[string]FaultInjector),
		monitors:    make(map[string]any),
		scheduler:   NewChaosScheduler(config),
		executor:    NewExperimentExecutor(config),
		reporter:    NewChaosReporter(),
		metrics:     &ResilienceMetrics{},
	}
}

// NewChaosScheduler creates a new chaos scheduler
func NewChaosScheduler(config *ChaosEngineeringConfig) *ChaosScheduler {
	return &ChaosScheduler{
		experiments: make(map[string]*PendingExperiment),
		config:      config,
		executor:    NewExperimentExecutor(config),
	}
}

// NewExperimentExecutor creates a new experiment executor
func NewExperimentExecutor(config *ChaosEngineeringConfig) *ExperimentExecutor {
	return &ExperimentExecutor{
		config:    config,
		injectors: make(map[string]FaultInjector),
		workers:   make(map[string]any),
		queue:     make([]any, 0),
		results:   make(map[string]*ExperimentResults),
	}
}

// NewChaosReporter creates a new chaos reporter
func NewChaosReporter() *ChaosReporter {
	reporter := &ChaosReporter{
		templates:  make(map[string]*ReportTemplate),
		exporters:  make(map[string]ReportExporter),
		generators: make(map[string]any),
	}

	// Initialize default templates
	reporter.templates["experiment"] = &ReportTemplate{
		ID:          "experiment_template",
		Name:        "Chaos Experiment Report",
		Framework:   "chaos",
		Type:        ChaosReportType,
		Format:      JSONFormat,
		Description: "Standard chaos experiment report template",
		Sections: []ReportSection{
			{
				ID:          "summary",
				Title:       "Executive Summary",
				Description: "High-level overview of experiment results",
				Type:        SummarySection,
			},
			{
				ID:          "details",
				Title:       "Experiment Details",
				Description: "Detailed experiment configuration and results",
				Type:        DetailSection,
			},
			{
				ID:          "recommendations",
				Title:       "Recommendations",
				Description: "Recommendations based on experiment results",
				Type:        RecommendationSection,
			},
		},
		Metadata: make(map[string]any),
	}

	return reporter
}

// RunExperiment runs a chaos experiment
func (f *ChaosEngineeringFramework) RunExperiment(_ context.Context, experiment *ChaosExperiment) (*ExperimentResults, error) {
	// Validate experiment first
	if err := f.validateExperiment(experiment); err != nil {
		return nil, fmt.Errorf("experiment validation failed: %w", err)
	}

	// Store the experiment in the framework
	f.experiments[experiment.ID] = experiment

	// Implementation would go here
	return &ExperimentResults{
		ExperimentID:    experiment.ID,
		Status:          CompletedExperimentStatus,
		StartTime:       time.Now(),
		EndTime:         time.Now(),
		Duration:        time.Minute,
		Summary:         "Experiment completed successfully",
		HypothesisValid: true,
		Observations:    []Observation{},
		Failures:        []ExperimentFailure{},
		Recovery: &RecoveryResults{
			Successful: true,
			Attempted:  true,
			Duration:   30 * time.Second,
		},
		Metrics:         make(map[string]any),
		Recommendations: []string{"System shows good resilience"},
	}, nil
}

// validateExperiment validates a chaos experiment
func (f *ChaosEngineeringFramework) validateExperiment(experiment *ChaosExperiment) error {
	if experiment.ID == "" {
		return fmt.Errorf("experiment ID is required")
	}
	if experiment.Name == "" {
		return fmt.Errorf("experiment name is required")
	}
	if len(experiment.Faults) == 0 {
		return fmt.Errorf("experiment must have at least one fault")
	}

	// Check forbidden targets
	for _, forbidden := range f.config.Security.ForbiddenTargets {
		if experiment.Target.Identifier == forbidden {
			return fmt.Errorf("target %s is forbidden", forbidden)
		}
	}

	return nil
}

// generateRecommendations generates recommendations based on experiment results
func (f *ChaosEngineeringFramework) generateRecommendations(experiment *ChaosExperiment, results *ExperimentResults) []string {
	if experiment == nil || results == nil {
		return []string{"Unable to generate recommendations due to invalid data"}
	}

	var recs []string
	recs = append(recs, f.baseOutcomeRecs(results)...)
	recs = append(recs, f.failureSeverityRecs(results)...)
	recs = append(recs, f.typeSpecificRecs(experiment)...)

	if len(recs) == 0 {
		recs = append(recs, "System performed well - continue regular chaos testing")
	}
	return recs
}

func (f *ChaosEngineeringFramework) baseOutcomeRecs(results *ExperimentResults) []string {
	if len(results.Failures) == 0 && results.Recovery != nil && results.Recovery.Successful {
		return []string{"System demonstrates good resilience to this type of failure"}
	}
	if results.Recovery != nil && !results.Recovery.Successful {
		return []string{"Recovery mechanisms need improvement"}
	}
	return nil
}

func (f *ChaosEngineeringFramework) failureSeverityRecs(results *ExperimentResults) []string {
	if len(results.Failures) == 0 {
		return nil
	}
	recs := []string{"Consider implementing additional error handling and recovery mechanisms"}
	for _, failure := range results.Failures {
		switch failure.Severity {
		case CriticalSeverity:
			recs = append(recs, "Critical failures detected - immediate action required")
		case HighSeverity:
			recs = append(recs, "High severity issues found - prioritize fixes")
		}
	}
	return recs
}

func (f *ChaosEngineeringFramework) typeSpecificRecs(experiment *ChaosExperiment) []string {
	switch experiment.Type {
	case NetworkChaos:
		return []string{"Consider implementing circuit breakers and retry logic"}
	case ServiceChaos:
		return []string{"Evaluate service dependencies and fallback mechanisms"}
	case ResourceChaos:
		return []string{"Review resource allocation and scaling policies"}
	default:
		return nil
	}
}

// ============================================================================
// HELPER FUNCTIONS FOR CHAOS ENGINEERING
// ============================================================================

// ResilienceMetrics represents metrics for measuring system resilience
type ResilienceMetrics struct {
	LastIncident    time.Time      `json:"last_incident"`
	LastUpdated     time.Time      `json:"last_updated"`
	Trends          map[string]any `json:"trends"`
	MTTR            time.Duration  `json:"mttr"`
	MTBF            time.Duration  `json:"mtbf"`
	Availability    float64        `json:"availability"`
	ErrorBudget     float64        `json:"error_budget"`
	IncidentCount   int            `json:"incident_count"`
	ResilienceScore float64        `json:"resilience_score"`
	ExperimentCount int            `json:"experiment_count"`
}

// BlastRadius represents the potential impact scope of a chaos experiment
type BlastRadius struct {
	Impact   map[string]any `json:"impact"`
	Scope    string         `json:"scope"`
	Severity string         `json:"severity"`
}

// CalculateResilienceScore calculates a resilience score based on experiment results
func CalculateResilienceScore(results *ExperimentResults) float64 {
	if results == nil {
		return 0.0
	}

	baseScore := 100.0

	// Deduct points for failures
	if len(results.Failures) > 0 {
		baseScore -= float64(len(results.Failures)) * 10.0
	}

	// Deduct points if hypothesis was invalid
	if !results.HypothesisValid {
		baseScore -= 20.0
	}

	// Adjust based on recovery success
	if results.Recovery != nil && !results.Recovery.Successful {
		baseScore -= 15.0
	}

	// Ensure score is between 0 and 100
	if baseScore < 0 {
		baseScore = 0
	}

	return baseScore
}

// GenerateBlastRadius generates a blast radius assessment for chaos experiments
func GenerateBlastRadius(experiment *ChaosExperiment) *BlastRadius {
	if experiment == nil {
		return &BlastRadius{
			Scope:    "unknown",
			Severity: "low",
			Impact:   make(map[string]any),
		}
	}

	severity := string(LowSeverity)
	scope := "service"

	// Determine severity based on fault type
	switch experiment.Type {
	case NetworkChaos:
		severity = string(MediumSeverity)
		scope = "network"
	case ServiceChaos:
		severity = string(HighSeverity)
		scope = "service"
	case DatabaseChaos:
		severity = string(HighSeverity)
		scope = "data"
	case ResourceChaos:
		severity = string(MediumSeverity)
		scope = "infrastructure"
	case StorageChaos:
		severity = string(HighSeverity)
		scope = "data"
	}

	return &BlastRadius{
		Scope:    scope,
		Severity: severity,
		Impact: map[string]any{
			"target":    experiment.Target.Name,
			"type":      string(experiment.Type),
			"duration":  experiment.Duration.String(),
			"namespace": experiment.Target.Namespace,
		},
	}
}

// validateHypothesis validates the hypothesis of a chaos experiment
func (f *ChaosEngineeringFramework) validateHypothesis(hypothesis *ExperimentHypothesis, results *ExperimentResults) bool {
	if hypothesis == nil || results == nil {
		return false
	}

	// Check if the expected behavior matches actual results
	if results.Status != CompletedExperimentStatus {
		return false
	}

	// If there were critical failures, hypothesis is invalid
	for _, failure := range results.Failures {
		if failure.Severity == CriticalSeverity {
			return false
		}
	}

	// Check recovery expectations
	if hypothesis.ExpectedRecovery != nil && results.Recovery != nil {
		if hypothesis.ExpectedRecovery.Timeout > 0 &&
			results.Recovery.Duration > hypothesis.ExpectedRecovery.Timeout {
			return false
		}
	}

	return true
}

// calculateImpact calculates the impact of a chaos experiment
func (f *ChaosEngineeringFramework) calculateImpact(experiment *ChaosExperiment, results *ExperimentResults) string {
	if experiment == nil || results == nil {
		return "unknown"
	}

	// Base impact on failures and recovery
	failureCount := len(results.Failures)
	criticalFailures := 0

	for _, failure := range results.Failures {
		if failure.Severity == CriticalSeverity {
			criticalFailures++
		}
	}

	switch {
	case criticalFailures > 0:
		return "critical"
	case failureCount > 5:
		return "high"
	case failureCount > 0:
		return "medium"
	default:
		return "low"
	}
}

// generateExperimentSummary generates a summary of the experiment
func (f *ChaosEngineeringFramework) generateExperimentSummary(experiment *ChaosExperiment, results *ExperimentResults) string {
	if experiment == nil || results == nil {
		return "No experiment data available"
	}

	summary := fmt.Sprintf("Chaos Experiment '%s' (%s) - Status: %s",
		experiment.Name,
		experiment.ID,
		results.Status)

	if len(results.Failures) > 0 {
		summary += fmt.Sprintf(", Failures: %d", len(results.Failures))
	}

	if results.Recovery != nil {
		if results.Recovery.Successful {
			summary += fmt.Sprintf(", Recovery: Successful (%.2fs)", results.Recovery.Duration.Seconds())
		} else {
			summary += ", Recovery: Failed"
		}
	}

	summary += fmt.Sprintf(", Duration: %.2fs", results.Duration.Seconds())

	if results.HypothesisValid {
		summary += ", Hypothesis: Valid"
	} else {
		summary += ", Hypothesis: Invalid"
	}

	return summary
}

// Prevent unused warnings for optional analysis helpers by referencing them.
var (
	_ = (*ChaosEngineeringFramework).generateRecommendations
	_ = (*ChaosEngineeringFramework).baseOutcomeRecs
	_ = (*ChaosEngineeringFramework).failureSeverityRecs
	_ = (*ChaosEngineeringFramework).typeSpecificRecs
	_ = (*ChaosEngineeringFramework).validateHypothesis
	_ = (*ChaosEngineeringFramework).calculateImpact
	_ = (*ChaosEngineeringFramework).generateExperimentSummary
)
