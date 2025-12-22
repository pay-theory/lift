package constructs

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsevents"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssqs"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"

	"github.com/pay-theory/lift/pkg/naming"
)

// EventBusSchedulerProps defines properties for wiring an EventBridge schedule to an EventBus table.
//
// The scheduler Lambda is expected to use DynamORM to:
//   - query due scheduled items (services.EventBusDueScheduled)
//   - publish events (services.DynamoDBEventBus)
//   - delete scheduled items (services.EventBusDeleteScheduled)
type EventBusSchedulerProps struct {
	// Existing EventBus table (optional). If omitted, a new EventBusTable is created.
	Table *EventBusTable
	// Properties for creating a new EventBus table (optional).
	TableProps *EventBusTableProps

	// Required: schedule expression like "rate(1 minute)" or "cron(0/1 * * * ? *)".
	ScheduleExpression *string

	// Required: Lambda function configuration.
	FunctionProps awslambda.FunctionProps

	// Optional: EventBridge rule overrides.
	RuleProps *awsevents.RuleProps

	// Optional: EventBridge target tuning.
	MaxEventAge   awscdk.Duration
	RetryAttempts *float64
	EnableDLQ     *bool
	DLQProps      *awssqs.QueueProps

	// Optional: stable naming + runtime table resolution.
	// When AppName+Stage are provided, Lift injects APP_NAME/STAGE[/PARTNER] into the function env,
	// and uses them for deterministic resource names.
	AppName *string
	Stage   *string
	Partner *string
}

// EventBusScheduler composes an EventBridge schedule with an EventBus table and grants permissions.
type EventBusScheduler struct {
	constructs.Construct

	Table   *EventBusTable
	Handler *EventBridgeHandler
}

// NewEventBusScheduler creates a scheduled Lambda wired to an EventBus table.
func NewEventBusScheduler(scope constructs.Construct, id *string, props *EventBusSchedulerProps) *EventBusScheduler {
	construct := constructs.NewConstruct(scope, id)
	if props == nil {
		props = &EventBusSchedulerProps{}
	}
	if props.ScheduleExpression == nil || *props.ScheduleExpression == "" {
		panic("EventBusScheduler requires ScheduleExpression")
	}

	nameCtx, hasNameCtx := eventBusSchedulerNamingContext(props)

	scheduler := &EventBusScheduler{
		Construct: construct,
		Table:     resolveEventBusSchedulerTable(construct, nameCtx, hasNameCtx, props),
	}

	functionProps := props.FunctionProps
	functionProps.Environment = mergeEventBusSchedulerEnv(functionProps.Environment, nameCtx, hasNameCtx)
	if functionProps.FunctionName == nil && hasNameCtx {
		functionProps.FunctionName = jsii.String(nameCtx.ResourceName("eventbus-scheduler"))
	}

	handler, err := NewEventBridgeHandler(construct, jsii.String("Schedule"), &EventBridgeHandlerProps{
		FunctionProps:         functionProps,
		RuleProps:             props.RuleProps,
		ScheduleExpression:    props.ScheduleExpression,
		MaxEventAge:           props.MaxEventAge,
		RetryAttempts:         props.RetryAttempts,
		EnableDeadLetterQueue: props.EnableDLQ,
		DeadLetterQueueProps:  props.DLQProps,
	})
	if err != nil {
		panic(err)
	}
	scheduler.Handler = handler

	// Scheduler needs to read due items and publish/delete in the EventBus table.
	scheduler.Table.GrantReadWrite(scheduler.Handler.Function.Function)

	return scheduler
}

func eventBusSchedulerNamingContext(props *EventBusSchedulerProps) (naming.Context, bool) {
	if props == nil {
		return naming.Context{}, false
	}

	ctx := naming.Context{}
	if props.AppName != nil {
		ctx.AppName = *props.AppName
	}
	if props.Stage != nil {
		ctx.Stage = *props.Stage
	}
	if props.Partner != nil {
		ctx.Tenant = *props.Partner
	}

	if !ctx.IsComplete() {
		return naming.Context{}, false
	}

	return ctx.Normalize(), true
}

func resolveEventBusSchedulerTable(scope constructs.Construct, nameCtx naming.Context, hasNameCtx bool, props *EventBusSchedulerProps) *EventBusTable {
	if props.Table != nil {
		return props.Table
	}

	tableProps := props.TableProps
	if tableProps == nil {
		tableProps = &EventBusTableProps{}
	}
	if tableProps.TableName == nil && hasNameCtx {
		tableProps.TableName = jsii.String(nameCtx.ResourceName("events"))
	}

	return NewEventBusTable(scope, jsii.String("EventBusTable"), tableProps)
}

func mergeEventBusSchedulerEnv(env *map[string]*string, nameCtx naming.Context, hasNameCtx bool) *map[string]*string {
	out := make(map[string]*string)
	if env != nil {
		for k, v := range *env {
			out[k] = v
		}
	}

	if hasNameCtx {
		if _, ok := out[naming.EnvAppName]; !ok && nameCtx.AppName != "" {
			out[naming.EnvAppName] = jsii.String(nameCtx.AppName)
		}
		if _, ok := out[naming.EnvStage]; !ok && nameCtx.Stage != "" {
			out[naming.EnvStage] = jsii.String(nameCtx.Stage)
		}
		if _, ok := out[naming.EnvTenant]; !ok && nameCtx.Tenant != "" {
			out[naming.EnvTenant] = jsii.String(nameCtx.Tenant)
		}
	}

	return &out
}
