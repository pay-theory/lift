package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigateway"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
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

	// Create DynamORM table with multi-tenant support
	table := liftconstructs.NewDynamORMTable(stack, jsii.String("MultiTenantTable"), &liftconstructs.DynamORMTableProps{
		TableName: jsii.String("DynamORMMultiTenantTable"),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("sk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		EnableMultiTenant:   jsii.Bool(true),
		TenantAttribute:     jsii.String("tenant_id"),
		EnableVersioning:    jsii.Bool(true),
		EnableTimestamps:    jsii.Bool(true),
		TimeToLiveAttribute: jsii.String("ttl"),
		RemovalPolicy:       awscdk.RemovalPolicy_RETAIN,
	})

	// Configure multi-tenant GSIs
	table.AddTenantEntityGSI("tenant_id", "entity_type")
	table.AddTenantTimeSeriesGSI("tenant_id", "created_at")
	table.AddTenantStatusGSI("status", "tenant_id")

	// Configure comprehensive monitoring
	monitoringComponents := table.SetupComprehensiveMonitoring(
		alertTopic.TopicArn(),
		"DynamORMMultiTenantDashboard",
	)

	// Enable X-Ray tracing
	table.ConfigureComprehensiveXRayTracing("DynamORMMultiTenant", false)

	// Create Lambda function with DynamORM support
	lambdaFunction := awslambda.NewFunction(stack, jsii.String("MultiTenantFunction"), &awslambda.FunctionProps{
		Runtime: awslambda.Runtime_PROVIDED_AL2023(),
		Handler: jsii.String("bootstrap"),
		Code:    awslambda.Code_FromAsset(jsii.String("../"), nil),
		Environment: &map[string]*string{
			"DYNAMODB_TABLE_NAME": table.GetTableName(),
			"AWS_REGION":          stack.Region(),
		},
		Tracing: awslambda.Tracing_ACTIVE,
		Timeout: awscdk.Duration_Seconds(jsii.Number(30)),
	})

	// Grant DynamORM permissions to Lambda
	table.AddDynamORMPermissions(lambdaFunction)
	table.AddXRayPermissions(lambdaFunction)

	// Create tenant-specific IAM role for demonstration
	tenantRole := awsiam.NewRole(stack, jsii.String("TenantRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("lambda.amazonaws.com"), nil),
		ManagedPolicies: &[]awsiam.IManagedPolicy{
			awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("service-role/AWSLambdaBasicExecutionRole")),
		},
	})

	// Attach tenant boundary policy
	table.AttachTenantBoundaryPolicy(tenantRole, "tenant_id")

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
		Value:       table.GetTableName(),
		Description: jsii.String("DynamORM Multi-Tenant Table Name"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("TableArn"), &awscdk.CfnOutputProps{
		Value:       table.GetTableArn(),
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

	// Output monitoring information
	if dashboard, exists := monitoringComponents["dashboard"]; exists {
		if dashboardRef, ok := dashboard.(awscloudwatch.Dashboard); ok {
			awscdk.NewCfnOutput(stack, jsii.String("DynamORMDashboardURL"), &awscdk.CfnOutputProps{
				Value: jsii.String("https://console.aws.amazon.com/cloudwatch/home?region=" + 
					*stack.Region() + "#dashboards:name=" + *dashboardRef.DashboardName()),
				Description: jsii.String("DynamORM Table Dashboard URL"),
			})
		}
	}

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