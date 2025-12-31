package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/require"
)

type fakeDynamoDBClient struct {
	describeOut *dynamodb.DescribeTableOutput
	describeErr error

	scanOut *dynamodb.ScanOutput
	scanErr error
}

func (f *fakeDynamoDBClient) DescribeTable(_ context.Context, _ *dynamodb.DescribeTableInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	if f.describeErr != nil {
		return nil, f.describeErr
	}
	return f.describeOut, nil
}

func (f *fakeDynamoDBClient) Scan(_ context.Context, _ *dynamodb.ScanInput, _ ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
	if f.scanErr != nil {
		return nil, f.scanErr
	}
	return f.scanOut, nil
}

func TestDynamORMMigrateCommand_AnalyzeTable_BuilderHappyPath(t *testing.T) {
	cmd := &DynamORMMigrateCommand{}

	itemCount := int64(42)
	tableSize := int64(1024)
	streamArn := "arn:stream"
	streamEnabled := true

	table := &types.TableDescription{
		BillingModeSummary: &types.BillingModeSummary{BillingMode: types.BillingModePayPerRequest},
		ItemCount:          &itemCount,
		TableSizeBytes:     &tableSize,
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: strPtr("tenant_id"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: strPtr("id"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: strPtr("ttl"), AttributeType: types.ScalarAttributeTypeN},
			{AttributeName: strPtr("gsi_pk"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: strPtr("gsi_sk"), AttributeType: types.ScalarAttributeTypeN},
			{AttributeName: strPtr("lsi_sk"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: strPtr("tenant_id"), KeyType: types.KeyTypeHash},
			{AttributeName: strPtr("id"), KeyType: types.KeyTypeRange},
		},
		GlobalSecondaryIndexes: []types.GlobalSecondaryIndexDescription{
			{
				IndexName:  strPtr("gsi-1"),
				ItemCount:  int64Ptr(10),
				Projection: &types.Projection{ProjectionType: types.ProjectionTypeKeysOnly},
				KeySchema: []types.KeySchemaElement{
					{AttributeName: strPtr("gsi_pk"), KeyType: types.KeyTypeHash},
					{AttributeName: strPtr("gsi_sk"), KeyType: types.KeyTypeRange},
				},
			},
		},
		LocalSecondaryIndexes: []types.LocalSecondaryIndexDescription{
			{
				IndexName:  strPtr("lsi-1"),
				ItemCount:  int64Ptr(1),
				Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
				KeySchema: []types.KeySchemaElement{
					{AttributeName: strPtr("tenant_id"), KeyType: types.KeyTypeHash},
					{AttributeName: strPtr("lsi_sk"), KeyType: types.KeyTypeRange},
				},
			},
		},
		StreamSpecification: &types.StreamSpecification{
			StreamEnabled:  &streamEnabled,
			StreamViewType: types.StreamViewTypeNewAndOldImages,
		},
		LatestStreamArn: &streamArn,
	}

	client := &fakeDynamoDBClient{
		describeOut: &dynamodb.DescribeTableOutput{Table: table},
		scanOut: &dynamodb.ScanOutput{
			Items: []map[string]types.AttributeValue{
				{
					"tenant_id": &types.AttributeValueMemberS{Value: "t1"},
					"id":        &types.AttributeValueMemberS{Value: "1"},
					"flag":      &types.AttributeValueMemberBOOL{Value: true},
				},
			},
		},
	}

	analysis, err := cmd.analyzeTable(context.Background(), client, "lift-orders-table")
	require.NoError(t, err)

	require.Equal(t, "lift-orders-table", analysis.TableName)
	require.Equal(t, "PAY_PER_REQUEST", analysis.BillingMode)
	require.Equal(t, int64(42), analysis.ItemCount)
	require.Equal(t, int64(1024), analysis.TableSizeBytes)

	require.Equal(t, "tenant_id", analysis.PartitionKey.Name)
	require.Equal(t, "string", analysis.PartitionKey.Type)
	require.NotNil(t, analysis.SortKey)
	require.Equal(t, "id", analysis.SortKey.Name)

	require.Len(t, analysis.GlobalSecondaryIndexes, 1)
	require.Equal(t, "gsi-1", analysis.GlobalSecondaryIndexes[0].IndexName)
	require.NotNil(t, analysis.GlobalSecondaryIndexes[0].SortKey)
	require.Equal(t, "number", analysis.GlobalSecondaryIndexes[0].SortKey.Type)

	require.Len(t, analysis.LocalSecondaryIndexes, 1)
	require.Equal(t, "lsi-1", analysis.LocalSecondaryIndexes[0].IndexName)

	require.NotNil(t, analysis.TimeToLiveSpec)
	require.Equal(t, "ttl", analysis.TimeToLiveSpec.AttributeName)

	require.NotNil(t, analysis.StreamSpec)
	require.True(t, analysis.StreamSpec.Enabled)
	require.Equal(t, "NEW_AND_OLD_IMAGES", analysis.StreamSpec.ViewType)
	require.Equal(t, "arn:stream", analysis.StreamSpec.StreamArn)

	require.Len(t, analysis.SampleItems, 1)
	require.Equal(t, true, analysis.SampleItems[0]["flag"])

	require.Equal(t, mediumSize, analysis.MigrationComplexity)
	require.True(t, analysis.MultiTenantCandidate)
	require.Equal(t, "Order (Multi-Tenant)", analysis.RecommendedModel)
}

func TestDynamORMMigrateCommand_AnalyzeTable_WarnsOnSamplingError(t *testing.T) {
	cmd := &DynamORMMigrateCommand{}

	itemCount := int64(1)
	tableSize := int64(1)

	table := &types.TableDescription{
		BillingModeSummary: &types.BillingModeSummary{BillingMode: types.BillingModePayPerRequest},
		ItemCount:          &itemCount,
		TableSizeBytes:     &tableSize,
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: strPtr("id"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: strPtr("id"), KeyType: types.KeyTypeHash},
		},
	}

	client := &fakeDynamoDBClient{
		describeOut: &dynamodb.DescribeTableOutput{Table: table},
		scanErr:     errors.New("scan failed"),
	}

	analysis, err := cmd.analyzeTable(context.Background(), client, "lift-simple-table")
	require.NoError(t, err)
	require.NotEmpty(t, analysis.Warnings)
	require.Contains(t, analysis.Warnings[0], "Failed to sample items")
}

func TestDynamORMMigrateCommand_AnalyzeTable_DescribeTableError(t *testing.T) {
	cmd := &DynamORMMigrateCommand{}
	client := &fakeDynamoDBClient{describeErr: errors.New("nope")}

	_, err := cmd.analyzeTable(context.Background(), client, "lift-table")
	require.Error(t, err)
}
