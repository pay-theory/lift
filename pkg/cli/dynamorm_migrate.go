package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// DynamORMMigrateCommand handles migration from existing DynamoDB tables to DynamORM
type DynamORMMigrateCommand struct {
	awsConfigFunc     func(ctx context.Context, region string) (aws.Config, error)
	newDynamoDBClient func(cfg aws.Config) dynamodbClient
}

// MigrationConfig holds migration configuration
type MigrationConfig struct {
	TableName     string `json:"table_name"`
	Region        string `json:"region"`
	OutputDir     string `json:"output_dir"`
	ModelName     string `json:"model_name"`
	AnalyzeOnly   bool   `json:"analyze_only"`
	MultiTenant   bool   `json:"multi_tenant"`
	GenerateTests bool   `json:"generate_tests"`
}

func (c *DynamORMMigrateCommand) Name() string { return "migrate" }
func (c *DynamORMMigrateCommand) Description() string {
	return "Migrate existing DynamoDB tables to DynamORM"
}
func (c *DynamORMMigrateCommand) Usage() string {
	return "lift dynamorm migrate --table <table-name> [--region <region>] [--output-dir <dir>] [--analyze-only]"
}

func (c *DynamORMMigrateCommand) Execute(ctx context.Context, args []string) error {
	// Parse arguments
	config, err := c.parseMigrateArgs(args)
	if err != nil {
		return err
	}

	// Check if we're in a Lift project
	if !c.isLiftProject() {
		return fmt.Errorf("not in a Lift project directory - run 'lift new' first")
	}

	// Create AWS session
	awsConfigFunc := c.awsConfigFunc
	if awsConfigFunc == nil {
		awsConfigFunc = awsConfig
	}

	cfg, err := awsConfigFunc(ctx, config.Region)
	if err != nil {
		return fmt.Errorf("failed to create AWS config: %w", err)
	}

	newClient := c.newDynamoDBClient
	if newClient == nil {
		newClient = func(cfg aws.Config) dynamodbClient {
			return dynamodb.NewFromConfig(cfg)
		}
	}

	client := newClient(cfg)

	// Analyze table
	fmt.Printf("🔍 Analyzing table: %s\n", config.TableName)
	analysis, err := c.analyzeTable(ctx, client, config.TableName)
	if err != nil {
		return fmt.Errorf("failed to analyze table: %w", err)
	}

	// Create output directory
	if err := os.MkdirAll(config.OutputDir, 0750); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Save analysis results
	analysisPath := filepath.Join(config.OutputDir, "table_analysis.json")
	if err := c.saveAnalysis(analysis, analysisPath); err != nil {
		return fmt.Errorf("failed to save analysis: %w", err)
	}

	fmt.Printf("📊 Analysis complete! Results saved to: %s\n", analysisPath)
	c.printAnalysisSummary(analysis)

	// If analyze-only mode, stop here
	if config.AnalyzeOnly {
		return nil
	}

	// Generate migration code
	fmt.Printf("🚀 Generating migration code...\n")
	if err := c.generateMigrationCode(analysis, config); err != nil {
		return fmt.Errorf("failed to generate migration code: %w", err)
	}

	fmt.Printf("✅ Migration complete! Generated files in: %s\n", config.OutputDir)
	return nil
}

func (c *DynamORMMigrateCommand) parseMigrateArgs(args []string) (*MigrationConfig, error) {
	config := &MigrationConfig{
		Region:        "us-east-1",
		OutputDir:     "migration",
		GenerateTests: true,
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--table":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--table requires a value")
			}
			config.TableName = args[i+1]
			i++
		case "--region":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--region requires a value")
			}
			config.Region = args[i+1]
			i++
		case "--output-dir":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--output-dir requires a value")
			}
			config.OutputDir = args[i+1]
			i++
		case "--analyze-only":
			config.AnalyzeOnly = true
		case "--model":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--model requires a value")
			}
			config.ModelName = args[i+1]
			i++
		case "--multi-tenant":
			config.MultiTenant = true
		case "--no-tests":
			config.GenerateTests = false
		}
	}

	if config.TableName == "" {
		return nil, fmt.Errorf("--table is required")
	}

	// Generate model name if not provided
	if config.ModelName == "" {
		config.ModelName = c.generateModelName(config.TableName)
	}

	return config, nil
}

func (c *DynamORMMigrateCommand) isLiftProject() bool {
	if _, err := os.Stat("go.mod"); err != nil {
		return false
	}

	content, err := os.ReadFile("go.mod")
	if err != nil {
		return false
	}

	return strings.Contains(string(content), "github.com/pay-theory/lift")
}

func (c *DynamORMMigrateCommand) generateModelName(tableName string) string {
	// Remove common prefixes/suffixes
	name := strings.TrimPrefix(tableName, "lift-")
	name = strings.TrimPrefix(name, "app-")
	name = strings.TrimSuffix(name, "-table")
	name = strings.TrimSuffix(name, "s")

	// Convert to PascalCase
	parts := strings.Split(name, "-")
	var result strings.Builder
	for _, part := range parts {
		if len(part) > 0 {
			result.WriteString(strings.ToUpper(part[:1]) + strings.ToLower(part[1:]))
		}
	}

	return result.String()
}

func awsConfig(ctx context.Context, region string) (aws.Config, error) {
	return config.LoadDefaultConfig(ctx, config.WithRegion(region))
}
