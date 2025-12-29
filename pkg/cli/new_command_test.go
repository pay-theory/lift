package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/pay-theory/lift/internal/liftconfig"
)

func TestNewCommandV2_CreateAppDirectory(t *testing.T) {
	// Create temp dir as working directory
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &NewCommandV2{}
	args := []string{"my-app", "--template", "basic-api", "--base-domain", "example.com"}

	err = cmd.Execute(context.Background(), args)
	require.NoError(t, err)

	appDir := filepath.Join(tmpDir, "my-app")

	// Verify expected files exist
	assertFileExists(t, filepath.Join(appDir, "lift.yaml"))
	assertFileExists(t, filepath.Join(appDir, "go.mod"))
	assertFileExists(t, filepath.Join(appDir, "README.md"))
	assertFileExists(t, filepath.Join(appDir, "cmd", "api", "main.go"))
	assertFileExists(t, filepath.Join(appDir, "cdk", "main.go"))
	assertFileExists(t, filepath.Join(appDir, "cdk", "go.mod"))
	assertFileExists(t, filepath.Join(appDir, "cdk", "cdk.json"))
	assertFileExists(t, filepath.Join(appDir, ".gitignore"))
	assertFileExists(t, filepath.Join(appDir, ".github", "workflows", "deploy.yml"))
	assertFileExists(t, filepath.Join(appDir, ".github", "workflows", "pr.yml"))
}

func TestNewCommandV2_BootstrapCurrentDirectory(t *testing.T) {
	// Create temp dir as working directory
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &NewCommandV2{}
	args := []string{"--template", "basic-api", "--base-domain", "example.com"}

	err = cmd.Execute(context.Background(), args)
	require.NoError(t, err)

	// Verify expected files exist in current directory
	assertFileExists(t, filepath.Join(tmpDir, "lift.yaml"))
	assertFileExists(t, filepath.Join(tmpDir, "go.mod"))
	assertFileExists(t, filepath.Join(tmpDir, "cmd", "api", "main.go"))
	assertFileExists(t, filepath.Join(tmpDir, "cdk", "main.go"))
}

func TestNewCommandV2_LiftYAMLContainsCorrectDomains(t *testing.T) {
	// Create temp dir as working directory
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &NewCommandV2{}
	args := []string{"my-app", "--template", "basic-api", "--base-domain", "example.com"}

	err = cmd.Execute(context.Background(), args)
	require.NoError(t, err)

	// Load and parse the generated lift.yaml
	appDir := filepath.Join(tmpDir, "my-app")
	cfg, err := liftconfig.LoadConfig(appDir)
	require.NoError(t, err)

	// Verify stage domains per contract
	assert.Equal(t, "dev.example.com", cfg.Stages["dev"].RootDomain)
	assert.Equal(t, "staging.example.com", cfg.Stages["staging"].RootDomain)
	assert.Equal(t, "example.com", cfg.Stages["live"].RootDomain)

	// Verify base domain
	require.NotNil(t, cfg.Domains)
	assert.Equal(t, "example.com", cfg.Domains.BaseDomain)

	// Verify app metadata
	assert.Equal(t, "my-app", cfg.App.Name)
	assert.Equal(t, "basic-api", cfg.App.Template)

	// Verify services subdomain
	require.NotNil(t, cfg.Services)
	require.Contains(t, cfg.Services, "api")
	assert.Equal(t, "api", cfg.Services["api"].Subdomain)
}

func TestNewCommandV2_GeneratedProjectLiftUpSucceeds(t *testing.T) {
	// Create temp dir as working directory
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Create the project
	newCmd := &NewCommandV2{}
	args := []string{"test-project", "--template", "basic-api", "--base-domain", "test.com"}
	err = newCmd.Execute(context.Background(), args)
	require.NoError(t, err)

	// Change to project directory
	projectDir := filepath.Join(tmpDir, "test-project")
	require.NoError(t, os.Chdir(projectDir))

	// Run lift up with mocked cmdFactory to avoid requiring real go/cdk toolchain
	upCmd := &UpCommand{
		cmdFactory: func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			// Return a command that succeeds (true exits with 0)
			return exec.CommandContext(ctx, "true")
		},
	}
	err = upCmd.Execute(context.Background(), nil)
	require.NoError(t, err)
}

func TestNewCommandV2_FailsIfLiftYAMLExists(t *testing.T) {
	// Create temp dir as working directory
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Create existing lift.yaml
	err = os.WriteFile(filepath.Join(tmpDir, "lift.yaml"), []byte("version: 1\n"), 0600)
	require.NoError(t, err)

	cmd := &NewCommandV2{}
	args := []string{"--template", "basic-api", "--base-domain", "example.com"}

	err = cmd.Execute(context.Background(), args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lift.yaml already exists")
}

func TestNewCommandV2_FailsIfAppDirectoryExists(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Create existing directory
	existingDir := filepath.Join(tmpDir, "existing-app")
	require.NoError(t, os.MkdirAll(existingDir, 0750))

	cmd := &NewCommandV2{}
	args := []string{"existing-app", "--template", "basic-api", "--base-domain", "example.com"}

	err = cmd.Execute(context.Background(), args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestNewCommandV2_RequiresBaseDomain(t *testing.T) {
	cmd := &NewCommandV2{}
	args := []string{"my-app", "--template", "basic-api"}

	err := cmd.Execute(context.Background(), args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--base-domain is required")
}

func TestNewCommandV2_DefaultTemplateIsBasicAPI(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &NewCommandV2{}
	// No --template flag, should default to basic-api
	args := []string{"my-app", "--base-domain", "example.com"}

	err = cmd.Execute(context.Background(), args)
	require.NoError(t, err)

	// Verify template was used by checking lift.yaml
	appDir := filepath.Join(tmpDir, "my-app")
	cfg, err := liftconfig.LoadConfig(appDir)
	require.NoError(t, err)
	assert.Equal(t, "basic-api", cfg.App.Template)
}

func TestNewCommandV2_PTFlagReturnsNotImplementedError(t *testing.T) {
	cmd := &NewCommandV2{}
	args := []string{"my-app", "--base-domain", "example.com", "--pt"}

	err := cmd.Execute(context.Background(), args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not yet implemented")
	assert.Contains(t, err.Error(), "Milestone 6")
}

func TestNewCommandV2_UnknownTemplateReturnsError(t *testing.T) {
	cmd := &NewCommandV2{}
	args := []string{"my-app", "--template", "unknown-template", "--base-domain", "example.com"}

	err := cmd.Execute(context.Background(), args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown template")
}

func TestNewCommandV2_VerifyGitHubWorkflowContent(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &NewCommandV2{}
	args := []string{"my-app", "--template", "basic-api", "--base-domain", "example.com"}
	err = cmd.Execute(context.Background(), args)
	require.NoError(t, err)

	// Read deploy.yml and verify key content
	deployPath := filepath.Join(tmpDir, "my-app", ".github", "workflows", "deploy.yml")
	deployContent, err := os.ReadFile(deployPath)
	require.NoError(t, err)

	// Verify OIDC permissions
	assert.Contains(t, string(deployContent), "id-token: write")
	assert.Contains(t, string(deployContent), "contents: read")

	// Verify environment reference
	assert.Contains(t, string(deployContent), "environment:")

	// Verify AWS role from vars
	assert.Contains(t, string(deployContent), "vars.AWS_ROLE_ARN")
	assert.Contains(t, string(deployContent), "vars.AWS_REGION")

	// Verify stage choices
	assert.Contains(t, string(deployContent), "dev")
	assert.Contains(t, string(deployContent), "staging")
	assert.Contains(t, string(deployContent), "live")
}

func TestNewCommandV2_VerifyREADMEContent(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &NewCommandV2{}
	args := []string{"my-app", "--template", "basic-api", "--base-domain", "example.com"}
	err = cmd.Execute(context.Background(), args)
	require.NoError(t, err)

	// Read README.md and verify key content
	readmePath := filepath.Join(tmpDir, "my-app", "README.md")
	readmeContent, err := os.ReadFile(readmePath)
	require.NoError(t, err)

	// Verify CI Setup section exists
	assert.Contains(t, string(readmeContent), "## CI Setup")
	assert.Contains(t, string(readmeContent), "GitHub Actions")
	assert.Contains(t, string(readmeContent), "OIDC")

	// Verify environment creation instructions
	assert.Contains(t, string(readmeContent), "Create GitHub Environments")
	assert.Contains(t, string(readmeContent), "`dev`")
	assert.Contains(t, string(readmeContent), "`staging`")
	assert.Contains(t, string(readmeContent), "`live`")

	// Verify variable instructions
	assert.Contains(t, string(readmeContent), "AWS_ROLE_ARN")
	assert.Contains(t, string(readmeContent), "AWS_REGION")

	// Verify stage deployment isolation is mentioned
	assert.Contains(t, string(readmeContent), "stage-isolated deployments")

	// Verify quickstart commands
	assert.Contains(t, string(readmeContent), "lift build")
	assert.Contains(t, string(readmeContent), "lift up --stage dev")

	// Verify stages section with domains
	assert.Contains(t, string(readmeContent), "api.dev.example.com")
	assert.Contains(t, string(readmeContent), "api.staging.example.com")
	assert.Contains(t, string(readmeContent), "api.example.com")
}

func TestNewCommandV2_VerifyLiftYAMLFullSchema(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &NewCommandV2{}
	args := []string{"schema-test", "--template", "basic-api", "--base-domain", "myapp.com"}
	err = cmd.Execute(context.Background(), args)
	require.NoError(t, err)

	// Read raw YAML to verify structure
	configPath := filepath.Join(tmpDir, "schema-test", "lift.yaml")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var raw map[string]any
	err = yaml.Unmarshal(data, &raw)
	require.NoError(t, err)

	// Verify required fields
	assert.Equal(t, 1, raw["version"])

	app := raw["app"].(map[string]any)
	assert.Equal(t, "schema-test", app["name"])
	assert.Equal(t, "basic-api", app["template"])

	domains := raw["domains"].(map[string]any)
	assert.Equal(t, "myapp.com", domains["base_domain"])

	stages := raw["stages"].(map[string]any)
	dev := stages["dev"].(map[string]any)
	staging := stages["staging"].(map[string]any)
	live := stages["live"].(map[string]any)
	assert.Equal(t, "dev.myapp.com", dev["root_domain"])
	assert.Equal(t, "staging.myapp.com", staging["root_domain"])
	assert.Equal(t, "myapp.com", live["root_domain"])

	// Verify build config
	build := raw["build"].(map[string]any)
	assert.Equal(t, "linux", build["goos"])
	assert.Equal(t, "arm64", build["goarch"])

	// Verify functions config
	functions := raw["functions"].(map[string]any)
	apiFunc := functions["api"].(map[string]any)
	assert.Equal(t, "./cmd/api", apiFunc["cmd"])
	assert.Equal(t, "./dist/api/bootstrap", apiFunc["out"])

	// Verify CDK config
	cdk := raw["cdk"].(map[string]any)
	assert.Equal(t, "./cdk", cdk["path"])
}

// Helper function to assert file exists
func assertFileExists(t *testing.T, path string) {
	t.Helper()
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		t.Errorf("expected file to exist: %s", path)
	}
}
