# WebSocket API Modular Pattern

## Problem

When using the WebSocket API construct with default settings, it creates Lambda functions internally which leads to deep nesting in the CDK construct tree. This causes CloudFormation to generate very long resource names for Lambda permissions, which can exceed AWS limits.

Example of problematic long names:
- `PennyWebSocketAPIDisconnectFunctionInvokevX45SoP9HzavMlHnWXJDq59djssXRa0Z7T35MPhgI655A3BBA`

## Solution

Create Lambda functions at the stack level and pass them to the WebSocket API construct. This reduces nesting depth and results in shorter resource names.

## Modular Pattern Example

```go
package main

import (
    "github.com/aws/aws-cdk-go/awscdk/v2"
    "github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
    "github.com/aws/constructs-go/constructs/v10"
    "github.com/aws/jsii-runtime-go"
    "github.com/pay-theory/lift/pkg/cdk/constructs"
)

type WebSocketStackProps struct {
    awscdk.StackProps
}

func NewWebSocketStack(scope constructs.Construct, id string, props *WebSocketStackProps) awscdk.Stack {
    var sprops awscdk.StackProps
    if props != nil {
        sprops = props.StackProps
    }
    stack := awscdk.NewStack(scope, &id, &sprops)

    // Step 1: Create Lambda functions at stack level (not nested)
    connectFunction := awslambda.NewFunction(stack, jsii.String("Connect"), &awslambda.FunctionProps{
        FunctionName: jsii.String("my-app-ws-connect"),
        Runtime:      awslambda.Runtime_PROVIDED_AL2023(),
        Architecture: awslambda.Architecture_ARM_64(),
        Code:         awslambda.Code_FromAsset(jsii.String("./dist/connect"), nil),
        Handler:      jsii.String("bootstrap"),
        Timeout:      awscdk.Duration_Seconds(jsii.Number(30)),
        Environment: &map[string]*string{
            "APP_NAME": jsii.String("my-app"),
        },
    })

    disconnectFunction := awslambda.NewFunction(stack, jsii.String("Disconnect"), &awslambda.FunctionProps{
        FunctionName: jsii.String("my-app-ws-disconnect"),
        Runtime:      awslambda.Runtime_PROVIDED_AL2023(),
        Architecture: awslambda.Architecture_ARM_64(),
        Code:         awslambda.Code_FromAsset(jsii.String("./dist/disconnect"), nil),
        Handler:      jsii.String("bootstrap"),
        Timeout:      awscdk.Duration_Seconds(jsii.Number(30)),
    })

    defaultFunction := awslambda.NewFunction(stack, jsii.String("Default"), &awslambda.FunctionProps{
        FunctionName: jsii.String("my-app-ws-default"),
        Runtime:      awslambda.Runtime_PROVIDED_AL2023(),
        Architecture: awslambda.Architecture_ARM_64(),
        Code:         awslambda.Code_FromAsset(jsii.String("./dist/default"), nil),
        Handler:      jsii.String("bootstrap"),
        Timeout:      awscdk.Duration_Seconds(jsii.Number(30)),
    })

    // Step 2: Create WebSocket API with external functions
    wsApi := constructs.NewWebSocketAPI(stack, jsii.String("API"), &constructs.WebSocketAPIProps{
        ApiName:                 jsii.String("my-app-websocket"),
        Description:             jsii.String("WebSocket API for my app"),
        // Pass the functions - this prevents internal function creation
        ConnectRouteFunction:    connectFunction,
        DisconnectRouteFunction: disconnectFunction,
        DefaultRouteFunction:    defaultFunction,
        // Other configuration
        EnableConnectionManagement: jsii.Bool(true),
        EnableAccessLogging:        jsii.Bool(true),
        EnableMultiTenant:          jsii.Bool(true),
    })

    // Step 3: Grant necessary permissions
    if wsApi.ConnectionTable != nil {
        wsApi.ConnectionTable.GrantReadWrite(connectFunction)
        wsApi.ConnectionTable.GrantReadWrite(disconnectFunction)
        wsApi.ConnectionTable.GrantReadWrite(defaultFunction)
    }

    // Step 4: Add custom routes if needed
    customFunction := awslambda.NewFunction(stack, jsii.String("Custom"), &awslambda.FunctionProps{
        FunctionName: jsii.String("my-app-ws-custom"),
        Runtime:      awslambda.Runtime_PROVIDED_AL2023(),
        Code:         awslambda.Code_FromAsset(jsii.String("./dist/custom"), nil),
        Handler:      jsii.String("bootstrap"),
    })

    wsApi.AddRoute("sendMessage", customFunction, &constructs.WebSocketRouteConfig{
        RouteKey: jsii.String("sendMessage"),
        Function: customFunction,
    })

    // Output WebSocket URL
    awscdk.NewCfnOutput(stack, jsii.String("WebSocketURL"), &awscdk.CfnOutputProps{
        Value:       wsApi.GetWebSocketUrl(),
        Description: jsii.String("WebSocket API URL"),
    })

    return stack
}
```

## Benefits

1. **Shorter Resource Names**: By creating functions at the stack level, the nesting hierarchy is flatter, resulting in shorter CloudFormation resource names.

2. **Better Control**: You have full control over Lambda function configuration, including names, memory, environment variables, etc.

3. **Reusability**: Functions can be shared across multiple constructs if needed.

4. **Clearer Architecture**: The infrastructure code better reflects the actual architecture with explicit function definitions.

## Migration Guide

If you're currently using the default pattern:

```go
// OLD: Functions created internally
wsApi := constructs.NewWebSocketAPI(stack, jsii.String("WebSocketAPI"), &constructs.WebSocketAPIProps{
    ApiName: jsii.String("my-api"),
    FunctionProps: awslambda.FunctionProps{
        Code:    awslambda.Code_FromAsset(jsii.String("./dist"), nil),
        Handler: jsii.String("bootstrap"),
        Runtime: awslambda.Runtime_PROVIDED_AL2023(),
    },
})
```

Migrate to the modular pattern:

```go
// NEW: Functions created externally
connectFn := awslambda.NewFunction(stack, jsii.String("C"), &awslambda.FunctionProps{
    Code:    awslambda.Code_FromAsset(jsii.String("./dist"), nil),
    Handler: jsii.String("bootstrap"),
    Runtime: awslambda.Runtime_PROVIDED_AL2023(),
})
// ... create other functions ...

wsApi := constructs.NewWebSocketAPI(stack, jsii.String("API"), &constructs.WebSocketAPIProps{
    ApiName:              jsii.String("my-api"),
    ConnectRouteFunction: connectFn,
    // ... other functions ...
})
```

## Best Practices

1. Use short, meaningful IDs for constructs (e.g., "API" instead of "WebSocketAPI")
2. Create all Lambda functions at the stack level
3. Use consistent naming patterns for functions
4. Consider creating a helper function to standardize Lambda creation
5. Always test your CloudFormation template to ensure resource names are within limits

## Resource Name Length

AWS CloudFormation resource names must be:
- Under 64 characters for most resources
- Under 80 characters for IAM roles
- Under 128 characters for Lambda function names

The modular pattern helps ensure you stay within these limits.