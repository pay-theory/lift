package middleware

import (
	"context"
	"testing"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/require"
)

func TestServiceMeshAdapter_HealthCheckHandler(t *testing.T) {
	adapter := &ServiceMeshAdapter{
		config: ServiceMeshConfig{
			ServiceName: "svc",
			VirtualNode: "vn",
			MeshName:    "mesh",
		},
		instanceID: "inst-1",
	}

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/health"})
	ctx := lift.NewContext(context.Background(), req)

	require.NoError(t, adapter.HealthCheckHandler().Handle(ctx))
	require.Equal(t, 200, ctx.Response.StatusCode)
}

func TestServiceMeshAdapter_IPAndAZHelpers_CoverEnvBranches(t *testing.T) {
	adapter := &ServiceMeshAdapter{}

	t.Setenv("AWS_LAMBDA_FUNCTION_PRIVATE_IP", "10.0.0.1")
	require.Equal(t, "10.0.0.1", adapter.getEC2PrivateIP())

	t.Setenv("AWS_LAMBDA_FUNCTION_PRIVATE_IP", "")
	t.Setenv("ECS_TASK_PRIVATE_IP", "10.0.0.2")
	require.Equal(t, "10.0.0.2", adapter.getEC2PrivateIP())

	t.Setenv("ECS_TASK_PRIVATE_IP", "")
	require.Equal(t, "", adapter.getEC2PrivateIP())

	// getPrivateIP fallback (non-deterministic, but should not be empty)
	require.NotEmpty(t, adapter.getPrivateIP())

	t.Setenv("AWS_AVAILABILITY_ZONE", "us-east-1b")
	require.Equal(t, "us-east-1b", adapter.getAvailabilityZone())

	t.Setenv("AWS_AVAILABILITY_ZONE", "")
	t.Setenv("AWS_DEFAULT_AVAILABILITY_ZONE", "us-east-1c")
	require.Equal(t, "us-east-1c", adapter.getAvailabilityZone())

	t.Setenv("AWS_DEFAULT_AVAILABILITY_ZONE", "")
	t.Setenv("AWS_REGION", "us-west-2")
	require.Equal(t, "us-west-2a", adapter.getAvailabilityZone())

	t.Setenv("AWS_REGION", "")
	require.Equal(t, "us-east-1a", adapter.getAvailabilityZone())
}

func TestServiceMeshAdapter_ExtractTraceHeaders_CoversAllHeaderTypes(t *testing.T) {
	adapter := &ServiceMeshAdapter{}

	req := lift.NewRequest(&adapters.Request{
		Method: "GET",
		Path:   "/test",
		Headers: map[string]string{
			"X-Amzn-Trace-Id":     "xray",
			"traceparent":         "tp",
			"tracestate":          "ts",
			"uber-trace-id":       "jaeger",
			"X-B3-TraceId":        "b3t",
			"X-B3-SpanId":         "b3s",
			"X-B3-ParentSpanId":   "b3p",
			"X-B3-Sampled":        "1",
			"X-Unrelated-Header":  "x",
			"X-Another-Unrelated": "y",
		},
	})
	ctx := lift.NewContext(context.Background(), req)

	headers := adapter.extractTraceHeaders(ctx)
	require.Equal(t, "xray", headers["trace_id"])
	require.Equal(t, "tp", headers["traceparent"])
	require.Equal(t, "ts", headers["tracestate"])
	require.Equal(t, "jaeger", headers["uber-trace-id"])
	require.Equal(t, "b3t", headers["X-B3-TraceId"])
	require.Equal(t, "b3s", headers["X-B3-SpanId"])
	require.Equal(t, "b3p", headers["X-B3-ParentSpanId"])
	require.Equal(t, "1", headers["X-B3-Sampled"])
}
