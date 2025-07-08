package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigateway"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssns"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"

	liftconstructs "github.com/pay-theory/lift/pkg/cdk/constructs"
)

type DynamORMMultiTenantStackProps struct {
	awscdk.StackProps
}

type DynamORMMultiTenantStack struct {
	awscdk.Stack
}

func NewDynamORMMultiTenantStack(scope constructs.Construct, id string, props *DynamORMMultiTenantStackProps) *DynamORMMultiTenantStack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	// Create SNS topic for alerts
	alertTopic := awssns.NewTopic(stack, jsii.String("AlertTopic"), &awssns.TopicProps{
		TopicName: jsii.String("DynamORMMultiTenantAlerts"),
	})

	// Create table with multi-tenant support
	// Note: Multi-tenancy is now handled at the data layer through DynamORM models
	table := liftconstructs.NewLiftTable(stack, jsii.String("MultiTenantTable"), &liftconstructs.LiftTableProps{
		TableName: jsii.String("DynamORMMultiTenantTable"),
		TimeToLiveAttribute: jsii.String("ttl"),
		EnablePointInTimeRecovery: jsii.Bool(true),
		EnableStreams: jsii.Bool(true),
	})

	// GSIs are now defined in DynamORM model structs using tags like:
	// TenantID string `dynamorm:"index:tenant-entity,pk"`
	// EntityType string `dynamorm:"index:tenant-entity,sk"`

	// Create Lambda function with DynamORM support
	lambdaFunction := awslambda.NewFunction(stack, jsii.String("MultiTenantFunction"), &awslambda.FunctionProps{
		Runtime: awslambda.Runtime_PROVIDED_AL2023(),
		Handler: jsii.String("bootstrap"),
		Code:    awslambda.Code_FromAsset(jsii.String("../"), nil),
		Environment: &map[string]*string{
			"DYNAMODB_TABLE_NAME": table.Table.TableName(),
			"AWS_REGION":          stack.Region(),
		},
		Tracing: awslambda.Tracing_ACTIVE,
		Timeout: awscdk.Duration_Seconds(jsii.Number(30)),
	})

	// Grant permissions to Lambda
	table.GrantReadWrite(lambdaFunction)

	// Create tenant-specific IAM role for demonstration
	tenantRole := awsiam.NewRole(stack, jsii.String("TenantRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("lambda.amazonaws.com"), nil),
		ManagedPolicies: &[]awsiam.IManagedPolicy{
			awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("service-role/AWSLambdaBasicExecutionRole")),
		},
	})

	// Grant table permissions to the tenant role
	table.Table.GrantReadWriteData(tenantRole)

	// Create API Gateway
	api := awsapigateway.NewRestApi(stack, jsii.String("MultiTenantAPI"), &awsapigateway.RestApiProps{
		RestApiName: jsii.String("DynamORMMultiTenantAPI"),
		Description: jsii.String("Multi-tenant API with DynamORM backend"),
		DefaultCorsPreflightOptions: &awsapigateway.CorsOptions{
			AllowOrigins: awsapigateway.Cors_ALL_ORIGINS(),
			AllowMethods: awsapigateway.Cors_ALL_METHODS(),
			AllowHeaders: &[]*string{
				jsii.String("Content-Type"),
				jsii.String("X-Amz-Date"),
				jsii.String("Authorization"),
				jsii.String("X-Api-Key"),
				jsii.String("X-Tenant-ID"),
			},
		},
	})

	// Create Lambda integration
	integration := awsapigateway.NewLambdaIntegration(lambdaFunction, &awsapigateway.LambdaIntegrationOptions{
		RequestTemplates: &map[string]*string{
			"application/json": jsii.String("{ \"statusCode\": \"200\" }"),
		},
	})

	// Add API routes
	apiResource := api.Root().AddResource(jsii.String("api"), nil)

	// Tenant management
	tenantsResource := apiResource.AddResource(jsii.String("tenants"), nil)
	tenantsResource.AddMethod(jsii.String("POST"), integration, nil)
	
	tenantResource := tenantsResource.AddResource(jsii.String("{id}"), nil)
	tenantResource.AddMethod(jsii.String("GET"), integration, nil)

	// User management
	usersResource := apiResource.AddResource(jsii.String("users"), nil)
	usersResource.AddMethod(jsii.String("POST"), integration, nil)
	usersResource.AddMethod(jsii.String("GET"), integration, nil)

	// Project management
	projectsResource := apiResource.AddResource(jsii.String("projects"), nil)
	projectsResource.AddMethod(jsii.String("POST"), integration, nil)
	projectsResource.AddMethod(jsii.String("GET"), integration, nil)

	// Health and metrics
	healthResource := apiResource.AddResource(jsii.String("health"), nil)
	healthResource.AddMethod(jsii.String("GET"), integration, nil)

	metricsResource := apiResource.AddResource(jsii.String("metrics"), nil)
	metricsResource.AddMethod(jsii.String("GET"), integration, nil)

	// Admin endpoints
	adminResource := api.Root().AddResource(jsii.String("admin"), nil)
	adminTenantsResource := adminResource.AddResource(jsii.String("tenants"), nil)
	adminTenantsResource.AddMethod(jsii.String("GET"), integration, nil)

	// Create CloudWatch dashboard for the entire stack
	stackDashboard := awscloudwatch.NewDashboard(stack, jsii.String("StackDashboard"), &awscloudwatch.DashboardProps{
		DashboardName: jsii.String("DynamORMMultiTenantStack"),
	})

	// Add Lambda metrics to dashboard
	lambdaMetrics := []awscloudwatch.IMetric{
		lambdaFunction.MetricInvocations(nil),
		lambdaFunction.MetricErrors(nil),
		lambdaFunction.MetricDuration(nil),
		lambdaFunction.MetricThrottles(nil),
	}

	stackDashboard.AddWidgets(awscloudwatch.NewGraphWidget(&awscloudwatch.GraphWidgetProps{
		Title:  jsii.String("Lambda Function Metrics"),
		Width:  jsii.Number(24),
		Height: jsii.Number(6),
		Left:   &lambdaMetrics,
	}))

	// Add API Gateway metrics
	apiMetrics := []awscloudwatch.IMetric{
		api.MetricCount(nil),
		api.MetricLatency(nil),
		api.MetricIntegrationLatency(nil),
		api.MetricClientError(nil),
		api.MetricServerError(nil),
	}

	stackDashboard.AddWidgets(awscloudwatch.NewGraphWidget(&awscloudwatch.GraphWidgetProps{
		Title:  jsii.String("API Gateway Metrics"),
		Width:  jsii.Number(24),
		Height: jsii.Number(6),
		Left:   &apiMetrics,
	}))

	// Output important values
	awscdk.NewCfnOutput(stack, jsii.String("TableName"), &awscdk.CfnOutputProps{
		Value:       table.Table.TableName(),
		Description: jsii.String("DynamORM Multi-Tenant Table Name"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("TableArn"), &awscdk.CfnOutputProps{
		Value:       table.Table.TableArn(),
		Description: jsii.String("DynamORM Multi-Tenant Table ARN"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("APIUrl"), &awscdk.CfnOutputProps{
		Value:       api.Url(),
		Description: jsii.String("Multi-Tenant API URL"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("LambdaFunctionName"), &awscdk.CfnOutputProps{
		Value:       lambdaFunction.FunctionName(),
		Description: jsii.String("Lambda Function Name"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("DashboardURL"), &awscdk.CfnOutputProps{
		Value: jsii.String("https://console.aws.amazon.com/cloudwatch/home?region=" + 
			*stack.Region() + "#dashboards:name=DynamORMMultiTenantStack"),
		Description: jsii.String("CloudWatch Dashboard URL"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("AlertTopicArn"), &awscdk.CfnOutputProps{
		Value:       alertTopic.TopicArn(),
		Description: jsii.String("SNS Topic for Alerts"),
	})

	// The stack dashboard URL is already output above

	// Output GSI information
	awscdk.NewCfnOutput(stack, jsii.String("GSIPatterns"), &awscdk.CfnOutputProps{
		Value: jsii.String("gsi-tenant-entity, gsi-tenant-timeseries, gsi-tenant-status"),
		Description: jsii.String("Available Global Secondary Indexes"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("AccessPatterns"), &awscdk.CfnOutputProps{
		Value: jsii.String("PK=tenant#{id} SK=user#{id}|project#{id}, GSI queries by tenant_id"),
		Description: jsii.String("DynamORM Access Patterns"),
	})

	return &DynamORMMultiTenantStack{
		Stack: stack,
	}
}

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)

	NewDynamORMMultiTenantStack(app, "DynamORMMultiTenantStack", &DynamORMMultiTenantStackProps{
		StackProps: awscdk.StackProps{
			Env: env(),
		},
	})

	app.Synth(nil)
}

// env determines the AWS environment (account+region) in which our stack is to
// be deployed. For more information see: https://docs.aws.amazon.com/cdk/latest/guide/environments.html
func env() *awscdk.Environment {
	return nil
	// If you know exactly what account and region you want to deploy the stack to,
	// you can uncomment the return statement below and specify the exact values.
	//
	// return &awscdk.Environment{
	//  Account: jsii.String("123456789012"),
	//  Region:  jsii.String("us-east-1"),
	// }
}