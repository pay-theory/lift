package features

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/appconfig"
	"github.com/stretchr/testify/require"
)

func TestFeatureFlags_LoadLocalOverridesFromFile(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "flags.json")
	require.NoError(t, os.WriteFile(configFile, []byte(`{"rate_limiting_enabled":false,"custom_feature":true}`), 0o600))

	require.NoError(t, os.Setenv("LIFT_FEATURE_FLAGS_FILE", configFile))
	defer func() { _ = os.Unsetenv("LIFT_FEATURE_FLAGS_FILE") }()

	ff, err := NewFeatureFlags(FeatureFlagConfig{LocalOnly: true})
	require.NoError(t, err)

	require.False(t, ff.IsEnabled(RateLimitingEnabled))
	require.True(t, ff.IsEnabled("custom_feature"))
}

func TestFeatureFlags_LoadLocalOverrides_InvalidJSONDoesNotOverride(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "flags.json")
	require.NoError(t, os.WriteFile(configFile, []byte("{"), 0o600))

	require.NoError(t, os.Setenv("LIFT_FEATURE_FLAGS_FILE", configFile))
	defer func() { _ = os.Unsetenv("LIFT_FEATURE_FLAGS_FILE") }()

	require.NoError(t, os.Setenv("LIFT_ENV", "production"))
	defer func() { _ = os.Unsetenv("LIFT_ENV") }()

	ff, err := NewFeatureFlags(FeatureFlagConfig{LocalOnly: true})
	require.NoError(t, err)

	// Production defaults for known flags should remain in effect.
	require.True(t, ff.IsEnabled(RateLimitingEnabled))
}

func TestFeatureFlags_refresh_WithStubbedAppConfigClient(t *testing.T) {
	var mu sync.Mutex
	responseBody := `{"rate_limiting_enabled":false,"new_dashboard_ui":true}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		body := responseBody
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	cfg := aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("AKID", "SECRET", ""),
		HTTPClient:  server.Client(),
	}

	client := appconfig.NewFromConfig(cfg, func(o *appconfig.Options) {
		o.BaseEndpoint = aws.String(server.URL)
	})

	ff := &FeatureFlags{
		flags:       make(map[string]bool),
		client:      client,
		environment: "env",
		application: "app",
		clientId:    "client-id",
		stopRefresh: make(chan struct{}),
	}

	require.NoError(t, ff.refresh())
	require.False(t, ff.IsEnabled(RateLimitingEnabled))
	require.True(t, ff.IsEnabled(NewDashboardUI))

	mu.Lock()
	responseBody = "{"
	mu.Unlock()

	require.Error(t, ff.refresh())
}

func TestFeatureFlags_refresh_ErrorsWithoutClient(t *testing.T) {
	ff := &FeatureFlags{
		flags: make(map[string]bool),
	}
	require.Error(t, ff.refresh())
}

func TestFeatureFlags_refreshLoop_Stop(t *testing.T) {
	ff := &FeatureFlags{
		flags:       make(map[string]bool),
		stopRefresh: make(chan struct{}),
	}

	done := make(chan struct{})
	go func() {
		ff.refreshLoop()
		close(done)
	}()

	ff.Stop()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("refresh loop did not stop")
	}
}

func TestGlobalIsEnabled_WhenUninitializedUsesDefaults(t *testing.T) {
	previous := defaultFlags
	defaultFlags = nil
	defer func() { defaultFlags = previous }()

	require.NoError(t, os.Setenv("LIFT_ENV", "production"))
	defer func() { _ = os.Unsetenv("LIFT_ENV") }()

	require.True(t, IsEnabled(RateLimitingEnabled))
	require.False(t, IsEnabled(MockServicesEnabled))
}

func TestFeatureFlags_IsEnabledEnvOverrideAcceptsOne(t *testing.T) {
	ff, err := NewFeatureFlags(FeatureFlagConfig{LocalOnly: true})
	require.NoError(t, err)

	// Explicitly disable in the in-memory map, then override via env var.
	ff.SetFlag(RateLimitingEnabled, false)

	require.NoError(t, os.Setenv("LIFT_FEATURE_"+RateLimitingEnabled, "1"))
	defer func() { _ = os.Unsetenv("LIFT_FEATURE_" + RateLimitingEnabled) }()

	require.True(t, ff.IsEnabled(RateLimitingEnabled))
}
