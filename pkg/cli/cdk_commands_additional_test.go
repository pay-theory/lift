package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCDKInitCommand_Execute_StacksAndErrors(t *testing.T) {
	t.Run("errors outside lift project", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(origDir) }()
		require.NoError(t, os.Chdir(tmpDir))

		err = (&CDKInitCommand{}).Execute(context.Background(), nil)
		require.Error(t, err)
	})

	type stackTest struct {
		stackType     string
		wantSnippet   string
		wantGitIgnore bool
	}

	tests := []stackTest{
		{stackType: "", wantSnippet: "patterns.NewLiftApp", wantGitIgnore: true},
		{stackType: "microservice", wantSnippet: "stacks.NewMicroserviceStack", wantGitIgnore: false},
		{stackType: "saas", wantSnippet: "stacks.NewMultiTenantSaaSStack", wantGitIgnore: false},
		{stackType: "event-driven", wantSnippet: "stacks.NewEventDrivenStack", wantGitIgnore: false},
	}

	for _, tc := range tests {
		t.Run("stack="+tc.stackType, func(t *testing.T) {
			tmpDir := t.TempDir()
			origDir, err := os.Getwd()
			require.NoError(t, err)
			defer func() { _ = os.Chdir(origDir) }()
			require.NoError(t, os.Chdir(tmpDir))

			require.NoError(t, os.WriteFile("go.mod", []byte("module example.com/test\n\ngo 1.21\n"), 0600))

			args := []string(nil)
			if tc.stackType != "" {
				args = []string{tc.stackType}
			}

			require.NoError(t, (&CDKInitCommand{}).Execute(context.Background(), args))

			mainPath := filepath.Join(tmpDir, "cdk", "main.go")
			assertFileExists(t, mainPath)
			content, err := os.ReadFile(mainPath)
			require.NoError(t, err)
			require.Contains(t, string(content), tc.wantSnippet)

			assertFileExists(t, filepath.Join(tmpDir, "cdk", "cdk.json"))

			gitignorePath := filepath.Join(tmpDir, "cdk", ".gitignore")
			_, err = os.Stat(gitignorePath)
			if tc.wantGitIgnore {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}

	t.Run("unknown stack type errors", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(origDir) }()
		require.NoError(t, os.Chdir(tmpDir))
		require.NoError(t, os.WriteFile("go.mod", []byte("module example.com/test\n"), 0600))

		err = (&CDKInitCommand{}).Execute(context.Background(), []string{"nope"})
		require.Error(t, err)
	})
}

func TestCDKCommands_DeploySynthDiffDestroy_WithFakeBinaries(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	require.NoError(t, os.WriteFile("go.mod", []byte("module example.com/test\n\ngo 1.21\n"), 0600))
	require.NoError(t, os.MkdirAll("cmd", 0750))
	require.NoError(t, os.WriteFile(filepath.Join("cmd", "main.go"), []byte("package main\n\nfunc main() {}\n"), 0600))
	require.NoError(t, os.MkdirAll("cdk", 0750))

	recordFile := filepath.Join(tmpDir, "record.txt")
	binDir := filepath.Join(tmpDir, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0750))

	writeExecutable(t, filepath.Join(binDir, "go"), `#!/bin/sh
set -eu
out=""
prev=""
for arg in "$@"; do
  if [ "$prev" = "-o" ]; then out="$arg"; prev=""; continue; fi
  if [ "$arg" = "-o" ]; then prev="-o"; continue; fi
done
if [ -n "$out" ]; then
  mkdir -p "$(dirname "$out")"
  echo "fake-binary" > "$out"
fi
exit 0
`)

	writeExecutable(t, filepath.Join(binDir, "cdk"), `#!/bin/sh
set -eu
if [ -n "${CDK_RECORD_FILE:-}" ]; then
  echo "$@" >> "$CDK_RECORD_FILE"
fi

if [ "$1" = "diff" ]; then
  if [ -n "${CDK_DIFF_EXIT:-}" ]; then
    exit "$CDK_DIFF_EXIT"
  fi
  exit 1
fi

exit 0
`)

	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
	t.Setenv("CDK_RECORD_FILE", recordFile)

	t.Run("deploy succeeds and invokes cdk", func(t *testing.T) {
		require.NoError(t, (&CDKDeployCommand{}).Execute(context.Background(), []string{"MyStack"}))
		assertFileExists(t, filepath.Join(tmpDir, "dist", "bootstrap"))

		content, err := os.ReadFile(recordFile)
		require.NoError(t, err)
		require.Contains(t, string(content), "deploy --require-approval never MyStack")
	})

	t.Run("synth succeeds", func(t *testing.T) {
		require.NoError(t, os.Chdir(tmpDir))
		require.NoError(t, (&CDKSynthCommand{}).Execute(context.Background(), []string{"MyStack"}))
	})

	t.Run("diff exit 1 is treated as success", func(t *testing.T) {
		require.NoError(t, os.Chdir(tmpDir))
		require.NoError(t, (&CDKDiffCommand{}).Execute(context.Background(), []string{"MyStack"}))
	})

	t.Run("diff non-1 exit is treated as error", func(t *testing.T) {
		require.NoError(t, os.Chdir(tmpDir))
		t.Setenv("CDK_DIFF_EXIT", "2")
		require.Error(t, (&CDKDiffCommand{}).Execute(context.Background(), nil))
	})

	t.Run("destroy canceled on EOF", func(t *testing.T) {
		require.NoError(t, os.Chdir(tmpDir))
		restore := replaceStdin(t, "")
		defer restore()

		require.NoError(t, (&CDKDestroyCommand{}).Execute(context.Background(), nil))
	})

	t.Run("destroy canceled on non-yes input", func(t *testing.T) {
		require.NoError(t, os.Chdir(tmpDir))
		restore := replaceStdin(t, "n\n")
		defer restore()

		require.NoError(t, (&CDKDestroyCommand{}).Execute(context.Background(), nil))
	})

	t.Run("destroy proceeds on yes input", func(t *testing.T) {
		require.NoError(t, os.Chdir(tmpDir))
		restore := replaceStdin(t, "y\n")
		defer restore()

		require.NoError(t, (&CDKDestroyCommand{}).Execute(context.Background(), []string{"--all"}))

		content, err := os.ReadFile(recordFile)
		require.NoError(t, err)
		require.Contains(t, string(content), "destroy --force --all")
	})

	t.Run("commands error when cdk dir missing", func(t *testing.T) {
		tmp := t.TempDir()
		require.NoError(t, os.Chdir(tmp))

		require.Error(t, (&CDKSynthCommand{}).Execute(context.Background(), nil))
		require.Error(t, (&CDKDiffCommand{}).Execute(context.Background(), nil))
		require.Error(t, (&CDKDestroyCommand{}).Execute(context.Background(), nil))
	})
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0755))
}

func replaceStdin(t *testing.T, input string) func() {
	t.Helper()

	old := os.Stdin
	r, w, err := os.Pipe()
	require.NoError(t, err)

	if input != "" {
		_, _ = io.WriteString(w, input)
	}
	_ = w.Close()

	os.Stdin = r
	return func() {
		os.Stdin = old
		_ = r.Close()
	}
}
