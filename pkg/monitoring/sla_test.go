package monitoring

import (
	"context"
	"math"
	"testing"
	"time"
)

func TestCalculateAvailability(t *testing.T) {
	sm := &SLAMonitor{}
	points := []MetricDataPoint{
		{Value: 1},
		{Value: 0},
		{Value: 1},
	}

	got := sm.calculateAvailability(points)
	want := (2.0 / 3.0) * 100.0

	if math.Abs(got-want) > 1e-6 {
		t.Fatalf("availability: want %.4f, got %.4f", want, got)
	}
}

func TestCalculateErrorRate(t *testing.T) {
	sm := &SLAMonitor{}
	points := []MetricDataPoint{
		{Value: 0},
		{Value: 1},
		{Value: 1},
		{Value: 0},
	}

	got := sm.calculateErrorRate(points)
	want := (2.0 / 4.0) * 100.0

	if math.Abs(got-want) > 1e-6 {
		t.Fatalf("error rate: want %.4f, got %.4f", want, got)
	}
}

func TestCalculateThroughput(t *testing.T) {
	sm := &SLAMonitor{}
	points := []MetricDataPoint{
		{Value: 5},
		{Value: 7},
		{Value: 3},
	}

	got := sm.calculateThroughput(points)
	want := 15.0

	if got != want {
		t.Fatalf("throughput: want %.2f, got %.2f", want, got)
	}
}

func TestCalculatePercentile(t *testing.T) {
	sm := &SLAMonitor{}
	points := []MetricDataPoint{
		{Value: 10},
		{Value: 20},
		{Value: 30},
		{Value: 40},
		{Value: 50},
	}

	got := sm.calculatePercentile(points, 95.0)
	want := 30.0

	if got != want {
		t.Fatalf("percentile: want %.2f, got %.2f", want, got)
	}
}

func TestUpdateErrorBudget(t *testing.T) {
	now := time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		slo        SLO
		metrics    *SLAMetrics
		wantBudget float64
		wantBurn   float64
	}{
		{
			name: "availability clamps and updates burn rate",
			slo: SLO{
				Type:   SLOAvailability,
				Target: 95,
			},
			metrics: &SLAMetrics{
				CurrentValue: 90,
				History: []MetricDataPoint{
					{Timestamp: now.Add(-2 * time.Hour), Value: 98},
					{Timestamp: now.Add(-1 * time.Hour), Value: 90},
				},
			},
			wantBudget: 0,
			wantBurn:   8,
		},
		{
			name: "error rate calculates remaining budget",
			slo: SLO{
				Type:   SLOErrorRate,
				Target: 5,
			},
			metrics: &SLAMetrics{
				CurrentValue: 1.5,
				History: []MetricDataPoint{
					{Timestamp: now.Add(-1 * time.Hour), Value: 1.5},
				},
			},
			wantBudget: 70,
			wantBurn:   0,
		},
		{
			name: "throughput defaults to full budget",
			slo: SLO{
				Type:   SLOThroughput,
				Target: 100,
			},
			metrics: &SLAMetrics{
				CurrentValue: 500,
				History: []MetricDataPoint{
					{Timestamp: now.Add(-30 * time.Minute), Value: 500},
				},
			},
			wantBudget: 100,
			wantBurn:   0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sm := &SLAMonitor{}
			sm.updateErrorBudget(tc.metrics, tc.slo)

			if math.Abs(tc.metrics.ErrorBudget-tc.wantBudget) > 1e-6 {
				t.Fatalf("error budget: want %.2f, got %.2f", tc.wantBudget, tc.metrics.ErrorBudget)
			}

			if math.Abs(tc.metrics.BurnRate-tc.wantBurn) > 1e-6 {
				t.Fatalf("burn rate: want %.2f, got %.2f", tc.wantBurn, tc.metrics.BurnRate)
			}
		})
	}
}

func TestDetermineSeverity(t *testing.T) {
	sm := &SLAMonitor{
		config: SLAConfig{
			Thresholds: ThresholdConfig{
				WarningThreshold:  40,
				CriticalThreshold: 10,
			},
		},
	}

	tests := []struct {
		name     string
		slo      SLO
		metrics  *SLAMetrics
		expected Severity
	}{
		{
			name:     "critical SLO forces critical severity",
			slo:      SLO{Critical: true},
			metrics:  &SLAMetrics{ErrorBudget: 80},
			expected: SeverityCritical,
		},
		{
			name:     "critical threshold",
			slo:      SLO{Critical: false},
			metrics:  &SLAMetrics{ErrorBudget: 5},
			expected: SeverityCritical,
		},
		{
			name:     "warning threshold",
			slo:      SLO{Critical: false},
			metrics:  &SLAMetrics{ErrorBudget: 30},
			expected: SeverityWarning,
		},
		{
			name:     "healthy budget",
			slo:      SLO{Critical: false},
			metrics:  &SLAMetrics{ErrorBudget: 90},
			expected: SeverityInfo,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := sm.determineSeverity(tc.metrics, tc.slo); got != tc.expected {
				t.Fatalf("expected severity %s, got %s", tc.expected, got)
			}
		})
	}
}

type staticCollector struct {
	points []MetricDataPoint
}

func (s *staticCollector) CollectMetrics(context.Context) ([]MetricDataPoint, error) {
	return s.points, nil
}

func (s *staticCollector) GetMetricType() SLOType {
	return SLOAvailability
}

func TestAlertManagerTriggersOnViolation(t *testing.T) {
	now := time.Now()
	sloName := "availability"
	sm := &SLAMonitor{
		config: SLAConfig{
			SLOs: []SLO{
				{
					Name:     sloName,
					Type:     SLOAvailability,
					Target:   95,
					Window:   time.Hour,
					Enabled:  true,
					Critical: false,
				},
			},
			Thresholds: ThresholdConfig{
				WarningThreshold:  40,
				CriticalThreshold: 10,
			},
		},
		metrics: map[string]*SLAMetrics{
			sloName: {
				SLOName:     sloName,
				Target:      95,
				Status:      SLAStatusHealthy,
				ErrorBudget: 100,
				History:     []MetricDataPoint{},
				Violations:  []SLAViolation{},
			},
		},
		alerts: NewAlertManager(),
		collectors: map[string]MetricCollector{
			sloName: &staticCollector{
				points: []MetricDataPoint{
					{Timestamp: now, Value: 0},
					{Timestamp: now, Value: 0},
				},
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sm.collectAndProcessMetrics(ctx)

	select {
	case alert := <-sm.alerts.GetAlertChannel():
		if alert.Metadata["slo_name"] != sloName {
			t.Fatalf("expected alert metadata slo_name=%s, got %v", sloName, alert.Metadata)
		}
		if alert.Severity != SeverityCritical {
			t.Fatalf("expected critical severity, got %s", alert.Severity)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected violation alert to be emitted")
	}
}
