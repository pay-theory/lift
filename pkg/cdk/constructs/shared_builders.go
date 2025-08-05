package constructs

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssqs"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// deadLetterQueueBuilder provides shared DLQ building functionality
type deadLetterQueueBuilder struct {
	scope        constructs.Construct
	props        *awssqs.QueueProps
	functionName *string
	queueSuffix  string
}

// newDeadLetterQueueBuilder creates a new DLQ builder
func newDeadLetterQueueBuilder(scope constructs.Construct, props *awssqs.QueueProps, functionName *string, queueSuffix string) *deadLetterQueueBuilder {
	return &deadLetterQueueBuilder{
		scope:        scope,
		props:        props,
		functionName: functionName,
		queueSuffix:  queueSuffix,
	}
}

// build creates the dead letter queue with standard configuration
func (dlqb *deadLetterQueueBuilder) build() awssqs.IQueue {
	dlqProps := &awssqs.QueueProps{
		RetentionPeriod: awscdk.Duration_Days(jsii.Number(14)),
	}
	
	if dlqb.props != nil {
		dlqProps = dlqb.props
		if dlqProps.RetentionPeriod == nil {
			dlqProps.RetentionPeriod = awscdk.Duration_Days(jsii.Number(14))
		}
	}

	// Set DLQ name if not provided
	if dlqProps.QueueName == nil && dlqb.functionName != nil {
		dlqProps.QueueName = jsii.String(*dlqb.functionName + dlqb.queueSuffix)
	}

	return awssqs.NewQueue(dlqb.scope, jsii.String("DeadLetterQueue"), dlqProps)
}

// propertyMerger provides generic property merging functionality
