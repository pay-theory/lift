package cli

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShouldRetryWithMod(t *testing.T) {
	require.True(t, shouldRetryWithMod("missing go.sum entry for module github.com/example/foo"))
	require.True(t, shouldRetryWithMod("updates to go.mod needed; to update it:\n\tgo mod tidy"))
	require.False(t, shouldRetryWithMod("some other error"))
}

func TestResolveOutputPath(t *testing.T) {
	root := t.TempDir()

	relative := filepath.Join("dist", "api", "bootstrap")
	require.Equal(t, filepath.Join(root, relative), resolveOutputPath(root, relative))

	abs := filepath.Join(root, "dist", "api", "bootstrap")
	require.Equal(t, abs, resolveOutputPath(root, abs))
}

func TestFormatBuildFailure(t *testing.T) {
	err := formatBuildFailure("api", []string{"build", "-o", "x", "./cmd/api"}, "", errors.New("boom"))
	require.Error(t, err)
	require.NotContains(t, err.Error(), "stderr:")

	err = formatBuildFailure("api", []string{"build", "-o", "x", "./cmd/api"}, "some stderr\n", errors.New("boom"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "stderr: some stderr")
}
