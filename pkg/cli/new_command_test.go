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
		lookPath: mockLookPathAlwaysFound,
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

func TestNewCommandV2_BasicAPITemplate_CDKHasDynamoDBByDefault(t *testing.T) {
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

	cdkMainContent, err := os.ReadFile(filepath.Join(appDir, "cdk", "main.go"))
	require.NoError(t, err)
	cdkMain := string(cdkMainContent)

	assert.Contains(t, cdkMain, "NewLiftTable")
	assert.Contains(t, cdkMain, "EnableDynamORM")
}

func TestNewCommandV2_NoDataFlag_DisablesDynamoDBScaffolding(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &NewCommandV2{}
	args := []string{"my-app", "--template", "basic-api", "--base-domain", "example.com", "--no-data"}
	err = cmd.Execute(context.Background(), args)
	require.NoError(t, err)

	appDir := filepath.Join(tmpDir, "my-app")

	cdkMainContent, err := os.ReadFile(filepath.Join(appDir, "cdk", "main.go"))
	require.NoError(t, err)
	cdkMain := string(cdkMainContent)

	assert.NotContains(t, cdkMain, "NewLiftTable")
	assert.NotContains(t, cdkMain, "EnableDynamORM")
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

// =============================================================================
// Tests for new templates: microservice, event-driven, merchant-app
// =============================================================================

func TestNewCommandV2_MicroserviceTemplate_CreatesExpectedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &NewCommandV2{}
	args := []string{"my-microservice", "--template", "microservice", "--base-domain", "example.com"}

	err = cmd.Execute(context.Background(), args)
	require.NoError(t, err)

	appDir := filepath.Join(tmpDir, "my-microservice")

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

func TestNewCommandV2_MicroserviceTemplate_LiftYAMLValid(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &NewCommandV2{}
	args := []string{"my-microservice", "--template", "microservice", "--base-domain", "example.com"}

	err = cmd.Execute(context.Background(), args)
	require.NoError(t, err)

	appDir := filepath.Join(tmpDir, "my-microservice")
	cfg, err := liftconfig.LoadConfig(appDir)
	require.NoError(t, err)

	// Verify template name
	assert.Equal(t, "microservice", cfg.App.Template)
	assert.Equal(t, "my-microservice", cfg.App.Name)

	// Verify stage domains
	assert.Equal(t, "dev.example.com", cfg.Stages["dev"].RootDomain)
	assert.Equal(t, "staging.example.com", cfg.Stages["staging"].RootDomain)
	assert.Equal(t, "example.com", cfg.Stages["live"].RootDomain)

	// Verify functions - microservice has only api
	require.NotNil(t, cfg.Functions)
	require.Contains(t, cfg.Functions, "api")
	assert.Equal(t, "./cmd/api", cfg.Functions["api"].Cmd)
	assert.Equal(t, "./dist/api/bootstrap", cfg.Functions["api"].Out)
}

func TestNewCommandV2_EventDrivenTemplate_CreatesExpectedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &NewCommandV2{}
	args := []string{"my-event-app", "--template", "event-driven", "--base-domain", "example.com"}

	err = cmd.Execute(context.Background(), args)
	require.NoError(t, err)

	appDir := filepath.Join(tmpDir, "my-event-app")

	// Verify expected files exist
	assertFileExists(t, filepath.Join(appDir, "lift.yaml"))
	assertFileExists(t, filepath.Join(appDir, "go.mod"))
	assertFileExists(t, filepath.Join(appDir, "README.md"))
	assertFileExists(t, filepath.Join(appDir, "cmd", "api", "main.go"))
	assertFileExists(t, filepath.Join(appDir, "cmd", "processor", "main.go")) // event-driven has processor
	assertFileExists(t, filepath.Join(appDir, "cdk", "main.go"))
	assertFileExists(t, filepath.Join(appDir, "cdk", "go.mod"))
	assertFileExists(t, filepath.Join(appDir, "cdk", "cdk.json"))
	assertFileExists(t, filepath.Join(appDir, ".gitignore"))
	assertFileExists(t, filepath.Join(appDir, ".github", "workflows", "deploy.yml"))
	assertFileExists(t, filepath.Join(appDir, ".github", "workflows", "pr.yml"))
}

func TestNewCommandV2_EventDrivenTemplate_LiftYAMLValid(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &NewCommandV2{}
	args := []string{"my-event-app", "--template", "event-driven", "--base-domain", "example.com"}

	err = cmd.Execute(context.Background(), args)
	require.NoError(t, err)

	appDir := filepath.Join(tmpDir, "my-event-app")
	cfg, err := liftconfig.LoadConfig(appDir)
	require.NoError(t, err)

	// Verify template name
	assert.Equal(t, "event-driven", cfg.App.Template)
	assert.Equal(t, "my-event-app", cfg.App.Name)

	// Verify stage domains
	assert.Equal(t, "dev.example.com", cfg.Stages["dev"].RootDomain)
	assert.Equal(t, "staging.example.com", cfg.Stages["staging"].RootDomain)
	assert.Equal(t, "example.com", cfg.Stages["live"].RootDomain)

	// Verify functions - event-driven has api and processor
	require.NotNil(t, cfg.Functions)
	require.Contains(t, cfg.Functions, "api")
	require.Contains(t, cfg.Functions, "processor")
	assert.Equal(t, "./cmd/api", cfg.Functions["api"].Cmd)
	assert.Equal(t, "./dist/api/bootstrap", cfg.Functions["api"].Out)
	assert.Equal(t, "./cmd/processor", cfg.Functions["processor"].Cmd)
	assert.Equal(t, "./dist/processor/bootstrap", cfg.Functions["processor"].Out)
}

func TestNewCommandV2_EventDrivenTemplate_CDKHasSQS(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &NewCommandV2{}
	args := []string{"my-event-app", "--template", "event-driven", "--base-domain", "example.com"}

	err = cmd.Execute(context.Background(), args)
	require.NoError(t, err)

	appDir := filepath.Join(tmpDir, "my-event-app")

	// Read CDK main.go and verify SQS resources
	cdkMainContent, err := os.ReadFile(filepath.Join(appDir, "cdk", "main.go"))
	require.NoError(t, err)
	cdkMain := string(cdkMainContent)

	// Verify SQS queue is created
	assert.Contains(t, cdkMain, "awssqs")
	assert.Contains(t, cdkMain, "ProcessingQueue")
	assert.Contains(t, cdkMain, "DeadLetterQueue")
	assert.Contains(t, cdkMain, "SqsEventSource")
}

func TestNewCommandV2_SNSProcessorTemplate_CreatesExpectedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &NewCommandV2{}
	args := []string{"my-sns-app", "--template", "sns-processor", "--base-domain", "example.com"}

	err = cmd.Execute(context.Background(), args)
	require.NoError(t, err)

	appDir := filepath.Join(tmpDir, "my-sns-app")

	// Verify expected files exist
	assertFileExists(t, filepath.Join(appDir, "lift.yaml"))
	assertFileExists(t, filepath.Join(appDir, "go.mod"))
	assertFileExists(t, filepath.Join(appDir, "README.md"))
	assertFileExists(t, filepath.Join(appDir, "cmd", "processor", "main.go"))
	assertFileExists(t, filepath.Join(appDir, "cdk", "main.go"))
	assertFileExists(t, filepath.Join(appDir, "cdk", "go.mod"))
	assertFileExists(t, filepath.Join(appDir, "cdk", "cdk.json"))
	assertFileExists(t, filepath.Join(appDir, ".gitignore"))
	assertFileExists(t, filepath.Join(appDir, ".github", "workflows", "deploy.yml"))
	assertFileExists(t, filepath.Join(appDir, ".github", "workflows", "pr.yml"))
}

func TestNewCommandV2_SNSProcessorTemplate_LiftYAMLValid(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &NewCommandV2{}
	args := []string{"my-sns-app", "--template", "sns-processor", "--base-domain", "example.com"}

	err = cmd.Execute(context.Background(), args)
	require.NoError(t, err)

	appDir := filepath.Join(tmpDir, "my-sns-app")
	cfg, err := liftconfig.LoadConfig(appDir)
	require.NoError(t, err)

	// Verify template name
	assert.Equal(t, "sns-processor", cfg.App.Template)
	assert.Equal(t, "my-sns-app", cfg.App.Name)

	// Verify stage domains
	assert.Equal(t, "dev.example.com", cfg.Stages["dev"].RootDomain)
	assert.Equal(t, "staging.example.com", cfg.Stages["staging"].RootDomain)
	assert.Equal(t, "example.com", cfg.Stages["live"].RootDomain)

	// Verify functions - sns-processor has processor
	require.NotNil(t, cfg.Functions)
	require.Contains(t, cfg.Functions, "processor")
	assert.Equal(t, "./cmd/processor", cfg.Functions["processor"].Cmd)
	assert.Equal(t, "./dist/processor/bootstrap", cfg.Functions["processor"].Out)
}

func TestNewCommandV2_SNSProcessorTemplate_CDKHasSNSAndDynamoDB(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &NewCommandV2{}
	args := []string{"my-sns-app", "--template", "sns-processor", "--base-domain", "example.com"}

	err = cmd.Execute(context.Background(), args)
	require.NoError(t, err)

	appDir := filepath.Join(tmpDir, "my-sns-app")

	// Read CDK main.go and verify SNS + DynamoDB via Lift constructs
	cdkMainContent, err := os.ReadFile(filepath.Join(appDir, "cdk", "main.go"))
	require.NoError(t, err)
	cdkMain := string(cdkMainContent)

	assert.Contains(t, cdkMain, "awssns")
	assert.Contains(t, cdkMain, "NewSNSProcessor")
	assert.Contains(t, cdkMain, "NewLiftTable")
}

func TestNewCommandV2_MerchantAppTemplate_CreatesExpectedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &NewCommandV2{}
	args := []string{"my-merchant-app", "--template", "merchant-app", "--base-domain", "example.com"}

	err = cmd.Execute(context.Background(), args)
	require.NoError(t, err)

	appDir := filepath.Join(tmpDir, "my-merchant-app")

	// Verify expected files exist
	assertFileExists(t, filepath.Join(appDir, "lift.yaml"))
	assertFileExists(t, filepath.Join(appDir, "go.mod"))
	assertFileExists(t, filepath.Join(appDir, "README.md"))
	assertFileExists(t, filepath.Join(appDir, "cmd", "api", "main.go"))
	assertFileExists(t, filepath.Join(appDir, "cmd", "worker", "main.go")) // merchant-app has worker
	assertFileExists(t, filepath.Join(appDir, "cdk", "main.go"))
	assertFileExists(t, filepath.Join(appDir, "cdk", "go.mod"))
	assertFileExists(t, filepath.Join(appDir, "cdk", "cdk.json"))
	assertFileExists(t, filepath.Join(appDir, ".gitignore"))
	assertFileExists(t, filepath.Join(appDir, ".github", "workflows", "deploy.yml"))
	assertFileExists(t, filepath.Join(appDir, ".github", "workflows", "pr.yml"))
}

func TestNewCommandV2_MerchantAppTemplate_LiftYAMLValid(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &NewCommandV2{}
	args := []string{"my-merchant-app", "--template", "merchant-app", "--base-domain", "example.com"}

	err = cmd.Execute(context.Background(), args)
	require.NoError(t, err)

	appDir := filepath.Join(tmpDir, "my-merchant-app")
	cfg, err := liftconfig.LoadConfig(appDir)
	require.NoError(t, err)

	// Verify template name
	assert.Equal(t, "merchant-app", cfg.App.Template)
	assert.Equal(t, "my-merchant-app", cfg.App.Name)

	// Verify stage domains
	assert.Equal(t, "dev.example.com", cfg.Stages["dev"].RootDomain)
	assert.Equal(t, "staging.example.com", cfg.Stages["staging"].RootDomain)
	assert.Equal(t, "example.com", cfg.Stages["live"].RootDomain)

	// Verify functions - merchant-app has api and worker
	require.NotNil(t, cfg.Functions)
	require.Contains(t, cfg.Functions, "api")
	require.Contains(t, cfg.Functions, "worker")
	assert.Equal(t, "./cmd/api", cfg.Functions["api"].Cmd)
	assert.Equal(t, "./dist/api/bootstrap", cfg.Functions["api"].Out)
	assert.Equal(t, "./cmd/worker", cfg.Functions["worker"].Cmd)
	assert.Equal(t, "./dist/worker/bootstrap", cfg.Functions["worker"].Out)
}

func TestNewCommandV2_MerchantAppTemplate_MultiStackDeployOrder(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &NewCommandV2{}
	args := []string{"my-merchant-app", "--template", "merchant-app", "--base-domain", "example.com"}

	err = cmd.Execute(context.Background(), args)
	require.NoError(t, err)

	appDir := filepath.Join(tmpDir, "my-merchant-app")
	cfg, err := liftconfig.LoadConfig(appDir)
	require.NoError(t, err)

	// Verify multi-stack deploy order
	require.NotNil(t, cfg.CDK)
	require.NotNil(t, cfg.CDK.DeployOrder)
	require.Len(t, cfg.CDK.DeployOrder, 2)
	assert.Equal(t, "data", cfg.CDK.DeployOrder[0])
	assert.Equal(t, "service", cfg.CDK.DeployOrder[1])

	// Verify stack configurations
	require.NotNil(t, cfg.CDK.Stacks)
	require.Contains(t, cfg.CDK.Stacks, "data")
	require.Contains(t, cfg.CDK.Stacks, "service")
}

func TestNewCommandV2_MerchantAppTemplate_CDKHasDynamoDB(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &NewCommandV2{}
	args := []string{"my-merchant-app", "--template", "merchant-app", "--base-domain", "example.com"}

	err = cmd.Execute(context.Background(), args)
	require.NoError(t, err)

	appDir := filepath.Join(tmpDir, "my-merchant-app")

	// Read CDK main.go and verify DynamoDB resources
	cdkMainContent, err := os.ReadFile(filepath.Join(appDir, "cdk", "main.go"))
	require.NoError(t, err)
	cdkMain := string(cdkMainContent)

	// Verify DynamoDB table is created
	assert.Contains(t, cdkMain, "awsdynamodb")
	assert.Contains(t, cdkMain, "MainTable")
	assert.Contains(t, cdkMain, "dataStack")
	assert.Contains(t, cdkMain, "serviceStack")
	assert.Contains(t, cdkMain, "AddDependency")
}

func TestNewCommandV2_PTModeSupportsNewTemplates(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	tests := []struct {
		name          string
		template      string
		appName       string
		wantFunctions []string
		wantStacks    []string
	}{
		{
			name:          "microservice",
			template:      "microservice",
			appName:       "pt-microservice",
			wantFunctions: []string{"api"},
			wantStacks:    []string{"service"},
		},
		{
			name:          "event-driven",
			template:      "event-driven",
			appName:       "pt-event-driven",
			wantFunctions: []string{"api", "processor"},
			wantStacks:    []string{"service"},
		},
		{
			name:          "sns-processor",
			template:      "sns-processor",
			appName:       "pt-sns-processor",
			wantFunctions: []string{"processor"},
			wantStacks:    []string{"service"},
		},
		{
			name:          "merchant-app",
			template:      "merchant-app",
			appName:       "pt-merchant-app",
			wantFunctions: []string{"api", "worker"},
			wantStacks:    []string{"data", "service"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &NewCommandV2{}
			args := []string{tt.appName, "--template", tt.template, "--base-domain", "example.com", "--pt"}

			err := cmd.Execute(context.Background(), args)
			require.NoError(t, err)

			appDir := filepath.Join(tmpDir, tt.appName)

			// Verify PT devops files exist
			assertFileExists(t, filepath.Join(appDir, "buildspec.yml"))
			assertFileExists(t, filepath.Join(appDir, "shell", "build.sh"))
			assertFileExists(t, filepath.Join(appDir, "shell", "deploy.sh"))
			assertFileExists(t, filepath.Join(appDir, "shell", "init_env_vars.sh"))
			assertFileExists(t, filepath.Join(appDir, "shell", "DEPLOYMENT.md"))

			// Verify GitHub workflows are NOT generated in PT mode
			_, err = os.Stat(filepath.Join(appDir, ".github"))
			assert.True(t, os.IsNotExist(err), ".github should not exist in PT mode")

			// Verify lift.yaml + cdk main exist
			assertFileExists(t, filepath.Join(appDir, "lift.yaml"))
			assertFileExists(t, filepath.Join(appDir, "cdk", "main.go"))

			// Verify lift.yaml app/template and functions
			cfg, err := liftconfig.LoadConfig(appDir)
			require.NoError(t, err)
			assert.Equal(t, tt.appName, cfg.App.Name)
			assert.Equal(t, tt.template, cfg.App.Template)

			for _, fn := range tt.wantFunctions {
				require.Contains(t, cfg.Functions, fn)
			}
			for _, stack := range tt.wantStacks {
				require.Contains(t, cfg.CDK.Stacks, stack)
			}

			// Verify stack name templates include partner marker (enables CLI preflight)
			liftYAMLContent, err := os.ReadFile(filepath.Join(appDir, "lift.yaml"))
			require.NoError(t, err)
			assert.Contains(t, string(liftYAMLContent), "{{.Partner}}")

			// Verify CDK app reads partner context
			cdkMainContent, err := os.ReadFile(filepath.Join(appDir, "cdk", "main.go"))
			require.NoError(t, err)
			assert.Contains(t, string(cdkMainContent), "partner")
			assert.Contains(t, string(cdkMainContent), "targetMode")

			// Verify buildspec includes partner/stage vars
			buildspecContent, err := os.ReadFile(filepath.Join(appDir, "buildspec.yml"))
			require.NoError(t, err)
			buildspec := string(buildspecContent)
			assert.Contains(t, buildspec, "PARTNER")
			assert.Contains(t, buildspec, "STAGE")
			assert.Contains(t, buildspec, "TARGET_MODE")

			// Verify buildspec deploys expected stacks
			if tt.template == "merchant-app" {
				assert.Contains(t, buildspec, tt.appName+"-data-$PARTNER-$STAGE")
				assert.Contains(t, buildspec, tt.appName+"-service-$PARTNER-$STAGE")
			} else {
				assert.Contains(t, buildspec, tt.appName+"-service-$PARTNER-$STAGE")
			}
		})
	}
}

func TestNewCommandV2_AllTemplatesListedInUsage(t *testing.T) {
	cmd := &NewCommandV2{}
	usage := cmd.Usage()

	// Verify all templates are mentioned in usage
	assert.Contains(t, usage, "basic-api")
	assert.Contains(t, usage, "microservice")
	assert.Contains(t, usage, "event-driven")
	assert.Contains(t, usage, "merchant-app")
	assert.Contains(t, usage, "sns-processor")
	assert.Contains(t, usage, "--no-data")
}
