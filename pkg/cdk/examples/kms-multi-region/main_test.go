package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExampleMain(t *testing.T) {
	tmp := t.TempDir()

	origWD, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	require.NoError(t, os.Chdir(tmp))

	t.Setenv("CDK_DEFAULT_ACCOUNT", "123456789012")

	main()
}
