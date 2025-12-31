package lift

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLiftError_BuildersAndHelpers(t *testing.T) {
	cause := errors.New("cause")
	err := NewLiftError("CODE", "msg", 400).
		WithDetail("field", "value").
		WithCause(cause).
		WithRequestID("req-1").
		WithTraceID("").
		WithStackTrace().
		WithErrorData(map[string]any{"k": "v"}).
		WithErrorInfo(map[string]any{"i": "j"}).
		WithLogging()

	if err.Code != "CODE" {
		t.Fatalf("expected code CODE, got %q", err.Code)
	}
	if err.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", err.StatusCode)
	}
	if err.Details["field"] != "value" {
		t.Fatalf("expected details.field value, got %v", err.Details["field"])
	}
	if !errors.Is(err, cause) {
		t.Fatalf("expected errors.Is to match cause")
	}
	if err.RequestID != "req-1" {
		t.Fatalf("expected request id req-1, got %q", err.RequestID)
	}
	if err.TraceID == "" {
		t.Fatalf("expected generated trace id")
	}
	if err.StackTrace == "" {
		t.Fatalf("expected stack trace")
	}
	if err.ErrorData["k"] != "v" {
		t.Fatalf("expected error data")
	}
	if err.ErrorInfo["i"] != "j" {
		t.Fatalf("expected error info")
	}
	if !err.LogError {
		t.Fatalf("expected LogError true")
	}
}

func TestLiftError_Constructors(t *testing.T) {
	if ParameterError("field", "bad").StatusCode != 400 {
		t.Fatalf("expected ParameterError status 400")
	}
	if Unauthorized("no").StatusCode != 401 {
		t.Fatalf("expected Unauthorized status 401")
	}
	if AuthorizationError("no").StatusCode != 403 {
		t.Fatalf("expected AuthorizationError status 403")
	}
	if NotFound("no").StatusCode != 404 {
		t.Fatalf("expected NotFound status 404")
	}
	if ValidationError("no").StatusCode != 422 {
		t.Fatalf("expected ValidationError status 422")
	}

	sys := SystemError("boom")
	if sys.StatusCode != 500 {
		t.Fatalf("expected SystemError status 500")
	}
	if sys.StackTrace == "" || sys.TraceID == "" {
		t.Fatalf("expected system error to have stack trace and trace id")
	}

	if NetworkError("no").StatusCode != 500 {
		t.Fatalf("expected NetworkError status 500")
	}
	if ProcessingError("no").StatusCode != 500 {
		t.Fatalf("expected ProcessingError status 500")
	}
	if TokenizationFailure("no").StatusCode != 500 {
		t.Fatalf("expected TokenizationFailure status 500")
	}
}

func TestLiftError_ErrorResponse(t *testing.T) {
	err := NewLiftError("CODE", "msg", 400).
		WithRequestID("req-1").
		WithDetails(map[string]any{"a": "b"})

	resp := ErrorResponse(err)
	if resp["request_id"] != "req-1" {
		t.Fatalf("expected request_id")
	}

	errorObj := resp["error"].(map[string]any)
	if errorObj["code"] != "CODE" || errorObj["message"] != "msg" {
		t.Fatalf("expected code/message in error response")
	}
	details := errorObj["details"].(map[string]any)
	if details["a"] != "b" {
		t.Fatalf("expected details in error response")
	}
}

func TestLiftError_ErrorStringIncludesCauseAndDetails(t *testing.T) {
	cause := errors.New("cause")

	err := NewLiftError("CODE", "msg", 400).
		WithCause(cause).
		WithDetails(map[string]any{"a": "b"})
	require.Contains(t, err.Error(), "[CODE] msg")
	require.Contains(t, err.Error(), "caused by: cause")
	require.Contains(t, err.Error(), "details: map[a:b]")

	noDetails := NewLiftError("CODE", "msg", 400)
	require.NotContains(t, noDetails.Error(), "caused by:")
	require.NotContains(t, noDetails.Error(), "details:")
}

func TestLiftError_WithTraceID_NonEmptyUsesProvidedValue(t *testing.T) {
	err := NewLiftError("CODE", "msg", 400).WithTraceID("trace-1")
	require.Equal(t, "trace-1", err.TraceID)
}
