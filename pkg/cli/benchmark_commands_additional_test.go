package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDynamORMBenchmarkCommand_ParseBenchmarkArgs_DefaultsAndOverrides(t *testing.T) {
	cmd := &DynamORMBenchmarkCommand{}

	cfg, err := cmd.parseBenchmarkArgs([]string{tableFlag, "my-table"})
	require.NoError(t, err)
	require.Equal(t, "my-table", cfg.TableName)
	require.Equal(t, []string{"put", "get", "query"}, cfg.Operations)
	require.Equal(t, 10, cfg.Concurrency)
	require.Equal(t, 30*time.Second, cfg.Duration)
	require.Equal(t, 1024, cfg.ItemSize)
	require.Equal(t, "benchmarks", cfg.OutputDir)
	require.Equal(t, "us-east-1", cfg.Region)
	require.Equal(t, 5*time.Second, cfg.Warmup)

	cfg, err = cmd.parseBenchmarkArgs([]string{
		tableFlag, "my-table",
		"--operations", "put,get",
		"--concurrency", "25",
		"--duration", "45s",
		"--output-dir", "out",
		"--region", "us-west-2",
		"--unknown-flag", "ignored",
	})
	require.NoError(t, err)
	require.Equal(t, []string{"put", "get"}, cfg.Operations)
	require.Equal(t, 25, cfg.Concurrency)
	require.Equal(t, 45*time.Second, cfg.Duration)
	require.Equal(t, "out", cfg.OutputDir)
	require.Equal(t, "us-west-2", cfg.Region)
}

func TestDynamORMBenchmarkCommand_ParseBenchmarkArgs_ValidationErrors(t *testing.T) {
	cmd := &DynamORMBenchmarkCommand{}

	_, err := cmd.parseBenchmarkArgs([]string{})
	require.Error(t, err)

	_, err = cmd.parseBenchmarkArgs([]string{tableFlag})
	require.Error(t, err)

	_, err = cmd.parseBenchmarkArgs([]string{tableFlag, "t", "--concurrency"})
	require.Error(t, err)

	_, err = cmd.parseBenchmarkArgs([]string{tableFlag, "t", "--concurrency", "nope"})
	require.Error(t, err)

	_, err = cmd.parseBenchmarkArgs([]string{tableFlag, "t", "--duration"})
	require.Error(t, err)

	_, err = cmd.parseBenchmarkArgs([]string{tableFlag, "t", "--duration", "nope"})
	require.Error(t, err)
}

func TestDynamORMBenchmarkCommand_IsLiftProject_AndGenerateBenchmarkCode(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &DynamORMBenchmarkCommand{}
	require.False(t, cmd.isLiftProject())

	require.NoError(t, os.WriteFile("go.mod", []byte("module example.com/test\n\nrequire github.com/pay-theory/lift v0.0.0\n"), 0600))
	require.True(t, cmd.isLiftProject())

	cfg := &BenchmarkConfig{
		TableName:   "my-table",
		OutputDir:   "bench-out",
		Region:      "us-east-1",
		Operations:  []string{"put"},
		Concurrency: 1,
		Duration:    time.Second,
		ItemSize:    128,
		Warmup:      time.Second,
	}
	require.NoError(t, cmd.generateBenchmarkCode(cfg))
	assertFileExists(t, filepath.Join(tmpDir, "bench-out", "benchmark_runner.go"))
}

func TestDynamORMBenchmarkCommand_Execute_GeneratesRunner(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	require.NoError(t, os.WriteFile("go.mod", []byte("module example.com/test\n\nrequire github.com/pay-theory/lift v0.0.0\n"), 0600))

	cmd := &DynamORMBenchmarkCommand{}
	require.NoError(t, cmd.Execute(context.Background(), []string{
		"--table", "my-table",
		"--output-dir", "benchmarks",
	}))

	assertFileExists(t, filepath.Join(tmpDir, "benchmarks", "benchmark_runner.go"))
}
