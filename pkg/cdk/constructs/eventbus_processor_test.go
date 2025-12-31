package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/jsii-runtime-go"
	"github.com/stretchr/testify/assert"
)

func TestEventBusProcessor_BasicCreation(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	processor := NewEventBusProcessor(stack, jsii.String("TestEventBusProcessor"), &EventBusProcessorProps{
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String("test-eventbus-processor"),
			Code:         awslambda.Code_FromInline(jsii.String("exports.handler = async () => {};")),
			Handler:      jsii.String("index.handler"),
			Runtime:      awslambda.Runtime_NODEJS_18_X(),
		},
		EventTypes: []string{"partner.created"},
	})

	assert.NotNil(t, processor)
	assert.NotNil(t, processor.Table)
	assert.NotNil(t, processor.Function)
	assert.NotNil(t, processor.EventSource)
	assert.NotNil(t, processor.DeadLetterQueue)

	template := synthesizeTemplate(t, stack)
	assertResourceExists(t, template, "AWS::Lambda::EventSourceMapping")
	assertResourceExists(t, template, "AWS::SQS::Queue")

	// Verify filter criteria exists on the event source mapping when event types are provided.
	mappings := findResourcesByType(template, "AWS::Lambda::EventSourceMapping")
	foundFilter := false
	for _, mapping := range mappings {
		props, ok := mapping["Properties"].(map[string]interface{})
		if !ok {
			continue
		}
		if _, ok := props["FilterCriteria"]; ok {
			foundFilter = true
			break
		}
	}
	assert.True(t, foundFilter, "expected FilterCriteria on event source mapping")
}

func TestEventBusProcessor_DisabledDLQ(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	processor := NewEventBusProcessor(stack, jsii.String("TestEventBusProcessor"), &EventBusProcessorProps{
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String("test-eventbus-processor-no-dlq"),
			Code:         awslambda.Code_FromInline(jsii.String("exports.handler = async () => {};")),
			Handler:      jsii.String("index.handler"),
			Runtime:      awslambda.Runtime_NODEJS_18_X(),
		},
		EnableDeadLetterQueue: jsii.Bool(false),
		EventTypes:            []string{"partner.created"},
	})

	assert.NotNil(t, processor)
	assert.Nil(t, processor.DeadLetterQueue)

	template := synthesizeTemplate(t, stack)
	assertResourceCount(t, template, "AWS::SQS::Queue", 0)
}
