package streamer

import (
	"testing"
	"time"
)

func TestConnectionInfo(t *testing.T) {
	now := time.Now()
	conn := ConnectionInfo{
		ConnectionID: "test-conn-123",
		ConnectedAt:  now.Add(-1 * time.Hour),
		LastActiveAt: now.Add(-5 * time.Minute),
		SourceIP:     "192.168.1.1",
		UserAgent:    "TestClient/1.0",
		Identity: map[string]any{
			"userId": "user-123",
		},
	}

	if conn.ConnectionID != "test-conn-123" {
		t.Errorf("expected connection ID 'test-conn-123', got %s", conn.ConnectionID)
	}

	if conn.SourceIP != "192.168.1.1" {
		t.Errorf("expected source IP '192.168.1.1', got %s", conn.SourceIP)
	}

	if conn.UserAgent != "TestClient/1.0" {
		t.Errorf("expected user agent 'TestClient/1.0', got %s", conn.UserAgent)
	}

	if userId, ok := conn.Identity["userId"]; !ok || userId != "user-123" {
		t.Errorf("expected identity userId 'user-123', got %s", userId)
	}
}

func TestConnectionInfoIsActive(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name         string
		conn         ConnectionInfo
		expectActive bool
	}{
		{
			name: "recently active connection",
			conn: ConnectionInfo{
				ConnectionID: "conn-1",
				ConnectedAt:  now.Add(-30 * time.Minute),
				LastActiveAt: now.Add(-5 * time.Minute),
			},
			expectActive: true,
		},
		{
			name: "stale connection (over 2 hours)",
			conn: ConnectionInfo{
				ConnectionID: "conn-2",
				ConnectedAt:  now.Add(-3 * time.Hour),
				LastActiveAt: now.Add(-3 * time.Hour),
			},
			expectActive: false,
		},
		{
			name: "just created connection",
			conn: ConnectionInfo{
				ConnectionID: "conn-3",
				ConnectedAt:  now,
				LastActiveAt: now,
			},
			expectActive: true,
		},
		{
			name: "connection at 2 hour boundary",
			conn: ConnectionInfo{
				ConnectionID: "conn-4",
				ConnectedAt:  now.Add(-2 * time.Hour),
				LastActiveAt: now.Add(-2 * time.Hour),
			},
			expectActive: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			active := tt.conn.IsActive()
			if active != tt.expectActive {
				t.Errorf("expected IsActive=%v, got %v", tt.expectActive, active)
			}
		})
	}
}

func TestConnectionInfoAge(t *testing.T) {
	now := time.Now()
	connectedAt := now.Add(-1 * time.Hour)

	conn := ConnectionInfo{
		ConnectionID: "conn-123",
		ConnectedAt:  connectedAt,
		LastActiveAt: now,
	}

	age := conn.Age()
	expectedAge := time.Since(connectedAt)

	// Allow for small timing differences (100ms)
	diff := age - expectedAge
	if diff < 0 {
		diff = -diff
	}
	if diff > 100*time.Millisecond {
		t.Errorf("expected age ~%v, got %v (diff: %v)", expectedAge, age, diff)
	}
}

func TestConnectionInfoIdleDuration(t *testing.T) {
	now := time.Now()
	lastActiveAt := now.Add(-10 * time.Minute)

	conn := ConnectionInfo{
		ConnectionID: "conn-123",
		ConnectedAt:  now.Add(-1 * time.Hour),
		LastActiveAt: lastActiveAt,
	}

	idle := conn.IdleDuration()
	expectedIdle := time.Since(lastActiveAt)

	// Allow for small timing differences (100ms)
	diff := idle - expectedIdle
	if diff < 0 {
		diff = -diff
	}
	if diff > 100*time.Millisecond {
		t.Errorf("expected idle ~%v, got %v (diff: %v)", expectedIdle, idle, diff)
	}
}

func TestConnectionState(t *testing.T) {
	states := []ConnectionState{
		ConnectionStateActive,
		ConnectionStateDisconnected,
		ConnectionStateStale,
	}

	expectedStrings := []string{
		"ACTIVE",
		"DISCONNECTED",
		"STALE",
	}

	for i, state := range states {
		if string(state) != expectedStrings[i] {
			t.Errorf("expected state string %q, got %q", expectedStrings[i], string(state))
		}
	}
}

func TestConnectionStateConstants(t *testing.T) {
	// Verify the constants are defined correctly
	if ConnectionStateActive != "ACTIVE" {
		t.Errorf("ConnectionStateActive should be 'ACTIVE', got %s", ConnectionStateActive)
	}

	if ConnectionStateDisconnected != "DISCONNECTED" {
		t.Errorf("ConnectionStateDisconnected should be 'DISCONNECTED', got %s", ConnectionStateDisconnected)
	}

	if ConnectionStateStale != "STALE" {
		t.Errorf("ConnectionStateStale should be 'STALE', got %s", ConnectionStateStale)
	}
}

func TestConnectionInfoZeroTimestamps(t *testing.T) {
	t.Run("IsActive with zero LastActiveAt", func(t *testing.T) {
		conn := ConnectionInfo{
			ConnectionID: "test-conn",
			ConnectedAt:  time.Now(),
			LastActiveAt: time.Time{}, // Zero value
		}

		// Should be treated as active for fresh connections
		if !conn.IsActive() {
			t.Error("expected zero LastActiveAt to be treated as active")
		}
	})

	t.Run("Age with zero ConnectedAt", func(t *testing.T) {
		conn := ConnectionInfo{
			ConnectionID: "test-conn",
			ConnectedAt:  time.Time{}, // Zero value
		}

		age := conn.Age()
		if age != 0 {
			t.Errorf("expected age to be 0 for zero ConnectedAt, got %v", age)
		}
	})

	t.Run("IdleDuration with zero LastActiveAt falls back to ConnectedAt", func(t *testing.T) {
		connectedAt := time.Now().Add(-10 * time.Minute)
		conn := ConnectionInfo{
			ConnectionID: "test-conn",
			ConnectedAt:  connectedAt,
			LastActiveAt: time.Time{}, // Zero value
		}

		idle := conn.IdleDuration()
		expectedIdle := time.Since(connectedAt)

		// Allow for small timing differences (100ms)
		diff := idle - expectedIdle
		if diff < 0 {
			diff = -diff
		}
		if diff > 100*time.Millisecond {
			t.Errorf("expected idle ~%v, got %v (diff: %v)", expectedIdle, idle, diff)
		}
	})

	t.Run("IdleDuration with both timestamps zero", func(t *testing.T) {
		conn := ConnectionInfo{
			ConnectionID: "test-conn",
			ConnectedAt:  time.Time{}, // Zero value
			LastActiveAt: time.Time{}, // Zero value
		}

		idle := conn.IdleDuration()
		if idle != 0 {
			t.Errorf("expected idle to be 0 for zero timestamps, got %v", idle)
		}
	})

	t.Run("Fresh connection behavior", func(t *testing.T) {
		// Simulates what API Gateway returns for a fresh connection
		conn := ConnectionInfo{
			ConnectionID: "fresh-conn-123",
			ConnectedAt:  time.Now(),
			LastActiveAt: time.Time{}, // Not set yet
			SourceIP:     "192.168.1.1",
			UserAgent:    "TestClient/1.0",
		}

		// Fresh connection should be active
		if !conn.IsActive() {
			t.Error("fresh connection should be active")
		}

		// Age should be minimal
		if conn.Age() > 1*time.Second {
			t.Errorf("fresh connection age should be minimal, got %v", conn.Age())
		}

		// Idle duration should fall back to ConnectedAt
		if conn.IdleDuration() > 1*time.Second {
			t.Errorf("fresh connection idle should be minimal, got %v", conn.IdleDuration())
		}
	})
}
