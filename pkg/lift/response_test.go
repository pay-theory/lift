package lift

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResponseBinary_Base64EncodingAndHeaders(t *testing.T) {
	r := NewResponse()
	data := []byte{0x00, 0x01, 0x02, 0xFF}

	if err := r.Binary(data); err != nil {
		t.Fatalf("Binary() error = %v", err)
	}

	if !r.IsBase64Encoded {
		t.Fatalf("expected IsBase64Encoded = true")
	}

	// Marshal to Lambda response format
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("MarshalJSON error = %v", err)
	}

	var out struct {
		Headers         map[string]string `json:"headers"`
		Body            string            `json:"body"`
		StatusCode      int               `json:"statusCode"`
		IsBase64Encoded bool              `json:"isBase64Encoded"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("Unmarshal Lambda response error = %v", err)
	}

	if out.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", out.StatusCode)
	}
	if ct := out.Headers["Content-Type"]; ct != "application/octet-stream" {
		t.Fatalf("expected Content-Type application/octet-stream, got %s", ct)
	}
	if !out.IsBase64Encoded {
		t.Fatalf("expected isBase64Encoded true in marshaled output")
	}
	expected := base64.StdEncoding.EncodeToString(data)
	if out.Body != expected {
		t.Fatalf("expected base64 body %q, got %q", expected, out.Body)
	}
}

func TestResponse_WriteGuardsAndHeaders(t *testing.T) {
	r := NewResponse()
	require.False(t, r.IsWritten())

	require.NoError(t, r.JSON(map[string]string{"ok": "true"}))
	require.True(t, r.IsWritten())
	require.Equal(t, ContentTypeJSON, r.Headers[HeaderContentType])

	err := r.Text("nope")
	require.Error(t, err)
	require.Equal(t, ErrorCodeResponseWritten, err.(*LiftError).Code)

	err = r.HTML("<p>nope</p>")
	require.Error(t, err)
	require.Equal(t, ErrorCodeResponseWritten, err.(*LiftError).Code)
}

func TestResponse_MarshalJSON_BodyConversions(t *testing.T) {
	t.Run("string body stays string", func(t *testing.T) {
		r := NewResponse()
		r.Body = "hello"
		b, err := json.Marshal(r)
		require.NoError(t, err)

		var out map[string]any
		require.NoError(t, json.Unmarshal(b, &out))
		require.Equal(t, "hello", out["body"])
	})

	t.Run("raw bytes convert to string when not base64", func(t *testing.T) {
		r := NewResponse()
		r.Body = []byte("hi")
		b, err := json.Marshal(r)
		require.NoError(t, err)

		var out map[string]any
		require.NoError(t, json.Unmarshal(b, &out))
		require.Equal(t, "hi", out["body"])
	})

	t.Run("raw bytes are base64 encoded when IsBase64Encoded is true", func(t *testing.T) {
		r := NewResponse()
		r.IsBase64Encoded = true
		r.Body = []byte{0x00, 0x01, 0x02}

		b, err := json.Marshal(r)
		require.NoError(t, err)

		var out map[string]any
		require.NoError(t, json.Unmarshal(b, &out))
		require.Equal(t, base64.StdEncoding.EncodeToString([]byte{0x00, 0x01, 0x02}), out["body"])
	})

	t.Run("non-string data is marshaled to JSON string", func(t *testing.T) {
		r := NewResponse()
		r.Body = map[string]string{"a": "b"}
		b, err := json.Marshal(r)
		require.NoError(t, err)

		var out map[string]any
		require.NoError(t, json.Unmarshal(b, &out))
		require.Equal(t, `{"a":"b"}`, out["body"])
	})

	t.Run("marshal failure is wrapped in LiftError", func(t *testing.T) {
		r := NewResponse()
		r.Body = map[string]any{"ch": make(chan int)}

		_, err := json.Marshal(r)
		require.Error(t, err)
		var liftErr *LiftError
		require.ErrorAs(t, err, &liftErr)
		require.Equal(t, ErrorCodeMarshalError, liftErr.Code)
	})
}

func TestResponse_StatusAndHTML(t *testing.T) {
	r := NewResponse()
	r.Headers = nil

	r.Status(201)
	require.Equal(t, 201, r.StatusCode)

	require.NoError(t, r.HTML("<p>hi</p>"))
	require.Equal(t, ContentTypeHTML, r.Headers[HeaderContentType])
	require.Equal(t, "<p>hi</p>", r.Body)
}

func TestResponse_Binary_WhenAlreadyWrittenReturnsError(t *testing.T) {
	r := NewResponse()
	require.NoError(t, r.Text("written"))

	err := r.Binary([]byte("nope"))
	require.Error(t, err)
	require.Equal(t, ErrorCodeResponseWritten, err.(*LiftError).Code)
}
