package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/jsii-runtime-go"
	"github.com/pay-theory/lift/pkg/cdk/test"
)

func TestStreamProcessor_CreatesDLQAndEventSourceMapping(t *testing.T) {
	stack := test.NewTestStack()

	table := NewStreamingTable(stack.Stack(), jsii.String("Table"), &StreamingTableProps{
		TableName: jsii.String("streaming-table"),
	})

	NewStreamProcessor(stack.Stack(), jsii.String("Processor"), &StreamProcessorProps{
		StreamingTable: table,
		FunctionProps: awslambda.FunctionProps{
			Runtime: awslambda.Runtime_NODEJS_18_X(),
			Handler: jsii.String("index.handler"),
			Code:    awslambda.Code_FromInline(jsii.String("exports.handler = async () => {};")),
		},
	})

	template := assertions.Template_FromStack(stack.Stack(), nil)
	template.ResourceCountIs(jsii.String("AWS::SQS::Queue"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::Lambda::EventSourceMapping"), jsii.Number(1))

	template.HasResourceProperties(jsii.String("AWS::Lambda::Function"), map[string]interface{}{
		"Environment": map[string]interface{}{
			"Variables": map[string]interface{}{
				"DYNAMODB_STREAM_ARN": assertions.Match_AnyValue(),
				"DYNAMODB_TABLE_NAME": assertions.Match_AnyValue(),
			},
		},
	})
}

func TestStreamProcessor_DisablesDeadLetterQueue(t *testing.T) {
	stack := test.NewTestStack()

	table := NewStreamingTable(stack.Stack(), jsii.String("Table"), &StreamingTableProps{
		TableName:      jsii.String("streaming-table"),
		StreamViewType: awsdynamodb.StreamViewType_NEW_IMAGE,
	})

	NewStreamProcessor(stack.Stack(), jsii.String("Processor"), &StreamProcessorProps{
		StreamingTable:        table,
		EnableDeadLetterQueue: jsii.Bool(false),
		StartingPosition:      awslambda.StartingPosition_TRIM_HORIZON,
		BatchSize:             jsii.Number(25),
		FunctionProps: awslambda.FunctionProps{
			Runtime: awslambda.Runtime_NODEJS_18_X(),
			Handler: jsii.String("index.handler"),
			Code:    awslambda.Code_FromInline(jsii.String("exports.handler = async () => {};")),
		},
	})

	template := assertions.Template_FromStack(stack.Stack(), nil)
	template.ResourceCountIs(jsii.String("AWS::SQS::Queue"), jsii.Number(0))
	template.ResourceCountIs(jsii.String("AWS::Lambda::EventSourceMapping"), jsii.Number(1))
}
