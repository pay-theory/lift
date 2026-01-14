package middleware

import (
	"context"
	"testing"

	"github.com/pay-theory/lift/pkg/features"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/require"
)

func TestFeatureFlagMiddleware_SetsExpectedContextValues(t *testing.T) {
	t.Setenv("LIFT_ENV", "production")
	t.Setenv("LIFT_FEATURE_FLAGS_FILE", "does-not-exist.json")

	ff, err := features.NewFeatureFlags(features.FeatureFlagConfig{LocalOnly: true})
	require.NoError(t, err)
	ff.SetFlag(features.DebugLoggingEnabled, true)

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/test"})
	ctx := lift.NewContext(context.Background(), req)

	handler := FeatureFlagMiddleware(ff)(lift.HandlerFunc(func(ctx *lift.Context) error {
		require.Same(t, ff, GetFeatureFlags(ctx))

		isEnabledAny := ctx.Get("is_feature_enabled")
		isEnabled, ok := isEnabledAny.(func(string) bool)
		require.True(t, ok)
		require.True(t, isEnabled(features.DebugLoggingEnabled))

		allFlagsAny := ctx.Get("all_feature_flags")
		allFlags, ok := allFlagsAny.(map[string]bool)
		require.True(t, ok)
		require.NotEmpty(t, allFlags)

		return nil
	}))

	require.NoError(t, handler.Handle(ctx))
}

func TestFeatureFlagMiddleware_DoesNotSetAllFlagsWhenDebugDisabled(t *testing.T) {
	t.Setenv("LIFT_ENV", "production")
	t.Setenv("LIFT_FEATURE_FLAGS_FILE", "does-not-exist.json")

	ff, err := features.NewFeatureFlags(features.FeatureFlagConfig{LocalOnly: true})
	require.NoError(t, err)
	ff.SetFlag(features.DebugLoggingEnabled, false)

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/test"})
	ctx := lift.NewContext(context.Background(), req)

	handler := FeatureFlagMiddleware(ff)(lift.HandlerFunc(func(ctx *lift.Context) error {
		require.Nil(t, ctx.Get("all_feature_flags"))
		return nil
	}))

	require.NoError(t, handler.Handle(ctx))
}

func TestIsFeatureEnabled_FallsBackToGlobalDefaults(t *testing.T) {
	t.Setenv("LIFT_ENV", "production")

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/test"})
	ctx := lift.NewContext(context.Background(), req)

	require.True(t, IsFeatureEnabled(ctx, features.RateLimitingEnabled))
	require.False(t, IsFeatureEnabled(ctx, "unknown_flag"))
}

func TestEventAwareMiddleware_AppliesToEvents(t *testing.T) {
	var m eventAwareMiddleware
	require.True(t, m.AppliesToEvents())
}
