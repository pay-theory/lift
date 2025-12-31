package analytics

import (
	"context"
	"errors"
	"math"
	"strings"
	"sync"
	"testing"
	"time"
)

type stubAnalyticsStore struct {
	getMetricsErr error
	metrics       []PerformanceMetric

	mu            sync.Mutex
	getMetricsHit int
	getMetricsCh  chan MetricQuery
}

func (s *stubAnalyticsStore) StoreMetric(context.Context, *PerformanceMetric) error { return nil }

func (s *stubAnalyticsStore) GetMetrics(_ context.Context, query MetricQuery) ([]PerformanceMetric, error) {
	s.mu.Lock()
	s.getMetricsHit++
	ch := s.getMetricsCh
	s.mu.Unlock()

	if ch != nil {
		select {
		case ch <- query:
		default:
		}
	}

	if s.getMetricsErr != nil {
		return nil, s.getMetricsErr
	}
	return append([]PerformanceMetric(nil), s.metrics...), nil
}

func (s *stubAnalyticsStore) GetAggregatedMetrics(context.Context, AggregateQuery) ([]AggregatedMetric, error) {
	return nil, nil
}

func (s *stubAnalyticsStore) DeleteOldMetrics(context.Context, time.Time) error { return nil }
func (s *stubAnalyticsStore) GetMetricNames() []string                          { return nil }

type stubTrendAnalyzer struct {
	trends       []PerformanceTrend
	predictions  []PerformancePrediction
	analyzeErr   error
	predictErr   error
	seasonality  SeasonalityPattern
	correlations CorrelationMatrix
}

func (s *stubTrendAnalyzer) AnalyzeTrends(context.Context, []PerformanceMetric) ([]PerformanceTrend, error) {
	if s.analyzeErr != nil {
		return nil, s.analyzeErr
	}
	return append([]PerformanceTrend(nil), s.trends...), nil
}

func (s *stubTrendAnalyzer) PredictTrends(context.Context, []PerformanceMetric, time.Duration) ([]PerformancePrediction, error) {
	if s.predictErr != nil {
		return nil, s.predictErr
	}
	return append([]PerformancePrediction(nil), s.predictions...), nil
}

func (s *stubTrendAnalyzer) DetectSeasonality(context.Context, []PerformanceMetric) (SeasonalityPattern, error) {
	return s.seasonality, nil
}

func (s *stubTrendAnalyzer) CalculateCorrelations(context.Context, map[string][]PerformanceMetric) (CorrelationMatrix, error) {
	return s.correlations, nil
}

type stubAnomalyDetector struct {
	anomalies   []PerformanceAnomaly
	detectErr   error
	sensitivity float64
}

func (s *stubAnomalyDetector) DetectAnomalies(context.Context, []PerformanceMetric) ([]PerformanceAnomaly, error) {
	if s.detectErr != nil {
		return nil, s.detectErr
	}
	return append([]PerformanceAnomaly(nil), s.anomalies...), nil
}

func (s *stubAnomalyDetector) TrainModel(context.Context, []PerformanceMetric) error { return nil }
func (s *stubAnomalyDetector) UpdateBaseline(context.Context, []PerformanceMetric) error {
	return nil
}
func (s *stubAnomalyDetector) GetDetectionSensitivity() float64 { return s.sensitivity }
func (s *stubAnomalyDetector) SetDetectionSensitivity(sensitivity float64) {
	s.sensitivity = sensitivity
}

type stubThresholdManager struct {
	violationsByMetric map[string][]ThresholdViolation
}

func (s *stubThresholdManager) GetThresholds(string) []Threshold { return nil }
func (s *stubThresholdManager) SetThreshold(string, Threshold) error {
	return nil
}
func (s *stubThresholdManager) UpdateDynamicThresholds(context.Context, []PerformanceMetric) error {
	return nil
}
func (s *stubThresholdManager) EvaluateThresholds(metric PerformanceMetric) []ThresholdViolation {
	return append([]ThresholdViolation(nil), s.violationsByMetric[metric.Name]...)
}

func TestNormalizePerformanceAnalyticsConfig(t *testing.T) {
	_, err := normalizePerformanceAnalyticsConfig(PerformanceAnalyticsConfig{
		Enabled:           true,
		DataRetentionDays: -1,
	})
	if err == nil {
		t.Fatal("expected error for negative data retention")
	}

	_, err = normalizePerformanceAnalyticsConfig(PerformanceAnalyticsConfig{
		Enabled:          true,
		AnalysisInterval: -1 * time.Second,
	})
	if err == nil {
		t.Fatal("expected error for negative analysis interval")
	}

	cfg, err := normalizePerformanceAnalyticsConfig(PerformanceAnalyticsConfig{
		Enabled:          true,
		AnalysisInterval: 0,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if cfg.AnalysisInterval != defaultAnalysisInterval {
		t.Fatalf("expected default analysis interval %v, got %v", defaultAnalysisInterval, cfg.AnalysisInterval)
	}
}

func TestPerformanceAnalyticsEngine_Start_Errors(t *testing.T) {
	store := &stubAnalyticsStore{}

	notEnabled := NewPerformanceAnalyticsEngine(PerformanceAnalyticsConfig{Enabled: false})
	notEnabled.SetDataStore(store)
	if err := notEnabled.Start(context.Background()); err == nil {
		t.Fatal("expected error when engine is not enabled")
	}

	engine := NewPerformanceAnalyticsEngine(PerformanceAnalyticsConfig{
		Enabled:          true,
		AnalysisInterval: time.Millisecond,
	})
	engine.SetDataStore(store)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := engine.Start(ctx); err != nil {
		t.Fatalf("unexpected error starting engine: %v", err)
	}
	t.Cleanup(func() { _ = engine.Stop() })

	if err := engine.Start(ctx); err == nil {
		t.Fatal("expected error when engine is already running")
	}
}

func TestAnalyzePerformance_DataStoreErrorAndEmptyMetrics(t *testing.T) {
	engine := NewPerformanceAnalyticsEngine(PerformanceAnalyticsConfig{
		Enabled: true,
	})

	engine.SetDataStore(&stubAnalyticsStore{
		getMetricsErr: errors.New("boom"),
	})

	_, err := engine.AnalyzePerformance(context.Background(), TimeRange{
		Start: time.Now().Add(-time.Minute),
		End:   time.Now(),
	})
	if err == nil || !strings.Contains(err.Error(), "failed to get metrics") {
		t.Fatalf("expected wrapped get metrics error, got %v", err)
	}

	engine.SetDataStore(&stubAnalyticsStore{metrics: nil})
	got, err := engine.AnalyzePerformance(context.Background(), TimeRange{
		Start: time.Now().Add(-time.Minute),
		End:   time.Now(),
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got.ID == "" || !strings.HasPrefix(got.ID, "analysis-") {
		t.Fatalf("expected analysis ID to be set, got %q", got.ID)
	}
	if len(got.Metrics) != 0 {
		t.Fatalf("expected 0 metrics, got %d", len(got.Metrics))
	}
	if got.HealthScore != 0 {
		t.Fatalf("expected health score 0 for empty analysis, got %v", got.HealthScore)
	}
}

func TestAnalyzePerformance_FullPipeline_WithAlertsAndPredictions(t *testing.T) {
	now := time.Now()
	metrics := []PerformanceMetric{
		{Name: "latency", Value: 10, Timestamp: now.Add(-5 * time.Minute)},
		{Name: "latency", Value: 11, Timestamp: now.Add(-4 * time.Minute)},
		{Name: "latency", Value: 12, Timestamp: now.Add(-3 * time.Minute)},
		{Name: "latency", Value: -100, Timestamp: now.Add(-2 * time.Minute)},
		{Name: "latency", Value: 100, Timestamp: now.Add(-time.Minute)},
	}

	store := &stubAnalyticsStore{metrics: metrics}
	engine := NewPerformanceAnalyticsEngine(PerformanceAnalyticsConfig{
		Enabled:             true,
		EnablePredictive:    true,
		TrendAnalysisWindow: time.Hour,
		AlertingEnabled:     true,
	})
	engine.SetDataStore(store)

	engine.SetTrendAnalyzer(&stubTrendAnalyzer{
		trends: []PerformanceTrend{
			{
				ID:         "t1",
				MetricName: "latency",
				Direction:  TrendDirectionDown,
				Confidence: 0.9,
				ChangeRate: -0.1,
			},
		},
		predictions: []PerformancePrediction{
			{
				ID:             "p1",
				MetricName:     "latency",
				PredictionType: PredictionTypeValue,
				TimeHorizon:    time.Hour,
				PredictedValue: 9.5,
			},
		},
	})

	engine.SetAnomalyDetector(&stubAnomalyDetector{
		anomalies: []PerformanceAnomaly{
			{
				ID:          "a1",
				MetricName:  "latency",
				Severity:    AnomalySeverityCritical,
				Explanation: "spike",
				Deviation:   0.5,
				Value:       100,
				Timestamp:   now,
			},
		},
	})

	engine.SetThresholdManager(&stubThresholdManager{
		violationsByMetric: map[string][]ThresholdViolation{
			"latency": {
				{
					Timestamp: now,
					Threshold: Threshold{
						Name:     "p95",
						Operator: ThresholdOperatorGT,
						Severity: AlertSeverityError,
						Value:    12,
					},
					ActualValue: 100,
				},
			},
		},
	})

	got, err := engine.AnalyzePerformance(context.Background(), TimeRange{
		Start: now.Add(-10 * time.Minute),
		End:   now,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got.Statistics.Count != int64(len(metrics)) {
		t.Fatalf("expected stats count %d, got %d", len(metrics), got.Statistics.Count)
	}
	if len(got.Trends) != 1 || got.Trends[0].ID != "t1" {
		t.Fatalf("expected 1 trend, got %#v", got.Trends)
	}
	if len(got.Predictions) != 1 || got.Predictions[0].ID != "p1" {
		t.Fatalf("expected 1 prediction, got %#v", got.Predictions)
	}
	if len(got.Anomalies) != 1 || got.Anomalies[0].ID != "a1" {
		t.Fatalf("expected 1 anomaly, got %#v", got.Anomalies)
	}
	if got.HealthScore <= 0 || got.HealthScore > 100 {
		t.Fatalf("expected health score in (0,100], got %v", got.HealthScore)
	}
	if len(got.Recommendations) == 0 {
		t.Fatal("expected recommendations to be generated")
	}
	if len(got.Alerts) == 0 {
		t.Fatal("expected alerts to be generated")
	}
}

func TestAnalyzePerformance_IgnoresDependencyErrors(t *testing.T) {
	now := time.Now()
	engine := NewPerformanceAnalyticsEngine(PerformanceAnalyticsConfig{
		Enabled:             true,
		EnablePredictive:    true,
		TrendAnalysisWindow: time.Hour,
		AlertingEnabled:     false,
	})
	engine.SetDataStore(&stubAnalyticsStore{
		metrics: []PerformanceMetric{
			{Name: "latency", Value: 1, Timestamp: now.Add(-time.Minute)},
			{Name: "latency", Value: 2, Timestamp: now},
		},
	})
	engine.SetTrendAnalyzer(&stubTrendAnalyzer{
		analyzeErr: errors.New("trend fail"),
		predictErr: errors.New("predict fail"),
	})
	engine.SetAnomalyDetector(&stubAnomalyDetector{
		detectErr: errors.New("anomaly fail"),
	})

	got, err := engine.AnalyzePerformance(context.Background(), TimeRange{
		Start: now.Add(-10 * time.Minute),
		End:   now,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got.Trends != nil {
		t.Fatalf("expected trends to be omitted on analyzer error, got %#v", got.Trends)
	}
	if got.Predictions != nil {
		t.Fatalf("expected predictions to be omitted on predictor error, got %#v", got.Predictions)
	}
	if got.Anomalies != nil {
		t.Fatalf("expected anomalies to be omitted on detector error, got %#v", got.Anomalies)
	}
	if got.Alerts != nil {
		t.Fatalf("expected no alerts when alerting is disabled, got %#v", got.Alerts)
	}
}

func TestStatisticsHelpers_DistributionPercentilesOutliers(t *testing.T) {
	engine := NewPerformanceAnalyticsEngine(PerformanceAnalyticsConfig{Enabled: true})

	makeMetrics := func(zeros int) []PerformanceMetric {
		metrics := make([]PerformanceMetric, 0, zeros+2)
		now := time.Now()
		for i := 0; i < zeros; i++ {
			metrics = append(metrics, PerformanceMetric{Name: "m", Value: 0, Timestamp: now.Add(time.Duration(i) * time.Second)})
		}
		metrics = append(metrics,
			PerformanceMetric{Name: "m", Value: -100, Timestamp: now.Add(100 * time.Second)},
			PerformanceMetric{Name: "m", Value: 100, Timestamp: now.Add(101 * time.Second)},
		)
		return metrics
	}

	t.Run("moderate outliers", func(t *testing.T) {
		stats := engine.calculateStatistics(makeMetrics(8))
		if len(stats.Outliers) != 2 {
			t.Fatalf("expected 2 outliers, got %d", len(stats.Outliers))
		}
		for _, outlier := range stats.Outliers {
			if outlier.Severity != "moderate" {
				t.Fatalf("expected moderate outlier, got %q (z=%v)", outlier.Severity, outlier.ZScore)
			}
		}
	})

	t.Run("extreme outliers", func(t *testing.T) {
		stats := engine.calculateStatistics(makeMetrics(20))
		if len(stats.Outliers) != 2 {
			t.Fatalf("expected 2 outliers, got %d", len(stats.Outliers))
		}
		for _, outlier := range stats.Outliers {
			if outlier.Severity != "extreme" {
				t.Fatalf("expected extreme outlier, got %q (z=%v)", outlier.Severity, outlier.ZScore)
			}
		}
	})

	if got := calculatePercentile(nil, 50); got != 0 {
		t.Fatalf("expected percentile of empty slice to be 0, got %v", got)
	}
	if got := calculatePercentile([]float64{1, 2, 3}, 50); got != 2 {
		t.Fatalf("expected p50=2, got %v", got)
	}
	if got := calculatePercentile([]float64{1, 3}, 50); got != 2 {
		t.Fatalf("expected interpolated p50=2, got %v", got)
	}

	rightStats := engine.calculateStatistics([]PerformanceMetric{
		{Name: "m", Value: 0},
		{Name: "m", Value: 0},
		{Name: "m", Value: 0},
		{Name: "m", Value: 0},
		{Name: "m", Value: 100},
	})
	if rightStats.Distribution.Type != "right_skewed" && rightStats.Distribution.Type != "normal" {
		t.Fatalf("unexpected distribution type %q", rightStats.Distribution.Type)
	}

	leftStats := engine.calculateStatistics([]PerformanceMetric{
		{Name: "m", Value: -100},
		{Name: "m", Value: 0},
		{Name: "m", Value: 0},
		{Name: "m", Value: 0},
		{Name: "m", Value: 0},
	})
	if leftStats.Distribution.Type != "left_skewed" && leftStats.Distribution.Type != "normal" {
		t.Fatalf("unexpected distribution type %q", leftStats.Distribution.Type)
	}
}

func TestHealthScoreRecommendationsAndAlerts(t *testing.T) {
	engine := NewPerformanceAnalyticsEngine(PerformanceAnalyticsConfig{
		Enabled:         true,
		AlertingEnabled: true,
	})

	engine.SetThresholdManager(&stubThresholdManager{
		violationsByMetric: map[string][]ThresholdViolation{
			"latency": {
				{
					Threshold: Threshold{Name: "max", Severity: AlertSeverityError, Value: 10},
					ActualValue: 100,
				},
			},
		},
	})

	analysis := &PerformanceAnalysis{
		Metrics: []PerformanceMetric{
			{Name: "latency", Value: 100, Timestamp: time.Now()},
		},
		Statistics: PerformanceStatistics{
			Mean:  10,
			StdDev: 11,
		},
		Anomalies: []PerformanceAnomaly{
			{Severity: AnomalySeverityCritical, MetricName: "latency", Deviation: 0.5, Timestamp: time.Now()},
			{Severity: AnomalySeverityHigh, MetricName: "latency", Deviation: 0.2, Timestamp: time.Now()},
			{Severity: AnomalySeverityMedium, MetricName: "latency", Deviation: 0.1, Timestamp: time.Now()},
			{Severity: AnomalySeverityLow, MetricName: "latency", Deviation: 0.05, Timestamp: time.Now()},
		},
		Trends: []PerformanceTrend{
			{MetricName: "latency", Direction: TrendDirectionDown, Confidence: 0.8, ChangeRate: -0.1},
			{MetricName: "latency", Direction: TrendDirectionDown, Confidence: 0.6, ChangeRate: -0.1},
		},
	}

	score := engine.calculateHealthScore(analysis)
	if score != 49 {
		t.Fatalf("expected score 49, got %v", score)
	}

	analysis.Anomalies = make([]PerformanceAnomaly, 6)
	for i := range analysis.Anomalies {
		analysis.Anomalies[i] = PerformanceAnomaly{Severity: AnomalySeverityCritical}
	}
	if score := engine.calculateHealthScore(analysis); score != 0 {
		t.Fatalf("expected score to clamp to 0, got %v", score)
	}

	analysis.Metrics = nil
	if score := engine.calculateHealthScore(analysis); score != 0 {
		t.Fatalf("expected score 0 for empty metrics, got %v", score)
	}

	analysis.Metrics = []PerformanceMetric{{Name: "latency", Value: 100, Timestamp: time.Now()}}
	analysis.Anomalies = []PerformanceAnomaly{
		{Severity: AnomalySeverityHigh, MetricName: "latency", Deviation: 0.2, Timestamp: time.Now(), Value: 100},
		{Severity: AnomalySeverityLow, MetricName: "latency", Deviation: 0.1, Timestamp: time.Now(), Value: 100},
	}
	analysis.Trends = []PerformanceTrend{
		{MetricName: "latency", Direction: TrendDirectionDown, Confidence: 0.9, ChangeRate: -0.1},
	}
	analysis.Statistics = PerformanceStatistics{Mean: 10, StdDev: 6}

	recs := engine.generateRecommendations(analysis)
	if len(recs) < 3 {
		t.Fatalf("expected at least 3 recommendations, got %d", len(recs))
	}

	alerts := engine.generateAlerts(analysis)
	if len(alerts) < 1 {
		t.Fatalf("expected at least 1 alert, got %d", len(alerts))
	}
	foundCriticalAnomaly := false
	foundThreshold := false
	for _, alert := range alerts {
		switch alert.TriggerType {
		case AlertTriggerAnomaly:
			foundCriticalAnomaly = true
		case AlertTriggerThreshold:
			foundThreshold = true
		}
	}
	if !foundThreshold {
		t.Fatal("expected at least one threshold-based alert")
	}
	if foundCriticalAnomaly {
		t.Fatal("did not expect an anomaly-based alert without a critical anomaly")
	}
}

func TestPerformanceAnalyticsEngine_runContinuousAnalysis_TicksAndStops(t *testing.T) {
	store := &stubAnalyticsStore{
		getMetricsCh: make(chan MetricQuery, 5),
	}

	engine := NewPerformanceAnalyticsEngine(PerformanceAnalyticsConfig{
		Enabled:          true,
		AnalysisInterval: time.Millisecond,
	})
	engine.SetDataStore(store)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := engine.Start(ctx); err != nil {
		t.Fatalf("unexpected error starting engine: %v", err)
	}
	t.Cleanup(func() { _ = engine.Stop() })

	select {
	case <-store.getMetricsCh:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected continuous analysis loop to call GetMetrics")
	}

	if err := engine.Stop(); err != nil {
		t.Fatalf("unexpected error stopping engine: %v", err)
	}
}

func TestAnomalyDetectorSensitivityAccessors(t *testing.T) {
	detector := &stubAnomalyDetector{}
	if detector.GetDetectionSensitivity() != 0 {
		t.Fatalf("expected default sensitivity 0, got %v", detector.GetDetectionSensitivity())
	}
	detector.SetDetectionSensitivity(0.75)
	if math.Abs(detector.GetDetectionSensitivity()-0.75) > 1e-9 {
		t.Fatalf("expected sensitivity 0.75, got %v", detector.GetDetectionSensitivity())
	}
}

