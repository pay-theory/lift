package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/jsii-runtime-go"
)

func TestEventBusTable_EnvBasedNamingAndEnvironmentVariables(t *testing.T) {
	t.Setenv("APP_NAME", "myapp")
	t.Setenv("STAGE", "dev")
	t.Setenv("PARTNER", "tenant")

	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	table := NewEventBusTable(stack, jsii.String("Events"), nil)
	if table == nil {
		t.Fatal("expected event bus table")
	}

	if table.GetTableName() == nil {
		t.Fatal("expected table name")
	}

	env := table.GetEnvironmentVariables()
	if env == nil {
		t.Fatal("expected environment variables")
	}
	if _, ok := (*env)["EVENT_BUS_TABLE_NAME"]; !ok {
		t.Fatal("expected EVENT_BUS_TABLE_NAME")
	}
	if _, ok := (*env)["EVENT_BUS_STREAM_ARN"]; ok {
		t.Fatal("did not expect EVENT_BUS_STREAM_ARN when streams are disabled")
	}
	if table.GetTableArn() == nil {
		t.Fatal("expected table arn")
	}
	if table.GetStreamArn() != nil {
		t.Fatal("expected nil stream arn when streams are disabled")
	}

	template := synthesizeTemplate(t, stack)
	tables := findResourcesByType(template, "AWS::DynamoDB::Table")
	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}
	for _, res := range tables {
		props, ok := res["Properties"].(map[string]interface{})
		if !ok {
			continue
		}
		if props["TableName"] != "myapp-tenant-events-lab" {
			t.Fatalf("unexpected TableName: %v", props["TableName"])
		}
	}
}

func TestEventBusTable_ProvisionedStreamIndexesAndGrants(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	table := NewEventBusTable(stack, jsii.String("Events"), &EventBusTableProps{
		TableName:          jsii.String("explicit-events-table"),
		BillingMode:        awsdynamodb.BillingMode_PROVISIONED,
		EnableStream:       jsii.Bool(true),
		StreamViewType:     awsdynamodb.StreamViewType_NEW_IMAGE,
		EnableEventIDIndex: jsii.Bool(true),
		Tags: &map[string]*string{
			"Env": jsii.String("test"),
		},
	})

	fn := awslambda.NewFunction(stack, jsii.String("Fn"), &awslambda.FunctionProps{
		Runtime: awslambda.Runtime_NODEJS_18_X(),
		Handler: jsii.String("index.handler"),
		Code:    awslambda.Code_FromInline(jsii.String("exports.handler = async () => {}")),
	})

	table.GrantRead(fn)
	table.GrantWrite(fn)
	table.GrantReadWrite(fn)
	table.GrantStreamRead(fn)

	if table.GetTableName() == nil {
		t.Fatal("expected table name")
	}
	if table.GetTableArn() == nil {
		t.Fatal("expected table arn")
	}
	if table.GetStreamArn() == nil {
		t.Fatal("expected stream arn")
	}

	env := table.GetEnvironmentVariables()
	if env == nil {
		t.Fatal("expected environment variables")
	}
	if _, ok := (*env)["EVENT_BUS_STREAM_ARN"]; !ok {
		t.Fatal("expected EVENT_BUS_STREAM_ARN when streams are enabled")
	}

	template := synthesizeTemplate(t, stack)
	assertResourceExists(t, template, "AWS::DynamoDB::Table")
	assertResourceExists(t, template, "AWS::IAM::Policy")
}
