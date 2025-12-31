package services

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func resetEventBusTableNameOverride(t *testing.T) {
	t.Helper()
	eventBusTableNameMu.Lock()
	eventBusTableNameOverride = ""
	eventBusTableNameMu.Unlock()
}

func TestEvent_TableName_DefaultEnvAndOverride(t *testing.T) {
	resetEventBusTableNameOverride(t)
	t.Cleanup(func() { resetEventBusTableNameOverride(t) })

	// When env is incomplete, fall back to the hard-coded default.
	t.Setenv("APP_NAME", "")
	t.Setenv("STAGE", "")
	t.Setenv("PARTNER", "")
	require.Equal(t, "lift-events", (&Event{}).TableName())

	// When env is complete, derive deterministic names.
	t.Setenv("APP_NAME", "my-app")
	t.Setenv("STAGE", "dev")
	t.Setenv("PARTNER", "")
	require.Equal(t, "my-app-events-lab", (&Event{}).TableName())

	// Override takes precedence over env-derived naming.
	require.NoError(t, setEventBusTableNameOverride("custom-events-table"))
	require.Equal(t, "custom-events-table", getEventBusTableNameOverride())
	require.Equal(t, "custom-events-table", (&Event{}).TableName())

	// Setting the same override is allowed; changing is not.
	require.NoError(t, setEventBusTableNameOverride("custom-events-table"))
	require.Error(t, setEventBusTableNameOverride("different-table"))
}

func TestEventBusRelatedModels_UseEventTableName(t *testing.T) {
	resetEventBusTableNameOverride(t)
	t.Cleanup(func() { resetEventBusTableNameOverride(t) })

	t.Setenv("APP_NAME", "my-app")
	t.Setenv("STAGE", "live")

	expected := (&Event{}).TableName()
	require.Equal(t, expected, (&EventBusCheckpoint{}).TableName())
	require.Equal(t, expected, (&EventBusScheduledEvent{}).TableName())
	require.Equal(t, expected, (&EventBusQuarantinedScheduledEvent{}).TableName())
}
