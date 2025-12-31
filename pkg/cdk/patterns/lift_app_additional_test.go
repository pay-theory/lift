package patterns

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/jsii-runtime-go"
	"github.com/stretchr/testify/require"

	liftconstructs "github.com/pay-theory/lift/pkg/cdk/constructs"
)

func TestNewLiftApp_CreatesAPIAndTables(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	env := map[string]*string{
		"FOO": jsii.String("bar"),
	}

	liftApp := NewLiftApp(stack, jsii.String("LiftApp"), &LiftAppProps{
		AppName:              jsii.String("my-app"),
		CodeAssetPath:        jsii.String("."),
		EnableMultiTenant:    jsii.Bool(true),
		EnableAccessLogging:  jsii.Bool(true),
		Environment:          &env,
		MemorySize:           jsii.Number(256),
		Timeout:              jsii.Number(10),
		EnableDatabase:       jsii.Bool(true),
		DatabasePartitionKey: jsii.String("PK"),
		DatabaseSortKey:      jsii.String("SK"),
		EnableRateLimiting:   jsii.Bool(true),
		RateLimitTableName:   jsii.String("rate-table"),
		EnableIdempotency:    jsii.Bool(true),
	})

	require.NotNil(t, liftApp)
	require.NotNil(t, liftApp.API)
	require.NotNil(t, liftApp.Function)
	require.NotNil(t, liftApp.Database)
	require.NotNil(t, liftApp.RateLimitTable)

	template := assertions.Template_FromStack(stack, nil)
	template.ResourceCountIs(jsii.String("AWS::Lambda::Function"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::ApiGatewayV2::Api"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::DynamoDB::Table"), jsii.Number(2))

	template.HasResourceProperties(jsii.String("AWS::Lambda::Function"), &map[string]interface{}{
		"Environment": map[string]interface{}{
			"Variables": map[string]interface{}{
				"FOO":              "bar",
				"DYNAMODB_TABLE":   "my-app-table",
				"RATE_LIMIT_TABLE": "rate-table",
			},
		},
	})

	template.ResourceCountIs(jsii.String("AWS::ApiGatewayV2::Route"), jsii.Number(2))
	raw := template.ToJSON()
	outputs, ok := (*raw)["Outputs"].(map[string]interface{})
	require.True(t, ok)
	require.Contains(t, outputs, "ApiUrl")
}

func TestNewLiftApp_UsesExistingDatabaseTable(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	existing := liftconstructs.NewLiftTable(stack, jsii.String("ExistingDatabase"), &liftconstructs.LiftTableProps{
		TableName:        jsii.String("existing-table"),
		PartitionKeyName: jsii.String("ID"),
	})

	liftApp := NewLiftApp(stack, jsii.String("LiftApp"), &LiftAppProps{
		AppName:        jsii.String("my-app"),
		CodeAssetPath:  jsii.String("."),
		DatabaseTable:  existing,
		EnableDatabase: jsii.Bool(false),
	})

	require.NotNil(t, liftApp.Database)
	require.Equal(t, existing, liftApp.Database)

	template := assertions.Template_FromStack(stack, nil)
	template.HasResourceProperties(jsii.String("AWS::DynamoDB::Table"), &map[string]interface{}{
		"TableName": "existing-table",
	})
	template.HasResourceProperties(jsii.String("AWS::Lambda::Function"), &map[string]interface{}{
		"Environment": map[string]interface{}{
			"Variables": map[string]interface{}{
				"DYNAMODB_TABLE": assertions.Match_AnyValue(),
			},
		},
	})
}
