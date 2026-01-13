package lift

import (
	"context"
	"errors"
	"testing"

	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/require"
)

func TestApp_handleError_AppSyncFormatsLiftError(t *testing.T) {
	app := New()

	req := NewRequest(&adapters.Request{
		TriggerType: adapters.TriggerAppSync,
		Method:      "POST",
		Path:        "/graphql",
		EventID:     "evt-1",
	})
	ctx := NewContext(context.Background(), req)

	liftErr := NewLiftError("BAD_INPUT", "nope", 400).WithDetail("field", "name")
	liftErr.Timestamp = ""
	resp, err := app.handleError(ctx, liftErr)
	require.NoError(t, err)

	out, ok := resp.(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, out["pay_theory_error"])
	require.Equal(t, "nope", out["error_message"])
	require.Equal(t, "CLIENT_ERROR", out["error_type"])

	errorData := out["error_data"].(map[string]any)
	require.Equal(t, 400, errorData["status_code"])
	require.Equal(t, "evt-1", errorData["request_id"])

	errorInfo := out["error_info"].(map[string]any)
	require.Equal(t, "BAD_INPUT", errorInfo["code"])
	require.Equal(t, "/graphql", errorInfo["path"])
	require.Equal(t, "POST", errorInfo["method"])
	require.Equal(t, string(adapters.TriggerAppSync), errorInfo["trigger_type"])
	require.Equal(t, map[string]any{"field": "name"}, errorInfo["details"])
}

func TestApp_handleError_AppSyncFormatsNonLiftError(t *testing.T) {
	app := New()

	req := NewRequest(&adapters.Request{
		TriggerType: adapters.TriggerAppSync,
		Method:      "POST",
		Path:        "/graphql",
	})
	ctx := NewContext(context.Background(), req)

	resp, err := app.handleError(ctx, errors.New("boom"))
	require.NoError(t, err)

	out, ok := resp.(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, out["pay_theory_error"])
	require.Equal(t, "boom", out["error_message"])
	require.Equal(t, "SYSTEM_ERROR", out["error_type"])
	require.Equal(t, map[string]any{}, out["error_data"])
	require.Equal(t, map[string]any{}, out["error_info"])
}

func TestDetermineAppSyncErrorType(t *testing.T) {
	require.Equal(t, "SYSTEM_ERROR", determineAppSyncErrorType(500))
	require.Equal(t, "CLIENT_ERROR", determineAppSyncErrorType(400))
	require.Equal(t, "SYSTEM_ERROR", determineAppSyncErrorType(200))
}
