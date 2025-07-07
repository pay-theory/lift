package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
	liftconstructs "github.com/pay-theory/lift/pkg/cdk/constructs"
)

type DynamORMStreamProcessingStackProps struct {
	awscdk.StackProps
}

type DynamORMStreamProcessingStack struct {
	awscdk.Stack
}

func NewDynamORMStreamProcessingStack(scope constructs.Construct, id string, props *DynamORMStreamProcessingStackProps) *DynamORMStreamProcessingStack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	// Create a DynamORM table with streaming enabled
	userTable := liftconstructs.NewDynamORMTable(stack, jsii.String("UserTable"), &liftconstructs.DynamORMTableProps{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("sk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		TableName:            jsii.String("users"),
		EnableMultiTenant:    jsii.Bool(true),
		TenantAttribute:      jsii.String("tenant_id"),
		EnableVersioning:     jsii.Bool(true),
		EnableTimestamps:     jsii.Bool(true),
		Stream:              awsdynamodb.StreamViewType_NEW_AND_OLD_IMAGES,
		PointInTimeRecovery: jsii.Bool(true),
		Tags: &map[string]*string{
			"Environment": jsii.String("production"),
			"Service":     jsii.String("user-management"),
		},
	})

	// Configure the table for DynamORM patterns
	userTable.ConfigureForDynamORM()

	// Add GSIs for common query patterns
	userTable.AddDynamORMIndex("email", 
		&awsdynamodb.Attribute{
			Name: jsii.String("email"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		&awsdynamodb.Attribute{
			Name: jsii.String("created_at"),
			Type: awsdynamodb.AttributeType_STRING,
		},
	)

	// Create a stream processor for user events
	userStreamProcessor := liftconstructs.NewDynamORMStreamProcessor(stack, jsii.String("UserStreamProcessor"), &liftconstructs.DynamORMStreamProcessorProps{
		DynamORMTable: userTable,
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String("user-stream-processor"),
			Code:         awslambda.Code_FromAsset(jsii.String("./lambda"), nil),
			Handler:      jsii.String("user-stream-handler"),
			Runtime:      awslambda.Runtime_PROVIDED_AL2(),
			Architecture: awslambda.Architecture_ARM_64(),
			Timeout:      awscdk.Duration_Minutes(jsii.Number(5)),
			MemorySize:   jsii.Number(512),
			Environment: &map[string]*string{
				"LOG_LEVEL": jsii.String("INFO"),
			},
		},
		// Streaming configuration
		BatchSize:               jsii.Number(25),
		MaxBatchingWindow:       awscdk.Duration_Seconds(jsii.Number(5)),
		StartingPosition:        awslambda.StartingPosition_LATEST,
		ParallelizationFactor:   jsii.Number(2),
		ReportBatchItemFailures: jsii.Bool(true),
		ProcessingMode:          liftconstructs.StreamProcessingMode_SEQUENTIAL,
		
		// Event filtering
		EventFilters: []liftconstructs.StreamEventFilter{
			{
				EventName: jsii.String("INSERT"),
				AttributeFilters: map[string]string{
					"entity_type": "User",
				},
			},
			{
				EventName: jsii.String("MODIFY"),
				AttributeFilters: map[string]string{
					"entity_type": "User",
					"status": "active",
				},
			},
			{
				EventName: jsii.String("REMOVE"),
			},
		},
		
		// Multi-tenant configuration
		EnableMultiTenant: jsii.Bool(true),
		TenantAttribute:   jsii.String("tenant_id"),
		
		// Monitoring and observability
		EnableMonitoring:        jsii.Bool(true),
		EnableTracing:          jsii.Bool(true),
		EnableMetricsCollection: jsii.Bool(true),
		CustomMetrics:          []string{"UserCreated", "UserUpdated", "UserDeleted", "ProcessingLatency"},
		
		// Dead letter queue
		EnableDeadLetterQueue: jsii.Bool(true),
	})

	// Set up comprehensive monitoring with X-Ray tracing
	userStreamProcessor.SetupComprehensiveStreamProcessing("UserService", true, []string{
		"ValidationErrors",
		"EnrichmentFailures", 
		"NotificationsSent",
	})

	// Configure multi-tenant streaming patterns
	userStreamProcessor.ConfigureMultiTenantStreaming("tenant_id")

	// Create another stream processor for analytics
	analyticsStreamProcessor := liftconstructs.NewDynamORMStreamProcessor(stack, jsii.String("AnalyticsStreamProcessor"), &liftconstructs.DynamORMStreamProcessorProps{
		DynamORMTable: userTable,
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String("user-analytics-processor"),
			Code:         awslambda.Code_FromAsset(jsii.String("./analytics-lambda"), nil),
			Handler:      jsii.String("analytics-handler"),
			Runtime:      awslambda.Runtime_PROVIDED_AL2(),
			Architecture: awslambda.Architecture_ARM_64(),
			Timeout:      awscdk.Duration_Minutes(jsii.Number(10)),
			MemorySize:   jsii.Number(1024),
			Environment: &map[string]*string{
				"LOG_LEVEL":      jsii.String("INFO"),
				"ANALYTICS_MODE": jsii.String("realtime"),
			},
		},
		// Different configuration for analytics
		BatchSize:             jsii.Number(100),  // Larger batches for analytics
		MaxBatchingWindow:     awscdk.Duration_Seconds(jsii.Number(30)),
		ParallelizationFactor: jsii.Number(4),   // Higher parallelization
		ProcessingMode:        liftconstructs.StreamProcessingMode_PARALLEL,
		
		// Filter only for specific events
		EventFilters: []liftconstructs.StreamEventFilter{
			{
				EventName: jsii.String("INSERT"),
			},
			{
				EventName: jsii.String("MODIFY"),
				AttributeFilters: map[string]string{
					"status": "active",
				},
			},
		},
		
		EnableMonitoring: jsii.Bool(true),
		CustomMetrics:   []string{"AnalyticsProcessed", "DataPoints", "AggregationsComputed"},
	})

	// Create a notification stream processor for critical events
	notificationStreamProcessor := liftconstructs.NewDynamORMStreamProcessor(stack, jsii.String("NotificationStreamProcessor"), &liftconstructs.DynamORMStreamProcessorProps{
		DynamORMTable: userTable,
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String("user-notification-processor"),
			Code:         awslambda.Code_FromAsset(jsii.String("./notification-lambda"), nil),
			Handler:      jsii.String("notification-handler"),
			Runtime:      awslambda.Runtime_PROVIDED_AL2(),
			Architecture: awslambda.Architecture_ARM_64(),
			Timeout:      awscdk.Duration_Minutes(jsii.Number(2)),
			MemorySize:   jsii.Number(256),
		},
		// Fast processing for notifications
		BatchSize:         jsii.Number(1),  // Process one at a time for speed
		MaxBatchingWindow: awscdk.Duration_Seconds(jsii.Number(1)),
		ProcessingMode:    liftconstructs.StreamProcessingMode_SEQUENTIAL,
		
		// Only process specific high-priority events
		EventFilters: []liftconstructs.StreamEventFilter{
			{
				EventName: jsii.String("INSERT"),
				AttributeFilters: map[string]string{
					"entity_type": "User",
					"priority":    "high",
				},
			},
			{
				EventName: jsii.String("MODIFY"),
				AttributeFilters: map[string]string{
					"entity_type": "User",
					"status":      "suspended",
				},
			},
		},
		
		EnableMonitoring: jsii.Bool(true),
		CustomMetrics:   []string{"NotificationsSent", "NotificationFailures"},
	})

	// Add environment variables to processors
	userStreamProcessor.AddEnvironmentVariable("PROCESSOR_TYPE", "user-events")
	analyticsStreamProcessor.AddEnvironmentVariable("PROCESSOR_TYPE", "analytics")
	notificationStreamProcessor.AddEnvironmentVariable("PROCESSOR_TYPE", "notifications")

	// Grant additional permissions if needed
	// Example: Grant access to external services
	// userStreamProcessor.GrantDynamORMAccess(externalFunction)

	return &DynamORMStreamProcessingStack{
		Stack: stack,
	}
}

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)

	NewDynamORMStreamProcessingStack(app, "DynamORMStreamProcessingStack", &DynamORMStreamProcessingStackProps{
		awscdk.StackProps{
			Env: env(),
		},
	})

	app.Synth(nil)
}

// env determines the AWS environment (account+region) in which our stack is to
// be deployed. For more information see: https://docs.aws.amazon.com/cdk/latest/guide/environments.html
func env() *awscdk.Environment {
	return nil
}