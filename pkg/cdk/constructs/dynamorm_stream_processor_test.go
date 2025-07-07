package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/jsii-runtime-go"
)

func TestNewDynamORMStreamProcessor(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create a DynamORM table
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("sk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		TableName:         jsii.String("test-table"),
		EnableMultiTenant: jsii.Bool(true),
		Stream:           awsdynamodb.StreamViewType_NEW_AND_OLD_IMAGES,
	})

	// Create stream processor
	processor := NewDynamORMStreamProcessor(stack, jsii.String("TestStreamProcessor"), &DynamORMStreamProcessorProps{
		DynamORMTable: table,
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String("test-stream-processor"),
			Code:         awslambda.Code_FromInline(jsii.String("exports.handler = async (event) => console.log(event);")),
			Handler:      jsii.String("index.handler"),
			Runtime:      awslambda.Runtime_NODEJS_18_X(),
		},
		EnableMonitoring:  jsii.Bool(true),
		EnableTracing:     jsii.Bool(true),
		EnableMultiTenant: jsii.Bool(true),
		BatchSize:         jsii.Number(25),
		ProcessingMode:    StreamProcessingMode_SEQUENTIAL,
	})

	// Test that the processor was created successfully
	if processor == nil {
		t.Fatal("DynamORMStreamProcessor should not be nil")
	}

	// Test that the function was created
	if processor.Function == nil {
		t.Fatal("Function should not be nil")
	}

	// Test that the table reference is correct
	if processor.Table != table {
		t.Fatal("Table reference should match the provided table")
	}

	// Test that DLQ was created by default
	if processor.DeadLetterQueue == nil {
		t.Fatal("DeadLetterQueue should be created by default")
	}

	// Test that function has been configured with environment variables
	// Note: In a real test, we would need to access the environment variables differently
	// since they are not directly accessible from the CDK construct

	// Synth the stack to validate
	app.Synth(nil)
}

func TestDynamORMStreamProcessorWithCustomProps(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create a DynamORM table
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("sk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		TableName: jsii.String("test-table"),
		Stream:   awsdynamodb.StreamViewType_NEW_AND_OLD_IMAGES,
	})

	// Create stream processor with custom props
	processor := NewDynamORMStreamProcessor(stack, jsii.String("TestStreamProcessor"), &DynamORMStreamProcessorProps{
		DynamORMTable: table,
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String("test-stream-processor"),
			Code:         awslambda.Code_FromInline(jsii.String("exports.handler = async (event) => console.log(event);")),
			Handler:      jsii.String("index.handler"),
			Runtime:      awslambda.Runtime_NODEJS_18_X(),
		},
		EnableDeadLetterQueue: jsii.Bool(false),
		BatchSize:             jsii.Number(50),
		MaxBatchingWindow:     awscdk.Duration_Seconds(jsii.Number(10)),
		StartingPosition:      awslambda.StartingPosition_TRIM_HORIZON,
		ParallelizationFactor: jsii.Number(2),
		ProcessingMode:        StreamProcessingMode_PARALLEL,
		EventFilters: []StreamEventFilter{
			{
				EventName: jsii.String("INSERT"),
				AttributeFilters: map[string]string{
					"entity_type": "User",
				},
			},
			{
				EventName:    jsii.String("MODIFY"),
				TenantFilter: jsii.String("tenant-123"),
			},
		},
		CustomMetrics: []string{"ProcessedRecords", "FailedRecords"},
	})

	// Test that DLQ was not created
	if processor.DeadLetterQueue != nil {
		t.Fatal("DeadLetterQueue should not be created when disabled")
	}

	// Test that the processor was created successfully
	if processor == nil {
		t.Fatal("DynamORMStreamProcessor should not be nil")
	}

	// Synth the stack to validate
	app.Synth(nil)
}

func TestDynamORMStreamProcessorMultiTenant(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create a multi-tenant DynamORM table
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("sk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		TableName:         jsii.String("test-multi-tenant-table"),
		EnableMultiTenant: jsii.Bool(true),
		TenantAttribute:   jsii.String("tenant_id"),
		Stream:           awsdynamodb.StreamViewType_NEW_AND_OLD_IMAGES,
	})

	// Create stream processor with multi-tenant settings
	processor := NewDynamORMStreamProcessor(stack, jsii.String("TestStreamProcessor"), &DynamORMStreamProcessorProps{
		DynamORMTable: table,
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String("test-multi-tenant-stream-processor"),
			Code:         awslambda.Code_FromInline(jsii.String("exports.handler = async (event) => console.log(event);")),
			Handler:      jsii.String("index.handler"),
			Runtime:      awslambda.Runtime_NODEJS_18_X(),
		},
		EnableMultiTenant: jsii.Bool(true),
		TenantAttribute:   jsii.String("tenant_id"),
		EventFilters: []StreamEventFilter{
			{
				TenantFilter: jsii.String("tenant-123"),
			},
		},
	})

	// Configure multi-tenant streaming
	processor.ConfigureMultiTenantStreaming("tenant_id")

	// Test that the processor was created successfully
	if processor == nil {
		t.Fatal("DynamORMStreamProcessor should not be nil")
	}

	// Synth the stack to validate
	app.Synth(nil)
}

func TestDynamORMStreamProcessorPermissions(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create a DynamORM table
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("sk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		TableName: jsii.String("test-table"),
		Stream:   awsdynamodb.StreamViewType_NEW_AND_OLD_IMAGES,
	})

	// Create stream processor
	processor := NewDynamORMStreamProcessor(stack, jsii.String("TestStreamProcessor"), &DynamORMStreamProcessorProps{
		DynamORMTable: table,
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String("test-stream-processor"),
			Code:         awslambda.Code_FromInline(jsii.String("exports.handler = async (event) => console.log(event);")),
			Handler:      jsii.String("index.handler"),
			Runtime:      awslambda.Runtime_NODEJS_18_X(),
		},
	})

	// Create another function to test permissions
	testFunction := awslambda.NewFunction(stack, jsii.String("TestFunction"), &awslambda.FunctionProps{
		FunctionName: jsii.String("test-function"),
		Code:         awslambda.Code_FromInline(jsii.String("exports.handler = async () => {};")),
		Handler:      jsii.String("index.handler"),
		Runtime:      awslambda.Runtime_NODEJS_18_X(),
	})

	// Grant permissions
	processor.GrantDynamORMAccess(testFunction)
	processor.GrantStreamRead(testFunction)

	// Test that the processor was created successfully
	if processor == nil {
		t.Fatal("DynamORMStreamProcessor should not be nil")
	}

	// Synth the stack to validate
	app.Synth(nil)
}

func TestDynamORMStreamProcessorMonitoring(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create a DynamORM table
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("sk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		TableName: jsii.String("test-table"),
		Stream:   awsdynamodb.StreamViewType_NEW_AND_OLD_IMAGES,
	})

	// Create stream processor with monitoring enabled
	processor := NewDynamORMStreamProcessor(stack, jsii.String("TestStreamProcessor"), &DynamORMStreamProcessorProps{
		DynamORMTable: table,
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String("test-stream-processor"),
			Code:         awslambda.Code_FromInline(jsii.String("exports.handler = async (event) => console.log(event);")),
			Handler:      jsii.String("index.handler"),
			Runtime:      awslambda.Runtime_NODEJS_18_X(),
		},
		EnableMonitoring:        jsii.Bool(true),
		EnableMetricsCollection: jsii.Bool(true),
		CustomMetrics:          []string{"ProcessedEvents", "FilteredEvents"},
	})

	// Set up comprehensive monitoring
	processor.SetupComprehensiveStreamProcessing("DynamORMStreamService", true, []string{"CustomMetric1", "CustomMetric2"})

	// Test that the processor was created successfully
	if processor == nil {
		t.Fatal("DynamORMStreamProcessor should not be nil")
	}

	// Test that alert topic was created
	if processor.AlertTopic == nil {
		t.Fatal("AlertTopic should be created when monitoring is enabled")
	}

	// Synth the stack to validate
	app.Synth(nil)
}

func TestDynamORMStreamProcessorNilProps(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Test that creating with nil props panics
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Expected panic when props is nil")
		}
	}()

	NewDynamORMStreamProcessor(stack, jsii.String("TestStreamProcessor"), nil)
}

func TestDynamORMStreamProcessorNilTable(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Test that creating with nil table panics
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Expected panic when DynamORMTable is nil")
		}
	}()

	NewDynamORMStreamProcessor(stack, jsii.String("TestStreamProcessor"), &DynamORMStreamProcessorProps{
		DynamORMTable: nil,
	})
}

func TestDynamORMStreamProcessorGetters(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create a DynamORM table
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("sk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		TableName: jsii.String("test-table"),
		Stream:   awsdynamodb.StreamViewType_NEW_AND_OLD_IMAGES,
	})

	// Create stream processor
	processor := NewDynamORMStreamProcessor(stack, jsii.String("TestStreamProcessor"), &DynamORMStreamProcessorProps{
		DynamORMTable: table,
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String("test-stream-processor"),
			Code:         awslambda.Code_FromInline(jsii.String("exports.handler = async (event) => console.log(event);")),
			Handler:      jsii.String("index.handler"),
			Runtime:      awslambda.Runtime_NODEJS_18_X(),
		},
	})

	// Test getters
	if processor.GetTableName() == nil {
		t.Fatal("GetTableName should not return nil")
	}

	if processor.GetTableArn() == nil {
		t.Fatal("GetTableArn should not return nil")
	}

	if processor.GetStreamArn() == nil {
		t.Fatal("GetStreamArn should not return nil")
	}

	if processor.GetFunction() == nil {
		t.Fatal("GetFunction should not return nil")
	}

	if processor.GetDynamORMTable() != table {
		t.Fatal("GetDynamORMTable should return the correct table")
	}

	if processor.GetDeadLetterQueueUrl() == nil {
		t.Fatal("GetDeadLetterQueueUrl should not return nil when DLQ is enabled")
	}

	// Synth the stack to validate
	app.Synth(nil)
}