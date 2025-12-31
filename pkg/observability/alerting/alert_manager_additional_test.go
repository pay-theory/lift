package alerting

import (
	"context"
	"errors"
	"testing"
	"time"
)

type invalidChannel struct{}

func (invalidChannel) Send(context.Context, *Alert) error { return nil }
func (invalidChannel) Validate() error                   { return errors.New("invalid channel") }
func (invalidChannel) GetType() ChannelType              { return ChannelTypeEmail }
func (invalidChannel) GetConfig() map[string]any         { return map[string]any{} }

type errorChannel struct {
	ch chan *Alert
}

func (c *errorChannel) Send(_ context.Context, alert *Alert) error {
	select {
	case c.ch <- alert:
	default:
	}
	return errors.New("send failed")
}

func (c *errorChannel) Validate() error           { return nil }
func (c *errorChannel) GetType() ChannelType      { return ChannelTypeEmail }
func (c *errorChannel) GetConfig() map[string]any { return map[string]any{} }

func TestAlertManager_StartStopAndRemove(t *testing.T) {
	config := AlertManagerConfig{
		Enabled:        true,
		MaxHistorySize: 10,
		AlertTimeout:   time.Minute,
		FlushInterval:  time.Millisecond,
	}
	am := NewAlertManager(config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := am.Start(ctx); err != nil {
		t.Fatalf("unexpected error starting alert manager: %v", err)
	}
	if err := am.Start(ctx); err == nil {
		t.Fatal("expected error starting already running alert manager")
	}
	if err := am.Stop(); err != nil {
		t.Fatalf("unexpected error stopping alert manager: %v", err)
	}
	if err := am.Stop(); err != nil {
		t.Fatalf("unexpected error on idempotent stop: %v", err)
	}

	disabled := NewAlertManager(AlertManagerConfig{Enabled: false})
	if err := disabled.Start(ctx); err == nil {
		t.Fatal("expected error starting disabled alert manager")
	}
}

func TestAlertManager_AddRuleValidationAndRemoveRule(t *testing.T) {
	am := NewAlertManager(AlertManagerConfig{
		Enabled:        true,
		MaxHistorySize: 10,
		AlertTimeout:   time.Minute,
		FlushInterval:  time.Minute,
	})

	if err := am.AddRule(&AlertRule{ID: ""}); err == nil {
		t.Fatal("expected error for empty rule ID")
	}

	rule := &AlertRule{
		ID:       "rule-1",
		Name:     "Rule 1",
		Enabled:  true,
		Severity: AlertSeverityWarning,
		Priority: AlertPriorityLow,
	}
	if err := am.AddRule(rule); err != nil {
		t.Fatalf("unexpected add rule error: %v", err)
	}
	if rule.CreatedAt.IsZero() || rule.UpdatedAt.IsZero() {
		t.Fatalf("expected CreatedAt/UpdatedAt timestamps to be set, got CreatedAt=%v UpdatedAt=%v", rule.CreatedAt, rule.UpdatedAt)
	}

	if err := am.RemoveRule(rule.ID); err != nil {
		t.Fatalf("unexpected remove rule error: %v", err)
	}
	if err := am.TriggerAlert(context.Background(), rule.ID, 1, nil); err == nil {
		t.Fatal("expected error triggering removed rule")
	}
}

func TestAlertManager_AddChannelValidationAndRemoveChannel(t *testing.T) {
	am := NewAlertManager(AlertManagerConfig{})

	if err := am.AddChannel("bad", invalidChannel{}); err == nil {
		t.Fatal("expected error adding invalid channel")
	}

	channel := newStubChannel()
	if err := am.AddChannel("ok", channel); err != nil {
		t.Fatalf("unexpected add channel error: %v", err)
	}
	am.RemoveChannel("ok")

	rule := &AlertRule{
		ID:       "rule",
		Name:     "Rule",
		Enabled:  true,
		Severity: AlertSeverityWarning,
		Priority: AlertPriorityLow,
		Actions: []AlertAction{
			{Channel: "ok", Type: ActionTypeEmail, Enabled: true},
		},
	}
	if err := am.AddRule(rule); err != nil {
		t.Fatalf("unexpected add rule error: %v", err)
	}
	if err := am.TriggerAlert(context.Background(), rule.ID, 1, nil); err != nil {
		t.Fatalf("unexpected trigger error: %v", err)
	}
}

func TestAlertManager_TriggerAlertSuppressed(t *testing.T) {
	am := NewAlertManager(AlertManagerConfig{
		MaxHistorySize: 10,
	})

	channel := newStubChannel()
	if err := am.AddChannel("primary", channel); err != nil {
		t.Fatalf("failed to add channel: %v", err)
	}

	rule := &AlertRule{
		ID:       "rule",
		Name:     "Rule",
		Enabled:  true,
		Severity: AlertSeverityWarning,
		Priority: AlertPriorityLow,
		Actions:  []AlertAction{{Channel: "primary", Type: ActionTypeEmail, Enabled: true}},
		Suppression: &SuppressionRule{
			Enabled: true,
			Conditions: []SuppressionCondition{
				{Label: "region", Operator: "eq", Value: "us-west-2"},
			},
		},
	}
	if err := am.AddRule(rule); err != nil {
		t.Fatalf("failed to add rule: %v", err)
	}

	if err := am.TriggerAlert(context.Background(), rule.ID, 1, map[string]string{"region": "us-west-2"}); err != nil {
		t.Fatalf("unexpected trigger error: %v", err)
	}
	if len(am.GetActiveAlerts()) != 0 {
		t.Fatal("expected suppressed alert to not be stored as active")
	}
}

func TestAlertManager_ResolveAlertAndHistory(t *testing.T) {
	config := AlertManagerConfig{
		MaxHistorySize: 10,
		AlertTimeout:   time.Minute,
		FlushInterval:  time.Minute,
	}
	am := NewAlertManager(config)

	channel := newStubChannel()
	if err := am.AddChannel("primary", channel); err != nil {
		t.Fatalf("failed to add channel: %v", err)
	}

	rule := &AlertRule{
		ID:          "cpu-high",
		Name:        "High CPU",
		Description: "CPU exceeded threshold",
		Enabled:     true,
		Severity:    AlertSeverityWarning,
		Priority:    AlertPriorityHigh,
		Actions: []AlertAction{
			{Channel: "primary", Type: ActionTypeEmail, Enabled: true},
		},
		Conditions: []AlertCondition{{Threshold: 80}},
	}
	if err := am.AddRule(rule); err != nil {
		t.Fatalf("failed to add rule: %v", err)
	}

	if err := am.TriggerAlert(context.Background(), rule.ID, 95, map[string]string{"service": "api"}); err != nil {
		t.Fatalf("trigger failed: %v", err)
	}

	active := am.GetActiveAlerts()
	if len(active) != 1 {
		t.Fatalf("expected 1 active alert, got %d", len(active))
	}

	var alertID string
	for id := range active {
		alertID = id
		break
	}

	if err := am.ResolveAlert(alertID, "resolved"); err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if len(am.GetActiveAlerts()) != 0 {
		t.Fatal("expected alert to be removed from active after resolution")
	}

	history := am.GetAlertHistory(10)
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}
	if history[0].Status != AlertStatusResolved {
		t.Fatalf("expected resolved status, got %q", history[0].Status)
	}
	if history[0].EndTime == nil || history[0].Duration <= 0 {
		t.Fatalf("expected EndTime and Duration to be set, got EndTime=%v Duration=%v", history[0].EndTime, history[0].Duration)
	}

	if err := am.ResolveAlert("missing", "nope"); err == nil {
		t.Fatal("expected error resolving missing alert")
	}

	limited := am.GetAlertHistory(1)
	if len(limited) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(limited))
	}
	all := am.GetAlertHistory(0)
	if len(all) != 1 {
		t.Fatalf("expected 1 history entry when limit<=0, got %d", len(all))
	}
}

func TestAlertManager_HistoryTrimming(t *testing.T) {
	am := NewAlertManager(AlertManagerConfig{MaxHistorySize: 2})
	am.addToHistory(Alert{ID: "a"})
	am.addToHistory(Alert{ID: "b"})
	am.addToHistory(Alert{ID: "c"})

	history := am.GetAlertHistory(10)
	if len(history) != 2 {
		t.Fatalf("expected trimmed history length 2, got %d", len(history))
	}
	if history[0].ID != "b" || history[1].ID != "c" {
		t.Fatalf("unexpected trimmed history order: %#v", history)
	}
}

func TestAlertManager_processAlertsAndCleanupExpired(t *testing.T) {
	config := AlertManagerConfig{
		MaxHistorySize: 10,
		AlertTimeout:   5 * time.Minute,
		FlushInterval:  time.Minute,
	}
	am := NewAlertManager(config)

	triggered := &Alert{
		ID:        "a1",
		StartTime:  time.Now().Add(-2 * time.Minute),
		Status:    AlertStatusPending,
		State:     AlertStateTriggered,
		Events:    []AlertEvent{},
		Labels:    map[string]string{},
		Metadata:  map[string]any{},
		Threshold: 1,
	}
	expired := &Alert{
		ID:        "a2",
		StartTime:  time.Now().Add(-6 * time.Minute),
		Status:    AlertStatusActive,
		State:     AlertStateFiring,
		Events:    []AlertEvent{},
		Labels:    map[string]string{},
		Metadata:  map[string]any{},
		Threshold: 1,
	}

	am.mu.Lock()
	am.activeAlerts[triggered.ID] = triggered
	am.activeAlerts[expired.ID] = expired
	am.mu.Unlock()

	am.processAlerts(context.Background())
	if triggered.State != AlertStateFiring || triggered.Status != AlertStatusActive {
		t.Fatalf("expected triggered alert to transition to firing/active, got state=%q status=%q", triggered.State, triggered.Status)
	}

	am.cleanupExpiredAlerts()
	if len(am.GetActiveAlerts()) != 1 {
		t.Fatalf("expected 1 active alert after cleanup, got %d", len(am.GetActiveAlerts()))
	}
	if _, ok := am.GetActiveAlerts()[expired.ID]; ok {
		t.Fatal("expected expired alert to be removed from active")
	}
	history := am.GetAlertHistory(10)
	if len(history) != 1 || history[0].Status != AlertStatusExpired {
		t.Fatalf("expected expired alert to be in history, got %#v", history)
	}
}

func TestAlertManager_GetMetricsAndProcessorLoopsExit(t *testing.T) {
	config := AlertManagerConfig{
		Enabled:        true,
		MaxHistorySize: 10,
		AlertTimeout:   time.Minute,
		FlushInterval:  time.Millisecond,
	}
	am := NewAlertManager(config)

	am.AddProcessor(&stubProcessor{id: "p1"})
	am.SetEscalator(newStubEscalator(false))
	if err := am.AddChannel("primary", newStubChannel()); err != nil {
		t.Fatalf("failed to add channel: %v", err)
	}
	if err := am.AddRule(&AlertRule{ID: "r1", Name: "r1", Enabled: true}); err != nil {
		t.Fatalf("failed to add rule: %v", err)
	}
	am.mu.Lock()
	am.activeAlerts["a1"] = &Alert{ID: "a1"}
	am.mu.Unlock()

	metrics := am.GetMetrics()
	if metrics.ActiveAlerts != 1 || metrics.TotalRules != 1 || metrics.TotalChannels != 1 || metrics.ProcessorsCount != 1 {
		t.Fatalf("unexpected metrics: %+v", metrics)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Ensure background processor loops can observe ctx.Done and exit.
	am.runAlertProcessor(ctx)
	am.runEscalationProcessor(ctx)
	am.runCleanupProcessor(ctx)
}

func TestAlertManager_executeActions_RecordsSendErrorEvent(t *testing.T) {
	am := NewAlertManager(AlertManagerConfig{MaxHistorySize: 10})
	ch := &errorChannel{ch: make(chan *Alert, 1)}
	if err := am.AddChannel("primary", ch); err != nil {
		t.Fatalf("failed to add channel: %v", err)
	}

	rule := &AlertRule{
		ID:      "rule",
		Enabled: true,
		Actions: []AlertAction{{Channel: "primary", Type: ActionTypeEmail, Enabled: true}},
	}
	alert := &Alert{ID: "a1", Events: []AlertEvent{}}

	if err := am.executeActions(context.Background(), alert, rule); err != nil {
		t.Fatalf("unexpected error executing actions: %v", err)
	}

	select {
	case <-ch.ch:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected channel Send to be invoked")
	}

	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		am.mu.RLock()
		eventCount := len(alert.Events)
		am.mu.RUnlock()

		if eventCount > 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("expected send failure to be recorded as an alert event")
}
