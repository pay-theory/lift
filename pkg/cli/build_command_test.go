package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/pay-theory/lift/internal/liftconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestProject creates a minimal Go project for build testing.
// It creates go.mod, cmd/api/main.go, cmd/worker/main.go, and lift.yaml.
func setupTestProject(t *testing.T, dir string, liftYAML string) {
	t.Helper()

	// Create go.mod
	goMod := `module test-project

go 1.21
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0600))

	// Create cmd/api/main.go
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "cmd", "api"), 0750))
	apiMain := `package main

func main() {}
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cmd", "api", "main.go"), []byte(apiMain), 0600))

	// Create cmd/worker/main.go
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "cmd", "worker"), 0750))
	workerMain := `package main

func main() {}
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cmd", "worker", "main.go"), []byte(workerMain), 0600))

	// Create lift.yaml
	require.NoError(t, os.WriteFile(filepath.Join(dir, "lift.yaml"), []byte(liftYAML), 0600))
}

// setHermeticEnv sets GOCACHE and GOMODCACHE to temp directories
// to avoid polluting user caches during tests.
func setHermeticEnv(t *testing.T, cmd *exec.Cmd, dir string) {
	t.Helper()
	goCache := filepath.Join(dir, ".gocache")
	goModCache := filepath.Join(dir, ".gomodcache")
	require.NoError(t, os.MkdirAll(goCache, 0750))
	require.NoError(t, os.MkdirAll(goModCache, 0750))

	// Filter out any existing GOCACHE/GOMODCACHE entries
	var env []string
	for _, e := range os.Environ() {
		if !hasPrefix(e, "GOCACHE=") && !hasPrefix(e, "GOMODCACHE=") {
			env = append(env, e)
		}
	}
	env = append(env, "GOCACHE="+goCache, "GOMODCACHE="+goModCache)
	cmd.Env = env
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func TestBuildCommand_MultiFunction_FromProjectRoot(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()

	// Create multi-function project
	liftYAML := `version: 1

app:
  name: multi-func-test
  template: basic-api

build:
  goos: linux
  goarch: arm64
  cgo: 0
  trimpath: true
  ldflags: "-s -w"
  tags:
    - lambda.norpc

functions:
  api:
    cmd: ./cmd/api
    out: ./dist/api/bootstrap
  worker:
    cmd: ./cmd/worker
    out: ./dist/worker/bootstrap
`
	setupTestProject(t, tmpDir, liftYAML)

	// Change to project root
	require.NoError(t, os.Chdir(tmpDir))

	// Create a build command with hermetic environment
	cmd := &BuildCommand{
		cmdFactory: func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			c := exec.CommandContext(ctx, name, arg...)
			setHermeticEnv(t, c, tmpDir)
			return c
		},
	}

	err = cmd.Execute(context.Background(), nil)
	require.NoError(t, err)

	// Assert both output files exist
	assertFileExists(t, filepath.Join(tmpDir, "dist", "api", "bootstrap"))
	assertFileExists(t, filepath.Join(tmpDir, "dist", "worker", "bootstrap"))
}

func TestBuildCommand_MultiFunction_FromSubdirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()

	// Create multi-function project
	liftYAML := `version: 1

app:
  name: subdir-test
  template: basic-api

functions:
  api:
    cmd: ./cmd/api
    out: ./dist/api/bootstrap
`
	setupTestProject(t, tmpDir, liftYAML)

	// Change to a nested subdirectory (proves root detection)
	nestedDir := filepath.Join(tmpDir, "cmd", "api")
	require.NoError(t, os.Chdir(nestedDir))

	// Create a build command with hermetic environment
	cmd := &BuildCommand{
		cmdFactory: func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			c := exec.CommandContext(ctx, name, arg...)
			setHermeticEnv(t, c, tmpDir)
			return c
		},
	}

	err = cmd.Execute(context.Background(), nil)
	require.NoError(t, err)

	// Assert output file exists at the correct path relative to project root
	assertFileExists(t, filepath.Join(tmpDir, "dist", "api", "bootstrap"))
}

func TestBuildCommand_NoFunctionsInConfig_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()

	// Create project with no functions
	liftYAML := `version: 1

app:
  name: no-functions-test
  template: basic-api
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "lift.yaml"), []byte(liftYAML), 0600))
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &BuildCommand{}
	err = cmd.Execute(context.Background(), nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no functions defined")
}

func TestBuildCommand_EmptyFunctionsMap_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()

	// Create project with empty functions map
	liftYAML := `version: 1

app:
  name: empty-functions-test
  template: basic-api

functions: {}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "lift.yaml"), []byte(liftYAML), 0600))
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &BuildCommand{}
	err = cmd.Execute(context.Background(), nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no functions defined")
}

func TestBuildCommand_MissingLiftYAML_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()

	// Change to empty directory (no lift.yaml)
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &BuildCommand{}
	err = cmd.Execute(context.Background(), nil)

	require.Error(t, err)
	// ProjectNotFoundError message
	assert.Contains(t, err.Error(), "no Lift project found")
	assert.Contains(t, err.Error(), "lift.yaml")
}

func TestBuildCommand_MissingCmdField_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()

	// Create project with function missing cmd
	liftYAML := `version: 1

app:
  name: missing-cmd-test
  template: basic-api

functions:
  api:
    out: ./dist/api/bootstrap
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "lift.yaml"), []byte(liftYAML), 0600))
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &BuildCommand{}
	err = cmd.Execute(context.Background(), nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing 'cmd' field")
	assert.Contains(t, err.Error(), "api")
}

func TestBuildCommand_MissingOutField_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()

	// Create project with function missing out
	liftYAML := `version: 1

app:
  name: missing-out-test
  template: basic-api

functions:
  api:
    cmd: ./cmd/api
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "lift.yaml"), []byte(liftYAML), 0600))
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &BuildCommand{}
	err = cmd.Execute(context.Background(), nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing 'out' field")
	assert.Contains(t, err.Error(), "api")
}

func TestBuildCommand_InvalidArchFlag_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()

	// Create minimal project
	liftYAML := `version: 1

app:
  name: arch-test
  template: basic-api

functions:
  api:
    cmd: ./cmd/api
    out: ./dist/api/bootstrap
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "lift.yaml"), []byte(liftYAML), 0600))
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &BuildCommand{}
	err = cmd.Execute(context.Background(), []string{"--arch", "x86"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid architecture")
}

func TestBuildCommand_ArchFlagEqualsForm_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()

	// Create minimal project
	liftYAML := `version: 1

app:
  name: arch-test
  template: basic-api

functions:
  api:
    cmd: ./cmd/api
    out: ./dist/api/bootstrap
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "lift.yaml"), []byte(liftYAML), 0600))
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &BuildCommand{}
	err = cmd.Execute(context.Background(), []string{"--arch=invalid"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid architecture")
}

func TestBuildCommand_ArchFlagOverridesConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()

	// Create project with arm64 in config
	liftYAML := `version: 1

app:
  name: arch-override-test
  template: basic-api

build:
  goarch: arm64

functions:
  api:
    cmd: ./cmd/api
    out: ./dist/api/bootstrap
`
	setupTestProject(t, tmpDir, liftYAML)
	require.NoError(t, os.Chdir(tmpDir))

	// Create a build command with hermetic environment
	cmd := &BuildCommand{
		cmdFactory: func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			c := exec.CommandContext(ctx, name, arg...)
			setHermeticEnv(t, c, tmpDir)
			return c
		},
	}

	// Override with amd64 via CLI flag
	err = cmd.Execute(context.Background(), []string{"--arch", "amd64"})
	require.NoError(t, err)

	// Verify the binary is created (proves the command ran successfully with override)
	assertFileExists(t, filepath.Join(tmpDir, "dist", "api", "bootstrap"))
}

func TestBuildCommand_DefaultsApplied(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()

	// Create project with minimal config (no build section)
	liftYAML := `version: 1

app:
  name: defaults-test
  template: basic-api

functions:
  api:
    cmd: ./cmd/api
    out: ./dist/api/bootstrap
`
	setupTestProject(t, tmpDir, liftYAML)
	require.NoError(t, os.Chdir(tmpDir))

	// Create a build command with hermetic environment
	cmd := &BuildCommand{
		cmdFactory: func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			c := exec.CommandContext(ctx, name, arg...)
			setHermeticEnv(t, c, tmpDir)
			return c
		},
	}

	err = cmd.Execute(context.Background(), nil)
	require.NoError(t, err)

	// Assert output file exists (defaults were applied correctly)
	assertFileExists(t, filepath.Join(tmpDir, "dist", "api", "bootstrap"))
}

func TestBuildCommand_GoBuildFailure_ReturnsErrorWithContext(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()

	// Create project with a function that points to non-existent package
	liftYAML := `version: 1

app:
  name: build-failure-test
  template: basic-api

functions:
  api:
    cmd: ./cmd/nonexistent
    out: ./dist/api/bootstrap
`
	// Create go.mod but no actual Go files for the package
	goMod := `module test-project

go 1.21
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "lift.yaml"), []byte(liftYAML), 0600))
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &BuildCommand{
		cmdFactory: func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			c := exec.CommandContext(ctx, name, arg...)
			setHermeticEnv(t, c, tmpDir)
			return c
		},
	}

	err = cmd.Execute(context.Background(), nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "build failed")
	assert.Contains(t, err.Error(), "api") // function name included
}

func TestBuildCommand_ResolveBuildConfig_Defaults(t *testing.T) {
	cmd := &BuildCommand{}

	// Test with nil build config
	cfg := &liftconfig.Config{}
	bc := cmd.resolveBuildConfig(cfg, "")

	assert.Equal(t, "linux", bc.goos)
	assert.Equal(t, "arm64", bc.goarch)
	assert.False(t, bc.cgo)
	assert.True(t, bc.trimpath)
	assert.Equal(t, "-s -w", bc.ldflags)
	assert.Equal(t, []string{"lambda.norpc"}, bc.tags)
}

func TestBuildCommand_ResolveBuildConfig_ConfigOverrides(t *testing.T) {
	cmd := &BuildCommand{}
	trimpath := true

	cfg := &liftconfig.Config{
		Build: &liftconfig.Build{
			GOOS:     "darwin",
			GOARCH:   "amd64",
			CGO:      1,
			Trimpath: &trimpath,
			LDFlags:  "-X main.version=1.0.0",
			Tags:     []string{"custom", "tags"},
		},
	}
	bc := cmd.resolveBuildConfig(cfg, "")

	assert.Equal(t, "darwin", bc.goos)
	assert.Equal(t, "amd64", bc.goarch)
	assert.True(t, bc.cgo)
	assert.True(t, bc.trimpath)
	assert.Equal(t, "-X main.version=1.0.0", bc.ldflags)
	assert.Equal(t, []string{"lambda.norpc", "custom", "tags"}, bc.tags)
}

func TestBuildCommand_ResolveBuildConfig_CLIOverridesConfig(t *testing.T) {
	cmd := &BuildCommand{}

	cfg := &liftconfig.Config{
		Build: &liftconfig.Build{
			GOARCH: "arm64",
		},
	}
	bc := cmd.resolveBuildConfig(cfg, "amd64")

	assert.Equal(t, "amd64", bc.goarch) // CLI override wins
}

func TestBuildCommand_ResolveBuildConfig_TrimpathDefaultsToTrueWhenOmitted(t *testing.T) {
	cmd := &BuildCommand{}

	cfg := &liftconfig.Config{
		Build: &liftconfig.Build{
			GOOS: "linux",
		},
	}
	bc := cmd.resolveBuildConfig(cfg, "")

	assert.True(t, bc.trimpath)
}

func TestBuildCommand_ResolveBuildConfig_TrimpathCanBeDisabled(t *testing.T) {
	cmd := &BuildCommand{}
	trimpath := false

	cfg := &liftconfig.Config{
		Build: &liftconfig.Build{
			Trimpath: &trimpath,
		},
	}
	bc := cmd.resolveBuildConfig(cfg, "")

	assert.False(t, bc.trimpath)
}

// TestBuildCommand_GeneratedProject_CanBuild verifies that projects generated
// by "lift new" can be built by "lift build" (end-to-end integration).
// Note: This test creates simplified Go files that don't require go mod tidy
// to prove the build command works on the generated lift.yaml structure.
func TestBuildCommand_GeneratedProject_CanBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Generate a project using lift new
	newCmd := &NewCommandV2{}
	err = newCmd.Execute(context.Background(), []string{"my-generated-app", "--base-domain", "example.com"})
	require.NoError(t, err)

	// Verify lift.yaml was created with functions section
	projectDir := filepath.Join(tmpDir, "my-generated-app")
	cfg, err := liftconfig.LoadConfig(projectDir)
	require.NoError(t, err)
	require.NotNil(t, cfg.Functions)
	require.Contains(t, cfg.Functions, "api")

	// Replace the generated main.go with a simple one that doesn't require external deps
	simpleMain := `package main

func main() {}
`
	err = os.WriteFile(filepath.Join(projectDir, "cmd", "api", "main.go"), []byte(simpleMain), 0600)
	require.NoError(t, err)

	// Change to project directory
	require.NoError(t, os.Chdir(projectDir))

	// Build the project
	buildCmd := &BuildCommand{
		cmdFactory: func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			c := exec.CommandContext(ctx, name, arg...)
			setHermeticEnv(t, c, tmpDir)
			return c
		},
	}

	err = buildCmd.Execute(context.Background(), nil)
	require.NoError(t, err)

	// Verify the output exists at the configured path
	assertFileExists(t, filepath.Join(projectDir, "dist", "api", "bootstrap"))
}

// =============================================================================
// Integration tests for new templates: microservice, event-driven, merchant-app
// =============================================================================

// TestBuildCommand_MicroserviceTemplate_CanBuild verifies that microservice template
// generates a project that can be built successfully.
func TestBuildCommand_MicroserviceTemplate_CanBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Generate a microservice project
	newCmd := &NewCommandV2{}
	err = newCmd.Execute(context.Background(), []string{"my-microservice", "--template", "microservice", "--base-domain", "example.com"})
	require.NoError(t, err)

	projectDir := filepath.Join(tmpDir, "my-microservice")

	// Replace the generated main.go with a simple one
	simpleMain := `package main

func main() {}
`
	err = os.WriteFile(filepath.Join(projectDir, "cmd", "api", "main.go"), []byte(simpleMain), 0600)
	require.NoError(t, err)

	// Change to project directory
	require.NoError(t, os.Chdir(projectDir))

	// Build the project
	buildCmd := &BuildCommand{
		cmdFactory: func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			c := exec.CommandContext(ctx, name, arg...)
			setHermeticEnv(t, c, tmpDir)
			return c
		},
	}

	err = buildCmd.Execute(context.Background(), nil)
	require.NoError(t, err)

	// Verify the output exists for api function
	assertFileExists(t, filepath.Join(projectDir, "dist", "api", "bootstrap"))
}

// TestBuildCommand_EventDrivenTemplate_CanBuild verifies that event-driven template
// generates a project that can be built successfully.
func TestBuildCommand_EventDrivenTemplate_CanBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Generate an event-driven project
	newCmd := &NewCommandV2{}
	err = newCmd.Execute(context.Background(), []string{"my-event-app", "--template", "event-driven", "--base-domain", "example.com"})
	require.NoError(t, err)

	projectDir := filepath.Join(tmpDir, "my-event-app")

	// Replace the generated main.go files with simple ones
	simpleMain := `package main

func main() {}
`
	err = os.WriteFile(filepath.Join(projectDir, "cmd", "api", "main.go"), []byte(simpleMain), 0600)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(projectDir, "cmd", "processor", "main.go"), []byte(simpleMain), 0600)
	require.NoError(t, err)

	// Change to project directory
	require.NoError(t, os.Chdir(projectDir))

	// Build the project
	buildCmd := &BuildCommand{
		cmdFactory: func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			c := exec.CommandContext(ctx, name, arg...)
			setHermeticEnv(t, c, tmpDir)
			return c
		},
	}

	err = buildCmd.Execute(context.Background(), nil)
	require.NoError(t, err)

	// Verify the outputs exist for both functions
	assertFileExists(t, filepath.Join(projectDir, "dist", "api", "bootstrap"))
	assertFileExists(t, filepath.Join(projectDir, "dist", "processor", "bootstrap"))
}

// TestBuildCommand_SNSProcessorTemplate_CanBuild verifies that sns-processor template
// generates a project that can be built successfully.
func TestBuildCommand_SNSProcessorTemplate_CanBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Generate an SNS processor project
	newCmd := &NewCommandV2{}
	err = newCmd.Execute(context.Background(), []string{"my-sns-app", "--template", "sns-processor", "--base-domain", "example.com"})
	require.NoError(t, err)

	projectDir := filepath.Join(tmpDir, "my-sns-app")

	// Replace the generated main.go with a simple one
	simpleMain := `package main

func main() {}
`
	err = os.WriteFile(filepath.Join(projectDir, "cmd", "processor", "main.go"), []byte(simpleMain), 0600)
	require.NoError(t, err)

	// Change to project directory
	require.NoError(t, os.Chdir(projectDir))

	// Build the project
	buildCmd := &BuildCommand{
		cmdFactory: func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			c := exec.CommandContext(ctx, name, arg...)
			setHermeticEnv(t, c, tmpDir)
			return c
		},
	}

	err = buildCmd.Execute(context.Background(), nil)
	require.NoError(t, err)

	// Verify the output exists for processor function
	assertFileExists(t, filepath.Join(projectDir, "dist", "processor", "bootstrap"))
}

// TestBuildCommand_MerchantAppTemplate_CanBuild verifies that merchant-app template
// generates a project that can be built successfully.
func TestBuildCommand_MerchantAppTemplate_CanBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Generate a merchant-app project
	newCmd := &NewCommandV2{}
	err = newCmd.Execute(context.Background(), []string{"my-merchant-app", "--template", "merchant-app", "--base-domain", "example.com"})
	require.NoError(t, err)

	projectDir := filepath.Join(tmpDir, "my-merchant-app")

	// Replace the generated main.go files with simple ones
	simpleMain := `package main

func main() {}
`
	err = os.WriteFile(filepath.Join(projectDir, "cmd", "api", "main.go"), []byte(simpleMain), 0600)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(projectDir, "cmd", "worker", "main.go"), []byte(simpleMain), 0600)
	require.NoError(t, err)

	// Change to project directory
	require.NoError(t, os.Chdir(projectDir))

	// Build the project
	buildCmd := &BuildCommand{
		cmdFactory: func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			c := exec.CommandContext(ctx, name, arg...)
			setHermeticEnv(t, c, tmpDir)
			return c
		},
	}

	err = buildCmd.Execute(context.Background(), nil)
	require.NoError(t, err)

	// Verify the outputs exist for both functions
	assertFileExists(t, filepath.Join(projectDir, "dist", "api", "bootstrap"))
	assertFileExists(t, filepath.Join(projectDir, "dist", "worker", "bootstrap"))
}

// =============================================================================
// Prerequisite check tests
// =============================================================================

func TestBuildCommand_GoNotFound_ReturnsActionableError(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()

	// Create minimal project
	liftYAML := `version: 1

app:
  name: prereq-test

functions:
  api:
    cmd: ./cmd/api
    out: ./dist/api/bootstrap
`
	setupTestProject(t, tmpDir, liftYAML)
	require.NoError(t, os.Chdir(tmpDir))

	// Create command with mock lookPath that simulates missing go
	cmd := &BuildCommand{
		lookPath: func(name string) (string, error) {
			return "", &PrereqError{Binary: name, Message: "not found"}
		},
	}

	err = cmd.Execute(context.Background(), nil)
	require.Error(t, err)
	assert.True(t, IsPrereqError(err))
	assert.Contains(t, err.Error(), "go not found")
	assert.Contains(t, err.Error(), "https://go.dev/doc/install")
}
