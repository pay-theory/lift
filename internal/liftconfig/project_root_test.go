package liftconfig

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindProjectRoot_AtRoot(t *testing.T) {
	root := t.TempDir()

	// Create lift.yaml at root
	err := os.WriteFile(filepath.Join(root, ConfigFileName), []byte("version: 1\n"), 0600)
	require.NoError(t, err)

	// Start from root
	foundRoot, err := FindProjectRoot(root)
	require.NoError(t, err)
	assert.Equal(t, root, foundRoot)
}

func TestFindProjectRoot_FromNestedSubdir(t *testing.T) {
	root := t.TempDir()

	// Create lift.yaml at root
	err := os.WriteFile(filepath.Join(root, ConfigFileName), []byte("version: 1\n"), 0600)
	require.NoError(t, err)

	// Create nested subdirectories
	nestedDir := filepath.Join(root, "src", "handlers", "api")
	err = os.MkdirAll(nestedDir, 0755)
	require.NoError(t, err)

	// Start from deeply nested directory
	foundRoot, err := FindProjectRoot(nestedDir)
	require.NoError(t, err)
	assert.Equal(t, root, foundRoot)
}

func TestFindProjectRoot_FromDirectChild(t *testing.T) {
	root := t.TempDir()

	// Create lift.yaml at root
	err := os.WriteFile(filepath.Join(root, ConfigFileName), []byte("version: 1\n"), 0600)
	require.NoError(t, err)

	// Create a direct child directory
	childDir := filepath.Join(root, "cmd")
	err = os.MkdirAll(childDir, 0755)
	require.NoError(t, err)

	// Start from child directory
	foundRoot, err := FindProjectRoot(childDir)
	require.NoError(t, err)
	assert.Equal(t, root, foundRoot)
}

func TestFindProjectRoot_NotFound(t *testing.T) {
	root := t.TempDir() // Empty directory with no lift.yaml

	// Create some subdirectories (without lift.yaml)
	nestedDir := filepath.Join(root, "some", "nested", "dir")
	err := os.MkdirAll(nestedDir, 0755)
	require.NoError(t, err)

	foundRoot, err := FindProjectRoot(nestedDir)
	assert.Empty(t, foundRoot)
	require.Error(t, err)

	var notFoundErr *ProjectNotFoundError
	require.True(t, errors.As(err, &notFoundErr), "expected ProjectNotFoundError")
	assert.Contains(t, notFoundErr.Error(), ConfigFileName)
	assert.Contains(t, notFoundErr.Error(), "lift new")
}

func TestFindProjectRoot_RelativePath(t *testing.T) {
	root := t.TempDir()

	// Create lift.yaml at root
	err := os.WriteFile(filepath.Join(root, ConfigFileName), []byte("version: 1\n"), 0600)
	require.NoError(t, err)

	// Create subdirectory
	subDir := filepath.Join(root, "subdir")
	err = os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	// Change to subdir and use relative path
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalWd) }()

	err = os.Chdir(subDir)
	require.NoError(t, err)

	// Find from current directory (relative ".")
	foundRoot, err := FindProjectRoot(".")
	require.NoError(t, err)
	assert.Equal(t, root, foundRoot)
}

func TestFindProjectRoot_LiftYamlInParent(t *testing.T) {
	root := t.TempDir()

	// Create lift.yaml at root
	err := os.WriteFile(filepath.Join(root, ConfigFileName), []byte("version: 1\n"), 0600)
	require.NoError(t, err)

	// Create multiple levels of subdirectories
	deepDir := filepath.Join(root, "a", "b", "c", "d", "e")
	err = os.MkdirAll(deepDir, 0755)
	require.NoError(t, err)

	// Start from deepest directory
	foundRoot, err := FindProjectRoot(deepDir)
	require.NoError(t, err)
	assert.Equal(t, root, foundRoot)
}

func TestProjectNotFoundError_Message(t *testing.T) {
	err := &ProjectNotFoundError{StartDir: "/some/path"}

	msg := err.Error()
	assert.Contains(t, msg, ConfigFileName)
	assert.Contains(t, msg, "/some/path")
	assert.Contains(t, msg, "lift new")
}
