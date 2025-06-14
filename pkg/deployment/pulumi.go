package deployment

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// PulumiDeployer handles Pulumi-specific deployments using CLI automation
type PulumiDeployer struct {
	projectName   string
	stackName     string
	region        string
	config        InfrastructureConfig
	deploymentLog []DeploymentLogEntry
	workspaceDir  string
	pulumiCmd     string
}

// DeploymentLogEntry represents a deployment log entry
type DeploymentLogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Resource  string    `json:"resource,omitempty"`
	Operation string    `json:"operation,omitempty"`
	Status    string    `json:"status,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// PulumiStackConfig holds Pulumi stack configuration
type PulumiStackConfig struct {
	ProjectName     string            `json:"project_name"`
	StackName       string            `json:"stack_name"`
	Region          string            `json:"region"`
	BackendURL      string            `json:"backend_url,omitempty"`
	SecretsProvider string            `json:"secrets_provider,omitempty"`
	Config          map[string]string `json:"config"`
	Tags            map[string]string `json:"tags"`
}

// DeploymentResult represents the result of a deployment
type DeploymentResult struct {
	Success   bool                   `json:"success"`
	StackName string                 `json:"stack_name"`
	Outputs   map[string]interface{} `json:"outputs"`
	Resources []ResourceSummary      `json:"resources"`
	Duration  time.Duration          `json:"duration"`
	Error     string                 `json:"error,omitempty"`
	Permalink string                 `json:"permalink,omitempty"`
	UpdateID  string                 `json:"update_id,omitempty"`
}

// ResourceSummary represents a summary of a deployed resource
type ResourceSummary struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	URN    string `json:"urn"`
	Status string `json:"status"`
}

// PulumiOperationResult represents the result of a Pulumi CLI operation
type PulumiOperationResult struct {
	Version   int                    `json:"version"`
	Kind      string                 `json:"kind"`
	Stack     string                 `json:"stack"`
	Project   string                 `json:"project"`
	Result    string                 `json:"result"`
	Outputs   map[string]interface{} `json:"outputs"`
	Resources []PulumiResource       `json:"resources"`
}

// PulumiResource represents a resource in Pulumi output
type PulumiResource struct {
	URN    string `json:"urn"`
	Type   string `json:"type"`
	Custom bool   `json:"custom"`
	ID     string `json:"id,omitempty"`
}

// NewPulumiDeployer creates a new Pulumi deployer with CLI automation
func NewPulumiDeployer(projectName, stackName, region string, config InfrastructureConfig) *PulumiDeployer {
	// Find Pulumi CLI command
	pulumiCmd := "pulumi"
	if path, err := exec.LookPath("pulumi"); err == nil {
		pulumiCmd = path
	}

	return &PulumiDeployer{
		projectName:   projectName,
		stackName:     stackName,
		region:        region,
		config:        config,
		deploymentLog: make([]DeploymentLogEntry, 0),
		workspaceDir:  getWorkspaceDir(),
		pulumiCmd:     pulumiCmd,
	}
}

// Initialize initializes the Pulumi workspace and stack using CLI
func (pd *PulumiDeployer) Initialize(ctx context.Context, stackConfig PulumiStackConfig) error {
	pd.logEntry("INFO", "Initializing Pulumi workspace", stackConfig.StackName, "initialize", "started", "")

	// Check if Pulumi CLI is available
	if err := pd.checkPulumiCLI(); err != nil {
		pd.logEntry("ERROR", "Pulumi CLI not available", stackConfig.StackName, "initialize", "failed", err.Error())
		return fmt.Errorf("pulumi CLI not available: %w", err)
	}

	// Set backend URL if specified
	if stackConfig.BackendURL != "" {
		if err := pd.runPulumiCommand(ctx, "login", stackConfig.BackendURL); err != nil {
			pd.logEntry("ERROR", "Failed to set backend", stackConfig.StackName, "login", "failed", err.Error())
			return fmt.Errorf("failed to set backend: %w", err)
		}
	}

	// Initialize or select stack
	stackFullName := fmt.Sprintf("%s/%s", pd.projectName, pd.stackName)

	// Try to select existing stack first
	if err := pd.runPulumiCommand(ctx, "stack", "select", stackFullName); err != nil {
		// If stack doesn't exist, create it
		if err := pd.runPulumiCommand(ctx, "stack", "init", stackFullName); err != nil {
			errorMsg := fmt.Sprintf("failed to create stack: %v", err)
			pd.logEntry("ERROR", errorMsg, stackConfig.StackName, "stack_init", "failed", errorMsg)
			return fmt.Errorf("failed to create stack: %w", err)
		}
		pd.logEntry("INFO", "Created new Pulumi stack", stackConfig.StackName, "stack_init", "completed", "")
	} else {
		pd.logEntry("INFO", "Selected existing Pulumi stack", stackConfig.StackName, "stack_select", "completed", "")
	}

	// Set stack configuration
	for key, value := range stackConfig.Config {
		if err := pd.runPulumiCommand(ctx, "config", "set", key, value); err != nil {
			errorMsg := fmt.Sprintf("failed to set config %s: %v", key, err)
			pd.logEntry("ERROR", errorMsg, stackConfig.StackName, "config", "failed", errorMsg)
			return fmt.Errorf("failed to set config %s: %w", key, err)
		}
	}

	// Set stack tags
	for key, value := range stackConfig.Tags {
		if err := pd.runPulumiCommand(ctx, "stack", "tag", "set", key, value); err != nil {
			errorMsg := fmt.Sprintf("failed to set tag %s: %v", key, err)
			pd.logEntry("WARN", errorMsg, stackConfig.StackName, "tag", "failed", errorMsg)
			// Don't fail on tag errors, just log warning
		}
	}

	pd.logEntry("INFO", "Pulumi workspace initialized successfully", stackConfig.StackName, "initialize", "completed", "")
	return nil
}

// Deploy deploys the infrastructure using Pulumi CLI
func (pd *PulumiDeployer) Deploy(ctx context.Context) (*DeploymentResult, error) {
	startTime := time.Now()
	pd.logEntry("INFO", "Starting Pulumi deployment", pd.stackName, "deploy", "started", "")

	// Execute deployment with JSON output
	output, err := pd.runPulumiCommandWithOutput(ctx, "up", "--yes", "--json")
	if err != nil {
		errorMsg := fmt.Sprintf("deployment failed: %v", err)
		pd.logEntry("ERROR", errorMsg, pd.stackName, "deploy", "failed", errorMsg)

		return &DeploymentResult{
			Success:   false,
			StackName: pd.stackName,
			Duration:  time.Since(startTime),
			Error:     errorMsg,
		}, err
	}

	// Parse Pulumi output
	result, err := pd.parsePulumiOutput(output)
	if err != nil {
		pd.logEntry("WARN", "Failed to parse Pulumi output", pd.stackName, "parse", "failed", err.Error())
		result = &PulumiOperationResult{} // Continue with empty result
	}

	// Get stack outputs
	outputs, err := pd.GetStackOutputs(ctx)
	if err != nil {
		pd.logEntry("WARN", "Failed to retrieve outputs", pd.stackName, "outputs", "failed", err.Error())
		outputs = make(map[string]interface{}) // Continue with empty outputs
	}

	// Convert resources
	resources := pd.convertPulumiResources(result.Resources)

	deployResult := &DeploymentResult{
		Success:   true,
		StackName: pd.stackName,
		Outputs:   outputs,
		Resources: resources,
		Duration:  time.Since(startTime),
		UpdateID:  fmt.Sprintf("deploy-%d", time.Now().Unix()),
	}

	pd.logEntry("INFO", "Pulumi deployment completed successfully", pd.stackName, "deploy", "completed", "")
	return deployResult, nil
}

// Destroy destroys the infrastructure using Pulumi CLI
func (pd *PulumiDeployer) Destroy(ctx context.Context) (*DeploymentResult, error) {
	startTime := time.Now()
	pd.logEntry("INFO", "Starting Pulumi stack destruction", pd.stackName, "destroy", "started", "")

	// Execute destruction with JSON output
	output, err := pd.runPulumiCommandWithOutput(ctx, "destroy", "--yes", "--json")
	if err != nil {
		errorMsg := fmt.Sprintf("destruction failed: %v", err)
		pd.logEntry("ERROR", errorMsg, pd.stackName, "destroy", "failed", errorMsg)

		return &DeploymentResult{
			Success:   false,
			StackName: pd.stackName,
			Duration:  time.Since(startTime),
			Error:     errorMsg,
		}, err
	}

	// Parse Pulumi output
	result, err := pd.parsePulumiOutput(output)
	if err != nil {
		pd.logEntry("WARN", "Failed to parse Pulumi output", pd.stackName, "parse", "failed", err.Error())
		result = &PulumiOperationResult{} // Continue with empty result
	}

	// Convert resources
	resources := pd.convertPulumiResources(result.Resources)

	destroyResult := &DeploymentResult{
		Success:   true,
		StackName: pd.stackName,
		Resources: resources,
		Duration:  time.Since(startTime),
		UpdateID:  fmt.Sprintf("destroy-%d", time.Now().Unix()),
	}

	pd.logEntry("INFO", "Pulumi stack destruction completed successfully", pd.stackName, "destroy", "completed", "")
	return destroyResult, nil
}

// GetStackOutputs retrieves the current stack outputs using Pulumi CLI
func (pd *PulumiDeployer) GetStackOutputs(ctx context.Context) (map[string]interface{}, error) {
	output, err := pd.runPulumiCommandWithOutput(ctx, "stack", "output", "--json")
	if err != nil {
		return nil, fmt.Errorf("failed to get stack outputs: %w", err)
	}

	var outputs map[string]interface{}
	if err := json.Unmarshal([]byte(output), &outputs); err != nil {
		return nil, fmt.Errorf("failed to parse stack outputs: %w", err)
	}

	return outputs, nil
}

// Helper methods

// checkPulumiCLI verifies that Pulumi CLI is available and functional
func (pd *PulumiDeployer) checkPulumiCLI() error {
	cmd := exec.Command(pd.pulumiCmd, "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pulumi CLI not available or not working: %w", err)
	}
	return nil
}

// runPulumiCommand executes a Pulumi CLI command
func (pd *PulumiDeployer) runPulumiCommand(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, pd.pulumiCmd, args...)
	cmd.Dir = pd.workspaceDir

	// Set environment variables
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PULUMI_STACK=%s", pd.stackName),
		fmt.Sprintf("AWS_REGION=%s", pd.region),
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pulumi command failed: %s, output: %s", err, string(output))
	}

	return nil
}

// runPulumiCommandWithOutput executes a Pulumi CLI command and returns output
func (pd *PulumiDeployer) runPulumiCommandWithOutput(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, pd.pulumiCmd, args...)
	cmd.Dir = pd.workspaceDir

	// Set environment variables
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PULUMI_STACK=%s", pd.stackName),
		fmt.Sprintf("AWS_REGION=%s", pd.region),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("pulumi command failed: %s, output: %s", err, string(output))
	}

	return string(output), nil
}

// parsePulumiOutput parses JSON output from Pulumi CLI operations
func (pd *PulumiDeployer) parsePulumiOutput(output string) (*PulumiOperationResult, error) {
	// Pulumi JSON output can be multi-line with different event types
	// We'll look for the final result
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var event map[string]interface{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue // Skip non-JSON lines
		}

		// Look for summary events
		if kind, ok := event["kind"].(string); ok && kind == "summary" {
			var result PulumiOperationResult
			if err := json.Unmarshal([]byte(line), &result); err == nil {
				return &result, nil
			}
		}
	}

	// If no summary found, return empty result
	return &PulumiOperationResult{}, nil
}

// convertPulumiResources converts Pulumi resources to ResourceSummary format
func (pd *PulumiDeployer) convertPulumiResources(resources []PulumiResource) []ResourceSummary {
	var summaries []ResourceSummary

	for _, resource := range resources {
		// Extract resource name from URN
		urnParts := strings.Split(resource.URN, "::")
		name := "unknown"
		if len(urnParts) > 3 {
			name = urnParts[3]
		}

		summaries = append(summaries, ResourceSummary{
			Type:   resource.Type,
			Name:   name,
			URN:    resource.URN,
			Status: "deployed", // Simplified - would be extracted from operation context
		})
	}

	return summaries
}

// getWorkspaceDir returns the workspace directory for Pulumi operations
func getWorkspaceDir() string {
	if wd := os.Getenv("PULUMI_WORKSPACE_DIR"); wd != "" {
		return wd
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "/tmp"
}

// GetDeploymentLogs returns the deployment logs
func (pd *PulumiDeployer) GetDeploymentLogs() []DeploymentLogEntry {
	logs := make([]DeploymentLogEntry, len(pd.deploymentLog))
	copy(logs, pd.deploymentLog)
	return logs
}

// ExportLogs exports deployment logs in JSON format
func (pd *PulumiDeployer) ExportLogs() ([]byte, error) {
	return json.MarshalIndent(pd.deploymentLog, "", "  ")
}

// logEntry adds an entry to the deployment log
func (pd *PulumiDeployer) logEntry(level, message, resource, operation, status, errorMsg string) {
	entry := DeploymentLogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Resource:  resource,
		Operation: operation,
		Status:    status,
		Error:     errorMsg,
	}

	pd.deploymentLog = append(pd.deploymentLog, entry)
}

// PulumiStackManager manages multiple Pulumi stacks
type PulumiStackManager struct {
	stacks map[string]*PulumiDeployer
}

// NewPulumiStackManager creates a new stack manager
func NewPulumiStackManager() *PulumiStackManager {
	return &PulumiStackManager{
		stacks: make(map[string]*PulumiDeployer),
	}
}

// AddStack adds a stack to the manager
func (psm *PulumiStackManager) AddStack(name string, deployer *PulumiDeployer) {
	psm.stacks[name] = deployer
}

// GetStack retrieves a stack from the manager
func (psm *PulumiStackManager) GetStack(name string) (*PulumiDeployer, bool) {
	stack, exists := psm.stacks[name]
	return stack, exists
}

// ListStacks returns all stack names
func (psm *PulumiStackManager) ListStacks() []string {
	names := make([]string, 0, len(psm.stacks))
	for name := range psm.stacks {
		names = append(names, name)
	}
	return names
}

// PulumiConfigManager manages Pulumi configuration
type PulumiConfigManager struct {
	configs map[string]PulumiStackConfig
}

// NewPulumiConfigManager creates a new config manager
func NewPulumiConfigManager() *PulumiConfigManager {
	return &PulumiConfigManager{
		configs: make(map[string]PulumiStackConfig),
	}
}

// SetConfig sets configuration for a stack
func (pcm *PulumiConfigManager) SetConfig(stackName string, config PulumiStackConfig) {
	pcm.configs[stackName] = config
}

// GetConfig retrieves configuration for a stack
func (pcm *PulumiConfigManager) GetConfig(stackName string) (PulumiStackConfig, bool) {
	config, exists := pcm.configs[stackName]
	return config, exists
}
