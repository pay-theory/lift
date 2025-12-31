package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/require"
)

func TestDynamORMScaffoldCommand_ParseScaffoldArgs_AndGSIParsing(t *testing.T) {
	cmd := &DynamORMScaffoldCommand{}

	_, err := cmd.parseScaffoldArgs([]string{})
	require.Error(t, err)

	_, err = cmd.parseScaffoldArgs([]string{"--model"})
	require.Error(t, err)

	_, err = cmd.parseScaffoldArgs([]string{"--model", "User", "--table"})
	require.Error(t, err)

	_, err = cmd.parseScaffoldArgs([]string{"--model", "User", "--gsi"})
	require.Error(t, err)

	_, err = cmd.parseScaffoldArgs([]string{"--model", "User", "--gsi", "bad"})
	require.Error(t, err)

	cfg, err := cmd.parseScaffoldArgs([]string{
		"--model", "User",
		"--multi-tenant",
		"--enable-ttl",
		"--enable-streams",
		"--gsi", "TenantIndex:tenant_id:id",
		"--gsi", "StatusIndex:status",
	})
	require.NoError(t, err)
	require.Equal(t, "User", cfg.ModelName)
	require.Equal(t, "users", cfg.TableName)
	require.True(t, cfg.MultiTenant)
	require.True(t, cfg.EnableTTL)
	require.True(t, cfg.EnableStreams)
	require.Len(t, cfg.GSIs, 2)
	require.Equal(t, "TenantIndex", cfg.GSIs[0].IndexName)
	require.Equal(t, "tenant_id", cfg.GSIs[0].PartitionKey)
	require.Equal(t, "id", cfg.GSIs[0].SortKey)
	require.Equal(t, "StatusIndex", cfg.GSIs[1].IndexName)
	require.Equal(t, "status", cfg.GSIs[1].PartitionKey)
	require.Equal(t, "", cfg.GSIs[1].SortKey)

	gsi, err := cmd.parseGSI("Idx:pk:sk")
	require.NoError(t, err)
	require.Equal(t, "Idx", gsi.IndexName)
	require.Equal(t, "pk", gsi.PartitionKey)
	require.Equal(t, "sk", gsi.SortKey)
}

func TestDynamORMScaffoldCommand_Execute_GeneratesFiles(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &DynamORMScaffoldCommand{}
	require.Error(t, cmd.Execute(context.Background(), []string{"--model", "User"}))

	require.NoError(t, os.WriteFile("go.mod", []byte("module example.com/test\n\nrequire github.com/pay-theory/lift v0.0.0\n"), 0600))
	require.NoError(t, cmd.Execute(context.Background(), []string{
		"--model", "User",
		"--table", "users",
		"--multi-tenant",
		"--enable-ttl",
		"--enable-streams",
		"--gsi", "StatusIndex:status",
	}))

	assertFileExists(t, filepath.Join(tmpDir, "models", "user.go"))
	assertFileExists(t, filepath.Join(tmpDir, "cdk", "constructs", "user_table.go"))
	assertFileExists(t, filepath.Join(tmpDir, "examples", "user_example.go"))

	modelBytes, err := os.ReadFile(filepath.Join(tmpDir, "models", "user.go"))
	require.NoError(t, err)
	require.Contains(t, string(modelBytes), "TenantID string")
	require.Contains(t, string(modelBytes), "TTL")
	require.Contains(t, string(modelBytes), "ExpiresAt")

	exampleBytes, err := os.ReadFile(filepath.Join(tmpDir, "examples", "user_example.go"))
	require.NoError(t, err)
	require.Contains(t, string(exampleBytes), "\"example.com/test/models\"")
}

func TestDynamORMMigrateCommand_ParseMigrateArgs_AndHelpers(t *testing.T) {
	cmd := &DynamORMMigrateCommand{}

	_, err := cmd.parseMigrateArgs([]string{})
	require.Error(t, err)

	cfg, err := cmd.parseMigrateArgs([]string{"--table", "lift-merchant-applications"})
	require.NoError(t, err)
	require.Equal(t, "us-east-1", cfg.Region)
	require.Equal(t, "migration", cfg.OutputDir)
	require.True(t, cfg.GenerateTests)
	require.Equal(t, "MerchantApplication", cfg.ModelName)

	cfg, err = cmd.parseMigrateArgs([]string{
		"--table", "app-orders-table",
		"--region", "us-west-2",
		"--output-dir", "out",
		"--analyze-only",
		"--model", "Order",
		"--multi-tenant",
		"--no-tests",
	})
	require.NoError(t, err)
	require.Equal(t, "app-orders-table", cfg.TableName)
	require.Equal(t, "us-west-2", cfg.Region)
	require.Equal(t, "out", cfg.OutputDir)
	require.True(t, cfg.AnalyzeOnly)
	require.Equal(t, "Order", cfg.ModelName)
	require.True(t, cfg.MultiTenant)
	require.False(t, cfg.GenerateTests)

	require.Equal(t, "User", cmd.generateModelName("users"))
	require.Equal(t, "MyThing", cmd.generateModelName("lift-my-things-table"))
}

func TestDynamORMMigrateCommand_AnalyzeHelpers_AndGeneration(t *testing.T) {
	cmd := &DynamORMMigrateCommand{}

	require.Equal(t, "string", cmd.convertDynamoType(types.ScalarAttributeTypeS))
	require.Equal(t, "number", cmd.convertDynamoType(types.ScalarAttributeTypeN))
	require.Equal(t, "binary", cmd.convertDynamoType(types.ScalarAttributeTypeB))

	require.Equal(t, "v", cmd.convertAttributeValue(&types.AttributeValueMemberS{Value: "v"}))
	require.Equal(t, "1", cmd.convertAttributeValue(&types.AttributeValueMemberN{Value: "1"}))
	require.Equal(t, []byte{0x01}, cmd.convertAttributeValue(&types.AttributeValueMemberB{Value: []byte{0x01}}))
	require.Equal(t, true, cmd.convertAttributeValue(&types.AttributeValueMemberBOOL{Value: true}))
	require.Equal(t, nil, cmd.convertAttributeValue(&types.AttributeValueMemberNULL{Value: true}))
	require.Equal(t, "complex_type", cmd.convertAttributeValue(&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}}))

	attrs := []types.AttributeDefinition{
		{AttributeName: strPtr("id"), AttributeType: types.ScalarAttributeTypeS},
		{AttributeName: strPtr("count"), AttributeType: types.ScalarAttributeTypeN},
		{AttributeName: strPtr("ttl"), AttributeType: types.ScalarAttributeTypeN},
	}
	ttl := newTTLAnalyzer().analyzeTTL(attrs)
	require.NotNil(t, ttl)
	require.Equal(t, "ttl", ttl.AttributeName)

	keyAnalyzer := newKeySchemaAnalyzer(cmd, attrs)
	spec := keyAnalyzer.analyzeKey(types.KeySchemaElement{AttributeName: strPtr("id"), KeyType: types.KeyTypeHash})
	require.NotNil(t, spec)
	require.Equal(t, "id", spec.Name)
	require.Equal(t, "string", spec.Type)

	gsi := newGSIAnalyzer(cmd, attrs).analyzeGSI(types.GlobalSecondaryIndexDescription{
		IndexName:  strPtr("TenantIndex"),
		ItemCount:  int64Ptr(0),
		Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: strPtr("id"), KeyType: types.KeyTypeHash},
			{AttributeName: strPtr("count"), KeyType: types.KeyTypeRange},
		},
	})
	require.Equal(t, "TenantIndex", gsi.IndexName)
	require.Equal(t, "id", gsi.PartitionKey.Name)
	require.NotNil(t, gsi.SortKey)
	require.Equal(t, "count", gsi.SortKey.Name)

	lsi := newLSIAnalyzer(cmd, attrs).analyzeLSI(types.LocalSecondaryIndexDescription{
		IndexName:  strPtr("CountIndex"),
		ItemCount:  int64Ptr(0),
		Projection: &types.Projection{ProjectionType: types.ProjectionTypeKeysOnly},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: strPtr("id"), KeyType: types.KeyTypeHash},
			{AttributeName: strPtr("count"), KeyType: types.KeyTypeRange},
		},
	})
	require.Equal(t, "CountIndex", lsi.IndexName)
	require.Equal(t, "count", lsi.SortKey.Name)

	analysis := &TableAnalysis{
		TableName: "lift-my-items-table",
		CreatedAt: time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC),
		PartitionKey: AttributeSpec{
			Name:     "tenant_id",
			Type:     "string",
			Required: true,
		},
		SortKey: &AttributeSpec{
			Name:     "id",
			Type:     "number",
			Required: true,
		},
		TimeToLiveSpec: &TTLSpec{AttributeName: "expires_at", Enabled: false},
		StreamSpec:     &StreamSpec{Enabled: true, ViewType: keysOnlyView, StreamArn: "arn"},
		GlobalSecondaryIndexes: []GSIAnalysis{
			{
				IndexName:      "AllIndex",
				ProjectionType: allView,
				ItemCount:      0,
				PartitionKey:   AttributeSpec{Name: "status", Type: "binary", Required: true},
				SortKey:        &AttributeSpec{Name: "created_at", Type: "string", Required: true},
			},
			{
				IndexName:      "KeysOnlyIndex",
				ProjectionType: keysOnlyView,
				ItemCount:      0,
				PartitionKey:   AttributeSpec{Name: "account_id", Type: "unknown", Required: true},
			},
			{
				IndexName:      "IncludeIndex",
				ProjectionType: "INCLUDE",
				ItemCount:      0,
				PartitionKey:   AttributeSpec{Name: "org_id", Type: "string", Required: true},
			},
			{
				IndexName:      "UnknownProjection",
				ProjectionType: "SOMETHING",
				ItemCount:      0,
				PartitionKey:   AttributeSpec{Name: "pk", Type: "string", Required: true},
			},
		},
		LocalSecondaryIndexes: []LSIAnalysis{{IndexName: "lsi", ProjectionType: keysOnlyView, SortKey: AttributeSpec{Name: "count", Type: "number", Required: true}}},
		SampleItems: []map[string]interface{}{
			{"ok": "value"},
			{"nested": map[string]interface{}{"x": 1}},
		},
	}

	analysis.MigrationComplexity = cmd.determineMigrationComplexity(analysis)
	require.Equal(t, "Complex", analysis.MigrationComplexity)
	analysis.MultiTenantCandidate = cmd.isMultiTenantCandidate(analysis)
	require.True(t, analysis.MultiTenantCandidate)
	analysis.RecommendedModel = cmd.generateRecommendedModel(analysis)
	require.Contains(t, analysis.RecommendedModel, "Multi-Tenant")

	cmd.printAnalysisSummary(analysis)

	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "analysis.json")
	require.NoError(t, cmd.saveAnalysis(analysis, outPath))

	raw, err := os.ReadFile(outPath)
	require.NoError(t, err)
	var decoded TableAnalysis
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.Equal(t, analysis.TableName, decoded.TableName)

	cfg := &MigrationConfig{
		TableName:     analysis.TableName,
		Region:        "us-east-1",
		OutputDir:     filepath.Join(tmpDir, "out"),
		ModelName:     "MyItem",
		AnalyzeOnly:   false,
		MultiTenant:   true,
		GenerateTests: true,
	}
	require.NoError(t, os.MkdirAll(cfg.OutputDir, 0750))
	require.NoError(t, cmd.generateMigrationCode(analysis, cfg))

	assertFileExists(t, filepath.Join(cfg.OutputDir, "myitem.go"))
	assertFileExists(t, filepath.Join(cfg.OutputDir, "myitem_table.go"))
	assertFileExists(t, filepath.Join(cfg.OutputDir, "migrate_myitem.go"))
	assertFileExists(t, filepath.Join(cfg.OutputDir, "myitem_test.go"))

	cdkBytes, err := os.ReadFile(filepath.Join(cfg.OutputDir, "myitem_table.go"))
	require.NoError(t, err)
	require.Contains(t, string(cdkBytes), "AttributeType_STRING")
	require.Contains(t, string(cdkBytes), "AttributeType_NUMBER")
	require.Contains(t, string(cdkBytes), "AttributeType_BINARY")
	require.Contains(t, string(cdkBytes), "StreamViewType_KEYS_ONLY")
	require.Contains(t, string(cdkBytes), "ProjectionType_KEYS_ONLY")
	require.Contains(t, string(cdkBytes), "ProjectionType_INCLUDE")
}

func strPtr(s string) *string { return &s }
func int64Ptr(v int64) *int64 { return &v }
