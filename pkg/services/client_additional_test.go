package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/stretchr/testify/require"
)

type fixedHTTPClient struct {
	do func(req *http.Request) (*http.Response, error)
}

func (c fixedHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return c.do(req)
}

type metricsSpy struct {
	counters   int
	histograms int
	errors     int
}

type counterSpy struct{ parent *metricsSpy }

func (c counterSpy) Inc() { c.parent.counters++ }
func (c counterSpy) Add(_ float64) {
	c.parent.counters++
}

type histogramSpy struct{ parent *metricsSpy }

func (h histogramSpy) Observe(_ float64) { h.parent.histograms++ }

type gaugeSpy struct{}

func (g gaugeSpy) Set(_ float64) {}
func (g gaugeSpy) Inc()          {}
func (g gaugeSpy) Dec()          {}
func (g gaugeSpy) Add(_ float64) {}

func (m *metricsSpy) Counter(_ string, _ ...map[string]string) lift.Counter {
	return counterSpy{parent: m}
}
func (m *metricsSpy) Histogram(_ string, _ ...map[string]string) lift.Histogram {
	return histogramSpy{parent: m}
}
func (m *metricsSpy) Gauge(_ string, _ ...map[string]string) lift.Gauge {
	return gaugeSpy{}
}
func (m *metricsSpy) Flush() error { return nil }

var _ lift.MetricsCollector = (*metricsSpy)(nil)

type fakeCircuitBreaker struct {
	exec func(fn func() (any, error)) (any, error)
}

func (f fakeCircuitBreaker) Execute(fn func() (any, error)) (any, error) {
	return f.exec(fn)
}
func (fakeCircuitBreaker) GetState() CircuitBreakerState { return CircuitBreakerClosed }
func (fakeCircuitBreaker) GetStats() CircuitBreakerStats { return CircuitBreakerStats{} }

func TestNewServiceClient_DefaultsAndIDs(t *testing.T) {
	registry := &ServiceRegistry{}
	client := NewServiceClient(registry, ServiceClientConfig{})

	require.Equal(t, 30*time.Second, client.config.DefaultTimeout)
	require.Equal(t, 3, client.config.MaxRetries)
	require.Equal(t, 1*time.Second, client.config.RetryBackoff)
	require.Equal(t, "lift-service-client/1.0", client.config.UserAgent)
	require.NotNil(t, client.httpClient)
	require.NotNil(t, client.retryPolicy)

	require.True(t, strings.HasPrefix(client.generateRequestID(), "req_"))
	require.True(t, strings.HasPrefix(client.generateTraceID(), "trace_"))
	require.True(t, strings.HasPrefix(client.generateSpanID(), "span_"))
}

func TestServiceClient_executeWithRetry_CancelledContext(t *testing.T) {
	client := &ServiceClient{
		retryPolicy: &RetryPolicy{
			MaxRetries:        1,
			InitialBackoff:    100 * time.Millisecond,
			MaxBackoff:        100 * time.Millisecond,
			BackoffMultiplier: 2,
			RetryableErrors:   []string{"timeout"},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := client.executeWithRetry(ctx, func() error {
		return errors.New("timeout")
	})
	require.ErrorIs(t, err, context.Canceled)
}

func TestServiceClient_Call_CircuitBreakerBranches(t *testing.T) {
	discovery := newFakeDiscovery()
	discovery.setDiscoverResponse("user-service", []*ServiceInstance{
		{
			ID:          "inst-1",
			ServiceName: "user-service",
			Health:      HealthStatus{Status: "healthy"},
			Endpoint:    ServiceEndpoint{Protocol: "http", Host: "example.com", Port: 80},
		},
	})

	registry := NewServiceRegistry(RegistryConfig{EnableMetrics: false}, discovery, newFakeLoadBalancer(nil))

	t.Run("circuit breaker returns error", func(t *testing.T) {
		client := NewServiceClient(registry, ServiceClientConfig{
			EnableCircuitBreaker: true,
		})
		client.circuitBreaker = fakeCircuitBreaker{
			exec: func(func() (any, error)) (any, error) { return nil, errors.New("cb open") },
		}

		_, err := client.Call(context.Background(), &ServiceRequest{ServiceName: "user-service", Method: http.MethodGet, Path: "/"})
		require.Error(t, err)
	})

	t.Run("circuit breaker returns unexpected type", func(t *testing.T) {
		client := NewServiceClient(registry, ServiceClientConfig{
			EnableCircuitBreaker: true,
		})
		client.circuitBreaker = fakeCircuitBreaker{
			exec: func(func() (any, error)) (any, error) { return "not-a-response", nil },
		}

		_, err := client.Call(context.Background(), &ServiceRequest{ServiceName: "user-service", Method: http.MethodGet, Path: "/"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "unexpected circuit breaker result type")
	})

	t.Run("circuit breaker executes request successfully", func(t *testing.T) {
		metrics := &metricsSpy{}
		client := NewServiceClient(registry, ServiceClientConfig{
			EnableCircuitBreaker: true,
			EnableTracing:        true,
			EnableMetrics:        true,
		})
		client.metrics = metrics
		client.retryPolicy.MaxRetries = 0
		client.httpClient = fixedHTTPClient{
			do: func(req *http.Request) (*http.Response, error) {
				require.Equal(t, "application/json", req.Header.Get("Accept"))
				require.NotEmpty(t, req.Header.Get("X-Trace-ID"))
				require.NotEmpty(t, req.Header.Get("X-Span-ID"))
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader([]byte(`{"ok":true}`))),
					Header:     make(http.Header),
				}, nil
			},
		}
		client.circuitBreaker = fakeCircuitBreaker{
			exec: func(fn func() (any, error)) (any, error) { return fn() },
		}

		resp, err := client.Call(context.Background(), &ServiceRequest{
			ServiceName: "user-service",
			Method:      http.MethodGet,
			Path:        "/",
			Metadata:    map[string]any{"trace_id": "t1", "span_id": "s1"},
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Greater(t, metrics.counters, 0)
		require.Greater(t, metrics.histograms, 0)
	})
}

func TestServiceClientMiddleware_SetsClientInContext(t *testing.T) {
	client := &ServiceClient{}

	ctx := lift.NewContext(context.Background(), &lift.Request{})
	next := lift.HandlerFunc(func(ctx *lift.Context) error {
		require.Same(t, client, GetServiceClient(ctx))
		return nil
	})

	require.NoError(t, ServiceClientMiddleware(client)(next).Handle(ctx))
}

func TestUserServiceClient_UsesServiceClientAndParsesResponses(t *testing.T) {
	discovery := newFakeDiscovery()
	discovery.setDiscoverResponse("user-service", []*ServiceInstance{
		{
			ID:          "inst-1",
			ServiceName: "user-service",
			Health:      HealthStatus{Status: "healthy"},
			Endpoint:    ServiceEndpoint{Protocol: "http", Host: "example.com", Port: 80},
		},
	})
	registry := NewServiceRegistry(RegistryConfig{EnableMetrics: false}, discovery, newFakeLoadBalancer(nil))

	type responseSpec struct {
		status int
		body   string
	}
	responses := map[string]responseSpec{
		"GET /users/u1":      {status: 200, body: `{"id":"u1","email":"a@b.com","name":"A","tenant_id":"t1"}`},
		"GET /users/miss":    {status: 404, body: `{}`},
		"GET /users/bad":     {status: 200, body: `{not-json}`},
		"POST /users":        {status: 201, body: `{"id":"u2","email":"c@d.com","name":"C","tenant_id":"t1"}`},
		"PUT /users/u1":      {status: 200, body: `{"id":"u1","email":"e@f.com","name":"E","tenant_id":"t1"}`},
		"DELETE /users/u1":   {status: 204, body: ``},
		"DELETE /users/miss": {status: 404, body: ``},
		"GET /users":         {status: 200, body: `{"users":[{"id":"u1"}],"total":1,"limit":10,"offset":0}`},
	}

	client := NewServiceClient(registry, ServiceClientConfig{})
	client.retryPolicy.MaxRetries = 0
	client.httpClient = fixedHTTPClient{
		do: func(req *http.Request) (*http.Response, error) {
			key := req.Method + " " + req.URL.Path
			spec, ok := responses[key]
			if !ok {
				return nil, errors.New("no fixture for " + key)
			}
			return &http.Response{
				StatusCode: spec.status,
				Body:       io.NopCloser(bytes.NewReader([]byte(spec.body))),
				Header:     make(http.Header),
			}, nil
		},
	}

	userSvc := NewUserServiceClient(client, "t1")

	user, err := userSvc.GetUser(context.Background(), "u1")
	require.NoError(t, err)
	require.Equal(t, "u1", user.ID)

	_, err = userSvc.GetUser(context.Background(), "miss")
	require.Error(t, err)

	_, err = userSvc.GetUser(context.Background(), "bad")
	require.Error(t, err)

	created, err := userSvc.CreateUser(context.Background(), &CreateUserRequest{Email: "c@d.com"})
	require.NoError(t, err)
	require.Equal(t, "u2", created.ID)

	updated, err := userSvc.UpdateUser(context.Background(), "u1", &UpdateUserRequest{})
	require.NoError(t, err)
	require.Equal(t, "u1", updated.ID)

	require.NoError(t, userSvc.DeleteUser(context.Background(), "u1"))
	require.Error(t, userSvc.DeleteUser(context.Background(), "miss"))

	list, err := userSvc.ListUsers(context.Background(), &UserFilters{Limit: 10})
	require.NoError(t, err)
	require.Equal(t, 1, list.Total)

	// Exercise error branches for unexpected status codes.
	responses["POST /users"] = responseSpec{status: 500, body: `{}`}
	_, err = userSvc.CreateUser(context.Background(), &CreateUserRequest{})
	require.Error(t, err)

	responses["PUT /users/u1"] = responseSpec{status: 500, body: `{}`}
	_, err = userSvc.UpdateUser(context.Background(), "u1", &UpdateUserRequest{})
	require.Error(t, err)

	responses["DELETE /users/u1"] = responseSpec{status: 500, body: `{}`}
	require.Error(t, userSvc.DeleteUser(context.Background(), "u1"))

	responses["GET /users"] = responseSpec{status: 500, body: `{}`}
	_, err = userSvc.ListUsers(context.Background(), nil)
	require.Error(t, err)

	// Exercise unmarshal error on list users.
	responses["GET /users"] = responseSpec{status: 200, body: `{not-json}`}
	_, err = userSvc.ListUsers(context.Background(), nil)
	require.Error(t, err)
}

func TestServiceClient_executeRequest_MarshalFailure(t *testing.T) {
	client := &ServiceClient{
		retryPolicy: &RetryPolicy{MaxRetries: 0},
		config:      ServiceClientConfig{UserAgent: "lift-test"},
		httpClient: fixedHTTPClient{
			do: func(*http.Request) (*http.Response, error) {
				return nil, errors.New("should not be called")
			},
		},
	}

	instance := &ServiceInstance{
		ID:          "svc-instance",
		ServiceName: "test-service",
		Endpoint: ServiceEndpoint{
			Protocol: "http",
			Host:     "example.com",
			Port:     80,
		},
	}

	_, err := client.executeRequest(context.Background(), instance, &ServiceRequest{
		ServiceName: "test-service",
		Method:      http.MethodPost,
		Path:        "/",
		Body:        make(chan int),
	})
	require.Error(t, err)
}

func TestServiceClient_RetryableHelpers(t *testing.T) {
	client := &ServiceClient{
		retryPolicy: &RetryPolicy{
			RetryableStatusCodes: []int{500},
			RetryableErrors:      []string{"timeout"},
		},
	}

	require.True(t, client.isRetryableStatusCode(500))
	require.False(t, client.isRetryableStatusCode(200))
	require.True(t, client.isRetryableError(errors.New("timeout connecting")))
	require.False(t, client.isRetryableError(errors.New("nope")))

	// recordMetrics is a no-op unless enabled.
	client.recordMetrics("svc", "status", time.Millisecond, errors.New("boom"))

	metrics := &metricsSpy{}
	client.metrics = metrics
	client.config.EnableMetrics = true
	client.recordMetrics("svc", "status", time.Millisecond, errors.New("boom"))
	require.Greater(t, metrics.counters, 0)

	// Exercise gauge path without assertions.
	client.metrics.Gauge("service.instances.available", map[string]string{"service": "svc"}).Set(1)
}

func TestServiceClient_executeRequest_RetriesOnRetryableStatusCode(t *testing.T) {
	attempts := 0
	client := &ServiceClient{
		retryPolicy: &RetryPolicy{
			MaxRetries:           1,
			InitialBackoff:       1 * time.Nanosecond,
			MaxBackoff:           1 * time.Nanosecond,
			BackoffMultiplier:    1,
			RetryableStatusCodes: []int{500},
			RetryableErrors:      []string{"timeout"},
		},
		config: ServiceClientConfig{
			UserAgent:      "lift-test",
			EnableTracing:  true,
			DefaultTimeout: 1 * time.Second,
		},
		httpClient: fixedHTTPClient{
			do: func(req *http.Request) (*http.Response, error) {
				attempts++
				status := http.StatusInternalServerError
				if attempts > 1 {
					status = http.StatusOK
				}
				return &http.Response{
					StatusCode: status,
					Body:       io.NopCloser(bytes.NewReader([]byte(`{"ok":true}`))),
					Header:     make(http.Header),
				}, nil
			},
		},
	}

	instance := &ServiceInstance{
		ID:          "svc-instance",
		ServiceName: "test-service",
		Endpoint: ServiceEndpoint{
			Protocol: "http",
			Host:     "example.com",
			Port:     80,
		},
	}

	resp, err := client.executeRequest(context.Background(), instance, &ServiceRequest{
		ServiceName: "test-service",
		Method:      http.MethodGet,
		Path:        "/retryable-status",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, 2, attempts)

	// Ensure response body was consumed.
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(resp.Body, &decoded))
}
