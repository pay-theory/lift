package lift

// Canonical error code constants used across Lift. These codes are surfaced in
// LiftError responses and should be preferred over ad‑hoc strings to preserve
// consistency across services and documentation.
const (
	ErrorCodeParameterError     = "PARAMETER_ERROR"
	ErrorCodeUnauthorized       = "UNAUTHORIZED"
	ErrorCodeAuthorizationError = "AUTHORIZATION_ERROR"
	ErrorCodeNotFound           = "NOT_FOUND"
	ErrorCodeValidationError    = "VALIDATION_ERROR"
	ErrorCodeSystemError        = "SYSTEM_ERROR"
	ErrorCodeNetworkError       = "NETWORK_ERROR"
	ErrorCodeProcessingError    = "PROCESSING_ERROR"
	ErrorCodeTokenizationError  = "TOKENIZATION_FAILURE" // #nosec G101 - constant error code, not a credential

	// Internal framework error codes
	ErrorCodeResponseWritten = "RESPONSE_WRITTEN"
	ErrorCodeMarshalError    = "MARSHAL_ERROR"
)
