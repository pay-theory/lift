package monitoring

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestSLAMonitor_StartAndGetters(t *testing.T) {
	cfg := SLAConfig{
		ApplicationName: "app",
		Environment:     "test",
		SLOs: []SLO{
			{Name: "availability", Type: SLOAvailability, Target: 99.0, Window: time.Hour, Enabled: true},
			{Name: "disabled", Type: SLOAvailability, Target: 99.0, Window: time.Hour, Enabled: false},
		},
		Reporting: ReportConfig{
			Enabled:    true,
			Frequency:  time.Hour,
			Recipients: []string{"ops@example.com"},
		},
		Thresholds: ThresholdConfig{
			WarningThreshold:  40,
			CriticalThreshold: 10,
		},
	}

	sm := NewSLAMonitor(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := sm.Start(ctx); err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}

	sm.RegisterCollector("availability", &staticCollector{
		points: []MetricDataPoint{{Timestamp: time.Now(), Value: 1}},
	})
	if len(sm.collectors) != 1 {
		t.Fatalf("expected 1 collector, got %d", len(sm.collectors))
	}

	metrics := sm.GetMetrics()
	if _, ok := metrics["availability"]; !ok {
		t.Fatal("expected enabled SLO metrics to be initialized")
	}
	if _, ok := metrics["disabled"]; ok {
		t.Fatal("did not expect disabled SLO metrics to be initialized")
	}

	if _, err := sm.GetSLOStatus("missing"); err == nil {
		t.Fatal("expected error for missing SLO status")
	}

	status, err := sm.GetSLOStatus("availability")
	if err != nil {
		t.Fatalf("unexpected error getting SLO status: %v", err)
	}
	status.Status = SLAStatusCritical
	again, err := sm.GetSLOStatus("availability")
	if err != nil {
		t.Fatalf("unexpected error getting SLO status: %v", err)
	}
	if again.Status == SLAStatusCritical {
		t.Fatal("expected GetSLOStatus to return a copy, not a pointer to internal state")
	}

	// Cover background loops' ctx.Done branches without waiting for tickers.
	sm.startMetricCollection(ctx)
	sm.startReportGeneration(ctx)
	sm.startAlertProcessing(ctx)
}

func TestSLAMonitor_GenerateReportAndReportGenerator(t *testing.T) {
	sm := NewSLAMonitor(SLAConfig{
		Reporting: ReportConfig{
			Enabled:    true,
			Frequency:  time.Hour,
			Recipients: []string{"a@example.com", "b@example.com"},
		},
	})
	sm.metrics["healthy"] = &SLAMetrics{SLOName: "healthy", Status: SLAStatusHealthy}
	sm.metrics["warn"] = &SLAMetrics{SLOName: "warn", Status: SLAStatusWarning}
	sm.metrics["crit"] = &SLAMetrics{SLOName: "crit", Status: SLAStatusCritical}
	sm.metrics["viol"] = &SLAMetrics{SLOName: "viol", Status: SLAStatusViolated}

	report := sm.reports.GenerateSLAReport(sm.GetMetrics())
	var decoded map[string]any
	if err := json.Unmarshal(report, &decoded); err != nil {
		t.Fatalf("expected report to be valid JSON, got %v", err)
	}

	sm.generateReport(context.Background())
}

func TestSLAMonitor_AlertProcessing_MatchesAndExecutesActions(t *testing.T) {
	sm := NewSLAMonitor(SLAConfig{
		AlertRules: []AlertRule{
			{
				Name:    "disabled",
				SLOName: "availability",
				Enabled: false,
			},
			{
				Name:    "mismatch",
				SLOName: "latency",
				Enabled: true,
			},
			{
				Name:    "match",
				SLOName: "availability",
				Enabled: true,
				Actions: []AlertAction{
					{Type: "email"},
					{Type: "slack"},
					{Type: "webhook"},
				},
			},
		},
	})

	alert := Alert{
		ID:        "a1",
		Type:      "sla_violation",
		Timestamp: time.Now(),
		Severity:  SeverityWarning,
		Message:   "violation",
		Metadata:  map[string]any{"slo_name": "availability"},
	}

	sm.processAlert(context.Background(), alert)

	if sm.matchesAlertRule(Alert{Type: "other"}, sm.config.AlertRules[2]) {
		t.Fatal("did not expect non-sla_violation alert to match")
	}
	if sm.matchesAlertRule(Alert{Type: "sla_violation", Metadata: map[string]any{"slo_name": 123}}, sm.config.AlertRules[2]) {
		t.Fatal("did not expect non-string slo_name to match")
	}

	// Exercise the receive path in startAlertProcessing.
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		sm.startAlertProcessing(ctx)
		close(done)
	}()

	sm.alerts.TriggerAlert(alert)

	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(sm.alerts.alertChannel) == 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected alert processing loop to stop after context cancelation")
	}
}

func TestSLAMonitor_calculateSLOValue_AllTypes(t *testing.T) {
	sm := &SLAMonitor{}
	points := []MetricDataPoint{
		{Timestamp: time.Now(), Value: 1},
		{Timestamp: time.Now(), Value: 2},
	}

	if got := sm.calculateSLOValue(SLOAvailability, points); got <= 0 {
		t.Fatalf("expected availability > 0, got %v", got)
	}
	if got := sm.calculateSLOValue(SLOLatency, points); got != 1.5 {
		t.Fatalf("expected latency percentile 1.5, got %v", got)
	}
	if got := sm.calculateSLOValue(SLOErrorRate, points); got <= 0 {
		t.Fatalf("expected error rate > 0, got %v", got)
	}
	if got := sm.calculateSLOValue(SLOThroughput, points); got != 3 {
		t.Fatalf("expected throughput 3, got %v", got)
	}
	if got := sm.calculateSLOValue(SLOType("unknown"), points); got != 0 {
		t.Fatalf("expected unknown SLO type to return 0, got %v", got)
	}
	if got := sm.calculateSLOValue(SLOAvailability, nil); got != 0 {
		t.Fatalf("expected empty datapoints to return 0, got %v", got)
	}
}

func TestAlertManager_DropsWhenFull(t *testing.T) {
	am := NewAlertManager()
	for i := 0; i < cap(am.alertChannel); i++ {
		am.alertChannel <- Alert{ID: "seed"}
	}
	if len(am.alertChannel) != cap(am.alertChannel) {
		t.Fatalf("expected channel to be full, got len=%d cap=%d", len(am.alertChannel), cap(am.alertChannel))
	}

	am.TriggerAlert(Alert{ID: "overflow"})
	if len(am.alertChannel) != cap(am.alertChannel) {
		t.Fatalf("expected overflow alert to be dropped, got len=%d cap=%d", len(am.alertChannel), cap(am.alertChannel))
	}
}
