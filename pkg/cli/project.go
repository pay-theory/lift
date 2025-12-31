package cli

import (
	"fmt"
	"os"

	"github.com/pay-theory/lift/internal/liftconfig"
)

func loadProjectConfigFromCwd() (root string, cfg *liftconfig.Config, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	root, err = liftconfig.FindProjectRoot(cwd)
	if err != nil {
		return "", nil, err
	}

	cfg, err = liftconfig.LoadConfig(root)
	if err != nil {
		return "", nil, err
	}

	return root, cfg, nil
}
