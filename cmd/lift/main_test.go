package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRun_HelpReturnsZero(t *testing.T) {
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })

	os.Args = []string{"lift"}
	require.Equal(t, 0, run())
}

func TestRun_UnknownCommandReturnsOne(t *testing.T) {
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })

	os.Args = []string{"lift", "definitely-not-a-command"}
	require.Equal(t, 1, run())
}
