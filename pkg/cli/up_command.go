package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/pay-theory/lift/internal/liftconfig"
)

// UpCommand implements the "lift up" command for building and deploying
type UpCommand struct{}

func (c *UpCommand) Name() string        { return "up" }
func (c *UpCommand) Description() string { return "Build and deploy the Lift project for a stage" }
func (c *UpCommand) Usage() string       { return "lift up --stage <dev|staging|live>" }

// Execute runs the up command. It:
// 1. Finds the project root by walking up for lift.yaml
// 2. Parses the configuration
// 3. (Future milestones) Builds and deploys
func (c *UpCommand) Execute(_ context.Context, _ []string) error {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Find project root
	root, err := liftconfig.FindProjectRoot(cwd)
	if err != nil {
		// ProjectNotFoundError already has an actionable message
		return err
	}

	// Load and validate configuration
	cfg, err := liftconfig.LoadConfig(root)
	if err != nil {
		return err
	}

	// For Milestone 1, just acknowledge success
	fmt.Printf("✅ Found Lift project: %s\n", cfg.App.Name)
	fmt.Printf("📁 Project root: %s\n", root)
	fmt.Printf("\n🚧 Build and deploy not yet implemented (Milestone 3+4)\n")

	return nil
}
