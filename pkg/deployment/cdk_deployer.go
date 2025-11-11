package deployment

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// InfrastructureDeployer defines the behavior shared by deployment backends.
type InfrastructureDeployer interface {
	Initialize(ctx context.Context, cfg *StackDeploymentConfig) error
	Deploy(ctx context.Context) (*DeploymentResult, error)
	Destroy(ctx context.Context) (*DeploymentResult, error)
}

// StackDeploymentConfig captures deployment-time configuration for a stack.
type StackDeploymentConfig struct {
	Config          map[string]string `json:"config"`
	Tags            map[string]string `json:"tags"`
	ProjectName     string            `json:"project_name"`
	StackName       string            `json:"stack_name"`
	Region          string            `json:"region"`
	Context         map[string]string `json:"context,omitempty"`
	BackendURL      string            `json:"backend_url,omitempty"`
	SecretsProvider string            `json:"secrets_provider,omitempty"`
}

// DeploymentResult summarizes the outcome of a deployment run.
type DeploymentResult struct {
	Outputs   map[string]any    `json:"outputs"`
	StackName string            `json:"stack_name"`
	Error     string            `json:"error,omitempty"`
	Resources []ResourceSummary `json:"resources"`
	Duration  time.Duration     `json:"duration"`
	Success   bool              `json:"success"`
}

// ResourceSummary provides a high level description of a deployed resource.
type ResourceSummary struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	ARN    string `json:"arn,omitempty"`
	Status string `json:"status,omitempty"`
}

// CDKSynthesizerFunc transforms stack configuration into CDK outputs.
type CDKSynthesizerFunc func(ctx context.Context, cfg *StackDeploymentConfig, infra InfrastructureConfig) (map[string]any, []ResourceSummary, error)

// CDKDeployer orchestrates deployments through Lift CDK constructs.
//
//nolint:govet // suppress fieldalignment noise; layout groups related configuration.
type CDKDeployer struct {
	infraConfig InfrastructureConfig
	config      *StackDeploymentConfig
	synthesizer CDKSynthesizerFunc
	mu          sync.Mutex
	projectName string
	stackName   string
	region      string
	initialized bool
}

// CDKDeployerOption customizes a CDKDeployer instance.
type CDKDeployerOption func(*CDKDeployer)

// WithCDKSynthesizer overrides the default synthesizer used by the deployer.
func WithCDKSynthesizer(fn CDKSynthesizerFunc) CDKDeployerOption {
	return func(d *CDKDeployer) {
		d.synthesizer = fn
	}
}

// NewCDKDeployer constructs a deployer backed by Lift CDK constructs.
func NewCDKDeployer(projectName, stackName, region string, infra InfrastructureConfig, opts ...CDKDeployerOption) *CDKDeployer {
	deployer := &CDKDeployer{
		projectName: projectName,
		stackName:   stackName,
		region:      region,
		infraConfig: infra,
	}

	for _, opt := range opts {
		opt(deployer)
	}

	return deployer
}

// Initialize stores stack level configuration prior to deployment.
func (d *CDKDeployer) Initialize(_ context.Context, cfg *StackDeploymentConfig) error {
	if cfg == nil {
		return fmt.Errorf("stack deployment config is required")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	cloned := cloneStackDeploymentConfig(cfg)
	d.config = cloned
	d.initialized = true

	return nil
}

// Deploy synthesizes the stack and returns generated outputs.
func (d *CDKDeployer) Deploy(ctx context.Context) (*DeploymentResult, error) {
	d.mu.Lock()
	cfg := d.config
	initialized := d.initialized
	synth := d.synthesizer
	d.mu.Unlock()

	if !initialized || cfg == nil {
		return nil, fmt.Errorf("cdk deployer not initialized")
	}

	if synth == nil {
		synth = defaultCDKSynthesizer
	}

	start := time.Now()
	outputs, resources, err := synth(ctx, cfg, d.infraConfig)
	duration := time.Since(start)

	result := &DeploymentResult{
		Outputs:   outputs,
		StackName: cfg.StackName,
		Resources: resources,
		Duration:  duration,
		Success:   err == nil,
	}

	if err != nil {
		result.Error = err.Error()
		return result, fmt.Errorf("cdk deployment failed: %w", err)
	}

	return result, nil
}

// Destroy simulates tearing down provisioned infrastructure.
func (d *CDKDeployer) Destroy(context.Context) (*DeploymentResult, error) {
	// No-op destroy to keep multi-region rollback paths functional during tests.
	return &DeploymentResult{
		Outputs:   map[string]any{},
		StackName: d.stackName,
		Resources: []ResourceSummary{},
		Duration:  0,
		Success:   true,
	}, nil
}

func cloneStackDeploymentConfig(cfg *StackDeploymentConfig) *StackDeploymentConfig {
	cloned := *cfg
	if cfg.Config != nil {
		cloned.Config = make(map[string]string, len(cfg.Config))
		for k, v := range cfg.Config {
			cloned.Config[k] = v
		}
	}
	if cfg.Tags != nil {
		cloned.Tags = make(map[string]string, len(cfg.Tags))
		for k, v := range cfg.Tags {
			cloned.Tags[k] = v
		}
	}
	if cfg.Context != nil {
		cloned.Context = make(map[string]string, len(cfg.Context))
		for k, v := range cfg.Context {
			cloned.Context[k] = v
		}
	}
	return &cloned
}

func defaultCDKSynthesizer(_ context.Context, cfg *StackDeploymentConfig, infra InfrastructureConfig) (map[string]any, []ResourceSummary, error) {
	outputs := map[string]any{
		"StackName": fmt.Sprintf("%s-%s", cfg.ProjectName, cfg.Region),
		"Region":    cfg.Region,
	}

	if endpoint, ok := infra.Metadata["default_endpoint"].(string); ok {
		outputs["ServiceEndpoint"] = endpoint
	} else {
		outputs["ServiceEndpoint"] = fmt.Sprintf("https://%s.%s.lift.internal", cfg.ProjectName, cfg.Region)
	}

	resources := []ResourceSummary{
		{
			Type:   "lift.cdk.Stack",
			Name:   cfg.StackName,
			Status: "synthesized",
		},
	}

	return outputs, resources, nil
}
