package deployment

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/stretchr/testify/require"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/health"
)

type captureMetrics struct {
	counters   map[string]*captureCounter
	histograms map[string]*captureHistogram
	gauges     map[string]*captureGauge
	flushErr   error
}

func newCaptureMetrics() *captureMetrics {
	return &captureMetrics{
		counters:   map[string]*captureCounter{},
		histograms: map[string]*captureHistogram{},
		gauges:     map[string]*captureGauge{},
	}
}

func (m *captureMetrics) Counter(name string, _ ...map[string]string) lift.Counter {
	if c, ok := m.counters[name]; ok {
		return c
	}
	c := &captureCounter{}
	m.counters[name] = c
	return c
}

func (m *captureMetrics) Histogram(name string, _ ...map[string]string) lift.Histogram {
	if h, ok := m.histograms[name]; ok {
		return h
	}
	h := &captureHistogram{}
	m.histograms[name] = h
	return h
}

func (m *captureMetrics) Gauge(name string, _ ...map[string]string) lift.Gauge {
	if g, ok := m.gauges[name]; ok {
		return g
	}
	g := &captureGauge{}
	m.gauges[name] = g
	return g
}

func (m *captureMetrics) Flush() error {
	return m.flushErr
}

type captureCounter struct {
	inc int
}

func (c *captureCounter) Inc() {
	c.inc++
}

func (c *captureCounter) Add(_ float64) {
	c.inc++
}

type captureHistogram struct {
	observations int
}

func (h *captureHistogram) Observe(_ float64) {
	h.observations++
}

type captureGauge struct {
	setCalls int
	value    float64
}

func (g *captureGauge) Set(value float64) {
	g.setCalls++
	g.value = value
}

func (g *captureGauge) Inc() {
	g.setCalls++
	g.value++
}

func (g *captureGauge) Dec() {
	g.setCalls++
	g.value--
}

func (g *captureGauge) Add(v float64) {
	g.setCalls++
	g.value += v
}

type fakeHealthManager struct {
	overall health.HealthStatus
}

func (m *fakeHealthManager) RegisterChecker(string, health.HealthChecker) error { return nil }
func (m *fakeHealthManager) UnregisterChecker(string) error                     { return nil }
func (m *fakeHealthManager) CheckAll(context.Context) map[string]health.HealthStatus {
	return map[string]health.HealthStatus{}
}
func (m *fakeHealthManager) CheckComponent(context.Context, string) (health.HealthStatus, error) {
	return health.HealthStatus{}, nil
}
func (m *fakeHealthManager) OverallHealth(context.Context) health.HealthStatus { return m.overall }
func (m *fakeHealthManager) ListCheckers() []string                            { return nil }

func TestNewLambdaDeploymentRequiresApp(t *testing.T) {
	_, err := NewLambdaDeployment(nil, nil)
	require.Error(t, err)
}

func TestNewLambdaDeployment_DefaultsConfigWhenNil(t *testing.T) {
	deploy, err := NewLambdaDeployment(lift.New(), nil)
	require.NoError(t, err)
	require.NotNil(t, deploy.config)
}

func TestLambdaDeployment_EnrichContextWithLambdaContext(t *testing.T) {
	deploy, err := NewLambdaDeployment(lift.New(), DefaultDeploymentConfig())
	require.NoError(t, err)

	lc := &lambdacontext.LambdaContext{
		AwsRequestID:       "req-123",
		InvokedFunctionArn: "arn:aws:lambda:us-east-1:123:function:test",
	}
	ctx := lambdacontext.NewContext(context.Background(), lc)

	enriched := deploy.enrichContext(ctx, true)
	require.Equal(t, "req-123", enriched.Value(lambdaRequestIDKey))
	require.Equal(t, lc.InvokedFunctionArn, enriched.Value(lambdaFunctionNameKey))
	require.Equal(t, deploy.config.Environment, enriched.Value(deploymentEnvKey))
	require.Equal(t, true, enriched.Value(isColdStartKey))
	require.NotNil(t, enriched.Value(deploymentStartTimeKey))
}

func TestLambdaDeployment_DecodeLambdaEvent_EmptyInvalidAndNumbers(t *testing.T) {
	deploy, err := NewLambdaDeployment(lift.New(), DefaultDeploymentConfig())
	require.NoError(t, err)

	event, err := deploy.decodeLambdaEvent(nil)
	require.NoError(t, err)
	require.Equal(t, map[string]any{}, event)

	_, err = deploy.decodeLambdaEvent(json.RawMessage("{not-json"))
	require.Error(t, err)

	event, err = deploy.decodeLambdaEvent(json.RawMessage(`{"count":123}`))
	require.NoError(t, err)
	parsed := event.(map[string]any)
	require.IsType(t, json.Number(""), parsed["count"])
}

func TestLambdaDeployment_TranslateAndEncodeResponses(t *testing.T) {
	deploy, err := NewLambdaDeployment(lift.New(), DefaultDeploymentConfig())
	require.NoError(t, err)

	resp := lift.NewResponse()
	require.NoError(t, resp.JSON(map[string]string{"ok": "true"}))

	lambdaResp, err := deploy.translateResponse(resp)
	require.NoError(t, err)
	asMap := lambdaResp.(map[string]any)
	require.Equal(t, float64(200), asMap["statusCode"])

	raw, err := deploy.translateResponse(map[string]any{"hello": "world"})
	require.NoError(t, err)
	require.Equal(t, map[string]any{"hello": "world"}, raw)

	none, err := deploy.translateResponse(nil)
	require.NoError(t, err)
	require.Nil(t, none)

	other, err := deploy.translateResponse("ok")
	require.NoError(t, err)
	require.Equal(t, "ok", other)

	encoded, err := deploy.encodeLiftResponse(nil)
	require.NoError(t, err)
	require.Nil(t, encoded)

	_, err = deploy.encodeLiftResponse(&lift.Response{Body: make(chan int)})
	require.Error(t, err)
}

func TestLambdaDeployment_RecordMetricsAndShutdown(t *testing.T) {
	deploy, err := NewLambdaDeployment(lift.New(), DefaultDeploymentConfig())
	require.NoError(t, err)

	metrics := newCaptureMetrics()
	deploy.metrics = metrics
	deploy.startTime = time.Now().Add(-time.Second)

	deploy.recordMetrics(context.Background(), 50*time.Millisecond, nil, false)
	require.Equal(t, 1, metrics.histograms["lambda_request_duration"].observations)
	require.Equal(t, 1, metrics.counters["lambda_request_count"].inc)
	require.Equal(t, 1, metrics.gauges["lambda_memory_used_mb"].setCalls)
	require.Equal(t, 1, metrics.gauges["lambda_uptime_seconds"].setCalls)

	deploy.requestCount = 0
	deploy.recordMetrics(context.Background(), time.Millisecond, errors.New("boom"), true)
	require.Equal(t, 1, metrics.counters["lambda_error_count"].inc)
	require.Equal(t, 1, metrics.counters["lambda_cold_start_count"].inc)
	require.Equal(t, 1, metrics.gauges["lambda_error_rate"].setCalls)

	deploy.requestCount = 10
	deploy.recordMetrics(context.Background(), time.Millisecond, errors.New("boom"), false)
	require.GreaterOrEqual(t, metrics.gauges["lambda_error_rate"].setCalls, 2)

	deploy.metrics = &captureMetrics{flushErr: errors.New("flush failed")}
	require.Error(t, deploy.Shutdown(context.Background()))

	deploy.metrics = nil
	require.NoError(t, deploy.Shutdown(context.Background()))

	deploy.logWarning("test warning", errors.New("ignored"))
}

func TestLambdaDeployment_HealthCheckAndHealthCheckers(t *testing.T) {
	deploy, err := NewLambdaDeployment(lift.New(), DefaultDeploymentConfig())
	require.NoError(t, err)

	deploy.healthManager = &fakeHealthManager{
		overall: health.HealthStatus{
			Status: health.StatusHealthy,
			Details: map[string]any{
				"app": health.HealthStatus{Status: health.StatusHealthy, Message: "ok"},
				"raw": "skip",
			},
		},
	}

	status, err := deploy.HealthCheck(context.Background())
	require.NoError(t, err)
	require.Equal(t, health.StatusHealthy, status.Status)
	require.Contains(t, status.Checks, "app")
	require.NotContains(t, status.Checks, "raw")

	appChecker := &AppHealthChecker{app: nil}
	require.Equal(t, "app", appChecker.Name())
	require.Equal(t, health.StatusUnhealthy, appChecker.Check(context.Background()).Status)

	resourceChecker := &ResourceHealthChecker{
		maxGoroutines:            1,
		maxOpenFiles:             1,
		minDiskSpaceMB:           2048,
		checkDiskSpace:           true,
		checkNetworkConnectivity: true,
	}
	require.Equal(t, "resources", resourceChecker.Name())
	require.Equal(t, health.StatusUnhealthy, resourceChecker.Check(context.Background()).Status)

	memoryChecker := &MemoryHealthChecker{
		maxMemoryMB:   1,
		maxHeapMB:     1,
		enableGCStats: true,
	}
	buf := make([]byte, 2*1024*1024)
	runtime.KeepAlive(buf)
	runtime.GC()
	require.Equal(t, "memory", memoryChecker.Name())
	require.Equal(t, health.StatusUnhealthy, memoryChecker.Check(context.Background()).Status)
}

func TestDeploymentConfig_EnvironmentOverrides(t *testing.T) {
	t.Setenv("LIFT_ENVIRONMENT", "staging")
	t.Setenv("LIFT_METRICS_ENABLED", "0")
	t.Setenv("LIFT_TRACING_ENABLED", "1")

	cfg := DefaultDeploymentConfig()
	require.Equal(t, "staging", cfg.Environment)
	require.False(t, cfg.MetricsEnabled)
	require.True(t, cfg.TracingEnabled)

	os.Unsetenv("LIFT_ENVIRONMENT")
	require.Equal(t, "production", getEnv("LIFT_ENVIRONMENT", "production"))
	require.True(t, getEnvBool("MISSING_BOOL", true))
}
