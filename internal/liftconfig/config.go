// Package liftconfig provides configuration parsing and project root detection
// for the Lift CLI. It implements the lift.yaml contract defined in
// docs/planning/lift-cli-contract-v1.md.
package liftconfig

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ConfigFileName is the required configuration file name
const ConfigFileName = "lift.yaml"

// Config represents the parsed lift.yaml configuration
type Config struct {
	Version   int                  `yaml:"version"`
	App       AppConfig            `yaml:"app"`
	Domains   *Domains             `yaml:"domains,omitempty"`
	Stages    StageMap             `yaml:"stages,omitempty"`
	Services  map[string]*Service  `yaml:"services,omitempty"`
	Build     *Build               `yaml:"build,omitempty"`
	Functions map[string]*Function `yaml:"functions,omitempty"`
	CDK       *CDKConfig           `yaml:"cdk,omitempty"`
}

// AppConfig contains application metadata
type AppConfig struct {
	Name     string `yaml:"name"`
	Template string `yaml:"template,omitempty"`
}

// Domains contains domain configuration
type Domains struct {
	BaseDomain string `yaml:"base_domain,omitempty"`
}

// StageConfig represents a single stage's configuration
type StageConfig struct {
	RootDomain string `yaml:"root_domain,omitempty"`
}

// StageMap is a map of stage name to stage config
type StageMap map[string]StageConfig

// Service represents a service subdomain configuration
type Service struct {
	Subdomain string `yaml:"subdomain,omitempty"`
}

// Build contains build configuration
type Build struct {
	GOOS     string   `yaml:"goos,omitempty"`
	GOARCH   string   `yaml:"goarch,omitempty"`
	CGO      int      `yaml:"cgo,omitempty"`
	Trimpath *bool    `yaml:"trimpath,omitempty"`
	LDFlags  string   `yaml:"ldflags,omitempty"`
	Tags     []string `yaml:"tags,omitempty"`
}

// Function represents a Lambda function build configuration
type Function struct {
	Cmd string `yaml:"cmd,omitempty"`
	Out string `yaml:"out,omitempty"`
}

// CDKConfig contains CDK deployment configuration
type CDKConfig struct {
	Path        string            `yaml:"path,omitempty"`
	DeployOrder []string          `yaml:"deploy_order,omitempty"`
	Stacks      map[string]*Stack `yaml:"stacks,omitempty"`
}

// Stack represents a CDK stack configuration
type Stack struct {
	NameTemplate string `yaml:"name_template,omitempty"`
}

// LoadConfig reads and parses the lift.yaml file from the given root directory.
// Returns an error if the file doesn't exist or is not valid YAML.
func LoadConfig(root string) (*Config, error) {
	configPath := filepath.Join(root, ConfigFileName)

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &ConfigNotFoundError{Path: configPath}
		}
		return nil, fmt.Errorf("failed to read %s: %w", ConfigFileName, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, &ConfigParseError{Path: configPath, Err: err}
	}

	return &cfg, nil
}

// ConfigNotFoundError is returned when lift.yaml is not found
type ConfigNotFoundError struct {
	Path string
}

func (e *ConfigNotFoundError) Error() string {
	return fmt.Sprintf("%s not found at %s", ConfigFileName, e.Path)
}

// ConfigParseError is returned when lift.yaml cannot be parsed
type ConfigParseError struct {
	Path string
	Err  error
}

func (e *ConfigParseError) Error() string {
	return fmt.Sprintf("failed to parse %s: %v", e.Path, e.Err)
}

func (e *ConfigParseError) Unwrap() error {
	return e.Err
}
