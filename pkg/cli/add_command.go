// Package cli provides the Lift CLI command implementations.
package cli

import (
	"context"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/pay-theory/lift/internal/liftconfig"
	"github.com/pay-theory/lift/pkg/utils/stdio"
)

// AddCommand implements the "lift add" command for incremental scaffolding.
// Currently supports: lift add function <name>
type AddCommand struct{}

func (c *AddCommand) Name() string        { return "add" }
func (c *AddCommand) Description() string { return "Add components to an existing Lift project" }
func (c *AddCommand) Usage() string {
	return `lift add function <name>

Subcommands:
  function <name>    Add a new Lambda function to the project

The function name must be a valid identifier (alphanumeric and underscores only).
This command will:
  - Create cmd/<name>/main.go with a Lift handler skeleton
  - Update lift.yaml to include the new function
  - Update cdk/main.go to deploy the new function
  - For PT projects: update buildspec.yml and shell/build.sh

Examples:
  lift add function webhook
  lift add function payment_processor`
}

// Execute runs the add command
func (c *AddCommand) Execute(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("subcommand required\n\n%s", c.Usage())
	}

	subcommand := args[0]
	switch subcommand {
	case "function":
		return c.executeAddFunction(ctx, args[1:])
	default:
		return fmt.Errorf("unknown subcommand: %s\n\n%s", subcommand, c.Usage())
	}
}

func (c *AddCommand) executeAddFunction(_ context.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("function name required\n\nUsage: lift add function <name>")
	}

	name := args[0]

	// Validate function name
	if err := validateFunctionName(name); err != nil {
		return err
	}

	// Find project root
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	root, err := liftconfig.FindProjectRoot(cwd)
	if err != nil {
		return err
	}

	// Load and validate configuration
	cfg, err := liftconfig.LoadConfig(root)
	if err != nil {
		return err
	}

	// Check if function already exists in config
	if cfg.Functions != nil {
		if _, exists := cfg.Functions[name]; exists {
			return fmt.Errorf("function %q already exists in lift.yaml", name)
		}
	}

	// Check if cmd/<name> already exists
	cmdDir := filepath.Join(root, "cmd", name)
	if _, err := os.Stat(cmdDir); err == nil {
		return fmt.Errorf("directory cmd/%s already exists", name)
	}

	stdio.Stdoutf("🔧 Adding function %q to %s...\n\n", name, cfg.App.Name)

	// 1. Create cmd/<name>/main.go
	if err := c.createFunctionCode(root, name, cfg.App.Name); err != nil {
		return fmt.Errorf("failed to create function code: %w", err)
	}
	stdio.Stdoutf("  ✅ Created cmd/%s/main.go\n", name)

	// 2. Update lift.yaml
	if err := c.updateLiftYAML(root, name); err != nil {
		return fmt.Errorf("failed to update lift.yaml: %w", err)
	}
	stdio.Stdoutf("  ✅ Updated lift.yaml\n")

	// 3. Update cdk/main.go
	isPT := c.isPTProject(cfg)
	if err := c.updateCDK(root, name, cfg.App.Name, isPT); err != nil {
		return fmt.Errorf("failed to update cdk/main.go: %w", err)
	}
	stdio.Stdoutf("  ✅ Updated cdk/main.go\n")

	// 4. For PT projects, update build files
	if isPT {
		if err := c.updatePTBuildFiles(root, name); err != nil {
			return fmt.Errorf("failed to update PT build files: %w", err)
		}
		stdio.Stdoutf("  ✅ Updated buildspec.yml\n")
		stdio.Stdoutf("  ✅ Updated shell/build.sh\n")
	}

	stdio.Stdoutf("\n✅ Function %q added successfully!\n", name)
	stdio.Stdoutf("\n📝 Next steps:\n")
	stdio.Stdoutf("  1. Implement your handler in cmd/%s/main.go\n", name)
	stdio.Stdoutf("  2. Run 'lift build' to compile the new function\n")
	stdio.Stdoutf("  3. Run 'lift up --stage dev' to deploy\n")

	return nil
}

// validateFunctionName ensures the function name is safe and valid
func validateFunctionName(name string) error {
	if name == "" {
		return fmt.Errorf("function name cannot be empty")
	}

	// Check for path traversal
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return fmt.Errorf("function name cannot contain path separators or '..'")
	}

	// Must be a valid Go identifier (alphanumeric + underscore, not starting with number)
	validName := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
	if !validName.MatchString(name) {
		return fmt.Errorf("function name must be a valid identifier (letters, numbers, underscores; cannot start with a number)")
	}

	// Reject reserved names
	reserved := map[string]bool{"main": true, "init": true, "test": true}
	if reserved[strings.ToLower(name)] {
		return fmt.Errorf("function name %q is reserved", name)
	}

	return nil
}

// createFunctionCode creates the cmd/<name>/main.go file
func (c *AddCommand) createFunctionCode(root, name, appName string) error {
	cmdDir := filepath.Join(root, "cmd", name)
	if err := os.MkdirAll(cmdDir, 0750); err != nil {
		return err
	}

	mainContent := fmt.Sprintf(`package main

import (
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/pay-theory/lift/pkg/lift"
)

func main() {
	// Create Lift app
	app := lift.New()

	// Health endpoint
	app.GET("/health", func(ctx *lift.Context) error {
		return ctx.JSON(map[string]string{
			"status":  "healthy",
			"service": "%s-%s",
		})
	})

	// TODO: Add your handlers here
	app.GET("/", func(ctx *lift.Context) error {
		return ctx.JSON(map[string]string{
			"message": "Hello from %s!",
		})
	})

	// Run local test if not in Lambda
	app.RunLocalTest()

	// Start Lambda
	lambda.Start(app.HandleRequest)
}
	`, appName, name, name)

	mainPath := filepath.Join(cmdDir, "main.go")
	return os.WriteFile(mainPath, []byte(mainContent), 0600)
}

// updateLiftYAML adds the new function to lift.yaml using YAML node editing
func (c *AddCommand) updateLiftYAML(root, name string) error {
	configPath := filepath.Join(root, "lift.yaml")

	data, err := os.ReadFile(configPath) //nolint:gosec // file path is derived from the discovered project root
	if err != nil {
		return err
	}

	var doc yaml.Node
	if unmarshalErr := yaml.Unmarshal(data, &doc); unmarshalErr != nil {
		return unmarshalErr
	}

	// The document should have a single document node
	if len(doc.Content) == 0 {
		return fmt.Errorf("empty lift.yaml")
	}

	rootNode := doc.Content[0]
	if rootNode.Kind != yaml.MappingNode {
		return fmt.Errorf("lift.yaml root is not a mapping")
	}

	// Find or create the functions node
	var functionsNode *yaml.Node
	for i := 0; i < len(rootNode.Content); i += 2 {
		if rootNode.Content[i].Value == "functions" {
			functionsNode = rootNode.Content[i+1]
			break
		}
	}

	if functionsNode == nil {
		// Create functions section
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: "functions"}
		functionsNode = &yaml.Node{Kind: yaml.MappingNode}
		rootNode.Content = append(rootNode.Content, keyNode, functionsNode)
	}

	// Add the new function
	// Create the function entry: name -> {cmd: ./cmd/<name>, out: ./dist/<name>/bootstrap}
	nameNode := &yaml.Node{Kind: yaml.ScalarNode, Value: name}

	funcConfigNode := &yaml.Node{Kind: yaml.MappingNode}
	funcConfigNode.Content = append(funcConfigNode.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "cmd"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("./cmd/%s", name)},
		&yaml.Node{Kind: yaml.ScalarNode, Value: "out"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("./dist/%s/bootstrap", name)},
	)

	functionsNode.Content = append(functionsNode.Content, nameNode, funcConfigNode)

	// Write back
	output, err := yaml.Marshal(&doc)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, output, 0600)
}

// isPTProject detects if this is a PT-mode project
func (c *AddCommand) isPTProject(cfg *liftconfig.Config) bool {
	// Check if stack name templates reference {{.Partner}}
	if cfg.CDK != nil && cfg.CDK.Stacks != nil {
		for _, stack := range cfg.CDK.Stacks {
			if stack != nil && strings.Contains(stack.NameTemplate, ".Partner") {
				return true
			}
		}
	}
	return false
}

// detectStackVariable examines the CDK file content to find the stack variable
// used in existing liftcdk.NewLiftFunction calls. This correctly distinguishes
// between "serviceStack" (actual stack) and "serviceStackName" (just a string).
// Falls back to "stack" if no existing function calls are found.
func detectStackVariable(content string) string {
	// Look for existing liftcdk.NewLiftFunction(<stackVar>, ...) calls
	// Pattern: liftcdk.NewLiftFunction(stackVar, jsii.String(...)
	liftFunctionPattern := regexp.MustCompile(`liftcdk\.NewLiftFunction\((\w+),`)
	matches := liftFunctionPattern.FindStringSubmatch(content)
	if len(matches) > 1 {
		return matches[1]
	}

	// Fallback: look for awscdk.NewCfnOutput(<stackVar>, ...) calls
	cfnOutputPattern := regexp.MustCompile(`awscdk\.NewCfnOutput\((\w+),`)
	matches = cfnOutputPattern.FindStringSubmatch(content)
	if len(matches) > 1 {
		return matches[1]
	}

	// Default to "stack" if we can't detect
	return "stack"
}

// updateCDK modifies cdk/main.go to include the new function
func (c *AddCommand) updateCDK(root, name, appName string, isPT bool) error {
	cdkPath := filepath.Join(root, "cdk", "main.go")

	data, err := os.ReadFile(cdkPath) //nolint:gosec // file path is derived from the discovered project root
	if err != nil {
		return err
	}

	content := string(data)

	// Detect which stack variable is used in the file by examining existing
	// liftcdk.NewLiftFunction calls. This avoids mis-detection from variables
	// like "serviceStackName" (a string) vs "serviceStack" (the actual stack).
	stackVar := detectStackVariable(content)

	// Generate the function code to insert
	var functionCode string
	if isPT {
		functionCode = c.generatePTFunctionCode(name, appName, stackVar)
	} else {
		functionCode = c.generateFunctionCode(name, appName, stackVar)
	}

	// Try to find markers
	const functionsMarker = "// LIFT:ADD_FUNCTIONS"

	hasFunctionsMarker := strings.Contains(content, functionsMarker)

	if hasFunctionsMarker {
		// Insert at markers
		content = strings.Replace(content, functionsMarker, functionCode+"\n\n\t"+functionsMarker, 1)
	} else {
		// Fallback: insert before app.Synth(nil)
		synthPattern := "app.Synth(nil)"
		insertPoint := strings.LastIndex(content, synthPattern)
		if insertPoint == -1 {
			return fmt.Errorf("could not find insertion point in cdk/main.go (no markers or app.Synth)")
		}

		insertion := functionCode + "\n\n\t"
		content = content[:insertPoint] + insertion + content[insertPoint:]
	}

	// Format the Go code
	formatted, err := format.Source([]byte(content))
	if err != nil {
		// If formatting fails, write unformatted and warn
		stdio.Stdoutf("  ⚠️  Warning: could not format cdk/main.go: %v\n", err)
		return os.WriteFile(cdkPath, []byte(content), 0600)
	}

	return os.WriteFile(cdkPath, formatted, 0600)
}

// generateFunctionCode generates the LiftFunction code for default projects
func (c *AddCommand) generateFunctionCode(name, _, stackVar string) string {
	pascalName := toPascalCase(name)
	return fmt.Sprintf(`// Create the %s Lambda function
	%sFunction := liftcdk.NewLiftFunction(%s, jsii.String("%sFunction"), &liftcdk.LiftFunctionProps{
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String(fmt.Sprintf("%%s-%s-%%s", appNameStr, stageStr)),
			Code:         awslambda.Code_FromAsset(jsii.String(filepath.Join("..", "dist", "%s")), nil),
			Handler:      jsii.String("bootstrap"),
			Description:  jsii.String(fmt.Sprintf("%%s %s handler for %%s", appNameStr, stageStr)),
		},
		EnableTracing: jsii.Bool(true),
		EnableMetrics: jsii.Bool(true),
	})
	_ = %sFunction`, name, toLowerCamel(name), stackVar, pascalName, name, name, name, toLowerCamel(name))
}

// generatePTFunctionCode generates the LiftFunction code for PT projects
func (c *AddCommand) generatePTFunctionCode(name, _, stackVar string) string {
	pascalName := toPascalCase(name)
	return fmt.Sprintf(`// Create the %s Lambda function
	%sFunction := liftcdk.NewLiftFunction(%s, jsii.String("%sFunction"), &liftcdk.LiftFunctionProps{
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String(fmt.Sprintf("%%s-%s-%%s-%%s", appNameStr, partnerStr, stageStr)),
			Code:         awslambda.Code_FromAsset(jsii.String(filepath.Join("..", "dist", "%s")), nil),
			Handler:      jsii.String("bootstrap"),
			Description:  jsii.String(fmt.Sprintf("%%s %s handler for partner %%s, stage %%s", appNameStr, partnerStr, stageStr)),
			Environment: &map[string]*string{
				"PARTNER":     jsii.String(partnerStr),
				"STAGE":       jsii.String(stageStr),
				"TARGET_MODE": jsii.String(targetModeStr),
			},
		},
		EnableTracing: jsii.Bool(true),
		EnableMetrics: jsii.Bool(true),
	})
		_ = %sFunction`, name, toLowerCamel(name), stackVar, pascalName, name, name, name, toLowerCamel(name))
}

// updatePTBuildFiles updates buildspec.yml and shell/build.sh for PT projects
func (c *AddCommand) updatePTBuildFiles(root, name string) error {
	// Update buildspec.yml
	if err := c.updateBuildspecYML(root, name); err != nil {
		return fmt.Errorf("buildspec.yml: %w", err)
	}

	// Update shell/build.sh
	if err := c.updateBuildSH(root, name); err != nil {
		return fmt.Errorf("shell/build.sh: %w", err)
	}

	return nil
}

// updateBuildspecYML adds build steps for the new function
func (c *AddCommand) updateBuildspecYML(root, name string) error {
	buildspecPath := filepath.Join(root, "buildspec.yml")

	data, err := os.ReadFile(buildspecPath) //nolint:gosec // file path is derived from the discovered project root
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No buildspec.yml, skip
		}
		return err
	}

	content := string(data)
	const markerLine = "# LIFT:ADD_FUNCTION_BUILDS"

	insertBuildSteps := func(indent string, insertAt int) {
		buildSteps := fmt.Sprintf(`%s- mkdir -p dist/%s
%s- echo "Building %s..."
%s- GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -mod=mod -ldflags="-s -w" -tags lambda.norpc -trimpath -o dist/%s/bootstrap ./cmd/%s
`,
			indent, name,
			indent, name,
			indent, name, name,
		)
		content = content[:insertAt] + buildSteps + content[insertAt:]
	}

	// Preferred: insert at the marker line, preserving indentation so YAML stays valid.
	markerRe := regexp.MustCompile(`(?m)^(\s*)` + regexp.QuoteMeta(markerLine) + `\s*$`)
	if loc := markerRe.FindStringSubmatchIndex(content); loc != nil {
		indent := content[loc[2]:loc[3]]
		insertBuildSteps(indent, loc[0])
		return os.WriteFile(buildspecPath, []byte(content), 0600)
	}

	// Fallback: insert before the synth step, attempting to infer indentation.
	synthRe := regexp.MustCompile(`(?m)^(\s*)- echo "Synthesizing CDK stacks\.\.\."\s*$`)
	if loc := synthRe.FindStringSubmatchIndex(content); loc != nil {
		indent := content[loc[2]:loc[3]]
		insertBuildSteps(indent, loc[0])
	}

	return os.WriteFile(buildspecPath, []byte(content), 0600)
}

// updateBuildSH adds build steps for the new function
func (c *AddCommand) updateBuildSH(root, name string) error {
	buildshPath := filepath.Join(root, "shell", "build.sh")

	data, err := os.ReadFile(buildshPath) //nolint:gosec // file path is derived from the discovered project root
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No shell/build.sh, skip
		}
		return err
	}

	content := string(data)
	const functionMarker = "# LIFT:ADD_FUNCTION_BUILDS"
	const outputMarker = "# LIFT:ADD_OUTPUT_LINES"

	buildStep := fmt.Sprintf(`# Build %s Lambda
mkdir -p dist/%s
print_status "Building %s..."
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build \
    -mod=mod \
    -ldflags="-s -w" \
    -tags lambda.norpc \
    -trimpath \
    -o dist/%s/bootstrap \
    ./cmd/%s
if [ $? -eq 0 ]; then
    print_success "%s build successful!"
else
    print_error "%s build failed!"
    exit 1
fi
`, name, name, name, name, name, name, name)

	outputLine := fmt.Sprintf(`echo "   - dist/%s/bootstrap"`, name)

	content = strings.Replace(content, functionMarker, buildStep+"\n"+functionMarker, 1)
	content = strings.Replace(content, outputMarker, outputLine+"\n"+outputMarker, 1)

	mode := os.FileMode(0750)
	if info, statErr := os.Stat(buildshPath); statErr == nil {
		mode = info.Mode()
	}
	return os.WriteFile(buildshPath, []byte(content), mode)
}

// toPascalCase converts a snake_case or lowercase name to PascalCase
func toPascalCase(s string) string {
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}

// toLowerCamel converts a name to lowerCamelCase
func toLowerCamel(s string) string {
	pascal := toPascalCase(s)
	if len(pascal) == 0 {
		return pascal
	}
	return strings.ToLower(pascal[:1]) + pascal[1:]
}
