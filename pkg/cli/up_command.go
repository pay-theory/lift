package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/pay-theory/lift/internal/domains"
	"github.com/pay-theory/lift/internal/liftconfig"
	"github.com/pay-theory/lift/internal/liftstate"
)

// UpCommand implements the "lift up" command for building and deploying
type UpCommand struct {
	// For testing: allows overriding how commands are created
	cmdFactory func(ctx context.Context, name string, arg ...string) *exec.Cmd
}

func (c *UpCommand) Name() string        { return "up" }
func (c *UpCommand) Description() string { return "Build and deploy the Lift project for a stage" }
func (c *UpCommand) Usage() string       { return "lift up --stage <dev|staging|live>" }

// Execute runs the up command. It:
// 1. Finds the project root by walking up for lift.yaml
// 2. Parses the configuration
// 3. Resolves domains for the requested stage
// 4. Checks domain lock (prevents domain drift)
// 5. Builds all functions
// 6. Deploys stacks in order using CDK
// 7. Writes the state file on success
func (c *UpCommand) Execute(ctx context.Context, args []string) error {
	// Parse --stage flag
	stage := "dev" // default
	for i, arg := range args {
		if arg == "--stage" && i+1 < len(args) {
			stage = args[i+1]
		}
		if strings.HasPrefix(arg, "--stage=") {
			stage = strings.TrimPrefix(arg, "--stage=")
		}
	}

	// Validate stage
	if err := domains.ValidateStage(stage); err != nil {
		return err
	}

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Find project root
	root, err := liftconfig.FindProjectRoot(cwd)
	if err != nil {
		return err
	}

	// Load and validate configuration
	cfg, err := liftconfig.LoadConfig(root)
	if err != nil {
		return err
	}

	fmt.Printf("🚀 Deploying %s to stage: %s\n", cfg.App.Name, stage)
	fmt.Printf("📁 Project root: %s\n\n", root)

	// Resolve domains
	resolved, err := domains.Resolve(cfg, stage)
	if err != nil {
		return err
	}

	// Check domain lock
	if resolved != nil {
		existingState, err := liftstate.Load(root, stage)
		if err != nil {
			return err
		}

		services := make(map[string]string)
		for name, domain := range resolved.Services {
			services[name] = domain
		}

		if err := liftstate.CheckDomainLock(existingState, stage, resolved.BaseDomain, resolved.StageRootDomain, services); err != nil {
			return err
		}

		if existingState != nil {
			fmt.Printf("✅ Domain configuration matches existing deployment\n\n")
		}
	}

	// Build before deploy
	fmt.Printf("📦 Building functions...\n")
	buildCmd := &BuildCommand{cmdFactory: c.cmdFactory}
	if err := buildCmd.Execute(ctx, nil); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}
	fmt.Printf("\n")

	// Deploy stacks
	if err := c.deployStacks(ctx, root, cfg, stage, resolved); err != nil {
		return err
	}

	// Write state on success
	if resolved != nil {
		state := &liftstate.StageState{
			Stage:           stage,
			BaseDomain:      resolved.BaseDomain,
			StageRootDomain: resolved.StageRootDomain,
			Services:        make(map[string]liftstate.ServiceState),
		}
		for name, domain := range resolved.Services {
			subdomain := ""
			if cfg.Services != nil && cfg.Services[name] != nil {
				subdomain = cfg.Services[name].Subdomain
			}
			state.Services[name] = liftstate.ServiceState{
				Subdomain: subdomain,
				Domain:    domain,
			}
		}
		if err := liftstate.Save(root, state); err != nil {
			return fmt.Errorf("deployment succeeded but failed to save state: %w", err)
		}
		fmt.Printf("\n🔒 State saved to %s\n", liftstate.StatePath(root, stage))
	}

	fmt.Printf("\n✅ Deployment complete!\n")
	return nil
}

func (c *UpCommand) deployStacks(ctx context.Context, root string, cfg *liftconfig.Config, stage string, resolved *domains.ResolvedDomains) error {
	if cfg.CDK == nil || len(cfg.CDK.DeployOrder) == 0 {
		fmt.Printf("⚠️  No CDK stacks configured in deploy_order, skipping deployment\n")
		return nil
	}

	cdkPath := cfg.CDK.Path
	if cdkPath == "" {
		cdkPath = "./cdk"
	}
	cdkDir := filepath.Join(root, cdkPath)

	fmt.Printf("🏗️  Deploying CDK stacks...\n")

	for _, stackKey := range cfg.CDK.DeployOrder {
		stackCfg, ok := cfg.CDK.Stacks[stackKey]
		if !ok {
			return fmt.Errorf("stack %q in deploy_order not found in cdk.stacks", stackKey)
		}

		stackName, err := c.renderStackName(stackCfg.NameTemplate, cfg.App.Name, stage)
		if err != nil {
			return fmt.Errorf("failed to render stack name for %q: %w", stackKey, err)
		}

		fmt.Printf("  📤 Deploying stack: %s\n", stackName)

		cdkArgs := c.buildCDKArgs("deploy", stackName, cfg, stage, resolved)
		if err := c.runCDK(ctx, cdkDir, cdkArgs); err != nil {
			return fmt.Errorf("failed to deploy stack %q: %w", stackName, err)
		}
		fmt.Printf("  ✅ Stack %s deployed\n", stackName)
	}

	return nil
}

func (c *UpCommand) buildCDKArgs(command, stackName string, cfg *liftconfig.Config, stage string, resolved *domains.ResolvedDomains) []string {
	args := []string{command, stackName}

	if command == "deploy" {
		args = append(args, "--require-approval", "never")
	}
	if command == "destroy" {
		args = append(args, "--force")
	}

	// Add context flags
	args = append(args, "--context", fmt.Sprintf("stage=%s", stage))
	args = append(args, "--context", fmt.Sprintf("appName=%s", cfg.App.Name))

	if resolved != nil {
		args = append(args, "--context", fmt.Sprintf("baseDomain=%s", resolved.BaseDomain))
		args = append(args, "--context", fmt.Sprintf("stageRootDomain=%s", resolved.StageRootDomain))
		for name, domain := range resolved.Services {
			args = append(args, "--context", fmt.Sprintf("serviceDomain.%s=%s", name, domain))
		}
	}

	return args
}

func (c *UpCommand) renderStackName(tmpl, appName, stage string) (string, error) {
	if tmpl == "" {
		return "", fmt.Errorf("name_template is required")
	}

	t, err := template.New("stackName").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("invalid template: %w", err)
	}

	var buf bytes.Buffer
	data := map[string]string{
		"AppName": appName,
		"Stage":   stage,
	}
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (c *UpCommand) runCDK(ctx context.Context, cdkDir string, args []string) error {
	var cmd *exec.Cmd
	if c.cmdFactory != nil {
		cmd = c.cmdFactory(ctx, "cdk", args...)
	} else {
		cmd = exec.CommandContext(ctx, "cdk", args...)
	}

	cmd.Dir = cdkDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
