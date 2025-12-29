package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pay-theory/lift/internal/domains"
	"github.com/pay-theory/lift/internal/liftconfig"
	"github.com/pay-theory/lift/internal/liftstate"
)

// DownCommand implements the "lift down" command for destroying deployments
type DownCommand struct {
	// For testing: allows overriding how commands are created
	cmdFactory func(ctx context.Context, name string, arg ...string) *exec.Cmd
}

func (c *DownCommand) Name() string        { return "down" }
func (c *DownCommand) Description() string { return "Destroy the Lift project deployment for a stage" }
func (c *DownCommand) Usage() string {
	return `lift down --stage <dev|staging|live> [--partner <name>] [--target-mode <mode>]

Flags:
  --stage <stage>         Deployment stage: dev, staging, or live (default: dev)
  --partner <name>        Partner name for PT-mode deployments (optional)
  --target-mode <mode>    Target mode for PT-mode deployments (default: standard)

Examples:
  lift down --stage dev
  lift down --stage staging --partner mypartner`
}

// Execute runs the down command. It:
// 1. Finds the project root by walking up for lift.yaml
// 2. Parses the configuration
// 3. Destroys stacks in reverse deploy order using CDK
// 4. Removes the state file on success (clears domain lock)
func (c *DownCommand) Execute(ctx context.Context, args []string) error {
	// Parse flags
	stage := "dev" // default
	partner := ""
	targetMode := ""

	for i, arg := range args {
		switch {
		case arg == "--stage" && i+1 < len(args):
			stage = args[i+1]
		case strings.HasPrefix(arg, "--stage="):
			stage = strings.TrimPrefix(arg, "--stage=")
		case arg == "--partner" && i+1 < len(args):
			partner = args[i+1]
		case strings.HasPrefix(arg, "--partner="):
			partner = strings.TrimPrefix(arg, "--partner=")
		case arg == "--target-mode" && i+1 < len(args):
			targetMode = args[i+1]
		case strings.HasPrefix(arg, "--target-mode="):
			targetMode = strings.TrimPrefix(arg, "--target-mode=")
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

	requiresPartner := cdkTemplatesUsePartner(cfg)
	requiresTargetMode := cdkTemplatesUseTargetMode(cfg)

	// Enforce partner requirement when stack name templates reference {{.Partner}}.
	if requiresPartner && partner == "" {
		return fmt.Errorf("--partner is required for this project (cdk stack name_template references {{.Partner}})")
	}

	// Default targetMode to "standard" when relevant.
	if targetMode == "" && (partner != "" || requiresTargetMode) {
		targetMode = "standard"
	}

	fmt.Printf("🗑️  Destroying %s stage: %s\n", cfg.App.Name, stage)
	fmt.Printf("📁 Project root: %s\n\n", root)

	// Resolve domains for context
	resolved, err := domains.Resolve(cfg, stage)
	if err != nil {
		return err
	}

	// Destroy stacks in reverse order
	if err := c.destroyStacks(ctx, root, cfg, stage, partner, targetMode, resolved); err != nil {
		return err
	}

	// Remove state file (clears domain lock)
	if err := liftstate.Remove(root, stage); err != nil {
		return fmt.Errorf("stacks destroyed but failed to remove state file: %w", err)
	}

	fmt.Printf("\n🔓 Domain lock cleared for stage: %s\n", stage)
	fmt.Printf("\n✅ Teardown complete!\n")
	return nil
}

func (c *DownCommand) destroyStacks(ctx context.Context, root string, cfg *liftconfig.Config, stage, partner, targetMode string, resolved *domains.ResolvedDomains) error {
	if cfg.CDK == nil || len(cfg.CDK.DeployOrder) == 0 {
		fmt.Printf("⚠️  No CDK stacks configured in deploy_order, nothing to destroy\n")
		return nil
	}

	cdkPath := cfg.CDK.Path
	if cdkPath == "" {
		cdkPath = "./cdk"
	}
	cdkDir := filepath.Join(root, cdkPath)

	fmt.Printf("🏗️  Destroying CDK stacks in reverse order...\n")

	// Reverse the deploy order
	order := cfg.CDK.DeployOrder
	for i := len(order) - 1; i >= 0; i-- {
		stackKey := order[i]
		stackCfg, ok := cfg.CDK.Stacks[stackKey]
		if !ok {
			return fmt.Errorf("stack %q in deploy_order not found in cdk.stacks", stackKey)
		}

		stackName, err := c.renderStackName(stackCfg.NameTemplate, cfg.App.Name, stage, partner, targetMode)
		if err != nil {
			return fmt.Errorf("failed to render stack name for %q: %w", stackKey, err)
		}

		fmt.Printf("  🗑️  Destroying stack: %s\n", stackName)

		cdkArgs := c.buildCDKArgs(stackName, cfg, stage, partner, targetMode, resolved)
		if err := c.runCDK(ctx, cdkDir, cdkArgs); err != nil {
			return fmt.Errorf("failed to destroy stack %q: %w", stackName, err)
		}
		fmt.Printf("  ✅ Stack %s destroyed\n", stackName)
	}

	return nil
}

func (c *DownCommand) buildCDKArgs(stackName string, cfg *liftconfig.Config, stage, partner, targetMode string, resolved *domains.ResolvedDomains) []string {
	args := []string{"destroy", stackName, "--force"}

	// Add context flags (same as deploy for consistency)
	args = append(args, "--context", fmt.Sprintf("stage=%s", stage))
	args = append(args, "--context", fmt.Sprintf("appName=%s", cfg.App.Name))

	// Default targetMode to "standard" if partner is set but targetMode is not.
	if partner != "" && targetMode == "" {
		targetMode = "standard"
	}

	// Add optional context flags
	if partner != "" {
		args = append(args, "--context", fmt.Sprintf("partner=%s", partner))
	}
	if targetMode != "" {
		args = append(args, "--context", fmt.Sprintf("targetMode=%s", targetMode))
	}

	if resolved != nil {
		args = append(args, "--context", fmt.Sprintf("baseDomain=%s", resolved.BaseDomain))
		args = append(args, "--context", fmt.Sprintf("stageRootDomain=%s", resolved.StageRootDomain))
		for name, domain := range resolved.Services {
			args = append(args, "--context", fmt.Sprintf("serviceDomain.%s=%s", name, domain))
		}
	}

	return args
}

func (c *DownCommand) renderStackName(tmpl, appName, stage, partner, targetMode string) (string, error) {
	// Reuse the same logic as UpCommand
	up := &UpCommand{}
	return up.renderStackName(tmpl, appName, stage, partner, targetMode)
}

func (c *DownCommand) runCDK(ctx context.Context, cdkDir string, args []string) error {
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
