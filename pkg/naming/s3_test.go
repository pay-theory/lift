package naming

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizeS3BucketName(t *testing.T) {
	t.Run("basic sanitization", func(t *testing.T) {
		require.Equal(t, "my-app-live-static", SanitizeS3BucketName("My App LIVE static"))
	})

	t.Run("collapses invalid characters", func(t *testing.T) {
		require.Equal(t, "my-app", SanitizeS3BucketName("my__app!!"))
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
