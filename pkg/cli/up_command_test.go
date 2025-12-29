package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pay-theory/lift/internal/domains"
	"github.com/pay-theory/lift/internal/liftconfig"
	"github.com/pay-theory/lift/internal/liftstate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capturedCDKCall records a CDK invocation for test assertions
type capturedCDKCall struct {
	Name string
	Args []string
	Dir  string
}

// mockCmdFactory creates a factory that records CDK calls and simulates success
func mockCmdFactory(calls *[]capturedCDKCall) func(ctx context.Context, name string, arg ...string) *exec.Cmd {
	return func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		*calls = append(*calls, capturedCDKCall{Name: name, Args: arg})
		// Return a command that succeeds (true exits with 0)
		return exec.CommandContext(ctx, "true")
	}
}

// setupUpDownTestProject creates a minimal project for up/down testing
func setupUpDownTestProject(t *testing.T, dir string, liftYAML string) {
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

	// Create cdk directory
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "cdk"), 0750))

	// Create lift.yaml
	require.NoError(t, os.WriteFile(filepath.Join(dir, "lift.yaml"), []byte(liftYAML), 0600))
}

func TestUpCommand_StageValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
		errMsg  string
	}{
		{"valid dev stage", []string{"--stage", "dev"}, false, ""},
		{"valid staging stage", []string{"--stage", "staging"}, false, ""},
		{"valid live stage", []string{"--stage", "live"}, false, ""},
		{"invalid prod stage", []string{"--stage", "prod"}, true, "invalid stage"},
		{"default to dev", []string{}, false, ""},
		{"equals form", []string{"--stage=staging"}, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr {
				// For error cases, we don't need a full project
				tmpDir := t.TempDir()
				origDir, err := os.Getwd()
				require.NoError(t, err)
				defer func() { _ = os.Chdir(origDir) }()

				// Create minimal project
				liftYAML := `version: 1
app:
  name: stage-test
functions:
  api:
    cmd: ./cmd/api
    out: ./dist/api/bootstrap
`
				setupUpDownTestProject(t, tmpDir, liftYAML)
				require.NoError(t, os.Chdir(tmpDir))

				cmd := &UpCommand{cmdFactory: mockCmdFactory(&[]capturedCDKCall{})}
				err = cmd.Execute(context.Background(), tt.args)
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			}
		})
	}
}

func TestUpCommand_DomainResolution(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()

	liftYAML := `version: 1
app:
  name: domain-test
  template: basic-api

domains:
  base_domain: example.com

services:
  api:
    subdomain: api

functions:
  api:
    cmd: ./cmd/api
    out: ./dist/api/bootstrap

cdk:
  path: ./cdk
  deploy_order:
    - service
  stacks:
    service:
      name_template: "{{.AppName}}-{{.Stage}}"
`
	setupUpDownTestProject(t, tmpDir, liftYAML)
	require.NoError(t, os.Chdir(tmpDir))

	var calls []capturedCDKCall
	cmd := &UpCommand{cmdFactory: mockCmdFactory(&calls)}
	err = cmd.Execute(context.Background(), []string{"--stage", "dev"})
	require.NoError(t, err)

	// Find the CDK deploy call (skip "go" build calls)
	var cdkCall *capturedCDKCall
	for i := range calls {
		if calls[i].Name == "cdk" {
			cdkCall = &calls[i]
			break
		}
	}
	require.NotNil(t, cdkCall, "expected a CDK call")

	// Verify context flags include resolved domains
	argsStr := strings.Join(cdkCall.Args, " ")
	assert.Contains(t, argsStr, "stage=dev")
	assert.Contains(t, argsStr, "appName=domain-test")
	assert.Contains(t, argsStr, "baseDomain=example.com")
	assert.Contains(t, argsStr, "stageRootDomain=dev.example.com")
	assert.Contains(t, argsStr, "serviceDomain.api=api.dev.example.com")
}

func TestUpCommand_StackDeployOrder(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()

	liftYAML := `version: 1
app:
  name: order-test

functions:
  api:
    cmd: ./cmd/api
    out: ./dist/api/bootstrap

cdk:
  path: ./cdk
  deploy_order:
    - data
    - service
    - api
  stacks:
    data:
      name_template: "{{.AppName}}-data-{{.Stage}}"
    service:
      name_template: "{{.AppName}}-service-{{.Stage}}"
    api:
      name_template: "{{.AppName}}-api-{{.Stage}}"
`
	setupUpDownTestProject(t, tmpDir, liftYAML)
	require.NoError(t, os.Chdir(tmpDir))

	var calls []capturedCDKCall
	cmd := &UpCommand{cmdFactory: mockCmdFactory(&calls)}
	err = cmd.Execute(context.Background(), []string{"--stage", "dev"})
	require.NoError(t, err)

	// Extract CDK deploy calls in order
	var cdkCalls []string
	for _, call := range calls {
		if call.Name == "cdk" && len(call.Args) > 1 && call.Args[0] == "deploy" {
			cdkCalls = append(cdkCalls, call.Args[1])
		}
	}

	// Verify order: data, service, api
	require.Len(t, cdkCalls, 3)
	assert.Equal(t, "order-test-data-dev", cdkCalls[0])
	assert.Equal(t, "order-test-service-dev", cdkCalls[1])
	assert.Equal(t, "order-test-api-dev", cdkCalls[2])
}

func TestUpCommand_StateFileCreated(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()

	liftYAML := `version: 1
app:
  name: state-test

domains:
  base_domain: example.com

services:
  api:
    subdomain: api

functions:
  api:
    cmd: ./cmd/api
    out: ./dist/api/bootstrap

cdk:
  path: ./cdk
  deploy_order:
    - service
  stacks:
    service:
      name_template: "{{.AppName}}-{{.Stage}}"
`
	setupUpDownTestProject(t, tmpDir, liftYAML)
	require.NoError(t, os.Chdir(tmpDir))

	var calls []capturedCDKCall
	cmd := &UpCommand{cmdFactory: mockCmdFactory(&calls)}
	err = cmd.Execute(context.Background(), []string{"--stage", "dev"})
	require.NoError(t, err)

	// Verify state file was created
	statePath := liftstate.StatePath(tmpDir, "dev")
	require.FileExists(t, statePath)

	// Load and verify content
	state, err := liftstate.Load(tmpDir, "dev")
	require.NoError(t, err)
	require.NotNil(t, state)

	assert.Equal(t, "dev", state.Stage)
	assert.Equal(t, "example.com", state.BaseDomain)
	assert.Equal(t, "dev.example.com", state.StageRootDomain)
	assert.Equal(t, "api.dev.example.com", state.Services["api"].Domain)
}

func TestUpCommand_DomainLockEnforcement(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()

	// Initial deployment
	liftYAML := `version: 1
app:
  name: lock-test

domains:
  base_domain: example.com

services:
  api:
    subdomain: api

functions:
  api:
    cmd: ./cmd/api
    out: ./dist/api/bootstrap

cdk:
  path: ./cdk
  deploy_order:
    - service
  stacks:
    service:
      name_template: "{{.AppName}}-{{.Stage}}"
`
	setupUpDownTestProject(t, tmpDir, liftYAML)
	require.NoError(t, os.Chdir(tmpDir))

	var calls []capturedCDKCall
	cmd := &UpCommand{cmdFactory: mockCmdFactory(&calls)}
	err = cmd.Execute(context.Background(), []string{"--stage", "dev"})
	require.NoError(t, err)

	// Now change the domain config
	changedYAML := `version: 1
app:
  name: lock-test

domains:
  base_domain: new-domain.com

services:
  api:
    subdomain: api

functions:
  api:
    cmd: ./cmd/api
    out: ./dist/api/bootstrap

cdk:
  path: ./cdk
  deploy_order:
    - service
  stacks:
    service:
      name_template: "{{.AppName}}-{{.Stage}}"
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "lift.yaml"), []byte(changedYAML), 0600))

	// Attempt to deploy again - should fail
	cmd2 := &UpCommand{cmdFactory: mockCmdFactory(&calls)}
	err = cmd2.Execute(context.Background(), []string{"--stage", "dev"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "domain configuration changed")
	assert.Contains(t, err.Error(), "lift down --stage dev")
}

func TestUpCommand_NoCDKStacks(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()

	liftYAML := `version: 1
app:
  name: no-cdk-test

functions:
  api:
    cmd: ./cmd/api
    out: ./dist/api/bootstrap
`
	setupUpDownTestProject(t, tmpDir, liftYAML)
	require.NoError(t, os.Chdir(tmpDir))

	var calls []capturedCDKCall
	cmd := &UpCommand{cmdFactory: mockCmdFactory(&calls)}
	err = cmd.Execute(context.Background(), []string{"--stage", "dev"})
	require.NoError(t, err)

	// Should have no CDK calls (only go build)
	for _, call := range calls {
		assert.NotEqual(t, "cdk", call.Name)
	}
}

func TestUpCommand_StackNameTemplateRendering(t *testing.T) {
	cmd := &UpCommand{}

	tests := []struct {
		name     string
		template string
		appName  string
		stage    string
		want     string
		wantErr  bool
	}{
		{"basic template", "{{.AppName}}-{{.Stage}}", "myapp", "dev", "myapp-dev", false},
		{"data stack template", "{{.AppName}}-data-{{.Stage}}", "myapp", "live", "myapp-data-live", false},
		{"prefix only uses appName", "prefix-{{.AppName}}", "app", "staging", "prefix-app", false},
		{"undefined field produces empty", "{{.InvalidField}}", "app", "dev", "<no value>", false}, // Go templates don't error on undefined map keys
		{"empty template errors", "", "app", "dev", "", true},
		{"invalid template syntax", "{{.AppName", "app", "dev", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cmd.renderStackName(tt.template, tt.appName, tt.stage)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestUpCommand_CDKContextFlags(t *testing.T) {
	cmd := &UpCommand{}

	cfg := &liftconfig.Config{
		App: liftconfig.AppConfig{Name: "test-app"},
	}

	t.Run("without domains", func(t *testing.T) {
		args := cmd.buildCDKArgs("deploy", "test-stack", cfg, "dev", nil)

		assert.Contains(t, args, "deploy")
		assert.Contains(t, args, "test-stack")
		assert.Contains(t, args, "--require-approval")
		assert.Contains(t, args, "never")

		// Find context values
		var stageFound, appNameFound bool
		for i, arg := range args {
			if arg == "--context" && i+1 < len(args) {
				if args[i+1] == "stage=dev" {
					stageFound = true
				}
				if args[i+1] == "appName=test-app" {
					appNameFound = true
				}
			}
		}
		assert.True(t, stageFound)
		assert.True(t, appNameFound)
	})

	t.Run("with domains", func(t *testing.T) {
		resolved := &domains.ResolvedDomains{
			BaseDomain:      "example.com",
			StageRootDomain: "dev.example.com",
			Services:        map[string]string{"api": "api.dev.example.com"},
		}

		args := cmd.buildCDKArgs("deploy", "test-stack", cfg, "dev", resolved)

		// Convert to searchable string for easier testing
		argsStr := strings.Join(args, " ")
		assert.Contains(t, argsStr, "baseDomain=example.com")
		assert.Contains(t, argsStr, "stageRootDomain=dev.example.com")
		assert.Contains(t, argsStr, "serviceDomain.api=api.dev.example.com")
	})
}
