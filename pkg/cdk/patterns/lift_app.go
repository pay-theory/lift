package patterns

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
	liftconstructs "github.com/pay-theory/lift/pkg/cdk/constructs"
)

// LiftAppProps defines properties for a complete Lift application
type LiftAppProps struct {
	// Application name
	AppName *string
	// Lambda code asset path
	CodeAssetPath *string
	// Enable multi-tenant support
	EnableMultiTenant *bool
	// Enable API Gateway access logging
	EnableAccessLogging *bool
	// Custom domain configuration
	DomainName      *string
	CertificateArn  *string
	// Environment variables for Lambda
	Environment *map[string]*string
	// Memory size for Lambda function
	MemorySize *float64
	// Timeout for Lambda function
	Timeout *float64
	// Enable DynamoDB table
	EnableDatabase *bool
	// Database table name
	DatabaseTableName *string
	// Enable rate limiting table
	EnableRateLimiting *bool
	// Rate limiting table name
	RateLimitTableName *string
	// Enable idempotency
	EnableIdempotency *bool
}

// LiftApp is a complete Lift application pattern with API Gateway, Lambda, and DynamoDB
type LiftApp struct {
	constructs.Construct
	API      *liftconstructs.LiftAPI
	Function *liftconstructs.LiftFunction
	Database *liftconstructs.LiftTable
	RateLimitTable *liftconstructs.LiftTable
}

// NewLiftApp creates a complete Lift application stack
func NewLiftApp(scope constructs.Construct, id *string, props *LiftAppProps) *LiftApp {
	this := constructs.NewConstruct(scope, id)

	app := &LiftApp{
		Construct: this,
	}

	// Prepare environment variables
	env := make(map[string]*string)
	if props.Environment != nil {
		for k, v := range *props.Environment {
			env[k] = v
		}
	}
	
	// Add database table name if enabled
	if props.EnableDatabase != nil && *props.EnableDatabase {
		tableName := props.DatabaseTableName
		if tableName == nil {
			tableName = jsii.String(*props.AppName + "-table")
		}
		env["DYNAMODB_TABLE"] = tableName
	}
	
	// Add rate limit table name if enabled
	if props.EnableRateLimiting != nil && *props.EnableRateLimiting {
		tableName := props.RateLimitTableName
		if tableName == nil {
			tableName = jsii.String(*props.AppName + "-rate-limits")
		}
		env["RATE_LIMIT_TABLE"] = tableName
	}

	// Create Lambda function
	fnProps := &liftconstructs.LiftFunctionProps{
		FunctionProps: awslambda.FunctionProps{
			FunctionName: props.AppName,
			Code:         awslambda.Code_FromAsset(props.CodeAssetPath, nil),
			Handler:      jsii.String("bootstrap"),
			Environment:  &env,
			MemorySize:   props.MemorySize,
			Timeout:      func() awscdk.Duration { if props.Timeout != nil { return awscdk.Duration_Seconds(props.Timeout) }; return awscdk.Duration_Seconds(jsii.Number(30)) }(),
		},
		EnableTracing:     jsii.Bool(true),
		EnableMultiTenant: props.EnableMultiTenant,
	}

	app.Function = liftconstructs.NewLiftFunction(this, jsii.String("Function"), fnProps)

	// Create DynamoDB table if enabled
	if props.EnableDatabase != nil && *props.EnableDatabase {
		tableName := props.DatabaseTableName
		if tableName == nil {
			tableName = jsii.String(*props.AppName + "-table")
		}

		app.Database = liftconstructs.NewLiftTable(this, jsii.String("Database"), &liftconstructs.LiftTableProps{
			TableName:                 tableName,
			EnablePointInTimeRecovery: jsii.Bool(true),
			EnableStreams:             jsii.Bool(true),
			TimeToLiveAttribute:       jsii.String("ttl"),
			EnableAutoScaling:         jsii.Bool(true),
		})

		// Grant Lambda permissions to access the table
		app.Database.Table.GrantReadWriteData(app.Function.Function)
	}

	// Create rate limiting table if enabled
	if props.EnableRateLimiting != nil && *props.EnableRateLimiting {
		tableName := props.RateLimitTableName
		if tableName == nil {
			tableName = jsii.String(*props.AppName + "-rate-limits")
		}

		app.RateLimitTable = liftconstructs.NewLiftTable(this, jsii.String("RateLimitTable"), &liftconstructs.LiftTableProps{
			TableName:           tableName,
			TimeToLiveAttribute: jsii.String("expires"),
		})

		// Grant Lambda permissions
		app.RateLimitTable.Table.GrantReadWriteData(app.Function.Function)
	}

	// Create API Gateway
	apiProps := &liftconstructs.LiftAPIProps{
		Name:                jsii.String(*props.AppName + "-api"),
		Description:         jsii.String("API Gateway for " + *props.AppName),
		EnableCORS:          jsii.Bool(true),
		EnableAccessLogging: props.EnableAccessLogging,
		DomainName:          props.DomainName,
		CertificateArn:      props.CertificateArn,
	}

	app.API = liftconstructs.NewLiftAPI(this, jsii.String("API"), apiProps)

	// Add catch-all route to Lambda
	app.API.AddLambdaRoute(
		jsii.String("/{proxy+}"),
		awsapigatewayv2.HttpMethod_ANY,
		app.Function.Function,
	)

	// Also add root route
	app.API.AddLambdaRoute(
		jsii.String("/"),
		awsapigatewayv2.HttpMethod_ANY,
		app.Function.Function,
	)

	// Output important values (using scope instead of this so they appear in the stack)
	stack := awscdk.Stack_Of(this)
	awscdk.NewCfnOutput(stack, jsii.String("ApiUrl"), &awscdk.CfnOutputProps{
		Value:       app.API.GetUrl(),
		Description: jsii.String("API Gateway endpoint URL"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("FunctionName"), &awscdk.CfnOutputProps{
		Value:       app.Function.Function.FunctionName(),
		Description: jsii.String("Lambda function name"),
	})

	if app.Database != nil {
		awscdk.NewCfnOutput(stack, jsii.String("DatabaseTableName"), &awscdk.CfnOutputProps{
			Value:       app.Database.Table.TableName(),
			Description: jsii.String("DynamoDB table name"),
		})
	}

	return app
}