package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/servicediscovery"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/require"
)

func TestServiceMeshAdapter_NewServiceMeshAdapter_SetsDefaults(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKID")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "SECRET")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	adapter, err := NewServiceMeshAdapter(ServiceMeshConfig{
		ServiceName: "svc",
		Namespace:   "ns",
		Region:      "us-east-1",
	})
	require.NoError(t, err)
	require.Equal(t, "/health", adapter.config.HealthCheckPath)
	require.Equal(t, "8080", adapter.config.Port)
	require.NotEmpty(t, adapter.instanceID)
	require.NotNil(t, adapter.sdClient)
	require.NotNil(t, adapter.appMeshClient)
}

func TestServiceMeshAdapter_RegisterAndDeregisterService(t *testing.T) {
	serviceName := "my-service"
	serviceID := "svc-123"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := r.Header.Get("X-Amz-Target")
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")

		switch {
		case strings.Contains(target, "ListServices"):
			_, _ = fmt.Fprintf(w, `{"Services":[{"Id":"%s","Name":"%s"}]}`, serviceID, serviceName)
		case strings.Contains(target, "RegisterInstance"):
			_, _ = fmt.Fprint(w, `{"OperationId":"op-123"}`)
		case strings.Contains(target, "DeregisterInstance"):
			_, _ = fmt.Fprint(w, `{"OperationId":"op-456"}`)
		default:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprint(w, `{"message":"unknown target"}`)
		}
	}))
	defer server.Close()

	cfg := aws.Config{
		Region:      "us-east-1",
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider("AKID", "SECRET", "")),
	}
	sdClient := servicediscovery.NewFromConfig(cfg, func(o *servicediscovery.Options) {
		o.BaseEndpoint = aws.String(server.URL)
	})

	t.Setenv("AWS_LAMBDA_FUNCTION_PRIVATE_IP", "10.0.0.1")
	t.Setenv("AWS_AVAILABILITY_ZONE", "us-east-1b")

	adapter := &ServiceMeshAdapter{
		sdClient:    sdClient,
		instanceID:  "inst-1",
		config:      ServiceMeshConfig{ServiceName: serviceName, Namespace: "ns-1", VirtualNode: "vn", MeshName: "mesh", Port: "8080"},
		loggedError: false,
	}

	require.NoError(t, adapter.RegisterService(context.Background()))
	require.Equal(t, serviceID, adapter.serviceID)
	require.NoError(t, adapter.DeregisterService(context.Background()))
}

func TestServiceMeshAdapter_RegisterService_ServiceNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := r.Header.Get("X-Amz-Target")
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		if strings.Contains(target, "ListServices") {
			_, _ = fmt.Fprint(w, `{"Services":[{"Id":"svc-123","Name":"other"}]}`)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	cfg := aws.Config{
		Region:      "us-east-1",
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider("AKID", "SECRET", "")),
	}
	sdClient := servicediscovery.NewFromConfig(cfg, func(o *servicediscovery.Options) {
		o.BaseEndpoint = aws.String(server.URL)
	})

	adapter := &ServiceMeshAdapter{
		sdClient:   sdClient,
		instanceID: "inst-1",
		config:     ServiceMeshConfig{ServiceName: "missing", Namespace: "ns-1"},
	}
	require.Error(t, adapter.RegisterService(context.Background()))
}

func TestServiceMeshAdapter_DeregisterService_NoOpWhenNotRegistered(t *testing.T) {
	adapter := &ServiceMeshAdapter{}
	require.NoError(t, adapter.DeregisterService(context.Background()))
}

func TestServiceMeshAdapter_Middleware_SetsHeadersAndHandlesHealthChecks(t *testing.T) {
	logger := &mockLogger{}

	adapter := &ServiceMeshAdapter{
		config: ServiceMeshConfig{
			MeshName:        "mesh",
			VirtualNode:     "vn",
			ServiceName:     "svc",
			HealthCheckPath: "/healthz",
		},
		registrationError: errors.New("registration failed"),
	}

	mw := adapter.Middleware()

	nextCalled := false
	next := lift.HandlerFunc(func(_ *lift.Context) error {
		nextCalled = true
		return nil
	})

	req := lift.NewRequest(&adapters.Request{
		Method: "GET",
		Path:   "/test",
		Headers: map[string]string{
			"X-Amzn-Trace-Id": "xray",
			"traceparent":     "tp",
		},
	})
	ctx := lift.NewContext(context.Background(), req)
	ctx.Logger = logger

	require.NoError(t, mw(next).Handle(ctx))
	require.True(t, nextCalled)
	require.Equal(t, "svc", ctx.Response.Headers["X-Service-Name"])
	require.Equal(t, "vn", ctx.Response.Headers["X-Virtual-Node"])
	require.Equal(t, "mesh", ctx.Response.Headers["X-Mesh-Name"])
	require.Equal(t, "xray", ctx.Get("trace_id"))
	require.Equal(t, "tp", ctx.Get("traceparent"))
	require.Equal(t, "mesh", ctx.Get("mesh_name"))
	require.Equal(t, "vn", ctx.Get("virtual_node"))
	require.Equal(t, "svc", ctx.Get("service_name"))

	nextCalled = false
	req2 := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/healthz"})
	ctx2 := lift.NewContext(context.Background(), req2)
	ctx2.Logger = logger

	require.NoError(t, mw(next).Handle(ctx2))
	require.False(t, nextCalled)
	require.Equal(t, 200, ctx2.Response.StatusCode)

	// Ensure registration error only logs once
	firstWarnCount := len(logger.logs)
	req3 := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/another"})
	ctx3 := lift.NewContext(context.Background(), req3)
	ctx3.Logger = logger
	require.NoError(t, mw(next).Handle(ctx3))
	require.Equal(t, firstWarnCount, len(logger.logs))
}

func TestServiceMeshAdapter_TraceHeaderHelpersAndEnvHelpers(t *testing.T) {
	adapter := &ServiceMeshAdapter{config: ServiceMeshConfig{ServiceName: "svc", MeshName: "mesh", VirtualNode: "vn"}}

	t.Setenv("AWS_LAMBDA_FUNCTION_PRIVATE_IP", "10.0.0.2")
	t.Setenv("AWS_AVAILABILITY_ZONE", "us-east-1c")

	require.Equal(t, "10.0.0.2", adapter.getEC2PrivateIP())
	require.Equal(t, "10.0.0.2", adapter.getPrivateIP())
	require.Equal(t, "us-east-1c", adapter.getAvailabilityZone())

	req := lift.NewRequest(&adapters.Request{
		Method: "GET",
		Path:   "/test",
		Headers: map[string]string{
			"X-Amzn-Trace-Id": "xray",
			"uber-trace-id":   "jaeger",
			"X-B3-TraceId":    "b3",
		},
	})
	ctx := lift.NewContext(context.Background(), req)
	traceHeaders := adapter.extractTraceHeaders(ctx)
	require.Equal(t, "xray", traceHeaders["trace_id"])
	require.Equal(t, "jaeger", traceHeaders["uber-trace-id"])
	require.Equal(t, "b3", traceHeaders["X-B3-TraceId"])

	propagate := PropagateTraceHeaders()
	require.NoError(t, propagate(lift.HandlerFunc(func(ctx *lift.Context) error {
		headers := ctx.Get("trace_headers").(map[string]string)
		require.Equal(t, "xray", headers["X-Amzn-Trace-Id"])
		require.Equal(t, "jaeger", headers["uber-trace-id"])
		return nil
	})).Handle(ctx))
}
