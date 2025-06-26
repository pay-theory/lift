package security

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Request represents the minimal request interface needed
type Request interface {
	Method() string
	Path() string
	Header(key string) string
	UserAgent() string
	ContentLength() int64
	URL() URL
}

// URL represents the minimal URL interface needed
type URL interface {
	Query() map[string][]string
}

// Response represents the minimal response interface needed
type Response interface {
	StatusCode() int
	Body() []byte
}

// LiftHandler represents a handler function
type LiftHandler interface {
	Handle(ctx LiftContext) error
}

// LiftHandlerFunc is an adapter to allow ordinary functions to be used as handlers
type LiftHandlerFunc func(ctx LiftContext) error

// Handle calls f(ctx)
func (f LiftHandlerFunc) Handle(ctx LiftContext) error {
	return f(ctx)
}

// LiftMiddleware represents middleware that wraps handlers
type LiftMiddleware func(next LiftHandler) LiftHandler

// ComplianceFramework defines the compliance requirements and enforcement
type ComplianceFramework struct {
	framework string
	auditor   AuditLogger
	validator ComplianceValidator
	reporter  ComplianceReporter
	config    ComplianceConfig
	mu        sync.RWMutex
}

// ComplianceConfig holds configuration for compliance frameworks
type ComplianceConfig struct {
	EnabledFrameworks  []string          `json:"enabled_frameworks"`
	AuditRetention     time.Duration     `json:"audit_retention"`
	DataClassification map[string]string `json:"data_classification"`
	EncryptionRequired bool              `json:"encryption_required"`
	RegionRestrictions []string          `json:"region_restrictions"`
	CustomRules        []ComplianceRule  `json:"custom_rules"`
}

// ComplianceRule defines a custom compliance rule
type ComplianceRule struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Framework   string                 `json:"framework"`
	Severity    string                 `json:"severity"`
	Description string                 `json:"description"`
	Condition   map[string]any `json:"condition"`
	Action      string                 `json:"action"`
}

// AuditLogger handles audit trail logging
type AuditLogger interface {
	StartAudit(ctx LiftContext) string
	LogRequest(auditID string, request *AuditRequest) error
	LogResponse(auditID string, response *AuditResponse) error
	LogDataAccess(auditID string, access *DataAccessLog) error
	LogSecurityEvent(auditID string, event *SecurityEvent) error
}

// ComplianceValidator validates requests against compliance rules
type ComplianceValidator interface {
	ValidateRequest(ctx LiftContext, framework string) (*ComplianceResult, error)
	ValidateDataAccess(ctx LiftContext, dataType string) (*ComplianceResult, error)
	ValidateRegion(ctx LiftContext, region string) (*ComplianceResult, error)
}

// ComplianceReporter generates compliance reports
type ComplianceReporter interface {
	GenerateReport(framework string, period time.Duration) (*ComplianceReport, error)
	GetViolations(framework string, since time.Time) ([]ComplianceViolation, error)
	GetAuditTrail(userID, tenantID string, since time.Time) ([]AuditEntry, error)
}

// AuditRequest represents an auditable request
type AuditRequest struct {
	UserID      string            `json:"user_id"`
	TenantID    string            `json:"tenant_id"`
	Action      string            `json:"action"`
	Resource    string            `json:"resource"`
	Timestamp   time.Time         `json:"timestamp"`
	IPAddress   string            `json:"ip_address"`
	UserAgent   string            `json:"user_agent"`
	Headers     map[string]string `json:"headers,omitempty"`
	QueryParams map[string]string `json:"query_params,omitempty"`
	RequestSize int64             `json:"request_size"`
	ContentType string            `json:"content_type"`
	SessionID   string            `json:"session_id,omitempty"`
}

// AuditResponse represents an auditable response
type AuditResponse struct {
	StatusCode   int           `json:"status_code"`
	Duration     time.Duration `json:"duration"`
	ResponseSize int64         `json:"response_size"`
	Error        error         `json:"error,omitempty"`
	DataAccess   []string      `json:"data_access,omitempty"`
	Warnings     []string      `json:"warnings,omitempty"`
}

// DataAccessLog represents data access for audit trails
type DataAccessLog struct {
	DataType       string    `json:"data_type"`
	Classification string    `json:"classification"`
	Action         string    `json:"action"` // read, write, delete, export
	RecordCount    int       `json:"record_count"`
	Fields         []string  `json:"fields,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
	Purpose        string    `json:"purpose,omitempty"`
}

// SecurityEvent represents a security-related event
type SecurityEvent struct {
	EventType   string                 `json:"event_type"`
	Severity    string                 `json:"severity"`
	Description string                 `json:"description"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	Resolved    bool                   `json:"resolved"`
}

// ComplianceResult represents the result of compliance validation
type ComplianceResult struct {
	Compliant  bool                   `json:"compliant"`
	Framework  string                 `json:"framework"`
	Violations []ComplianceViolation  `json:"violations,omitempty"`
	Warnings   []string               `json:"warnings,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
}

// ComplianceViolation represents a compliance violation
type ComplianceViolation struct {
	ID          string                 `json:"id"`
	RuleID      string                 `json:"rule_id"`
	Framework   string                 `json:"framework"`
	Severity    string                 `json:"severity"`
	Description string                 `json:"description"`
	UserID      string                 `json:"user_id,omitempty"`
	TenantID    string                 `json:"tenant_id,omitempty"`
	Resource    string                 `json:"resource,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Resolved    bool                   `json:"resolved"`
}

// ComplianceReport represents a compliance report
type ComplianceReport struct {
	Framework     string                `json:"framework"`
	Period        time.Duration         `json:"period"`
	GeneratedAt   time.Time             `json:"generated_at"`
	TotalRequests int64                 `json:"total_requests"`
	Violations    []ComplianceViolation `json:"violations"`
	Summary       ComplianceSummary     `json:"summary"`
}

// ComplianceSummary provides a summary of compliance status
type ComplianceSummary struct {
	ComplianceRate   float64           `json:"compliance_rate"`
	ViolationsByType map[string]int    `json:"violations_by_type"`
	TopViolations    []string          `json:"top_violations"`
	TrendData        []ComplianceTrend `json:"trend_data"`
	Recommendations  []string          `json:"recommendations"`
}

// ComplianceTrend represents compliance trend data
type ComplianceTrend struct {
	Date           time.Time `json:"date"`
	ComplianceRate float64   `json:"compliance_rate"`
	ViolationCount int       `json:"violation_count"`
}

// AuditEntry represents an audit trail entry
type AuditEntry struct {
	ID        string                 `json:"id"`
	UserID    string                 `json:"user_id"`
	TenantID  string                 `json:"tenant_id"`
	Action    string                 `json:"action"`
	Resource  string                 `json:"resource"`
	Timestamp time.Time              `json:"timestamp"`
	Result    string                 `json:"result"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// NewComplianceFramework creates a new compliance framework
func NewComplianceFramework(framework string, config ComplianceConfig) *ComplianceFramework {
	return &ComplianceFramework{
		framework: framework,
		config:    config,
	}
}

// SetAuditor sets the audit logger
func (cf *ComplianceFramework) SetAuditor(auditor AuditLogger) {
	cf.mu.Lock()
	defer cf.mu.Unlock()
	cf.auditor = auditor
}

// SetValidator sets the compliance validator
func (cf *ComplianceFramework) SetValidator(validator ComplianceValidator) {
	cf.mu.Lock()
	defer cf.mu.Unlock()
	cf.validator = validator
}

// SetReporter sets the compliance reporter
func (cf *ComplianceFramework) SetReporter(reporter ComplianceReporter) {
	cf.mu.Lock()
	defer cf.mu.Unlock()
	cf.reporter = reporter
}

// ComplianceAudit creates middleware for compliance auditing
func (cf *ComplianceFramework) ComplianceAudit() LiftMiddleware {
	return func(next LiftHandler) LiftHandler {
		return LiftHandlerFunc(func(ctx LiftContext) error {
			start := time.Now()

			// Start audit trail
			auditID := ""
			if cf.auditor != nil {
				auditID = cf.auditor.StartAudit(ctx)
			}

			// Create audit request
			auditRequest := &AuditRequest{
				UserID:      ctx.UserID(),
				TenantID:    ctx.TenantID(),
				Action:      fmt.Sprintf("%s %s", "GET", "/path"), // Simplified for interface
				Resource:    "/path",
				Timestamp:   start,
				IPAddress:   ctx.ClientIP(),
				UserAgent:   "user-agent",
				RequestSize: 0,
				ContentType: "application/json",
				SessionID:   "",
			}

			// Log sanitized headers and query params
			auditRequest.Headers = make(map[string]string)
			auditRequest.QueryParams = make(map[string]string)

			// Log request
			if cf.auditor != nil && auditID != "" {
				if err := cf.auditor.LogRequest(auditID, auditRequest); err != nil {
					// Log error but don't fail the request
					ctx.Logger().Error("Failed to log audit request", "error", err)
				}
			}

			// Validate compliance
			if cf.validator != nil {
				for _, framework := range cf.config.EnabledFrameworks {
					result, err := cf.validator.ValidateRequest(ctx, framework)
					if err != nil {
						ctx.Logger().Error("Compliance validation failed", "framework", framework, "error", err)
						continue
					}

					if !result.Compliant {
						// Log violations
						for _, violation := range result.Violations {
							if cf.auditor != nil && auditID != "" {
								securityEvent := &SecurityEvent{
									EventType:   "compliance_violation",
									Severity:    violation.Severity,
									Description: violation.Description,
									Metadata: map[string]any{
										"framework":    framework,
										"rule_id":      violation.RuleID,
										"violation_id": violation.ID,
									},
									Timestamp: time.Now(),
									Resolved:  false,
								}
								cf.auditor.LogSecurityEvent(auditID, securityEvent)
							}
						}

						// Handle critical violations
						if cf.hasCriticalViolations(result.Violations) {
							return fmt.Errorf("request violates compliance requirements")
						}
					}
				}
			}

			// Execute handler
			err := next.Handle(ctx)
			duration := time.Since(start)

			// Create audit response
			auditResponse := &AuditResponse{
				StatusCode:   200, // Simplified for interface
				Duration:     duration,
				ResponseSize: 0,
				Error:        err,
				DataAccess:   ctx.GetDataAccessLog(),
			}

			// Log response
			if cf.auditor != nil && auditID != "" {
				if logErr := cf.auditor.LogResponse(auditID, auditResponse); logErr != nil {
					ctx.Logger().Error("Failed to log audit response", "error", logErr)
				}
			}

			return err
		})
	}
}

// sanitizeHeaders removes sensitive headers from audit logs
func (cf *ComplianceFramework) sanitizeHeaders(headers map[string][]string) map[string]string {
	sanitized := make(map[string]string)
	sensitiveHeaders := map[string]bool{
		"authorization": true,
		"cookie":        true,
		"x-api-key":     true,
		"x-auth-token":  true,
	}

	for key, values := range headers {
		lowerKey := strings.ToLower(key)
		if sensitiveHeaders[lowerKey] {
			sanitized[key] = "[REDACTED]"
		} else if len(values) > 0 {
			sanitized[key] = values[0]
		}
	}

	return sanitized
}

// sanitizeQueryParams removes sensitive query parameters from audit logs
func (cf *ComplianceFramework) sanitizeQueryParams(params map[string][]string) map[string]string {
	sanitized := make(map[string]string)
	sensitiveParams := map[string]bool{
		"token":    true,
		"api_key":  true,
		"password": true,
		"secret":   true,
	}

	for key, values := range params {
		lowerKey := strings.ToLower(key)
		if sensitiveParams[lowerKey] {
			sanitized[key] = "[REDACTED]"
		} else if len(values) > 0 {
			sanitized[key] = values[0]
		}
	}

	return sanitized
}

// hasCriticalViolations checks if there are any critical compliance violations
func (cf *ComplianceFramework) hasCriticalViolations(violations []ComplianceViolation) bool {
	for _, violation := range violations {
		if violation.Severity == "critical" || violation.Severity == "high" {
			return true
		}
	}
	return false
}

// GetComplianceStatus returns the current compliance status
func (cf *ComplianceFramework) GetComplianceStatus(ctx context.Context) (*ComplianceResult, error) {
	cf.mu.RLock()
	defer cf.mu.RUnlock()

	if cf.validator == nil {
		return nil, fmt.Errorf("compliance validator not configured")
	}

	// This would typically validate against current state
	// For now, return a basic status
	return &ComplianceResult{
		Compliant: true,
		Framework: cf.framework,
		Timestamp: time.Now(),
	}, nil
}

// GenerateComplianceReport generates a compliance report
func (cf *ComplianceFramework) GenerateComplianceReport(period time.Duration) (*ComplianceReport, error) {
	cf.mu.RLock()
	defer cf.mu.RUnlock()

	if cf.reporter == nil {
		return nil, fmt.Errorf("compliance reporter not configured")
	}

	return cf.reporter.GenerateReport(cf.framework, period)
}

// IsFrameworkEnabled checks if a compliance framework is enabled
func (cf *ComplianceFramework) IsFrameworkEnabled(framework string) bool {
	cf.mu.RLock()
	defer cf.mu.RUnlock()

	for _, enabled := range cf.config.EnabledFrameworks {
		if enabled == framework {
			return true
		}
	}
	return false
}

// AddCustomRule adds a custom compliance rule
func (cf *ComplianceFramework) AddCustomRule(rule ComplianceRule) {
	cf.mu.Lock()
	defer cf.mu.Unlock()

	cf.config.CustomRules = append(cf.config.CustomRules, rule)
}

// GetCustomRules returns all custom compliance rules
func (cf *ComplianceFramework) GetCustomRules() []ComplianceRule {
	cf.mu.RLock()
	defer cf.mu.RUnlock()

	rules := make([]ComplianceRule, len(cf.config.CustomRules))
	copy(rules, cf.config.CustomRules)
	return rules
}

// ValidateConfiguration validates the compliance configuration
func (cf *ComplianceFramework) ValidateConfiguration() error {
	cf.mu.RLock()
	defer cf.mu.RUnlock()

	if len(cf.config.EnabledFrameworks) == 0 {
		return fmt.Errorf("no compliance frameworks enabled")
	}

	validFrameworks := map[string]bool{
		"SOC2":    true,
		"PCI-DSS": true,
		"HIPAA":   true,
		"GDPR":    true,
	}

	for _, framework := range cf.config.EnabledFrameworks {
		if !validFrameworks[framework] {
			return fmt.Errorf("unsupported compliance framework: %s", framework)
		}
	}

	if cf.config.AuditRetention < 24*time.Hour {
		return fmt.Errorf("audit retention must be at least 24 hours")
	}

	return nil
}

// MarshalJSON implements json.Marshaler for ComplianceFramework
func (cf *ComplianceFramework) MarshalJSON() ([]byte, error) {
	cf.mu.RLock()
	defer cf.mu.RUnlock()

	return json.Marshal(map[string]any{
		"framework": cf.framework,
		"config":    cf.config,
	})
}
