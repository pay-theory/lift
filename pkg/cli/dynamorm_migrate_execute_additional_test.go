package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/require"
)

func TestDynamORMMigrateCommand_Execute_AnalyzeOnly_UsesInjectedAWSAndClient(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	require.NoError(t, os.WriteFile("go.mod", []byte("module example.com/test\n\nrequire github.com/pay-theory/lift v0.0.0\n"), 0600))

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

	fakeClient := &fakeDynamoDBClient{
		describeOut: &dynamodb.DescribeTableOutput{Table: table},
		scanOut:     &dynamodb.ScanOutput{Items: nil},
	}

	cmd := &DynamORMMigrateCommand{
		awsConfigFunc: func(context.Context, string) (aws.Config, error) {
			return aws.Config{}, nil
		},
		newDynamoDBClient: func(aws.Config) dynamodbClient {
			return fakeClient
		},
	}

	require.NoError(t, cmd.Execute(context.Background(), []string{
		"--table", "lift-simple-table",
		"--output-dir", "out",
		"--analyze-only",
	}))

	assertFileExists(t, filepath.Join(tmpDir, "out", "table_analysis.json"))
}

func TestDynamORMMigrateCommand_Execute_GeneratesMigrationCode(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	require.NoError(t, os.WriteFile("go.mod", []byte("module example.com/test\n\nrequire github.com/pay-theory/lift v0.0.0\n"), 0600))

	itemCount := int64(2)
	tableSize := int64(2)
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
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: strPtr("tenant_id"), KeyType: types.KeyTypeHash},
			{AttributeName: strPtr("id"), KeyType: types.KeyTypeRange},
		},
		StreamSpecification: &types.StreamSpecification{
			StreamEnabled:  &streamEnabled,
			StreamViewType: types.StreamViewTypeNewAndOldImages,
		},
		LatestStreamArn: &streamArn,
	}

	fakeClient := &fakeDynamoDBClient{
		describeOut: &dynamodb.DescribeTableOutput{Table: table},
		scanOut:     &dynamodb.ScanOutput{Items: nil},
	}

	cmd := &DynamORMMigrateCommand{
		awsConfigFunc: func(context.Context, string) (aws.Config, error) {
			return aws.Config{}, nil
		},
		newDynamoDBClient: func(aws.Config) dynamodbClient {
			return fakeClient
		},
	}

	require.NoError(t, cmd.Execute(context.Background(), []string{
		"--table", "lift-orders-table",
		"--output-dir", "out",
		"--model", "Order",
		"--multi-tenant",
	}))

	assertFileExists(t, filepath.Join(tmpDir, "out", "table_analysis.json"))
	assertFileExists(t, filepath.Join(tmpDir, "out", "order.go"))
	assertFileExists(t, filepath.Join(tmpDir, "out", "order_table.go"))
	assertFileExists(t, filepath.Join(tmpDir, "out", "migrate_order.go"))
	assertFileExists(t, filepath.Join(tmpDir, "out", "order_test.go"))
}
