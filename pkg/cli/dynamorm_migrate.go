package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	stringType          = "string"
	mediumSize          = "Medium"
	keysOnlyView        = "KEYS_ONLY"
	newAndOldImagesView = "NEW_AND_OLD_IMAGES"
	allView             = "ALL"
)

type dynamodbClient interface {
	DescribeTable(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error)
	Scan(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
}

// DynamORMMigrateCommand handles migration from existing DynamoDB tables to DynamORM
type DynamORMMigrateCommand struct {
	awsConfigFunc     func(ctx context.Context, region string) (aws.Config, error)
	newDynamoDBClient func(cfg aws.Config) dynamodbClient
}

// DynamORMMigrateCommand methods
func (c *DynamORMMigrateCommand) Name() string { return "migrate" }
func (c *DynamORMMigrateCommand) Description() string {
	return "Migrate existing DynamoDB tables to DynamORM"
}
func (c *DynamORMMigrateCommand) Usage() string {
	return "lift dynamorm migrate --table <table-name> [--region <region>] [--output-dir <dir>] [--analyze-only]"
}

// TableAnalysis contains the analysis results of a DynamoDB table
type TableAnalysis struct {
	CreatedAt              time.Time                `json:"created_at"`
	SortKey                *AttributeSpec           `json:"sort_key,omitempty"`
	TimeToLiveSpec         *TTLSpec                 `json:"time_to_live,omitempty"`
	StreamSpec             *StreamSpec              `json:"stream,omitempty"`
	Attributes             map[string]AttributeSpec `json:"attributes"`
	RecommendedModel       string                   `json:"recommended_model"`
	TableName              string                   `json:"table_name"`
	BillingMode            string                   `json:"billing_mode"`
	MigrationComplexity    string                   `json:"migration_complexity"`
	PartitionKey           AttributeSpec            `json:"partition_key"`
	LocalSecondaryIndexes  []LSIAnalysis            `json:"local_secondary_indexes,omitempty"`
	SampleItems            []map[string]interface{} `json:"sample_items,omitempty"`
	Warnings               []string                 `json:"warnings,omitempty"`
	GlobalSecondaryIndexes []GSIAnalysis            `json:"global_secondary_indexes,omitempty"`
	TableSizeBytes         int64                    `json:"table_size_bytes"`
	ItemCount              int64                    `json:"item_count"`
	MultiTenantCandidate   bool                     `json:"multi_tenant_candidate"`
}

// AttributeSpec defines an attribute specification
type AttributeSpec struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

// GSIAnalysis contains GSI analysis
type GSIAnalysis struct {
	SortKey        *AttributeSpec           `json:"sort_key,omitempty"`
	IndexName      string                   `json:"index_name"`
	ProjectionType string                   `json:"projection_type"`
	PartitionKey   AttributeSpec            `json:"partition_key"`
	KeySchema      []types.KeySchemaElement `json:"key_schema"`
	ItemCount      int64                    `json:"item_count"`
}

// LSIAnalysis contains LSI analysis
type LSIAnalysis struct {
	IndexName      string        `json:"index_name"`
	ProjectionType string        `json:"projection_type"`
	SortKey        AttributeSpec `json:"sort_key"`
	ItemCount      int64         `json:"item_count"`
}

// TTLSpec defines TTL specification
type TTLSpec struct {
	AttributeName string `json:"attribute_name"`
	Enabled       bool   `json:"enabled"`
}

// StreamSpec defines stream specification
type StreamSpec struct {
	ViewType  string `json:"view_type"`
	StreamArn string `json:"stream_arn"`
	Enabled   bool   `json:"enabled"`
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

func (c *DynamORMMigrateCommand) analyzeTable(ctx context.Context, client dynamodbClient, tableName string) (*TableAnalysis, error) {
	builder := newTableAnalysisBuilder(ctx, c, client, tableName)
	return builder.build()
}

// tableAnalysisBuilder builds table analysis
// Memory optimized: struct with 64 pointer bytes could be 56
type tableAnalysisBuilder struct {
	// Pointers first (8 bytes each)
	cmd      *DynamORMMigrateCommand
	client   dynamodbClient
	table    *types.TableDescription
	analysis *TableAnalysis
	// Interface (16 bytes)
	ctx context.Context
	// String (16 bytes)
	tableName string
}

// newTableAnalysisBuilder creates a new table analysis builder
func newTableAnalysisBuilder(ctx context.Context, cmd *DynamORMMigrateCommand, client dynamodbClient, tableName string) *tableAnalysisBuilder {
	return &tableAnalysisBuilder{
		cmd:       cmd,
		ctx:       ctx,
		client:    client,
		tableName: tableName,
	}
}

// build constructs the complete table analysis
func (tab *tableAnalysisBuilder) build() (*TableAnalysis, error) {
	// Get table description
	if err := tab.describeTable(); err != nil {
		return nil, err
	}

	// Initialize analysis
	tab.initializeAnalysis()

	// Analyze various aspects
	tab.analyzeKeySchema()
	tab.analyzeGlobalSecondaryIndexes()
	tab.analyzeLocalSecondaryIndexes()
	tab.analyzeTTL()
	tab.analyzeStreams()

	// Sample and finalize
	tab.sampleTableItems()
	tab.finalizeAnalysis()

	return tab.analysis, nil
}

// describeTable gets table description from DynamoDB
func (tab *tableAnalysisBuilder) describeTable() error {
	describeInput := &dynamodb.DescribeTableInput{
		TableName: aws.String(tab.tableName),
	}

	describeOutput, err := tab.client.DescribeTable(tab.ctx, describeInput)
	if err != nil {
		return fmt.Errorf("failed to describe table: %w", err)
	}

	tab.table = describeOutput.Table
	return nil
}

// initializeAnalysis creates initial analysis structure
func (tab *tableAnalysisBuilder) initializeAnalysis() {
	tab.analysis = &TableAnalysis{
		TableName:      tab.tableName,
		BillingMode:    string(tab.table.BillingModeSummary.BillingMode),
		ItemCount:      *tab.table.ItemCount,
		TableSizeBytes: *tab.table.TableSizeBytes,
		Attributes:     make(map[string]AttributeSpec),
		CreatedAt:      time.Now(),
	}
}

// analyzeKeySchema analyzes primary key schema
func (tab *tableAnalysisBuilder) analyzeKeySchema() {
	keyAnalyzer := newKeySchemaAnalyzer(tab.cmd, tab.table.AttributeDefinitions)

	for _, key := range tab.table.KeySchema {
		spec := keyAnalyzer.analyzeKey(key)
		if spec == nil {
			continue
		}

		switch key.KeyType {
		case types.KeyTypeHash:
			tab.analysis.PartitionKey = *spec
		case types.KeyTypeRange:
			tab.analysis.SortKey = spec
		}
	}
}

// analyzeGlobalSecondaryIndexes analyzes all GSIs
func (tab *tableAnalysisBuilder) analyzeGlobalSecondaryIndexes() {
	gsiAnalyzer := newGSIAnalyzer(tab.cmd, tab.table.AttributeDefinitions)

	for _, gsi := range tab.table.GlobalSecondaryIndexes {
		gsiAnalysis := gsiAnalyzer.analyzeGSI(gsi)
		tab.analysis.GlobalSecondaryIndexes = append(tab.analysis.GlobalSecondaryIndexes, gsiAnalysis)
	}
}

// analyzeLocalSecondaryIndexes analyzes all LSIs
func (tab *tableAnalysisBuilder) analyzeLocalSecondaryIndexes() {
	lsiAnalyzer := newLSIAnalyzer(tab.cmd, tab.table.AttributeDefinitions)

	for _, lsi := range tab.table.LocalSecondaryIndexes {
		lsiAnalysis := lsiAnalyzer.analyzeLSI(lsi)
		tab.analysis.LocalSecondaryIndexes = append(tab.analysis.LocalSecondaryIndexes, lsiAnalysis)
	}
}

// analyzeTTL analyzes TTL configuration
func (tab *tableAnalysisBuilder) analyzeTTL() {
	ttlAnalyzer := newTTLAnalyzer()
	tab.analysis.TimeToLiveSpec = ttlAnalyzer.analyzeTTL(tab.table.AttributeDefinitions)
}

// analyzeStreams analyzes DynamoDB streams configuration
func (tab *tableAnalysisBuilder) analyzeStreams() {
	if tab.table.StreamSpecification != nil &&
		tab.table.StreamSpecification.StreamEnabled != nil &&
		*tab.table.StreamSpecification.StreamEnabled {
		tab.analysis.StreamSpec = &StreamSpec{
			Enabled:   true,
			ViewType:  string(tab.table.StreamSpecification.StreamViewType),
			StreamArn: *tab.table.LatestStreamArn,
		}
	}
}

// sampleTableItems samples items for better analysis
func (tab *tableAnalysisBuilder) sampleTableItems() {
	if err := tab.cmd.sampleItems(tab.ctx, tab.client, tab.tableName, tab.analysis); err != nil {
		tab.analysis.Warnings = append(tab.analysis.Warnings, fmt.Sprintf("Failed to sample items: %v", err))
	}
}

// finalizeAnalysis determines complexity and generates recommendations
func (tab *tableAnalysisBuilder) finalizeAnalysis() {
	tab.analysis.MigrationComplexity = tab.cmd.determineMigrationComplexity(tab.analysis)
	tab.analysis.MultiTenantCandidate = tab.cmd.isMultiTenantCandidate(tab.analysis)
	tab.analysis.RecommendedModel = tab.cmd.generateRecommendedModel(tab.analysis)
}

// keySchemaAnalyzer analyzes key schemas
type keySchemaAnalyzer struct {
	cmd        *DynamORMMigrateCommand
	attributes []types.AttributeDefinition
}

// newKeySchemaAnalyzer creates a new key schema analyzer
func newKeySchemaAnalyzer(cmd *DynamORMMigrateCommand, attributes []types.AttributeDefinition) *keySchemaAnalyzer {
	return &keySchemaAnalyzer{
		cmd:        cmd,
		attributes: attributes,
	}
}

// analyzeKey analyzes a single key element
func (ksa *keySchemaAnalyzer) analyzeKey(key types.KeySchemaElement) *AttributeSpec {
	attr := ksa.cmd.findAttribute(ksa.attributes, *key.AttributeName)
	if attr == nil {
		return nil
	}

	return &AttributeSpec{
		Name:     *key.AttributeName,
		Type:     ksa.cmd.convertDynamoType(attr.AttributeType),
		Required: true,
	}
}

// gsiAnalyzer analyzes global secondary indexes
type gsiAnalyzer struct {
	cmd        *DynamORMMigrateCommand
	attributes []types.AttributeDefinition
}

// newGSIAnalyzer creates a new GSI analyzer
func newGSIAnalyzer(cmd *DynamORMMigrateCommand, attributes []types.AttributeDefinition) *gsiAnalyzer {
	return &gsiAnalyzer{
		cmd:        cmd,
		attributes: attributes,
	}
}

// analyzeGSI analyzes a single GSI
func (ga *gsiAnalyzer) analyzeGSI(gsi types.GlobalSecondaryIndexDescription) GSIAnalysis {
	gsiAnalysis := GSIAnalysis{
		IndexName:      *gsi.IndexName,
		ProjectionType: string(gsi.Projection.ProjectionType),
		ItemCount:      *gsi.ItemCount,
		KeySchema:      gsi.KeySchema,
	}

	keyAnalyzer := newKeySchemaAnalyzer(ga.cmd, ga.attributes)

	for _, key := range gsi.KeySchema {
		spec := keyAnalyzer.analyzeKey(key)
		if spec == nil {
			continue
		}

		switch key.KeyType {
		case types.KeyTypeHash:
			gsiAnalysis.PartitionKey = *spec
		case types.KeyTypeRange:
			gsiAnalysis.SortKey = spec
		}
	}

	return gsiAnalysis
}

// lsiAnalyzer analyzes local secondary indexes
type lsiAnalyzer struct {
	cmd        *DynamORMMigrateCommand
	attributes []types.AttributeDefinition
}

// newLSIAnalyzer creates a new LSI analyzer
func newLSIAnalyzer(cmd *DynamORMMigrateCommand, attributes []types.AttributeDefinition) *lsiAnalyzer {
	return &lsiAnalyzer{
		cmd:        cmd,
		attributes: attributes,
	}
}

// analyzeLSI analyzes a single LSI
func (la *lsiAnalyzer) analyzeLSI(lsi types.LocalSecondaryIndexDescription) LSIAnalysis {
	lsiAnalysis := LSIAnalysis{
		IndexName:      *lsi.IndexName,
		ProjectionType: string(lsi.Projection.ProjectionType),
		ItemCount:      *lsi.ItemCount,
	}

	for _, key := range lsi.KeySchema {
		if key.KeyType == types.KeyTypeRange {
			attr := la.cmd.findAttribute(la.attributes, *key.AttributeName)
			if attr != nil {
				lsiAnalysis.SortKey = AttributeSpec{
					Name:     *key.AttributeName,
					Type:     la.cmd.convertDynamoType(attr.AttributeType),
					Required: true,
				}
			}
		}
	}

	return lsiAnalysis
}

// ttlAnalyzer analyzes TTL configuration
type ttlAnalyzer struct{}

// newTTLAnalyzer creates a new TTL analyzer
func newTTLAnalyzer() *ttlAnalyzer {
	return &ttlAnalyzer{}
}

// analyzeTTL checks for TTL attributes
func (ta *ttlAnalyzer) analyzeTTL(attributes []types.AttributeDefinition) *TTLSpec {
	for _, attr := range attributes {
		attrName := *attr.AttributeName
		if ta.isTTLAttribute(attrName) {
			return &TTLSpec{
				AttributeName: attrName,
				Enabled:       false, // Would need separate API call to confirm
			}
		}
	}
	return nil
}

// isTTLAttribute checks if attribute name suggests TTL usage
func (ta *ttlAnalyzer) isTTLAttribute(attrName string) bool {
	lowerName := strings.ToLower(attrName)
	return strings.HasSuffix(lowerName, "ttl") ||
		strings.HasSuffix(lowerName, "expires") ||
		strings.HasSuffix(lowerName, "expiry")
}

func (c *DynamORMMigrateCommand) findAttribute(attrs []types.AttributeDefinition, name string) *types.AttributeDefinition {
	for _, attr := range attrs {
		if *attr.AttributeName == name {
			return &attr
		}
	}
	return nil
}

func (c *DynamORMMigrateCommand) convertDynamoType(attrType types.ScalarAttributeType) string {
	switch attrType {
	case types.ScalarAttributeTypeS:
		return stringType
	case types.ScalarAttributeTypeN:
		return "number"
	case types.ScalarAttributeTypeB:
		return "binary"
	default:
		return stringType
	}
}

func (c *DynamORMMigrateCommand) sampleItems(ctx context.Context, client dynamodbClient, tableName string, analysis *TableAnalysis) error {
	// Scan a few items to understand the structure
	scanInput := &dynamodb.ScanInput{
		TableName: aws.String(tableName),
		Limit:     aws.Int32(5),
	}

	scanOutput, err := client.Scan(ctx, scanInput)
	if err != nil {
		return err
	}

	// Convert items to map for analysis
	for _, item := range scanOutput.Items {
		itemMap := make(map[string]interface{})
		for k, v := range item {
			itemMap[k] = c.convertAttributeValue(v)
		}
		analysis.SampleItems = append(analysis.SampleItems, itemMap)
	}

	return nil
}

func (c *DynamORMMigrateCommand) convertAttributeValue(av types.AttributeValue) interface{} {
	switch v := av.(type) {
	case *types.AttributeValueMemberS:
		return v.Value
	case *types.AttributeValueMemberN:
		return v.Value
	case *types.AttributeValueMemberB:
		return v.Value
	case *types.AttributeValueMemberBOOL:
		return v.Value
	case *types.AttributeValueMemberNULL:
		return nil
	default:
		return "complex_type"
	}
}

func (c *DynamORMMigrateCommand) determineMigrationComplexity(analysis *TableAnalysis) string {
	complexity := "Simple"

	if len(analysis.GlobalSecondaryIndexes) > 2 {
		complexity = mediumSize
	}

	if len(analysis.LocalSecondaryIndexes) > 0 {
		complexity = mediumSize
	}

	if analysis.StreamSpec != nil && analysis.StreamSpec.Enabled {
		complexity = mediumSize
	}

	if len(analysis.SampleItems) > 0 {
		// Check for complex nested structures
		for _, item := range analysis.SampleItems {
			if c.hasComplexStructure(item) {
				complexity = "Complex"
				break
			}
		}
	}

	return complexity
}

func (c *DynamORMMigrateCommand) hasComplexStructure(item map[string]interface{}) bool {
	for _, value := range item {
		switch v := value.(type) {
		case map[string]interface{}:
			return true
		case []interface{}:
			return true
		case string:
			if v == "complex_type" {
				return true
			}
		}
	}
	return false
}

func (c *DynamORMMigrateCommand) generateRecommendedModel(analysis *TableAnalysis) string {
	modelName := c.generateModelName(analysis.TableName)
	if analysis.MultiTenantCandidate {
		return modelName + " (Multi-Tenant)"
	}
	return modelName
}

func (c *DynamORMMigrateCommand) isMultiTenantCandidate(analysis *TableAnalysis) bool {
	// Check if partition key suggests multi-tenancy
	pkName := strings.ToLower(analysis.PartitionKey.Name)
	tenantIndicators := []string{"tenant", "org", "account", "customer", "company"}

	for _, indicator := range tenantIndicators {
		if strings.Contains(pkName, indicator) {
			return true
		}
	}

	// Check GSI names
	for _, gsi := range analysis.GlobalSecondaryIndexes {
		gsiName := strings.ToLower(gsi.IndexName)
		for _, indicator := range tenantIndicators {
			if strings.Contains(gsiName, indicator) {
				return true
			}
		}
	}

	return false
}

func (c *DynamORMMigrateCommand) saveAnalysis(analysis *TableAnalysis, path string) error {
	data, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

func (c *DynamORMMigrateCommand) printAnalysisSummary(analysis *TableAnalysis) {
	fmt.Printf("\n📋 Analysis Summary for %s:\n", analysis.TableName)
	fmt.Printf("   • Partition Key: %s (%s)\n", analysis.PartitionKey.Name, analysis.PartitionKey.Type)

	if analysis.SortKey != nil {
		fmt.Printf("   • Sort Key: %s (%s)\n", analysis.SortKey.Name, analysis.SortKey.Type)
	}

	fmt.Printf("   • GSIs: %d\n", len(analysis.GlobalSecondaryIndexes))
	fmt.Printf("   • LSIs: %d\n", len(analysis.LocalSecondaryIndexes))
	fmt.Printf("   • Item Count: %d\n", analysis.ItemCount)
	fmt.Printf("   • Table Size: %.2f MB\n", float64(analysis.TableSizeBytes)/1024/1024)
	fmt.Printf("   • Billing Mode: %s\n", analysis.BillingMode)
	fmt.Printf("   • Migration Complexity: %s\n", analysis.MigrationComplexity)
	fmt.Printf("   • Recommended Model: %s\n", analysis.RecommendedModel)
	fmt.Printf("   • Multi-Tenant Candidate: %t\n", analysis.MultiTenantCandidate)

	if analysis.TimeToLiveSpec != nil {
		fmt.Printf("   • TTL: %s\n", analysis.TimeToLiveSpec.AttributeName)
	}

	if analysis.StreamSpec != nil {
		fmt.Printf("   • Stream: %s\n", analysis.StreamSpec.ViewType)
	}

	if len(analysis.Warnings) > 0 {
		fmt.Printf("   ⚠️  Warnings:\n")
		for _, warning := range analysis.Warnings {
			fmt.Printf("     - %s\n", warning)
		}
	}
}

func (c *DynamORMMigrateCommand) generateMigrationCode(analysis *TableAnalysis, config *MigrationConfig) error {
	// Generate model
	if err := c.generateMigrationModel(analysis, config); err != nil {
		return fmt.Errorf("failed to generate model: %w", err)
	}

	// Generate CDK construct
	if err := c.generateMigrationCDK(analysis, config); err != nil {
		return fmt.Errorf("failed to generate CDK: %w", err)
	}

	// Generate migration script
	if err := c.generateMigrationScript(analysis, config); err != nil {
		return fmt.Errorf("failed to generate migration script: %w", err)
	}

	// Generate tests if requested
	if config.GenerateTests {
		if err := c.generateMigrationTests(analysis, config); err != nil {
			return fmt.Errorf("failed to generate tests: %w", err)
		}
	}

	return nil
}
