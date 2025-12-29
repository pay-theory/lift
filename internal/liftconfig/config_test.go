package liftconfig

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_ValidConfig(t *testing.T) {
	root := t.TempDir()

	configContent := `version: 1
app:
  name: my-test-app
  template: basic-api
domains:
  base_domain: example.com
stages:
  dev:
    root_domain: dev.example.com
  live:
    root_domain: example.com
`
	err := os.WriteFile(filepath.Join(root, ConfigFileName), []byte(configContent), 0600)
	require.NoError(t, err)

	cfg, err := LoadConfig(root)
	require.NoError(t, err)

	assert.Equal(t, 1, cfg.Version)
	assert.Equal(t, "my-test-app", cfg.App.Name)
	assert.Equal(t, "basic-api", cfg.App.Template)
	assert.Equal(t, "example.com", cfg.Domains.BaseDomain)
	assert.Equal(t, "dev.example.com", cfg.Stages["dev"].RootDomain)
	assert.Equal(t, "example.com", cfg.Stages["live"].RootDomain)
}

func TestLoadConfig_MinimalConfig(t *testing.T) {
	root := t.TempDir()

	// Minimal valid config with just version and app.name
	configContent := `version: 1
app:
  name: minimal-app
`
	err := os.WriteFile(filepath.Join(root, ConfigFileName), []byte(configContent), 0600)
	require.NoError(t, err)

	cfg, err := LoadConfig(root)
	require.NoError(t, err)

	assert.Equal(t, 1, cfg.Version)
	assert.Equal(t, "minimal-app", cfg.App.Name)
	assert.Nil(t, cfg.Domains)
	assert.Nil(t, cfg.Stages)
}

func TestLoadConfig_MissingFile(t *testing.T) {
	root := t.TempDir() // Empty directory

	cfg, err := LoadConfig(root)
	assert.Nil(t, cfg)
	require.Error(t, err)

	var notFoundErr *ConfigNotFoundError
	require.True(t, errors.As(err, &notFoundErr), "expected ConfigNotFoundError")
	assert.Contains(t, notFoundErr.Error(), ConfigFileName)
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	root := t.TempDir()

	// Invalid YAML content
	configContent := `version: 1
app:
  name: [this is invalid yaml
    - unbalanced
`
	err := os.WriteFile(filepath.Join(root, ConfigFileName), []byte(configContent), 0600)
	require.NoError(t, err)

	cfg, err := LoadConfig(root)
	assert.Nil(t, cfg)
	require.Error(t, err)

	var parseErr *ConfigParseError
	require.True(t, errors.As(err, &parseErr), "expected ConfigParseError")
	assert.Contains(t, parseErr.Error(), "failed to parse")
}

func TestLoadConfig_WithBuildConfig(t *testing.T) {
	root := t.TempDir()

	configContent := `version: 1
app:
  name: build-test-app
build:
  goos: linux
  goarch: arm64
  cgo: 0
  ldflags: "-s -w"
  tags:
    - lambda.norpc
`
	err := os.WriteFile(filepath.Join(root, ConfigFileName), []byte(configContent), 0600)
	require.NoError(t, err)

	cfg, err := LoadConfig(root)
	require.NoError(t, err)

	require.NotNil(t, cfg.Build)
	assert.Equal(t, "linux", cfg.Build.GOOS)
	assert.Equal(t, "arm64", cfg.Build.GOARCH)
	assert.Equal(t, 0, cfg.Build.CGO)
	assert.Equal(t, "-s -w", cfg.Build.LDFlags)
	assert.Contains(t, cfg.Build.Tags, "lambda.norpc")
}

func TestLoadConfig_WithCDKConfig(t *testing.T) {
	root := t.TempDir()

	configContent := `version: 1
app:
  name: cdk-test-app
cdk:
  path: ./cdk
  deploy_order:
    - data
    - service
  stacks:
    data:
      name_template: "{{.AppName}}-data-{{.Stage}}"
    service:
      name_template: "{{.AppName}}-service-{{.Stage}}"
`
	err := os.WriteFile(filepath.Join(root, ConfigFileName), []byte(configContent), 0600)
	require.NoError(t, err)

	cfg, err := LoadConfig(root)
	require.NoError(t, err)

	require.NotNil(t, cfg.CDK)
	assert.Equal(t, "./cdk", cfg.CDK.Path)
	assert.Equal(t, []string{"data", "service"}, cfg.CDK.DeployOrder)
	require.Contains(t, cfg.CDK.Stacks, "data")
	assert.Equal(t, "{{.AppName}}-data-{{.Stage}}", cfg.CDK.Stacks["data"].NameTemplate)
}

func TestConfigParseError_Unwrap(t *testing.T) {
	underlying := errors.New("underlying error")
	parseErr := &ConfigParseError{Path: "/test/lift.yaml", Err: underlying}

	assert.True(t, errors.Is(parseErr, underlying))
}
