package security

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ComplianceDashboard provides real-time compliance visibility
type ComplianceDashboard struct {
	config         DashboardConfig
	metricsEngine  MetricsEngine
	dataAggregator DataAggregator
	alertManager   DashboardAlertManager
	cache          DashboardCache
	mu             sync.RWMutex
	running        bool
}

// DashboardConfig configuration for compliance dashboard
type DashboardConfig struct {
	Enabled              bool          `json:"enabled"`
	RefreshInterval      time.Duration `json:"refresh_interval"`
	CacheEnabled         bool          `json:"cache_enabled"`
	CacheTTL             time.Duration `json:"cache_ttl"`
	RealTimeUpdates      bool          `json:"real_time_updates"`
	HistoricalDataDays   int           `json:"historical_data_days"`
	MaxDataPoints        int           `json:"max_data_points"`
	AlertingEnabled      bool          `json:"alerting_enabled"`
	ExportEnabled        bool          `json:"export_enabled"`
	CustomMetricsEnabled bool          `json:"custom_metrics_enabled"`
}

// MetricsEngine interface for metrics calculation
type MetricsEngine interface {
	CalculateComplianceMetrics(ctx context.Context, timeRange TimeRange) (*ComplianceMetrics, error)
	CalculateRiskMetrics(ctx context.Context, timeRange TimeRange) (*RiskMetrics, error)
	CalculateAuditMetrics(ctx context.Context, timeRange TimeRange) (*AuditMetrics, error)
	CalculatePerformanceMetrics(ctx context.Context, timeRange TimeRange) (*PerformanceMetrics, error)
	CalculateCustomMetrics(ctx context.Context, queries []CustomMetricQuery) ([]*CustomMetric, error)
}

// DataAggregator interface for data aggregation
type DataAggregator interface {
	AggregateByTimeframe(ctx context.Context, data []DataPoint, interval time.Duration) ([]AggregatedDataPoint, error)
	AggregateByDimension(ctx context.Context, data []DataPoint, dimension string) (map[string]float64, error)
	CalculateTrends(ctx context.Context, data []DataPoint) (*TrendAnalysis, error)
	GenerateSummary(ctx context.Context, data []DataPoint) (*DataSummary, error)
}

// DashboardAlertManager interface for dashboard alerts
type DashboardAlertManager interface {
	CheckThresholds(ctx context.Context, metrics *DashboardMetrics) ([]*DashboardAlert, error)
	SendAlert(ctx context.Context, alert *DashboardAlert) error
	GetActiveAlerts(ctx context.Context) ([]*DashboardAlert, error)
	AcknowledgeAlert(ctx context.Context, alertID string, acknowledgedBy string) error
}

// DashboardCache interface for dashboard caching
type DashboardCache interface {
	Get(key string) (any, bool)
	Set(key string, value any, ttl time.Duration)
	Delete(key string)
	Clear()
}

// DashboardMetrics represents comprehensive dashboard metrics
type DashboardMetrics struct {
	Timestamp          time.Time           `json:"timestamp"`
	ComplianceMetrics  *ComplianceMetrics  `json:"compliance_metrics"`
	RiskMetrics        *RiskMetrics        `json:"risk_metrics"`
	AuditMetrics       *AuditMetrics       `json:"audit_metrics"`
	PerformanceMetrics *PerformanceMetrics `json:"performance_metrics"`
	CustomMetrics      []*CustomMetric     `json:"custom_metrics"`
	Alerts             []*DashboardAlert   `json:"alerts"`
	Summary            *DashboardSummary   `json:"summary"`
}

// ComplianceMetrics represents compliance-specific metrics
type ComplianceMetrics struct {
	OverallScore         float64                    `json:"overall_score"`
	FrameworkScores      map[string]float64         `json:"framework_scores"`
	ControlEffectiveness map[string]float64         `json:"control_effectiveness"`
	ViolationCount       int                        `json:"violation_count"`
	ViolationsByType     map[string]int             `json:"violations_by_type"`
	ViolationsBySeverity map[string]int             `json:"violations_by_severity"`
	ComplianceRate       float64                    `json:"compliance_rate"`
	TrendDirection       string                     `json:"trend_direction"`
	LastAuditDate        time.Time                  `json:"last_audit_date"`
	NextAuditDate        time.Time                  `json:"next_audit_date"`
	CertificationStatus  []CertificationStatus      `json:"certification_status"`
	Recommendations      []ComplianceRecommendation `json:"recommendations"`
	HistoricalData       []ComplianceDataPoint      `json:"historical_data"`
}

// RiskMetrics represents risk-specific metrics
type RiskMetrics struct {
	OverallRiskScore    float64            `json:"overall_risk_score"`
	RiskLevel           string             `json:"risk_level"`
	RiskDistribution    map[string]int     `json:"risk_distribution"`
	TopRiskFactors      []RiskFactor       `json:"top_risk_factors"`
	RiskTrend           string             `json:"risk_trend"`
	IncidentCount       int                `json:"incident_count"`
	IncidentsByType     map[string]int     `json:"incidents_by_type"`
	IncidentsBySeverity map[string]int     `json:"incidents_by_severity"`
	ThreatLevel         string             `json:"threat_level"`
	VulnerabilityCount  int                `json:"vulnerability_count"`
	MitigationProgress  map[string]float64 `json:"mitigation_progress"`
	RiskAppetite        float64            `json:"risk_appetite"`
	RiskTolerance       float64            `json:"risk_tolerance"`
	HistoricalData      []RiskDataPoint    `json:"historical_data"`
}

// AuditMetrics represents audit-specific metrics
type AuditMetrics struct {
	TotalEvents         int              `json:"total_events"`
	EventsByType        map[string]int   `json:"events_by_type"`
	EventsBySeverity    map[string]int   `json:"events_by_severity"`
	EventsBySource      map[string]int   `json:"events_by_source"`
	AnomalyCount        int              `json:"anomaly_count"`
	AnomaliesByType     map[string]int   `json:"anomalies_by_type"`
	FailedEvents        int              `json:"failed_events"`
	FailureRate         float64          `json:"failure_rate"`
	AverageEventSize    float64          `json:"average_event_size"`
	EventTrend          string           `json:"event_trend"`
	DataIntegrityScore  float64          `json:"data_integrity_score"`
	LogCompleteness     float64          `json:"log_completeness"`
	RetentionCompliance float64          `json:"retention_compliance"`
	HistoricalData      []AuditDataPoint `json:"historical_data"`
}

// CustomMetric represents a custom metric
type CustomMetric struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Value       float64                `json:"value"`
	Unit        string                 `json:"unit"`
	Type        string                 `json:"type"`
	Category    string                 `json:"category"`
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]any `json:"metadata"`
}

// CustomMetricQuery represents a query for custom metrics
type CustomMetricQuery struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Query       string                 `json:"query"`
	Type        string                 `json:"type"`
	Parameters  map[string]any `json:"parameters"`
	Aggregation string                 `json:"aggregation"`
	TimeRange   TimeRange              `json:"time_range"`
}

// DashboardAlert represents a dashboard alert
type DashboardAlert struct {
	ID             string                 `json:"id"`
	Type           string                 `json:"type"`
	Severity       string                 `json:"severity"`
	Title          string                 `json:"title"`
	Description    string                 `json:"description"`
	Metric         string                 `json:"metric"`
	Threshold      float64                `json:"threshold"`
	CurrentValue   float64                `json:"current_value"`
	Timestamp      time.Time              `json:"timestamp"`
	Status         string                 `json:"status"`
	AcknowledgedBy string                 `json:"acknowledged_by,omitempty"`
	AcknowledgedAt *time.Time             `json:"acknowledged_at,omitempty"`
	ResolvedAt     *time.Time             `json:"resolved_at,omitempty"`
	Actions        []AlertAction          `json:"actions"`
	Metadata       map[string]any `json:"metadata"`
}

// AlertAction represents an action for an alert
type AlertAction struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	Automated   bool                   `json:"automated"`
	Parameters  map[string]any `json:"parameters"`
}

// DashboardSummary represents a summary of dashboard data
type DashboardSummary struct {
	OverallHealth    string                 `json:"overall_health"`
	ComplianceStatus string                 `json:"compliance_status"`
	RiskStatus       string                 `json:"risk_status"`
	AuditStatus      string                 `json:"audit_status"`
	ActiveAlerts     int                    `json:"active_alerts"`
	CriticalIssues   int                    `json:"critical_issues"`
	TrendDirection   string                 `json:"trend_direction"`
	LastUpdated      time.Time              `json:"last_updated"`
	KeyMetrics       map[string]float64     `json:"key_metrics"`
	Recommendations  []string               `json:"recommendations"`
	Metadata         map[string]any `json:"metadata"`
}

// ComplianceDataPoint represents a compliance data point
type ComplianceDataPoint struct {
	Timestamp       time.Time              `json:"timestamp"`
	ComplianceScore float64                `json:"compliance_score"`
	ViolationCount  int                    `json:"violation_count"`
	ControlCount    int                    `json:"control_count"`
	Framework       string                 `json:"framework"`
	Metadata        map[string]any `json:"metadata"`
}

// RiskDataPoint represents a risk data point
type RiskDataPoint struct {
	Timestamp     time.Time              `json:"timestamp"`
	RiskScore     float64                `json:"risk_score"`
	IncidentCount int                    `json:"incident_count"`
	ThreatLevel   string                 `json:"threat_level"`
	Metadata      map[string]any `json:"metadata"`
}

// AuditDataPoint represents an audit data point
type AuditDataPoint struct {
	Timestamp    time.Time              `json:"timestamp"`
	EventCount   int                    `json:"event_count"`
	AnomalyCount int                    `json:"anomaly_count"`
	FailureRate  float64                `json:"failure_rate"`
	Metadata     map[string]any `json:"metadata"`
}

// DataPoint represents a generic data point
type DataPoint struct {
	Timestamp time.Time              `json:"timestamp"`
	Value     float64                `json:"value"`
	Labels    map[string]string      `json:"labels"`
	Metadata  map[string]any `json:"metadata"`
}

// AggregatedDataPoint represents an aggregated data point
type AggregatedDataPoint struct {
	Timestamp time.Time              `json:"timestamp"`
	Value     float64                `json:"value"`
	Count     int                    `json:"count"`
	Min       float64                `json:"min"`
	Max       float64                `json:"max"`
	Average   float64                `json:"average"`
	Sum       float64                `json:"sum"`
	StdDev    float64                `json:"std_dev"`
	Metadata  map[string]any `json:"metadata"`
}

// TrendAnalysis represents trend analysis results
type TrendAnalysis struct {
	Direction   string                 `json:"direction"`
	Magnitude   float64                `json:"magnitude"`
	Confidence  float64                `json:"confidence"`
	Seasonality bool                   `json:"seasonality"`
	Forecast    []ForecastPoint        `json:"forecast"`
	Metadata    map[string]any `json:"metadata"`
}

// ForecastPoint represents a forecast point
type ForecastPoint struct {
	Timestamp  time.Time `json:"timestamp"`
	Value      float64   `json:"value"`
	Confidence float64   `json:"confidence"`
	Lower      float64   `json:"lower"`
	Upper      float64   `json:"upper"`
}

// DataSummary represents a summary of data
type DataSummary struct {
	Count       int                    `json:"count"`
	Min         float64                `json:"min"`
	Max         float64                `json:"max"`
	Average     float64                `json:"average"`
	Median      float64                `json:"median"`
	StdDev      float64                `json:"std_dev"`
	Percentiles map[string]float64     `json:"percentiles"`
	Metadata    map[string]any `json:"metadata"`
}

// DashboardWidget represents a dashboard widget
type DashboardWidget struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Position    WidgetPosition         `json:"position"`
	Size        WidgetSize             `json:"size"`
	Config      WidgetConfig           `json:"config"`
	Data        any            `json:"data"`
	LastUpdated time.Time              `json:"last_updated"`
	Metadata    map[string]any `json:"metadata"`
}

// WidgetPosition represents widget position
type WidgetPosition struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// WidgetSize represents widget size
type WidgetSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// WidgetConfig represents widget configuration
type WidgetConfig struct {
	ChartType   string                 `json:"chart_type"`
	DataSource  string                 `json:"data_source"`
	RefreshRate time.Duration          `json:"refresh_rate"`
	Filters     map[string]any `json:"filters"`
	Aggregation string                 `json:"aggregation"`
	TimeRange   TimeRange              `json:"time_range"`
	Thresholds  map[string]float64     `json:"thresholds"`
	Colors      map[string]string      `json:"colors"`
	Metadata    map[string]any `json:"metadata"`
}

// DashboardLayout represents dashboard layout
type DashboardLayout struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Widgets     []DashboardWidget `json:"widgets"`
	CreatedBy   string            `json:"created_by"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	IsDefault   bool              `json:"is_default"`
	Permissions []string          `json:"permissions"`
}

// NewComplianceDashboard creates a new compliance dashboard
func NewComplianceDashboard(config DashboardConfig) *ComplianceDashboard {
	return &ComplianceDashboard{
		config: config,
	}
}

// SetMetricsEngine sets the metrics engine
func (cd *ComplianceDashboard) SetMetricsEngine(engine MetricsEngine) {
	cd.mu.Lock()
	defer cd.mu.Unlock()
	cd.metricsEngine = engine
}

// SetDataAggregator sets the data aggregator
func (cd *ComplianceDashboard) SetDataAggregator(aggregator DataAggregator) {
	cd.mu.Lock()
	defer cd.mu.Unlock()
	cd.dataAggregator = aggregator
}

// SetAlertManager sets the alert manager
func (cd *ComplianceDashboard) SetAlertManager(manager DashboardAlertManager) {
	cd.mu.Lock()
	defer cd.mu.Unlock()
	cd.alertManager = manager
}

// SetCache sets the dashboard cache
func (cd *ComplianceDashboard) SetCache(cache DashboardCache) {
	cd.mu.Lock()
	defer cd.mu.Unlock()
	cd.cache = cache
}

// Start starts the dashboard
func (cd *ComplianceDashboard) Start(ctx context.Context) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	if cd.running {
		return fmt.Errorf("dashboard already running")
	}

	if !cd.config.Enabled {
		return fmt.Errorf("dashboard not enabled")
	}

	// Start background refresh if real-time updates are enabled
	if cd.config.RealTimeUpdates {
		go cd.runBackgroundRefresh(ctx)
	}

	cd.running = true
	return nil
}

// Stop stops the dashboard
func (cd *ComplianceDashboard) Stop() error {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	if !cd.running {
		return nil
	}

	cd.running = false
	return nil
}

// GetDashboardMetrics returns current dashboard metrics
func (cd *ComplianceDashboard) GetDashboardMetrics(ctx context.Context, timeRange TimeRange) (*DashboardMetrics, error) {
	cd.mu.RLock()
	defer cd.mu.RUnlock()

	// Check cache first
	if cd.config.CacheEnabled && cd.cache != nil {
		cacheKey := fmt.Sprintf("dashboard_metrics_%d_%d", timeRange.Start.Unix(), timeRange.End.Unix())
		if cached, found := cd.cache.Get(cacheKey); found {
			if metrics, ok := cached.(*DashboardMetrics); ok {
				return metrics, nil
			}
		}
	}

	metrics := &DashboardMetrics{
		Timestamp: time.Now(),
	}

	// Get compliance metrics
	if cd.metricsEngine != nil {
		complianceMetrics, err := cd.metricsEngine.CalculateComplianceMetrics(ctx, timeRange)
		if err == nil {
			metrics.ComplianceMetrics = complianceMetrics
		}

		// Get risk metrics
		riskMetrics, err := cd.metricsEngine.CalculateRiskMetrics(ctx, timeRange)
		if err == nil {
			metrics.RiskMetrics = riskMetrics
		}

		// Get audit metrics
		auditMetrics, err := cd.metricsEngine.CalculateAuditMetrics(ctx, timeRange)
		if err == nil {
			metrics.AuditMetrics = auditMetrics
		}

		// Get performance metrics
		performanceMetrics, err := cd.metricsEngine.CalculatePerformanceMetrics(ctx, timeRange)
		if err == nil {
			metrics.PerformanceMetrics = performanceMetrics
		}

		// Get custom metrics if enabled
		if cd.config.CustomMetricsEnabled {
			customQueries := cd.getCustomMetricQueries(timeRange)
			customMetrics, err := cd.metricsEngine.CalculateCustomMetrics(ctx, customQueries)
			if err == nil {
				metrics.CustomMetrics = customMetrics
			}
		}
	}

	// Get active alerts
	if cd.alertManager != nil {
		alerts, err := cd.alertManager.GetActiveAlerts(ctx)
		if err == nil {
			metrics.Alerts = alerts
		}

		// Check for new alerts
		newAlerts, err := cd.alertManager.CheckThresholds(ctx, metrics)
		if err == nil {
			metrics.Alerts = append(metrics.Alerts, newAlerts...)
		}
	}

	// Generate summary
	metrics.Summary = cd.generateSummary(metrics)

	// Cache the result
	if cd.config.CacheEnabled && cd.cache != nil {
		cacheKey := fmt.Sprintf("dashboard_metrics_%d_%d", timeRange.Start.Unix(), timeRange.End.Unix())
		cd.cache.Set(cacheKey, metrics, cd.config.CacheTTL)
	}

	return metrics, nil
}

// GetWidget returns a specific widget's data
func (cd *ComplianceDashboard) GetWidget(ctx context.Context, widgetID string, config WidgetConfig) (*DashboardWidget, error) {
	cd.mu.RLock()
	defer cd.mu.RUnlock()

	widget := &DashboardWidget{
		ID:          widgetID,
		LastUpdated: time.Now(),
	}

	// Get data based on widget type and configuration
	switch config.DataSource {
	case "compliance_metrics":
		data, err := cd.metricsEngine.CalculateComplianceMetrics(ctx, config.TimeRange)
		if err != nil {
			return nil, err
		}
		widget.Data = data

	case "risk_metrics":
		data, err := cd.metricsEngine.CalculateRiskMetrics(ctx, config.TimeRange)
		if err != nil {
			return nil, err
		}
		widget.Data = data

	case "audit_metrics":
		data, err := cd.metricsEngine.CalculateAuditMetrics(ctx, config.TimeRange)
		if err != nil {
			return nil, err
		}
		widget.Data = data

	case "custom_metrics":
		queries := []CustomMetricQuery{{
			ID:        widgetID,
			TimeRange: config.TimeRange,
		}}
		data, err := cd.metricsEngine.CalculateCustomMetrics(ctx, queries)
		if err != nil {
			return nil, err
		}
		widget.Data = data

	default:
		return nil, fmt.Errorf("unsupported data source: %s", config.DataSource)
	}

	return widget, nil
}

// GetDashboardLayout returns a dashboard layout
func (cd *ComplianceDashboard) GetDashboardLayout(ctx context.Context, layoutID string) (*DashboardLayout, error) {
	// This would typically load from a database
	// For now, return a default layout
	return cd.getDefaultLayout(), nil
}

// CreateDashboardLayout creates a new dashboard layout
func (cd *ComplianceDashboard) CreateDashboardLayout(ctx context.Context, layout *DashboardLayout) error {
	// This would typically save to a database
	// For now, just validate the layout
	return cd.validateLayout(layout)
}

// UpdateDashboardLayout updates a dashboard layout
func (cd *ComplianceDashboard) UpdateDashboardLayout(ctx context.Context, layoutID string, layout *DashboardLayout) error {
	// This would typically update in a database
	// For now, just validate the layout
	return cd.validateLayout(layout)
}

// DeleteDashboardLayout deletes a dashboard layout
func (cd *ComplianceDashboard) DeleteDashboardLayout(ctx context.Context, layoutID string) error {
	// This would typically delete from a database
	// For now, just return success
	return nil
}

// ExportDashboardData exports dashboard data
func (cd *ComplianceDashboard) ExportDashboardData(ctx context.Context, format string, timeRange TimeRange) ([]byte, error) {
	if !cd.config.ExportEnabled {
		return nil, fmt.Errorf("export not enabled")
	}

	metrics, err := cd.GetDashboardMetrics(ctx, timeRange)
	if err != nil {
		return nil, err
	}

	switch format {
	case "json":
		return cd.exportJSON(metrics)
	case "csv":
		return cd.exportCSV(metrics)
	case "pdf":
		return cd.exportPDF(metrics)
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// runBackgroundRefresh runs background refresh
func (cd *ComplianceDashboard) runBackgroundRefresh(ctx context.Context) {
	ticker := time.NewTicker(cd.config.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cd.refreshCache(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// refreshCache refreshes the dashboard cache
func (cd *ComplianceDashboard) refreshCache(ctx context.Context) {
	if !cd.config.CacheEnabled || cd.cache == nil {
		return
	}

	// Refresh common time ranges
	timeRanges := []TimeRange{
		{Start: time.Now().Add(-24 * time.Hour), End: time.Now()},
		{Start: time.Now().Add(-7 * 24 * time.Hour), End: time.Now()},
		{Start: time.Now().Add(-30 * 24 * time.Hour), End: time.Now()},
	}

	for _, timeRange := range timeRanges {
		cd.GetDashboardMetrics(ctx, timeRange)
	}
}

// generateSummary generates a dashboard summary
func (cd *ComplianceDashboard) generateSummary(metrics *DashboardMetrics) *DashboardSummary {
	summary := &DashboardSummary{
		LastUpdated: time.Now(),
		KeyMetrics:  make(map[string]float64),
	}

	// Overall health calculation
	healthScore := 100.0
	if metrics.ComplianceMetrics != nil {
		summary.ComplianceStatus = cd.getComplianceStatus(metrics.ComplianceMetrics.OverallScore)
		summary.KeyMetrics["compliance_score"] = metrics.ComplianceMetrics.OverallScore
		healthScore *= (metrics.ComplianceMetrics.OverallScore / 100.0)
	}

	if metrics.RiskMetrics != nil {
		summary.RiskStatus = cd.getRiskStatus(metrics.RiskMetrics.OverallRiskScore)
		summary.KeyMetrics["risk_score"] = metrics.RiskMetrics.OverallRiskScore
		healthScore *= (1.0 - (metrics.RiskMetrics.OverallRiskScore / 100.0))
	}

	if metrics.AuditMetrics != nil {
		summary.AuditStatus = cd.getAuditStatus(metrics.AuditMetrics.FailureRate)
		summary.KeyMetrics["audit_failure_rate"] = metrics.AuditMetrics.FailureRate
		healthScore *= (1.0 - metrics.AuditMetrics.FailureRate)
	}

	// Count alerts
	if metrics.Alerts != nil {
		summary.ActiveAlerts = len(metrics.Alerts)
		criticalCount := 0
		for _, alert := range metrics.Alerts {
			if alert.Severity == "critical" {
				criticalCount++
			}
		}
		summary.CriticalIssues = criticalCount
	}

	// Overall health status
	summary.OverallHealth = cd.getOverallHealth(healthScore)

	// Generate recommendations
	summary.Recommendations = cd.generateRecommendations(metrics)

	return summary
}

// getCustomMetricQueries returns custom metric queries
func (cd *ComplianceDashboard) getCustomMetricQueries(timeRange TimeRange) []CustomMetricQuery {
	// This would typically be configured by users
	// For now, return some default queries
	return []CustomMetricQuery{
		{
			ID:        "user_activity",
			Name:      "User Activity",
			Type:      "count",
			TimeRange: timeRange,
		},
		{
			ID:        "data_access_volume",
			Name:      "Data Access Volume",
			Type:      "sum",
			TimeRange: timeRange,
		},
	}
}

// Helper methods

func (cd *ComplianceDashboard) getComplianceStatus(score float64) string {
	if score >= 95 {
		return "excellent"
	}
	if score >= 85 {
		return "good"
	}
	if score >= 70 {
		return "fair"
	}
	return "poor"
}

func (cd *ComplianceDashboard) getRiskStatus(score float64) string {
	if score >= 80 {
		return "critical"
	}
	if score >= 60 {
		return "high"
	}
	if score >= 40 {
		return "medium"
	}
	return "low"
}

func (cd *ComplianceDashboard) getAuditStatus(failureRate float64) string {
	if failureRate <= 0.01 {
		return "excellent"
	}
	if failureRate <= 0.05 {
		return "good"
	}
	if failureRate <= 0.1 {
		return "fair"
	}
	return "poor"
}

func (cd *ComplianceDashboard) getOverallHealth(score float64) string {
	if score >= 0.9 {
		return "excellent"
	}
	if score >= 0.8 {
		return "good"
	}
	if score >= 0.7 {
		return "fair"
	}
	if score >= 0.6 {
		return "poor"
	}
	return "critical"
}

func (cd *ComplianceDashboard) generateRecommendations(metrics *DashboardMetrics) []string {
	var recommendations []string

	if metrics.ComplianceMetrics != nil && metrics.ComplianceMetrics.OverallScore < 85 {
		recommendations = append(recommendations, "Improve compliance controls to reach target score")
	}

	if metrics.RiskMetrics != nil && metrics.RiskMetrics.OverallRiskScore > 70 {
		recommendations = append(recommendations, "Address high-risk factors to reduce overall risk")
	}

	if metrics.AuditMetrics != nil && metrics.AuditMetrics.FailureRate > 0.05 {
		recommendations = append(recommendations, "Investigate audit failures and improve logging")
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Continue monitoring and maintain current performance")
	}

	return recommendations
}

func (cd *ComplianceDashboard) getDefaultLayout() *DashboardLayout {
	return &DashboardLayout{
		ID:          "default",
		Name:        "Default Compliance Dashboard",
		Description: "Default layout for compliance monitoring",
		Widgets: []DashboardWidget{
			{
				ID:       "compliance_overview",
				Type:     "chart",
				Title:    "Compliance Overview",
				Position: WidgetPosition{X: 0, Y: 0},
				Size:     WidgetSize{Width: 6, Height: 4},
				Config: WidgetConfig{
					ChartType:  "gauge",
					DataSource: "compliance_metrics",
					TimeRange:  TimeRange{Start: time.Now().Add(-24 * time.Hour), End: time.Now()},
				},
			},
			{
				ID:       "risk_overview",
				Type:     "chart",
				Title:    "Risk Overview",
				Position: WidgetPosition{X: 6, Y: 0},
				Size:     WidgetSize{Width: 6, Height: 4},
				Config: WidgetConfig{
					ChartType:  "gauge",
					DataSource: "risk_metrics",
					TimeRange:  TimeRange{Start: time.Now().Add(-24 * time.Hour), End: time.Now()},
				},
			},
			{
				ID:       "audit_trends",
				Type:     "chart",
				Title:    "Audit Trends",
				Position: WidgetPosition{X: 0, Y: 4},
				Size:     WidgetSize{Width: 12, Height: 4},
				Config: WidgetConfig{
					ChartType:  "line",
					DataSource: "audit_metrics",
					TimeRange:  TimeRange{Start: time.Now().Add(-7 * 24 * time.Hour), End: time.Now()},
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		IsDefault: true,
	}
}

func (cd *ComplianceDashboard) validateLayout(layout *DashboardLayout) error {
	if layout.Name == "" {
		return fmt.Errorf("layout name is required")
	}

	for _, widget := range layout.Widgets {
		if widget.ID == "" {
			return fmt.Errorf("widget ID is required")
		}
		if widget.Type == "" {
			return fmt.Errorf("widget type is required")
		}
	}

	return nil
}

func (cd *ComplianceDashboard) exportJSON(_ *DashboardMetrics) ([]byte, error) {
	// This would implement JSON export
	return []byte("{}"), nil
}

func (cd *ComplianceDashboard) exportCSV(_ *DashboardMetrics) ([]byte, error) {
	// This would implement CSV export
	return []byte(""), nil
}

func (cd *ComplianceDashboard) exportPDF(_ *DashboardMetrics) ([]byte, error) {
	// This would implement PDF export
	return []byte(""), nil
}
