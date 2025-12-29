package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pay-theory/lift/internal/liftstate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownCommand_StageValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
		errMsg  string
	}{
		{"invalid prod stage", []string{"--stage", "prod"}, true, "invalid stage"},
		{"invalid test stage", []string{"--stage", "test"}, true, "invalid stage"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			origDir, err := os.Getwd()
			require.NoError(t, err)
			defer func() { _ = os.Chdir(origDir) }()

			liftYAML := `version: 1
app:
  name: stage-test
cdk:
  path: ./cdk
  deploy_order:
    - service
  stacks:
    service:
      name_template: "{{.AppName}}-{{.Stage}}"
`
			require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "cdk"), 0750))
			require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "lift.yaml"), []byte(liftYAML), 0600))
			require.NoError(t, os.Chdir(tmpDir))

			cmd := &DownCommand{cmdFactory: mockCmdFactory(&[]capturedCDKCall{})}
			err = cmd.Execute(context.Background(), tt.args)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestDownCommand_RequiresPartnerWhenTemplateUsesPartner(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()

	liftYAML := `version: 1
app:
  name: partner-required-test

cdk:
  path: ./cdk
  deploy_order:
    - service
  stacks:
    service:
      name_template: "{{.AppName}}-service-{{.Partner}}-{{.Stage}}"
`
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "cdk"), 0750))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "lift.yaml"), []byte(liftYAML), 0600))
	require.NoError(t, os.Chdir(tmpDir))

	var calls []capturedCDKCall
	cmd := &DownCommand{cmdFactory: mockCmdFactory(&calls)}
	err = cmd.Execute(context.Background(), []string{"--stage", "dev"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--partner is required")
	assert.Empty(t, calls, "expected no commands to run when partner preflight fails")
}

func TestDownCommand_ReverseDestroyOrder(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()

	liftYAML := `version: 1
app:
  name: order-test

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
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "cdk"), 0750))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "lift.yaml"), []byte(liftYAML), 0600))
	require.NoError(t, os.Chdir(tmpDir))

	var calls []capturedCDKCall
	cmd := &DownCommand{cmdFactory: mockCmdFactory(&calls)}
	err = cmd.Execute(context.Background(), []string{"--stage", "dev"})
	require.NoError(t, err)

	// Extract CDK destroy calls in order
	var cdkCalls []string
	for _, call := range calls {
		if call.Name == "cdk" && len(call.Args) > 1 && call.Args[0] == "destroy" {
			cdkCalls = append(cdkCalls, call.Args[1])
		}
	}

	// Verify reverse order: api, service, data
	require.Len(t, cdkCalls, 3)
	assert.Equal(t, "order-test-api-dev", cdkCalls[0])
	assert.Equal(t, "order-test-service-dev", cdkCalls[1])
	assert.Equal(t, "order-test-data-dev", cdkCalls[2])
}

func TestDownCommand_ClearsStateFile(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()

	liftYAML := `version: 1
app:
  name: state-test

domains:
  base_domain: example.com

cdk:
  path: ./cdk
  deploy_order:
    - service
  stacks:
    service:
      name_template: "{{.AppName}}-{{.Stage}}"
`
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "cdk"), 0750))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "lift.yaml"), []byte(liftYAML), 0600))

	// Create a pre-existing state file
	state := &liftstate.StageState{
		Stage:           "dev",
		BaseDomain:      "example.com",
		StageRootDomain: "dev.example.com",
	}
	require.NoError(t, liftstate.Save(tmpDir, state))

	// Verify state file exists
	statePath := liftstate.StatePath(tmpDir, "dev")
	require.FileExists(t, statePath)

	require.NoError(t, os.Chdir(tmpDir))

	var calls []capturedCDKCall
	cmd := &DownCommand{cmdFactory: mockCmdFactory(&calls)}
	err = cmd.Execute(context.Background(), []string{"--stage", "dev"})
	require.NoError(t, err)

	// Verify state file was removed
	_, err = os.Stat(statePath)
	require.True(t, os.IsNotExist(err), "state file should be removed")
}

func TestDownCommand_ForceFlag(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()

	liftYAML := `version: 1
app:
  name: force-test

cdk:
  path: ./cdk
  deploy_order:
    - service
  stacks:
    service:
      name_template: "{{.AppName}}-{{.Stage}}"
`
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "cdk"), 0750))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "lift.yaml"), []byte(liftYAML), 0600))
	require.NoError(t, os.Chdir(tmpDir))

	var calls []capturedCDKCall
	cmd := &DownCommand{cmdFactory: mockCmdFactory(&calls)}
	err = cmd.Execute(context.Background(), []string{"--stage", "dev"})
	require.NoError(t, err)

	// Verify --force flag is present in CDK destroy call
	require.Len(t, calls, 1)
	assert.Contains(t, calls[0].Args, "--force")
}

func TestDownCommand_NoCDKStacks(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()

	liftYAML := `version: 1
app:
  name: no-cdk-test
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "lift.yaml"), []byte(liftYAML), 0600))
	require.NoError(t, os.Chdir(tmpDir))

	var calls []capturedCDKCall
	cmd := &DownCommand{cmdFactory: mockCmdFactory(&calls)}
	err = cmd.Execute(context.Background(), []string{"--stage", "dev"})
	require.NoError(t, err)

	// Should have no CDK calls
	assert.Empty(t, calls)
}

func TestDownCommand_CDKContextFlags(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()

	liftYAML := `version: 1
app:
  name: context-test

domains:
  base_domain: example.com

services:
  api:
    subdomain: api

cdk:
  path: ./cdk
  deploy_order:
    - service
  stacks:
    service:
      name_template: "{{.AppName}}-{{.Stage}}"
`
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "cdk"), 0750))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "lift.yaml"), []byte(liftYAML), 0600))
	require.NoError(t, os.Chdir(tmpDir))

	var calls []capturedCDKCall
	cmd := &DownCommand{cmdFactory: mockCmdFactory(&calls)}
	err = cmd.Execute(context.Background(), []string{"--stage", "staging"})
	require.NoError(t, err)

	require.Len(t, calls, 1)
	argsStr := strings.Join(calls[0].Args, " ")

	// Verify context flags
	assert.Contains(t, argsStr, "stage=staging")
	assert.Contains(t, argsStr, "appName=context-test")
	assert.Contains(t, argsStr, "baseDomain=example.com")
	assert.Contains(t, argsStr, "stageRootDomain=staging.example.com")
	assert.Contains(t, argsStr, "serviceDomain.api=api.staging.example.com")
}

func TestDownCommand_AllowsRedeployAfterDown(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()

	// Create project with domains
	liftYAML := `version: 1
app:
  name: redeploy-test

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

	// First deploy
	upCmd := &UpCommand{cmdFactory: mockCmdFactory(&calls)}
	err = upCmd.Execute(context.Background(), []string{"--stage", "dev"})
	require.NoError(t, err)

	// Verify state file exists
	statePath := liftstate.StatePath(tmpDir, "dev")
	require.FileExists(t, statePath)

	// Change domain config
	changedYAML := `version: 1
app:
  name: redeploy-test

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

	// Attempt to deploy should fail
	upCmd2 := &UpCommand{cmdFactory: mockCmdFactory(&calls)}
	err = upCmd2.Execute(context.Background(), []string{"--stage", "dev"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "domain configuration changed")

	// Run down to clear the lock
	downCmd := &DownCommand{cmdFactory: mockCmdFactory(&calls)}
	err = downCmd.Execute(context.Background(), []string{"--stage", "dev"})
	require.NoError(t, err)

	// State file should be gone
	_, err = os.Stat(statePath)
	require.True(t, os.IsNotExist(err))

	// Now deploy should succeed with new domains
	upCmd3 := &UpCommand{cmdFactory: mockCmdFactory(&calls)}
	err = upCmd3.Execute(context.Background(), []string{"--stage", "dev"})
	require.NoError(t, err)

	// Verify new state was written
	state, err := liftstate.Load(tmpDir, "dev")
	require.NoError(t, err)
	assert.Equal(t, "new-domain.com", state.BaseDomain)
}
