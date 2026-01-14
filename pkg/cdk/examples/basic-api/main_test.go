package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExampleMain(t *testing.T) {
	tmp := t.TempDir()

	// This example references a relative asset path ("../../lambda").
	// Create that path in a temp workspace and run the example from the expected CWD.
	workDir := filepath.Join(tmp, "pkg", "cdk", "examples", "basic-api")
	lambdaDir := filepath.Join(tmp, "pkg", "cdk", "lambda")

	require.NoError(t, os.MkdirAll(workDir, 0o755))
	require.NoError(t, os.MkdirAll(lambdaDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(lambdaDir, "bootstrap"), []byte("x"), 0o644))

	origWD, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	require.NoError(t, os.Chdir(workDir))

	main()
}
