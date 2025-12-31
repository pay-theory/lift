package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/jsii-runtime-go"
)

func TestLiftTable_GSIAutoScalingAndHelperMethods(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	gsi := &awsdynamodb.GlobalSecondaryIndexProps{
		IndexName: jsii.String("GSI1"),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("GPK"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("GSK"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		ProjectionType: awsdynamodb.ProjectionType_ALL,
	}

	tags := map[string]*string{"Env": jsii.String("test")}

	table := NewLiftTable(stack, jsii.String("Table"), &LiftTableProps{
		TableName:         jsii.String("auto-scale-table"),
		PartitionKeyName:  jsii.String("PK"),
		SortKeyName:       jsii.String("SK"),
		ReadCapacity:      jsii.Number(5),
		WriteCapacity:     jsii.Number(5),
		EnableAutoScaling: jsii.Bool(true),
		EnableStreams:     jsii.Bool(true),
		GlobalSecondaryIndexes: &[]*awsdynamodb.GlobalSecondaryIndexProps{
			gsi,
		},
		Tags: &tags,
	})

	if table.GetResourceName() == nil {
		t.Fatal("expected resource name")
	}
	env := table.GetEnvironmentVariables()
	if env["DYNAMODB_TABLE_NAME"] == nil {
		t.Fatal("expected DYNAMODB_TABLE_NAME env var")
	}
	if env["DYNAMORM_REGION"] == nil {
		t.Fatal("expected DYNAMORM_REGION env var")
	}

	table.AddGlobalSecondaryIndex(&awsdynamodb.GlobalSecondaryIndexProps{
		IndexName: jsii.String("GSI2"),
	})
	if _, ok := table.GSIs["GSI2"]; !ok {
		t.Fatal("expected GSI2 to be recorded")
	}

	grantee := awsiam.NewRole(stack, jsii.String("Grantee"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("lambda.amazonaws.com"), nil),
	})

	table.GrantReadWriteData(grantee)
	table.GrantReadData(grantee)
	table.GrantWriteData(grantee)
	table.GrantStreamRead(grantee)

	template := assertions.Template_FromStack(stack, nil)
	assertResourceExists(t, template, "AWS::ApplicationAutoScaling::ScalableTarget")
	assertResourceExists(t, template, "AWS::ApplicationAutoScaling::ScalingPolicy")
	assertResourceExists(t, template, "AWS::IAM::Policy")
}
