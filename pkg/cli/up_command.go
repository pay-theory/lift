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
	// For testing: allows overriding binary lookup
	lookPath LookPathFunc
}

func (c *UpCommand) Name() string        { return "up" }
func (c *UpCommand) Description() string { return "Build and deploy the Lift project for a stage" }
func (c *UpCommand) Usage() string {
	return `lift up --stage <dev|staging|live> [--partner <name>] [--target-mode <mode>]

Flags:
  --stage <stage>         Deployment stage: dev, staging, or live (default: dev)
  --partner <name>        Partner name for PT-mode deployments (optional)
  --target-mode <mode>    Target mode for PT-mode deployments (default: standard)

Examples:
  lift up --stage dev
  lift up --stage staging --partner mypartner
  lift up --stage live --partner mypartner --target-mode custom`
}

// Execute runs the up command. It:
// 1. Finds the project root by walking up for lift.yaml
// 2. Parses the configuration
// 3. Resolves domains for the requested stage
// 4. Checks domain lock (prevents domain drift)
// 5. Builds all functions
// 6. Deploys stacks in order using CDK
// 7. Writes the state file on success
func (c *UpCommand) Execute(ctx context.Context, args []string) error {
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

	// Check for required binaries before proceeding
	if err := CheckGo(c.lookPath); err != nil {
		return err
	}
	if err := CheckCDK(c.lookPath); err != nil {
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
	if err := c.deployStacks(ctx, root, cfg, stage, partner, targetMode, resolved); err != nil {
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

func (c *UpCommand) deployStacks(ctx context.Context, root string, cfg *liftconfig.Config, stage, partner, targetMode string, resolved *domains.ResolvedDomains) error {
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

		stackName, err := c.renderStackName(stackCfg.NameTemplate, cfg.App.Name, stage, partner, targetMode)
		if err != nil {
			return fmt.Errorf("failed to render stack name for %q: %w", stackKey, err)
		}

		fmt.Printf("  📤 Deploying stack: %s\n", stackName)

		cdkArgs := c.buildCDKArgs("deploy", stackName, cfg, stage, partner, targetMode, resolved)
		if err := c.runCDK(ctx, cdkDir, cdkArgs); err != nil {
			return fmt.Errorf("failed to deploy stack %q: %w", stackName, err)
		}
		fmt.Printf("  ✅ Stack %s deployed\n", stackName)
	}

	return nil
}

func (c *UpCommand) buildCDKArgs(command, stackName string, cfg *liftconfig.Config, stage, partner, targetMode string, resolved *domains.ResolvedDomains) []string {
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

// renderStackName renders a stack name template with the provided values.
// Supports: {{.AppName}}, {{.Stage}}, {{.Partner}}, {{.TargetMode}}
func (c *UpCommand) renderStackName(tmpl, appName, stage, partner, targetMode string) (string, error) {
	if tmpl == "" {
		return "", fmt.Errorf("name_template is required")
	}

	t, err := template.New("stackName").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("invalid template: %w", err)
	}

	var buf bytes.Buffer
	data := map[string]string{
		"AppName":    appName,
		"Stage":      stage,
		"Partner":    partner,
		"TargetMode": targetMode,
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

func cdkTemplatesUsePartner(cfg *liftconfig.Config) bool {
	return cdkTemplatesContain(cfg, ".Partner")
}

func cdkTemplatesUseTargetMode(cfg *liftconfig.Config) bool {
	return cdkTemplatesContain(cfg, ".TargetMode")
}

func cdkTemplatesContain(cfg *liftconfig.Config, substr string) bool {
	if cfg == nil || cfg.CDK == nil || cfg.CDK.Stacks == nil {
		return false
	}
	for _, stack := range cfg.CDK.Stacks {
		if stack == nil {
			continue
		}
		if strings.Contains(stack.NameTemplate, substr) {
			return true
		}
	}
	return false
}
