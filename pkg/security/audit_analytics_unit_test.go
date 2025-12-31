package security

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRiskScorer struct {
	riskScore       *RiskScore
	riskScoreErr    error
	aggregate       *AggregateRiskScore
	aggregateErr    error
	calculateCalls  int
	aggregateCalls  int
	updateModelCalls int
}

func (f *fakeRiskScorer) CalculateRiskScore(context.Context, *AuditEvent) (*RiskScore, error) {
	f.calculateCalls++
	return f.riskScore, f.riskScoreErr
}

func (f *fakeRiskScorer) CalculateAggregateRisk(context.Context, []*AuditEvent) (*AggregateRiskScore, error) {
	f.aggregateCalls++
	return f.aggregate, f.aggregateErr
}

func (f *fakeRiskScorer) UpdateRiskModel(context.Context, []*RiskFeedback) error {
	f.updateModelCalls++
	return nil
}

func (f *fakeRiskScorer) GetRiskFactors() []RiskFactor {
	return []RiskFactor{{ID: "rf1", Weight: 1}}
}

type fakeAnomalyDetector struct {
	anomalies     []*Anomaly
	err           error
	detectCalls   int
	trainCalls    int
	updateCalls   int
	baselineCalls int
}

func (f *fakeAnomalyDetector) DetectAnomalies(context.Context, []*AuditEvent) ([]*Anomaly, error) {
	f.detectCalls++
	return f.anomalies, f.err
}

func (f *fakeAnomalyDetector) TrainModel(context.Context, []*AuditEvent) error {
	f.trainCalls++
	return nil
}

func (f *fakeAnomalyDetector) UpdateBaseline(context.Context, []*AuditEvent) error {
	f.baselineCalls++
	return nil
}

func (f *fakeAnomalyDetector) GetAnomalyPatterns() []AnomalyPattern {
	return []AnomalyPattern{{ID: "p1"}}
}

type fakePredictiveModel struct {
	compliance *CompliancePrediction
	trends     []*TrendPrediction
	incidents  []*IncidentForecast

	complianceErr error
	trendsErr     error
	incidentsErr  error
}

func (f *fakePredictiveModel) PredictComplianceRisk(context.Context, time.Duration) (*CompliancePrediction, error) {
	return f.compliance, f.complianceErr
}

func (f *fakePredictiveModel) PredictTrends(context.Context, []string, time.Duration) ([]*TrendPrediction, error) {
	return f.trends, f.trendsErr
}

func (f *fakePredictiveModel) ForecastIncidents(context.Context, time.Duration) ([]*IncidentForecast, error) {
	return f.incidents, f.incidentsErr
}

func (f *fakePredictiveModel) UpdateModel(context.Context, []*AnalyticsDataPoint) error {
	return nil
}

type fakeAnalyticsDataStore struct {
	storeErr     error
	stored       []*AnalyticsDataPoint
	aggregated   *AggregatedMetrics
	aggregatedErr error
}

func (f *fakeAnalyticsDataStore) StoreAnalyticsData(_ context.Context, data *AnalyticsDataPoint) error {
	f.stored = append(f.stored, data)
	return f.storeErr
}

func (f *fakeAnalyticsDataStore) GetAnalyticsData(context.Context, *AnalyticsQuery) ([]*AnalyticsDataPoint, error) {
	return nil, nil
}

func (f *fakeAnalyticsDataStore) GetAggregatedMetrics(context.Context, *MetricsQuery) (*AggregatedMetrics, error) {
	return f.aggregated, f.aggregatedErr
}

func (f *fakeAnalyticsDataStore) CleanupOldData(context.Context, time.Duration) error {
	return nil
}

func TestAuditAnalyticsEngine_StartStop(t *testing.T) {
	t.Parallel()

	disabled := NewAuditAnalyticsEngine(AnalyticsConfig{Enabled: false, AnalysisInterval: time.Millisecond, MLModelUpdateInterval: time.Millisecond})
	require.Error(t, disabled.Start(context.Background()))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	engine := NewAuditAnalyticsEngine(AnalyticsConfig{
		Enabled:               true,
		RealTimeAnalysis:      true,
		AnalysisInterval:      time.Millisecond,
		MLModelUpdateInterval: time.Millisecond,
	})
	require.NoError(t, engine.Start(ctx))
	require.Error(t, engine.Start(ctx))
	require.NoError(t, engine.Stop())
	require.NoError(t, engine.Stop())
}

func TestAuditAnalyticsEngine_AnalyzeEventAndBatch(t *testing.T) {
	t.Parallel()

	risk := &fakeRiskScorer{
		riskScore: &RiskScore{Score: 80, Level: riskLevelHigh},
		aggregate: &AggregateRiskScore{OverallScore: 50, Level: "medium"},
	}
	anomaly := &fakeAnomalyDetector{
		anomalies: []*Anomaly{{ID: "a1", Score: 0.9}},
	}
	store := &fakeAnalyticsDataStore{storeErr: errors.New("store failed")}

	engine := NewAuditAnalyticsEngine(AnalyticsConfig{
		Enabled:            true,
		RiskScoringEnabled: true,
		AnomalyDetection:   true,
	})
	engine.SetRiskScorer(risk)
	engine.SetAnomalyDetector(anomaly)
	engine.SetDataStore(store)

	event := &AuditEvent{ID: "e1", EventType: "login", Source: "api"}
	analysis, err := engine.AnalyzeEvent(context.Background(), event)
	require.NoError(t, err)
	require.NotNil(t, analysis.RiskScore)
	assert.Len(t, analysis.Anomalies, 1)
	assert.Contains(t, analysis.Analyzes, "risk_scoring")
	assert.Contains(t, analysis.Analyzes, "anomaly_detection")

	require.Len(t, store.stored, 1)
	assert.Equal(t, "analysis_e1", store.stored[0].ID)
	assert.Equal(t, 80.0, store.stored[0].Metrics["risk_score"])

	risk.calculateCalls = 0
	event2 := &AuditEvent{ID: "e2", EventType: "read", Source: "api"}
	batch, err := engine.AnalyzeBatch(context.Background(), []*AuditEvent{event, event2})
	require.NoError(t, err)
	assert.Equal(t, 2, batch.EventCount)
	assert.Len(t, batch.EventAnalyzes, 2)
	assert.NotNil(t, batch.AggregateRisk)
	assert.NotNil(t, batch.BatchAnomalies)
	assert.Equal(t, 2, risk.calculateCalls)
	assert.Equal(t, 1, risk.aggregateCalls)
	assert.GreaterOrEqual(t, anomaly.detectCalls, 2)
}

func TestAuditAnalyticsEngine_PredictionsAndMetrics(t *testing.T) {
	t.Parallel()

	engine := NewAuditAnalyticsEngine(AnalyticsConfig{Enabled: true})
	_, err := engine.GeneratePredictions(context.Background(), time.Hour)
	require.Error(t, err)

	engine = NewAuditAnalyticsEngine(AnalyticsConfig{Enabled: true, PredictiveAnalysis: true})
	model := &fakePredictiveModel{
		compliance: &CompliancePrediction{PredictedRisk: 0.5},
		trends:     []*TrendPrediction{{Metric: "risk_score"}},
		incidents:  []*IncidentForecast{{Type: "test"}},
	}
	engine.SetPredictiveModel(model)

	report, err := engine.GeneratePredictions(context.Background(), 24*time.Hour)
	require.NoError(t, err)
	require.NotNil(t, report.CompliancePrediction)
	assert.Len(t, report.TrendPredictions, 1)
	assert.Len(t, report.IncidentForecasts, 1)

	_, err = engine.GetAnalyticsMetrics(context.Background())
	require.Error(t, err)

	store := &fakeAnalyticsDataStore{aggregated: &AggregatedMetrics{Metadata: map[string]any{"ok": true}}}
	engine.SetDataStore(store)

	metrics, err := engine.GetAnalyticsMetrics(context.Background())
	require.NoError(t, err)
	require.NotNil(t, metrics.AggregatedMetrics)
	require.NotNil(t, metrics.Performance)
	assert.Equal(t, time.Millisecond, metrics.Performance.AvgAnalysisTime)
	assert.Equal(t, 0.95, metrics.Performance.Accuracy)

	store.aggregatedErr = errors.New("agg failed")
	_, err = engine.GetAnalyticsMetrics(context.Background())
	require.Error(t, err)
}
