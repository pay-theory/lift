package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

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
