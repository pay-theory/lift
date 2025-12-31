package disaster

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDisasterRecoveryManager_validateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  DRConfig
		wantErr error
	}{
		{
			name: "invalid health interval when enabled",
			config: DRConfig{
				PrimaryRegion: "us-east-1",
				BackupRegions: []string{"us-west-2"},
				HealthCheck: HealthCheckConfig{
					Enabled:  true,
					Interval: 0,
				},
			},
			wantErr: errInvalidHealthInterval,
		},
		{
			name: "invalid testing frequency when enabled",
			config: DRConfig{
				PrimaryRegion: "us-east-1",
				BackupRegions: []string{"us-west-2"},
				TestingSchedule: TestingScheduleConfig{
					Enabled:      true,
					Frequency:    0,
					NotifyBefore: 0,
				},
			},
			wantErr: errInvalidTestingFrequency,
		},
		{
			name: "invalid notify lead time when negative",
			config: DRConfig{
				PrimaryRegion: "us-east-1",
				BackupRegions: []string{"us-west-2"},
				TestingSchedule: TestingScheduleConfig{
					Enabled:      true,
					Frequency:    time.Minute,
					NotifyBefore: -1 * time.Second,
				},
			},
			wantErr: errInvalidNotifyLeadTime,
		},
		{
			name: "invalid notify lead time when >= frequency",
			config: DRConfig{
				PrimaryRegion: "us-east-1",
				BackupRegions: []string{"us-west-2"},
				TestingSchedule: TestingScheduleConfig{
					Enabled:      true,
					Frequency:    time.Minute,
					NotifyBefore: time.Minute,
				},
			},
			wantErr: errInvalidNotifyLeadTime,
		},
		{
			name: "valid when disabled sections have zero values",
			config: DRConfig{
				PrimaryRegion: "us-east-1",
				BackupRegions: []string{"us-west-2"},
				HealthCheck: HealthCheckConfig{
					Enabled:  false,
					Interval: 0,
				},
				TestingSchedule: TestingScheduleConfig{
					Enabled:      false,
					Frequency:    0,
					NotifyBefore: 0,
				},
			},
			wantErr: nil,
		},
		{
			name: "valid schedule when notify_before < frequency",
			config: DRConfig{
				PrimaryRegion: "us-east-1",
				BackupRegions: []string{"us-west-2"},
				TestingSchedule: TestingScheduleConfig{
					Enabled:      true,
					Frequency:    time.Minute,
					NotifyBefore: 0,
				},
			},
			wantErr: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			drm := NewDisasterRecoveryManager(DRConfig{
				PrimaryRegion: "us-east-1",
				BackupRegions: []string{"us-west-2"},
			})
			drm.config = tc.config
			err := drm.validateConfig()

			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				return
			}

			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected error %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestDisasterRecoveryManager_StartMonitoring_Valid(t *testing.T) {
	drm := NewDisasterRecoveryManager(DRConfig{
		PrimaryRegion: "us-east-1",
		BackupRegions: []string{"us-west-2"},
		HealthCheck: HealthCheckConfig{
			Enabled: false,
		},
		DataReplication: DataReplicationConfig{
			Enabled: false,
		},
		TestingSchedule: TestingScheduleConfig{
			Enabled:      true,
			Frequency:    time.Millisecond,
			NotifyBefore: 0,
		},
		Notifications: NotificationConfig{Enabled: false},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := drm.StartMonitoring(ctx); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	cancel()
	time.Sleep(5 * time.Millisecond)
}

func TestDisasterRecoveryManager_generateFailoverSteps(t *testing.T) {
	drm := NewDisasterRecoveryManager(DRConfig{
		PrimaryRegion: "us-east-1",
		BackupRegions: []string{"us-west-2"},
	})

	steps := drm.generateFailoverSteps("us-west-2")
	wantIDs := []string{
		"validate_target",
		"stop_traffic",
		"sync_data",
		"activate_standby",
		"update_dns",
		"start_traffic",
		"verify_health",
	}
	if len(steps) != len(wantIDs) {
		t.Fatalf("expected %d steps, got %d", len(wantIDs), len(steps))
	}
	for i, want := range wantIDs {
		if steps[i].ID != want {
			t.Fatalf("expected step[%d].ID=%q, got %q", i, want, steps[i].ID)
		}
		if steps[i].Status != StepStatusPending {
			t.Fatalf("expected step[%d].Status=%q, got %q", i, StepStatusPending, steps[i].Status)
		}
	}
}

func TestDisasterRecoveryManager_executeFailover_Success(t *testing.T) {
	drm := NewDisasterRecoveryManager(DRConfig{
		PrimaryRegion: "us-east-1",
		BackupRegions: []string{"us-west-2", "eu-west-1"},
		Notifications: NotificationConfig{Enabled: true},
	})

	event := &FailoverEvent{
		ID:         "test-success",
		Timestamp:  time.Now(),
		Type:       FailoverTypeManual,
		FromRegion: "us-east-1",
		ToRegion:   "us-west-2",
		Reason:     "unit test",
		Trigger:    TriggerManualRequest,
		Status:     FailoverStatusInProgress,
		Steps: []FailoverStep{
			{ID: "sync_data", Name: "Synchronize Data", Status: StepStatusPending},
			{ID: "verify_health", Name: "Verify Health", Status: StepStatusPending},
		},
	}

	got, err := drm.executeFailover(context.Background(), event)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got.Status != FailoverStatusCompleted {
		t.Fatalf("expected status %q, got %q", FailoverStatusCompleted, got.Status)
	}

	state := drm.GetCurrentState()
	if state.ActiveRegion != "us-west-2" {
		t.Fatalf("expected active region to change to %q, got %q", "us-west-2", state.ActiveRegion)
	}
	if state.Status != DRStatusNormal {
		t.Fatalf("expected state status %q, got %q", DRStatusNormal, state.Status)
	}
	if len(state.StandbyRegions) != 2 {
		t.Fatalf("expected 2 standby regions, got %d", len(state.StandbyRegions))
	}

	history := drm.GetFailoverHistory()
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}
	if history[0].ID != "test-success" {
		t.Fatalf("expected history[0].ID=%q, got %q", "test-success", history[0].ID)
	}

	metrics := drm.GetMetrics()
	if metrics.TotalFailovers != 1 || metrics.SuccessfulFailovers != 1 || metrics.FailedFailovers != 0 {
		t.Fatalf("unexpected metrics: %+v", metrics)
	}

	if got.Impact.DowntimeDuration != got.Duration {
		t.Fatalf("expected impact downtime to match duration, got %v vs %v", got.Impact.DowntimeDuration, got.Duration)
	}
}

func TestDisasterRecoveryManager_executeFailover_FailureUnknownStep(t *testing.T) {
	drm := NewDisasterRecoveryManager(DRConfig{
		PrimaryRegion: "us-east-1",
		BackupRegions: []string{"us-west-2"},
		Notifications: NotificationConfig{Enabled: false},
	})

	event := &FailoverEvent{
		ID:         "test-failure",
		Timestamp:  time.Now(),
		Type:       FailoverTypeManual,
		FromRegion: "us-east-1",
		ToRegion:   "us-west-2",
		Reason:     "unit test",
		Trigger:    TriggerManualRequest,
		Status:     FailoverStatusInProgress,
		Steps: []FailoverStep{
			{ID: "unknown_step", Name: "Unknown Step", Status: StepStatusPending},
		},
	}

	got, err := drm.executeFailover(context.Background(), event)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got.Status != FailoverStatusFailed {
		t.Fatalf("expected status %q, got %q", FailoverStatusFailed, got.Status)
	}
	if got.Steps[0].Status != StepStatusFailed {
		t.Fatalf("expected step status %q, got %q", StepStatusFailed, got.Steps[0].Status)
	}
	if got.Steps[0].Error == "" {
		t.Fatal("expected failed step to have error message")
	}

	history := drm.GetFailoverHistory()
	if len(history) != 1 || history[0].ID != "test-failure" {
		t.Fatalf("unexpected history: %+v", history)
	}

	metrics := drm.GetMetrics()
	if metrics.TotalFailovers != 1 || metrics.SuccessfulFailovers != 0 || metrics.FailedFailovers != 1 {
		t.Fatalf("unexpected metrics: %+v", metrics)
	}
}

func TestDisasterRecoveryManager_executeFailoverStep_SupportedAndUnknown(t *testing.T) {
	drm := NewDisasterRecoveryManager(DRConfig{
		PrimaryRegion: "us-east-1",
		BackupRegions: []string{"us-west-2"},
	})

	if err := drm.executeFailoverStep(context.Background(), &FailoverStep{ID: "sync_data"}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if err := drm.executeFailoverStep(context.Background(), &FailoverStep{ID: "verify_health"}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if err := drm.executeFailoverStep(context.Background(), &FailoverStep{ID: "not_a_real_step"}); err == nil {
		t.Fatal("expected error for unknown step, got nil")
	}
}

func TestDisasterRecoveryManager_shouldRollbackAndRollbackFailover(t *testing.T) {
	drm := NewDisasterRecoveryManager(DRConfig{
		PrimaryRegion: "us-east-1",
		BackupRegions: []string{"us-west-2"},
	})

	if !drm.shouldRollback(nil, &FailoverStep{ID: "activate_standby"}) {
		t.Fatal("expected activate_standby to be treated as critical")
	}
	if !drm.shouldRollback(nil, &FailoverStep{ID: "update_dns"}) {
		t.Fatal("expected update_dns to be treated as critical")
	}
	if !drm.shouldRollback(nil, &FailoverStep{ID: "start_traffic"}) {
		t.Fatal("expected start_traffic to be treated as critical")
	}
	if drm.shouldRollback(nil, &FailoverStep{ID: "sync_data"}) {
		t.Fatal("expected sync_data to not be treated as critical")
	}

	drm.currentState.Status = DRStatusFailover
	event := &FailoverEvent{ID: "rollback-me", Status: FailoverStatusInProgress}
	got, err := drm.rollbackFailover(context.Background(), event)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got.Status != FailoverStatusRolledBack {
		t.Fatalf("expected status %q, got %q", FailoverStatusRolledBack, got.Status)
	}
	if drm.GetCurrentState().Status != DRStatusNormal {
		t.Fatalf("expected manager state %q after rollback, got %q", DRStatusNormal, drm.GetCurrentState().Status)
	}
}

func TestDisasterRecoveryManager_handleSyncEvent_UpdatesStateAndDetectsRPOViolation(t *testing.T) {
	drm := NewDisasterRecoveryManager(DRConfig{
		PrimaryRegion: "us-east-1",
		BackupRegions: []string{"us-west-2"},
		RPO:           time.Second,
		Notifications: NotificationConfig{Enabled: true},
	})

	now := time.Now()
	drm.handleSyncEvent(context.Background(), SyncEvent{
		Timestamp:      now,
		Status:         SyncStatusInSync,
		ReplicationLag: 500 * time.Millisecond,
		TablesInSync:   map[string]bool{"users": true},
		BucketsInSync:  map[string]bool{"assets": true},
		Errors:         nil,
	})

	state := drm.GetCurrentState()
	if state.DataSyncStatus.Status != SyncStatusInSync {
		t.Fatalf("expected data sync status %q, got %q", SyncStatusInSync, state.DataSyncStatus.Status)
	}
	if state.DataSyncStatus.ReplicationLag != 500*time.Millisecond {
		t.Fatalf("expected replication lag %v, got %v", 500*time.Millisecond, state.DataSyncStatus.ReplicationLag)
	}

	drm.handleSyncEvent(context.Background(), SyncEvent{
		Timestamp:      now.Add(time.Second),
		Status:         SyncStatusOutOfSync,
		ReplicationLag: 2 * time.Second,
		TablesInSync:   map[string]bool{"users": false},
		BucketsInSync:  map[string]bool{"assets": false},
		Errors:         []SyncError{{Timestamp: now, Resource: "users", Error: "replication lag", Retries: 1}},
	})

	state = drm.GetCurrentState()
	if state.DataSyncStatus.Status != SyncStatusOutOfSync {
		t.Fatalf("expected data sync status %q, got %q", SyncStatusOutOfSync, state.DataSyncStatus.Status)
	}
	if state.DataSyncStatus.ReplicationLag != 2*time.Second {
		t.Fatalf("expected replication lag %v, got %v", 2*time.Second, state.DataSyncStatus.ReplicationLag)
	}
	if len(state.DataSyncStatus.SyncErrors) != 1 {
		t.Fatalf("expected 1 sync error, got %d", len(state.DataSyncStatus.SyncErrors))
	}
}

func TestDisasterRecoveryManager_selectBestBackupRegion(t *testing.T) {
	drm := NewDisasterRecoveryManager(DRConfig{
		PrimaryRegion: "us-east-1",
		BackupRegions: []string{"us-west-2", "eu-west-1"},
	})

	drm.currentState.StandbyRegions = []string{"us-west-2", "eu-west-1"}
	drm.currentState.RegionHealth["us-west-2"] = RegionHealth{
		Region:       "us-west-2",
		Status:       HealthStatusHealthy,
		Availability: 99.9,
		ResponseTime: 10 * time.Millisecond,
	}
	drm.currentState.RegionHealth["eu-west-1"] = RegionHealth{
		Region:       "eu-west-1",
		Status:       HealthStatusHealthy,
		Availability: 99.95,
		ResponseTime: 2 * time.Second,
	}

	best := drm.selectBestBackupRegion()
	if best != "us-west-2" {
		t.Fatalf("expected best region %q, got %q", "us-west-2", best)
	}

	// No healthy candidates.
	drm.currentState.RegionHealth["us-west-2"] = RegionHealth{Region: "us-west-2", Status: HealthStatusUnhealthy}
	drm.currentState.RegionHealth["eu-west-1"] = RegionHealth{Region: "eu-west-1", Status: HealthStatusUnknown}
	if best := drm.selectBestBackupRegion(); best != "" {
		t.Fatalf("expected empty best region, got %q", best)
	}

	// Healthy but negative score should not be selected (bestScore starts at 0).
	drm.currentState.RegionHealth["us-west-2"] = RegionHealth{
		Region:       "us-west-2",
		Status:       HealthStatusHealthy,
		Availability: 0,
		ResponseTime: 5 * time.Second,
	}
	drm.currentState.RegionHealth["eu-west-1"] = RegionHealth{
		Region:       "eu-west-1",
		Status:       HealthStatusHealthy,
		Availability: 0,
		ResponseTime: 5 * time.Second,
	}
	if best := drm.selectBestBackupRegion(); best != "" {
		t.Fatalf("expected empty best region, got %q", best)
	}
}

func TestDisasterRecoveryManager_startPeriodicTesting_TicksAndStops(t *testing.T) {
	drm := NewDisasterRecoveryManager(DRConfig{
		PrimaryRegion: "us-east-1",
		BackupRegions: []string{"us-west-2"},
		TestingSchedule: TestingScheduleConfig{
			Enabled:      true,
			Frequency:    time.Millisecond,
			NotifyBefore: 0,
		},
		Notifications: NotificationConfig{Enabled: false},
	})

	drm.currentState.RegionHealth["us-west-2"] = RegionHealth{
		Region:       "us-west-2",
		Status:       HealthStatusHealthy,
		Availability: 99.9,
		ResponseTime: 10 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		drm.startPeriodicTesting(ctx)
		close(done)
	}()

	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(drm.GetFailoverHistory()) > 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if len(drm.GetFailoverHistory()) == 0 {
		cancel()
		<-done
		t.Fatal("expected periodic testing to create at least one failover history entry")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected periodic testing loop to stop after context cancelation")
	}
}

func TestDisasterRecoveryManager_performDRTest_WaitsNotifyBeforeThenRecordsTestFailover(t *testing.T) {
	drm := NewDisasterRecoveryManager(DRConfig{
		PrimaryRegion: "us-east-1",
		BackupRegions: []string{"us-west-2"},
		TestingSchedule: TestingScheduleConfig{
			Enabled:      true,
			Frequency:    time.Minute,
			NotifyBefore: 5 * time.Millisecond,
		},
		Notifications: NotificationConfig{Enabled: false},
	})

	drm.currentState.RegionHealth["us-west-2"] = RegionHealth{
		Region:       "us-west-2",
		Status:       HealthStatusHealthy,
		Availability: 99.9,
		ResponseTime: 10 * time.Millisecond,
	}

	drm.performDRTest(context.Background())

	history := drm.GetFailoverHistory()
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}
	if history[0].Type != FailoverTypeTesting {
		t.Fatalf("expected testing failover, got %q", history[0].Type)
	}
	if history[0].Duration != 5*time.Minute {
		t.Fatalf("expected test duration %v, got %v", 5*time.Minute, history[0].Duration)
	}
}

func TestDisasterRecoveryManager_ExportImportConfigurationAndGetters(t *testing.T) {
	drm := NewDisasterRecoveryManager(DRConfig{
		Environment:   "dev",
		PrimaryRegion: "us-east-1",
		BackupRegions: []string{"us-west-2"},
		RPO:           time.Second,
	})

	state := drm.GetCurrentState()
	if state.ActiveRegion != "us-east-1" {
		t.Fatalf("expected active region %q, got %q", "us-east-1", state.ActiveRegion)
	}

	metrics := drm.GetMetrics()
	if metrics.LastUpdated.IsZero() {
		t.Fatal("expected metrics to have a LastUpdated timestamp")
	}

	out, err := drm.ExportConfiguration()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	var decoded DRConfig
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("expected valid json, got %v", err)
	}
	if decoded.PrimaryRegion != "us-east-1" {
		t.Fatalf("expected primary region to round-trip, got %q", decoded.PrimaryRegion)
	}

	if err := drm.ImportConfiguration([]byte("not-json")); err == nil {
		t.Fatal("expected import error, got nil")
	} else if !strings.Contains(err.Error(), "failed to unmarshal configuration") {
		t.Fatalf("expected wrapped unmarshal error, got %v", err)
	}

	newConfig := DRConfig{PrimaryRegion: "eu-central-1", BackupRegions: []string{"us-east-1"}}
	newBytes, err := json.Marshal(newConfig)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}
	if err := drm.ImportConfiguration(newBytes); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if drm.config.PrimaryRegion != "eu-central-1" {
		t.Fatalf("expected imported primary region %q, got %q", "eu-central-1", drm.config.PrimaryRegion)
	}
}

func TestDisasterRecoveryManager_GetFailoverHistory_ReturnsCopy(t *testing.T) {
	drm := NewDisasterRecoveryManager(DRConfig{
		PrimaryRegion: "us-east-1",
		BackupRegions: []string{"us-west-2"},
	})

	drm.failoverHistory = []FailoverEvent{{ID: "a"}, {ID: "b"}}

	got := drm.GetFailoverHistory()
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
	got[0].ID = "changed"
	got = append(got, FailoverEvent{ID: "c"})

	history := drm.GetFailoverHistory()
	if len(history) != 2 {
		t.Fatalf("expected internal history to remain length 2, got %d", len(history))
	}
	if history[0].ID != "a" {
		t.Fatalf("expected internal history[0].ID=%q, got %q", "a", history[0].ID)
	}
}

func TestDisasterRecoveryManager_TriggerFailover_AlreadyInProgress(t *testing.T) {
	drm := NewDisasterRecoveryManager(DRConfig{
		PrimaryRegion: "us-east-1",
		BackupRegions: []string{"us-west-2"},
	})

	drm.currentState.Status = DRStatusFailover
	if _, err := drm.TriggerFailover(context.Background(), "us-west-2", "unit test"); err == nil {
		t.Fatal("expected error when failover already in progress, got nil")
	}
}

func TestDisasterRecoveryManager_TriggerFailover_Success(t *testing.T) {
	previousValidate := validateTargetRegionDelay
	previousStopTraffic := stopTrafficToPrimaryDelay
	previousActivate := activateStandbyRegionDelay
	previousUpdateDNS := updateDNSRecordsDelay
	previousStartTraffic := startTrafficToNewPrimaryDelay

	validateTargetRegionDelay = 0
	stopTrafficToPrimaryDelay = 0
	activateStandbyRegionDelay = 0
	updateDNSRecordsDelay = 0
	startTrafficToNewPrimaryDelay = 0
	t.Cleanup(func() {
		validateTargetRegionDelay = previousValidate
		stopTrafficToPrimaryDelay = previousStopTraffic
		activateStandbyRegionDelay = previousActivate
		updateDNSRecordsDelay = previousUpdateDNS
		startTrafficToNewPrimaryDelay = previousStartTraffic
	})

	drm := NewDisasterRecoveryManager(DRConfig{
		PrimaryRegion: "us-east-1",
		BackupRegions: []string{"us-west-2"},
		Notifications: NotificationConfig{Enabled: false},
	})

	event, err := drm.TriggerFailover(context.Background(), "us-west-2", "unit test")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if event.Status != FailoverStatusCompleted {
		t.Fatalf("expected event status %q, got %q", FailoverStatusCompleted, event.Status)
	}
	if len(event.Steps) != 7 {
		t.Fatalf("expected 7 failover steps, got %d", len(event.Steps))
	}
	for i, step := range event.Steps {
		if step.Status != StepStatusCompleted {
			t.Fatalf("expected step[%d] status %q, got %q", i, StepStatusCompleted, step.Status)
		}
	}

	state := drm.GetCurrentState()
	if state.ActiveRegion != "us-west-2" {
		t.Fatalf("expected active region %q, got %q", "us-west-2", state.ActiveRegion)
	}
	if state.Status != DRStatusNormal {
		t.Fatalf("expected state status %q, got %q", DRStatusNormal, state.Status)
	}
	if len(state.StandbyRegions) != 1 || state.StandbyRegions[0] != "us-east-1" {
		t.Fatalf("expected standby regions to contain %q, got %#v", "us-east-1", state.StandbyRegions)
	}
}
