package validation

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// ValidationError represents a validation error
type ValidationError struct {
	Field   string      `json:"field"`
	Message string      `json:"message"`
	Tag     string      `json:"tag"`
	Value   interface{} `json:"value"`
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field '%s': %s", e.Field, e.Message)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "validation failed"
	}

	var messages []string
	for _, err := range e {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "; ")
}

// Validate validates a struct based on struct tags
func Validate(v interface{}) error {
	return validateStruct(v, "")
}

func validateStruct(v interface{}, prefix string) error {
	val := reflect.ValueOf(v)
	typ := reflect.TypeOf(v)

	// Handle pointers
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
		typ = typ.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil
	}

	var errors ValidationErrors

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Skip unexported fields
		if !field.CanInterface() {
			continue
		}

		fieldName := fieldType.Name
		if prefix != "" {
			fieldName = prefix + "." + fieldName
		}

		// Get validation tag
		validateTag := fieldType.Tag.Get("validate")
		if validateTag == "" {
			continue
		}

		// Parse validation rules
		rules := strings.Split(validateTag, ",")
		for _, rule := range rules {
			rule = strings.TrimSpace(rule)
			if rule == "" {
				continue
			}

			if err := validateField(field, fieldName, rule); err != nil {
				if validationErr, ok := err.(ValidationError); ok {
					errors = append(errors, validationErr)
				} else {
					errors = append(errors, ValidationError{
						Field:   fieldName,
						Message: err.Error(),
						Value:   field.Interface(),
					})
				}
			}
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

func validateField(field reflect.Value, fieldName, rule string) error {
	parts := strings.SplitN(rule, "=", 2)
	ruleName := parts[0]
	var ruleValue string
	if len(parts) > 1 {
		ruleValue = parts[1]
	}

	switch ruleName {
	case "required":
		return validateRequired(field, fieldName)
	case "min":
		return validateMin(field, fieldName, ruleValue)
	case "max":
		return validateMax(field, fieldName, ruleValue)
	case "email":
		return validateEmail(field, fieldName)
	case "oneof":
		return validateOneOf(field, fieldName, ruleValue)
	case "omitempty":
		// Skip validation if field is empty
		if isEmpty(field) {
			return nil
		}
	default:
		// Unknown rule, skip
		return nil
	}

	return nil
}

func validateRequired(field reflect.Value, fieldName string) error {
	if isEmpty(field) {
		return ValidationError{
			Field:   fieldName,
			Message: "field is required",
			Tag:     "required",
			Value:   field.Interface(),
		}
	}
	return nil
}

func validateMin(field reflect.Value, fieldName, ruleValue string) error {
	minVal, err := strconv.Atoi(ruleValue)
	if err != nil {
		return fmt.Errorf("invalid min rule value: %s", ruleValue)
	}

	switch field.Kind() {
	case reflect.String:
		if len(field.String()) < minVal {
			return ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("field must be at least %d characters", minVal),
				Tag:     "min",
				Value:   field.Interface(),
			}
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if field.Int() < int64(minVal) {
			return ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("field must be at least %d", minVal),
				Tag:     "min",
				Value:   field.Interface(),
			}
		}
	case reflect.Float32, reflect.Float64:
		if field.Float() < float64(minVal) {
			return ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("field must be at least %d", minVal),
				Tag:     "min",
				Value:   field.Interface(),
			}
		}
	}

	return nil
}

func validateMax(field reflect.Value, fieldName, ruleValue string) error {
	maxVal, err := strconv.Atoi(ruleValue)
	if err != nil {
		return fmt.Errorf("invalid max rule value: %s", ruleValue)
	}

	switch field.Kind() {
	case reflect.String:
		if len(field.String()) > maxVal {
			return ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("field must be at most %d characters", maxVal),
				Tag:     "max",
				Value:   field.Interface(),
			}
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if field.Int() > int64(maxVal) {
			return ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("field must be at most %d", maxVal),
				Tag:     "max",
				Value:   field.Interface(),
			}
		}
	case reflect.Float32, reflect.Float64:
		if field.Float() > float64(maxVal) {
			return ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("field must be at most %d", maxVal),
				Tag:     "max",
				Value:   field.Interface(),
			}
		}
	}

	return nil
}

func validateEmail(field reflect.Value, fieldName string) error {
	if field.Kind() != reflect.String {
		return nil
	}

	email := field.String()
	if email == "" {
		return nil // Let required handle empty values
	}

	// Simple email regex
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return ValidationError{
			Field:   fieldName,
			Message: "field must be a valid email address",
			Tag:     "email",
			Value:   field.Interface(),
		}
	}

	return nil
}

func validateOneOf(field reflect.Value, fieldName, ruleValue string) error {
	if field.Kind() != reflect.String {
		return nil
	}

	value := field.String()
	if value == "" {
		return nil // Let required handle empty values
	}

	validValues := strings.Split(ruleValue, " ")
	for _, validValue := range validValues {
		if value == validValue {
			return nil
		}
	}

	return ValidationError{
		Field:   fieldName,
		Message: fmt.Sprintf("field must be one of: %s", strings.Join(validValues, ", ")),
		Tag:     "oneof",
		Value:   field.Interface(),
	}
}

func isEmpty(field reflect.Value) bool {
	switch field.Kind() {
	case reflect.String:
		return field.String() == ""
	case reflect.Ptr, reflect.Interface:
		return field.IsNil()
	case reflect.Slice, reflect.Map, reflect.Array:
		return field.Len() == 0
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return field.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return field.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return field.Float() == 0
	case reflect.Bool:
		return !field.Bool()
	default:
		return false
	}
}
