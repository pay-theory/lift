package features

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
)

// ValidationRule defines a validation rule
type ValidationRule struct {
	Field      string                  `json:"field"`
	Type       string                  `json:"type"`
	Required   bool                    `json:"required"`
	Min        interface{}             `json:"min,omitempty"`
	Max        interface{}             `json:"max,omitempty"`
	Pattern    string                  `json:"pattern,omitempty"`
	Enum       []interface{}           `json:"enum,omitempty"`
	Custom     func(interface{}) error `json:"-"`
	Message    string                  `json:"message,omitempty"`
	Conditions []ValidationCondition   `json:"conditions,omitempty"`
}

// ValidationCondition defines conditional validation
type ValidationCondition struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"` // "eq", "ne", "gt", "lt", "in", "not_in"
	Value    interface{} `json:"value"`
}

// ValidationSchema defines a complete validation schema
type ValidationSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]ValidationRule `json:"properties"`
	Required   []string                  `json:"required"`
	Rules      []ValidationRule          `json:"rules"`
	Custom     func(interface{}) error   `json:"-"`
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string      `json:"field"`
	Message string      `json:"message"`
	Value   interface{} `json:"value,omitempty"`
	Code    string      `json:"code"`
}

// ValidationResult contains validation results
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// ValidationConfig configures validation behavior
type ValidationConfig struct {
	RequestSchema    *ValidationSchema
	ResponseSchema   *ValidationSchema
	ValidateRequest  bool
	ValidateResponse bool
	StrictMode       bool
	CustomValidators map[string]func(interface{}) error
	ErrorHandler     func(*lift.Context, []ValidationError) error
}

// ValidationMiddleware provides advanced validation
type ValidationMiddleware struct {
	config ValidationConfig
}

// NewValidationMiddleware creates a new validation middleware
func NewValidationMiddleware(config ValidationConfig) *ValidationMiddleware {
	if config.ErrorHandler == nil {
		config.ErrorHandler = defaultErrorHandler
	}

	return &ValidationMiddleware{config: config}
}

// Validate creates the validation middleware
func (vm *ValidationMiddleware) Validate() lift.Middleware {
	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			// Validate request if configured
			if vm.config.ValidateRequest && vm.config.RequestSchema != nil {
				if err := vm.validateRequest(ctx); err != nil {
					return err
				}
			}

			// Execute handler
			err := next.Handle(ctx)
			if err != nil {
				return err
			}

			// Validate response if configured
			if vm.config.ValidateResponse && vm.config.ResponseSchema != nil {
				if err := vm.validateResponse(ctx); err != nil {
					return err
				}
			}

			return nil
		})
	}
}

func (vm *ValidationMiddleware) validateRequest(ctx *lift.Context) error {
	// Parse request body
	var requestData interface{}
	if err := ctx.ParseRequest(&requestData); err != nil {
		return vm.config.ErrorHandler(ctx, []ValidationError{
			{
				Field:   "body",
				Message: "Invalid JSON format",
				Code:    "INVALID_JSON",
			},
		})
	}

	// Validate against schema
	result := vm.validateData(requestData, vm.config.RequestSchema)
	if !result.Valid {
		return vm.config.ErrorHandler(ctx, result.Errors)
	}

	return nil
}

func (vm *ValidationMiddleware) validateResponse(ctx *lift.Context) error {
	// This would need to capture the response data
	// For now, we'll skip response validation
	return nil
}

func (vm *ValidationMiddleware) validateData(data interface{}, schema *ValidationSchema) ValidationResult {
	result := ValidationResult{Valid: true, Errors: []ValidationError{}}

	// Convert data to map for easier processing
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "root",
			Message: "Data must be an object",
			Code:    "INVALID_TYPE",
		})
		return result
	}

	// Validate required fields
	for _, field := range schema.Required {
		if _, exists := dataMap[field]; !exists {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   field,
				Message: fmt.Sprintf("Field '%s' is required", field),
				Code:    "REQUIRED_FIELD",
			})
		}
	}

	// Validate properties
	for field, rule := range schema.Properties {
		value, exists := dataMap[field]

		// Check if field is required
		if rule.Required && !exists {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   field,
				Message: fmt.Sprintf("Field '%s' is required", field),
				Code:    "REQUIRED_FIELD",
			})
			continue
		}

		// Skip validation if field doesn't exist and is not required
		if !exists {
			continue
		}

		// Validate field
		if err := vm.validateField(field, value, rule); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, *err)
		}
	}

	// Validate custom rules
	for _, rule := range schema.Rules {
		if rule.Custom != nil {
			if err := rule.Custom(data); err != nil {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					Field:   rule.Field,
					Message: err.Error(),
					Code:    "CUSTOM_VALIDATION",
				})
			}
		}
	}

	// Run schema-level custom validation
	if schema.Custom != nil {
		if err := schema.Custom(data); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   "root",
				Message: err.Error(),
				Code:    "CUSTOM_VALIDATION",
			})
		}
	}

	return result
}

func (vm *ValidationMiddleware) validateField(field string, value interface{}, rule ValidationRule) *ValidationError {
	// Check conditions first
	if len(rule.Conditions) > 0 {
		// For now, skip conditional validation
		// This would require access to the full data context
	}

	// Type validation
	if rule.Type != "" {
		if err := vm.validateType(field, value, rule.Type); err != nil {
			return err
		}
	}

	// Min/Max validation
	if rule.Min != nil || rule.Max != nil {
		if err := vm.validateRange(field, value, rule.Min, rule.Max); err != nil {
			return err
		}
	}

	// Pattern validation
	if rule.Pattern != "" {
		if err := vm.validatePattern(field, value, rule.Pattern); err != nil {
			return err
		}
	}

	// Enum validation
	if len(rule.Enum) > 0 {
		if err := vm.validateEnum(field, value, rule.Enum); err != nil {
			return err
		}
	}

	// Custom validation
	if rule.Custom != nil {
		if err := rule.Custom(value); err != nil {
			message := err.Error()
			if rule.Message != "" {
				message = rule.Message
			}
			return &ValidationError{
				Field:   field,
				Message: message,
				Value:   value,
				Code:    "CUSTOM_VALIDATION",
			}
		}
	}

	return nil
}

func (vm *ValidationMiddleware) validateType(field string, value interface{}, expectedType string) *ValidationError {
	actualType := vm.getValueType(value)

	if actualType != expectedType {
		return &ValidationError{
			Field:   field,
			Message: fmt.Sprintf("Expected type '%s', got '%s'", expectedType, actualType),
			Value:   value,
			Code:    "INVALID_TYPE",
		}
	}

	return nil
}

func (vm *ValidationMiddleware) validateRange(field string, value interface{}, min, max interface{}) *ValidationError {
	switch v := value.(type) {
	case string:
		length := len(v)
		if min != nil {
			if minLen, ok := min.(int); ok && length < minLen {
				return &ValidationError{
					Field:   field,
					Message: fmt.Sprintf("String length must be at least %d", minLen),
					Value:   value,
					Code:    "MIN_LENGTH",
				}
			}
		}
		if max != nil {
			if maxLen, ok := max.(int); ok && length > maxLen {
				return &ValidationError{
					Field:   field,
					Message: fmt.Sprintf("String length must be at most %d", maxLen),
					Value:   value,
					Code:    "MAX_LENGTH",
				}
			}
		}
	case int, int32, int64, float32, float64:
		numValue := vm.toFloat64(v)
		if min != nil {
			if minVal := vm.toFloat64(min); numValue < minVal {
				return &ValidationError{
					Field:   field,
					Message: fmt.Sprintf("Value must be at least %v", min),
					Value:   value,
					Code:    "MIN_VALUE",
				}
			}
		}
		if max != nil {
			if maxVal := vm.toFloat64(max); numValue > maxVal {
				return &ValidationError{
					Field:   field,
					Message: fmt.Sprintf("Value must be at most %v", max),
					Value:   value,
					Code:    "MAX_VALUE",
				}
			}
		}
	case []interface{}:
		length := len(v)
		if min != nil {
			if minLen, ok := min.(int); ok && length < minLen {
				return &ValidationError{
					Field:   field,
					Message: fmt.Sprintf("Array length must be at least %d", minLen),
					Value:   value,
					Code:    "MIN_ITEMS",
				}
			}
		}
		if max != nil {
			if maxLen, ok := max.(int); ok && length > maxLen {
				return &ValidationError{
					Field:   field,
					Message: fmt.Sprintf("Array length must be at most %d", maxLen),
					Value:   value,
					Code:    "MAX_ITEMS",
				}
			}
		}
	}

	return nil
}

func (vm *ValidationMiddleware) validatePattern(field string, value interface{}, pattern string) *ValidationError {
	str, ok := value.(string)
	if !ok {
		return &ValidationError{
			Field:   field,
			Message: "Pattern validation only applies to strings",
			Value:   value,
			Code:    "INVALID_TYPE",
		}
	}

	regex, err := regexp.Compile(pattern)
	if err != nil {
		return &ValidationError{
			Field:   field,
			Message: "Invalid regex pattern",
			Value:   value,
			Code:    "INVALID_PATTERN",
		}
	}

	if !regex.MatchString(str) {
		return &ValidationError{
			Field:   field,
			Message: fmt.Sprintf("Value does not match pattern '%s'", pattern),
			Value:   value,
			Code:    "PATTERN_MISMATCH",
		}
	}

	return nil
}

func (vm *ValidationMiddleware) validateEnum(field string, value interface{}, enum []interface{}) *ValidationError {
	for _, enumValue := range enum {
		if vm.valuesEqual(value, enumValue) {
			return nil
		}
	}

	return &ValidationError{
		Field:   field,
		Message: fmt.Sprintf("Value must be one of: %v", enum),
		Value:   value,
		Code:    "INVALID_ENUM",
	}
}

func (vm *ValidationMiddleware) getValueType(value interface{}) string {
	if value == nil {
		return "null"
	}

	switch value.(type) {
	case bool:
		return "boolean"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "integer"
	case float32, float64:
		return "number"
	case string:
		return "string"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	default:
		return "unknown"
	}
}

func (vm *ValidationMiddleware) toFloat64(value interface{}) float64 {
	switch v := value.(type) {
	case int:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case float32:
		return float64(v)
	case float64:
		return v
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return 0
}

func (vm *ValidationMiddleware) valuesEqual(a, b interface{}) bool {
	return reflect.DeepEqual(a, b)
}

func defaultErrorHandler(ctx *lift.Context, errors []ValidationError) error {
	return fmt.Errorf("validation failed: %d errors", len(errors))
}

// Predefined validation rules

// EmailValidation validates email format
func EmailValidation() ValidationRule {
	return ValidationRule{
		Type:    "string",
		Pattern: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
		Message: "Invalid email format",
	}
}

// PhoneValidation validates phone number format
func PhoneValidation() ValidationRule {
	return ValidationRule{
		Type:    "string",
		Pattern: `^\+?[1-9]\d{1,14}$`,
		Message: "Invalid phone number format",
	}
}

// URLValidation validates URL format
func URLValidation() ValidationRule {
	return ValidationRule{
		Type:    "string",
		Pattern: `^https?://[^\s/$.?#].[^\s]*$`,
		Message: "Invalid URL format",
	}
}

// DateValidation validates ISO date format
func DateValidation() ValidationRule {
	return ValidationRule{
		Type: "string",
		Custom: func(value interface{}) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("date must be a string")
			}

			_, err := time.Parse(time.RFC3339, str)
			if err != nil {
				return fmt.Errorf("invalid date format, expected ISO 8601")
			}

			return nil
		},
		Message: "Invalid date format",
	}
}

// UUIDValidation validates UUID format
func UUIDValidation() ValidationRule {
	return ValidationRule{
		Type:    "string",
		Pattern: `^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`,
		Message: "Invalid UUID format",
	}
}

// CreditCardValidation validates credit card number
func CreditCardValidation() ValidationRule {
	return ValidationRule{
		Type: "string",
		Custom: func(value interface{}) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("credit card number must be a string")
			}

			// Remove spaces and dashes
			cleaned := strings.ReplaceAll(strings.ReplaceAll(str, " ", ""), "-", "")

			// Check length
			if len(cleaned) < 13 || len(cleaned) > 19 {
				return fmt.Errorf("credit card number must be 13-19 digits")
			}

			// Luhn algorithm validation
			if !luhnCheckNumber(cleaned) {
				return fmt.Errorf("invalid credit card number")
			}

			return nil
		},
		Message: "Invalid credit card number",
	}
}

// luhnCheckNumber validates a credit card number using Luhn algorithm
func luhnCheckNumber(number string) bool {
	sum := 0
	alternate := false

	for i := len(number) - 1; i >= 0; i-- {
		digit, err := strconv.Atoi(string(number[i]))
		if err != nil {
			return false
		}

		if alternate {
			digit *= 2
			if digit > 9 {
				digit = digit%10 + digit/10
			}
		}

		sum += digit
		alternate = !alternate
	}

	return sum%10 == 0
}

// Utility functions

// Validation creates a simple validation middleware
func Validation(schema *ValidationSchema) lift.Middleware {
	config := ValidationConfig{
		RequestSchema:   schema,
		ValidateRequest: true,
	}

	middleware := NewValidationMiddleware(config)
	return middleware.Validate()
}

// RequestValidation creates request-only validation middleware
func RequestValidation(schema *ValidationSchema) lift.Middleware {
	config := ValidationConfig{
		RequestSchema:   schema,
		ValidateRequest: true,
	}

	middleware := NewValidationMiddleware(config)
	return middleware.Validate()
}

// ResponseValidation creates response-only validation middleware
func ResponseValidation(schema *ValidationSchema) lift.Middleware {
	config := ValidationConfig{
		ResponseSchema:   schema,
		ValidateResponse: true,
	}

	middleware := NewValidationMiddleware(config)
	return middleware.Validate()
}

// StrictValidation creates strict validation middleware
func StrictValidation(requestSchema, responseSchema *ValidationSchema) lift.Middleware {
	config := ValidationConfig{
		RequestSchema:    requestSchema,
		ResponseSchema:   responseSchema,
		ValidateRequest:  true,
		ValidateResponse: true,
		StrictMode:       true,
	}

	middleware := NewValidationMiddleware(config)
	return middleware.Validate()
}

// Schema builders

// NewSchema creates a new validation schema
func NewSchema() *ValidationSchema {
	return &ValidationSchema{
		Type:       "object",
		Properties: make(map[string]ValidationRule),
		Required:   []string{},
		Rules:      []ValidationRule{},
	}
}

// AddProperty adds a property to the schema
func (s *ValidationSchema) AddProperty(name string, rule ValidationRule) *ValidationSchema {
	s.Properties[name] = rule
	return s
}

// AddRequired adds a required field
func (s *ValidationSchema) AddRequired(fields ...string) *ValidationSchema {
	s.Required = append(s.Required, fields...)
	return s
}

// AddRule adds a custom rule
func (s *ValidationSchema) AddRule(rule ValidationRule) *ValidationSchema {
	s.Rules = append(s.Rules, rule)
	return s
}

// SetCustom sets a custom validation function
func (s *ValidationSchema) SetCustom(fn func(interface{}) error) *ValidationSchema {
	s.Custom = fn
	return s
}
