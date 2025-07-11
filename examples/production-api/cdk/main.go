package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
	"github.com/pay-theory/lift/pkg/cdk/patterns"
)

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)

	NewProductionApiStack(app, "ProductionApiStack", &awscdk.StackProps{
		Env: env(),
	})

	app.Synth(nil)
}

func env() *awscdk.Environment {
	// For production, specify your account and region
	// return &awscdk.Environment{
	//     Account: jsii.String("123456789012"),
	//     Region:  jsii.String("us-east-1"),
	// }
	return nil
}

func NewProductionApiStack(scope constructs.Construct, id string, props *awscdk.StackProps) awscdk.Stack {
	stack := awscdk.NewStack(scope, &id, props)

	// Create production-grade Lift application
	app := patterns.NewLiftApp(stack, jsii.String("ProductionApi"), &patterns.LiftAppProps{
		AppName:             jsii.String("production-api"),
		CodeAssetPath:       jsii.String("../dist"),
		EnableDatabase:      jsii.Bool(true),
		EnableRateLimiting:  jsii.Bool(true),
		EnableMultiTenant:   jsii.Bool(true),
		EnableAccessLogging: jsii.Bool(true),
		MemorySize:          jsii.Number(1024),
		Timeout:             jsii.Number(300),
		Environment: &map[string]*string{
			"LOG_LEVEL":   jsii.String("info"),
			"ENVIRONMENT": jsii.String("production"),
			"APP_NAME":    jsii.String("production-api"),
		},
		// Uncomment for custom domain
		// DomainName:     jsii.String("api.production.com"),
		// CertificateArn: jsii.String("arn:aws:acm:..."),
	})

	// Add additional production configurations

	// Configure auto-scaling for the DynamoDB table
	if app.Database != nil {
		// Read capacity auto-scaling
		readScaling := app.Database.Table.AutoScaleReadCapacity(&awsdynamodb.EnableScalingProps{
			MinCapacity: jsii.Number(5),
			MaxCapacity: jsii.Number(1000),
		})
		readScaling.ScaleOnUtilization(&awsdynamodb.UtilizationScalingProps{
			TargetUtilizationPercent: jsii.Number(70),
		})

		// Write capacity auto-scaling
		writeScaling := app.Database.Table.AutoScaleWriteCapacity(&awsdynamodb.EnableScalingProps{
			MinCapacity: jsii.Number(5),
			MaxCapacity: jsii.Number(1000),
		})
		writeScaling.ScaleOnUtilization(&awsdynamodb.UtilizationScalingProps{
			TargetUtilizationPercent: jsii.Number(70),
		})
	}

	// Configure API throttling
	stage := app.API.HttpAPI.DefaultStage()
	if stage != nil {
		stage.ApplyRemovalPolicy(awscdk.RemovalPolicy_RETAIN)
	}

	// Add CloudWatch alarms
	NewLiftAlarms(stack, jsii.String("Alarms"), &LiftAlarmsProps{
		Function:   app.Function.Function,
		Api:        app.API.HttpAPI,
		Table:      app.Database.Table,
		AlarmEmail: jsii.String("alerts@example.com"), // Change this
	})

	// Outputs
	awscdk.NewCfnOutput(stack, jsii.String("ApiEndpoint"), &awscdk.CfnOutputProps{
		Value:       app.API.HttpAPI.ApiEndpoint(),
		Description: jsii.String("Production API endpoint"),
		ExportName:  jsii.String("production-api-endpoint"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("FunctionArn"), &awscdk.CfnOutputProps{
		Value:       app.Function.Function.FunctionArn(),
		Description: jsii.String("Lambda function ARN"),
		ExportName:  jsii.String("production-api-function-arn"),
	})

	if app.Database != nil {
		awscdk.NewCfnOutput(stack, jsii.String("TableName"), &awscdk.CfnOutputProps{
			Value:       app.Database.Table.TableName(),
			Description: jsii.String("DynamoDB table name"),
			ExportName:  jsii.String("production-api-table"),
		})
	}

	return stack
}

// LiftAlarms creates CloudWatch alarms for monitoring
type LiftAlarmsProps struct {
	Function   awslambda.Function
	Api        awsapigatewayv2.HttpApi
	Table      awsdynamodb.Table
	AlarmEmail *string
}

type LiftAlarms struct {
	constructs.Construct
}

func NewLiftAlarms(scope constructs.Construct, id *string, props *LiftAlarmsProps) *LiftAlarms {
	this := constructs.NewConstruct(scope, id)

	// Note: In a real implementation, you would create CloudWatch alarms here
	// using awscloudwatch.NewAlarm() for function errors, API 4xx/5xx rates,
	// DynamoDB throttles, etc.

	return &LiftAlarms{
		Construct: this,
	}
}
