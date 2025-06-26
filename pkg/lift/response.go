package lift

import (
	"encoding/json"
)

// Response represents a unified response structure for Lambda functions
type Response struct {
	StatusCode      int               `json:"statusCode"`
	Body            any       `json:"body"`
	Headers         map[string]string `json:"headers"`
	IsBase64Encoded bool              `json:"isBase64Encoded"`

	// Internal state
	written bool
}

// NewResponse creates a new Response with default values
func NewResponse() *Response {
	return &Response{
		StatusCode: 200,
		Headers:    make(map[string]string),
		written:    false,
	}
}

// Status sets the HTTP status code and returns the response for chaining
func (r *Response) Status(code int) *Response {
	r.StatusCode = code
	return r
}

// Header sets a response header
func (r *Response) Header(key, value string) *Response {
	if r.Headers == nil {
		r.Headers = make(map[string]string)
	}
	r.Headers[key] = value
	return r
}

// JSON sets the response body as JSON and marks the response as written
func (r *Response) JSON(data any) error {
	if r.written {
		return NewLiftError("RESPONSE_WRITTEN", "Response has already been written", 500)
	}

	r.Body = data
	r.Header("Content-Type", "application/json")
	r.written = true
	return nil
}

// Text sets the response body as plain text
func (r *Response) Text(text string) error {
	if r.written {
		return NewLiftError("RESPONSE_WRITTEN", "Response has already been written", 500)
	}

	r.Body = text
	r.Header("Content-Type", "text/plain")
	r.written = true
	return nil
}

// HTML sets the response body as HTML
func (r *Response) HTML(html string) error {
	if r.written {
		return NewLiftError("RESPONSE_WRITTEN", "Response has already been written", 500)
	}

	r.Body = html
	r.Header("Content-Type", "text/html")
	r.written = true
	return nil
}

// Binary sets the response body as binary data
func (r *Response) Binary(data []byte) error {
	if r.written {
		return NewLiftError("RESPONSE_WRITTEN", "Response has already been written", 500)
	}

	r.Body = data
	r.Header("Content-Type", "application/octet-stream")
	r.IsBase64Encoded = true
	r.written = true
	return nil
}

// IsWritten returns whether the response has been written
func (r *Response) IsWritten() bool {
	return r.written
}

// MarshalJSON implements custom JSON marshaling for Lambda response format
func (r *Response) MarshalJSON() ([]byte, error) {
	// Convert body to string if it's not already
	var bodyStr string
	if r.Body != nil {
		switch v := r.Body.(type) {
		case string:
			bodyStr = v
		case []byte:
			bodyStr = string(v)
		default:
			// Marshal non-string data as JSON
			jsonData, err := json.Marshal(v)
			if err != nil {
				return nil, NewLiftError("MARSHAL_ERROR", "Failed to marshal response body", 500).WithCause(err)
			}
			bodyStr = string(jsonData)
		}
	}

	// Create the Lambda response structure
	lambdaResponse := struct {
		StatusCode      int               `json:"statusCode"`
		Body            string            `json:"body"`
		Headers         map[string]string `json:"headers"`
		IsBase64Encoded bool              `json:"isBase64Encoded"`
	}{
		StatusCode:      r.StatusCode,
		Body:            bodyStr,
		Headers:         r.Headers,
		IsBase64Encoded: r.IsBase64Encoded,
	}

	return json.Marshal(lambdaResponse)
}
