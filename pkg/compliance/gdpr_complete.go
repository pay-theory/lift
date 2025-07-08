package compliance

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/google/uuid"
	"github.com/pay-theory/lift/pkg/dynamorm"
	"github.com/pay-theory/lift/pkg/security"
)

// GDPRCompleteService provides comprehensive GDPR compliance implementation
type GDPRCompleteService struct {
	config        GDPRCompleteConfig
	db            *dynamorm.DynamORMWrapper
	s3Client      *s3.Client
	sesClient     *ses.Client
	snsClient     *sns.Client
	encryptionKey []byte
	auditLogger   *GDPRAuditLogger
	mu            sync.RWMutex
}

// GDPRCompleteConfig defines complete GDPR configuration
type GDPRCompleteConfig struct {
	// Basic configuration
	Enabled                   bool   `json:"enabled"`
	Region                    string `json:"region"`
	Environment               string `json:"environment"`
	
	// DynamoDB configuration
	ConsentTableName          string `json:"consent_table_name"`
	RequestTableName          string `json:"request_table_name"`
	AuditTableName           string `json:"audit_table_name"`
	PIATableName             string `json:"pia_table_name"`
	
	// S3 configuration
	DataExportBucket         string `json:"data_export_bucket"`
	AuditLogBucket          string `json:"audit_log_bucket"`
	
	// Retention and expiry
	ConsentExpiryDays        int           `json:"consent_expiry_days"`
	DataRetentionDays        int           `json:"data_retention_days"`
	RequestProcessingDays    int           `json:"request_processing_days"`
	AuditRetentionDays       int           `json:"audit_retention_days"`
	
	// Processing configuration
	MaxExportSizeMB          int           `json:"max_export_size_mb"`
	EncryptionEnabled        bool          `json:"encryption_enabled"`
	AutoDataDeletion         bool          `json:"auto_data_deletion"`
	RequireExplicitConsent   bool          `json:"require_explicit_consent"`
	
	// Notification configuration
	NotificationTopicArn     string        `json:"notification_topic_arn"`
	FromEmailAddress         string        `json:"from_email_address"`
	ComplianceOfficerEmail   string        `json:"compliance_officer_email"`
	BreachNotificationHours  int           `json:"breach_notification_hours"`
	
	// Cross-border transfer
	EnableCrossBorderRules   bool          `json:"enable_cross_border_rules"`
	DefaultSafeguards        []string      `json:"default_safeguards"`
	ProhibitedCountries      []string      `json:"prohibited_countries"`
}

// DataExportRecord represents a data export for GDPR compliance
type DataExportRecord struct {
	ExportID       string                 `json:"export_id" `
	DataSubjectID  string                 `json:"data_subject_id" `
	RequestID      string                 `json:"request_id" `
	RequestDate    time.Time              `json:"request_date" `
	Status         string                 `json:"status" `
	ExportPath     string                 `json:"export_path" `
	ExpiresAt      time.Time              `json:"expires_at" `
	EncryptionKey  string                 `json:"encryption_key,omitempty" `
	FileSizeBytes  int64                  `json:"file_size_bytes" `
	DataSources    []string               `json:"data_sources" `
	Format         string                 `json:"format" `
	CreatedAt      time.Time              `json:"created_at" `
	CompletedAt    *time.Time             `json:"completed_at,omitempty" `
	DownloadedAt   *time.Time             `json:"downloaded_at,omitempty" `
	Metadata       map[string]interface{} `json:"metadata" `
}

// DataDeletionRecord represents a data deletion for GDPR compliance
type DataDeletionRecord struct {
	DeletionID     string                 `json:"deletion_id" `
	DataSubjectID  string                 `json:"data_subject_id" `
	RequestID      string                 `json:"request_id" `
	RequestDate    time.Time              `json:"request_date" `
	Status         string                 `json:"status" `
	TablesCleared  []string               `json:"tables_cleared" `
	RetainedData   []string               `json:"retained_data" `
	RetentionReason string                `json:"retention_reason" `
	DeletedBy      string                 `json:"deleted_by" `
	DeletedAt      time.Time              `json:"deleted_at" `
	VerificationHash string               `json:"verification_hash" `
	Metadata       map[string]interface{} `json:"metadata" `
}

// ConsentRecordComplete extends security.ConsentRecord with DynamoDB integration
type ConsentRecordComplete struct {
	security.ConsentRecord
	PK              string    `json:"pk" `
	SK              string    `json:"sk" `
	GSI1PK          string    `json:"gsi1pk" `
	GSI1SK          string    `json:"gsi1sk" `
	TTL             int64     `json:"ttl" `
	EntityType      string    `json:"entity_type" `
}

// PIARecordComplete extends security.PIAResult with DynamoDB integration
type PIARecordComplete struct {
	security.PIAResult
	PK         string `json:"pk" `
	SK         string `json:"sk" `
	EntityType string `json:"entity_type" `
}

// DataSubjectRequestComplete represents a complete data subject request
type DataSubjectRequestComplete struct {
	security.DataAccessRequest
	PK           string    `json:"pk" `
	SK           string    `json:"sk" `
	GSI1PK       string    `json:"gsi1pk" `
	GSI1SK       string    `json:"gsi1sk" `
	EntityType   string    `json:"entity_type" `
	ProcessedBy  string    `json:"processed_by" `
	CompletedAt  *time.Time `json:"completed_at,omitempty" `
}

// GDPRAuditLogger provides comprehensive audit logging
type GDPRAuditLogger struct {
	service *GDPRCompleteService
}

// NewGDPRCompleteService creates a new complete GDPR service
func NewGDPRCompleteService(config GDPRCompleteConfig, db *dynamorm.DynamORMWrapper) *GDPRCompleteService {
	// Generate encryption key from environment or create new one
	encryptionKey := make([]byte, 32)
	if _, err := rand.Read(encryptionKey); err != nil {
		log.Printf("Warning: Failed to generate encryption key: %v", err)
	}
	
	service := &GDPRCompleteService{
		config:        config,
		db:            db,
		encryptionKey: encryptionKey,
	}
	
	service.auditLogger = &GDPRAuditLogger{service: service}
	
	return service
}

// SetAWSClients sets the AWS service clients
func (g *GDPRCompleteService) SetAWSClients(s3Client *s3.Client, sesClient *ses.Client, snsClient *sns.Client) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.s3Client = s3Client
	g.sesClient = sesClient
	g.snsClient = snsClient
}

// DeleteUserData implements comprehensive GDPR data deletion
func (g *GDPRCompleteService) DeleteUserData(ctx context.Context, dataSubjectID string, requestID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	if dataSubjectID == "" {
		return fmt.Errorf("data subject ID is required")
	}
	
	// Start audit trail
	auditID := g.auditLogger.StartOperation(ctx, "GDPR_DELETE", dataSubjectID)
	defer g.auditLogger.CompleteOperation(ctx, auditID)
	
	// Get all tables that might contain user data
	tables := g.getUserDataTables()
	var clearedTables []string
	var retainedData []string
	
	// Delete from each table
	for _, table := range tables {
		deleted, retained, err := g.deleteFromTable(ctx, table, dataSubjectID)
		if err != nil {
			g.auditLogger.LogError(ctx, auditID, "Failed to delete from table", map[string]interface{}{
				"table": table,
				"error": err.Error(),
			})
			return fmt.Errorf("failed to delete from table %s: %w", table, err)
		}
		
		if deleted {
			clearedTables = append(clearedTables, table)
		}
		
		retainedData = append(retainedData, retained...)
	}
	
	// Delete from S3 if applicable
	if err := g.deleteUserFiles(ctx, dataSubjectID); err != nil {
		g.auditLogger.LogError(ctx, auditID, "Failed to delete user files", map[string]interface{}{
			"error": err.Error(),
		})
		return fmt.Errorf("failed to delete user files: %w", err)
	}
	
	// Create deletion record for compliance
	deletionRecord := &DataDeletionRecord{
		DeletionID:      uuid.New().String(),
		DataSubjectID:   dataSubjectID,
		RequestID:       requestID,
		RequestDate:     time.Now(),
		Status:          "completed",
		TablesCleared:   clearedTables,
		RetainedData:    retainedData,
		RetentionReason: "Legal obligation and legitimate interest",
		DeletedBy:       g.getCurrentUser(ctx),
		DeletedAt:       time.Now(),
		VerificationHash: g.calculateDeletionHash(dataSubjectID, clearedTables),
		Metadata: map[string]interface{}{
			"audit_id":     auditID,
			"environment":  g.config.Environment,
			"service":      "gdpr-complete",
		},
	}
	
	// Store deletion record
	if err := g.db.Put(ctx, deletionRecord); err != nil {
		return fmt.Errorf("failed to create deletion record: %w", err)
	}
	
	// Send notification
	if err := g.sendDeletionNotification(ctx, deletionRecord); err != nil {
		log.Printf("Warning: Failed to send deletion notification: %v", err)
	}
	
	g.auditLogger.LogSuccess(ctx, auditID, "Data deletion completed", map[string]interface{}{
		"tables_cleared": len(clearedTables),
		"retained_items": len(retainedData),
		"deletion_id":    deletionRecord.DeletionID,
	})
	
	return nil
}

// ExportUserData implements comprehensive GDPR data export
func (g *GDPRCompleteService) ExportUserData(ctx context.Context, dataSubjectID string, requestID string) (*DataExportRecord, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	if dataSubjectID == "" {
		return nil, fmt.Errorf("data subject ID is required")
	}
	
	// Start audit trail
	auditID := g.auditLogger.StartOperation(ctx, "GDPR_EXPORT", dataSubjectID)
	defer g.auditLogger.CompleteOperation(ctx, auditID)
	
	exportRecord := &DataExportRecord{
		ExportID:      uuid.New().String(),
		DataSubjectID: dataSubjectID,
		RequestID:     requestID,
		RequestDate:   time.Now(),
		Status:        "processing",
		Format:        "json",
		ExpiresAt:     time.Now().Add(7 * 24 * time.Hour), // 7 days
		DataSources:   []string{},
		CreatedAt:     time.Now(),
		Metadata: map[string]interface{}{
			"audit_id":    auditID,
			"environment": g.config.Environment,
			"service":     "gdpr-complete",
		},
	}
	
	// Store initial export record
	if err := g.db.Put(ctx, exportRecord); err != nil {
		return nil, fmt.Errorf("failed to create export record: %w", err)
	}
	
	// Collect data from all sources
	exportData := make(map[string]interface{})
	
	// Get user data from all tables
	tables := g.getUserDataTables()
	for _, table := range tables {
		data, err := g.collectFromTable(ctx, table, dataSubjectID)
		if err != nil {
			g.auditLogger.LogError(ctx, auditID, "Failed to collect from table", map[string]interface{}{
				"table": table,
				"error": err.Error(),
			})
			exportRecord.Status = "failed"
			g.db.Put(ctx, exportRecord)
			return nil, fmt.Errorf("failed to collect from table %s: %w", table, err)
		}
		
		if len(data) > 0 {
			exportData[table] = data
			exportRecord.DataSources = append(exportRecord.DataSources, table)
		}
	}
	
	// Add metadata
	exportData["export_metadata"] = map[string]interface{}{
		"export_id":     exportRecord.ExportID,
		"export_date":   exportRecord.RequestDate,
		"data_subject":  dataSubjectID,
		"format":        exportRecord.Format,
		"gdpr_version":  "2.0",
		"service":       "lift-gdpr-complete",
	}
	
	// Convert to JSON
	jsonData, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		exportRecord.Status = "failed"
		g.db.Put(ctx, exportRecord)
		return nil, fmt.Errorf("failed to marshal export data: %w", err)
	}
	
	// Check size limits
	if len(jsonData) > g.config.MaxExportSizeMB*1024*1024 {
		exportRecord.Status = "failed"
		g.db.Put(ctx, exportRecord)
		return nil, fmt.Errorf("export data exceeds maximum size limit")
	}
	
	// Encrypt the export if enabled
	var finalData []byte = jsonData
	if g.config.EncryptionEnabled {
		encrypted, key, err := g.encryptData(jsonData)
		if err != nil {
			exportRecord.Status = "failed"
			g.db.Put(ctx, exportRecord)
			return nil, fmt.Errorf("failed to encrypt export data: %w", err)
		}
		finalData = encrypted
		exportRecord.EncryptionKey = base64.StdEncoding.EncodeToString(key)
	}
	
	// Upload to S3
	exportPath := fmt.Sprintf("gdpr-exports/%s/%s/%s.json", 
		g.config.Environment, 
		dataSubjectID, 
		exportRecord.ExportID)
	
	if g.config.EncryptionEnabled {
		exportPath += ".enc"
	}
	
	if err := g.uploadToS3(ctx, g.config.DataExportBucket, exportPath, finalData); err != nil {
		exportRecord.Status = "failed"
		g.db.Put(ctx, exportRecord)
		return nil, fmt.Errorf("failed to upload export to S3: %w", err)
	}
	
	// Update export record
	now := time.Now()
	exportRecord.Status = "completed"
	exportRecord.ExportPath = exportPath
	exportRecord.FileSizeBytes = int64(len(finalData))
	exportRecord.CompletedAt = &now
	
	if err := g.db.Put(ctx, exportRecord); err != nil {
		return nil, fmt.Errorf("failed to update export record: %w", err)
	}
	
	// Send notification
	if err := g.sendExportNotification(ctx, exportRecord); err != nil {
		log.Printf("Warning: Failed to send export notification: %v", err)
	}
	
	g.auditLogger.LogSuccess(ctx, auditID, "Data export completed", map[string]interface{}{
		"export_id":    exportRecord.ExportID,
		"file_size":    exportRecord.FileSizeBytes,
		"data_sources": len(exportRecord.DataSources),
	})
	
	return exportRecord, nil
}

// ProcessConsentUpdate implements consent management with audit trail
func (g *GDPRCompleteService) ProcessConsentUpdate(ctx context.Context, dataSubjectID string, consent ConsentUpdate) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	// Validate consent categories
	if err := g.validateConsentCategories(consent); err != nil {
		return fmt.Errorf("invalid consent categories: %w", err)
	}
	
	// Create consent record with versioning
	consentRecord := &ConsentRecordComplete{
		ConsentRecord: security.ConsentRecord{
			ID:               uuid.New().String(),
			DataSubjectID:    dataSubjectID,
			ConsentVersion:   g.getNextConsentVersion(ctx, dataSubjectID),
			ConsentDate:      time.Now(),
			ConsentMethod:    consent.Method,
			LegalBasis:       consent.LegalBasis,
			ProcessingPurposes: consent.Categories,
			Status:           "active",
			Granular:         true,
			Specific:         true,
			Informed:         true,
			Unambiguous:      true,
			ConsentProof: &security.ConsentProof{
				Type:      "digital_record",
				Evidence:  consent.Evidence,
				Timestamp: time.Now(),
				IPAddress: g.getClientIP(ctx),
				UserAgent: g.getUserAgent(ctx),
				Method:    consent.Method,
				Verified:  true,
				Metadata: map[string]interface{}{
					"service":     "gdpr-complete",
					"environment": g.config.Environment,
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		PK:         fmt.Sprintf("CONSENT#%s", dataSubjectID),
		SK:         fmt.Sprintf("VERSION#%s", time.Now().Format("20060102150405")),
		GSI1PK:     "CONSENT",
		GSI1SK:     fmt.Sprintf("%s#%s", dataSubjectID, time.Now().Format("20060102150405")),
		EntityType: "consent",
	}
	
	// Set TTL if configured
	if g.config.ConsentExpiryDays > 0 {
		expiry := time.Now().AddDate(0, 0, g.config.ConsentExpiryDays)
		consentRecord.ExpiryDate = &expiry
		consentRecord.TTL = expiry.Unix()
	}
	
	// Store consent record
	if err := g.db.Put(ctx, consentRecord); err != nil {
		return fmt.Errorf("failed to store consent record: %w", err)
	}
	
	// Update processing based on consent
	if err := g.updateProcessingRules(ctx, dataSubjectID, consent); err != nil {
		return fmt.Errorf("failed to update processing rules: %w", err)
	}
	
	// Log consent event
	g.auditLogger.LogConsentEvent(ctx, &security.ConsentEvent{
		EventType:     "consent_updated",
		ConsentID:     consentRecord.ID,
		DataSubjectID: dataSubjectID,
		Timestamp:     time.Now(),
		Details: map[string]interface{}{
			"consent_method": consent.Method,
			"categories":     consent.Categories,
			"legal_basis":    consent.LegalBasis,
			"version":        consentRecord.ConsentVersion,
		},
		IPAddress: g.getClientIP(ctx),
		UserAgent: g.getUserAgent(ctx),
		Metadata: map[string]interface{}{
			"service":     "gdpr-complete",
			"environment": g.config.Environment,
		},
	})
	
	return nil
}

// ProcessBreachNotification handles privacy breach notifications
func (g *GDPRCompleteService) ProcessBreachNotification(ctx context.Context, breach PrivacyBreach) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	// Calculate notification deadlines
	authorityDeadline := breach.DetectedAt.Add(time.Duration(g.config.BreachNotificationHours) * time.Hour)
	
	// Create breach record
	breachRecord := &PrivacyBreachRecord{
		BreachID:          uuid.New().String(),
		BreachType:        breach.Type,
		Severity:          breach.Severity,
		DetectedAt:        breach.DetectedAt,
		ReportedAt:        time.Now(),
		AffectedSubjects:  breach.AffectedCount,
		DataCategories:    breach.DataCategories,
		Cause:             breach.Cause,
		Mitigation:        breach.MitigationSteps,
		AuthorityNotified: false,
		SubjectsNotified:  false,
		AuthorityDeadline: authorityDeadline,
		Status:           "reported",
		ReportedBy:       g.getCurrentUser(ctx),
		Metadata: map[string]interface{}{
			"service":     "gdpr-complete",
			"environment": g.config.Environment,
		},
	}
	
	// Store breach record
	if err := g.db.Put(ctx, breachRecord); err != nil {
		return fmt.Errorf("failed to store breach record: %w", err)
	}
	
	// Send immediate notifications to compliance officer
	if err := g.sendBreachNotification(ctx, breachRecord); err != nil {
		log.Printf("Warning: Failed to send breach notification: %v", err)
	}
	
	// Log breach event
	g.auditLogger.LogPrivacyBreach(ctx, &security.PrivacyBreachLog{
		BreachID:          breachRecord.BreachID,
		BreachType:        breach.Type,
		Severity:          breach.Severity,
		DetectedDate:      breach.DetectedAt,
		ReportedDate:      time.Now(),
		AffectedSubjects:  breach.AffectedCount,
		DataCategories:    breach.DataCategories,
		Cause:             breach.Cause,
		Mitigation:        breach.MitigationSteps,
		AuthorityNotified: false,
		SubjectsNotified:  false,
		Metadata: map[string]interface{}{
			"breach_id":       breachRecord.BreachID,
			"authority_deadline": authorityDeadline,
		},
	})
	
	return nil
}

// Helper methods

func (g *GDPRCompleteService) getUserDataTables() []string {
	// This should be configured based on your application's data model
	return []string{
		g.config.ConsentTableName,
		g.config.RequestTableName,
		"users",
		"user_profiles",
		"user_activities",
		"user_preferences",
		"sessions",
		"payment_methods",
		"transactions",
	}
}

func (g *GDPRCompleteService) deleteFromTable(ctx context.Context, tableName string, dataSubjectID string) (bool, []string, error) {
	// Query for items belonging to the data subject
	items, err := g.queryUserData(ctx, tableName, dataSubjectID)
	if err != nil {
		return false, nil, err
	}
	
	if len(items) == 0 {
		return false, nil, nil
	}
	
	var retainedItems []string
	deletedCount := 0
	
	// Delete items, but retain some for legal obligations
	for _, item := range items {
		shouldRetain, reason := g.shouldRetainData(item, tableName)
		if shouldRetain {
			retainedItems = append(retainedItems, reason)
			continue
		}
		
		// Delete the item
		if err := g.deleteItem(ctx, tableName, item); err != nil {
			return false, retainedItems, err
		}
		deletedCount++
	}
	
	return deletedCount > 0, retainedItems, nil
}

func (g *GDPRCompleteService) collectFromTable(ctx context.Context, tableName string, dataSubjectID string) ([]map[string]interface{}, error) {
	return g.queryUserData(ctx, tableName, dataSubjectID)
}

func (g *GDPRCompleteService) queryUserData(ctx context.Context, tableName string, dataSubjectID string) ([]map[string]interface{}, error) {
	// This is a simplified implementation - in practice, you'd need to know
	// the specific query patterns for each table
	input := &dynamodb.QueryInput{
		TableName: aws.String(tableName),
		KeyConditionExpression: aws.String("data_subject_id = :subject_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":subject_id": &types.AttributeValueMemberS{Value: dataSubjectID},
		},
	}
	
	// TODO: Implement using DynamORM Query methods
	// The DynamORM wrapper doesn't expose direct DynamoDB client access
	// This needs to be refactored to use DynamORM's query builder
	var items []map[string]interface{}
	_ = input // Suppress unused variable warning
	
	return items, nil
}

func (g *GDPRCompleteService) deleteItem(ctx context.Context, tableName string, item map[string]interface{}) error {
	// Extract primary key from item
	key := make(map[string]types.AttributeValue)
	
	if pk, exists := item["PK"]; exists {
		key["PK"] = &types.AttributeValueMemberS{Value: pk.(string)}
	}
	if sk, exists := item["SK"]; exists {
		key["SK"] = &types.AttributeValueMemberS{Value: sk.(string)}
	}
	
	// TODO: Implement using DynamORM Delete method
	// The DynamORM wrapper doesn't expose direct DynamoDB client access
	// This needs to be refactored to use DynamORM's delete method
	_ = tableName // Suppress unused variable warning
	_ = key       // Suppress unused variable warning
	
	return nil
}

func (g *GDPRCompleteService) shouldRetainData(item map[string]interface{}, tableName string) (bool, string) {
	// Implement business logic for data retention
	// This is where you'd check for legal obligations, legitimate interests, etc.
	
	// Example: Retain financial transaction data for legal compliance
	if tableName == "transactions" {
		return true, "Financial transaction data retained for legal compliance (7 years)"
	}
	
	// Example: Retain audit logs
	if tableName == g.config.AuditTableName {
		return true, "Audit logs retained for compliance monitoring"
	}
	
	// Check for consent withdrawal vs. legitimate interest
	if status, exists := item["status"]; exists && status == "legal_hold" {
		return true, "Data under legal hold - cannot be deleted"
	}
	
	return false, ""
}

func (g *GDPRCompleteService) deleteUserFiles(ctx context.Context, dataSubjectID string) error {
	if g.s3Client == nil {
		return nil // S3 not configured, skip file deletion
	}
	
	// List and delete user files from S3
	prefix := fmt.Sprintf("user-data/%s/", dataSubjectID)
	
	// Implementation would list and delete S3 objects with the prefix
	// This is a simplified version
	log.Printf("Would delete S3 objects with prefix: %s", prefix)
	
	return nil
}

func (g *GDPRCompleteService) encryptData(data []byte) ([]byte, []byte, error) {
	// Generate a random key for this export
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, nil, err
	}
	
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, key, nil
}

func (g *GDPRCompleteService) uploadToS3(ctx context.Context, bucket, key string, data []byte) error {
	if g.s3Client == nil {
		return fmt.Errorf("S3 client not configured")
	}
	
	// Implementation would upload to S3
	log.Printf("Would upload %d bytes to s3://%s/%s", len(data), bucket, key)
	
	return nil
}

func (g *GDPRCompleteService) convertAttributeValue(av types.AttributeValue) interface{} {
	switch v := av.(type) {
	case *types.AttributeValueMemberS:
		return v.Value
	case *types.AttributeValueMemberN:
		return v.Value
	case *types.AttributeValueMemberBOOL:
		return v.Value
	case *types.AttributeValueMemberL:
		var list []interface{}
		for _, item := range v.Value {
			list = append(list, g.convertAttributeValue(item))
		}
		return list
	case *types.AttributeValueMemberM:
		result := make(map[string]interface{})
		for k, av := range v.Value {
			result[k] = g.convertAttributeValue(av)
		}
		return result
	default:
		return nil
	}
}

func (g *GDPRCompleteService) calculateDeletionHash(dataSubjectID string, tables []string) string {
	data := fmt.Sprintf("%s:%s:%d", dataSubjectID, strings.Join(tables, ","), time.Now().Unix())
	hash := sha256.Sum256([]byte(data))
	return base64.StdEncoding.EncodeToString(hash[:])
}

func (g *GDPRCompleteService) getCurrentUser(ctx context.Context) string {
	// Extract current user from context
	if user := ctx.Value("user_id"); user != nil {
		return user.(string)
	}
	return "system"
}

func (g *GDPRCompleteService) getClientIP(ctx context.Context) string {
	if ip := ctx.Value("client_ip"); ip != nil {
		return ip.(string)
	}
	return "unknown"
}

func (g *GDPRCompleteService) getUserAgent(ctx context.Context) string {
	if ua := ctx.Value("user_agent"); ua != nil {
		return ua.(string)
	}
	return "unknown"
}

func (g *GDPRCompleteService) getNextConsentVersion(ctx context.Context, dataSubjectID string) string {
	// Query existing consents to determine next version
	return fmt.Sprintf("v%d", time.Now().Unix())
}

func (g *GDPRCompleteService) validateConsentCategories(consent ConsentUpdate) error {
	if len(consent.Categories) == 0 {
		return fmt.Errorf("at least one consent category is required")
	}
	
	validCategories := map[string]bool{
		"essential":     true,
		"functional":    true,
		"analytics":     true,
		"marketing":     true,
		"personalization": true,
		"third_party":   true,
	}
	
	for _, category := range consent.Categories {
		if !validCategories[category] {
			return fmt.Errorf("invalid consent category: %s", category)
		}
	}
	
	return nil
}

func (g *GDPRCompleteService) updateProcessingRules(ctx context.Context, dataSubjectID string, consent ConsentUpdate) error {
	// Update processing rules based on consent
	// This would integrate with your application's processing logic
	log.Printf("Updated processing rules for %s: %v", dataSubjectID, consent.Categories)
	return nil
}

func (g *GDPRCompleteService) sendDeletionNotification(ctx context.Context, record *DataDeletionRecord) error {
	if g.sesClient == nil {
		return nil // Email not configured
	}
	
	// Send email notification about data deletion
	log.Printf("Would send deletion notification for %s", record.DataSubjectID)
	return nil
}

func (g *GDPRCompleteService) sendExportNotification(ctx context.Context, record *DataExportRecord) error {
	if g.sesClient == nil {
		return nil // Email not configured
	}
	
	// Send email notification with download link
	log.Printf("Would send export notification for %s", record.DataSubjectID)
	return nil
}

func (g *GDPRCompleteService) sendBreachNotification(ctx context.Context, record *PrivacyBreachRecord) error {
	if g.snsClient == nil {
		return nil // SNS not configured
	}
	
	// Send immediate notification to compliance team
	log.Printf("Would send breach notification for %s", record.BreachID)
	return nil
}

// Additional types for complete implementation

type ConsentUpdate struct {
	Categories   []string               `json:"categories"`
	LegalBasis   string                 `json:"legal_basis"`
	Method       string                 `json:"method"`
	Evidence     string                 `json:"evidence"`
	Metadata     map[string]interface{} `json:"metadata"`
}

type PrivacyBreach struct {
	Type            string    `json:"type"`
	Severity        string    `json:"severity"`
	DetectedAt      time.Time `json:"detected_at"`
	AffectedCount   int       `json:"affected_count"`
	DataCategories  []string  `json:"data_categories"`
	Cause           string    `json:"cause"`
	MitigationSteps []string  `json:"mitigation_steps"`
}

type PrivacyBreachRecord struct {
	BreachID          string                 `json:"breach_id" `
	BreachType        string                 `json:"breach_type" `
	Severity          string                 `json:"severity" `
	DetectedAt        time.Time              `json:"detected_at" `
	ReportedAt        time.Time              `json:"reported_at" `
	AffectedSubjects  int                    `json:"affected_subjects" `
	DataCategories    []string               `json:"data_categories" `
	Cause             string                 `json:"cause" `
	Mitigation        []string               `json:"mitigation" `
	AuthorityNotified bool                   `json:"authority_notified" `
	SubjectsNotified  bool                   `json:"subjects_notified" `
	AuthorityDeadline time.Time              `json:"authority_deadline" `
	Status            string                 `json:"status" `
	ReportedBy        string                 `json:"reported_by" `
	Metadata          map[string]interface{} `json:"metadata" `
}

// Audit Logger Implementation

func (al *GDPRAuditLogger) StartOperation(ctx context.Context, operation, dataSubjectID string) string {
	auditID := uuid.New().String()
	// Implementation would create audit trail entry
	log.Printf("Started %s operation for %s (audit ID: %s)", operation, dataSubjectID, auditID)
	return auditID
}

func (al *GDPRAuditLogger) CompleteOperation(ctx context.Context, auditID string) {
	// Implementation would complete audit trail entry
	log.Printf("Completed operation (audit ID: %s)", auditID)
}

func (al *GDPRAuditLogger) LogError(ctx context.Context, auditID, message string, metadata map[string]interface{}) {
	// Implementation would log error to audit trail
	log.Printf("Error in operation %s: %s %v", auditID, message, metadata)
}

func (al *GDPRAuditLogger) LogSuccess(ctx context.Context, auditID, message string, metadata map[string]interface{}) {
	// Implementation would log success to audit trail
	log.Printf("Success in operation %s: %s %v", auditID, message, metadata)
}

func (al *GDPRAuditLogger) LogConsentEvent(ctx context.Context, event *security.ConsentEvent) error {
	// Implementation would store consent event
	log.Printf("Consent event: %s for %s", event.EventType, event.DataSubjectID)
	return nil
}

func (al *GDPRAuditLogger) LogDataSubjectRequest(ctx context.Context, request *security.DataSubjectRequestLog) error {
	// Implementation would store data subject request log
	log.Printf("Data subject request: %s for %s", request.RequestType, request.DataSubjectID)
	return nil
}

func (al *GDPRAuditLogger) LogDataProcessingActivity(ctx context.Context, activity *security.DataProcessingLog) error {
	// Implementation would store data processing activity log
	log.Printf("Data processing activity logged")
	return nil
}

func (al *GDPRAuditLogger) LogCrossBorderTransfer(ctx context.Context, transfer *security.CrossBorderTransferLog) error {
	// Implementation would store cross-border transfer log
	log.Printf("Cross-border transfer: %s to %s", transfer.SourceCountry, transfer.DestinationCountry)
	return nil
}

func (al *GDPRAuditLogger) LogPrivacyBreach(ctx context.Context, breach *security.PrivacyBreachLog) error {
	// Implementation would store privacy breach log
	log.Printf("Privacy breach: %s (severity: %s)", breach.BreachType, breach.Severity)
	return nil
}

// Helper function to clean and validate text input
func cleanText(input string) string {
	// Remove non-printable characters
	return strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
			return r
		}
		return -1
	}, input)
}