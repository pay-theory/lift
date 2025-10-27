package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/jsii-runtime-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLiftAPIBuilderCreateLogGroupDisabled(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("APITest"), nil)
	props := &LiftAPIProps{
		APICommonProps: APICommonProps{
			Name:                jsii.String("test-api"),
			EnableAccessLogging: jsii.Bool(false),
		},
	}

	builder := newLiftAPIBuilder(stack, props)
	assert.Nil(t, builder.createLogGroup())
}

func TestLiftAPIBuilderCreateLogGroupReusesExisting(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("APITest"), nil)
	existing := awslogs.NewLogGroup(stack, jsii.String("Existing"), nil)

	props := &LiftAPIProps{
		APICommonProps: APICommonProps{
			Name:                jsii.String("test-api"),
			EnableAccessLogging: jsii.Bool(true),
			AccessLogGroup:      existing,
		},
	}

	builder := newLiftAPIBuilder(stack, props)
	logGroup := builder.createLogGroup()
	assert.Same(t, existing, logGroup)
}

func TestLiftAPIBuilderCreateLogGroupCreatesNewGroup(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("APITest"), nil)

	props := &LiftAPIProps{
		APICommonProps: APICommonProps{
			Name:                jsii.String("test-api"),
			EnableAccessLogging: jsii.Bool(true),
		},
	}

	builder := newLiftAPIBuilder(stack, props)
	logGroup := builder.createLogGroup()
	require.NotNil(t, logGroup)

	template := assertions.Template_FromStack(stack, nil)
	template.HasResourceProperties(jsii.String("AWS::Logs::LogGroup"), map[string]any{
		"LogGroupName":    "/aws/apigateway/test-api",
		"RetentionInDays": float64(7),
	})
}

func TestLiftAPIBuilderNeedsCustomStage(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("APITest"), nil)

	defaultBuilder := newLiftAPIBuilder(stack, &LiftAPIProps{})
	assert.False(t, defaultBuilder.needsCustomStage(defaultRoute))

	overriddenStage := newLiftAPIBuilder(stack, &LiftAPIProps{
		APICommonProps: APICommonProps{
			StageName: jsii.String("prod"),
		},
	})
	assert.True(t, overriddenStage.needsCustomStage("prod"))

	withLogging := newLiftAPIBuilder(stack, &LiftAPIProps{
		APICommonProps: APICommonProps{
			EnableAccessLogging: jsii.Bool(true),
		},
	})
	assert.True(t, withLogging.needsCustomStage(defaultRoute))

	withMetrics := newLiftAPIBuilder(stack, &LiftAPIProps{
		EnableDetailedMetrics: jsii.Bool(true),
	})
	assert.True(t, withMetrics.needsCustomStage(defaultRoute))
}

func TestLiftAPIBuilderCreateCORSConfig(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("APITest"), nil)

	defaultBuilder := newLiftAPIBuilder(stack, &LiftAPIProps{})
	cfg := defaultBuilder.createCORSConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, []string{"*"}, derefStrings(cfg.AllowOrigins))

	customOrigins := &[]*string{jsii.String("https://example.com")}
	customBuilder := newLiftAPIBuilder(stack, &LiftAPIProps{
		APICommonProps: APICommonProps{
			AllowOrigins: customOrigins,
		},
	})
	customCfg := customBuilder.createCORSConfig()
	require.NotNil(t, customCfg)
	assert.Same(t, customOrigins, customCfg.AllowOrigins)
}
