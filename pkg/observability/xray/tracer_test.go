package xray

import (
	"context"
	"errors"
	"testing"
	"time"

	awsxray "github.com/aws/aws-xray-sdk-go/v2/xray"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/require"
)

func TestNewXRayTracer_Defaults(t *testing.T) {
	tracer := NewXRayTracer(XRayConfig{})
	require.Equal(t, "lift-service", tracer.config.ServiceName)
	require.Equal(t, 0.1, tracer.config.SamplingRate)
	require.NotNil(t, tracer.config.Annotations)
	require.NotNil(t, tracer.config.Metadata)
}

func TestFilterSensitiveHeaders(t *testing.T) {
	filtered := filterSensitiveHeaders(map[string]string{
		"authorization": "Bearer secret",
		"cookie":        "c=1",
		"x-api-key":     "k",
		"ok":            "v",
	})

	require.Equal(t, "[REDACTED]", filtered["authorization"])
	require.Equal(t, "[REDACTED]", filtered["cookie"])
	require.Equal(t, "[REDACTED]", filtered["x-api-key"])
	require.Equal(t, "v", filtered["ok"])
}

func TestTraceHelpers_NoSegmentAreNoOps(t *testing.T) {
	ctx := context.Background()

	ctx2, closeDynamo := TraceDynamoDBOperation(ctx, "GetItem", "tbl")
	closeDynamo()
	require.Equal(t, "", GetTraceID(ctx2))

	ctx3, closeHTTP := TraceHTTPCall(ctx, "GET", "https://example.com")
	closeHTTP(200, nil)
	require.Equal(t, "", GetTraceID(ctx3))

	ctx4, closeCustom := TraceCustomOperation(ctx, "op", map[string]any{"k": "v"})
	closeCustom(nil)
	require.Equal(t, "", GetTraceID(ctx4))
}

func TestTraceHelpers_WithSegment(t *testing.T) {
	ctx, seg := awsxray.BeginSegment(context.Background(), "test-seg")
	require.NotNil(t, seg)
	defer seg.Close(nil)

	require.NotEmpty(t, GetTraceID(ctx))
	require.NotEmpty(t, GetSegmentID(ctx))

	AddAnnotation(ctx, "k", "v")
	AddMetadata(ctx, "ns", "k", "v")
	SetError(ctx, errors.New("boom"))

	ctx2, closeDynamo := TraceDynamoDBOperation(ctx, "PutItem", "tbl")
	closeDynamo()

	ctx3, closeHTTP := TraceHTTPCall(ctx, "POST", "https://example.com")
	closeHTTP(500, errors.New("http failed"))

	_, closeCustom := TraceCustomOperation(ctx, "op", map[string]any{"k": "v"})
	closeCustom(errors.New("custom failed"))

	// Ensure returned contexts still have the segment.
	require.NotEmpty(t, GetTraceID(ctx2))
	require.NotEmpty(t, GetTraceID(ctx3))
}

func TestXRayMiddleware_SuccessAddsTraceHeaders(t *testing.T) {
	cfg := XRayConfig{
		ServiceName:       "svc",
		ServiceVersion:    "v1",
		Environment:       "lab",
		RecoverPanics:     true,
		EnableSubsegments: true,
		SamplingRate:      1.0,
		Annotations:       map[string]string{"a": "b"},
		Metadata:          map[string]string{"m": "n"},
	}

	mw := XRayMiddleware(cfg)

	req := lift.NewRequest(&adapters.Request{
		Method:      "GET",
		Path:        "/ok",
		Headers:     map[string]string{"authorization": "Bearer secret"},
		QueryParams: map[string]string{"q": "v"},
	})
	ctx := lift.NewContext(context.Background(), req)
	ctx.SetTenantID("tenant-1")
	ctx.SetUserID("user-1")
	ctx.SetRequestID("req-1")

	// Ensure response exists for middleware response annotation paths.
	ctx.Response = lift.NewResponse()
	ctx.Response.StatusCode = 204

	called := false
	handler := mw(lift.HandlerFunc(func(_ *lift.Context) error {
		called = true
		return nil
	}))

	require.NoError(t, handler.Handle(ctx))
	require.True(t, called)

	require.NotEmpty(t, ctx.Request.Headers["X-Trace-Id"])
	require.NotEmpty(t, ctx.Request.Headers["X-Span-Id"])
}

func TestXRayMiddleware_RecoversPanicsWhenEnabled(t *testing.T) {
	cfg := XRayConfig{
		ServiceName:   "svc",
		RecoverPanics: true,
		SamplingRate:  1.0,
	}

	mw := XRayMiddleware(cfg)
	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/panic"})
	ctx := lift.NewContext(context.Background(), req)
	ctx.Response = lift.NewResponse()

	err := mw(lift.HandlerFunc(func(_ *lift.Context) error {
		panic("boom")
	})).Handle(ctx)

	require.Error(t, err)
	require.Equal(t, 500, ctx.Response.StatusCode)
	require.Equal(t, lift.ContentTypeJSON, ctx.Response.Headers[lift.HeaderContentType])
	require.Equal(t, []byte(`{"error":"internal server error"}`), ctx.Response.Body)
}

func TestXRayMiddleware_RePanicsWhenDisabled(t *testing.T) {
	cfg := XRayConfig{
		ServiceName:   "svc",
		RecoverPanics: false,
		SamplingRate:  1.0,
	}

	mw := XRayMiddleware(cfg)
	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/panic"})
	ctx := lift.NewContext(context.Background(), req)
	ctx.Response = lift.NewResponse()

	require.Panics(t, func() {
		_ = mw(lift.HandlerFunc(func(_ *lift.Context) error {
			panic("boom")
		})).Handle(ctx)
	})
}

func TestXRayTracer_StandardMetadataHandlesNilRequest(t *testing.T) {
	tracer := NewXRayTracer(XRayConfig{ServiceName: "svc", SamplingRate: 1.0})
	ctx := &lift.Context{Request: nil}

	_, seg := awsxray.BeginSegment(context.Background(), "seg")
	require.NotNil(t, seg)
	defer seg.Close(nil)

	tracer.addStandardMetadata(seg, ctx) // should be a no-op for nil request
}

func TestAnnotationManager_ResponseDataBranches(t *testing.T) {
	cfg := XRayConfig{ServiceName: "svc", RecoverPanics: true, SamplingRate: 1.0}
	tracer := NewXRayTracer(cfg)
	am := newAnnotationManager(cfg, tracer)

	ctx := lift.NewContext(context.Background(), lift.NewRequest(&adapters.Request{Method: "GET", Path: "/"}))
	ctx.Response = lift.NewResponse()
	ctx.Response.StatusCode = 200

	_, seg := awsxray.BeginSegment(context.Background(), "seg")
	require.NotNil(t, seg)
	defer seg.Close(nil)

	am.addResponseData(seg, ctx, 10*time.Millisecond, nil)
	am.addResponseData(seg, ctx, 10*time.Millisecond, errors.New("boom"))
}
