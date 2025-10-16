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
// an existing vpc-authorizer Lambda function in the partner account.
type VPCAuthorizerProps struct {
	// Partner name (e.g., "paytheory", "innovate", "austin")
	Partner *string

	// Stage name (e.g., "paytheory", "paytheorystudy", "paytheorylab")
	Stage *string

	// Identity source for the authorizer (default: "$request.header.Authorization")
	IdentitySource *[]*string

	// TTL for authorization cache in seconds (default: 300)
	ResultsCacheTtl *float64

	// AWS account ID where the vpc-authorizer Lambda is deployed
	// If not provided, will use the current stack's account
	AccountID *string

	// AWS region where the vpc-authorizer Lambda is deployed
	// If not provided, will use the current stack's region
	Region *string
}

// VPCAuthorizer is a construct that creates a Lambda authorizer for VPC authentication.
//
// This construct references an existing vpc-authorizer Lambda function that is
// deployed in all partner accounts following the naming pattern:
// vpc-authorizer-{partner}-{stage}
//
// The authorizer validates requests using the Authorization header and returns
// simple responses for HTTP API Gateway v2.
type VPCAuthorizer struct {
	constructs.Construct
	CfnAuthorizer awsapigatewayv2.CfnAuthorizer
	props         *VPCAuthorizerProps
}

// NewVPCAuthorizer creates a new VPC authorizer construct.
//
// This function creates a Lambda authorizer that references an existing
// vpc-authorizer Lambda function in the partner account. The Lambda function
// should already exist with the naming pattern: vpc-authorizer-{partner}-{stage}
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
//   - props: Configuration properties including partner and stage
//
// Returns:
//   - A new VPCAuthorizer instance
func NewVPCAuthorizer(scope constructs.Construct, id *string, props *VPCAuthorizerProps) *VPCAuthorizer {
	this := constructs.NewConstruct(scope, id)

	// Set defaults
	if props.IdentitySource == nil {
		props.IdentitySource = &[]*string{jsii.String("$request.header.Authorization")}
	}
	if props.ResultsCacheTtl == nil {
		props.ResultsCacheTtl = jsii.Number(300)
	}

	// Get stack to access account and region
	stack := awscdk.Stack_Of(scope)
	accountID := props.AccountID
	if accountID == nil {
		accountID = stack.Account()
	}
	region := props.Region
	if region == nil {
		region = stack.Region()
	}

	// Construct ARN for existing vpc-authorizer Lambda
	// Pattern: arn:aws:lambda:{region}:{account}:function:vpc-authorizer-{partner}-{stage}:published
	authorizerFunctionArn := jsii.String(fmt.Sprintf(
		"arn:aws:lambda:%s:%s:function:vpc-authorizer-%s-%s:published",
		*region,
		*accountID,
		*props.Partner,
		*props.Stage,
	))

	// Create HTTP Lambda authorizer using lower-level Cfn construct
	// We use Cfn constructs to have full control over the authorizer configuration
	// This is necessary because the high-level constructs don't support all options
	// Note: ApiId is NOT set here - it must be set later when attaching to an API
	cfnAuthorizer := awsapigatewayv2.NewCfnAuthorizer(this, jsii.String("VPCAuthorizerCfn"), &awsapigatewayv2.CfnAuthorizerProps{
		Name:           jsii.String(fmt.Sprintf("vpc-authorizer-%s-%s", *props.Partner, *props.Stage)),
		AuthorizerType: jsii.String("REQUEST"),
		AuthorizerUri: jsii.String(fmt.Sprintf(
			"arn:aws:apigateway:%s:lambda:path/2015-03-31/functions/%s/invocations",
			*region,
			*authorizerFunctionArn,
		)),
		// Use the existing vpc-authorizer IAM role to invoke the authorizer
		AuthorizerCredentialsArn: jsii.String(fmt.Sprintf(
			"arn:aws:iam::%s:role/vpc-authorizer-%s-%s-role",
			*accountID,
			*props.Partner,
			*props.Stage,
		)),
		EnableSimpleResponses:          jsii.Bool(true),
		AuthorizerPayloadFormatVersion: jsii.String("2.0"),
		IdentitySource:                 props.IdentitySource,
		AuthorizerResultTtlInSeconds:   jsii.Number(*props.ResultsCacheTtl),
	})

	return &VPCAuthorizer{
		Construct:     this,
		CfnAuthorizer: cfnAuthorizer,
		props:         props,
	}
}
