// Package cli provides the Lift CLI command implementations.
package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pay-theory/lift/internal/liftconfig"
)

// BuildCommand builds Lambda functions defined in lift.yaml.
// It reads the functions map from configuration and produces the configured
// output files (e.g. dist/<fn>/bootstrap) for each function.
type BuildCommand struct {
	// For testing: allows overriding how commands are created
	cmdFactory func(ctx context.Context, name string, arg ...string) *exec.Cmd
	// For testing: allows overriding binary lookup
	lookPath LookPathFunc
}

func (c *BuildCommand) Name() string {
	return "build"
}

func (c *BuildCommand) Description() string {
	return "Build Lambda functions defined in lift.yaml"
}

func (c *BuildCommand) Usage() string {
	return "lift build [--arch arm64|amd64]"
}

func parseArchOverride(args []string) (string, error) {
	archOverride := ""

	for i := 0; i < len(args); i++ {
		arg := args[i]

		switch {
		case arg == "--arch" && i+1 < len(args):
			i++
			archOverride = args[i]
		case strings.HasPrefix(arg, "--arch="):
			archOverride = strings.TrimPrefix(arg, "--arch=")
		}

		if archOverride != "" && archOverride != "arm64" && archOverride != "amd64" {
			return "", fmt.Errorf("invalid architecture: %s (must be arm64 or amd64)", archOverride)
		}
	}

	return archOverride, nil
}

// Execute builds all Lambda functions defined in lift.yaml.
// It supports running from project root or any subdirectory.
func (c *BuildCommand) Execute(ctx context.Context, args []string) error {
	archOverride, err := parseArchOverride(args)
	if err != nil {
		return err
	}

	// Check for Go binary before proceeding
	if prereqErr := CheckGo(c.lookPath); prereqErr != nil {
		return prereqErr
	}

	root, cfg, err := loadProjectConfigFromCwd()
	if err != nil {
		return err
	}

	// Validate functions are defined
	if len(cfg.Functions) == 0 {
		return fmt.Errorf("no functions defined in lift.yaml\n\nAdd a functions section to define Lambda functions to build:\n\nfunctions:\n  api:\n    cmd: ./cmd/api\n    out: ./dist/api/bootstrap")
	}

	// Determine build settings from config with defaults
	buildCfg := c.resolveBuildConfig(cfg, archOverride)

	// Build each function
	fmt.Printf("🔨 Building %d function(s) for Linux/%s...\n", len(cfg.Functions), buildCfg.goarch)

	names := make([]string, 0, len(cfg.Functions))
	for name := range cfg.Functions {
		names = append(names, name)
	}
	sort.Strings(names)

	retriedWithMod := false
	for _, name := range names {
		fn := cfg.Functions[name]
		if err := c.buildFunction(ctx, root, name, fn, buildCfg, &retriedWithMod); err != nil {
			return err
		}
	}

	fmt.Printf("\n✅ Build complete!\n")
	return nil
}

// buildConfig holds resolved build configuration
type buildConfig struct {
	goos     string
	goarch   string
	ldflags  string
	tags     []string
	cgo      bool
	trimpath bool
}

// resolveBuildConfig merges defaults with lift.yaml build config and CLI overrides
func (c *BuildCommand) resolveBuildConfig(cfg *liftconfig.Config, archOverride string) buildConfig {
	bc := buildConfig{
		goos:     "linux",
		goarch:   "arm64",
		cgo:      false,
		trimpath: true,
		ldflags:  "-s -w",
		tags:     []string{"lambda.norpc"},
	}

	// Apply lift.yaml build settings
	if cfg.Build != nil {
		if cfg.Build.GOOS != "" {
			bc.goos = cfg.Build.GOOS
		}
		if cfg.Build.GOARCH != "" {
			bc.goarch = cfg.Build.GOARCH
		}
		if cfg.Build.CGO != 0 {
			bc.cgo = cfg.Build.CGO != 0
		}
		if cfg.Build.Trimpath != nil {
			bc.trimpath = *cfg.Build.Trimpath
		}
		if cfg.Build.LDFlags != "" {
			bc.ldflags = cfg.Build.LDFlags
		}
		if len(cfg.Build.Tags) > 0 {
			bc.tags = mergeTags(bc.tags, cfg.Build.Tags)
		}
	}

	// CLI --arch overrides config
	if archOverride != "" {
		bc.goarch = archOverride
	}

	return bc
}

func mergeTags(base, extra []string) []string {
	seen := make(map[string]struct{}, len(base)+len(extra))
	merged := make([]string, 0, len(base)+len(extra))

	for _, tag := range base {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		merged = append(merged, tag)
	}

	for _, tag := range extra {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		merged = append(merged, tag)
	}

	return merged
}

func validateFunctionConfig(name string, fn *liftconfig.Function) error {
	if fn.Cmd == "" {
		return fmt.Errorf("function %q is missing 'cmd' field (package path to build)", name)
	}
	if fn.Out == "" {
		return fmt.Errorf("function %q is missing 'out' field (output path)", name)
	}
	return nil
}

func resolveOutputPath(root, out string) string {
	if filepath.IsAbs(out) {
		return out
	}
	return filepath.Join(root, out)
}

func buildGoBuildArgs(outPath, pkgPath string, bc buildConfig) []string {
	args := []string{"build"}

	if bc.trimpath {
		args = append(args, "-trimpath")
	}

	if bc.ldflags != "" {
		args = append(args, "-ldflags", bc.ldflags)
	}

	if len(bc.tags) > 0 {
		args = append(args, "-tags", strings.Join(bc.tags, ","))
	}

	return append(args, "-o", outPath, pkgPath)
}

func (c *BuildCommand) runGoBuild(ctx context.Context, root string, args []string, bc buildConfig) (string, error) {
	var cmd *exec.Cmd
	if c.cmdFactory != nil {
		cmd = c.cmdFactory(ctx, "go", args...)
	} else {
		cmd = exec.CommandContext(ctx, "go", args...)
	}

	cmd.Dir = root

	cgoEnabled := "0"
	if bc.cgo {
		cgoEnabled = "1"
	}

	env := cmd.Env
	if len(env) == 0 {
		env = os.Environ()
	}
	env = append(env,
		"GOOS="+bc.goos,
		"GOARCH="+bc.goarch,
		"CGO_ENABLED="+cgoEnabled,
	)
	cmd.Env = env

	cmd.Stdout = os.Stdout

	var stderr bytes.Buffer
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)

	err := cmd.Run()
	return stderr.String(), err
}

func formatBuildFailure(name string, args []string, stderr string, err error) error {
	stderr = strings.TrimSpace(stderr)
	if stderr == "" {
		return fmt.Errorf("build failed for function %q\n  command: go %s\n  error: %w",
			name, strings.Join(args, " "), err)
	}

	return fmt.Errorf("build failed for function %q\n  command: go %s\n  error: %w\n  stderr: %s",
		name, strings.Join(args, " "), err, stderr)
}

// buildFunction builds a single Lambda function
func (c *BuildCommand) buildFunction(ctx context.Context, root, name string, fn *liftconfig.Function, bc buildConfig, retriedWithMod *bool) error {
	if err := validateFunctionConfig(name, fn); err != nil {
		return err
	}

	fmt.Printf("  📦 %s: %s → %s\n", name, fn.Cmd, fn.Out)

	outPath := resolveOutputPath(root, fn.Out)

	// Ensure output directory exists
	outDir := filepath.Dir(outPath)
	if err := os.MkdirAll(outDir, 0750); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", outDir, err)
	}

	buildArgs := buildGoBuildArgs(outPath, fn.Cmd, bc)

	failedArgs := buildArgs
	stderr, err := c.runGoBuild(ctx, root, buildArgs, bc)
	if err == nil {
		return nil
	}

	if shouldRetryWithMod(stderr) && retriedWithMod != nil && !*retriedWithMod {
		fmt.Printf("  🔧 %s: go module metadata needs updates; retrying with `-mod=mod`...\n", name)
		*retriedWithMod = true

		buildArgsMod := append([]string{buildArgs[0], "-mod=mod"}, buildArgs[1:]...)
		failedArgs = buildArgsMod
		stderr, err = c.runGoBuild(ctx, root, buildArgsMod, bc)
		if err == nil {
			return nil
		}
	}

	return formatBuildFailure(name, failedArgs, stderr, err)
}

func shouldRetryWithMod(stderr string) bool {
	if strings.Contains(stderr, "missing go.sum entry") {
		return true
	}
	if strings.Contains(stderr, "updates to go.mod needed") {
		return true
	}
	return false
}
