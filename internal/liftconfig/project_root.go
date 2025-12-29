package liftconfig

import (
	"fmt"
	"os"
	"path/filepath"
)

// ProjectNotFoundError is returned when no lift.yaml is found walking up the directory tree
type ProjectNotFoundError struct {
	StartDir string
}

func (e *ProjectNotFoundError) Error() string {
	return fmt.Sprintf(
		"no Lift project found (could not find %s searching from %s)\n\n"+
			"To create a new Lift project, run:\n"+
			"  lift new <project-name>",
		ConfigFileName, e.StartDir,
	)
}

// FindProjectRoot walks up the directory tree from startDir looking for lift.yaml.
// Returns the directory containing lift.yaml, or a ProjectNotFoundError if not found.
func FindProjectRoot(startDir string) (string, error) {
	// Resolve to absolute path
	absDir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path %s: %w", startDir, err)
	}

	current := absDir
	for {
		configPath := filepath.Join(current, ConfigFileName)
		if _, err := os.Stat(configPath); err == nil {
			return current, nil
		}

		// Get parent directory
		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root
			return "", &ProjectNotFoundError{StartDir: absDir}
		}
		current = parent
	}
}
