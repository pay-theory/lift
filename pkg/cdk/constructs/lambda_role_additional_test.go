package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/jsii-runtime-go"
)

func TestLiftLambdaRole_SQSAndS3AccessAndHelpers(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	liftRole := NewLiftLambdaRole(stack, jsii.String("TestRole"), &LiftLambdaRoleProps{
		RoleName: jsii.String("test-lambda-role"),
		SQSQueueArns: []string{
			"arn:aws:sqs:us-east-1:123456789012:test-queue",
		},
		EnableSQSSendMessage:   jsii.Bool(true),
		EnableSQSReceiveDelete: jsii.Bool(true),
		S3BucketArns: []string{
			"arn:aws:s3:::test-bucket",
		},
		EnableS3Read:  jsii.Bool(true),
		EnableS3Write: jsii.Bool(true),
		Tags: map[string]string{
			"Env": "test",
		},
	})

	if liftRole.GetRole() == nil {
		t.Fatal("expected underlying role")
	}
	if liftRole.GetRoleArn() == nil {
		t.Fatal("expected role arn")
	}
	if liftRole.GetRoleName() == nil {
		t.Fatal("expected role name")
	}
	if liftRole.AsLambdaExecutionRole() == nil {
		t.Fatal("expected lambda execution role")
	}

	grantee := awsiam.NewRole(stack, jsii.String("Grantee"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewAccountPrincipal(jsii.String("123456789012")),
	})
	liftRole.GrantPassRole(grantee)

	liftRole.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Effect: awsiam.Effect_ALLOW,
		Actions: &[]*string{
			jsii.String("logs:CreateLogGroup"),
		},
		Resources: &[]*string{
			jsii.String("*"),
		},
	}))

	liftRole.AddManagedPolicy(
		awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("AWSXRayDaemonWriteAccess")),
	)

	template := synthesizeTemplate(t, stack)
	policies := findResourcesByType(template, "AWS::IAM::Policy")
	if len(policies) == 0 {
		t.Fatal("expected at least one IAM policy")
	}

	actions := map[string]bool{}
	for _, policy := range policies {
		props, ok := policy["Properties"].(map[string]interface{})
		if !ok {
			continue
		}
		doc, ok := props["PolicyDocument"].(map[string]interface{})
		if !ok {
			continue
		}
		statements, ok := doc["Statement"]
		if !ok {
			continue
		}

		var statementList []interface{}
		switch v := statements.(type) {
		case []interface{}:
			statementList = v
		case map[string]interface{}:
			statementList = []interface{}{v}
		default:
			continue
		}

		for _, st := range statementList {
			stmt, ok := st.(map[string]interface{})
			if !ok {
				continue
			}
			actionVal, ok := stmt["Action"]
			if !ok {
				continue
			}
			switch a := actionVal.(type) {
			case string:
				actions[a] = true
			case []interface{}:
				for _, item := range a {
					if s, ok := item.(string); ok {
						actions[s] = true
					}
				}
			}
		}
	}

	for _, required := range []string{"sqs:SendMessage", "s3:GetObject", "iam:PassRole"} {
		if !actions[required] {
			t.Fatalf("expected to find %s in IAM policies", required)
		}
	}
}
