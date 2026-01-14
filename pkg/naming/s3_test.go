package naming

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizeS3BucketName(t *testing.T) {
	t.Run("basic sanitization", func(t *testing.T) {
		require.Equal(t, "my-app-live-static", SanitizeS3BucketName("My App LIVE static"))
	})

	t.Run("defaults when empty", func(t *testing.T) {
		require.Equal(t, "bucket", SanitizeS3BucketName(""))
	})

	t.Run("pads short names", func(t *testing.T) {
		require.Equal(t, "a-bucket", SanitizeS3BucketName("a"))
	})

	t.Run("collapses invalid characters", func(t *testing.T) {
		require.Equal(t, "my-app", SanitizeS3BucketName("my__app!!"))
	})

	t.Run("falls back when sanitization empties name", func(t *testing.T) {
		require.Equal(t, "bucket", SanitizeS3BucketName("!!!"))
	})

	t.Run("truncates with stable hash", func(t *testing.T) {
		long := "this-is-a-very-long-application-name-that-would-exceed-the-s3-bucket-name-limit-live"
		name := SanitizeS3BucketName(long)
		require.LessOrEqual(t, len(name), 63)
		require.Contains(t, name, "-")
		require.Equal(t, name, SanitizeS3BucketName(long))
	})
}

func TestContext_S3BucketName(t *testing.T) {
	ctx := Context{AppName: "Repo_Name", Stage: "dev", Tenant: "Partner"}
	require.Equal(t, "repo-name-partner-events-lab", ctx.S3BucketName("events"))
}

func TestS3BucketNameFromEnv(t *testing.T) {
	t.Run("incomplete context returns false", func(t *testing.T) {
		t.Setenv(EnvAppName, "")
		t.Setenv(EnvStage, "")

		name, ok := S3BucketNameFromEnv("events")
		require.False(t, ok)
		require.Empty(t, name)
	})

	t.Run("complete context returns deterministic name", func(t *testing.T) {
		t.Setenv(EnvAppName, "My_App")
		t.Setenv(EnvStage, "prod")
		t.Setenv(EnvTenant, "Tenant")

		name, ok := S3BucketNameFromEnv("events")
		require.True(t, ok)
		require.Equal(t, "my-app-tenant-events-live", name)
	})
}
