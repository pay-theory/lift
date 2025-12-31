package disaster

import (
	"context"
	"testing"
	"time"
)

func TestHealthMonitor_Start_EmitsEventAndStopsOnCancel(t *testing.T) {
	hm := NewHealthMonitor(HealthCheckConfig{
		Enabled:  true,
		Interval: time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan HealthEvent, 1)
	done := make(chan struct{})

	go func() {
		hm.Start(ctx, func(_ context.Context, event HealthEvent) {
			select {
			case events <- event:
			default:
			}
			cancel()
		})
		close(done)
	}()

	select {
	case <-events:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected health monitor to emit at least one event")
	}

	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected health monitor to stop after context cancelation")
	}
}

func TestHealthMonitor_Start_Disabled(t *testing.T) {
	hm := NewHealthMonitor(HealthCheckConfig{
		Enabled:  false,
		Interval: time.Millisecond,
	})

	called := false
	hm.Start(context.Background(), func(context.Context, HealthEvent) {
		called = true
	})

	if called {
		t.Fatal("expected disabled health monitor to not invoke event handler")
	}
}

func TestDataSynchronizer_StartMonitoring_Disabled(t *testing.T) {
	ds := NewDataSynchronizer(DataReplicationConfig{Enabled: false})

	called := false
	ds.StartMonitoring(context.Background(), func(context.Context, SyncEvent) {
		called = true
	})

	if called {
		t.Fatal("expected disabled data synchronizer to not invoke event handler")
	}
}

func TestDataSynchronizer_StartMonitoring_StopsOnCanceledContext(t *testing.T) {
	ds := NewDataSynchronizer(DataReplicationConfig{Enabled: true})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		ds.StartMonitoring(ctx, func(context.Context, SyncEvent) {})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected data synchronizer to stop on canceled context")
	}
}

func TestNotificationAndManagerStubs_ReturnNil(t *testing.T) {
	ctx := context.Background()

	nm := NewNotificationManager(NotificationConfig{Enabled: true})
	if err := nm.SendNotification(ctx, "event", map[string]any{"k": "v"}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	hm := NewHealthMonitor(HealthCheckConfig{})
	if err := hm.VerifyRegionHealth(ctx, "us-east-1"); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	ds := NewDataSynchronizer(DataReplicationConfig{})
	if err := ds.ForceSynchronization(ctx); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	rhc := NewRegionHealthChecker("us-east-1", HealthCheckConfig{})
	if err := rhc.CheckHealth(ctx, "https://example.com/health"); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	dm := NewDNSManager(DNSConfig{DomainName: "example.com", TTL: 60})
	if err := dm.UpdateRecords(ctx, []string{"us-east-1"}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	glb := NewGlobalLoadBalancer(LoadBalancingConfig{Type: "alb"})
	if err := glb.UpdateTrafficWeights(ctx, map[string]int{"us-east-1": 100}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

