package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestAddCommand_UpdateLiftYAML_CreatesFunctionsSection(t *testing.T) {
	tmpDir := t.TempDir()

	liftYAML := `version: 1
app:
  name: test-app
  template: basic-api
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "lift.yaml"), []byte(liftYAML), 0600))

	cmd := &AddCommand{}
	require.NoError(t, cmd.updateLiftYAML(tmpDir, "webhook"))

	type functionConfig struct {
		Cmd string `yaml:"cmd"`
		Out string `yaml:"out"`
	}
	var parsed struct {
		Functions map[string]functionConfig `yaml:"functions"`
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "lift.yaml"))
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(data, &parsed))

	require.Contains(t, parsed.Functions, "webhook")
	require.Equal(t, "./cmd/webhook", parsed.Functions["webhook"].Cmd)
	require.Equal(t, "./dist/webhook/bootstrap", parsed.Functions["webhook"].Out)
}

func TestAddCommand_UpdateLiftYAML_Errors(t *testing.T) {
	cmd := &AddCommand{}

	t.Run("empty file", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "lift.yaml"), []byte(""), 0600))
		err := cmd.updateLiftYAML(tmpDir, "x")
		require.Error(t, err)
		require.Contains(t, err.Error(), "empty lift.yaml")
	})

	t.Run("root not mapping", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "lift.yaml"), []byte("- not-a-map\n"), 0600))
		err := cmd.updateLiftYAML(tmpDir, "x")
		require.Error(t, err)
		require.Contains(t, err.Error(), "root is not a mapping")
	})

	t.Run("invalid yaml", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "lift.yaml"), []byte(":\n"), 0600))
		err := cmd.updateLiftYAML(tmpDir, "x")
		require.Error(t, err)
	})
}

func TestAddCommand_UpdateCDK_FallbackAndErrors(t *testing.T) {
	cmd := &AddCommand{}

	t.Run("fallback inserts before synth when no marker", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "cdk"), 0750))

		// No marker line, but includes app.Synth(nil) for fallback insertion.
		cdkMain := `package main

func main() {
	stack := "stack"
	appNameStr := "test-app"
	stageStr := "dev"

	app.Synth(nil)
}
`
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "cdk", "main.go"), []byte(cdkMain), 0600))

		require.NoError(t, cmd.updateCDK(tmpDir, "worker", "test-app", false))

		updated, err := os.ReadFile(filepath.Join(tmpDir, "cdk", "main.go"))
		require.NoError(t, err)
		updatedStr := string(updated)

		insertLoc := strings.Index(updatedStr, "workerFunction")
		synthLoc := strings.Index(updatedStr, "app.Synth(nil)")
		require.Greater(t, insertLoc, -1)
		require.Greater(t, synthLoc, -1)
		require.Less(t, insertLoc, synthLoc)
	})

	t.Run("errors when no marker and no synth", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "cdk"), 0750))

		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "cdk", "main.go"), []byte("package main\n\nfunc main() {}\n"), 0600))
		err := cmd.updateCDK(tmpDir, "worker", "test-app", false)
		require.Error(t, err)
		require.Contains(t, err.Error(), "insertion point")
	})

	t.Run("writes unformatted content on go/format failure", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "cdk"), 0750))

		// Intentionally invalid Go (missing closing brace) so go/format fails.
		cdkMain := `package main

func main() {
	stack := "stack"
	appNameStr := "test-app"
	stageStr := "dev"

	// LIFT:ADD_FUNCTIONS

	app.Synth(nil)
`
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "cdk", "main.go"), []byte(cdkMain), 0600))
		require.NoError(t, cmd.updateCDK(tmpDir, "worker", "test-app", false))

		updated, err := os.ReadFile(filepath.Join(tmpDir, "cdk", "main.go"))
		require.NoError(t, err)
		require.Contains(t, string(updated), "workerFunction")
	})
}

func TestAddCommand_UpdateBuildspecYML_FallbackAndNoop(t *testing.T) {
	cmd := &AddCommand{}

	t.Run("skips when buildspec.yml missing", func(t *testing.T) {
		require.NoError(t, cmd.updateBuildspecYML(t.TempDir(), "worker"))
	})

	t.Run("fallback inserts before synth when marker missing", func(t *testing.T) {
		tmpDir := t.TempDir()
		buildspec := `version: 0.2
phases:
  build:
    commands:
      - echo "Building api..."
      - echo "Synthesizing CDK stacks..."
`
		path := filepath.Join(tmpDir, "buildspec.yml")
		require.NoError(t, os.WriteFile(path, []byte(buildspec), 0600))

		require.NoError(t, cmd.updateBuildspecYML(tmpDir, "worker"))

		updated, err := os.ReadFile(path)
		require.NoError(t, err)
		updatedStr := string(updated)

		insertLoc := strings.Index(updatedStr, "mkdir -p dist/worker")
		synthLoc := strings.Index(updatedStr, `echo "Synthesizing CDK stacks..."`)
		require.Greater(t, insertLoc, -1)
		require.Greater(t, synthLoc, -1)
		require.Less(t, insertLoc, synthLoc)
	})

	t.Run("no marker or synth leaves content unchanged", func(t *testing.T) {
		tmpDir := t.TempDir()
		buildspec := `version: 0.2
phases:
  build:
    commands:
      - echo "Building api..."
`
		path := filepath.Join(tmpDir, "buildspec.yml")
		require.NoError(t, os.WriteFile(path, []byte(buildspec), 0600))

		require.NoError(t, cmd.updateBuildspecYML(tmpDir, "worker"))

		updated, err := os.ReadFile(path)
		require.NoError(t, err)
		require.NotContains(t, string(updated), "mkdir -p dist/worker")
	})
}

func TestAddCommand_UpdateBuildSH_NoopsWhenMissingOrMarkersAbsent(t *testing.T) {
	cmd := &AddCommand{}

	t.Run("skips when shell/build.sh missing", func(t *testing.T) {
		require.NoError(t, cmd.updateBuildSH(t.TempDir(), "worker"))
	})

	t.Run("writes even when markers absent", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "shell"), 0750))

		path := filepath.Join(tmpDir, "shell", "build.sh")
		contents := "#!/bin/bash\necho hi\n"
		require.NoError(t, os.WriteFile(path, []byte(contents), 0600))

		require.NoError(t, cmd.updateBuildSH(tmpDir, "worker"))

		updated, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, contents, string(updated))
	})
}

func TestAddCommand_UpdatePTBuildFiles_WrapsErrors(t *testing.T) {
	cmd := &AddCommand{}

	t.Run("buildspec read error is wrapped", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(tmpDir, "buildspec.yml"), 0750))

		err := cmd.updatePTBuildFiles(tmpDir, "worker")
		require.Error(t, err)
		require.Contains(t, err.Error(), "buildspec.yml:")
	})

	t.Run("shell build read error is wrapped", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "shell", "build.sh"), 0750))

		err := cmd.updatePTBuildFiles(tmpDir, "worker")
		require.Error(t, err)
		require.Contains(t, err.Error(), "shell/build.sh:")
	})
}
