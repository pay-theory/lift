package security

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDashboardCache struct {
	mu   sync.Mutex
	data map[string]any
	ttl  map[string]time.Duration
}

func newFakeDashboardCache() *fakeDashboardCache {
	return &fakeDashboardCache{
		data: make(map[string]any),
		ttl:  make(map[string]time.Duration),
	}
}

func (c *fakeDashboardCache) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.data[key]
	return v, ok
}

func (c *fakeDashboardCache) Set(key string, value any, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
	c.ttl[key] = ttl
}

func (c *fakeDashboardCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
	delete(c.ttl, key)
}

func (c *fakeDashboardCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string]any)
	c.ttl = make(map[string]time.Duration)
}

type fakeMetricsEngine struct {
	compliance     *ComplianceMetrics
	complianceErr  error
	risk           *RiskMetrics
	riskErr        error
	audit          *AuditMetrics
	auditErr       error
	performance    *PerformanceMetrics
	performanceErr error
	custom         []*CustomMetric
	customErr      error

	complianceCalls  int
	riskCalls        int
	auditCalls       int
	performanceCalls int
	customCalls      int

	lastCustomQueries []CustomMetricQuery
}

func (f *fakeMetricsEngine) CalculateComplianceMetrics(context.Context, TimeRange) (*ComplianceMetrics, error) {
	f.complianceCalls++
	return f.compliance, f.complianceErr
}

func (f *fakeMetricsEngine) CalculateRiskMetrics(context.Context, TimeRange) (*RiskMetrics, error) {
	f.riskCalls++
	return f.risk, f.riskErr
}

func (f *fakeMetricsEngine) CalculateAuditMetrics(context.Context, TimeRange) (*AuditMetrics, error) {
	f.auditCalls++
	return f.audit, f.auditErr
}

func (f *fakeMetricsEngine) CalculatePerformanceMetrics(context.Context, TimeRange) (*PerformanceMetrics, error) {
	f.performanceCalls++
	return f.performance, f.performanceErr
}

func (f *fakeMetricsEngine) CalculateCustomMetrics(context.Context, []CustomMetricQuery) ([]*CustomMetric, error) {
	f.customCalls++
	return f.custom, f.customErr
}

type fakeDashboardAlertManager struct {
	active       []*DashboardAlert
	activeErr    error
	thresholds   []*DashboardAlert
	thresholdErr error

	activeCalls     int
	thresholdsCalls int
}

func (f *fakeDashboardAlertManager) CheckThresholds(context.Context, *DashboardMetrics) ([]*DashboardAlert, error) {
	f.thresholdsCalls++
	return f.thresholds, f.thresholdErr
}

func (f *fakeDashboardAlertManager) SendAlert(context.Context, *DashboardAlert) error {
	return nil
}

func (f *fakeDashboardAlertManager) GetActiveAlerts(context.Context) ([]*DashboardAlert, error) {
	f.activeCalls++
	return f.active, f.activeErr
}

func (f *fakeDashboardAlertManager) AcknowledgeAlert(context.Context, string, string) error {
	return nil
}

func TestComplianceDashboard_StartStopAndCaching(t *testing.T) {
	t.Parallel()

	timeRange := TimeRange{Start: time.Unix(100, 0), End: time.Unix(200, 0)}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	engine := &fakeMetricsEngine{
		compliance:  &ComplianceMetrics{OverallScore: 1},
		risk:        &RiskMetrics{OverallRiskScore: 99},
		audit:       &AuditMetrics{FailureRate: 0.9},
		performance: &PerformanceMetrics{Throughput: 123.0},
		custom: []*CustomMetric{
			{ID: "m1", Value: 1},
		},
	}

	alerts := &fakeDashboardAlertManager{
		active: []*DashboardAlert{
			{ID: "a1", Severity: riskLevelCritical},
		},
		thresholds: []*DashboardAlert{
			{ID: "a2", Severity: "low"},
		},
	}

	cache := newFakeDashboardCache()

	dashboard := NewComplianceDashboard(DashboardConfig{
		Enabled:              true,
		CacheEnabled:         true,
		RealTimeUpdates:      true,
		ExportEnabled:        true,
		CustomMetricsEnabled: true,
		CacheTTL:             time.Minute,
		RefreshInterval:      time.Millisecond,
	})
	dashboard.SetMetricsEngine(engine)
	dashboard.SetAlertManager(alerts)
	dashboard.SetCache(cache)

	require.NoError(t, dashboard.Start(ctx))
	require.Error(t, dashboard.Start(ctx), "dashboard already running")
	require.NoError(t, dashboard.Stop())
	require.NoError(t, dashboard.Stop(), "stopping when not running should be no-op")

	metrics, err := dashboard.GetDashboardMetrics(context.Background(), timeRange)
	require.NoError(t, err)
	require.NotNil(t, metrics)
	require.NotNil(t, metrics.ComplianceMetrics)
	require.NotNil(t, metrics.RiskMetrics)
	require.NotNil(t, metrics.AuditMetrics)
	require.NotNil(t, metrics.PerformanceMetrics)
	require.NotNil(t, metrics.Summary)
	assert.Len(t, metrics.CustomMetrics, 1)
	assert.Len(t, metrics.Alerts, 2)
	assert.Equal(t, 2, metrics.Summary.ActiveAlerts)
	assert.Equal(t, 1, metrics.Summary.CriticalIssues)
	assert.NotEmpty(t, metrics.Summary.Recommendations)

	// Second call should hit cache and avoid recalculating metrics.
	metrics2, err := dashboard.GetDashboardMetrics(context.Background(), timeRange)
	require.NoError(t, err)
	assert.Same(t, metrics, metrics2)
	assert.Equal(t, 1, engine.complianceCalls)
	assert.Equal(t, 1, engine.riskCalls)
	assert.Equal(t, 1, engine.auditCalls)
	assert.Equal(t, 1, engine.performanceCalls)
	assert.Equal(t, 1, engine.customCalls)
	assert.Equal(t, 1, alerts.activeCalls)
	assert.Equal(t, 1, alerts.thresholdsCalls)
}

func TestComplianceDashboard_MetricsCollectionErrorsAreIgnored(t *testing.T) {
	t.Parallel()

	engine := &fakeMetricsEngine{
		complianceErr: errors.New("fail"),
		risk:          &RiskMetrics{OverallRiskScore: 10},
		auditErr:      errors.New("fail"),
	}
	alerts := &fakeDashboardAlertManager{activeErr: errors.New("fail"), thresholdErr: errors.New("fail")}

	dashboard := NewComplianceDashboard(DashboardConfig{
		Enabled:              true,
		CacheEnabled:         false,
		ExportEnabled:        false,
		CustomMetricsEnabled: true,
	})
	dashboard.SetMetricsEngine(engine)
	dashboard.SetAlertManager(alerts)

	metrics, err := dashboard.GetDashboardMetrics(context.Background(), TimeRange{Start: time.Unix(1, 0), End: time.Unix(2, 0)})
	require.NoError(t, err)
	assert.Nil(t, metrics.ComplianceMetrics)
	assert.NotNil(t, metrics.RiskMetrics)
	assert.Nil(t, metrics.AuditMetrics)
	assert.Nil(t, metrics.Alerts)
}

func TestComplianceDashboard_CheckCacheWrongType(t *testing.T) {
	t.Parallel()

	cache := newFakeDashboardCache()
	cache.Set("dashboard_metrics_1_2", "not-metrics", time.Minute)

	dashboard := NewComplianceDashboard(DashboardConfig{Enabled: true, CacheEnabled: true, CacheTTL: time.Minute})
	dashboard.SetCache(cache)

	metrics, err := dashboard.GetDashboardMetrics(context.Background(), TimeRange{Start: time.Unix(1, 0), End: time.Unix(2, 0)})
	require.NoError(t, err)
	require.NotNil(t, metrics)
}

func TestComplianceDashboard_GetWidgetAndExportAndLayout(t *testing.T) {
	t.Parallel()

	engine := &fakeMetricsEngine{
		compliance: &ComplianceMetrics{OverallScore: 90},
		risk:       &RiskMetrics{OverallRiskScore: 10},
		audit:      &AuditMetrics{FailureRate: 0.01},
		custom: []*CustomMetric{
			{ID: "c1", Value: 1},
		},
	}

	dashboard := NewComplianceDashboard(DashboardConfig{Enabled: true, ExportEnabled: true})
	dashboard.SetMetricsEngine(engine)

	widget, err := dashboard.GetWidget(context.Background(), "w1", WidgetConfig{DataSource: "compliance_metrics", TimeRange: TimeRange{Start: time.Unix(1, 0), End: time.Unix(2, 0)}})
	require.NoError(t, err)
	assert.IsType(t, &ComplianceMetrics{}, widget.Data)

	widget, err = dashboard.GetWidget(context.Background(), "w2", WidgetConfig{DataSource: "risk_metrics", TimeRange: TimeRange{Start: time.Unix(1, 0), End: time.Unix(2, 0)}})
	require.NoError(t, err)
	assert.IsType(t, &RiskMetrics{}, widget.Data)

	widget, err = dashboard.GetWidget(context.Background(), "w3", WidgetConfig{DataSource: "audit_metrics", TimeRange: TimeRange{Start: time.Unix(1, 0), End: time.Unix(2, 0)}})
	require.NoError(t, err)
	assert.IsType(t, &AuditMetrics{}, widget.Data)

	widget, err = dashboard.GetWidget(context.Background(), "w4", WidgetConfig{DataSource: "custom_metrics", TimeRange: TimeRange{Start: time.Unix(1, 0), End: time.Unix(2, 0)}})
	require.NoError(t, err)
	assert.IsType(t, []*CustomMetric{}, widget.Data)

	_, err = dashboard.GetWidget(context.Background(), "w5", WidgetConfig{DataSource: "nope"})
	require.Error(t, err)

	timeRange := TimeRange{Start: time.Unix(1, 0), End: time.Unix(2, 0)}
	out, err := dashboard.ExportDashboardData(context.Background(), "json", timeRange)
	require.NoError(t, err)
	assert.Equal(t, []byte("{}"), out)
	out, err = dashboard.ExportDashboardData(context.Background(), "csv", timeRange)
	require.NoError(t, err)
	assert.Equal(t, []byte(""), out)
	out, err = dashboard.ExportDashboardData(context.Background(), "pdf", timeRange)
	require.NoError(t, err)
	assert.Equal(t, []byte(""), out)
	_, err = dashboard.ExportDashboardData(context.Background(), "nope", timeRange)
	require.Error(t, err)

	noExport := NewComplianceDashboard(DashboardConfig{Enabled: true, ExportEnabled: false})
	_, err = noExport.ExportDashboardData(context.Background(), "json", timeRange)
	require.Error(t, err)

	layout, err := dashboard.GetDashboardLayout(context.Background(), "id")
	require.NoError(t, err)
	require.NotNil(t, layout)
	assert.True(t, layout.IsDefault)

	require.Error(t, dashboard.CreateDashboardLayout(context.Background(), &DashboardLayout{}))
	require.Error(t, dashboard.UpdateDashboardLayout(context.Background(), "id", &DashboardLayout{}))
	require.NoError(t, dashboard.DeleteDashboardLayout(context.Background(), "id"))
}

func TestComplianceDashboard_StatusHelpers(t *testing.T) {
	t.Parallel()

	dashboard := NewComplianceDashboard(DashboardConfig{Enabled: true})

	assert.Equal(t, statusExcellent, dashboard.getComplianceStatus(95))
	assert.Equal(t, statusGood, dashboard.getComplianceStatus(85))
	assert.Equal(t, statusFair, dashboard.getComplianceStatus(70))
	assert.Equal(t, statusPoor, dashboard.getComplianceStatus(0))

	assert.Equal(t, riskLevelCritical, dashboard.getRiskStatus(80))
	assert.Equal(t, "high", dashboard.getRiskStatus(60))
	assert.Equal(t, "medium", dashboard.getRiskStatus(40))
	assert.Equal(t, "low", dashboard.getRiskStatus(0))

	assert.Equal(t, statusExcellent, dashboard.getAuditStatus(0.01))
	assert.Equal(t, statusGood, dashboard.getAuditStatus(0.05))
	assert.Equal(t, statusFair, dashboard.getAuditStatus(0.1))
	assert.Equal(t, statusPoor, dashboard.getAuditStatus(0.2))

	assert.Equal(t, statusExcellent, dashboard.getOverallHealth(0.95))
	assert.Equal(t, statusGood, dashboard.getOverallHealth(0.85))
	assert.Equal(t, statusFair, dashboard.getOverallHealth(0.75))
	assert.Equal(t, statusPoor, dashboard.getOverallHealth(0.65))
	assert.Equal(t, riskLevelCritical, dashboard.getOverallHealth(0.55))
}
