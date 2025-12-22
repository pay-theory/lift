package alerting

import (
	"context"
	"sync"
	"testing"
	"time"
)

type stubChannel struct {
	mu    sync.Mutex
	alert *Alert
	ch    chan *Alert
}

func newStubChannel() *stubChannel {
	return &stubChannel{
		ch: make(chan *Alert, 1),
	}
}

func (s *stubChannel) Send(_ context.Context, alert *Alert) error {
	s.mu.Lock()
	s.alert = alert
	s.mu.Unlock()
	s.ch <- alert
	return nil
}

func (s *stubChannel) Validate() error {
	return nil
}

func (s *stubChannel) GetType() ChannelType {
	return ChannelTypeEmail
}

func (s *stubChannel) GetConfig() map[string]any {
	return map[string]any{}
}

type stubProcessor struct {
	id string
}

func (s *stubProcessor) Process(_ context.Context, alert *Alert) (*Alert, error) {
	alert.ID = s.id
	return alert, nil
}

func (s *stubProcessor) GetPriority() int {
	return 0
}

type stubEscalator struct {
	should bool
	called chan *Alert
}

func newStubEscalator(should bool) *stubEscalator {
	return &stubEscalator{
		should: should,
		called: make(chan *Alert, 1),
	}
}

func (s *stubEscalator) ShouldEscalate(*Alert) bool {
	return s.should
}

func (s *stubEscalator) Escalate(_ context.Context, alert *Alert) error {
	s.called <- alert
	return nil
}

func (s *stubEscalator) GetEscalationLevels() []EscalationLevel {
	return nil
}

func TestTriggerAlertRuleMatching(t *testing.T) {
	config := AlertManagerConfig{
		Enabled:        true,
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
		Description: "CPU usage exceeded threshold",
		Severity:    AlertSeverityWarning,
		Priority:    AlertPriorityHigh,
		Enabled:     true,
		Annotations: map[string]string{"summary": "cpu alert"},
		Actions: []AlertAction{
			{
				Channel: "primary",
				Type:    ActionTypeEmail,
				Enabled: true,
			},
		},
		Conditions: []AlertCondition{
			{Threshold: 80},
		},
	}

	if err := am.AddRule(rule); err != nil {
		t.Fatalf("failed to add rule: %v", err)
	}

	ctx := context.Background()
	if err := am.TriggerAlert(ctx, rule.ID, 95, map[string]string{"service": "api"}); err != nil {
		t.Fatalf("expected alert to trigger, got error: %v", err)
	}

	select {
	case alert := <-channel.ch:
		if alert.RuleID != rule.ID {
			t.Fatalf("expected rule ID %s, got %s", rule.ID, alert.RuleID)
		}
		if alert.Value != 95 {
			t.Fatalf("expected alert value 95, got %.2f", alert.Value)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected alert to be delivered to channel")
	}

	if err := am.TriggerAlert(ctx, "missing-rule", 42, nil); err == nil {
		t.Fatal("expected error for missing rule")
	}
}

func TestSuppressionConditions(t *testing.T) {
	am := NewAlertManager(AlertManagerConfig{})

	rule := &AlertRule{
		Suppression: &SuppressionRule{
			Enabled: true,
			Conditions: []SuppressionCondition{
				{
					Label:    "region",
					Operator: "eq",
					Value:    "us-west-2",
				},
			},
		},
	}

	if !am.shouldSuppress(rule, map[string]string{"region": "us-west-2"}) {
		t.Fatal("expected suppression when labels match")
	}

	rule.Suppression.Conditions = []SuppressionCondition{
		{
			Label:    "environment",
			Operator: "ne",
			Value:    "production",
		},
	}

	if !am.shouldSuppress(rule, map[string]string{"environment": "staging"}) {
		t.Fatal("expected suppression when 'ne' condition satisfied")
	}

	if am.shouldSuppress(rule, map[string]string{"environment": "production"}) {
		t.Fatal("did not expect suppression when condition not met")
	}
}

func TestDeduplicationReplacesAlert(t *testing.T) {
	config := AlertManagerConfig{
		Enabled:        true,
		MaxHistorySize: 10,
		AlertTimeout:   time.Minute,
		FlushInterval:  time.Minute,
	}
	am := NewAlertManager(config)
	am.AddProcessor(&stubProcessor{id: "dedupe"})

	rule := &AlertRule{
		ID:       "disk-usage",
		Name:     "Disk Usage High",
		Severity: AlertSeverityError,
		Priority: AlertPriorityMedium,
		Enabled:  true,
	}

	if err := am.AddRule(rule); err != nil {
		t.Fatalf("failed to add rule: %v", err)
	}

	ctx := context.Background()
	if err := am.TriggerAlert(ctx, rule.ID, 70, nil); err != nil {
		t.Fatalf("first trigger failed: %v", err)
	}

	if err := am.TriggerAlert(ctx, rule.ID, 85, nil); err != nil {
		t.Fatalf("second trigger failed: %v", err)
	}

	active := am.GetActiveAlerts()
	if len(active) != 1 {
		t.Fatalf("expected single active alert after deduplication, got %d", len(active))
	}

	alert, ok := active["dedupe"]
	if !ok {
		t.Fatalf("expected deduped alert with ID 'dedupe', keys: %v", active)
	}

	if alert.Value != 85 {
		t.Fatalf("expected latest alert value 85, got %.2f", alert.Value)
	}
}

func TestEscalationInvocation(t *testing.T) {
	config := AlertManagerConfig{
		Enabled:        true,
		MaxHistorySize: 5,
		AlertTimeout:   time.Minute,
		FlushInterval:  time.Minute,
	}
	am := NewAlertManager(config)

	escalator := newStubEscalator(true)
	am.SetEscalator(escalator)

	am.mu.Lock()
	am.activeAlerts["alert-1"] = &Alert{ID: "alert-1"}
	am.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	am.processEscalations(ctx)

	select {
	case alert := <-escalator.called:
		if alert.ID != "alert-1" {
			t.Fatalf("expected escalation for alert-1, got %s", alert.ID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected escalator to be invoked")
	}
}
