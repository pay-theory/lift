package streamer

import (
	"errors"
	"testing"
)

func TestGoneError(t *testing.T) {
	err := GoneError{
		ConnectionID: "conn-123",
		Message:      "connection is gone",
	}

	if err.Error() != "connection is gone" {
		t.Errorf("expected message 'connection is gone', got %s", err.Error())
	}

	if err.HTTPStatusCode() != 410 {
		t.Errorf("expected status 410, got %d", err.HTTPStatusCode())
	}

	if err.ErrorCode() != "GoneException" {
		t.Errorf("expected code 'GoneException', got %s", err.ErrorCode())
	}

	if err.IsRetryable() {
		t.Error("GoneError should not be retryable")
	}

	if !errors.Is(err, ErrConnectionGone) {
		t.Error("GoneError should unwrap to ErrConnectionGone")
	}
}

func TestGoneErrorDefaultMessage(t *testing.T) {
	err := GoneError{
		ConnectionID: "conn-123",
	}

	expected := "connection conn-123 is gone"
	if err.Error() != expected {
		t.Errorf("expected message %q, got %q", expected, err.Error())
	}
}

func TestForbiddenError(t *testing.T) {
	err := ForbiddenError{
		ConnectionID: "conn-123",
		Message:      "access denied",
	}

	if err.Error() != "access denied" {
		t.Errorf("expected message 'access denied', got %s", err.Error())
	}

	if err.HTTPStatusCode() != 403 {
		t.Errorf("expected status 403, got %d", err.HTTPStatusCode())
	}

	if err.ErrorCode() != "ForbiddenException" {
		t.Errorf("expected code 'ForbiddenException', got %s", err.ErrorCode())
	}

	if err.IsRetryable() {
		t.Error("ForbiddenError should not be retryable")
	}

	if !errors.Is(err, ErrForbidden) {
		t.Error("ForbiddenError should unwrap to ErrForbidden")
	}
}

func TestPayloadTooLargeError(t *testing.T) {
	err := PayloadTooLargeError{
		ConnectionID: "conn-123",
		Message:      "payload exceeds limit",
		PayloadSize:  256000,
		MaxSize:      131072,
	}

	if err.Error() != "payload exceeds limit" {
		t.Errorf("expected message 'payload exceeds limit', got %s", err.Error())
	}

	if err.HTTPStatusCode() != 413 {
		t.Errorf("expected status 413, got %d", err.HTTPStatusCode())
	}

	if err.ErrorCode() != "PayloadTooLargeException" {
		t.Errorf("expected code 'PayloadTooLargeException', got %s", err.ErrorCode())
	}

	if err.IsRetryable() {
		t.Error("PayloadTooLargeError should not be retryable")
	}

	if !errors.Is(err, ErrPayloadTooLarge) {
		t.Error("PayloadTooLargeError should unwrap to ErrPayloadTooLarge")
	}
}

func TestPayloadTooLargeErrorDefaultMessage(t *testing.T) {
	err := PayloadTooLargeError{
		ConnectionID: "conn-123",
		PayloadSize:  256000,
		MaxSize:      131072,
	}

	expected := "payload size 256000 exceeds maximum 131072 for connection conn-123"
	if err.Error() != expected {
		t.Errorf("expected message %q, got %q", expected, err.Error())
	}
}

func TestThrottlingError(t *testing.T) {
	err := ThrottlingError{
		ConnectionID: "conn-123",
		Message:      "rate limited",
		RetryAfter:   30,
	}

	if err.Error() != "rate limited" {
		t.Errorf("expected message 'rate limited', got %s", err.Error())
	}

	if err.HTTPStatusCode() != 429 {
		t.Errorf("expected status 429, got %d", err.HTTPStatusCode())
	}

	if err.ErrorCode() != "ThrottlingException" {
		t.Errorf("expected code 'ThrottlingException', got %s", err.ErrorCode())
	}

	if !err.IsRetryable() {
		t.Error("ThrottlingError should be retryable")
	}

	if !errors.Is(err, ErrThrottled) {
		t.Error("ThrottlingError should unwrap to ErrThrottled")
	}
}

func TestThrottlingErrorDefaultMessage(t *testing.T) {
	err := ThrottlingError{
		ConnectionID: "conn-123",
		RetryAfter:   30,
	}

	expected := "throttled for connection conn-123, retry after 30 seconds"
	if err.Error() != expected {
		t.Errorf("expected message %q, got %q", expected, err.Error())
	}
}

func TestThrottlingErrorNoRetryAfter(t *testing.T) {
	err := ThrottlingError{
		ConnectionID: "conn-123",
	}

	expected := "throttled for connection conn-123"
	if err.Error() != expected {
		t.Errorf("expected message %q, got %q", expected, err.Error())
	}
}

func TestInternalServerError(t *testing.T) {
	err := InternalServerError{
		Message: "AWS service error",
	}

	if err.Error() != "AWS service error" {
		t.Errorf("expected message 'AWS service error', got %s", err.Error())
	}

	if err.HTTPStatusCode() != 500 {
		t.Errorf("expected status 500, got %d", err.HTTPStatusCode())
	}

	if err.ErrorCode() != "InternalServerError" {
		t.Errorf("expected code 'InternalServerError', got %s", err.ErrorCode())
	}

	if !err.IsRetryable() {
		t.Error("InternalServerError should be retryable")
	}

	if !errors.Is(err, ErrInternalServer) {
		t.Error("InternalServerError should unwrap to ErrInternalServer")
	}
}

func TestInternalServerErrorDefaultMessage(t *testing.T) {
	err := InternalServerError{}

	expected := "internal server error"
	if err.Error() != expected {
		t.Errorf("expected message %q, got %q", expected, err.Error())
	}
}

func TestAPIErrorInterface(t *testing.T) {
	// Test that all error types implement APIError
	var _ APIError = GoneError{}
	var _ APIError = ForbiddenError{}
	var _ APIError = PayloadTooLargeError{}
	var _ APIError = ThrottlingError{}
	var _ APIError = InternalServerError{}
}

func TestErrorUnwrapping(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		expectedErr error
	}{
		{
			name:        "GoneError unwraps to ErrConnectionGone",
			err:         GoneError{ConnectionID: "test"},
			expectedErr: ErrConnectionGone,
		},
		{
			name:        "ForbiddenError unwraps to ErrForbidden",
			err:         ForbiddenError{ConnectionID: "test"},
			expectedErr: ErrForbidden,
		},
		{
			name:        "PayloadTooLargeError unwraps to ErrPayloadTooLarge",
			err:         PayloadTooLargeError{ConnectionID: "test"},
			expectedErr: ErrPayloadTooLarge,
		},
		{
			name:        "ThrottlingError unwraps to ErrThrottled",
			err:         ThrottlingError{ConnectionID: "test"},
			expectedErr: ErrThrottled,
		},
		{
			name:        "InternalServerError unwraps to ErrInternalServer",
			err:         InternalServerError{},
			expectedErr: ErrInternalServer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !errors.Is(tt.err, tt.expectedErr) {
				t.Errorf("expected error to unwrap to %v", tt.expectedErr)
			}
		})
	}
}
