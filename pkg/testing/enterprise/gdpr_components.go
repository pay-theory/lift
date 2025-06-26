package enterprise

import (
	"context"
	"fmt"
	"time"
)

// DataMapper maps and tracks personal data processing
type DataMapper struct {
	mappings   map[string]*DataMapping
	categories []PersonalDataType
	processors []DataProcessor
	purposes   []ProcessingPurpose
}

// DataMapping represents a data mapping entry
type DataMapping struct {
	ID          string                 `json:"id"`
	DataType    PersonalDataType       `json:"data_type"`
	Source      string                 `json:"source"`
	Destination string                 `json:"destination"`
	Purpose     string                 `json:"purpose"`
	LegalBasis  string                 `json:"legal_basis"`
	Retention   time.Duration          `json:"retention"`
	Processors  []string               `json:"processors"`
	Recipients  []string               `json:"recipients"`
	Safeguards  []string               `json:"safeguards"`
	Documented  bool                   `json:"documented"`
	LastUpdated time.Time              `json:"last_updated"`
	Metadata    map[string]any `json:"metadata"`
}

// DataProcessor represents a data processor
type DataProcessor struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Type        ProcessorType          `json:"type"`
	Location    string                 `json:"location"`
	Safeguards  []string               `json:"safeguards"`
	Agreements  []string               `json:"agreements"`
	Certified   bool                   `json:"certified"`
	LastAudited *time.Time             `json:"last_audited,omitempty"`
	Metadata    map[string]any `json:"metadata"`
}

// ProcessorType defines types of data processors
type ProcessorType string

const (
	InternalProcessor ProcessorType = "internal"
	ExternalProcessor ProcessorType = "external"
	SubProcessor      ProcessorType = "sub_processor"
	JointProcessor    ProcessorType = "joint_processor"
)

// ProcessingPurpose represents a data processing purpose
type ProcessingPurpose struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	LegalBasis  string                 `json:"legal_basis"`
	Categories  []PersonalDataType     `json:"categories"`
	Retention   time.Duration          `json:"retention"`
	Automated   bool                   `json:"automated"`
	Profiling   bool                   `json:"profiling"`
	Metadata    map[string]any `json:"metadata"`
}

// NewDataMapper creates a new data mapper
func NewDataMapper() *DataMapper {
	return &DataMapper{
		mappings:   make(map[string]*DataMapping),
		categories: getDefaultDataCategories(),
		processors: []DataProcessor{},
		purposes:   getDefaultProcessingPurposes(),
	}
}

// MapData creates a new data mapping
func (dm *DataMapper) MapData(ctx context.Context, mapping *DataMapping) error {
	mapping.LastUpdated = time.Now()
	dm.mappings[mapping.ID] = mapping
	return nil
}

// GetDataMappings returns all data mappings
func (dm *DataMapper) GetDataMappings(ctx context.Context) map[string]*DataMapping {
	return dm.mappings
}

// LocalConsentRecord represents a consent record (local version to avoid conflicts)
type LocalConsentRecord struct {
	ID          string                 `json:"id"`
	Purpose     string                 `json:"purpose"`
	LegalBasis  string                 `json:"legal_basis"`
	Granted     bool                   `json:"granted"`
	Timestamp   time.Time              `json:"timestamp"`
	ExpiryDate  *time.Time             `json:"expiry_date,omitempty"`
	Withdrawn   bool                   `json:"withdrawn"`
	WithdrawnAt *time.Time             `json:"withdrawn_at,omitempty"`
	Granular    bool                   `json:"granular"`
	Metadata    map[string]any `json:"metadata"`
}

// LocalConsentHistory represents consent history (local version to avoid conflicts)
type LocalConsentHistory struct {
	Timestamp time.Time              `json:"timestamp"`
	Action    string                 `json:"action"`
	Details   map[string]any `json:"details"`
}

// ConsentManager manages GDPR consent
type ConsentManager struct {
	consents    map[string]*LocalConsentRecord
	preferences map[string]*ConsentPreferences
	history     map[string][]ConsentHistory
	policies    map[string]*ConsentPolicy
}

// ConsentPreferences represents user consent preferences
type ConsentPreferences struct {
	UserID      string                 `json:"user_id"`
	Preferences map[string]bool        `json:"preferences"`
	LastUpdated time.Time              `json:"last_updated"`
	Metadata    map[string]any `json:"metadata"`
}

// ConsentPolicy represents a consent policy
type ConsentPolicy struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Version  string                 `json:"version"`
	Purposes []string               `json:"purposes"`
	Required bool                   `json:"required"`
	Granular bool                   `json:"granular"`
	Expiry   *time.Duration         `json:"expiry,omitempty"`
	Metadata map[string]any `json:"metadata"`
}

// NewConsentManager creates a new consent manager
func NewConsentManager() *ConsentManager {
	return &ConsentManager{
		consents:    make(map[string]*LocalConsentRecord),
		preferences: make(map[string]*ConsentPreferences),
		history:     make(map[string][]ConsentHistory),
		policies:    getDefaultConsentPolicies(),
	}
}

// GrantConsent grants consent for a data subject
func (cm *ConsentManager) GrantConsent(ctx context.Context, consent *LocalConsentRecord) error {
	consent.Timestamp = time.Now()
	consent.Granted = true
	cm.consents[consent.ID] = consent

	// Add to history
	history := ConsentHistory{
		ConsentID: consent.ID,
		Changes: []ConsentChange{
			{
				Timestamp: time.Now(),
				Action:    "granted",
				NewValue:  true,
				Reason:    "User granted consent",
			},
		},
	}

	cm.history[consent.ID] = append(cm.history[consent.ID], history)

	return nil
}

// WithdrawConsent withdraws consent for a data subject
func (cm *ConsentManager) WithdrawConsent(ctx context.Context, consentID string) error {
	consent, exists := cm.consents[consentID]
	if !exists {
		return fmt.Errorf("consent not found: %s", consentID)
	}

	now := time.Now()
	consent.Withdrawn = true
	consent.WithdrawnAt = &now

	// Add to history
	history := ConsentHistory{
		ConsentID: consentID,
		Changes: []ConsentChange{
			{
				Timestamp: time.Now(),
				Action:    "withdrawn",
				NewValue:  false,
				Reason:    "User withdrew consent",
			},
		},
	}

	cm.history[consentID] = append(cm.history[consentID], history)

	return nil
}

// GetConsent retrieves consent by ID
func (cm *ConsentManager) GetConsent(ctx context.Context, consentID string) (*LocalConsentRecord, error) {
	consent, exists := cm.consents[consentID]
	if !exists {
		return nil, fmt.Errorf("consent not found: %s", consentID)
	}
	return consent, nil
}

// TransferValidator validates cross-border data transfers
type TransferValidator struct {
	mechanisms map[string]*TransferMechanismConfig
	countries  map[string]*CountryInfo
	safeguards map[string]*SafeguardConfig
}

// TransferMechanismConfig configures transfer mechanisms
type TransferMechanismConfig struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Type         TransferMechanism      `json:"type"`
	Requirements []string               `json:"requirements"`
	Validity     time.Duration          `json:"validity"`
	Automated    bool                   `json:"automated"`
	Metadata     map[string]any `json:"metadata"`
}

// CountryInfo provides information about countries for transfers
type CountryInfo struct {
	Code             string                 `json:"code"`
	Name             string                 `json:"name"`
	AdequacyDecision bool                   `json:"adequacy_decision"`
	DecisionDate     *time.Time             `json:"decision_date,omitempty"`
	Restrictions     []string               `json:"restrictions"`
	Safeguards       []string               `json:"safeguards"`
	Metadata         map[string]any `json:"metadata"`
}

// SafeguardConfig configures transfer safeguards
type SafeguardConfig struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Type          string                 `json:"type"`
	Requirements  []string               `json:"requirements"`
	Effectiveness string                 `json:"effectiveness"`
	Metadata      map[string]any `json:"metadata"`
}

// NewTransferValidator creates a new transfer validator
func NewTransferValidator() *TransferValidator {
	return &TransferValidator{
		mechanisms: getDefaultTransferMechanisms(),
		countries:  getDefaultCountryInfo(),
		safeguards: getDefaultSafeguards(),
	}
}

// ValidateTransfer validates a data transfer
func (tv *TransferValidator) ValidateTransfer(ctx context.Context, transfer *DataTransfer) (*TransferValidationResult, error) {
	result := &TransferValidationResult{
		TransferID: transfer.ID,
		Valid:      true,
		Issues:     []string{},
		Timestamp:  time.Now(),
	}

	// Check destination country
	country, exists := tv.countries[transfer.Destination]
	if !exists {
		result.Valid = false
		result.Issues = append(result.Issues, fmt.Sprintf("Unknown destination country: %s", transfer.Destination))
		return result, nil
	}

	// Check adequacy decision
	if !country.AdequacyDecision {
		// Check if appropriate safeguards are in place
		if len(transfer.Safeguards) == 0 {
			result.Valid = false
			result.Issues = append(result.Issues, "No safeguards specified for transfer to non-adequate country")
		}
	}

	// Validate mechanism
	if _, exists := tv.mechanisms[string(transfer.Mechanism)]; !exists {
		result.Valid = false
		result.Issues = append(result.Issues, fmt.Sprintf("Unknown transfer mechanism: %s", transfer.Mechanism))
	}

	return result, nil
}

// TransferValidationResult represents transfer validation results
type TransferValidationResult struct {
	TransferID string    `json:"transfer_id"`
	Valid      bool      `json:"valid"`
	Issues     []string  `json:"issues"`
	Timestamp  time.Time `json:"timestamp"`
}

// Helper functions for default configurations

func getDefaultDataCategories() []PersonalDataType {
	return []PersonalDataType{
		IdentifyingData,
		SensitiveData,
		BiometricData,
		HealthData,
		FinancialData,
		LocationData,
		BehavioralData,
		CommunicationData,
	}
}

func getDefaultProcessingPurposes() []ProcessingPurpose {
	return []ProcessingPurpose{
		{
			ID:          "marketing",
			Name:        "Marketing Communications",
			Description: "Sending marketing emails and promotional materials",
			LegalBasis:  "consent",
			Categories:  []PersonalDataType{IdentifyingData, BehavioralData},
			Retention:   2 * 365 * 24 * time.Hour, // 2 years
			Automated:   true,
			Profiling:   true,
		},
		{
			ID:          "service_delivery",
			Name:        "Service Delivery",
			Description: "Providing core services to customers",
			LegalBasis:  "contract",
			Categories:  []PersonalDataType{IdentifyingData, FinancialData},
			Retention:   7 * 365 * 24 * time.Hour, // 7 years
			Automated:   false,
			Profiling:   false,
		},
	}
}

func getDefaultConsentPolicies() map[string]*ConsentPolicy {
	return map[string]*ConsentPolicy{
		"marketing": {
			ID:       "marketing_policy",
			Name:     "Marketing Consent Policy",
			Version:  "1.0",
			Purposes: []string{"marketing", "analytics"},
			Required: false,
			Granular: true,
		},
		"essential": {
			ID:       "essential_policy",
			Name:     "Essential Services Policy",
			Version:  "1.0",
			Purposes: []string{"service_delivery", "security"},
			Required: true,
			Granular: false,
		},
	}
}

func getDefaultTransferMechanisms() map[string]*TransferMechanismConfig {
	return map[string]*TransferMechanismConfig{
		"adequacy_decision": {
			ID:           "adequacy",
			Name:         "Adequacy Decision",
			Type:         AdequacyDecision,
			Requirements: []string{"Valid adequacy decision"},
			Validity:     0, // Indefinite
			Automated:    true,
		},
		"standard_clauses": {
			ID:           "scc",
			Name:         "Standard Contractual Clauses",
			Type:         StandardClauses,
			Requirements: []string{"Signed SCCs", "Risk assessment"},
			Validity:     365 * 24 * time.Hour, // 1 year
			Automated:    false,
		},
	}
}

func getDefaultCountryInfo() map[string]*CountryInfo {
	return map[string]*CountryInfo{
		"US": {
			Code:             "US",
			Name:             "United States",
			AdequacyDecision: false,
			Restrictions:     []string{"Privacy Shield invalidated"},
			Safeguards:       []string{"Standard Contractual Clauses"},
		},
		"CA": {
			Code:             "CA",
			Name:             "Canada",
			AdequacyDecision: true,
			DecisionDate:     &time.Time{},
			Restrictions:     []string{},
			Safeguards:       []string{},
		},
	}
}

func getDefaultSafeguards() map[string]*SafeguardConfig {
	return map[string]*SafeguardConfig{
		"encryption": {
			ID:            "encryption",
			Name:          "Data Encryption",
			Type:          "technical",
			Requirements:  []string{"AES-256 encryption", "Key management"},
			Effectiveness: "high",
		},
		"access_controls": {
			ID:            "access_controls",
			Name:          "Access Controls",
			Type:          "organizational",
			Requirements:  []string{"Role-based access", "Regular audits"},
			Effectiveness: "medium",
		},
	}
}
