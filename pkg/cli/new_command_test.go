package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestNewCommandV2_PTModeGeneratesBuildspec(t *testing.T) {
	// Create temp dir as working directory
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &NewCommandV2{}
	args := []string{"pt-app", "--base-domain", "example.com", "--pt"}

	err = cmd.Execute(context.Background(), args)
	require.NoError(t, err)

	appDir := filepath.Join(tmpDir, "pt-app")

	// Verify PT devops files exist
	assertFileExists(t, filepath.Join(appDir, "buildspec.yml"))
	assertFileExists(t, filepath.Join(appDir, "shell", "build.sh"))
	assertFileExists(t, filepath.Join(appDir, "shell", "deploy.sh"))
	assertFileExists(t, filepath.Join(appDir, "shell", "init_env_vars.sh"))
	assertFileExists(t, filepath.Join(appDir, "shell", "DEPLOYMENT.md"))

	// Verify GitHub workflows are NOT generated in PT mode
	_, err = os.Stat(filepath.Join(appDir, ".github", "workflows", "deploy.yml"))
	assert.True(t, os.IsNotExist(err), "deploy.yml should not exist in PT mode")
	_, err = os.Stat(filepath.Join(appDir, ".github", "workflows", "pr.yml"))
	assert.True(t, os.IsNotExist(err), "pr.yml should not exist in PT mode")

	// Verify core project files still exist
	assertFileExists(t, filepath.Join(appDir, "lift.yaml"))
	assertFileExists(t, filepath.Join(appDir, "go.mod"))
	assertFileExists(t, filepath.Join(appDir, "cmd", "api", "main.go"))
	assertFileExists(t, filepath.Join(appDir, "cdk", "main.go"))
}

func TestNewCommandV2_PTModeBuildspecContent(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &NewCommandV2{}
	args := []string{"content-test", "--base-domain", "example.com", "--pt"}
	err = cmd.Execute(context.Background(), args)
	require.NoError(t, err)

	appDir := filepath.Join(tmpDir, "content-test")

	// Read buildspec.yml and verify key content
	buildspecContent, err := os.ReadFile(filepath.Join(appDir, "buildspec.yml"))
	require.NoError(t, err)
	content := string(buildspecContent)

	// Verify CodeBuild env vars
	assert.Contains(t, content, "PARTNER")
	assert.Contains(t, content, "STAGE")
	assert.Contains(t, content, "TARGET_MODE")

	// Verify cdk deploy or synth
	assert.True(t, strings.Contains(content, "cdk deploy") || strings.Contains(content, "cdk synth"))

	// Verify Go 1.25 or higher
	assert.Contains(t, content, "golang: 1.25")

	// Verify lift.yaml contains partner in stack name template
	liftYAMLContent, err := os.ReadFile(filepath.Join(appDir, "lift.yaml"))
	require.NoError(t, err)
	liftYAML := string(liftYAMLContent)
	assert.Contains(t, liftYAML, "{{.Partner}}", "lift.yaml should include {{.Partner}} in stack name template")

	// Verify CDK main.go reads partner context
	cdkMainContent, err := os.ReadFile(filepath.Join(appDir, "cdk", "main.go"))
	require.NoError(t, err)
	cdkMain := string(cdkMainContent)
	assert.Contains(t, cdkMain, "partner", "cdk/main.go should read partner context")
	assert.Contains(t, cdkMain, "targetMode", "cdk/main.go should read targetMode context")
}

func TestNewCommandV2_PTModeShellScriptsContent(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &NewCommandV2{}
	args := []string{"shell-test", "--base-domain", "example.com", "--pt"}
	err = cmd.Execute(context.Background(), args)
	require.NoError(t, err)

	appDir := filepath.Join(tmpDir, "shell-test")

	// Verify deploy.sh content
	deployContent, err := os.ReadFile(filepath.Join(appDir, "shell", "deploy.sh"))
	require.NoError(t, err)
	deployStr := string(deployContent)

	assert.Contains(t, deployStr, "--partner")
	assert.Contains(t, deployStr, "--stage")
	assert.Contains(t, deployStr, "--target-mode")
	assert.Contains(t, deployStr, "deploy|destroy|diff|synth|bootstrap")
	assert.Contains(t, deployStr, "dev|staging|live")

	// Verify build.sh content
	buildContent, err := os.ReadFile(filepath.Join(appDir, "shell", "build.sh"))
	require.NoError(t, err)
	buildStr := string(buildContent)

	assert.Contains(t, buildStr, "GOOS=linux")
	assert.Contains(t, buildStr, "GOARCH=arm64")
	assert.Contains(t, buildStr, "-mod=mod")

	// Verify init_env_vars.sh content
	initContent, err := os.ReadFile(filepath.Join(appDir, "shell", "init_env_vars.sh"))
	require.NoError(t, err)
	initStr := string(initContent)

	assert.Contains(t, initStr, "PARTNER")
	assert.Contains(t, initStr, "STAGE")
	assert.Contains(t, initStr, "TARGET_MODE")
}

func TestNewCommandV2_PTModeShellScriptsExecutable(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &NewCommandV2{}
	args := []string{"exec-test", "--base-domain", "example.com", "--pt"}
	err = cmd.Execute(context.Background(), args)
	require.NoError(t, err)

	appDir := filepath.Join(tmpDir, "exec-test")

	// Verify shell scripts have executable permissions
	for _, script := range []string{"build.sh", "deploy.sh", "init_env_vars.sh"} {
		info, err := os.Stat(filepath.Join(appDir, "shell", script))
		require.NoError(t, err)
		// Check for owner execute bit (0100)
		assert.True(t, info.Mode()&0100 != 0, "%s should be executable", script)
	}
}

func TestNewCommandV2_PTModeREADMEContent(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &NewCommandV2{}
	args := []string{"readme-test", "--base-domain", "example.com", "--pt"}
	err = cmd.Execute(context.Background(), args)
	require.NoError(t, err)

	appDir := filepath.Join(tmpDir, "readme-test")

	// Verify README.md exists
	assertFileExists(t, filepath.Join(appDir, "README.md"))

	// Read README.md and verify it's PT-focused
	readmeContent, err := os.ReadFile(filepath.Join(appDir, "README.md"))
	require.NoError(t, err)
	readmeStr := string(readmeContent)

	// Verify PT-specific content
	assert.Contains(t, readmeStr, "Pay Theory-style deployment")
	assert.Contains(t, readmeStr, "CodeBuild")
	assert.Contains(t, readmeStr, "buildspec.yml")
	assert.Contains(t, readmeStr, "./shell/deploy.sh")
	assert.Contains(t, readmeStr, "--partner")
	assert.Contains(t, readmeStr, "PARTNER")
	assert.Contains(t, readmeStr, "STAGE")
	assert.Contains(t, readmeStr, "lift up --stage dev --partner")

	// Verify GitHub Actions content is NOT present
	assert.NotContains(t, readmeStr, "GitHub Actions")
	assert.NotContains(t, readmeStr, "vars.AWS_ROLE_ARN")
	assert.NotContains(t, readmeStr, "GitHub Environments")
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
