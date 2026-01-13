package features

import (
	"errors"
	"testing"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/stretchr/testify/require"
)

func TestValidationMiddleware_validateRequest_InvalidJSON_UsesDefaultErrorHandler(t *testing.T) {
	vm := NewValidationMiddleware(ValidationConfig{
		RequestSchema:   NewSchema().AddProperty("name", ValidationRule{Type: "string", Required: true}),
		ValidateRequest: true,
		// ErrorHandler intentionally nil to exercise the default handler.
	})

	ctx := createCacheTestContext(httpGET, "/things", []byte("{"))
	ctx.Logger = &lift.NoOpLogger{}
	ctx.Metrics = &lift.NoOpMetrics{}

	err := vm.validateRequest(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "validation failed: 1 errors")
}

func TestValidationMiddleware_validateRequest_SchemaErrorsAndSuccess(t *testing.T) {
	schema := NewSchema().
		AddRequired("name").
		AddProperty("name", ValidationRule{Type: "string", Required: true})

	var got []ValidationError
	handlerErr := errors.New("handled")

	vm := NewValidationMiddleware(ValidationConfig{
		RequestSchema:   schema,
		ValidateRequest: true,
		ErrorHandler: func(_ *lift.Context, errs []ValidationError) error {
			got = errs
			return handlerErr
		},
	})

	t.Run("missing required field", func(t *testing.T) {
		ctx := createCacheTestContext(httpGET, "/things", []byte(`{"other":"x"}`))
		ctx.Logger = &lift.NoOpLogger{}
		ctx.Metrics = &lift.NoOpMetrics{}

		err := vm.validateRequest(ctx)
		require.ErrorIs(t, err, handlerErr)
		require.NotEmpty(t, got)
	})

	t.Run("valid request", func(t *testing.T) {
		got = nil
		ctx := createCacheTestContext(httpGET, "/things", []byte(`{"name":"ok"}`))
		ctx.Logger = &lift.NoOpLogger{}
		ctx.Metrics = &lift.NoOpMetrics{}

		require.NoError(t, vm.validateRequest(ctx))
		require.Empty(t, got)
	})
}

func TestValidationMiddleware_validateResponse_EmptyBody_StrictAndNonStrict(t *testing.T) {
	schema := NewSchema().AddProperty("message", ValidationRule{Type: "string", Required: true})

	t.Run("non-strict allows empty response body", func(t *testing.T) {
		vm := NewValidationMiddleware(ValidationConfig{
			ResponseSchema: schema,
			StrictMode:     false,
			ErrorHandler: func(_ *lift.Context, errs []ValidationError) error {
				return errors.New("should not be called")
			},
		})

		ctx := createCacheTestContext(httpGET, "/things", nil)
		ctx.Response.Body = nil
		require.NoError(t, vm.validateResponse(ctx))
	})

	t.Run("strict rejects empty response body", func(t *testing.T) {
		var got []ValidationError
		handlerErr := errors.New("handled")

		vm := NewValidationMiddleware(ValidationConfig{
			ResponseSchema: schema,
			StrictMode:     true,
			ErrorHandler: func(_ *lift.Context, errs []ValidationError) error {
				got = errs
				return handlerErr
			},
		})

		ctx := createCacheTestContext(httpGET, "/things", nil)
		ctx.Logger = &lift.NoOpLogger{}
		ctx.Metrics = &lift.NoOpMetrics{}
		ctx.Response.Body = nil

		err := vm.validateResponse(ctx)
		require.ErrorIs(t, err, handlerErr)
		require.Len(t, got, 1)
		require.Equal(t, "EMPTY_RESPONSE", got[0].Code)
		require.NotNil(t, ctx.Response)
	})
}

func TestValidationMiddleware_validateResponse_InvalidJSONAndSchemaFailure(t *testing.T) {
	schema := NewSchema().AddProperty("message", ValidationRule{Type: "string", Required: true}).AddRequired("message")

	t.Run("invalid JSON body returns INVALID_JSON", func(t *testing.T) {
		var got []ValidationError
		handlerErr := errors.New("handled")

		vm := NewValidationMiddleware(ValidationConfig{
			ResponseSchema: schema,
			StrictMode:     true,
			ErrorHandler: func(_ *lift.Context, errs []ValidationError) error {
				got = errs
				return handlerErr
			},
		})

		ctx := createCacheTestContext(httpGET, "/things", nil)
		ctx.Response.Body = "not-json"

		err := vm.validateResponse(ctx)
		require.ErrorIs(t, err, handlerErr)
		require.Len(t, got, 1)
		require.Equal(t, "INVALID_JSON", got[0].Code)
	})

	t.Run("buffered response uses body when captured is nil", func(t *testing.T) {
		var got []ValidationError
		handlerErr := errors.New("handled")

		vm := NewValidationMiddleware(ValidationConfig{
			ResponseSchema: schema,
			ErrorHandler: func(_ *lift.Context, errs []ValidationError) error {
				got = errs
				return handlerErr
			},
		})

		ctx := createCacheTestContext(httpGET, "/things", nil)
		ctx.EnableResponseBuffering()
		buffer := ctx.GetResponseBuffer()
		require.NotNil(t, buffer)
		buffer.SetBody([]byte(`{"status":"missing message"}`), nil)

		err := vm.validateResponse(ctx)
		require.ErrorIs(t, err, handlerErr)
		require.NotEmpty(t, got)
	})

	t.Run("schema validation failure returns required field error", func(t *testing.T) {
		var got []ValidationError
		handlerErr := errors.New("handled")

		vm := NewValidationMiddleware(ValidationConfig{
			ResponseSchema: schema,
			StrictMode:     true,
			ErrorHandler: func(_ *lift.Context, errs []ValidationError) error {
				got = errs
				return handlerErr
			},
		})

		ctx := createCacheTestContext(httpGET, "/things", nil)
		ctx.Response.Body = map[string]any{"status": "missing"}

		err := vm.validateResponse(ctx)
		require.ErrorIs(t, err, handlerErr)
		require.NotEmpty(t, got)
		require.Equal(t, "REQUIRED_FIELD", got[0].Code)
	})
}

func TestValidationMiddleware_normalizeResponseBody_Variants(t *testing.T) {
	vm := NewValidationMiddleware(ValidationConfig{})

	got, err := vm.normalizeResponseBody(map[string]any{"a": "b"})
	require.NoError(t, err)
	require.Equal(t, map[string]any{"a": "b"}, got)

	got, err = vm.normalizeResponseBody([]byte{})
	require.NoError(t, err)
	require.Equal(t, map[string]any{}, got)

	got, err = vm.normalizeResponseBody([]byte(`{"a":1}`))
	require.NoError(t, err)
	require.Equal(t, map[string]any{"a": float64(1)}, got)

	_, err = vm.normalizeResponseBody([]byte("{"))
	require.Error(t, err)

	got, err = vm.normalizeResponseBody("")
	require.NoError(t, err)
	require.Equal(t, map[string]any{}, got)

	got, err = vm.normalizeResponseBody(`{"a":2}`)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"a": float64(2)}, got)

	_, err = vm.normalizeResponseBody("{")
	require.Error(t, err)

	type wrapped struct {
		A string `json:"a"`
	}

	got, err = vm.normalizeResponseBody(wrapped{A: "ok"})
	require.NoError(t, err)
	require.Equal(t, map[string]any{"a": "ok"}, got)
}

func TestValidationInternals_validateDataAndFieldRules(t *testing.T) {
	vm := NewValidationMiddleware(ValidationConfig{})

	t.Run("root must be object", func(t *testing.T) {
		result := vm.validateData([]any{"x"}, NewSchema())
		require.False(t, result.Valid)
		require.Len(t, result.Errors, 1)
		require.Equal(t, "INVALID_TYPE", result.Errors[0].Code)
	})

	t.Run("required fields and property validations", func(t *testing.T) {
		schema := NewSchema().
			AddRequired("name").
			AddProperty("name", ValidationRule{Type: "string", Min: 2, Max: 4}).
			AddProperty("count", ValidationRule{Type: "number", Min: "2", Max: "3"}).
			AddProperty("tags", ValidationRule{Type: "array", Min: 2, Max: 3}).
			AddProperty("status", ValidationRule{Enum: []any{"a", "b"}}).
			AddProperty("conditional", ValidationRule{
				Type:       "string",
				Conditions: []ValidationCondition{{Field: "name", Operator: "eq", Value: "ok"}},
			}).
			AddProperty("custom", ValidationRule{
				Type:    "string",
				Message: "override message",
				Custom: func(_ any) error {
					return errors.New("original message")
				},
			}).
			AddRule(ValidationRule{
				Field: "root_rule",
				Custom: func(_ any) error {
					return errors.New("root rule failed")
				},
			}).
			SetCustom(func(_ any) error {
				return errors.New("schema custom failed")
			})

		result := vm.validateData(map[string]any{
			"name":        "a",
			"count":       float64(10),
			"tags":        []any{"x"},
			"status":      "c",
			"conditional": "ok",
			"custom":      "x",
		}, schema)

		require.False(t, result.Valid)
		require.NotEmpty(t, result.Errors)
	})
}

func TestValidationInternals_validateRangeAndPatternAndEnum(t *testing.T) {
	vm := NewValidationMiddleware(ValidationConfig{})

	t.Run("validateRange ignores unsupported types", func(t *testing.T) {
		require.Nil(t, vm.validateRange("field", map[string]any{"x": 1}, 1, 2))
	})

	t.Run("string range min/max", func(t *testing.T) {
		err := vm.validateRange("field", "a", 2, nil)
		require.NotNil(t, err)
		require.Equal(t, "MIN_LENGTH", err.Code)

		err = vm.validateRange("field", "abcdef", nil, 2)
		require.NotNil(t, err)
		require.Equal(t, "MAX_LENGTH", err.Code)

		require.Nil(t, vm.validateRange("field", "ab", 2, 3))
	})

	t.Run("numeric range min/max uses toFloat64", func(t *testing.T) {
		err := vm.validateRange("field", float64(1), "2", nil)
		require.NotNil(t, err)
		require.Equal(t, "MIN_VALUE", err.Code)

		err = vm.validateRange("field", int64(10), nil, "3")
		require.NotNil(t, err)
		require.Equal(t, "MAX_VALUE", err.Code)
	})

	t.Run("array range min/max", func(t *testing.T) {
		err := vm.validateRange("field", []any{"x"}, 2, nil)
		require.NotNil(t, err)
		require.Equal(t, "MIN_ITEMS", err.Code)

		err = vm.validateRange("field", []any{"a", "b", "c"}, nil, 2)
		require.NotNil(t, err)
		require.Equal(t, "MAX_ITEMS", err.Code)
	})

	t.Run("validatePattern handles invalid types and regex errors", func(t *testing.T) {
		err := vm.validatePattern("field", 123, `^a$`)
		require.NotNil(t, err)
		require.Equal(t, "INVALID_TYPE", err.Code)

		err = vm.validatePattern("field", "a", "[")
		require.NotNil(t, err)
		require.Equal(t, "INVALID_PATTERN", err.Code)

		err = vm.validatePattern("field", "b", `^a$`)
		require.NotNil(t, err)
		require.Equal(t, "PATTERN_MISMATCH", err.Code)

		require.Nil(t, vm.validatePattern("field", "a", `^a$`))
	})

	t.Run("validateEnum matches via DeepEqual", func(t *testing.T) {
		require.Nil(t, vm.validateEnum("field", map[string]any{"a": 1}, []any{map[string]any{"a": 1}}))

		err := vm.validateEnum("field", "c", []any{"a", "b"})
		require.NotNil(t, err)
		require.Equal(t, "INVALID_ENUM", err.Code)
	})

	t.Run("getValueType/toFloat64 edge cases", func(t *testing.T) {
		require.Equal(t, "null", vm.getValueType(nil))
		require.Equal(t, "unknown", vm.getValueType(struct{}{}))
		require.Equal(t, float64(0), vm.toFloat64("nope"))
	})
}

func TestPredefinedValidationRulesAndLuhn(t *testing.T) {
	vm := NewValidationMiddleware(ValidationConfig{})

	require.NotNil(t, EmailValidation())
	require.NotNil(t, PhoneValidation())
	require.NotNil(t, URLValidation())
	require.NotNil(t, UUIDValidation())

	dateRule := DateValidation()
	require.Error(t, dateRule.Custom("nope"))
	require.NoError(t, dateRule.Custom(time.Now().UTC().Format(time.RFC3339)))

	ccRule := CreditCardValidation()
	require.Error(t, ccRule.Custom(123))
	require.Error(t, ccRule.Custom("123"))
	require.Error(t, ccRule.Custom("4242 4242 4242 4241"))
	require.Error(t, ccRule.Custom("4242-4242-4242-424x"))
	require.NoError(t, ccRule.Custom("4242 4242 4242 4242"))

	require.True(t, luhnCheckNumber("4242424242424242"))
	require.False(t, luhnCheckNumber("4242424242424241"))
	require.False(t, luhnCheckNumber("4242x24242424242"))

	// Also cover validateType via a direct call.
	require.Nil(t, vm.validateType("field", true, "boolean"))
	require.NotNil(t, vm.validateType("field", true, "string"))
}

func TestValidationHelpers_RecordValidationFailure(t *testing.T) {
	vm := NewValidationMiddleware(ValidationConfig{})
	ctx := createCacheTestContext(httpGET, "/things", nil)
	ctx.Logger = &lift.NoOpLogger{}
	ctx.Metrics = &lift.NoOpMetrics{}

	vm.recordValidationFailure(ctx, "request", []ValidationError{})
	vm.recordValidationFailure(ctx, "request", []ValidationError{{Field: "f", Code: "X"}})
}

func TestValidationMiddleware_BuilderHelpersReturnMiddleware(t *testing.T) {
	requestSchema := NewSchema().AddProperty("name", ValidationRule{Type: "string", Required: true})
	responseSchema := NewSchema().AddProperty("ok", ValidationRule{Type: "boolean", Required: true})

	_ = Validation(requestSchema)
	_ = RequestValidation(requestSchema)
	_ = ResponseValidation(responseSchema)
	_ = StrictValidation(requestSchema, responseSchema)

	// Exercise middleware execution to cover response buffering enablement.
	mw := StrictValidation(requestSchema, responseSchema)
	next := lift.HandlerFunc(func(ctx *lift.Context) error {
		return ctx.JSON(map[string]any{"ok": true})
	})
	wrapped := mw(next)

	ctx := createCacheTestContext(httpGET, "/things", []byte(`{"name":"ok"}`))
	require.NoError(t, wrapped.Handle(ctx))
}
