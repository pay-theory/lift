package constructs

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awskms"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// LambdaFunctionConfig defines configuration for creating Lambda functions
type LambdaFunctionConfig struct {
	FunctionName string
	Description  string
	Timeout      awscdk.Duration
	Permissions  string // "read" or "readwrite"
	Environment  map[string]*string
}

// CreateStandardLambdaFunction creates a Lambda function with common configurations
func CreateStandardLambdaFunction(scope constructs.Construct, id string, bucket awss3.Bucket, encryptionKey awskms.Key, config LambdaFunctionConfig) awslambda.Function {
	// Create IAM role
	role := awsiam.NewRole(scope, jsii.String(id+"Role"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("lambda.amazonaws.com"), nil),
		ManagedPolicies: &[]awsiam.IManagedPolicy{
			awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("service-role/AWSLambdaBasicExecutionRole")),
		},
	})

	// Grant permissions
	if config.Permissions == "readwrite" {
		bucket.GrantReadWrite(role, nil)
	} else {
		bucket.GrantRead(role, nil)
	}
	
	if encryptionKey != nil {
		encryptionKey.GrantEncryptDecrypt(role)
	}

	return awslambda.NewFunction(scope, jsii.String(id), &awslambda.FunctionProps{
		FunctionName: jsii.String(config.FunctionName),
		Runtime:      awslambda.Runtime_PROVIDED_AL2(),
		Handler:      jsii.String("bootstrap"),
		Code:         awslambda.Code_FromAsset(jsii.String("./dist"), nil),
		Role:         role,
		Description:  jsii.String(config.Description),
		Timeout:      config.Timeout,
		Environment:  &config.Environment,
	})
}