// Package cli provides the Lift CLI command implementations.
package cli

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/pay-theory/lift/internal/liftconfig"
	"github.com/pay-theory/lift/internal/templates"
	"github.com/pay-theory/lift/pkg/utils/stdio"
)

// templateInfo holds information about a template including its embedded FS and root path.
type templateInfo struct {
	FS   embed.FS
	Root string
}

// availableTemplates lists all available templates.
var availableTemplates = map[string]templateInfo{
	"basic-api":     {FS: templates.BasicAPIFS, Root: "basic-api"},
	"microservice":  {FS: templates.MicroserviceFS, Root: "microservice"},
	"event-driven":  {FS: templates.EventDrivenFS, Root: "event-driven"},
	"merchant-app":  {FS: templates.MerchantAppFS, Root: "merchant-app"},
	"sns-processor": {FS: templates.SNSProcessorFS, Root: "sns-processor"},
}

// ptTemplates lists templates that have PT variants.
var ptTemplates = map[string]templateInfo{
	"basic-api":     {FS: templates.BasicAPIPTFS, Root: "basic-api-pt"},
	"microservice":  {FS: templates.MicroservicePTFS, Root: "microservice-pt"},
	"event-driven":  {FS: templates.EventDrivenPTFS, Root: "event-driven-pt"},
	"merchant-app":  {FS: templates.MerchantAppPTFS, Root: "merchant-app-pt"},
	"sns-processor": {FS: templates.SNSProcessorPTFS, Root: "sns-processor-pt"},
}

// NewCommandV2 implements the "lift new" command for Milestone 2+.
// It scaffolds a new Lift project using embedded templates.
type NewCommandV2 struct{}

func (c *NewCommandV2) Name() string        { return "new" }
func (c *NewCommandV2) Description() string { return "Create a new Lift project" }
func (c *NewCommandV2) Usage() string {
	return `lift new [<app>] --template <template> --base-domain <apex>

Flags:
  --template <name>       Template to use (default: basic-api)
  --base-domain <apex>    Base domain for the project (required for domain-enabled templates)
  --no-data               Scaffold without DynamoDB (templates include data by default)
  --pt                    Generate Pay Theory devops assets (buildspec.yml, shell/*)
                          instead of GitHub Actions workflows

Available templates:
  - basic-api (default)   Single Lambda API with minimal infrastructure
  - microservice          Lightweight single Lambda microservice
  - event-driven          API + processor with SQS queue for async processing
  - merchant-app          Multi-Lambda, multi-stack (data + service) architecture
  - sns-processor         SNS topic + processor Lambda + DynamoDB table

Examples:
  lift new my-app --base-domain example.com
  lift new --template basic-api --base-domain example.com
  lift new my-app --template microservice --base-domain example.com
  lift new my-app --template event-driven --base-domain example.com
  lift new my-app --template merchant-app --base-domain example.com
  lift new my-app --template sns-processor --base-domain example.com
  lift new my-app --template microservice --base-domain example.com --no-data
  lift new my-app --base-domain example.com --pt`
}

// TemplateData contains the data passed to templates during rendering.
type TemplateData struct {
	AppName    string
	ModuleName string
	BaseDomain string
	EnableData bool
}

// Execute runs the new command with the provided arguments.
func (c *NewCommandV2) Execute(_ context.Context, args []string) error {
	// Parse arguments
	opts, err := c.parseArgs(args)
	if err != nil {
		return err
	}

	// If opts is nil, --help was printed
	if opts == nil {
		return nil
	}

	// Validate options
	if validateErr := c.validateOpts(opts); validateErr != nil {
		return validateErr
	}

	// Determine target directory
	targetDir, err := c.resolveTargetDir(opts.appName)
	if err != nil {
		return err
	}

	// Check for existing lift.yaml
	if err := c.checkExistingProject(targetDir); err != nil {
		return err
	}

	// Scaffold the project
	if err := c.scaffold(targetDir, opts); err != nil {
		return err
	}

	// Print success message
	c.printSuccess(targetDir, opts.appName, opts.pt, opts.template)

	return nil
}

// newOpts holds parsed command options.
type newOpts struct {
	appName    string
	template   string
	baseDomain string
	pt         bool
	noData     bool
}

func (c *NewCommandV2) parseArgs(args []string) (*newOpts, error) {
	opts := &newOpts{
		template: "basic-api", // default
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]

		switch {
		case arg == "--template" && i+1 < len(args):
			i++
			opts.template = args[i]
		case strings.HasPrefix(arg, "--template="):
			opts.template = strings.TrimPrefix(arg, "--template=")
		case arg == "--base-domain" && i+1 < len(args):
			i++
			opts.baseDomain = args[i]
		case strings.HasPrefix(arg, "--base-domain="):
			opts.baseDomain = strings.TrimPrefix(arg, "--base-domain=")
		case arg == "--help" || arg == "-h":
			stdio.Stdoutln(c.Usage())
			return nil, nil // Signal to caller to exit without error
		case arg == "--pt":
			opts.pt = true
		case arg == "--no-data":
			opts.noData = true
		case strings.HasPrefix(arg, "-"):
			return nil, fmt.Errorf("unknown flag: %s\nUsage: %s", arg, c.Usage())
		default:
			// Positional argument (app name)
			if opts.appName == "" {
				opts.appName = arg
			} else {
				return nil, fmt.Errorf("unexpected argument: %s\nUsage: %s", arg, c.Usage())
			}
		}
	}

	return opts, nil
}

func (c *NewCommandV2) validateOpts(opts *newOpts) error {
	// Validate template
	if _, ok := availableTemplates[opts.template]; !ok {
		templateList := make([]string, 0, len(availableTemplates))
		for name := range availableTemplates {
			templateList = append(templateList, name)
		}
		return fmt.Errorf("unknown template: %s\n\nAvailable templates:\n  - %s", opts.template, strings.Join(templateList, "\n  - "))
	}

	// Validate PT mode is available for the selected template
	if opts.pt {
		if _, ok := ptTemplates[opts.template]; !ok {
			templateList := make([]string, 0, len(ptTemplates))
			for name := range ptTemplates {
				templateList = append(templateList, name)
			}
			return fmt.Errorf("--pt mode is not available for template %s\n\nTemplates with PT support:\n  - %s", opts.template, strings.Join(templateList, "\n  - "))
		}
	}

	// Validate base-domain for domain-enabled templates
	if opts.baseDomain == "" {
		return fmt.Errorf("--base-domain is required for template %s\n\nExample: lift new my-app --base-domain example.com", opts.template)
	}

	return nil
}

func (c *NewCommandV2) resolveTargetDir(appName string) (string, error) {
	if appName == "" {
		// Bootstrap current directory
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current directory: %w", err)
		}
		return cwd, nil
	}

	// Create new directory for the app
	absPath, err := filepath.Abs(appName)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path %s: %w", appName, err)
	}

	// Check if directory already exists
	if info, err := os.Stat(absPath); err == nil {
		if info.IsDir() {
			return "", fmt.Errorf("directory %s already exists\n\nTo bootstrap an existing directory, cd into it and run: lift new --base-domain <apex>", appName)
		}
		return "", fmt.Errorf("path %s already exists and is not a directory", appName)
	}

	return absPath, nil
}

func (c *NewCommandV2) checkExistingProject(targetDir string) error {
	configPath := filepath.Join(targetDir, liftconfig.ConfigFileName)
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("%s already exists in %s\n\nThis directory is already a Lift project. To re-scaffold, remove %s first",
			liftconfig.ConfigFileName, targetDir, liftconfig.ConfigFileName)
	}
	return nil
}

func (c *NewCommandV2) scaffold(targetDir string, opts *newOpts) error {
	data := c.buildTemplateData(targetDir, opts)

	// Create target directory if needed
	if err := os.MkdirAll(targetDir, 0750); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", targetDir, err)
	}

	tmplInfo := availableTemplates[opts.template]
	if err := c.scaffoldTemplateFiles(targetDir, tmplInfo, data, opts); err != nil {
		return err
	}

	// In PT mode, also scaffold PT-specific files (buildspec.yml, shell/*)
	if opts.pt {
		if err := c.scaffoldPTFiles(targetDir, data, opts.template); err != nil {
			return fmt.Errorf("failed to scaffold PT files: %w", err)
		}
	}

	return nil
}

func (c *NewCommandV2) buildTemplateData(targetDir string, opts *newOpts) TemplateData {
	appName := opts.appName
	if appName == "" {
		appName = filepath.Base(targetDir)
	}

	return TemplateData{
		AppName:    appName,
		ModuleName: appName,
		BaseDomain: opts.baseDomain,
		EnableData: !opts.noData,
	}
}

func (c *NewCommandV2) scaffoldTemplateFiles(targetDir string, tmplInfo templateInfo, data TemplateData, opts *newOpts) error {
	err := fs.WalkDir(tmplInfo.FS, tmplInfo.Root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relPath, err := filepath.Rel(tmplInfo.Root, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		if skip, skipDir := c.shouldSkipTemplatePath(relPath, d, opts); skip {
			if skipDir {
				return fs.SkipDir
			}
			return nil
		}

		targetPath := filepath.Join(targetDir, strings.TrimSuffix(relPath, ".tmpl"))

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0750)
		}

		content, err := tmplInfo.FS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read template %s: %w", path, err)
		}

		rendered, err := c.renderTemplate(string(content), data)
		if err != nil {
			return fmt.Errorf("failed to render template %s: %w", path, err)
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0750); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", targetPath, err)
		}

		if err := os.WriteFile(targetPath, []byte(rendered), 0600); err != nil {
			return fmt.Errorf("failed to write %s: %w", targetPath, err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to scaffold project: %w", err)
	}

	return nil
}

func (c *NewCommandV2) shouldSkipTemplatePath(relPath string, d fs.DirEntry, opts *newOpts) (skip bool, skipDir bool) {
	if !opts.pt {
		return false, false
	}

	// In PT mode, skip base template files that are replaced by PT variants.
	if relPath == "README.md.tmpl" || relPath == "README.md" {
		return true, false
	}
	if relPath == "lift.yaml.tmpl" || relPath == "lift.yaml" {
		return true, false
	}
	if relPath == "cdk/main.go.tmpl" || relPath == "cdk/main.go" {
		return true, false
	}

	// In PT mode, skip .github directory entirely (PT uses buildspec/shell).
	if relPath == ".github" || strings.HasPrefix(relPath, ".github/") {
		return true, d.IsDir()
	}

	return false, false
}

// scaffoldPTFiles scaffolds Pay Theory-style devops files (buildspec.yml, shell/*)
func (c *NewCommandV2) scaffoldPTFiles(targetDir string, data TemplateData, templateName string) error {
	ptTemplate, ok := ptTemplates[templateName]
	if !ok {
		return fmt.Errorf("internal error: PT template not found for %s", templateName)
	}

	return fs.WalkDir(ptTemplate.FS, ptTemplate.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Get relative path from template root
		relPath, err := filepath.Rel(ptTemplate.Root, path)
		if err != nil {
			return err
		}

		// Skip root
		if relPath == "." {
			return nil
		}

		// Compute target path
		targetPath := filepath.Join(targetDir, relPath)

		// Strip .tmpl extension (TrimSuffix is a no-op if suffix not present)
		targetPath = strings.TrimSuffix(targetPath, ".tmpl")

		if d.IsDir() {
			// Create directory
			return os.MkdirAll(targetPath, 0750)
		}

		// Read and render template file
		content, err := ptTemplate.FS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read PT template %s: %w", path, err)
		}

		// Parse and execute template
		rendered, err := c.renderTemplate(string(content), data)
		if err != nil {
			return fmt.Errorf("failed to render PT template %s: %w", path, err)
		}

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(targetPath), 0750); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", targetPath, err)
		}

		// Determine file permissions (shell scripts need executable bit)
		perm := os.FileMode(0600)
		if strings.HasSuffix(relPath, ".sh.tmpl") || strings.HasSuffix(relPath, ".sh") {
			perm = 0750
		}

		// Write file
		if err := os.WriteFile(targetPath, []byte(rendered), perm); err != nil {
			return fmt.Errorf("failed to write %s: %w", targetPath, err)
		}

		return nil
	})
}

func (c *NewCommandV2) renderTemplate(content string, data TemplateData) (string, error) {
	tmpl, err := template.New("file").Parse(content)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (c *NewCommandV2) printSuccess(targetDir string, appName string, ptMode bool, templateName string) {
	name := appName
	if name == "" {
		name = filepath.Base(targetDir)
	}

	stdio.Stdoutf("✅ Created Lift project: %s\n", name)
	stdio.Stdoutf("📁 Location: %s\n", targetDir)
	stdio.Stdoutf("📦 Template: %s\n", templateName)
	stdio.Stdoutf("\n")
	stdio.Stdoutf("📁 Project structure:\n")
	stdio.Stdoutf("   %s/\n", name)
	stdio.Stdoutf("   ├── lift.yaml          # Project configuration\n")
	stdio.Stdoutf("   ├── go.mod             # Go module\n")

	// Print function structure based on template
	switch templateName {
	case "event-driven":
		stdio.Stdoutf("   ├── cmd/api/main.go    # API Lambda entrypoint\n")
		stdio.Stdoutf("   ├── cmd/processor/main.go  # Processor Lambda entrypoint\n")
	case "merchant-app":
		stdio.Stdoutf("   ├── cmd/api/main.go    # API Lambda entrypoint\n")
		stdio.Stdoutf("   ├── cmd/worker/main.go # Worker Lambda entrypoint\n")
	case "sns-processor":
		stdio.Stdoutf("   ├── cmd/processor/main.go  # SNS processor Lambda entrypoint\n")
	default:
		stdio.Stdoutf("   ├── cmd/api/main.go    # Lambda entrypoint\n")
	}

	stdio.Stdoutf("   ├── cdk/               # CDK infrastructure\n")
	stdio.Stdoutf("   │   ├── cdk.json\n")
	stdio.Stdoutf("   │   ├── go.mod\n")
	stdio.Stdoutf("   │   └── main.go\n")

	if ptMode {
		// PT mode structure
		stdio.Stdoutf("   ├── buildspec.yml      # CodeBuild spec\n")
		stdio.Stdoutf("   └── shell/             # Deploy scripts\n")
		stdio.Stdoutf("       ├── build.sh\n")
		stdio.Stdoutf("       ├── deploy.sh\n")
		stdio.Stdoutf("       ├── init_env_vars.sh\n")
		stdio.Stdoutf("       └── DEPLOYMENT.md\n")
	} else {
		// GitHub Actions structure
		stdio.Stdoutf("   └── .github/workflows/ # CI/CD\n")
		stdio.Stdoutf("       ├── deploy.yml\n")
		stdio.Stdoutf("       └── pr.yml\n")
	}

	stdio.Stdoutf("\n")
	stdio.Stdoutf("🚀 Next steps:\n")
	if appName != "" {
		stdio.Stdoutf("   cd %s\n", appName)
	}
	stdio.Stdoutf("   go mod tidy\n")

	if ptMode {
		// PT mode next steps
		stdio.Stdoutf("   ./shell/deploy.sh --partner <partner> --stage dev\n")
		stdio.Stdoutf("\n")
		stdio.Stdoutf("📖 Or use lift CLI with partner context:\n")
		stdio.Stdoutf("   lift up --stage dev --partner <partner>\n")
		stdio.Stdoutf("\n")
		stdio.Stdoutf("📖 CodeBuild setup:\n")
		stdio.Stdoutf("   Set environment variables: PARTNER, STAGE, AWS_REGION\n")
		stdio.Stdoutf("   See shell/DEPLOYMENT.md for details\n")
	} else {
		// GitHub Actions next steps
		stdio.Stdoutf("   lift up --stage dev\n")
		stdio.Stdoutf("\n")
		stdio.Stdoutf("📖 GitHub setup (for CI/CD):\n")
		stdio.Stdoutf("   1. Create GitHub Environments: dev, staging, live\n")
		stdio.Stdoutf("   2. Set environment variable: AWS_ROLE_ARN\n")
		stdio.Stdoutf("   3. Set environment variable: AWS_REGION (recommended)\n")
	}
}
