package security

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileSecretsProviderRotateSecretLifecycle(t *testing.T) {
	provider := NewFileSecretsProvider(t.TempDir())

	ctx := context.Background()
	require.NoError(t, provider.PutSecret(ctx, "api_key", "initial-value"))

	err := provider.RotateSecret(ctx, "api_key")
	require.NoError(t, err)

	secret, err := provider.GetSecret(ctx, "api_key")
	require.NoError(t, err)
	assert.NotEqual(t, "initial-value", secret)

	history := provider.GetRotationHistory("api_key")
	require.Len(t, history, 1)
	assert.True(t, history[0].Success)
	assert.Equal(t, "file_provider_simulation", history[0].Method)
}

func TestFileSecretsProviderRotationDisabled(t *testing.T) {
	provider := NewFileSecretsProviderWithConfig(t.TempDir(), false)

	err := provider.RotateSecret(context.Background(), "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rotation not enabled")
}

func TestFileSecretsProviderSimulatedFailureRecorded(t *testing.T) {
	provider := NewFileSecretsProvider(t.TempDir())

	err := provider.SimulateRotationFailure(context.Background(), "api", "network issue")
	require.Error(t, err)

	history := provider.GetRotationHistory("api")
	require.Len(t, history, 1)
	assert.False(t, history[0].Success)
	assert.Equal(t, "network issue", history[0].Error)
}

func TestFileSecretsProviderRotationHistoryBounded(t *testing.T) {
	provider := NewFileSecretsProvider(t.TempDir())

	ctx := context.Background()
	require.NoError(t, provider.PutSecret(ctx, "secret", "value-base"))

	for i := 0; i < 12; i++ {
		time.Sleep(time.Millisecond) // ensure timestamp progression
		require.NoError(t, provider.RotateSecret(ctx, "secret"))
	}

	history := provider.GetRotationHistory("secret")
	require.Len(t, history, 10, "history should keep the 10 most recent records")
	assert.True(t, history[0].Timestamp.Before(history[len(history)-1].Timestamp))
}
