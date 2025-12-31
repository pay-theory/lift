package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awskinesis"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/jsii-runtime-go"
	"github.com/pay-theory/lift/pkg/cdk/test"
)

func TestKinesisProcessor_DefaultsCreateStreamDLQAndMapping(t *testing.T) {
	stack := test.NewTestStack()

	processor := NewKinesisProcessor(stack.Stack(), jsii.String("Processor"), &KinesisProcessorProps{
		FunctionProps: &LiftFunctionProps{
			FunctionProps: awslambda.FunctionProps{
				Runtime: awslambda.Runtime_NODEJS_18_X(),
				Handler: jsii.String("index.handler"),
				Code:    awslambda.Code_FromInline(jsii.String("exports.handler = async () => {};")),
			},
		},
	})

	if processor == nil || processor.Stream == nil || processor.Function.Function == nil {
		t.Fatal("expected processor stream and function")
	}
	if processor.GetDeadLetterQueueUrl() == nil {
		t.Fatal("expected dlq url when dlq enabled by default")
	}
	processor.GrantRead(processor.Function.Function)
	processor.GrantWrite(processor.Function.Function)
	processor.GrantReadWrite(processor.Function.Function)
	if processor.GetStreamName() == nil {
		t.Fatal("expected stream name")
	}
	if processor.GetStreamArn() == nil {
		t.Fatal("expected stream arn")
	}

	template := assertions.Template_FromStack(stack.Stack(), nil)
	template.ResourceCountIs(jsii.String("AWS::Kinesis::Stream"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::Lambda::EventSourceMapping"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::SQS::Queue"), jsii.Number(1))
}

func TestKinesisProcessor_ExistingStream_EnhancedFanOut_NoDLQ(t *testing.T) {
	stack := test.NewTestStack()

	stream := awskinesis.NewStream(stack.Stack(), jsii.String("ExistingStream"), &awskinesis.StreamProps{
		StreamMode: awskinesis.StreamMode_ON_DEMAND,
	})

	processor := NewKinesisProcessor(stack.Stack(), jsii.String("Processor"), &KinesisProcessorProps{
		ExistingStream:       stream,
		EnableDLQ:            jsii.Bool(false),
		EnableEnhancedFanOut: jsii.Bool(true),
		BatchSize:            jsii.Number(10),
		FunctionProps: &LiftFunctionProps{
			FunctionProps: awslambda.FunctionProps{
				Runtime: awslambda.Runtime_NODEJS_18_X(),
				Handler: jsii.String("index.handler"),
				Code:    awslambda.Code_FromInline(jsii.String("exports.handler = async () => {};")),
			},
		},
	})

	if processor.GetDeadLetterQueueUrl() != nil {
		t.Fatal("expected no dlq url when dlq disabled")
	}
	processor.AddEnvironmentVariable("EXTRA", "1")

	template := assertions.Template_FromStack(stack.Stack(), nil)
	template.ResourceCountIs(jsii.String("AWS::Kinesis::Stream"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::Kinesis::StreamConsumer"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::SQS::Queue"), jsii.Number(0))
}
