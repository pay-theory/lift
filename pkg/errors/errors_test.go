package errors

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLiftError_StackTrace(t *testing.T) {
	t.Run("SystemError includes stack trace", func(t *testing.T) {
		err := SystemError("test error")
		
		if err.StackTrace == "" {
			t.Error("expected SystemError to include stack trace")
		}
		
		if !strings.Contains(err.StackTrace, "SystemError") {
			t.Error("expected stack trace to contain 'SystemError' function name")
		}
	})
	
	t.Run("NetworkError includes stack trace and correct code", func(t *testing.T) {
		err := NetworkError("network failure")
		
		if err.Code != "NETWORK_ERROR" {
			t.Errorf("expected code NETWORK_ERROR, got %s", err.Code)
		}
		
		if err.StatusCode != 500 {
			t.Errorf("expected status code 500, got %d", err.StatusCode)
		}
		
		if err.StackTrace == "" {
			t.Error("expected NetworkError to include stack trace")
		}
		
		if err.TraceID == "" {
			t.Error("expected NetworkError to include trace ID")
		}
	})
	
	t.Run("ProcessingError includes stack trace and correct code", func(t *testing.T) {
		err := ProcessingError("processing failed")
		
		if err.Code != "PROCESSING_ERROR" {
			t.Errorf("expected code PROCESSING_ERROR, got %s", err.Code)
		}
		
		if err.StatusCode != 500 {
			t.Errorf("expected status code 500, got %d", err.StatusCode)
		}
		
		if err.StackTrace == "" {
			t.Error("expected ProcessingError to include stack trace")
		}
		
		if err.TraceID == "" {
			t.Error("expected ProcessingError to include trace ID")
		}
	})
	
	t.Run("TokenizationFailure includes stack trace and correct code", func(t *testing.T) {
		err := TokenizationFailure("tokenization failed")
		
		if err.Code != "TOKENIZATION_FAILURE" {
			t.Errorf("expected code TOKENIZATION_FAILURE, got %s", err.Code)
		}
		
		if err.StatusCode != 500 {
			t.Errorf("expected status code 500, got %d", err.StatusCode)
		}
		
		if err.StackTrace == "" {
			t.Error("expected TokenizationFailure to include stack trace")
		}
		
		if err.TraceID == "" {
			t.Error("expected TokenizationFailure to include trace ID")
		}
	})
	
	t.Run("WithStackTrace adds stack trace to any error", func(t *testing.T) {
		err := ParameterError("field", "invalid")
		
		if err.StackTrace != "" {
			t.Error("expected no stack trace initially")
		}
		
		err.WithStackTrace()
		
		if err.StackTrace == "" {
			t.Error("expected WithStackTrace to add stack trace")
		}
		
		if !strings.Contains(err.StackTrace, "TestLiftError_StackTrace") {
			t.Error("expected stack trace to contain current test function name")
		}
	})
}

func TestLiftError_AppSyncFields(t *testing.T) {
	t.Run("WithErrorData adds data correctly", func(t *testing.T) {
		err := SystemError("test error")
		
		err.WithErrorData("field1", "value1").
			WithErrorData("field2", 123)
		
		if err.ErrorData == nil {
			t.Error("expected ErrorData to be initialized")
		}
		
		if err.ErrorData["field1"] != "value1" {
			t.Errorf("expected field1='value1', got '%v'", err.ErrorData["field1"])
		}
		
		if err.ErrorData["field2"] != 123 {
			t.Errorf("expected field2=123, got '%v'", err.ErrorData["field2"])
		}
	})
	
	t.Run("WithErrorInfo adds info correctly", func(t *testing.T) {
		err := ValidationError("validation failed")
		
		err.WithErrorInfo("type", "FIELD_REQUIRED").
			WithErrorInfo("severity", "HIGH")
		
		if err.ErrorInfo == nil {
			t.Error("expected ErrorInfo to be initialized")
		}
		
		if err.ErrorInfo["type"] != "FIELD_REQUIRED" {
			t.Errorf("expected type='FIELD_REQUIRED', got '%v'", err.ErrorInfo["type"])
		}
		
		if err.ErrorInfo["severity"] != "HIGH" {
			t.Errorf("expected severity='HIGH', got '%v'", err.ErrorInfo["severity"])
		}
	})
	
	t.Run("AppSync fields are omitted when empty", func(t *testing.T) {
		err := NetworkError("network issue")
		
		// Verify JSON omits empty fields
		data, _ := json.Marshal(err)
		jsonStr := string(data)
		
		if strings.Contains(jsonStr, "errorData") {
			t.Error("expected errorData to be omitted from JSON when empty")
		}
		
		if strings.Contains(jsonStr, "errorInfo") {
			t.Error("expected errorInfo to be omitted from JSON when empty")
		}
	})
	
	t.Run("AppSync fields work with method chaining", func(t *testing.T) {
		err := ProcessingError("processing failed").
			WithTraceID("trace-123").
			WithErrorData("retryCount", 3).
			WithErrorInfo("processor", "payment").
			WithDetails("amount", 100.50)
		
		if err.TraceID != "trace-123" {
			t.Errorf("expected trace ID 'trace-123', got '%s'", err.TraceID)
		}
		
		if err.ErrorData["retryCount"] != 3 {
			t.Errorf("expected retryCount=3, got '%v'", err.ErrorData["retryCount"])
		}
		
		if err.ErrorInfo["processor"] != "payment" {
			t.Errorf("expected processor='payment', got '%v'", err.ErrorInfo["processor"])
		}
		
		if err.Details["amount"] != 100.50 {
			t.Errorf("expected amount=100.50, got '%v'", err.Details["amount"])
		}
	})
}

func TestLiftError_LogError(t *testing.T) {
	t.Run("5xx errors have LogError true by default", func(t *testing.T) {
		testCases := []struct {
			name string
			err  *LiftError
		}{
			{"SystemError", SystemError("system failed")},
			{"NetworkError", NetworkError("network failed")},
			{"ProcessingError", ProcessingError("processing failed")},
			{"TokenizationFailure", TokenizationFailure("tokenization failed")},
		}
		
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				if !tc.err.LogError {
					t.Errorf("expected %s to have LogError=true", tc.name)
				}
			})
		}
	})
	
	t.Run("4xx errors have LogError false by default", func(t *testing.T) {
		testCases := []struct {
			name string
			err  *LiftError
		}{
			{"ValidationError", ValidationError("validation failed")},
			{"Unauthorized", Unauthorized("unauthorized")},
			{"AuthorizationError", AuthorizationError("forbidden")},
			{"NotFound", NotFound("not found")},
		}
		
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				if tc.err.LogError {
					t.Errorf("expected %s to have LogError=false", tc.name)
				}
			})
		}
	})
	
	t.Run("WithLogging method works correctly", func(t *testing.T) {
		// Test enabling logging on a 4xx error
		err := ValidationError("validation failed")
		err.WithLogging(true)
		
		if !err.LogError {
			t.Error("expected LogError to be true after WithLogging(true)")
		}
		
		// Test disabling logging on a 5xx error
		sysErr := SystemError("system failed")
		sysErr.WithLogging(false)
		
		if sysErr.LogError {
			t.Error("expected LogError to be false after WithLogging(false)")
		}
	})
	
	t.Run("LogError is excluded from JSON serialization", func(t *testing.T) {
		err := SystemError("test error").WithLogging(true)
		
		// Verify the field exists and is set
		if !err.LogError {
			t.Error("expected LogError to be true")
		}
		
		data, _ := json.Marshal(err)
		
		// The field is correctly excluded from JSON with json:"-" tag
		// We can see from the output that it doesn't contain LogError
		// Just verify the JSON is valid and contains expected fields
		var result map[string]any
		if err := json.Unmarshal(data, &result); err != nil {
			t.Errorf("failed to unmarshal JSON: %v", err)
		}
		
		// Verify expected fields are present
		if _, ok := result["code"]; !ok {
			t.Error("expected 'code' field in JSON")
		}
		
		// Verify LogError is NOT in the JSON
		if _, ok := result["LogError"]; ok {
			t.Error("LogError should not be in JSON output")
		}
		if _, ok := result["logError"]; ok {
			t.Error("logError should not be in JSON output")
		}
		if _, ok := result["log_error"]; ok {
			t.Error("log_error should not be in JSON output")
		}
	})
	
	t.Run("Method chaining with WithLogging", func(t *testing.T) {
		err := NetworkError("network issue").
			WithTraceID("trace-123").
			WithLogging(false).
			WithErrorData("retry", 3)
		
		if err.LogError {
			t.Error("expected LogError to be false")
		}
		
		if err.TraceID != "trace-123" {
			t.Errorf("expected trace ID 'trace-123', got '%s'", err.TraceID)
		}
		
		if err.ErrorData["retry"] != 3 {
			t.Errorf("expected retry=3, got '%v'", err.ErrorData["retry"])
		}
	})
}

func TestLiftError_WithTraceID(t *testing.T) {
	tests := []struct {
		name     string
		err      *LiftError
		traceID  string
		validate func(t *testing.T, err *LiftError)
	}{
		{
			name:    "adds trace ID to error",
			err:     ParameterError("test", "test error"),
			traceID: "test-trace-123",
			validate: func(t *testing.T, err *LiftError) {
				if err.TraceID != "test-trace-123" {
					t.Errorf("expected trace ID 'test-trace-123', got '%s'", err.TraceID)
				}
			},
		},
		{
			name:    "generates trace ID when empty string provided",
			err:     SystemError("server error"),
			traceID: "",
			validate: func(t *testing.T, err *LiftError) {
				if err.TraceID == "" {
					t.Error("expected generated trace ID, got empty string")
				}
				// Check it's a valid UUID format
				if len(err.TraceID) != 36 || !strings.Contains(err.TraceID, "-") {
					t.Errorf("expected UUID format, got '%s'", err.TraceID)
				}
			},
		},
		{
			name: "works with chained methods",
			err:  NotFound("resource not found"),
			validate: func(t *testing.T, err *LiftError) {
				// Test chaining methods
				err.WithRequestID("req-123").WithTraceID("trace-456").WithDetails("key", "value")
				
				if err.RequestID != "req-123" {
					t.Errorf("expected request ID 'req-123', got '%s'", err.RequestID)
				}
				if err.TraceID != "trace-456" {
					t.Errorf("expected trace ID 'trace-456', got '%s'", err.TraceID)
				}
				if err.Details["key"] != "value" {
					t.Errorf("expected detail key='value', got '%v'", err.Details["key"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.traceID != "" || tt.name == "generates trace ID when empty string provided" {
				tt.err.WithTraceID(tt.traceID)
			}
			tt.validate(t, tt.err)
		})
	}
}