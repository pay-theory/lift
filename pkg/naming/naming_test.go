package naming

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeStage(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "lab", NormalizeStage("dev"))
	assert.Equal(t, "lab", NormalizeStage("LAB"))
	assert.Equal(t, "study", NormalizeStage("sandbox"))
	assert.Equal(t, "live", NormalizeStage("prod"))
	assert.Equal(t, "qa", NormalizeStage("qa"))
}

func TestContextBaseName(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "repo-lab", Context{AppName: "repo", Stage: "dev"}.BaseName())
	assert.Equal(t, "repo-tenant-live", Context{AppName: "repo", Tenant: "tenant", Stage: "live"}.BaseName())
}

func TestContextResourceName(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "repo-events-lab", Context{AppName: "repo", Stage: "dev"}.ResourceName("events"))
	assert.Equal(t, "repo-tenant-events-live", Context{AppName: "repo", Tenant: "tenant", Stage: "prod"}.ResourceName("events"))
}

func TestResourceNameFromEnv(t *testing.T) {
	t.Parallel()

	originalApp := os.Getenv(EnvAppName)
	originalStage := os.Getenv(EnvStage)
	originalTenant := os.Getenv(EnvTenant)
	t.Cleanup(func() {
		_ = os.Setenv(EnvAppName, originalApp)
		_ = os.Setenv(EnvStage, originalStage)
		_ = os.Setenv(EnvTenant, originalTenant)
	})

	_ = os.Unsetenv(EnvAppName)
	_ = os.Unsetenv(EnvStage)
	_ = os.Unsetenv(EnvTenant)

	name, ok := ResourceNameFromEnv("events")
	assert.False(t, ok)
	assert.Empty(t, name)

	_ = os.Setenv(EnvAppName, "repo")
	_ = os.Setenv(EnvStage, "lab")

	name, ok = ResourceNameFromEnv("events")
	assert.True(t, ok)
	assert.Equal(t, "repo-events-lab", name)

	_ = os.Setenv(EnvTenant, "tenant")

	name, ok = ResourceNameFromEnv("events")
	assert.True(t, ok)
	assert.Equal(t, "repo-tenant-events-lab", name)
}
