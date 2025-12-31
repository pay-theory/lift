package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommandMetadata_NameDescriptionUsage_AreNotEmpty(t *testing.T) {
	cli := NewCLI("v0.0.0")

	commands := []Command{
		&NewCommandV2{},
		&DevCommand{},
		&TestCommand{},
		&BenchmarkCommand{},
		&DeployCommand{},
		&LogsCommand{},
		&MetricsCommand{},
		&HealthCommand{},
		&VersionCommand{version: "v0.0.0"},
		&HelpCommand{cli: cli},
		&BuildCommand{},
		&CDKInitCommand{},
		&CDKDeployCommand{},
		&CDKSynthCommand{},
		&CDKDiffCommand{},
		&CDKDestroyCommand{},
		&DynamORMScaffoldCommand{},
		&DynamORMMigrateCommand{},
		&DynamORMBenchmarkCommand{},
		&UpCommand{},
		&DownCommand{},
		&AddCommand{},
	}

	for _, cmd := range commands {
		require.NotEmpty(t, cmd.Name())
		require.NotEmpty(t, cmd.Description())
		require.NotEmpty(t, cmd.Usage())
	}
}
