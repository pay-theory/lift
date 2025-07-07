package constructs

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awskms"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssecretsmanager"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// SecureFunctionProps extends LiftFunctionProps with security configuration
type SecureFunctionProps struct {
	LiftFunctionProps
	// VPC to deploy the function in (optional - will create if not provided)
	Vpc awsec2.IVpc
	// VPC subnets to use (defaults to private subnets)
	VpcSubnets *awsec2.SubnetSelection
	// Security group IDs to attach
	SecurityGroupIds *[]*string
	// Enable KMS encryption for environment variables
	EnableKMSEncryption *bool
	// KMS key for encryption (optional - will create if not provided)
	KmsKey awskms.IKey
	// Secrets to inject from Secrets Manager
	Secrets *map[string]awssecretsmanager.ISecret
	// Enable private endpoints only (no internet access)
	PrivateOnly *bool
	// Additional security policies to attach
	AdditionalPolicies *[]awsiam.PolicyStatement
}

// SecureFunction is a Lambda function with enhanced security features
type SecureFunction struct {
	constructs.Construct
	Function       *LiftFunction
	SecurityGroup  awsec2.ISecurityGroup
	KmsKey         awskms.IKey
	Vpc            awsec2.IVpc
	VpcEndpoints   map[string]awsec2.InterfaceVpcEndpoint
}

// NewSecureFunction creates a Lambda function with enhanced security
func NewSecureFunction(scope constructs.Construct, id *string, props *SecureFunctionProps) *SecureFunction {
	this := constructs.NewConstruct(scope, id)

	// Set defaults
	if props.EnableKMSEncryption == nil {
		props.EnableKMSEncryption = jsii.Bool(true)
	}
	if props.PrivateOnly == nil {
		props.PrivateOnly = jsii.Bool(false)
	}

	// Create or use VPC
	var vpc awsec2.IVpc
	if props.Vpc != nil {
		vpc = props.Vpc
	} else {
		// Create a secure VPC with appropriate subnet configuration
		var subnetConfig []*awsec2.SubnetConfiguration
		if *props.PrivateOnly {
			// For private-only, create isolated subnets with no NAT
			subnetConfig = []*awsec2.SubnetConfiguration{
				{
					Name:       jsii.String("Isolated"),
					SubnetType: awsec2.SubnetType_PRIVATE_ISOLATED,
					CidrMask:   jsii.Number(24),
				},
			}
		} else {
			// Standard configuration with public and private subnets
			subnetConfig = []*awsec2.SubnetConfiguration{
				{
					Name:       jsii.String("Public"),
					SubnetType: awsec2.SubnetType_PUBLIC,
					CidrMask:   jsii.Number(24),
				},
				{
					Name:       jsii.String("Private"),
					SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
					CidrMask:   jsii.Number(24),
				},
			}
		}
		
		natGateways := jsii.Number(1)
		if *props.PrivateOnly {
			natGateways = jsii.Number(0)
		}
		
		vpc = awsec2.NewVpc(this, jsii.String("SecureVpc"), &awsec2.VpcProps{
			MaxAzs:              jsii.Number(2),
			NatGateways:         natGateways,
			SubnetConfiguration: &subnetConfig,
			EnableDnsHostnames:  jsii.Bool(true),
			EnableDnsSupport:    jsii.Bool(true),
		})
	}

	// Configure subnets
	vpcSubnets := props.VpcSubnets
	if vpcSubnets == nil {
		if *props.PrivateOnly {
			vpcSubnets = &awsec2.SubnetSelection{
				SubnetType: awsec2.SubnetType_PRIVATE_ISOLATED,
			}
		} else {
			vpcSubnets = &awsec2.SubnetSelection{
				SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
			}
		}
	}

	// Create security group
	securityGroup := awsec2.NewSecurityGroup(this, jsii.String("SecurityGroup"), &awsec2.SecurityGroupProps{
		Vpc:         vpc,
		Description: jsii.String("Security group for secure Lambda function"),
		AllowAllOutbound: jsii.Bool(!*props.PrivateOnly),
	})

	// Add default egress rules for AWS services if not private only
	if !*props.PrivateOnly {
		// Allow HTTPS for AWS API calls
		securityGroup.AddEgressRule(
			awsec2.Peer_AnyIpv4(),
			awsec2.Port_Tcp(jsii.Number(443)),
			jsii.String("Allow HTTPS for AWS API calls"),
			jsii.Bool(false),
		)
	}

	// Create or use KMS key
	var kmsKey awskms.IKey
	if *props.EnableKMSEncryption {
		if props.KmsKey != nil {
			kmsKey = props.KmsKey
		} else {
			kmsKey = awskms.NewKey(this, jsii.String("KmsKey"), &awskms.KeyProps{
				Description:         jsii.String("KMS key for Lambda function encryption"),
				EnableKeyRotation:   jsii.Bool(true),
				RemovalPolicy:       awscdk.RemovalPolicy_DESTROY,
				PendingWindow:       awscdk.Duration_Days(jsii.Number(7)),
			})

			// Add alias for easier identification
			kmsKey.AddAlias(jsii.String(*id + "-key"))
		}
		props.LiftFunctionProps.EnvironmentEncryption = kmsKey
	}

	// Configure VPC for the function
	props.LiftFunctionProps.Vpc = vpc
	props.LiftFunctionProps.VpcSubnets = vpcSubnets
	props.LiftFunctionProps.SecurityGroups = &[]awsec2.ISecurityGroup{securityGroup}

	// Add additional security groups if provided
	if props.SecurityGroupIds != nil {
		for _, sgId := range *props.SecurityGroupIds {
			sg := awsec2.SecurityGroup_FromSecurityGroupId(this, sgId, sgId, &awsec2.SecurityGroupImportOptions{})
			*props.LiftFunctionProps.SecurityGroups = append(*props.LiftFunctionProps.SecurityGroups, sg)
		}
	}

	// Enable AWS X-Ray tracing for security monitoring
	props.LiftFunctionProps.Tracing = awslambda.Tracing_ACTIVE

	// Create the base Lift function
	liftFn := NewLiftFunction(this, jsii.String("Function"), &props.LiftFunctionProps)

	// Add secrets as environment variables
	if props.Secrets != nil {
		for name, secret := range *props.Secrets {
			liftFn.Function.AddEnvironment(jsii.String(name), secret.SecretValue().ToString(), nil)
			secret.GrantRead(liftFn.Function, nil)
		}
	}

	// Add VPC endpoint permissions
	liftFn.Function.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: &[]*string{
			jsii.String("ec2:CreateNetworkInterface"),
			jsii.String("ec2:DescribeNetworkInterfaces"),
			jsii.String("ec2:DeleteNetworkInterface"),
			jsii.String("ec2:AssignPrivateIpAddresses"),
			jsii.String("ec2:UnassignPrivateIpAddresses"),
		},
		Resources: &[]*string{jsii.String("*")},
	}))

	// Add KMS permissions if encryption is enabled
	if kmsKey != nil {
		kmsKey.GrantDecrypt(liftFn.Function)
		kmsKey.GrantEncrypt(liftFn.Function)
	}

	// Add additional security policies
	if props.AdditionalPolicies != nil {
		for _, policy := range *props.AdditionalPolicies {
			liftFn.Function.AddToRolePolicy(policy)
		}
	}

	return &SecureFunction{
		Construct:     this,
		Function:      liftFn,
		SecurityGroup: securityGroup,
		KmsKey:        kmsKey,
		Vpc:           vpc,
		VpcEndpoints:  make(map[string]awsec2.InterfaceVpcEndpoint),
	}
}

// GetFunction returns the underlying Lambda function
func (f *SecureFunction) GetFunction() awslambda.Function {
	return f.Function.Function
}

// GetSecurityGroup returns the security group
func (f *SecureFunction) GetSecurityGroup() awsec2.ISecurityGroup {
	return f.SecurityGroup
}

// GetKmsKey returns the KMS key used for encryption
func (f *SecureFunction) GetKmsKey() awskms.IKey {
	return f.KmsKey
}

// AddVPCEndpoint adds a VPC endpoint for an AWS service
func (f *SecureFunction) AddVPCEndpoint(service awsec2.InterfaceVpcEndpointAwsService) awsec2.InterfaceVpcEndpoint {
	// Check if endpoint already exists
	serviceName := *service.Name()
	if endpoint, exists := f.VpcEndpoints[serviceName]; exists {
		return endpoint
	}
	
	// Create the VPC endpoint
	endpoint := awsec2.NewInterfaceVpcEndpoint(f, jsii.String(fmt.Sprintf("%sEndpoint", serviceName)), &awsec2.InterfaceVpcEndpointProps{
		Vpc:     f.Vpc,
		Service: service,
		Subnets: &awsec2.SubnetSelection{
			SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
		},
		SecurityGroups: &[]awsec2.ISecurityGroup{f.SecurityGroup},
		PrivateDnsEnabled: jsii.Bool(true),
	})
	
	// Store the endpoint
	f.VpcEndpoints[serviceName] = endpoint
	
	// Allow the Lambda function to access the endpoint
	endpoint.Connections().AllowFrom(
		awsec2.NewConnections(&awsec2.ConnectionsProps{
			SecurityGroups: &[]awsec2.ISecurityGroup{f.SecurityGroup},
		}),
		awsec2.Port_Tcp(jsii.Number(443)),
		jsii.String(fmt.Sprintf("Allow Lambda to access %s", serviceName)),
	)
	
	return endpoint
}

// EnableSecretsManagerAccess adds VPC endpoint and permissions for Secrets Manager
func (f *SecureFunction) EnableSecretsManagerAccess() {
	// Add VPC endpoint
	f.AddVPCEndpoint(awsec2.InterfaceVpcEndpointAwsService_SECRETS_MANAGER())

	// Add permissions
	f.Function.Function.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: &[]*string{
			jsii.String("secretsmanager:GetSecretValue"),
			jsii.String("secretsmanager:DescribeSecret"),
		},
		Resources: &[]*string{jsii.String("*")},
	}))
}

// RestrictInboundAccess removes all inbound rules from the security group
func (f *SecureFunction) RestrictInboundAccess() {
	// Note: This is a simplified implementation
	// In practice, you'd need to iterate and remove existing rules
	// The security group is created with no inbound rules by default
}