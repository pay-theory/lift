package cli

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCLI_Execute_HelpUnknownAndVersion(t *testing.T) {
	cli := NewCLI("v1.2.3")

	require.Contains(t, cli.ListCommands(), "help")
	require.Contains(t, cli.ListCommands(), "version")

	require.NoError(t, cli.Execute(context.Background(), []string{}))

	err := cli.Execute(context.Background(), []string{"nope"})
	require.Error(t, err)

	require.NoError(t, cli.Execute(context.Background(), []string{"version"}))
}

func TestHelpCommand_Execute_SpecificAndUnknown(t *testing.T) {
	cli := NewCLI("v0")
	help := cli.ListCommands()["help"]
	require.NotNil(t, help)

	require.NoError(t, help.Execute(context.Background(), []string{"version"}))
	require.Error(t, help.Execute(context.Background(), []string{"does-not-exist"}))
}

func TestCommands_Execute_CoversCommonBranches(t *testing.T) {
	t.Run("dev cancels via context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		require.NoError(t, (&DevCommand{}).Execute(ctx, []string{"--port=not-a-number", "--no-hot-reload"}))
	})

	t.Run("test parses flags and packages", func(t *testing.T) {
		require.NoError(t, (&TestCommand{}).Execute(context.Background(), []string{"--coverage", "--race", "./pkg/cli"}))
	})

	t.Run("benchmark parses flags and patterns", func(t *testing.T) {
		require.NoError(t, (&BenchmarkCommand{}).Execute(context.Background(), []string{"--cpu", "--mem", "MyBench"}))
	})

	t.Run("deploy requires environment arg", func(t *testing.T) {
		require.Error(t, (&DeployCommand{}).Execute(context.Background(), nil))
	})

	t.Run("metrics and health require function arg", func(t *testing.T) {
		require.Error(t, (&MetricsCommand{}).Execute(context.Background(), nil))
		require.Error(t, (&HealthCommand{}).Execute(context.Background(), nil))
	})

	t.Run("metrics success", func(t *testing.T) {
		require.NoError(t, (&MetricsCommand{}).Execute(context.Background(), []string{"fn", "--period=5m"}))
	})

	t.Run("health detailed success", func(t *testing.T) {
		require.NoError(t, (&HealthCommand{}).Execute(context.Background(), []string{"fn", "--detailed"}))
	})

	t.Run("logs validates args and follow cancel", func(t *testing.T) {
		require.Error(t, (&LogsCommand{}).Execute(context.Background(), nil))

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		start := time.Now()
		require.NoError(t, (&LogsCommand{}).Execute(ctx, []string{"fn", "--follow", "--since=10m"}))
		require.Less(t, time.Since(start), 5*time.Second)
	})
}

func TestDeployCommand_Execute_SuccessPath(t *testing.T) {
	start := time.Now()
	require.NoError(t, (&DeployCommand{}).Execute(context.Background(), []string{"dev", "--dry-run"}))
	require.Less(t, time.Since(start), 15*time.Second)
}
