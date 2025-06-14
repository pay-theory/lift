package security

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"
)

// Error constants
var (
	ErrConsentNotFound = errors.New("consent not found")
	ErrInvalidEmail    = errors.New("invalid email address")
)

// GDPRConsentManager provides comprehensive GDPR consent management
type GDPRConsentManager struct {
	config               GDPRConsentConfig
	consentStore         ConsentStore
	dataSubjectRights    DataSubjectRightsHandler
	privacyAssessment    PrivacyImpactAssessment
	crossBorderValidator CrossBorderValidator
	auditLogger          GDPRAuditLogger
	mu                   sync.RWMutex
}

// GDPRConsentConfig configuration for GDPR consent management
type GDPRConsentConfig struct {
	Enabled                  bool                     `json:"enabled"`
	ConsentRenewalDays       int                      `json:"consent_renewal_days"`
	AutomaticConsentRenewal  bool                     `json:"automatic_consent_renewal"`
	GranularConsentRequired  bool                     `json:"granular_consent_required"`
	ConsentWithdrawalEnabled bool                     `json:"consent_withdrawal_enabled"`
	DataPortabilityEnabled   bool                     `json:"data_portability_enabled"`
	RightToErasureEnabled    bool                     `json:"right_to_erasure_enabled"`
	BreachNotificationHours  int                      `json:"breach_notification_hours"`
	DataRetentionPolicies    map[string]time.Duration `json:"data_retention_policies"`
	CrossBorderTransferRules []CrossBorderRule        `json:"cross_border_transfer_rules"`
	PrivacyByDesignEnabled   bool                     `json:"privacy_by_design_enabled"`
	// Additional fields needed by tests
	ConsentExpiryDays      int  `json:"consent_expiry_days"`
	RequireExplicitConsent bool `json:"require_explicit_consent"`
	RequireConsentProof    bool `json:"require_consent_proof"`
	DataRetentionDays      int  `json:"data_retention_days"`
	RequestProcessingDays  int  `json:"request_processing_days"`
	ConsentProofRequired   bool `json:"consent_proof_required"`
}

// ConsentStore interface for storing and retrieving consent data
type ConsentStore interface {
	StoreConsent(ctx context.Context, consent *ConsentRecord) error
	GetConsent(ctx context.Context, dataSubjectID, purpose string) (*ConsentRecord, error)
	GetAllConsents(ctx context.Context, dataSubjectID string) ([]*ConsentRecord, error)
	UpdateConsent(ctx context.Context, consentID string, updates *ConsentUpdates) error
	WithdrawConsent(ctx context.Context, consentID string, withdrawal *ConsentWithdrawal) error
	GetExpiredConsents(ctx context.Context) ([]*ConsentRecord, error)
	GetConsentsForRenewal(ctx context.Context) ([]*ConsentRecord, error)
	// Additional methods needed by tests
	RecordConsent(ctx context.Context, consent *ConsentRecord) error
	ListConsents(ctx context.Context, dataSubjectID string) ([]*ConsentRecord, error)
	GetConsentHistory(ctx context.Context, consentID string) ([]*ConsentHistoryEntry, error)
	CleanupExpiredConsents(ctx context.Context) error
}

// DataSubjectRightsHandler interface for handling data subject rights
type DataSubjectRightsHandler interface {
	HandleAccessRequest(ctx context.Context, request *DataAccessRequest) (*DataAccessResponse, error)
	HandlePortabilityRequest(ctx context.Context, request *DataPortabilityRequest) (*DataPortabilityResponse, error)
	HandleErasureRequest(ctx context.Context, request *DataErasureRequest) (*DataErasureResponse, error)
	HandleRectificationRequest(ctx context.Context, request *DataRectificationRequest) (*DataRectificationResponse, error)
	HandleObjectionRequest(ctx context.Context, request *DataObjectionRequest) (*DataObjectionResponse, error)
	GetRequestStatus(ctx context.Context, requestID string) (*RequestStatus, error)
}

// PrivacyImpactAssessment interface for privacy impact assessments
type PrivacyImpactAssessment interface {
	ConductPIA(ctx context.Context, assessment *PIARequest) (*PIAResult, error)
	GetPIATemplate(processingType string) (*PIATemplate, error)
	ValidateDataProcessing(ctx context.Context, processing *DataProcessingActivity) (*ProcessingValidation, error)
	GetRiskAssessment(ctx context.Context, activityID string) (*RiskAssessment, error)
	// Additional methods needed by tests
	UpdatePIA(ctx context.Context, piaID string, updates *PIAUpdate) error
	GetPIA(ctx context.Context, piaID string) (*PIAResult, error)
	ListPIAs(ctx context.Context, filters *PIAFilters) ([]*PIAResult, error)
}

// CrossBorderValidator interface for cross-border data transfer validation
type CrossBorderValidator interface {
	ValidateTransfer(ctx context.Context, transfer *CrossBorderTransfer) (*TransferValidation, error)
	GetAdequacyDecisions() ([]AdequacyDecision, error)
	ValidateStandardContractualClauses(ctx context.Context, clauses *SCCValidation) (*SCCResult, error)
	ValidateBindingCorporateRules(ctx context.Context, bcr *BCRValidation) (*BCRResult, error)
}

// GDPRAuditLogger interface for GDPR-specific audit logging
type GDPRAuditLogger interface {
	LogConsentEvent(ctx context.Context, event *ConsentEvent) error
	LogDataSubjectRequest(ctx context.Context, request *DataSubjectRequestLog) error
	LogDataProcessingActivity(ctx context.Context, activity *DataProcessingLog) error
	LogCrossBorderTransfer(ctx context.Context, transfer *CrossBorderTransferLog) error
	LogPrivacyBreach(ctx context.Context, breach *PrivacyBreachLog) error
}

// ConsentRecord represents a complete consent record
type ConsentRecord struct {
	ID                 string           `json:"id"`
	DataSubjectID      string           `json:"data_subject_id"`
	DataSubjectEmail   string           `json:"data_subject_email"`
	ConsentVersion     string           `json:"consent_version"`
	ConsentDate        time.Time        `json:"consent_date"`
	ConsentMethod      string           `json:"consent_method"` // "explicit", "implicit", "opt_in", "opt_out"
	ConsentScope       []ConsentPurpose `json:"consent_scope"`
	LegalBasis         string           `json:"legal_basis"`
	ProcessingPurposes []string         `json:"processing_purposes"`
	DataCategories     []string         `json:"data_categories"`
	Recipients         []DataRecipient  `json:"recipients"`
	RetentionPeriod    time.Duration    `json:"retention_period"`
	ExpiryDate         *time.Time       `json:"expiry_date,omitempty"`
	RenewalDate        *time.Time       `json:"renewal_date,omitempty"`
	WithdrawalDate     *time.Time       `json:"withdrawal_date,omitempty"`
	WithdrawalMethod   string           `json:"withdrawal_method,omitempty"`
	ConsentProof       *ConsentProof    `json:"consent_proof,omitempty"`
	Status             string           `json:"status"` // "active", "expired", "withdrawn", "renewed"
	Granular           bool             `json:"granular"`
	Specific           bool             `json:"specific"`
	Informed           bool             `json:"informed"`
	Unambiguous        bool             `json:"unambiguous"`
	// Additional fields needed by tests
	Purpose      string                 `json:"purpose,omitempty"`
	ConsentGiven bool                   `json:"consent_given"`
	Timestamp    *time.Time             `json:"timestamp,omitempty"`
	Source       string                 `json:"source,omitempty"`
	IPAddress    string                 `json:"ip_address,omitempty"`
	UserAgent    string                 `json:"user_agent,omitempty"`
	Metadata     map[string]interface{} `json:"metadata"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// ConsentPurpose represents a specific purpose for data processing
type ConsentPurpose struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Required    bool      `json:"required"`
	Consented   bool      `json:"consented"`
	ConsentDate time.Time `json:"consent_date"`
	LegalBasis  string    `json:"legal_basis"`
}

// DataRecipient represents a recipient of personal data
type DataRecipient struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Type       string   `json:"type"` // "controller", "processor", "third_party"
	Country    string   `json:"country"`
	Purposes   []string `json:"purposes"`
	Safeguards []string `json:"safeguards"`
}

// ConsentProof represents proof of consent
type ConsentProof struct {
	Type      string                 `json:"type"` // "digital_signature", "double_opt_in", "recorded_consent"
	Evidence  string                 `json:"evidence"`
	Timestamp time.Time              `json:"timestamp"`
	IPAddress string                 `json:"ip_address"`
	UserAgent string                 `json:"user_agent"`
	Method    string                 `json:"method"`
	Verified  bool                   `json:"verified"`
	Signature string                 `json:"signature,omitempty"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// ConsentUpdates represents updates to consent
type ConsentUpdates struct {
	ConsentScope    []ConsentPurpose `json:"consent_scope,omitempty"`
	Recipients      []DataRecipient  `json:"recipients,omitempty"`
	RetentionPeriod *time.Duration   `json:"retention_period,omitempty"`
	ExpiryDate      *time.Time       `json:"expiry_date,omitempty"`
	UpdatedBy       string           `json:"updated_by"`
	UpdateReason    string           `json:"update_reason"`
	// Additional fields needed by tests
	ConsentGiven bool                   `json:"consent_given,omitempty"`
	Timestamp    time.Time              `json:"timestamp,omitempty"`
	Reason       string                 `json:"reason,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// ConsentWithdrawal represents consent withdrawal
type ConsentWithdrawal struct {
	WithdrawalDate    time.Time `json:"withdrawal_date"`
	WithdrawalMethod  string    `json:"withdrawal_method"`
	Reason            string    `json:"reason,omitempty"`
	PartialWithdrawal bool      `json:"partial_withdrawal"`
	WithdrawnPurposes []string  `json:"withdrawn_purposes,omitempty"`
	RequestedBy       string    `json:"requested_by"`
	Verified          bool      `json:"verified"`
	// Additional fields needed by tests
	Timestamp time.Time              `json:"timestamp,omitempty"`
	Method    string                 `json:"method,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// DataAccessRequest represents a data subject access request
type DataAccessRequest struct {
	ID            string                `json:"id"`
	DataSubjectID string                `json:"data_subject_id"`
	Email         string                `json:"email"`
	RequestDate   time.Time             `json:"request_date"`
	RequestType   string                `json:"request_type"` // "access", "portability", "erasure", "rectification", "objection"
	Scope         []string              `json:"scope"`
	Verification  *IdentityVerification `json:"verification"`
	Status        string                `json:"status"`
	DueDate       time.Time             `json:"due_date"`
	// Additional fields needed by tests
	Timestamp   time.Time              `json:"timestamp,omitempty"`
	ContactInfo string                 `json:"contact_info,omitempty"`
	UserID      string                 `json:"user_id,omitempty"`
	Purpose     string                 `json:"purpose,omitempty"`
	Region      string                 `json:"region,omitempty"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// DataAccessResponse represents the response to a data access request
type DataAccessResponse struct {
	RequestID      string                 `json:"request_id"`
	ResponseDate   time.Time              `json:"response_date"`
	Data           map[string]interface{} `json:"data"`
	DataSources    []string               `json:"data_sources"`
	Format         string                 `json:"format"`
	DeliveryMethod string                 `json:"delivery_method"`
	Encrypted      bool                   `json:"encrypted"`
	// Additional fields needed by tests
	Status   string                 `json:"status,omitempty"`
	Metadata map[string]interface{} `json:"metadata"`
}

// DataPortabilityRequest represents a data portability request
type DataPortabilityRequest struct {
	DataAccessRequest
	TargetController string `json:"target_controller,omitempty"`
	Format           string `json:"format"` // "json", "xml", "csv"
	StructuredData   bool   `json:"structured_data"`
}

// DataPortabilityResponse represents the response to a data portability request
type DataPortabilityResponse struct {
	RequestID      string                 `json:"request_id"`
	ResponseDate   time.Time              `json:"response_date"`
	Data           map[string]interface{} `json:"data"`
	Format         string                 `json:"format"`
	StructuredData bool                   `json:"structured_data"`
	TransferMethod string                 `json:"transfer_method"`
	Metadata       map[string]interface{} `json:"metadata"`
}

// DataErasureRequest represents a data erasure request
type DataErasureRequest struct {
	DataAccessRequest
	ErasureScope   []string `json:"erasure_scope"`
	RetainForLegal bool     `json:"retain_for_legal"`
	Reason         string   `json:"reason"`
}

// DataErasureResponse represents the response to a data erasure request
type DataErasureResponse struct {
	RequestID          string    `json:"request_id"`
	ResponseDate       time.Time `json:"response_date"`
	ErasedData         []string  `json:"erased_data"`
	RetainedData       []string  `json:"retained_data"`
	RetentionReason    string    `json:"retention_reason,omitempty"`
	ThirdPartyNotified bool      `json:"third_party_notified"`
	// Additional fields needed by tests
	Status       string                 `json:"status,omitempty"`
	DataDeleted  []string               `json:"data_deleted,omitempty"`
	DeletedCount int                    `json:"deleted_count,omitempty"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// DataRectificationRequest represents a data rectification request
type DataRectificationRequest struct {
	DataAccessRequest
	IncorrectData map[string]interface{} `json:"incorrect_data"`
	CorrectedData map[string]interface{} `json:"corrected_data"`
}

// DataRectificationResponse represents the response to a data rectification request
type DataRectificationResponse struct {
	RequestID          string                 `json:"request_id"`
	ResponseDate       time.Time              `json:"response_date"`
	RectifiedData      map[string]interface{} `json:"rectified_data"`
	ThirdPartyNotified bool                   `json:"third_party_notified"`
	Metadata           map[string]interface{} `json:"metadata"`
}

// DataObjectionRequest represents a data processing objection request
type DataObjectionRequest struct {
	DataAccessRequest
	ProcessingPurposes []string `json:"processing_purposes"`
	ObjectionReason    string   `json:"objection_reason"`
	LegalGrounds       string   `json:"legal_grounds"`
}

// DataObjectionResponse represents the response to a data objection request
type DataObjectionResponse struct {
	RequestID           string                 `json:"request_id"`
	ResponseDate        time.Time              `json:"response_date"`
	ProcessingStopped   bool                   `json:"processing_stopped"`
	ContinuedProcessing []string               `json:"continued_processing,omitempty"`
	LegalJustification  string                 `json:"legal_justification,omitempty"`
	Metadata            map[string]interface{} `json:"metadata"`
}

// IdentityVerification represents identity verification for data subject requests
type IdentityVerification struct {
	Method       string                 `json:"method"`
	Verified     bool                   `json:"verified"`
	VerifiedBy   string                 `json:"verified_by"`
	VerifiedDate time.Time              `json:"verified_date"`
	Evidence     []string               `json:"evidence"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// RequestStatus represents the status of a data subject request
type RequestStatus struct {
	RequestID   string    `json:"request_id"`
	Status      string    `json:"status"`
	LastUpdated time.Time `json:"last_updated"`
	DueDate     time.Time `json:"due_date"`
	Progress    int       `json:"progress"` // percentage
	NextAction  string    `json:"next_action"`
	AssignedTo  string    `json:"assigned_to"`
	Notes       []string  `json:"notes"`
}

// PIARequest represents a privacy impact assessment request
type PIARequest struct {
	ID                 string                  `json:"id"`
	ProcessingActivity *DataProcessingActivity `json:"processing_activity"`
	AssessmentType     string                  `json:"assessment_type"`
	Scope              []string                `json:"scope"`
	RequestedBy        string                  `json:"requested_by"`
	RequestDate        time.Time               `json:"request_date"`
	DueDate            time.Time               `json:"due_date"`
	Stakeholders       []string                `json:"stakeholders"`
	// Additional fields needed by tests
	ProjectName string                 `json:"project_name,omitempty"`
	DataTypes   []string               `json:"data_types,omitempty"`
	Purpose     string                 `json:"purpose,omitempty"`
	LegalBasis  string                 `json:"legal_basis,omitempty"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// PIAResult represents the result of a privacy impact assessment
type PIAResult struct {
	AssessmentID       string              `json:"assessment_id"`
	CompletionDate     time.Time           `json:"completion_date"`
	RiskLevel          string              `json:"risk_level"`
	RiskScore          float64             `json:"risk_score"`
	Findings           []PIAFinding        `json:"findings"`
	Recommendations    []PIARecommendation `json:"recommendations"`
	MitigationMeasures []MitigationMeasure `json:"mitigation_measures"`
	ApprovalRequired   bool                `json:"approval_required"`
	ApprovedBy         string              `json:"approved_by,omitempty"`
	ApprovalDate       *time.Time          `json:"approval_date,omitempty"`
	ReviewDate         time.Time           `json:"review_date"`
	// Additional fields needed by tests
	ID        string                 `json:"id,omitempty"`
	Status    string                 `json:"status,omitempty"`
	Timestamp time.Time              `json:"timestamp,omitempty"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// PIAFinding represents a finding from a privacy impact assessment
type PIAFinding struct {
	ID          string   `json:"id"`
	Category    string   `json:"category"`
	Severity    string   `json:"severity"`
	Description string   `json:"description"`
	Impact      string   `json:"impact"`
	Likelihood  string   `json:"likelihood"`
	RiskScore   float64  `json:"risk_score"`
	Evidence    []string `json:"evidence"`
}

// PIARecommendation represents a recommendation from a privacy impact assessment
type PIARecommendation struct {
	ID          string   `json:"id"`
	Priority    string   `json:"priority"`
	Description string   `json:"description"`
	Actions     []string `json:"actions"`
	Timeline    string   `json:"timeline"`
	Owner       string   `json:"owner"`
	Status      string   `json:"status"`
}

// MitigationMeasure represents a mitigation measure
type MitigationMeasure struct {
	ID             string    `json:"id"`
	Type           string    `json:"type"`
	Description    string    `json:"description"`
	Implementation string    `json:"implementation"`
	Effectiveness  string    `json:"effectiveness"`
	Cost           string    `json:"cost"`
	Timeline       string    `json:"timeline"`
	Owner          string    `json:"owner"`
	Status         string    `json:"status"`
	ReviewDate     time.Time `json:"review_date"`
}

// PIATemplate represents a template for privacy impact assessments
type PIATemplate struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	ProcessingType   string                 `json:"processing_type"`
	Questions        []PIAQuestion          `json:"questions"`
	RiskFactors      []PIARiskFactor        `json:"risk_factors"`
	RequiredEvidence []string               `json:"required_evidence"`
	Metadata         map[string]interface{} `json:"metadata"`
}

// PIAQuestion represents a question in a PIA template
type PIAQuestion struct {
	ID         string   `json:"id"`
	Category   string   `json:"category"`
	Question   string   `json:"question"`
	Type       string   `json:"type"` // "text", "boolean", "multiple_choice", "scale"
	Required   bool     `json:"required"`
	Options    []string `json:"options,omitempty"`
	Guidance   string   `json:"guidance"`
	RiskWeight float64  `json:"risk_weight"`
}

// PIARiskFactor represents a risk factor in privacy assessment
type PIARiskFactor struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	Weight      float64 `json:"weight"`
	Threshold   float64 `json:"threshold"`
}

// DataProcessingActivity represents a data processing activity
type DataProcessingActivity struct {
	ID                string                 `json:"id"`
	Name              string                 `json:"name"`
	Description       string                 `json:"description"`
	Controller        string                 `json:"controller"`
	Processor         string                 `json:"processor,omitempty"`
	Purposes          []string               `json:"purposes"`
	LegalBasis        []string               `json:"legal_basis"`
	DataCategories    []string               `json:"data_categories"`
	DataSubjects      []string               `json:"data_subjects"`
	Recipients        []DataRecipient        `json:"recipients"`
	ThirdCountries    []string               `json:"third_countries"`
	Safeguards        []string               `json:"safeguards"`
	RetentionPeriod   time.Duration          `json:"retention_period"`
	SecurityMeasures  []string               `json:"security_measures"`
	DataSources       []string               `json:"data_sources"`
	AutomatedDecision bool                   `json:"automated_decision"`
	Profiling         bool                   `json:"profiling"`
	HighRisk          bool                   `json:"high_risk"`
	PIARequired       bool                   `json:"pia_required"`
	PIACompleted      bool                   `json:"pia_completed"`
	LastReview        time.Time              `json:"last_review"`
	NextReview        time.Time              `json:"next_review"`
	Metadata          map[string]interface{} `json:"metadata"`
}

// ProcessingValidation represents validation of data processing activity
type ProcessingValidation struct {
	Valid           bool                   `json:"valid"`
	ValidationDate  time.Time              `json:"validation_date"`
	Issues          []ValidationIssue      `json:"issues"`
	Recommendations []string               `json:"recommendations"`
	ComplianceScore float64                `json:"compliance_score"`
	RequiredActions []string               `json:"required_actions"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// ValidationIssue represents a validation issue
type ValidationIssue struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Remediation string `json:"remediation"`
}

// RiskAssessment represents a risk assessment
type RiskAssessment struct {
	ID             string                 `json:"id"`
	ActivityID     string                 `json:"activity_id"`
	AssessmentDate time.Time              `json:"assessment_date"`
	RiskLevel      string                 `json:"risk_level"`
	RiskScore      float64                `json:"risk_score"`
	RiskFactors    []AssessedRiskFactor   `json:"risk_factors"`
	Mitigations    []MitigationMeasure    `json:"mitigations"`
	ResidualRisk   float64                `json:"residual_risk"`
	Approved       bool                   `json:"approved"`
	ApprovedBy     string                 `json:"approved_by,omitempty"`
	ReviewDate     time.Time              `json:"review_date"`
	Metadata       map[string]interface{} `json:"metadata"`
}

// AssessedRiskFactor represents an assessed risk factor
type AssessedRiskFactor struct {
	PIARiskFactor
	Score      float64 `json:"score"`
	Impact     string  `json:"impact"`
	Likelihood string  `json:"likelihood"`
	Rationale  string  `json:"rationale"`
}

// CrossBorderTransfer represents a cross-border data transfer
type CrossBorderTransfer struct {
	ID                 string                 `json:"id"`
	DataExporter       string                 `json:"data_exporter"`
	DataImporter       string                 `json:"data_importer"`
	SourceCountry      string                 `json:"source_country"`
	DestinationCountry string                 `json:"destination_country"`
	DataCategories     []string               `json:"data_categories"`
	Purposes           []string               `json:"purposes"`
	LegalBasis         string                 `json:"legal_basis"`
	Safeguards         []string               `json:"safeguards"`
	AdequacyDecision   bool                   `json:"adequacy_decision"`
	SCCApplied         bool                   `json:"scc_applied"`
	BCRApplied         bool                   `json:"bcr_applied"`
	TransferDate       time.Time              `json:"transfer_date"`
	Volume             string                 `json:"volume"`
	Frequency          string                 `json:"frequency"`
	Metadata           map[string]interface{} `json:"metadata"`
}

// TransferValidation represents validation of cross-border transfer
type TransferValidation struct {
	Valid           bool                   `json:"valid"`
	ValidationDate  time.Time              `json:"validation_date"`
	LegalBasisValid bool                   `json:"legal_basis_valid"`
	SafeguardsValid bool                   `json:"safeguards_valid"`
	Issues          []ValidationIssue      `json:"issues"`
	Recommendations []string               `json:"recommendations"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// CrossBorderRule represents a rule for cross-border transfers
type CrossBorderRule struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	SourceCountries    []string `json:"source_countries"`
	DestCountries      []string `json:"dest_countries"`
	DataCategories     []string `json:"data_categories"`
	RequiredSafeguards []string `json:"required_safeguards"`
	Prohibited         bool     `json:"prohibited"`
	Conditions         []string `json:"conditions"`
}

// AdequacyDecision represents an adequacy decision
type AdequacyDecision struct {
	Country      string     `json:"country"`
	Decision     string     `json:"decision"`
	DecisionDate time.Time  `json:"decision_date"`
	ValidUntil   *time.Time `json:"valid_until,omitempty"`
	Conditions   []string   `json:"conditions"`
}

// SCCValidation represents Standard Contractual Clauses validation
type SCCValidation struct {
	ClausesVersion string                 `json:"clauses_version"`
	DataExporter   string                 `json:"data_exporter"`
	DataImporter   string                 `json:"data_importer"`
	DataCategories []string               `json:"data_categories"`
	Purposes       []string               `json:"purposes"`
	Metadata       map[string]interface{} `json:"metadata"`
}

// SCCResult represents the result of SCC validation
type SCCResult struct {
	Valid             bool                   `json:"valid"`
	ValidationDate    time.Time              `json:"validation_date"`
	ClausesApplicable bool                   `json:"clauses_applicable"`
	Issues            []ValidationIssue      `json:"issues"`
	Recommendations   []string               `json:"recommendations"`
	Metadata          map[string]interface{} `json:"metadata"`
}

// BCRValidation represents Binding Corporate Rules validation
type BCRValidation struct {
	CompanyGroup   string                 `json:"company_group"`
	BCRVersion     string                 `json:"bcr_version"`
	DataCategories []string               `json:"data_categories"`
	Purposes       []string               `json:"purposes"`
	Countries      []string               `json:"countries"`
	Metadata       map[string]interface{} `json:"metadata"`
}

// BCRResult represents the result of BCR validation
type BCRResult struct {
	Valid           bool                   `json:"valid"`
	ValidationDate  time.Time              `json:"validation_date"`
	BCRApplicable   bool                   `json:"bcr_applicable"`
	Issues          []ValidationIssue      `json:"issues"`
	Recommendations []string               `json:"recommendations"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// ConsentEvent represents a consent-related event for audit logging
type ConsentEvent struct {
	EventType     string                 `json:"event_type"`
	ConsentID     string                 `json:"consent_id"`
	DataSubjectID string                 `json:"data_subject_id"`
	Timestamp     time.Time              `json:"timestamp"`
	Details       map[string]interface{} `json:"details"`
	IPAddress     string                 `json:"ip_address"`
	UserAgent     string                 `json:"user_agent"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// DataSubjectRequestLog represents a data subject request for audit logging
type DataSubjectRequestLog struct {
	RequestID     string                 `json:"request_id"`
	RequestType   string                 `json:"request_type"`
	DataSubjectID string                 `json:"data_subject_id"`
	Timestamp     time.Time              `json:"timestamp"`
	Status        string                 `json:"status"`
	ProcessedBy   string                 `json:"processed_by"`
	Details       map[string]interface{} `json:"details"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// CrossBorderTransferLog represents a cross-border transfer for audit logging
type CrossBorderTransferLog struct {
	TransferID         string                 `json:"transfer_id"`
	DataExporter       string                 `json:"data_exporter"`
	DataImporter       string                 `json:"data_importer"`
	SourceCountry      string                 `json:"source_country"`
	DestinationCountry string                 `json:"destination_country"`
	Timestamp          time.Time              `json:"timestamp"`
	LegalBasis         string                 `json:"legal_basis"`
	Safeguards         []string               `json:"safeguards"`
	Metadata           map[string]interface{} `json:"metadata"`
}

// PrivacyBreachLog represents a privacy breach for audit logging
type PrivacyBreachLog struct {
	BreachID          string                 `json:"breach_id"`
	BreachType        string                 `json:"breach_type"`
	Severity          string                 `json:"severity"`
	DetectedDate      time.Time              `json:"detected_date"`
	ReportedDate      time.Time              `json:"reported_date"`
	AffectedSubjects  int                    `json:"affected_subjects"`
	DataCategories    []string               `json:"data_categories"`
	Cause             string                 `json:"cause"`
	Mitigation        []string               `json:"mitigation"`
	AuthorityNotified bool                   `json:"authority_notified"`
	SubjectsNotified  bool                   `json:"subjects_notified"`
	Metadata          map[string]interface{} `json:"metadata"`
}

// NewGDPRConsentManager creates a new GDPR consent manager
func NewGDPRConsentManager(config GDPRConsentConfig) *GDPRConsentManager {
	return &GDPRConsentManager{
		config: config,
	}
}

// SetConsentStore sets the consent store
func (gcm *GDPRConsentManager) SetConsentStore(store ConsentStore) {
	gcm.mu.Lock()
	defer gcm.mu.Unlock()
	gcm.consentStore = store
}

// SetDataSubjectRightsHandler sets the data subject rights handler
func (gcm *GDPRConsentManager) SetDataSubjectRightsHandler(handler DataSubjectRightsHandler) {
	gcm.mu.Lock()
	defer gcm.mu.Unlock()
	gcm.dataSubjectRights = handler
}

// SetPrivacyImpactAssessment sets the privacy impact assessment handler
func (gcm *GDPRConsentManager) SetPrivacyImpactAssessment(pia PrivacyImpactAssessment) {
	gcm.mu.Lock()
	defer gcm.mu.Unlock()
	gcm.privacyAssessment = pia
}

// SetCrossBorderValidator sets the cross-border validator
func (gcm *GDPRConsentManager) SetCrossBorderValidator(validator CrossBorderValidator) {
	gcm.mu.Lock()
	defer gcm.mu.Unlock()
	gcm.crossBorderValidator = validator
}

// SetAuditLogger sets the GDPR audit logger
func (gcm *GDPRConsentManager) SetAuditLogger(logger GDPRAuditLogger) {
	gcm.mu.Lock()
	defer gcm.mu.Unlock()
	gcm.auditLogger = logger
}

// RecordConsent records a new consent
func (gcm *GDPRConsentManager) RecordConsent(ctx context.Context, consent *ConsentRecord) error {
	gcm.mu.RLock()
	defer gcm.mu.RUnlock()

	if !gcm.config.Enabled {
		return fmt.Errorf("GDPR consent management not enabled")
	}

	if gcm.consentStore == nil {
		return fmt.Errorf("consent store not configured")
	}

	// Validate consent
	if err := gcm.validateConsent(consent); err != nil {
		return fmt.Errorf("consent validation failed: %w", err)
	}

	// Set expiry date if configured
	if gcm.config.ConsentExpiryDays > 0 && consent.ExpiryDate == nil {
		expiryDate := time.Now().AddDate(0, 0, gcm.config.ConsentExpiryDays)
		consent.ExpiryDate = &expiryDate
	}

	// Store consent
	if err := gcm.consentStore.StoreConsent(ctx, consent); err != nil {
		return fmt.Errorf("failed to store consent: %w", err)
	}

	// Log consent event
	if gcm.auditLogger != nil {
		event := &ConsentEvent{
			EventType:     "consent_recorded",
			ConsentID:     consent.ID,
			DataSubjectID: consent.DataSubjectID,
			Timestamp:     time.Now(),
			Details: map[string]interface{}{
				"consent_method": consent.ConsentMethod,
				"purposes":       consent.ProcessingPurposes,
				"legal_basis":    consent.LegalBasis,
			},
		}
		gcm.auditLogger.LogConsentEvent(ctx, event)
	}

	return nil
}

// GetConsent retrieves consent for a data subject and purpose
func (gcm *GDPRConsentManager) GetConsent(ctx context.Context, dataSubjectID, purpose string) (*ConsentRecord, error) {
	gcm.mu.RLock()
	defer gcm.mu.RUnlock()

	if dataSubjectID == "" {
		return nil, fmt.Errorf("data subject ID is required")
	}

	if gcm.consentStore == nil {
		return nil, fmt.Errorf("consent store not configured")
	}

	return gcm.consentStore.GetConsent(ctx, dataSubjectID, purpose)
}

// WithdrawConsent withdraws consent
func (gcm *GDPRConsentManager) WithdrawConsent(ctx context.Context, consentID string, withdrawal *ConsentWithdrawal) error {
	gcm.mu.RLock()
	defer gcm.mu.RUnlock()

	if consentID == "" {
		return fmt.Errorf("consent ID is required")
	}

	if withdrawal == nil {
		return fmt.Errorf("withdrawal information is required")
	}

	if !gcm.config.ConsentWithdrawalEnabled {
		return fmt.Errorf("consent withdrawal not enabled")
	}

	if gcm.consentStore == nil {
		return fmt.Errorf("consent store not configured")
	}

	// Withdraw consent
	if err := gcm.consentStore.WithdrawConsent(ctx, consentID, withdrawal); err != nil {
		return fmt.Errorf("failed to withdraw consent: %w", err)
	}

	// Log withdrawal event
	if gcm.auditLogger != nil {
		event := &ConsentEvent{
			EventType: "consent_withdrawn",
			ConsentID: consentID,
			Timestamp: time.Now(),
			Details: map[string]interface{}{
				"withdrawal_method": withdrawal.WithdrawalMethod,
				"reason":            withdrawal.Reason,
				"partial":           withdrawal.PartialWithdrawal,
			},
		}
		gcm.auditLogger.LogConsentEvent(ctx, event)
	}

	return nil
}

// ProcessDataSubjectRequest processes a data subject request
func (gcm *GDPRConsentManager) ProcessDataSubjectRequest(ctx context.Context, request *DataAccessRequest) error {
	gcm.mu.RLock()
	defer gcm.mu.RUnlock()

	if gcm.dataSubjectRights == nil {
		return fmt.Errorf("data subject rights handler not configured")
	}

	// Log request
	if gcm.auditLogger != nil {
		requestLog := &DataSubjectRequestLog{
			RequestID:     request.ID,
			RequestType:   request.RequestType,
			DataSubjectID: request.DataSubjectID,
			Timestamp:     time.Now(),
			Status:        "received",
		}
		gcm.auditLogger.LogDataSubjectRequest(ctx, requestLog)
	}

	// Process based on request type
	switch request.RequestType {
	case "access":
		_, err := gcm.dataSubjectRights.HandleAccessRequest(ctx, request)
		return err
	case "portability":
		portabilityRequest := &DataPortabilityRequest{DataAccessRequest: *request}
		_, err := gcm.dataSubjectRights.HandlePortabilityRequest(ctx, portabilityRequest)
		return err
	case "erasure":
		erasureRequest := &DataErasureRequest{DataAccessRequest: *request}
		_, err := gcm.dataSubjectRights.HandleErasureRequest(ctx, erasureRequest)
		return err
	case "rectification":
		rectificationRequest := &DataRectificationRequest{DataAccessRequest: *request}
		_, err := gcm.dataSubjectRights.HandleRectificationRequest(ctx, rectificationRequest)
		return err
	case "objection":
		objectionRequest := &DataObjectionRequest{DataAccessRequest: *request}
		_, err := gcm.dataSubjectRights.HandleObjectionRequest(ctx, objectionRequest)
		return err
	default:
		return fmt.Errorf("unsupported request type: %s", request.RequestType)
	}
}

// validateConsent validates a consent record
func (gcm *GDPRConsentManager) validateConsent(consent *ConsentRecord) error {
	if consent == nil {
		return fmt.Errorf("consent record is required")
	}

	if consent.DataSubjectID == "" {
		return fmt.Errorf("data subject ID is required")
	}

	// Check for purpose - support both legacy Purpose field and ProcessingPurposes
	if consent.Purpose == "" && len(consent.ProcessingPurposes) == 0 {
		return fmt.Errorf("purpose is required")
	}

	if consent.LegalBasis == "" {
		return fmt.Errorf("legal basis is required")
	}

	// Validate legal basis values
	validLegalBases := []string{"consent", "contract", "legal_obligation", "vital_interests", "public_task", "legitimate_interests"}
	isValidLegalBasis := false
	for _, validBasis := range validLegalBases {
		if consent.LegalBasis == validBasis {
			isValidLegalBasis = true
			break
		}
	}
	if !isValidLegalBasis {
		return fmt.Errorf("invalid legal basis")
	}

	// Check for expired consent
	if consent.ExpiryDate != nil && consent.ExpiryDate.Before(time.Now()) {
		return fmt.Errorf("consent has expired")
	}

	if gcm.config.GranularConsentRequired && !consent.Granular {
		return fmt.Errorf("granular consent is required")
	}

	if gcm.config.ConsentProofRequired && consent.ConsentProof == nil {
		return fmt.Errorf("consent proof is required")
	}

	// Validate GDPR consent requirements
	if !consent.Specific {
		return fmt.Errorf("consent must be specific")
	}

	if !consent.Informed {
		return fmt.Errorf("consent must be informed")
	}

	if !consent.Unambiguous {
		return fmt.Errorf("consent must be unambiguous")
	}

	return nil
}

// Additional types needed by tests

// ConsentUpdate represents updates to consent (alias for ConsentUpdates for test compatibility)
type ConsentUpdate = ConsentUpdates

// ConsentHistoryEntry represents a historical consent entry
type ConsentHistoryEntry struct {
	ID            string                 `json:"id"`
	ConsentID     string                 `json:"consent_id"`
	Action        string                 `json:"action"` // "created", "updated", "withdrawn", "renewed"
	Timestamp     time.Time              `json:"timestamp"`
	DataSubjectID string                 `json:"data_subject_id"`
	Changes       map[string]interface{} `json:"changes"`
	UpdatedBy     string                 `json:"updated_by"`
	Reason        string                 `json:"reason"`
	IPAddress     string                 `json:"ip_address"`
	UserAgent     string                 `json:"user_agent"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// PIAUpdate represents updates to a Privacy Impact Assessment
type PIAUpdate struct {
	RiskLevel          *string                `json:"risk_level,omitempty"`
	RiskScore          *float64               `json:"risk_score,omitempty"`
	Findings           []PIAFinding           `json:"findings,omitempty"`
	Recommendations    []PIARecommendation    `json:"recommendations,omitempty"`
	MitigationMeasures []MitigationMeasure    `json:"mitigation_measures,omitempty"`
	ApprovalRequired   *bool                  `json:"approval_required,omitempty"`
	ApprovedBy         string                 `json:"approved_by,omitempty"`
	ApprovalDate       *time.Time             `json:"approval_date,omitempty"`
	ReviewDate         *time.Time             `json:"review_date,omitempty"`
	UpdatedBy          string                 `json:"updated_by"`
	UpdateReason       string                 `json:"update_reason"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
}

// PIAFilters represents filters for PIA queries
type PIAFilters struct {
	RiskLevel       []string   `json:"risk_level,omitempty"`
	AssessmentType  []string   `json:"assessment_type,omitempty"`
	RequestedBy     []string   `json:"requested_by,omitempty"`
	DateFrom        *time.Time `json:"date_from,omitempty"`
	DateTo          *time.Time `json:"date_to,omitempty"`
	ApprovalStatus  []string   `json:"approval_status,omitempty"`
	ProcessingTypes []string   `json:"processing_types,omitempty"`
	Limit           int        `json:"limit,omitempty"`
	Offset          int        `json:"offset,omitempty"`
}

// Additional methods needed by tests

// HandleAccessRequest handles a data access request
func (gcm *GDPRConsentManager) HandleAccessRequest(ctx context.Context, request *DataAccessRequest) (*DataAccessResponse, error) {
	gcm.mu.RLock()
	defer gcm.mu.RUnlock()

	if request == nil {
		return nil, fmt.Errorf("request is required")
	}

	if request.DataSubjectID == "" {
		return nil, fmt.Errorf("data subject ID is required")
	}

	if gcm.dataSubjectRights == nil {
		return nil, fmt.Errorf("data subject rights handler not configured")
	}

	return gcm.dataSubjectRights.HandleAccessRequest(ctx, request)
}

// ConductPIA conducts a privacy impact assessment
func (gcm *GDPRConsentManager) ConductPIA(ctx context.Context, request *PIARequest) (*PIAResult, error) {
	gcm.mu.RLock()
	defer gcm.mu.RUnlock()

	if request == nil {
		return nil, fmt.Errorf("PIA request is required")
	}

	if request.ProjectName == "" {
		return nil, fmt.Errorf("project name is required")
	}

	if gcm.privacyAssessment == nil {
		return nil, fmt.Errorf("privacy impact assessment handler not configured")
	}

	return gcm.privacyAssessment.ConductPIA(ctx, request)
}

// UpdateConsent updates an existing consent
func (gcm *GDPRConsentManager) UpdateConsent(ctx context.Context, consentID string, updates *ConsentUpdate) error {
	gcm.mu.RLock()
	defer gcm.mu.RUnlock()

	if gcm.consentStore == nil {
		return fmt.Errorf("consent store not configured")
	}

	return gcm.consentStore.UpdateConsent(ctx, consentID, updates)
}

// HandleErasureRequest handles a data erasure request
func (gcm *GDPRConsentManager) HandleErasureRequest(ctx context.Context, request *DataErasureRequest) (*DataErasureResponse, error) {
	gcm.mu.RLock()
	defer gcm.mu.RUnlock()

	if gcm.dataSubjectRights == nil {
		return nil, fmt.Errorf("data subject rights handler not configured")
	}

	return gcm.dataSubjectRights.HandleErasureRequest(ctx, request)
}

// validateConsentRecord validates a consent record (alias for validateConsent for test compatibility)
func (gcm *GDPRConsentManager) validateConsentRecord(consent *ConsentRecord) error {
	return gcm.validateConsent(consent)
}

// generateConsentID generates a unique consent ID
func (gcm *GDPRConsentManager) generateConsentID() string {
	return fmt.Sprintf("consent-%d", time.Now().UnixNano())
}

// isValidEmail validates an email address
func (gcm *GDPRConsentManager) isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

// calculateExpiryDate calculates the expiry date for consent
func (gcm *GDPRConsentManager) calculateExpiryDate() time.Time {
	if gcm.config.ConsentExpiryDays > 0 {
		return time.Now().AddDate(0, 0, gcm.config.ConsentExpiryDays)
	}
	return time.Now().AddDate(1, 0, 0) // Default to 1 year
}
