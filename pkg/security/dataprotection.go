package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// DataClassification defines data sensitivity levels
type DataClassification string

const (
	DataPublic       DataClassification = "public"
	DataInternal     DataClassification = "internal"
	DataConfidential DataClassification = "confidential"
	DataRestricted   DataClassification = "restricted"
)

// DataProtectionConfig holds configuration for data protection
type DataProtectionConfig struct {
	DefaultClassification DataClassification                   `json:"default_classification"`
	FieldClassifications  map[string]DataClassification        `json:"field_classifications"`
	EncryptionKey         string                               `json:"encryption_key"`
	RegionRestrictions    map[DataClassification][]string      `json:"region_restrictions"`
	RetentionPolicies     map[DataClassification]time.Duration `json:"retention_policies"`
	AccessControls        map[DataClassification][]string      `json:"access_controls"`
	MaskingRules          map[string]MaskingRule               `json:"masking_rules"`
}

// MaskingRule defines how to mask sensitive data
type MaskingRule struct {
	Type        string `json:"type"`        // "partial", "full", "hash", "tokenize"
	Pattern     string `json:"pattern"`     // regex pattern for partial masking
	Replacement string `json:"replacement"` // replacement character/string
}

// DataProtectionManager handles data classification and protection
type DataProtectionManager struct {
	config    DataProtectionConfig
	encryptor *AESEncryptor
	tokenizer *DataTokenizer
	mu        sync.RWMutex
}

// DataContext represents data with its classification and metadata
type DataContext struct {
	Data           any                   `json:"data"`
	Classification DataClassification            `json:"classification"`
	Fields         map[string]DataClassification `json:"fields"`
	Metadata       map[string]any        `json:"metadata"`
	Timestamp      time.Time                     `json:"timestamp"`
	UserID         string                        `json:"user_id"`
	TenantID       string                        `json:"tenant_id"`
	Region         string                        `json:"region"`
	Purpose        string                        `json:"purpose"`
}

// DataProtectionRequest represents a request to access protected data
type DataProtectionRequest struct {
	UserID         string                 `json:"user_id"`
	TenantID       string                 `json:"tenant_id"`
	DataType       string                 `json:"data_type"`
	Classification DataClassification     `json:"classification"`
	Purpose        string                 `json:"purpose"`
	Region         string                 `json:"region"`
	Fields         []string               `json:"fields"`
	Metadata       map[string]any `json:"metadata"`
}

// DataAccessResult represents the result of a data access request
type DataAccessResult struct {
	Allowed       bool                   `json:"allowed"`
	Data          any            `json:"data,omitempty"`
	MaskedData    any            `json:"masked_data,omitempty"`
	Restrictions  []string               `json:"restrictions,omitempty"`
	Violations    []string               `json:"violations,omitempty"`
	AuditRequired bool                   `json:"audit_required"`
	ExpiresAt     time.Time              `json:"expires_at,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// AESEncryptor handles AES encryption/decryption
type AESEncryptor struct {
	key []byte
}

// DataTokenizer handles data tokenization for PCI compliance
type DataTokenizer struct {
	tokens map[string]string
	data   map[string]string
	mu     sync.RWMutex
}

// NewDataProtectionManager creates a new data protection manager
func NewDataProtectionManager(config DataProtectionConfig) (*DataProtectionManager, error) {
	// Initialize encryptor
	encryptor, err := NewAESEncryptor(config.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize encryptor: %w", err)
	}

	// Initialize tokenizer
	tokenizer := NewDataTokenizer()

	return &DataProtectionManager{
		config:    config,
		encryptor: encryptor,
		tokenizer: tokenizer,
	}, nil
}

// ClassifyData classifies data based on content and configuration
func (dpm *DataProtectionManager) ClassifyData(data any, context map[string]any) *DataContext {
	dpm.mu.RLock()
	defer dpm.mu.RUnlock()

	dataCtx := &DataContext{
		Data:           data,
		Classification: dpm.config.DefaultClassification,
		Fields:         make(map[string]DataClassification),
		Metadata:       context,
		Timestamp:      time.Now(),
	}

	// Extract user and tenant info from context
	if userID, ok := context["user_id"].(string); ok {
		dataCtx.UserID = userID
	}
	if tenantID, ok := context["tenant_id"].(string); ok {
		dataCtx.TenantID = tenantID
	}
	if region, ok := context["region"].(string); ok {
		dataCtx.Region = region
	}
	if purpose, ok := context["purpose"].(string); ok {
		dataCtx.Purpose = purpose
	}

	// Classify based on data content
	if jsonData, err := json.Marshal(data); err == nil {
		var dataMap map[string]any
		if json.Unmarshal(jsonData, &dataMap) == nil {
			dataCtx.Classification = dpm.classifyDataMap(dataMap, dataCtx.Fields)
		}
	}

	return dataCtx
}

// classifyDataMap classifies a data map and its fields
func (dpm *DataProtectionManager) classifyDataMap(data map[string]any, fieldClassifications map[string]DataClassification) DataClassification {
	highestClassification := DataPublic

	for field, value := range data {
		fieldClass := dpm.classifyField(field, value)
		fieldClassifications[field] = fieldClass

		// Update highest classification
		if dpm.isHigherClassification(fieldClass, highestClassification) {
			highestClassification = fieldClass
		}
	}

	return highestClassification
}

// classifyField classifies a single field
func (dpm *DataProtectionManager) classifyField(field string, value any) DataClassification {
	// Check explicit field classifications
	if classification, exists := dpm.config.FieldClassifications[field]; exists {
		return classification
	}

	// Check field name patterns for common sensitive data
	fieldLower := strings.ToLower(field)

	// Sensitive number fields
	sensitiveNumberFields := map[string]bool{
		"account_number":                true,
		"business_tin_ssn_number":       true,
		"card_num":                      true,
		"card_number":                   true,
		"cardnumber":                    true,
		"dda_number":                    true,
		"ein":                           true,
		"employer_identification_number": true,
		"merchant_tax_id":               true,
		"number":                        true,
		"owner_tin_ssn_number":          true,
		"social_security":               true,
		"social_security_number":        true,
		"ssn":                           true,
		"tax_id":                        true,
		"tax_identification_number":     true,
		"taxid":                         true,
		"tin":                           true,
	}
	
	if sensitiveNumberFields[fieldLower] {
		return DataRestricted
	}

	// High sensitivity fields from sanitization logic
	highSensitiveFields := []string{
		"password", "token", "secret", "key", "auth", "credential",
		"email", "phone", "ssn", "card", "account", "routing",
		"pin", "cvv", "security", "private", "confidential",
	}

	for _, sensitive := range highSensitiveFields {
		if strings.Contains(fieldLower, sensitive) {
			// Determine classification based on the type of sensitive field
			switch sensitive {
			case "ssn", "card", "account", "routing", "cvv", "pin":
				return DataRestricted
			case "password", "token", "secret", "key", "auth", "credential", "private":
				return DataConfidential
			case "email", "phone":
				return DataInternal
			default:
				return DataConfidential
			}
		}
	}

	// User content fields from sanitization logic - these are INTERNAL
	userContentFields := []string{
		"body", "request_body", "response_body", "user_input",
		"query", "search", "message", "comment", "description",
	}

	for _, userField := range userContentFields {
		if strings.Contains(fieldLower, userField) {
			return DataInternal
		}
	}

	// Additional restricted data patterns
	restrictedPatterns := []string{
		"tax_id", "passport", "driver_license", "bank_account",
		"medical_record", "health_record", "diagnosis", "prescription",
	}

	for _, pattern := range restrictedPatterns {
		if strings.Contains(fieldLower, pattern) {
			return DataRestricted
		}
	}

	// Additional confidential data patterns
	confidentialPatterns := []string{
		"salary", "income", "financial", "revenue", "profit",
	}

	for _, pattern := range confidentialPatterns {
		if strings.Contains(fieldLower, pattern) {
			return DataConfidential
		}
	}

	// Additional internal data patterns
	internalPatterns := []string{
		"address", "name", "birth", "employee", "organization",
	}

	for _, pattern := range internalPatterns {
		if strings.Contains(fieldLower, pattern) {
			return DataInternal
		}
	}

	// Check value patterns (e.g., credit card numbers, SSNs)
	if strValue, ok := value.(string); ok {
		if dpm.isRestrictedValue(strValue) {
			return DataRestricted
		}
	}

	return DataPublic
}

// isRestrictedValue checks if a value matches restricted data patterns
func (dpm *DataProtectionManager) isRestrictedValue(value string) bool {
	// Remove spaces and dashes for pattern matching
	cleaned := strings.ReplaceAll(strings.ReplaceAll(value, " ", ""), "-", "")

	// Credit card pattern (13-19 digits)
	if len(cleaned) >= 13 && len(cleaned) <= 19 {
		allDigits := true
		for _, char := range cleaned {
			if char < '0' || char > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			return true
		}
	}

	// SSN pattern (9 digits)
	if len(cleaned) == 9 {
		allDigits := true
		for _, char := range cleaned {
			if char < '0' || char > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			return true
		}
	}

	return false
}

// isHigherClassification checks if one classification is higher than another
func (dpm *DataProtectionManager) isHigherClassification(a, b DataClassification) bool {
	levels := map[DataClassification]int{
		DataPublic:       0,
		DataInternal:     1,
		DataConfidential: 2,
		DataRestricted:   3,
	}

	return levels[a] > levels[b]
}

// ValidateDataAccess validates if data access is allowed
func (dpm *DataProtectionManager) ValidateDataAccess(request DataProtectionRequest) *DataAccessResult {
	dpm.mu.RLock()
	defer dpm.mu.RUnlock()

	result := &DataAccessResult{
		Allowed:       true,
		AuditRequired: true,
		Metadata:      make(map[string]any),
	}

	// Check region restrictions
	if restrictions, exists := dpm.config.RegionRestrictions[request.Classification]; exists {
		allowed := false
		for _, allowedRegion := range restrictions {
			if allowedRegion == request.Region || allowedRegion == "*" {
				allowed = true
				break
			}
		}
		if !allowed {
			result.Allowed = false
			result.Violations = append(result.Violations, fmt.Sprintf("Data access not allowed in region: %s", request.Region))
			return result // Return early if region check fails
		}
	}

	// Check access controls
	if controls, exists := dpm.config.AccessControls[request.Classification]; exists {
		// This would typically check user roles/permissions
		// For now, we'll assume access is allowed if user is specified
		if request.UserID == "" {
			result.Allowed = false
			result.Violations = append(result.Violations, "User authentication required for this data classification")
			return result // Return early if user check fails
		}

		result.Restrictions = controls
	}

	// Set expiration based on classification
	if retention, exists := dpm.config.RetentionPolicies[request.Classification]; exists {
		result.ExpiresAt = time.Now().Add(retention)
	}

	// Require audit for sensitive data
	if request.Classification == DataConfidential || request.Classification == DataRestricted {
		result.AuditRequired = true
	}

	return result
}

// ValidateDataAccessFromGDPR validates data access from a GDPR DataAccessRequest
func (dpm *DataProtectionManager) ValidateDataAccessFromGDPR(request any) *DataAccessResult {
	// Convert DataAccessRequest to DataProtectionRequest
	var protectionRequest DataProtectionRequest

	// Use type assertion to handle the interface
	if dar, ok := request.(DataAccessRequest); ok {
		protectionRequest = DataProtectionRequest{
			UserID:         dar.UserID,
			TenantID:       "", // Not available in DataAccessRequest
			DataType:       dar.RequestType,
			Classification: DataClassification(dar.Purpose), // Map purpose to classification
			Purpose:        dar.Purpose,
			Region:         dar.Region,
			Fields:         dar.Scope,
			Metadata:       dar.Metadata,
		}

		// Try to infer classification from purpose or other fields
		switch dar.Purpose {
		case "restricted", "processing":
			protectionRequest.Classification = DataRestricted
		case "confidential":
			protectionRequest.Classification = DataConfidential
		case "internal":
			protectionRequest.Classification = DataInternal
		default:
			protectionRequest.Classification = DataPublic
		}
	} else {
		// If it's already a DataProtectionRequest, use it directly
		if dpr, ok := request.(DataProtectionRequest); ok {
			protectionRequest = dpr
		} else {
			// Return error result for unsupported type
			return &DataAccessResult{
				Allowed:    false,
				Violations: []string{"Unsupported request type"},
				Metadata:   make(map[string]any),
			}
		}
	}

	return dpm.ValidateDataAccess(protectionRequest)
}

// ProtectData applies protection measures to data based on classification
func (dpm *DataProtectionManager) ProtectData(dataCtx *DataContext, accessRequest DataProtectionRequest) (*DataAccessResult, error) {
	// Validate access first
	result := dpm.ValidateDataAccess(accessRequest)
	if !result.Allowed {
		return result, nil
	}

	// Apply data protection based on classification
	switch dataCtx.Classification {
	case DataRestricted:
		// Encrypt or tokenize restricted data
		if accessRequest.Purpose == "display" {
			maskedData, err := dpm.maskData(dataCtx.Data, dataCtx.Fields)
			if err != nil {
				return nil, fmt.Errorf("failed to mask data: %w", err)
			}
			result.MaskedData = maskedData
		} else {
			// For processing purposes, provide encrypted data
			encryptedData, err := dpm.encryptor.Encrypt(dataCtx.Data)
			if err != nil {
				return nil, fmt.Errorf("failed to encrypt data: %w", err)
			}
			result.Data = encryptedData
		}

	case DataConfidential:
		// Apply field-level masking for confidential data
		maskedData, err := dpm.maskData(dataCtx.Data, dataCtx.Fields)
		if err != nil {
			return nil, fmt.Errorf("failed to mask data: %w", err)
		}
		result.MaskedData = maskedData

	case DataInternal:
		// Apply minimal masking for internal data
		if accessRequest.Purpose == "external" {
			maskedData, err := dpm.maskData(dataCtx.Data, dataCtx.Fields)
			if err != nil {
				return nil, fmt.Errorf("failed to mask data: %w", err)
			}
			result.MaskedData = maskedData
		} else {
			result.Data = dataCtx.Data
		}

	case DataPublic:
		// No protection needed for public data
		result.Data = dataCtx.Data
	}

	return result, nil
}

// maskData applies masking rules to data
func (dpm *DataProtectionManager) maskData(data any, fieldClassifications map[string]DataClassification) (any, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var dataMap map[string]any
	if err := json.Unmarshal(jsonData, &dataMap); err != nil {
		return nil, err
	}

	maskedMap := make(map[string]any)

	for field, value := range dataMap {
		classification := fieldClassifications[field]

		// Apply masking based on classification and rules
		if rule, exists := dpm.config.MaskingRules[field]; exists {
			maskedValue, err := dpm.applyMaskingRule(value, rule)
			if err != nil {
				return nil, err
			}
			maskedMap[field] = maskedValue
		} else {
			// Apply default masking based on classification
			maskedMap[field] = dpm.applyDefaultMasking(value, classification)
		}
	}

	return maskedMap, nil
}

// applyMaskingRule applies a specific masking rule to a value
func (dpm *DataProtectionManager) applyMaskingRule(value any, rule MaskingRule) (any, error) {
	strValue, ok := value.(string)
	if !ok {
		return value, nil // Don't mask non-string values
	}

	switch rule.Type {
	case "full":
		return strings.Repeat(rule.Replacement, len(strValue)), nil

	case "partial":
		if len(strValue) <= 4 {
			return strings.Repeat(rule.Replacement, len(strValue)), nil
		}
		// Show first 2 and last 2 characters
		return strValue[:2] + strings.Repeat(rule.Replacement, len(strValue)-4) + strValue[len(strValue)-2:], nil

	case "hash":
		hash := sha256.Sum256([]byte(strValue))
		return base64.StdEncoding.EncodeToString(hash[:]), nil

	case "tokenize":
		token, err := dpm.tokenizer.Tokenize(strValue)
		if err != nil {
			return nil, err
		}
		return token, nil

	default:
		return value, nil
	}
}

// applyDefaultMasking applies default masking based on classification
func (dpm *DataProtectionManager) applyDefaultMasking(value any, classification DataClassification) any {
	strValue, ok := value.(string)
	if !ok {
		return value
	}

	switch classification {
	case DataRestricted:
		// Full masking for restricted data
		return strings.Repeat("*", len(strValue))

	case DataConfidential:
		// Partial masking for confidential data
		if len(strValue) <= 4 {
			return strings.Repeat("*", len(strValue))
		}
		return strValue[:2] + strings.Repeat("*", len(strValue)-4) + strValue[len(strValue)-2:]

	default:
		// Internal and Public data are not masked by default
		return value
	}
}

// NewAESEncryptor creates a new AES encryptor
func NewAESEncryptor(keyString string) (*AESEncryptor, error) {
	// Generate key from string
	hash := sha256.Sum256([]byte(keyString))

	return &AESEncryptor{
		key: hash[:],
	}, nil
}

// Encrypt encrypts data using AES
func (e *AESEncryptor) Encrypt(data any) (string, error) {
	// Convert data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	// Create cipher
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", err
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Encrypt
	ciphertext := gcm.Seal(nonce, nonce, jsonData, nil)

	// Encode to base64
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts data using AES
func (e *AESEncryptor) Decrypt(encryptedData string, result any) error {
	// Decode from base64
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return err
	}

	// Create cipher
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return err
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	// Extract nonce
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return err
	}

	// Unmarshal JSON
	return json.Unmarshal(plaintext, result)
}

// NewDataTokenizer creates a new data tokenizer
func NewDataTokenizer() *DataTokenizer {
	return &DataTokenizer{
		tokens: make(map[string]string),
		data:   make(map[string]string),
	}
}

// Tokenize creates a token for sensitive data
func (dt *DataTokenizer) Tokenize(data string) (string, error) {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	// Check if already tokenized
	if token, exists := dt.tokens[data]; exists {
		return token, nil
	}

	// Generate random token
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}

	token := base64.URLEncoding.EncodeToString(tokenBytes)

	// Store mapping
	dt.tokens[data] = token
	dt.data[token] = data

	return token, nil
}

// Detokenize retrieves original data from token
func (dt *DataTokenizer) Detokenize(token string) (string, error) {
	dt.mu.RLock()
	defer dt.mu.RUnlock()

	data, exists := dt.data[token]
	if !exists {
		return "", fmt.Errorf("token not found")
	}

	return data, nil
}

// DataProtection creates middleware for data protection
func DataProtection(config DataProtectionConfig) LiftMiddleware {
	manager, err := NewDataProtectionManager(config)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize data protection: %v", err))
	}

	return func(next LiftHandler) LiftHandler {
		return LiftHandlerFunc(func(ctx LiftContext) error {
			// Store data protection manager in context
			ctx.Set("data_protection", manager)

			// Execute handler
			return next.Handle(ctx)
		})
	}
}

// GetDataProtectionManager retrieves the data protection manager from context
func GetDataProtectionManager(ctx LiftContext) (*DataProtectionManager, error) {
	manager, ok := ctx.Get("data_protection").(*DataProtectionManager)
	if !ok {
		return nil, fmt.Errorf("data protection manager not found in context")
	}
	return manager, nil
}
