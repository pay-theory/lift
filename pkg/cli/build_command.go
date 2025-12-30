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

// Execute builds all Lambda functions defined in lift.yaml.
// It supports running from project root or any subdirectory.
func (c *BuildCommand) Execute(ctx context.Context, args []string) error {
	// Parse --arch flag (override from CLI)
	archOverride := ""
	for i, arg := range args {
		if arg == "--arch" && i+1 < len(args) {
			archOverride = args[i+1]
			if archOverride != "arm64" && archOverride != "amd64" {
				return fmt.Errorf("invalid architecture: %s (must be arm64 or amd64)", archOverride)
			}
		}
		if strings.HasPrefix(arg, "--arch=") {
			archOverride = strings.TrimPrefix(arg, "--arch=")
			if archOverride != "arm64" && archOverride != "amd64" {
				return fmt.Errorf("invalid architecture: %s (must be arm64 or amd64)", archOverride)
			}
		}
	}

	// Check for Go binary before proceeding
	if err := CheckGo(c.lookPath); err != nil {
		return err
	}

	// Find project root
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	root, err := liftconfig.FindProjectRoot(cwd)
	if err != nil {
		// ProjectNotFoundError already has actionable message
		return err
	}

	// Load configuration
	cfg, err := liftconfig.LoadConfig(root)
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
	cgo      bool
	trimpath bool
	ldflags  string
	tags     []string
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
	var merged []string

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

// buildFunction builds a single Lambda function
func (c *BuildCommand) buildFunction(ctx context.Context, root, name string, fn *liftconfig.Function, bc buildConfig, retriedWithMod *bool) error {
	// Validate function config
	if fn.Cmd == "" {
		return fmt.Errorf("function %q is missing 'cmd' field (package path to build)", name)
	}
	if fn.Out == "" {
		return fmt.Errorf("function %q is missing 'out' field (output path)", name)
	}

	fmt.Printf("  📦 %s: %s → %s\n", name, fn.Cmd, fn.Out)

	// Resolve output path relative to project root
	outPath := fn.Out
	if !filepath.IsAbs(outPath) {
		outPath = filepath.Join(root, outPath)
	}

	// Ensure output directory exists
	outDir := filepath.Dir(outPath)
	if err := os.MkdirAll(outDir, 0750); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", outDir, err)
	}

	// Build go build arguments
	buildArgs := []string{"build"}

	// Add -trimpath if enabled
	if bc.trimpath {
		buildArgs = append(buildArgs, "-trimpath")
	}

	// Add -ldflags if set
	if bc.ldflags != "" {
		buildArgs = append(buildArgs, "-ldflags", bc.ldflags)
	}

	// Add -tags if set
	if len(bc.tags) > 0 {
		buildArgs = append(buildArgs, "-tags", strings.Join(bc.tags, ","))
	}

	// Add output and package path
	buildArgs = append(buildArgs, "-o", outPath, fn.Cmd)

	// Create command
	runBuild := func(args []string) (string, error) {
		var cmd *exec.Cmd
		if c.cmdFactory != nil {
			cmd = c.cmdFactory(ctx, "go", args...)
		} else {
			cmd = exec.CommandContext(ctx, "go", args...)
		}

		// Set working directory to project root
		cmd.Dir = root

		// Set environment variables (respect cmdFactory overrides)
		cgoEnabled := "0"
		if bc.cgo {
			cgoEnabled = "1"
		}
		baseEnv := cmd.Env
		if len(baseEnv) == 0 {
			baseEnv = os.Environ()
		}
		cmd.Env = append(baseEnv,
			"GOOS="+bc.goos,
			"GOARCH="+bc.goarch,
			"CGO_ENABLED="+cgoEnabled,
		)

		cmd.Stdout = os.Stdout

		var stderr bytes.Buffer
		cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)

		err := cmd.Run()
		return stderr.String(), err
	}

	failedArgs := buildArgs
	stderr, err := runBuild(buildArgs)
	if err == nil {
		return nil
	}

	if shouldRetryWithMod(stderr) && retriedWithMod != nil && !*retriedWithMod {
		fmt.Printf("  🔧 %s: go module metadata needs updates; retrying with `-mod=mod`...\n", name)
		*retriedWithMod = true

		buildArgsMod := append([]string{buildArgs[0], "-mod=mod"}, buildArgs[1:]...)
		failedArgs = buildArgsMod
		stderr, err = runBuild(buildArgsMod)
		if err == nil {
			return nil
		}
	}

	return fmt.Errorf("build failed for function %q\n  command: go %s\n  error: %w",
		name, strings.Join(failedArgs, " "), err)
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
