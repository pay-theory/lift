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

// =============================================================================
// Tests for lift add function
// =============================================================================

func TestAddCommand_FunctionCreatesMainGo(t *testing.T) {
	// Set up a test project
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "test-project")
	require.NoError(t, os.MkdirAll(projectDir, 0750))

	// Create minimal lift.yaml
	liftYAML := `version: 1
app:
  name: test-app
  template: basic-api
functions:
  api:
    cmd: ./cmd/api
    out: ./dist/api/bootstrap
cdk:
  path: ./cdk
  deploy_order: [service]
  stacks:
    service:
      name_template: "{{.AppName}}-service-{{.Stage}}"
`
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "lift.yaml"), []byte(liftYAML), 0644))

	// Create cdk/main.go with markers
	cdkDir := filepath.Join(projectDir, "cdk")
	require.NoError(t, os.MkdirAll(cdkDir, 0750))
	cdkMainGo := `package main

import "fmt"

func main() {
	stack := "test"
	appNameStr := "test-app"
	stageStr := "dev"
	fmt.Println(stack, appNameStr, stageStr)

	// LIFT:ADD_FUNCTIONS

	// LIFT:ADD_OUTPUTS

	app.Synth(nil)
}
`
	require.NoError(t, os.WriteFile(filepath.Join(cdkDir, "main.go"), []byte(cdkMainGo), 0644))

	// Run lift add function
	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(projectDir))
	defer os.Chdir(origDir)

	cmd := &AddCommand{}
	err := cmd.Execute(context.Background(), []string{"function", "webhook"})
	require.NoError(t, err)

	// Verify cmd/webhook/main.go was created
	mainGoPath := filepath.Join(projectDir, "cmd", "webhook", "main.go")
	assert.FileExists(t, mainGoPath)

	// Check content
	content, err := os.ReadFile(mainGoPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "package main")
	assert.Contains(t, string(content), "lift.New()")
	assert.Contains(t, string(content), "test-app-webhook")
}

func TestAddCommand_FunctionUpdatesLiftYAML(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "test-project")
	require.NoError(t, os.MkdirAll(projectDir, 0750))

	// Create minimal lift.yaml
	liftYAML := `version: 1
app:
  name: test-app
  template: basic-api
functions:
  api:
    cmd: ./cmd/api
    out: ./dist/api/bootstrap
cdk:
  path: ./cdk
  deploy_order: [service]
  stacks:
    service:
      name_template: "{{.AppName}}-service-{{.Stage}}"
`
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "lift.yaml"), []byte(liftYAML), 0644))

	// Create cdk/main.go with markers
	cdkDir := filepath.Join(projectDir, "cdk")
	require.NoError(t, os.MkdirAll(cdkDir, 0750))
	cdkMainGo := `package main

func main() {
	// LIFT:ADD_FUNCTIONS
	// LIFT:ADD_OUTPUTS
	app.Synth(nil)
}
`
	require.NoError(t, os.WriteFile(filepath.Join(cdkDir, "main.go"), []byte(cdkMainGo), 0644))

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(projectDir))
	defer os.Chdir(origDir)

	cmd := &AddCommand{}
	err := cmd.Execute(context.Background(), []string{"function", "payment"})
	require.NoError(t, err)

	// Load and verify lift.yaml
	cfg, err := liftconfig.LoadConfig(projectDir)
	require.NoError(t, err)

	assert.Contains(t, cfg.Functions, "payment")
	paymentFn := cfg.Functions["payment"]
	assert.Equal(t, "./cmd/payment", paymentFn.Cmd)
	assert.Equal(t, "./dist/payment/bootstrap", paymentFn.Out)

	// Original function should still exist
	assert.Contains(t, cfg.Functions, "api")
}

func TestAddCommand_FunctionUpdatesCDK(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "test-project")
	require.NoError(t, os.MkdirAll(projectDir, 0750))

	liftYAML := `version: 1
app:
  name: test-app
  template: basic-api
functions:
  api:
    cmd: ./cmd/api
    out: ./dist/api/bootstrap
cdk:
  path: ./cdk
  deploy_order: [service]
  stacks:
    service:
      name_template: "{{.AppName}}-service-{{.Stage}}"
`
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "lift.yaml"), []byte(liftYAML), 0644))

	cdkDir := filepath.Join(projectDir, "cdk")
	require.NoError(t, os.MkdirAll(cdkDir, 0750))
	cdkMainGo := `package main

import "fmt"

func main() {
	stack := "test"
	appNameStr := "test-app"
	stageStr := "dev"
	fmt.Println(stack, appNameStr, stageStr)

	// LIFT:ADD_FUNCTIONS

	// LIFT:ADD_OUTPUTS

	app.Synth(nil)
}
`
	require.NoError(t, os.WriteFile(filepath.Join(cdkDir, "main.go"), []byte(cdkMainGo), 0644))

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(projectDir))
	defer os.Chdir(origDir)

	cmd := &AddCommand{}
	err := cmd.Execute(context.Background(), []string{"function", "notification"})
	require.NoError(t, err)

	// Verify cdk/main.go was updated
	cdkContent, err := os.ReadFile(filepath.Join(cdkDir, "main.go"))
	require.NoError(t, err)

	contentStr := string(cdkContent)
	assert.Contains(t, contentStr, "notificationFunction")
	assert.Contains(t, contentStr, "_ = notificationFunction")
	assert.Contains(t, contentStr, `"notification"`)
	assert.NotContains(t, contentStr, "NotificationFunctionArn")
	assert.NotContains(t, contentStr, "NewCfnOutput(")
}

func TestAddCommand_FunctionErrorIfAlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "test-project")
	require.NoError(t, os.MkdirAll(projectDir, 0750))

	liftYAML := `version: 1
app:
  name: test-app
  template: basic-api
functions:
  api:
    cmd: ./cmd/api
    out: ./dist/api/bootstrap
`
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "lift.yaml"), []byte(liftYAML), 0644))

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(projectDir))
	defer os.Chdir(origDir)

	cmd := &AddCommand{}
	err := cmd.Execute(context.Background(), []string{"function", "api"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists in lift.yaml")
}

func TestAddCommand_FunctionErrorIfCmdDirExists(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "test-project")
	require.NoError(t, os.MkdirAll(projectDir, 0750))

	liftYAML := `version: 1
app:
  name: test-app
  template: basic-api
functions:
  api:
    cmd: ./cmd/api
    out: ./dist/api/bootstrap
`
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "lift.yaml"), []byte(liftYAML), 0644))

	// Create cmd/existing directory
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, "cmd", "existing"), 0750))

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(projectDir))
	defer os.Chdir(origDir)

	cmd := &AddCommand{}
	err := cmd.Execute(context.Background(), []string{"function", "existing"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cmd/existing already exists")
}

func TestAddCommand_FunctionValidatesName(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		errorPart   string
	}{
		{"valid lowercase", "webhook", false, ""},
		{"valid with underscore", "payment_processor", false, ""},
		{"valid with numbers", "handler2", false, ""},
		{"invalid slash", "web/hook", true, "path separators"},
		{"invalid dotdot", "web..hook", true, "path separators"},
		{"invalid backslash", "web\\hook", true, "path separators"},
		{"invalid starts with number", "2handler", true, "valid identifier"},
		{"invalid special chars", "web-hook", true, "valid identifier"},
		{"reserved main", "main", true, "reserved"},
		{"reserved init", "init", true, "reserved"},
		{"empty name", "", true, "cannot be empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFunctionName(tt.input)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorPart)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAddCommand_FunctionNoLiftYAML(t *testing.T) {
	tmpDir := t.TempDir()

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	cmd := &AddCommand{}
	err := cmd.Execute(context.Background(), []string{"function", "webhook"})
	require.Error(t, err)
	// Should return the standard "not a Lift project" error
	assert.Contains(t, err.Error(), "lift.yaml")
}

// =============================================================================
// Tests for PT mode support
// =============================================================================

func TestAddCommand_PTModeUpdatesBuildFiles(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "test-project")
	require.NoError(t, os.MkdirAll(projectDir, 0750))

	// PT mode is detected by stack name templates referencing {{.Partner}}
	liftYAML := `version: 1
app:
  name: test-app
  template: basic-api-pt
functions:
  api:
    cmd: ./cmd/api
    out: ./dist/api/bootstrap
cdk:
  path: ./cdk
  deploy_order: [service]
  stacks:
    service:
      name_template: "{{.AppName}}-service-{{.Partner}}-{{.Stage}}"
`
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "lift.yaml"), []byte(liftYAML), 0644))

	// Create cdk/main.go with PT markers
	cdkDir := filepath.Join(projectDir, "cdk")
	require.NoError(t, os.MkdirAll(cdkDir, 0750))
	cdkMainGo := `package main

import "fmt"

func main() {
	stack := "test"
	appNameStr := "test-app"
	stageStr := "dev"
	partnerStr := "partner1"
	targetModeStr := "standard"
	fmt.Println(stack, appNameStr, stageStr, partnerStr, targetModeStr)

	// LIFT:ADD_FUNCTIONS

	// LIFT:ADD_OUTPUTS

	app.Synth(nil)
}
`
	require.NoError(t, os.WriteFile(filepath.Join(cdkDir, "main.go"), []byte(cdkMainGo), 0644))

	// Create buildspec.yml
	buildspec := `version: 0.2
phases:
  build:
    commands:
      - echo "Building api..."
      - go build -o dist/api/bootstrap ./cmd/api
      # LIFT:ADD_FUNCTION_BUILDS
      - echo "Synthesizing CDK stacks..."
`
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "buildspec.yml"), []byte(buildspec), 0644))

	// Create shell/build.sh
	shellDir := filepath.Join(projectDir, "shell")
	require.NoError(t, os.MkdirAll(shellDir, 0750))
	buildsh := `#!/bin/bash
# Build API
go build -o dist/api/bootstrap ./cmd/api

# LIFT:ADD_FUNCTION_BUILDS

echo "Functions built:"
echo "   - dist/api/bootstrap"
# LIFT:ADD_OUTPUT_LINES
`
	require.NoError(t, os.WriteFile(filepath.Join(shellDir, "build.sh"), []byte(buildsh), 0750))

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(projectDir))
	defer os.Chdir(origDir)

	cmd := &AddCommand{}
	err := cmd.Execute(context.Background(), []string{"function", "worker"})
	require.NoError(t, err)

	// Verify buildspec.yml was updated
	buildspecContent, err := os.ReadFile(filepath.Join(projectDir, "buildspec.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(buildspecContent), "dist/worker")
	assert.Contains(t, string(buildspecContent), "./cmd/worker")

	// Parse YAML to ensure the inserted build steps are distinct commands (not folded into a prior line)
	var buildspecDoc struct {
		Phases struct {
			Build struct {
				Commands []string `yaml:"commands"`
			} `yaml:"build"`
		} `yaml:"phases"`
	}
	require.NoError(t, yaml.Unmarshal(buildspecContent, &buildspecDoc))
	commands := buildspecDoc.Phases.Build.Commands
	assert.Contains(t, commands, "mkdir -p dist/worker")
	assert.Contains(t, commands, `echo "Building worker..."`)

	foundWorkerBuild := false
	for _, cmdStr := range commands {
		if strings.Contains(cmdStr, "dist/worker/bootstrap") && strings.Contains(cmdStr, "./cmd/worker") {
			foundWorkerBuild = true
		}
		// Regression guard: avoid YAML folding that turns the next list item into a continuation line.
		assert.NotContains(t, cmdStr, "./cmd/api - mkdir -p dist/worker")
	}
	assert.True(t, foundWorkerBuild, "expected a distinct go build command for worker")

	// Verify shell/build.sh was updated
	buildshContent, err := os.ReadFile(filepath.Join(shellDir, "build.sh"))
	require.NoError(t, err)
	assert.Contains(t, string(buildshContent), "dist/worker")
	assert.Contains(t, string(buildshContent), "./cmd/worker")

	// Verify cdk/main.go has PT-style code with partnerStr
	cdkContent, err := os.ReadFile(filepath.Join(cdkDir, "main.go"))
	require.NoError(t, err)
	assert.Contains(t, string(cdkContent), "partnerStr")
	assert.Contains(t, string(cdkContent), "_ = workerFunction")
}

// =============================================================================
// Integration test: lift new + lift add function + lift build
// =============================================================================

func TestAddCommand_IntegrationWithNewAndBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "my-test-app")

	// Set up isolated Go cache
	goCache := filepath.Join(tmpDir, "go-cache")
	goModCache := filepath.Join(tmpDir, "go-mod-cache")
	os.MkdirAll(goCache, 0750)
	os.MkdirAll(goModCache, 0750)

	// Run lift new
	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))

	newCmd := &NewCommandV2{}
	err := newCmd.Execute(context.Background(), []string{
		"my-test-app",
		"--base-domain", "example.com",
		"--template", "basic-api",
	})
	require.NoError(t, err)

	require.NoError(t, os.Chdir(projectDir))
	defer os.Chdir(origDir)

	// Run lift add function foo
	addCmd := &AddCommand{}
	err = addCmd.Execute(context.Background(), []string{"function", "foo"})
	require.NoError(t, err)

	// Verify cmd/foo/main.go exists
	assert.FileExists(t, filepath.Join(projectDir, "cmd", "foo", "main.go"))

	// Replace cmd/foo/main.go with simple compileable code (avoids module downloads)
	simpleMain := "package main\n\nfunc main() {}\n"
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "cmd", "foo", "main.go"), []byte(simpleMain), 0644))

	// Also replace cmd/api/main.go to avoid module downloads
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "cmd", "api", "main.go"), []byte(simpleMain), 0644))

	// Run lift build with mocked go command
	buildCmd := &BuildCommand{
		cmdFactory: func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			cmd := exec.CommandContext(ctx, name, arg...)
			cmd.Env = append(os.Environ(),
				"GOCACHE="+goCache,
				"GOMODCACHE="+goModCache,
			)
			return cmd
		},
	}
	err = buildCmd.Execute(context.Background(), nil)
	require.NoError(t, err)

	// Verify dist/foo/bootstrap exists
	assert.FileExists(t, filepath.Join(projectDir, "dist", "foo", "bootstrap"))
	assert.FileExists(t, filepath.Join(projectDir, "dist", "api", "bootstrap"))
}

func TestAddCommand_IntegrationPTMode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "pt-test-app")

	// Set up isolated Go cache
	goCache := filepath.Join(tmpDir, "go-cache")
	goModCache := filepath.Join(tmpDir, "go-mod-cache")
	os.MkdirAll(goCache, 0750)
	os.MkdirAll(goModCache, 0750)

	// Run lift new --pt
	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))

	newCmd := &NewCommandV2{}
	err := newCmd.Execute(context.Background(), []string{
		"pt-test-app",
		"--base-domain", "example.com",
		"--template", "basic-api",
		"--pt",
	})
	require.NoError(t, err)

	require.NoError(t, os.Chdir(projectDir))
	defer os.Chdir(origDir)

	// Run lift add function bar
	addCmd := &AddCommand{}
	err = addCmd.Execute(context.Background(), []string{"function", "bar"})
	require.NoError(t, err)

	// Verify cmd/bar/main.go exists
	assert.FileExists(t, filepath.Join(projectDir, "cmd", "bar", "main.go"))

	// Verify buildspec.yml includes bar
	buildspecContent, err := os.ReadFile(filepath.Join(projectDir, "buildspec.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(buildspecContent), "dist/bar")

	// Verify shell/build.sh includes bar
	buildshContent, err := os.ReadFile(filepath.Join(projectDir, "shell", "build.sh"))
	require.NoError(t, err)
	assert.Contains(t, string(buildshContent), "dist/bar")

	// Replace function files with simple code
	simpleMain := "package main\n\nfunc main() {}\n"
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "cmd", "bar", "main.go"), []byte(simpleMain), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "cmd", "api", "main.go"), []byte(simpleMain), 0644))

	// Run lift build
	buildCmd := &BuildCommand{
		cmdFactory: func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			cmd := exec.CommandContext(ctx, name, arg...)
			cmd.Env = append(os.Environ(),
				"GOCACHE="+goCache,
				"GOMODCACHE="+goModCache,
			)
			return cmd
		},
	}
	err = buildCmd.Execute(context.Background(), nil)
	require.NoError(t, err)

	// Verify dist/bar/bootstrap exists
	assert.FileExists(t, filepath.Join(projectDir, "dist", "bar", "bootstrap"))
}

// =============================================================================
// Helper function tests
// =============================================================================

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"webhook", "Webhook"},
		{"payment_processor", "PaymentProcessor"},
		{"api", "Api"},
		{"my_cool_function", "MyCoolFunction"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toPascalCase(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToLowerCamel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"webhook", "webhook"},
		{"payment_processor", "paymentProcessor"},
		{"Api", "api"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toLowerCamel(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// Tests for detectStackVariable - critical for correct CDK code generation
// =============================================================================

func TestDetectStackVariable(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "detects stack from NewLiftFunction",
			content: `
				apiFunction := liftcdk.NewLiftFunction(stack, jsii.String("ApiFunction"), &liftcdk.LiftFunctionProps{`,
			expected: "stack",
		},
		{
			name: "detects serviceStack from NewLiftFunction",
			content: `
				apiFunction := liftcdk.NewLiftFunction(serviceStack, jsii.String("ApiFunction"), &liftcdk.LiftFunctionProps{`,
			expected: "serviceStack",
		},
		{
			name: "does NOT mis-detect serviceStackName as serviceStack",
			content: `
				serviceStackName := fmt.Sprintf("%s-service-%s", appNameStr, stageStr)
				stack := awscdk.NewStack(app, jsii.String(serviceStackName), &awscdk.StackProps{
				apiFunction := liftcdk.NewLiftFunction(stack, jsii.String("ApiFunction"), &liftcdk.LiftFunctionProps{`,
			expected: "stack",
		},
		{
			name: "merchant-app with serviceStack variable",
			content: `
				// Create stack name
				serviceStackName := fmt.Sprintf("%s-service-%s", appNameStr, stageStr)
				// Create the service stack
				serviceStack := awscdk.NewStack(app, jsii.String(serviceStackName), &awscdk.StackProps{
				// Create the API Lambda function
				apiFunction := liftcdk.NewLiftFunction(serviceStack, jsii.String("ApiFunction"), &liftcdk.LiftFunctionProps{`,
			expected: "serviceStack",
		},
		{
			name: "fallback to stack when no NewLiftFunction found",
			content: `
				package main
				func main() {
					app.Synth(nil)
				}`,
			expected: "stack",
		},
		{
			name: "uses CfnOutput as fallback detection",
			content: `
				awscdk.NewCfnOutput(dataStack, jsii.String("TableArn"), &awscdk.CfnOutputProps{`,
			expected: "dataStack",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectStackVariable(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// REGRESSION TEST: serviceStackName vs serviceStack detection
// This ensures we don't generate code that references undefined variables
// =============================================================================

func TestAddCommand_CDKStackVariableRegressionServiceStackName(t *testing.T) {
	// This test reproduces the bug where serviceStackName (a string variable)
	// was incorrectly detected as the stack variable, causing lift add to
	// generate code that references undefined "serviceStack" variable.
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "test-project")
	require.NoError(t, os.MkdirAll(projectDir, 0750))

	liftYAML := `version: 1
app:
  name: test-app
  template: basic-api
functions:
  api:
    cmd: ./cmd/api
    out: ./dist/api/bootstrap
cdk:
  path: ./cdk
  deploy_order: [service]
  stacks:
    service:
      name_template: "{{.AppName}}-service-{{.Stage}}"
`
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "lift.yaml"), []byte(liftYAML), 0644))

	// This CDK file has "serviceStackName" (just a string) but uses "stack"
	// as the actual stack variable - mimics basic-api template structure
	cdkDir := filepath.Join(projectDir, "cdk")
	require.NoError(t, os.MkdirAll(cdkDir, 0750))
	cdkMainGo := `package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/jsii-runtime-go"
	liftcdk "github.com/pay-theory/lift/pkg/cdk/constructs"
)

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	appNameStr := "test-app"
	stageStr := "dev"

	// Create stack name - THIS IS JUST A STRING, NOT THE STACK
	serviceStackName := fmt.Sprintf("%s-service-%s", appNameStr, stageStr)

	// Create the service stack - THIS IS THE ACTUAL STACK VARIABLE
	stack := awscdk.NewStack(app, jsii.String(serviceStackName), &awscdk.StackProps{
		Env: env(),
	})

	// Create the API Lambda function
	apiFunction := liftcdk.NewLiftFunction(stack, jsii.String("ApiFunction"), &liftcdk.LiftFunctionProps{
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String(fmt.Sprintf("%s-api-%s", appNameStr, stageStr)),
			Code:         awslambda.Code_FromAsset(jsii.String(filepath.Join("..", "dist", "api")), nil),
			Handler:      jsii.String("bootstrap"),
		},
	})

	// LIFT:ADD_FUNCTIONS

	awscdk.NewCfnOutput(stack, jsii.String("ApiFunctionArn"), &awscdk.CfnOutputProps{
		Value: apiFunction.Function.FunctionArn(),
	})

	// LIFT:ADD_OUTPUTS

	app.Synth(nil)
}

func env() *awscdk.Environment {
	return &awscdk.Environment{
		Account: jsii.String(os.Getenv("CDK_DEFAULT_ACCOUNT")),
		Region:  jsii.String(os.Getenv("CDK_DEFAULT_REGION")),
	}
}
`
	require.NoError(t, os.WriteFile(filepath.Join(cdkDir, "main.go"), []byte(cdkMainGo), 0644))

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(projectDir))
	defer os.Chdir(origDir)

	cmd := &AddCommand{}
	err := cmd.Execute(context.Background(), []string{"function", "webhook"})
	require.NoError(t, err)

	// Read the updated CDK file
	cdkContent, err := os.ReadFile(filepath.Join(cdkDir, "main.go"))
	require.NoError(t, err)
	contentStr := string(cdkContent)

	// THE CRITICAL ASSERTION: Generated code MUST use "stack", NOT "serviceStack"
	// If this assertion fails, cdk synth/deploy would fail with undefined variable error
	assert.Contains(t, contentStr, "liftcdk.NewLiftFunction(stack,",
		"Generated function must use 'stack' variable, not 'serviceStack'")

	// Ensure we did NOT accidentally generate code using serviceStack
	// Count occurrences - should only have the ones from original code
	assert.NotContains(t, contentStr, "NewLiftFunction(serviceStack,",
		"Must NOT generate code with undefined serviceStack variable")
}

func TestAddCommand_Usage(t *testing.T) {
	cmd := &AddCommand{}
	assert.Equal(t, "add", cmd.Name())
	assert.Contains(t, cmd.Description(), "Add components")
	assert.Contains(t, cmd.Usage(), "lift add function")
}

func TestAddCommand_UnknownSubcommand(t *testing.T) {
	cmd := &AddCommand{}
	err := cmd.Execute(context.Background(), []string{"unknown"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown subcommand")
}

func TestAddCommand_NoSubcommand(t *testing.T) {
	cmd := &AddCommand{}
	err := cmd.Execute(context.Background(), []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "subcommand required")
}

func TestAddCommand_FunctionNoName(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "test-project")
	require.NoError(t, os.MkdirAll(projectDir, 0750))

	liftYAML := `version: 1
app:
  name: test-app
`
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "lift.yaml"), []byte(liftYAML), 0644))

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(projectDir))
	defer os.Chdir(origDir)

	cmd := &AddCommand{}
	err := cmd.Execute(context.Background(), []string{"function"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "function name required")
}

// =============================================================================
// Test YAML node editing preserves structure
// =============================================================================

func TestAddCommand_YAMLPreservesExistingContent(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "test-project")
	require.NoError(t, os.MkdirAll(projectDir, 0750))

	// Create lift.yaml with comments and specific formatting
	liftYAML := `version: 1
app:
  name: test-app
  template: basic-api
domains:
  base_domain: example.com
stages:
  dev:
    root_domain: dev.example.com
  staging:
    root_domain: staging.example.com
  live:
    root_domain: example.com
functions:
  api:
    cmd: ./cmd/api
    out: ./dist/api/bootstrap
cdk:
  path: ./cdk
  deploy_order: [service]
  stacks:
    service:
      name_template: "{{.AppName}}-service-{{.Stage}}"
`
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "lift.yaml"), []byte(liftYAML), 0644))

	// Create minimal cdk/main.go
	cdkDir := filepath.Join(projectDir, "cdk")
	require.NoError(t, os.MkdirAll(cdkDir, 0750))
	cdkMainGo := `package main
func main() {
	// LIFT:ADD_FUNCTIONS
	// LIFT:ADD_OUTPUTS
	app.Synth(nil)
}
`
	require.NoError(t, os.WriteFile(filepath.Join(cdkDir, "main.go"), []byte(cdkMainGo), 0644))

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(projectDir))
	defer os.Chdir(origDir)

	cmd := &AddCommand{}
	err := cmd.Execute(context.Background(), []string{"function", "processor"})
	require.NoError(t, err)

	// Load updated config and verify all sections are preserved
	cfg, err := liftconfig.LoadConfig(projectDir)
	require.NoError(t, err)

	// Verify original content preserved
	assert.Equal(t, "test-app", cfg.App.Name)
	assert.Equal(t, "example.com", cfg.Domains.BaseDomain)
	assert.Contains(t, cfg.Functions, "api")
	assert.Contains(t, cfg.Functions, "processor")
	assert.Equal(t, "dev.example.com", cfg.Stages["dev"].RootDomain)

	// Read raw YAML to verify structure
	content, err := os.ReadFile(filepath.Join(projectDir, "lift.yaml"))
	require.NoError(t, err)

	// Parse as generic YAML to verify structure
	var yamlData map[string]interface{}
	err = yaml.Unmarshal(content, &yamlData)
	require.NoError(t, err)

	// Verify functions section has both entries
	functions, ok := yamlData["functions"].(map[string]interface{})
	require.True(t, ok, "functions should be a map")
	assert.Contains(t, functions, "api")
	assert.Contains(t, functions, "processor")
}

// =============================================================================
// Test CDK update fallback (no markers)
// =============================================================================

func TestAddCommand_CDKFallbackInsertionBeforeSynth(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "test-project")
	require.NoError(t, os.MkdirAll(projectDir, 0750))

	liftYAML := `version: 1
app:
  name: test-app
  template: basic-api
functions:
  api:
    cmd: ./cmd/api
    out: ./dist/api/bootstrap
cdk:
  path: ./cdk
  deploy_order: [service]
  stacks:
    service:
      name_template: "{{.AppName}}-service-{{.Stage}}"
`
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "lift.yaml"), []byte(liftYAML), 0644))

	// Create cdk/main.go WITHOUT markers
	cdkDir := filepath.Join(projectDir, "cdk")
	require.NoError(t, os.MkdirAll(cdkDir, 0750))
	cdkMainGo := `package main

import "fmt"

func main() {
	stack := "test"
	appNameStr := "test-app"
	stageStr := "dev"
	fmt.Println(stack, appNameStr, stageStr)

	app.Synth(nil)
}
`
	require.NoError(t, os.WriteFile(filepath.Join(cdkDir, "main.go"), []byte(cdkMainGo), 0644))

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(projectDir))
	defer os.Chdir(origDir)

	cmd := &AddCommand{}
	err := cmd.Execute(context.Background(), []string{"function", "fallback"})
	require.NoError(t, err)

	// Verify cdk/main.go was updated with the function code before app.Synth
	cdkContent, err := os.ReadFile(filepath.Join(cdkDir, "main.go"))
	require.NoError(t, err)

	contentStr := string(cdkContent)
	assert.Contains(t, contentStr, "fallbackFunction")
	assert.NotContains(t, contentStr, "FallbackFunctionArn")
	assert.NotContains(t, contentStr, "NewCfnOutput(")

	// Verify the function code appears before app.Synth
	fnIndex := strings.Index(contentStr, "fallbackFunction")
	synthIndex := strings.Index(contentStr, "app.Synth")
	assert.Less(t, fnIndex, synthIndex, "function should be inserted before app.Synth")
}
