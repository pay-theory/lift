package enterprise

import (
	"context"
	"time"
)

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
// HELPER TYPES AND CONSTANTS
// ============================================================================

// AlertType represents different types of alerts
type AlertType string

const (
	ComplianceAlertType   AlertType = "compliance"
	SecurityAlertType     AlertType = "security"
	PerformanceAlertType  AlertType = "performance"
	AvailabilityAlertType AlertType = "availability"
)

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

// SecurityConfig configures security settings
