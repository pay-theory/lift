package lift

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/require"
)

type testValidator struct {
	err error
}

func (v testValidator) Validate(any) error {
	return v.err
}

func TestContext_SQSRecords_DecodeAndCache(t *testing.T) {
	req := NewRequest(&adapters.Request{
		TriggerType: TriggerSQS,
		Records: []any{
			map[string]any{
				"messageId": "m1",
				"body":      "hello",
			},
		},
	})
	ctx := NewContext(context.Background(), req)

	records, err := ctx.SQSRecords()
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, "m1", records[0].MessageId)
	require.Equal(t, "hello", records[0].Body)

	ctx.Request.Records = []any{make(chan int)}
	cached, err := ctx.SQSRecords()
	require.NoError(t, err)
	require.Equal(t, records, cached)
}

func TestContext_SQSRecords_NoRecords(t *testing.T) {
	req := NewRequest(&adapters.Request{
		TriggerType: TriggerSQS,
	})
	ctx := NewContext(context.Background(), req)

	records, err := ctx.SQSRecords()
	require.NoError(t, err)
	require.Nil(t, records)
}

func TestContext_SQSRecords_DecodeError(t *testing.T) {
	req := NewRequest(&adapters.Request{
		TriggerType: TriggerSQS,
		Records:     []any{make(chan int)},
	})
	ctx := NewContext(context.Background(), req)

	records, err := ctx.SQSRecords()
	require.Error(t, err)
	require.Nil(t, records)
}

func TestContext_S3Records_DecodeAndCache(t *testing.T) {
	req := NewRequest(&adapters.Request{
		TriggerType: TriggerS3,
		Records: []any{
			map[string]any{
				"eventSource": "aws:s3",
				"s3": map[string]any{
					"bucket": map[string]any{"name": "my-bucket"},
					"object": map[string]any{"key": "uploads/file.txt"},
				},
			},
		},
	})
	ctx := NewContext(context.Background(), req)

	records, err := ctx.S3Records()
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, "my-bucket", records[0].S3.Bucket.Name)
	require.Equal(t, "uploads/file.txt", records[0].S3.Object.Key)

	ctx.Request.Records = []any{make(chan int)}
	cached, err := ctx.S3Records()
	require.NoError(t, err)
	require.Equal(t, records, cached)
}

func TestContext_decodeEventRecords_NoRecords(t *testing.T) {
	ctx := &Context{Request: NewRequest(nil)}

	var event events.SQSEvent
	decoded, err := ctx.decodeEventRecords(&event)
	require.NoError(t, err)
	require.False(t, decoded)
}

func TestContext_ParseRequest_Branches(t *testing.T) {
	t.Run("nil request returns EMPTY_BODY", func(t *testing.T) {
		ctx := &Context{Response: NewResponse()}
		var out map[string]any
		err := ctx.ParseRequest(&out)
		require.Error(t, err)
		require.Equal(t, "EMPTY_BODY", err.(*LiftError).Code)
	})

	t.Run("empty body with records decodes records", func(t *testing.T) {
		req := NewRequest(&adapters.Request{
			Body: []byte{},
			Records: []any{
				map[string]any{"foo": "bar"},
			},
		})
		ctx := NewContext(context.Background(), req)

		var out []map[string]any
		require.NoError(t, ctx.ParseRequest(&out))
		require.Len(t, out, 1)
		require.Equal(t, "bar", out[0]["foo"])
	})

	t.Run("empty body with raw event decodes raw event", func(t *testing.T) {
		req := NewRequest(&adapters.Request{
			Body:     []byte{},
			RawEvent: map[string]any{"hello": "world"},
		})
		ctx := NewContext(context.Background(), req)

		var out map[string]any
		require.NoError(t, ctx.ParseRequest(&out))
		require.Equal(t, "world", out["hello"])
	})

	t.Run("empty body with nothing returns EMPTY_BODY", func(t *testing.T) {
		req := NewRequest(&adapters.Request{Body: []byte{}})
		ctx := NewContext(context.Background(), req)

		var out map[string]any
		err := ctx.ParseRequest(&out)
		require.Error(t, err)
		require.Equal(t, "EMPTY_BODY", err.(*LiftError).Code)
	})

	t.Run("invalid json returns INVALID_JSON", func(t *testing.T) {
		req := NewRequest(&adapters.Request{Body: []byte("{not-json}")})
		ctx := NewContext(context.Background(), req)

		var out map[string]any
		err := ctx.ParseRequest(&out)
		require.Error(t, err)
		require.Equal(t, "INVALID_JSON", err.(*LiftError).Code)
	})

	t.Run("validator failure returns VALIDATION_ERROR and wraps cause", func(t *testing.T) {
		req := NewRequest(&adapters.Request{Body: []byte(`{"name":"bob"}`)})
		ctx := NewContext(context.Background(), req)

		validatorErr := errors.New("bad")
		ctx.SetValidator(testValidator{err: validatorErr})

		var out map[string]any
		err := ctx.ParseRequest(&out)
		require.Error(t, err)
		require.Equal(t, "VALIDATION_ERROR", err.(*LiftError).Code)
		require.ErrorIs(t, err, validatorErr)
	})

	t.Run("record marshalling failure returns INVALID_JSON", func(t *testing.T) {
		req := NewRequest(&adapters.Request{
			Body:    []byte{},
			Records: []any{make(chan int)},
		})
		ctx := NewContext(context.Background(), req)

		var out []map[string]any
		err := ctx.ParseRequest(&out)
		require.Error(t, err)
		require.Equal(t, "INVALID_JSON", err.(*LiftError).Code)
	})

	t.Run("raw event marshalling failure returns INVALID_JSON", func(t *testing.T) {
		req := NewRequest(&adapters.Request{
			Body:     []byte{},
			RawEvent: make(chan int),
		})
		ctx := NewContext(context.Background(), req)

		var out map[string]any
		err := ctx.ParseRequest(&out)
		require.Error(t, err)
		require.Equal(t, "INVALID_JSON", err.(*LiftError).Code)
	})
}

func TestContext_ResponseBuffering_CapturesJSON(t *testing.T) {
	req := NewRequest(&adapters.Request{
		Method:      "GET",
		Path:        "/",
		Headers:     map[string]string{},
		QueryParams: map[string]string{},
		TriggerType: TriggerAPIGateway,
	})
	ctx := NewContext(context.Background(), req)

	require.Nil(t, ctx.GetResponseBuffer())

	ctx.EnableResponseBuffering()
	require.NotNil(t, ctx.GetResponseBuffer())

	payload := map[string]string{"ok": "true"}
	require.NoError(t, ctx.Status(201).JSON(payload))

	body, status, headers, captured := ctx.GetResponseBuffer().Get()
	require.Equal(t, 201, status)
	require.Equal(t, payload, body)
	require.Equal(t, payload, captured)
	require.Equal(t, ContentTypeJSON, headers[HeaderContentType])

	require.NoError(t, ctx.FlushResponse())
}

func TestContext_WithTimeout(t *testing.T) {
	req := NewRequest(&adapters.Request{})
	ctx := NewContext(context.Background(), req)

	value, err := ctx.WithTimeout(100*time.Millisecond, func() (any, error) {
		return "ok", nil
	})
	require.NoError(t, err)
	require.Equal(t, "ok", value)

	value, err = ctx.WithTimeout(10*time.Millisecond, func() (any, error) {
		time.Sleep(50 * time.Millisecond)
		return "late", nil
	})
	require.Error(t, err)
	require.Nil(t, value)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestContext_ClaimsAndRequestIDHelpers(t *testing.T) {
	req := NewRequest(&adapters.Request{})
	ctx := NewContext(context.Background(), req)

	require.False(t, ctx.IsAuthenticated())
	require.Nil(t, ctx.GetClaim("missing"))
	require.Empty(t, ctx.GetRequestID())

	ctx.Set("request_id", "req-1")
	require.Equal(t, "req-1", ctx.GetRequestID())

	ctx.SetRequestID("req-2")
	require.Equal(t, "req-2", ctx.GetRequestID())

	claims := map[string]any{
		"sub":        "user-sub",
		"tenant_id":  "tenant-1",
		"account_id": "account-1",
	}
	ctx.SetClaims(claims)
	require.True(t, ctx.IsAuthenticated())
	require.Equal(t, "user-sub", ctx.UserID())
	require.Equal(t, "tenant-1", ctx.TenantID())
	require.Equal(t, "account-1", ctx.AccountID())

	claims["sub"] = "mutated"
	require.Equal(t, "user-sub", ctx.GetClaim("sub"))
}

func TestContext_ResponseConvenienceMethodsAndAliases(t *testing.T) {
	t.Run("aliases return expected values", func(t *testing.T) {
		req := NewRequest(&adapters.Request{
			Method:      "GET",
			Path:        "/",
			Headers:     map[string]string{"X-Test": "value"},
			QueryParams: map[string]string{"q": "search"},
		})
		ctx := NewContext(context.Background(), req)
		ctx.SetParam("id", "123")

		require.Equal(t, "123", ctx.PathParam("id"))
		require.Equal(t, "search", ctx.QueryParam("q"))

		ctx.SetUserID("user-1")
		require.Equal(t, "user-1", ctx.GetUserID())
	})

	t.Run("HTML captures response when buffering enabled", func(t *testing.T) {
		req := NewRequest(&adapters.Request{Method: "GET", Path: "/"})
		ctx := NewContext(context.Background(), req)
		ctx.EnableResponseBuffering()

		require.NoError(t, ctx.HTML("<p>hi</p>"))
		_, status, headers, _ := ctx.GetResponseBuffer().Get()
		require.Equal(t, 200, status)
		require.Equal(t, ContentTypeHTML, headers[HeaderContentType])
	})

	t.Run("Created sets status and captures JSON", func(t *testing.T) {
		req := NewRequest(&adapters.Request{Method: "POST", Path: "/"})
		ctx := NewContext(context.Background(), req)
		ctx.EnableResponseBuffering()

		require.NoError(t, ctx.Created(map[string]string{"id": "123"}))
		_, status, _, _ := ctx.GetResponseBuffer().Get()
		require.Equal(t, 201, status)
	})

	t.Run("BadRequest returns LiftError when err is nil", func(t *testing.T) {
		req := NewRequest(&adapters.Request{Method: "POST", Path: "/"})
		ctx := NewContext(context.Background(), req)
		ctx.EnableResponseBuffering()

		err := ctx.BadRequest("nope", nil)
		require.Error(t, err)
		require.Equal(t, "BAD_REQUEST", err.(*LiftError).Code)
	})

	t.Run("Forbidden returns original error when provided", func(t *testing.T) {
		req := NewRequest(&adapters.Request{Method: "POST", Path: "/"})
		ctx := NewContext(context.Background(), req)

		orig := errors.New("stop")
		err := ctx.Forbidden("nope", orig)
		require.ErrorIs(t, err, orig)
	})

	t.Run("SystemError returns LiftError when err is nil", func(t *testing.T) {
		req := NewRequest(&adapters.Request{Method: "POST", Path: "/"})
		ctx := NewContext(context.Background(), req)

		err := ctx.SystemError("boom", nil)
		require.Error(t, err)
		require.Equal(t, "SYSTEM_ERROR", err.(*LiftError).Code)
	})

	t.Run("NotFound writes JSON and returns response write error if already written", func(t *testing.T) {
		req := NewRequest(&adapters.Request{Method: "GET", Path: "/"})
		ctx := NewContext(context.Background(), req)

		require.NoError(t, ctx.JSON(map[string]string{"ok": "true"}))
		err := ctx.NotFound("missing", nil)
		require.Error(t, err)
		require.Equal(t, ErrorCodeResponseWritten, err.(*LiftError).Code)
	})
}
