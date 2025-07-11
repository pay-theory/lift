package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
	"github.com/pay-theory/lift/pkg/cdk/stacks"
)

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)

	// Deploy an event-driven architecture
	stacks.NewEventDrivenStack(app, "EventAdaptersStack", &stacks.EventDrivenStackProps{
		StackProps: awscdk.StackProps{
			Env: env(),
		},
		AppName:                "event-adapters-demo",
		ApiCodePath:            "../dist/bootstrap",
		EventProcessorCodePath: "../dist/bootstrap", // Same handler for demo
		EnableDLQ:              true,
		EventBusName:           "lift-events",
	})

	app.Synth(nil)
}

func env() *awscdk.Environment {
	return nil
}
