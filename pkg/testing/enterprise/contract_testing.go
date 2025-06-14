package enterprise

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ContractTestingFramework provides contract testing capabilities
type ContractTestingFramework struct {
	registry  *ContractRegistry
	validator *BasicContractValidator
	runner    *ContractTestRunner
	reporter  *ContractTestReporter
	config    *ContractTestingConfig
}

// ContractTestingConfig configures contract testing
type ContractTestingConfig struct {
	DefaultTimeout    time.Duration          `json:"default_timeout"`
	MaxRetries        int                    `json:"max_retries"`
	ParallelExecution bool                   `json:"parallel_execution"`
	ReportFormat      string                 `json:"report_format"`
	Environment       map[string]interface{} `json:"environment"`
}

// BasicContractValidator implements ContractValidator interface
type BasicContractValidator struct {
	rules   []ValidationRule
	config  *ValidationConfig
	metrics *ValidationMetrics
}

// ValidationConfig configures contract validation
type ValidationConfig struct {
	StrictMode      bool          `json:"strict_mode"`
	Timeout         time.Duration `json:"timeout"`
	MaxErrors       int           `json:"max_errors"`
	FailFast        bool          `json:"fail_fast"`
	ValidateHeaders bool          `json:"validate_headers"`
	ValidateBody    bool          `json:"validate_body"`
}

// ValidationMetrics tracks validation performance
type ValidationMetrics struct {
	TotalValidations      int64         `json:"total_validations"`
	SuccessfulValidations int64         `json:"successful_validations"`
	FailedValidations     int64         `json:"failed_validations"`
	AverageTime           time.Duration `json:"average_time"`
	LastValidation        time.Time     `json:"last_validation"`
}

// NewContractTestingFramework creates a new contract testing framework
func NewContractTestingFramework(config *ContractTestConfig) *ContractTestingFramework {
	if config == nil {
		config = &ContractTestConfig{
			Environment:    "test",
			Timeout:        30 * time.Second,
			RetryAttempts:  3,
			RetryDelay:     1 * time.Second,
			StrictMode:     false,
			Parallel:       true,
			MaxConcurrency: 5,
		}
	}

	return &ContractTestingFramework{
		registry:  NewContractRegistry(nil),
		validator: NewBasicContractValidator(),
		runner:    NewContractTestRunner(nil),
		reporter:  NewContractTestReporter(),
		config: &ContractTestingConfig{
			DefaultTimeout:    config.Timeout,
			MaxRetries:        config.RetryAttempts,
			ParallelExecution: config.Parallel,
			ReportFormat:      "json",
			Environment:       make(map[string]interface{}),
		},
	}
}

// NewBasicContractValidator creates a new basic contract validator
func NewBasicContractValidator() *BasicContractValidator {
	return &BasicContractValidator{
		rules: []ValidationRule{},
		config: &ValidationConfig{
			StrictMode:      false,
			Timeout:         10 * time.Second,
			MaxErrors:       10,
			FailFast:        false,
			ValidateHeaders: true,
			ValidateBody:    true,
		},
		metrics: &ValidationMetrics{
			LastValidation: time.Now(),
		},
	}
}

// Validate validates a contract interaction
func (v *BasicContractValidator) Validate(ctx context.Context, interaction *ContractInteraction) error {
	start := time.Now()
	defer func() {
		v.updateMetrics(time.Since(start))
	}()

	// Validate request
	if err := v.validateRequest(interaction.Request); err != nil {
		v.metrics.FailedValidations++
		return fmt.Errorf("request validation failed: %w", err)
	}

	// Validate response
	if err := v.validateResponse(interaction.Response); err != nil {
		v.metrics.FailedValidations++
		return fmt.Errorf("response validation failed: %w", err)
	}

	v.metrics.SuccessfulValidations++
	return nil
}

// validateRequest validates an interaction request
func (v *BasicContractValidator) validateRequest(request *InteractionRequest) error {
	if request == nil {
		return fmt.Errorf("request cannot be nil")
	}

	if request.Method == "" {
		return fmt.Errorf("request method cannot be empty")
	}

	if request.Path == "" {
		return fmt.Errorf("request path cannot be empty")
	}

	return nil
}

// validateResponse validates an interaction response
func (v *BasicContractValidator) validateResponse(response *InteractionResponse) error {
	if response == nil {
		return fmt.Errorf("response cannot be nil")
	}

	if response.Status < 100 || response.Status > 599 {
		return fmt.Errorf("invalid response status: %d", response.Status)
	}

	return nil
}

// updateMetrics updates validation metrics
func (v *BasicContractValidator) updateMetrics(duration time.Duration) {
	v.metrics.TotalValidations++

	// Calculate average time
	if v.metrics.TotalValidations > 0 {
		totalTime := v.metrics.AverageTime * time.Duration(v.metrics.TotalValidations-1)
		v.metrics.AverageTime = (totalTime + duration) / time.Duration(v.metrics.TotalValidations)
	} else {
		v.metrics.AverageTime = duration
	}

	v.metrics.LastValidation = time.Now()
}

// CreateContractTest creates a new contract test
func (f *ContractTestingFramework) CreateContractTest(contract *ServiceContract, validator ContractValidator) (*ContractTest, error) {
	if contract == nil {
		return nil, fmt.Errorf("contract cannot be nil")
	}

	if validator == nil {
		validator = f.validator
	}

	test := &ContractTest{
		Provider:  contract.Provider.Name,
		Consumer:  contract.Consumer.Name,
		Contract:  contract,
		Validator: validator,
	}

	return test, nil
}

// RunContractTest executes a contract test
func (f *ContractTestingFramework) RunContractTest(ctx context.Context, test *ContractTest) (*ContractTestResult, error) {
	startTime := time.Now()

	// Validate contract
	if _, err := test.Validator.ValidateContract(ctx, test.Contract); err != nil {
		return &ContractTestResult{
			ContractID: test.Contract.ID,
			Provider:   test.Provider,
			Consumer:   test.Consumer,
			StartTime:  startTime,
			EndTime:    time.Now(),
			Duration:   time.Since(startTime),
			Status:     TestStatusFailed,
		}, err
	}

	return &ContractTestResult{
		ContractID: test.Contract.ID,
		Provider:   test.Provider,
		Consumer:   test.Consumer,
		StartTime:  startTime,
		EndTime:    time.Now(),
		Duration:   time.Since(startTime),
		Status:     TestStatusPassed,
	}, nil
}

// NewContractRegistry creates a new contract registry
func NewContractRegistry(config *ContractTestConfig) *ContractRegistry {
	return &ContractRegistry{
		contracts: make(map[string]*ServiceContract),
		versions:  make(map[string][]string),
	}
}

// NewContractTestRunner creates a new contract test runner
func NewContractTestRunner(config *ContractTestConfig) *ContractTestRunner {
	return &ContractTestRunner{
		registry: NewContractRegistry(config),
		config: &TestConfig{
			Timeout:  30 * time.Second,
			Retries:  3,
			Parallel: true,
		},
	}
}

// NewContractTestReporter creates a new contract test reporter
func NewContractTestReporter() *ContractTestReporter {
	return &ContractTestReporter{
		templates: make(map[string]*ContractReportTemplate),
		exporters: make(map[string]ContractReportExporter),
	}
}

// ValidateContract validates a contract (implementing ContractValidator interface)
func (v *BasicContractValidator) ValidateContract(ctx context.Context, contract *ServiceContract) (*TestResult, error) {
	start := time.Now()

	if contract == nil {
		return &TestResult{
			TestID:    fmt.Sprintf("validation-%d", time.Now().Unix()),
			Status:    TestStatusFailed,
			StartTime: start,
			EndTime:   time.Now(),
			Duration:  time.Since(start),
			Passed:    false,
			Errors:    []string{"contract cannot be nil"},
		}, fmt.Errorf("contract cannot be nil")
	}

	// Validate all interactions
	for _, interaction := range contract.Interactions {
		if err := v.Validate(ctx, &interaction); err != nil {
			return &TestResult{
				TestID:    fmt.Sprintf("validation-%d", time.Now().Unix()),
				Status:    TestStatusFailed,
				StartTime: start,
				EndTime:   time.Now(),
				Duration:  time.Since(start),
				Passed:    false,
				Errors:    []string{err.Error()},
			}, err
		}
	}

	return &TestResult{
		TestID:    fmt.Sprintf("validation-%d", time.Now().Unix()),
		Status:    TestStatusPassed,
		StartTime: start,
		EndTime:   time.Now(),
		Duration:  time.Since(start),
		Passed:    true,
		Errors:    []string{},
	}, nil
}

// ValidateInteraction validates a contract interaction (implementing ContractValidator interface)
func (v *BasicContractValidator) ValidateInteraction(ctx context.Context, interaction *ContractInteraction) (*InteractionResult, error) {
	if err := v.Validate(ctx, interaction); err != nil {
		return &InteractionResult{
			InteractionID: interaction.ID,
			Status:        TestStatusFailed,
			Request:       interaction.Request,
			Response:      interaction.Response,
			Expected:      interaction.Response,
			Errors:        []string{err.Error()},
		}, err
	}

	return &InteractionResult{
		InteractionID: interaction.ID,
		Status:        TestStatusPassed,
		Request:       interaction.Request,
		Response:      interaction.Response,
		Expected:      interaction.Response,
		Errors:        []string{},
	}, nil
}

// ValidateContract validates a complete service contract (implementing ContractTestingFramework method)
func (f *ContractTestingFramework) ValidateContract(ctx context.Context, contract *ServiceContract) (*ContractValidationResult, error) {
	startTime := time.Now()

	result := &ContractValidationResult{
		ID:          fmt.Sprintf("validation-%d", time.Now().Unix()),
		ContractID:  contract.ID,
		Status:      TestStatusPassed,
		Errors:      []string{},
		Warnings:    []string{},
		Validations: make(map[string]*InteractionValidation),
		Timestamp:   startTime,
		Metadata:    make(map[string]interface{}),
	}

	// Validate each interaction
	for _, interaction := range contract.Interactions {
		validation := f.validateInteraction(&interaction)
		result.Validations[interaction.ID] = &validation

		if validation.Status == "failed" {
			result.Status = TestStatusFailed
			result.Errors = append(result.Errors, validation.Errors...)
		}
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// validateInteraction validates a single contract interaction
func (f *ContractTestingFramework) validateInteraction(interaction *ContractInteraction) InteractionValidation {
	start := time.Now()
	validation := InteractionValidation{
		InteractionID: interaction.ID,
		Status:        "passed",
		Checks:        make(map[string]*ValidationCheck),
		Errors:        []string{},
		Warnings:      []string{},
		Duration:      0,
		Timestamp:     start,
	}

	// Validate request
	if interaction.Request != nil {
		if check := f.validateHTTPMethod(interaction.Request.Method); !check.Valid {
			validation.Checks["http_method"] = check
			validation.Status = "failed"
			validation.Errors = append(validation.Errors, check.Errors...)
		}

		if check := f.validateHTTPPath(interaction.Request.Path); !check.Valid {
			validation.Checks["http_path"] = check
			validation.Status = "failed"
			validation.Errors = append(validation.Errors, check.Errors...)
		}

		if check := f.validateHeaders(interaction.Request.Headers); !check.Valid {
			validation.Checks["request_headers"] = check
			validation.Status = "failed"
			validation.Errors = append(validation.Errors, check.Errors...)
		}
	}

	// Validate response
	if interaction.Response != nil {
		if check := f.validateHeaders(interaction.Response.Headers); !check.Valid {
			validation.Checks["response_headers"] = check
			validation.Status = "failed"
			validation.Errors = append(validation.Errors, check.Errors...)
		}
	}

	validation.Duration = time.Since(start)
	return validation
}

// validateSchema validates data against a JSON schema
func (f *ContractTestingFramework) validateSchema(data interface{}, schema *SchemaDefinition) (*ValidationCheck, error) {
	check := &ValidationCheck{
		ID:          fmt.Sprintf("schema-%d", time.Now().Unix()),
		Name:        "Schema Validation",
		Description: "Validates data against JSON schema",
		Status:      "passed",
		Valid:       true,
		Expected:    schema,
		Actual:      data,
		Errors:      []string{},
		Warnings:    []string{},
		Metadata:    make(map[string]interface{}),
	}

	if schema == nil {
		return check, nil
	}

	// Basic type validation
	if !f.validateType(data, schema.Type) {
		check.Valid = false
		check.Status = "failed"
		check.Errors = append(check.Errors, fmt.Sprintf("Expected type %s, got %T", schema.Type, data))
		return check, nil
	}

	// String validation
	if schema.Type == "string" {
		if str, ok := data.(string); ok {
			if schema.MinLength != nil && len(str) < *schema.MinLength {
				check.Valid = false
				check.Status = "failed"
				check.Errors = append(check.Errors, fmt.Sprintf("String too short: %d < %d", len(str), *schema.MinLength))
			}
			if schema.MaxLength != nil && len(str) > *schema.MaxLength {
				check.Valid = false
				check.Status = "failed"
				check.Errors = append(check.Errors, fmt.Sprintf("String too long: %d > %d", len(str), *schema.MaxLength))
			}
		}
	}

	// Object validation
	if schema.Type == "object" {
		if obj, ok := data.(map[string]interface{}); ok {
			// Check required fields
			for _, required := range schema.Required {
				if _, exists := obj[required]; !exists {
					check.Valid = false
					check.Status = "failed"
					check.Errors = append(check.Errors, fmt.Sprintf("Missing required field: %s", required))
				}
			}
		}
	}

	return check, nil
}

// validateHTTPMethod validates HTTP method
func (f *ContractTestingFramework) validateHTTPMethod(method string) *ValidationCheck {
	check := &ValidationCheck{
		ID:          fmt.Sprintf("method-%d", time.Now().Unix()),
		Name:        "HTTP Method Validation",
		Description: "Validates HTTP method",
		Status:      "passed",
		Valid:       true,
		Expected:    "Valid HTTP method",
		Actual:      method,
		Errors:      []string{},
		Warnings:    []string{},
		Metadata:    make(map[string]interface{}),
	}

	validMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	for _, valid := range validMethods {
		if method == valid {
			return check
		}
	}

	check.Valid = false
	check.Status = "failed"
	check.Errors = append(check.Errors, fmt.Sprintf("Invalid HTTP method: %s", method))
	return check
}

// validateHTTPPath validates HTTP path
func (f *ContractTestingFramework) validateHTTPPath(path string) *ValidationCheck {
	check := &ValidationCheck{
		ID:          fmt.Sprintf("path-%d", time.Now().Unix()),
		Name:        "HTTP Path Validation",
		Description: "Validates HTTP path",
		Status:      "passed",
		Valid:       true,
		Expected:    "Valid HTTP path starting with /",
		Actual:      path,
		Errors:      []string{},
		Warnings:    []string{},
		Metadata:    make(map[string]interface{}),
	}

	if path == "" {
		check.Valid = false
		check.Status = "failed"
		check.Errors = append(check.Errors, "Path cannot be empty")
		return check
	}

	if !strings.HasPrefix(path, "/") {
		check.Valid = false
		check.Status = "failed"
		check.Errors = append(check.Errors, "Path must start with /")
		return check
	}

	return check
}

// validateHeaders validates HTTP headers
func (f *ContractTestingFramework) validateHeaders(headers map[string]string) *ValidationCheck {
	check := &ValidationCheck{
		ID:          fmt.Sprintf("headers-%d", time.Now().Unix()),
		Name:        "HTTP Headers Validation",
		Description: "Validates HTTP headers",
		Status:      "passed",
		Valid:       true,
		Expected:    "Valid HTTP headers",
		Actual:      headers,
		Errors:      []string{},
		Warnings:    []string{},
		Metadata:    make(map[string]interface{}),
	}

	for name, value := range headers {
		if name == "" {
			check.Valid = false
			check.Status = "failed"
			check.Errors = append(check.Errors, "Header name cannot be empty")
		}
		if value == "" {
			check.Valid = false
			check.Status = "failed"
			check.Errors = append(check.Errors, fmt.Sprintf("Header value cannot be empty for %s", name))
		}
	}

	return check
}

// validateType validates data type
func (f *ContractTestingFramework) validateType(data interface{}, expectedType string) bool {
	switch expectedType {
	case "string":
		_, ok := data.(string)
		return ok
	case "number":
		_, ok := data.(float64)
		return ok
	case "integer":
		if num, ok := data.(float64); ok {
			return num == float64(int(num))
		}
		return false
	case "boolean":
		_, ok := data.(bool)
		return ok
	case "array":
		_, ok := data.([]interface{})
		return ok
	case "object":
		_, ok := data.(map[string]interface{})
		return ok
	case "null":
		return data == nil
	default:
		return false
	}
}

// generateValidationSummary generates a summary of validation results
func (f *ContractTestingFramework) generateValidationSummary(validations map[string]*InteractionValidation) *ValidationSummary {
	summary := &ValidationSummary{
		TotalInteractions:   len(validations),
		ValidInteractions:   0,
		InvalidInteractions: 0,
		TotalChecks:         0,
		PassedChecks:        0,
		FailedChecks:        0,
		SuccessRate:         0.0,
	}

	for _, validation := range validations {
		if validation.Status == "passed" {
			summary.ValidInteractions++
		} else {
			summary.InvalidInteractions++
		}

		for _, check := range validation.Checks {
			summary.TotalChecks++
			if check.Status == "passed" {
				summary.PassedChecks++
			} else {
				summary.FailedChecks++
			}
		}
	}

	if summary.TotalInteractions > 0 {
		summary.SuccessRate = float64(summary.ValidInteractions) / float64(summary.TotalInteractions) * 100
	}

	return summary
}

// calculateValidationStatus calculates overall validation status
func (f *ContractTestingFramework) calculateValidationStatus(validations map[string]*InteractionValidation) string {
	for _, validation := range validations {
		if validation.Status == "failed" {
			return "failed"
		}
	}
	return "passed"
}

// calculateInteractionStatus calculates interaction status from checks
func (f *ContractTestingFramework) calculateInteractionStatus(checks map[string]*ValidationCheck) string {
	for _, check := range checks {
		if check.Status == "failed" {
			return "failed"
		}
	}
	return "passed"
}

// Contract Testing Implementation leverages existing framework
