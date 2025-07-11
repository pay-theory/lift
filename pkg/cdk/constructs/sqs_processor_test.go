package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambdaeventsources"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssqs"
	"github.com/aws/jsii-runtime-go"
)

// Test helper functions
func synthesizeTemplate(stack awscdk.Stack) assertions.Template {
	return assertions.Template_FromStack(stack, nil)
}

func assertResourceExists(_ *testing.T, template assertions.Template, resourceType string, props map[string]interface{}) {
	template.HasResourceProperties(jsii.String(resourceType), &props)
}

func findResourcesByType(template assertions.Template, resourceType string) []map[string]interface{} {
	templateJSON := template.ToJSON()
	resources := (*templateJSON)["Resources"].(map[string]interface{})

	var found []map[string]interface{}
	for _, resource := range resources {
		if resMap, ok := resource.(map[string]interface{}); ok {
			if resType, ok := resMap["Type"].(string); ok && resType == resourceType {
				found = append(found, resMap)
			}
		}
	}
	return found
}

func TestSQSProcessor_DefaultConfiguration(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	processor := NewSQSProcessor(stack, jsii.String("TestSQSProcessor"), &SQSProcessorProps{
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String("test-sqs-processor"),
			Code:         awslambda.Code_FromInline(jsii.String("exports.handler = async () => {}")),
			Handler:      jsii.String("index.handler"),
			Runtime:      awslambda.Runtime_NODEJS_18_X(),
		},
	})

	// Test that all components are created
	if processor.Function == nil {
		t.Error("Function should be created")
	}
	if processor.Queue == nil {
		t.Error("Queue should be created")
	}
	if processor.DeadLetterQueue == nil {
		t.Error("Dead letter queue should be created by default")
	}
	if processor.EventSource == nil {
		t.Error("Event source should be created")
	}

	// Synthesize to verify template
	template := synthesizeTemplate(stack)

	// Verify SQS queue exists
	assertResourceExists(t, template, "AWS::SQS::Queue", map[string]interface{}{
		"QueueName": "test-sqs-processor-queue",
	})

	// Verify dead letter queue exists
	assertResourceExists(t, template, "AWS::SQS::Queue", map[string]interface{}{
		"QueueName": "test-sqs-processor-dlq",
	})

	// Verify Lambda function
	assertResourceExists(t, template, "AWS::Lambda::Function", map[string]interface{}{
		"FunctionName": "test-sqs-processor",
		"Handler":      "index.handler",
	})

	// Verify event source mapping
	assertResourceExists(t, template, "AWS::Lambda::EventSourceMapping", map[string]interface{}{
		"BatchSize":             jsii.Number(10),
		"FunctionResponseTypes": []string{"ReportBatchItemFailures"},
	})
}

func TestSQSProcessor_CustomConfiguration(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	processor := NewSQSProcessor(stack, jsii.String("TestSQSProcessor"), &SQSProcessorProps{
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String("custom-sqs-processor"),
			Code:         awslambda.Code_FromInline(jsii.String("exports.handler = async () => {}")),
			Handler:      jsii.String("index.handler"),
			Runtime:      awslambda.Runtime_NODEJS_18_X(),
		},
		BatchSize:         jsii.Number(20),
		MaxBatchingWindow: awscdk.Duration_Seconds(jsii.Number(10)),
		MaxReceiveCount:   jsii.Number(5),
		VisibilityTimeout: awscdk.Duration_Minutes(jsii.Number(10)),
		QueueProps: &awssqs.QueueProps{
			QueueName: jsii.String("custom-queue-name"),
		},
		EnableTracing:     jsii.Bool(true),
		EnableMultiTenant: jsii.Bool(true),
	})

	if processor == nil {
		t.Fatal("Processor should be created")
	}

	template := synthesizeTemplate(stack)

	// Verify custom configuration is applied
	assertResourceExists(t, template, "AWS::SQS::Queue", map[string]interface{}{
		"QueueName": "custom-queue-name",
	})

	// Verify custom event source configuration
	assertResourceExists(t, template, "AWS::Lambda::EventSourceMapping", map[string]interface{}{
		"BatchSize": jsii.Number(20),
	})
}

func TestSQSProcessor_FIFOQueue(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	processor := NewSQSProcessor(stack, jsii.String("TestSQSProcessor"), &SQSProcessorProps{
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String("fifo-sqs-processor"),
			Code:         awslambda.Code_FromInline(jsii.String("exports.handler = async () => {}")),
			Handler:      jsii.String("index.handler"),
			Runtime:      awslambda.Runtime_NODEJS_18_X(),
		},
		FifoQueue:                       jsii.Bool(true),
		EnableContentBasedDeduplication: jsii.Bool(true),
	})

	if processor == nil {
		t.Fatal("Processor should be created")
	}

	template := synthesizeTemplate(stack)

	// Verify FIFO queue configuration
	assertResourceExists(t, template, "AWS::SQS::Queue", map[string]interface{}{
		"QueueName":                 "fifo-sqs-processor-queue.fifo",
		"FifoQueue":                 true,
		"ContentBasedDeduplication": true,
	})

	// Verify FIFO DLQ configuration
	assertResourceExists(t, template, "AWS::SQS::Queue", map[string]interface{}{
		"QueueName":                 "fifo-sqs-processor-dlq.fifo",
		"FifoQueue":                 true,
		"ContentBasedDeduplication": true,
	})
}

func TestSQSProcessor_ExistingQueue(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create existing queue
	existingQueue := awssqs.NewQueue(stack, jsii.String("ExistingQueue"), &awssqs.QueueProps{
		QueueName: jsii.String("existing-queue"),
	})

	processor := NewSQSProcessor(stack, jsii.String("TestSQSProcessor"), &SQSProcessorProps{
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String("existing-queue-processor"),
			Code:         awslambda.Code_FromInline(jsii.String("exports.handler = async () => {}")),
			Handler:      jsii.String("index.handler"),
			Runtime:      awslambda.Runtime_NODEJS_18_X(),
		},
		ExistingQueue: existingQueue,
	})

	if processor == nil {
		t.Fatal("Processor should be created")
	}

	if processor.Queue != existingQueue {
		t.Error("Should use existing queue")
	}

	template := synthesizeTemplate(stack)

	// Should still create Lambda and event source mapping
	assertResourceExists(t, template, "AWS::Lambda::Function", map[string]interface{}{
		"FunctionName": "existing-queue-processor",
	})

	assertResourceExists(t, template, "AWS::Lambda::EventSourceMapping", map[string]interface{}{})
}

func TestSQSProcessor_DisabledDLQ(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	processor := NewSQSProcessor(stack, jsii.String("TestSQSProcessor"), &SQSProcessorProps{
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String("no-dlq-processor"),
			Code:         awslambda.Code_FromInline(jsii.String("exports.handler = async () => {}")),
			Handler:      jsii.String("index.handler"),
			Runtime:      awslambda.Runtime_NODEJS_18_X(),
		},
		EnableDeadLetterQueue: jsii.Bool(false),
	})

	if processor == nil {
		t.Fatal("Processor should be created")
	}

	if processor.DeadLetterQueue != nil {
		t.Error("Dead letter queue should not be created when disabled")
	}

	template := synthesizeTemplate(stack)

	// Count SQS queues - should only be 1 (main queue, no DLQ)
	queues := findResourcesByType(template, "AWS::SQS::Queue")
	if len(queues) != 1 {
		t.Errorf("Expected 1 SQS queue, got %d", len(queues))
	}
}

func TestSQSProcessor_LongPolling(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	processor := NewSQSProcessor(stack, jsii.String("TestSQSProcessor"), &SQSProcessorProps{
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String("long-polling-processor"),
			Code:         awslambda.Code_FromInline(jsii.String("exports.handler = async () => {}")),
			Handler:      jsii.String("index.handler"),
			Runtime:      awslambda.Runtime_NODEJS_18_X(),
		},
		ReceiveMessageWaitTimeSeconds: jsii.Number(20),
	})

	if processor == nil {
		t.Fatal("Processor should be created")
	}

	template := synthesizeTemplate(stack)

	// Verify long polling configuration
	assertResourceExists(t, template, "AWS::SQS::Queue", map[string]interface{}{
		"ReceiveMessageWaitTimeSeconds": jsii.Number(20),
	})
}

func TestSQSProcessor_EnvironmentVariables(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	processor := NewSQSProcessor(stack, jsii.String("TestSQSProcessor"), &SQSProcessorProps{
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String("env-processor"),
			Code:         awslambda.Code_FromInline(jsii.String("exports.handler = async () => {}")),
			Handler:      jsii.String("index.handler"),
			Runtime:      awslambda.Runtime_NODEJS_18_X(),
			Environment: &map[string]*string{
				"CUSTOM_VAR": jsii.String("custom-value"),
			},
		},
	})

	if processor == nil {
		t.Fatal("Processor should be created")
	}

	template := synthesizeTemplate(stack)

	// Verify environment variables are set (including SQS-specific ones)
	functions := findResourcesByType(template, "AWS::Lambda::Function")
	if len(functions) != 1 {
		t.Fatalf("Expected 1 function, got %d", len(functions))
	}

	function := functions[0]
	props, ok := function["Properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Function should have Properties")
	}

	env, ok := props["Environment"].(map[string]interface{})
	if !ok {
		t.Fatal("Function should have Environment")
	}

	variables, ok := env["Variables"].(map[string]interface{})
	if !ok {
		t.Fatal("Environment should have Variables")
	}

	// Should have custom variable
	if variables["CUSTOM_VAR"] != "custom-value" {
		t.Error("Should preserve custom environment variables")
	}

	// Should have SQS queue URL
	if _, exists := variables["SQS_QUEUE_URL"]; !exists {
		t.Error("Should add SQS_QUEUE_URL environment variable")
	}

	// Should have DLQ URL (DLQ is enabled by default)
	if _, exists := variables["SQS_DLQ_URL"]; !exists {
		t.Error("Should add SQS_DLQ_URL environment variable when DLQ is enabled")
	}
}

func TestSQSProcessor_GrantPermissions(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	processor := NewSQSProcessor(stack, jsii.String("TestSQSProcessor"), &SQSProcessorProps{
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String("permission-processor"),
			Code:         awslambda.Code_FromInline(jsii.String("exports.handler = async () => {}")),
			Handler:      jsii.String("index.handler"),
			Runtime:      awslambda.Runtime_NODEJS_18_X(),
		},
	})

	// Create another function to test permissions
	anotherFunction := awslambda.NewFunction(stack, jsii.String("AnotherFunction"), &awslambda.FunctionProps{
		FunctionName: jsii.String("another-function"),
		Code:         awslambda.Code_FromInline(jsii.String("exports.handler = async () => {}")),
		Handler:      jsii.String("index.handler"),
		Runtime:      awslambda.Runtime_NODEJS_18_X(),
	})

	// Grant permissions
	processor.GrantSendMessages(anotherFunction)
	processor.GrantConsumeMessages(anotherFunction)

	template := synthesizeTemplate(stack)

	// Verify IAM policies are created
	policies := findResourcesByType(template, "AWS::IAM::Policy")
	if len(policies) == 0 {
		t.Error("Should create IAM policies for SQS permissions")
	}
}

func TestSQSProcessor_CustomEventSourceProps(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	processor := NewSQSProcessor(stack, jsii.String("TestSQSProcessor"), &SQSProcessorProps{
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String("custom-event-source-processor"),
			Code:         awslambda.Code_FromInline(jsii.String("exports.handler = async () => {}")),
			Handler:      jsii.String("index.handler"),
			Runtime:      awslambda.Runtime_NODEJS_18_X(),
		},
		EventSourceProps: &awslambdaeventsources.SqsEventSourceProps{
			BatchSize:               jsii.Number(5),
			MaxBatchingWindow:       awscdk.Duration_Seconds(jsii.Number(30)),
			ReportBatchItemFailures: jsii.Bool(false),
			MaxConcurrency:          jsii.Number(10),
		},
	})

	if processor == nil {
		t.Fatal("Processor should be created")
	}

	template := synthesizeTemplate(stack)

	// Verify custom event source configuration
	assertResourceExists(t, template, "AWS::Lambda::EventSourceMapping", map[string]interface{}{
		"BatchSize": jsii.Number(5),
		"ScalingConfig": map[string]interface{}{
			"MaximumConcurrency": jsii.Number(10),
		},
	})
}
