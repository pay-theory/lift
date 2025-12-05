// Package constructs provides AWS CDK constructs for Lift applications.
package constructs

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// VPCAuthorizerProps defines properties for creating a VPC authorizer.
//
// This struct contains configuration for the VPC authorizer that references
// an existing Lambda authorizer function.
type VPCAuthorizerProps struct {
	// AuthorizerFunctionArn is the full ARN of the authorizer Lambda function (required)
	// Example: "arn:aws:lambda:us-east-1:123456789:function:my-authorizer"
	AuthorizerFunctionArn *string

	// AuthorizerName is the name for the authorizer in API Gateway (required)
	// Example: "my-vpc-authorizer"
	AuthorizerName *string

	// AuthorizerCredentialsArn is the IAM role ARN that API Gateway uses to invoke the Lambda (required)
	// Example: "arn:aws:iam::123456789:role/my-authorizer-role"
	AuthorizerCredentialsArn *string

	// API ID to attach the authorizer to (required)
	ApiId *string

	// Identity source for the authorizer (default: "$request.header.Authorization")
	IdentitySource *[]*string

	// TTL for authorization cache in seconds (default: 300)
	ResultsCacheTtl *float64
}

// VPCAuthorizer is a wrapper for a CloudFormation API Gateway authorizer.
//
// This struct references an existing Lambda authorizer function.
// The authorizer validates requests using the Authorization header and returns
// simple responses for HTTP API Gateway v2.
type VPCAuthorizer struct {
	CfnAuthorizer awsapigatewayv2.CfnAuthorizer
	props         *VPCAuthorizerProps
}

// NewVPCAuthorizer creates a new VPC authorizer construct.
//
// This function creates a Lambda authorizer that references an existing
// Lambda authorizer function. The caller must provide the full ARN of the
// authorizer function, the authorizer name, and the IAM role ARN.
//
// The authorizer is configured with:
// - REQUEST authorizer type (validates entire request)
// - Simple response format (for HTTP API v2)
// - Authorization header as identity source
// - 5-minute cache TTL by default
//
// Parameters:
//   - scope: The CDK construct scope
//   - id: The construct ID
//   - props: Configuration properties including AuthorizerFunctionArn, AuthorizerName, and AuthorizerCredentialsArn
//
// Returns:
//   - A new VPCAuthorizer instance
func NewVPCAuthorizer(scope constructs.Construct, id *string, props *VPCAuthorizerProps) *VPCAuthorizer {
	// Set defaults
	if props.IdentitySource == nil {
		props.IdentitySource = &[]*string{jsii.String("$request.header.Authorization")}
	}
	if props.ResultsCacheTtl == nil {
		props.ResultsCacheTtl = jsii.Number(300)
	}

	// Get stack to access region for the authorizer URI
	stack := awscdk.Stack_Of(scope)

	// Create HTTP Lambda authorizer using lower-level Cfn construct
	// We use Cfn constructs to have full control over the authorizer configuration
	// This is necessary because the high-level constructs don't support all options
	cfnAuthorizer := awsapigatewayv2.NewCfnAuthorizer(scope, id, &awsapigatewayv2.CfnAuthorizerProps{
		ApiId:          props.ApiId,
		Name:           props.AuthorizerName,
		AuthorizerType: jsii.String("REQUEST"),
		AuthorizerUri: jsii.String(fmt.Sprintf(
			"arn:aws:apigateway:%s:lambda:path/2015-03-31/functions/%s/invocations",
			*stack.Region(),
			*props.AuthorizerFunctionArn,
		)),
		AuthorizerCredentialsArn:       props.AuthorizerCredentialsArn,
		EnableSimpleResponses:          jsii.Bool(true),
		AuthorizerPayloadFormatVersion: jsii.String("2.0"),
		IdentitySource:                 props.IdentitySource,
		AuthorizerResultTtlInSeconds:   jsii.Number(*props.ResultsCacheTtl),
	})

	return &VPCAuthorizer{
		CfnAuthorizer: cfnAuthorizer,
		props:         props,
	}
}
