package constructs

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatchactions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssns"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// AlarmConfig defines configuration for CloudWatch alarms
type AlarmConfig struct {
	// Enable error rate alarm
	EnableErrorAlarm *bool
	// Error rate threshold (percentage)
	ErrorRateThreshold *float64
	// Enable latency alarm
	EnableLatencyAlarm *bool
	// Latency threshold in milliseconds
	LatencyThreshold *float64
	// Enable throttle alarm
	EnableThrottleAlarm *bool
	// Throttle count threshold
	ThrottleThreshold *float64
	// Enable concurrent executions alarm
	EnableConcurrentAlarm *bool
	// Concurrent executions threshold
	ConcurrentThreshold *float64
	// SNS topic for alarm notifications
	AlarmTopic awssns.ITopic
}

// MonitoredFunctionProps extends LiftFunctionProps with monitoring configuration
type MonitoredFunctionProps struct {
	LiftFunctionProps
	// CloudWatch Logs retention in days
	LogRetentionDays *float64
	// Enable CloudWatch dashboard
	EnableDashboard *bool
	// Dashboard name (optional - will generate if not provided)
	DashboardName *string
	// Alarm configuration
	AlarmConfig *AlarmConfig
	// Custom metrics namespace
	MetricsNamespace *string
	// Enable enhanced monitoring (Lambda Insights)
	EnableLambdaInsights *bool
	// Log level (ERROR, WARN, INFO, DEBUG)
	LogLevel *string
	// Enable CloudWatch Logs Insights queries
	EnableLogInsightsQueries *bool
}

// MonitoredFunction is a Lambda function with comprehensive monitoring
type MonitoredFunction struct {
	constructs.Construct
	Function  *LiftFunction
	LogGroup  awslogs.LogGroup
	Dashboard awscloudwatch.Dashboard
	Alarms    map[string]awscloudwatch.Alarm
}

// NewMonitoredFunction creates a Lambda function with comprehensive monitoring
func NewMonitoredFunction(scope constructs.Construct, id *string, props *MonitoredFunctionProps) *MonitoredFunction {
	this := constructs.NewConstruct(scope, id)

	// Set defaults
	if props.LogRetentionDays == nil {
		props.LogRetentionDays = jsii.Number(30) // 30 days default
	}
	if props.EnableDashboard == nil {
		props.EnableDashboard = jsii.Bool(true)
	}
	if props.EnableLambdaInsights == nil {
		props.EnableLambdaInsights = jsii.Bool(true)
	}
	if props.LogLevel == nil {
		props.LogLevel = jsii.String("INFO")
	}
	if props.MetricsNamespace == nil {
		props.MetricsNamespace = jsii.String("Lift/Functions")
	}

	// Set alarm defaults
	if props.AlarmConfig == nil {
		props.AlarmConfig = &AlarmConfig{}
	}
	if props.AlarmConfig.EnableErrorAlarm == nil {
		props.AlarmConfig.EnableErrorAlarm = jsii.Bool(true)
	}
	if props.AlarmConfig.ErrorRateThreshold == nil {
		props.AlarmConfig.ErrorRateThreshold = jsii.Number(1) // 1% error rate
	}
	if props.AlarmConfig.EnableLatencyAlarm == nil {
		props.AlarmConfig.EnableLatencyAlarm = jsii.Bool(true)
	}
	if props.AlarmConfig.LatencyThreshold == nil {
		props.AlarmConfig.LatencyThreshold = jsii.Number(3000) // 3 seconds
	}
	if props.AlarmConfig.EnableThrottleAlarm == nil {
		props.AlarmConfig.EnableThrottleAlarm = jsii.Bool(true)
	}
	if props.AlarmConfig.ThrottleThreshold == nil {
		props.AlarmConfig.ThrottleThreshold = jsii.Number(5) // 5 throttles
	}

	// Enable Lambda Insights
	if *props.EnableLambdaInsights {
		props.LiftFunctionProps.InsightsVersion = awslambda.LambdaInsightsVersion_VERSION_1_0_229_0()
	}

	// Add monitoring environment variables
	if props.LiftFunctionProps.Environment == nil {
		props.LiftFunctionProps.Environment = &map[string]*string{}
	}
	env := *props.LiftFunctionProps.Environment
	env["LOG_LEVEL"] = props.LogLevel
	env["METRICS_NAMESPACE"] = props.MetricsNamespace
	env["MONITORING_ENABLED"] = jsii.String("true")
	props.LiftFunctionProps.Environment = &env

	// Create the base Lift function
	liftFn := NewLiftFunction(this, jsii.String("Function"), &props.LiftFunctionProps)

	// Create or get the log group
	logGroupName := fmt.Sprintf("/aws/lambda/%s", *liftFn.Function.FunctionName())
	logGroup := awslogs.NewLogGroup(this, jsii.String("LogGroup"), &awslogs.LogGroupProps{
		LogGroupName:  jsii.String(logGroupName),
		Retention:     getRetentionDays(*props.LogRetentionDays),
		RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
	})

	// Create CloudWatch dashboard if enabled
	var dashboard awscloudwatch.Dashboard
	if *props.EnableDashboard {
		dashboardName := props.DashboardName
		if dashboardName == nil {
			dashboardName = jsii.String(fmt.Sprintf("%s-dashboard", *id))
		}
		dashboard = awscloudwatch.NewDashboard(this, jsii.String("Dashboard"), &awscloudwatch.DashboardProps{
			DashboardName: dashboardName,
		})

		// Add widgets to dashboard
		dashboard.AddWidgets(
			createInvocationsWidget(liftFn.Function),
			createErrorsWidget(liftFn.Function),
			createLatencyWidget(liftFn.Function),
			createConcurrentExecutionsWidget(liftFn.Function),
		)
	}

	// Create alarms
	alarms := make(map[string]awscloudwatch.Alarm)

	// Error rate alarm
	if *props.AlarmConfig.EnableErrorAlarm {
		errorAlarm := liftFn.Function.MetricErrors(&awscloudwatch.MetricOptions{
			Period: awscdk.Duration_Minutes(jsii.Number(5)),
		}).CreateAlarm(this, jsii.String("ErrorAlarm"), &awscloudwatch.CreateAlarmOptions{
			AlarmName:          jsii.String(fmt.Sprintf("%s-errors", *liftFn.Function.FunctionName())),
			AlarmDescription:   jsii.String("Lambda function error rate too high"),
			Threshold:          props.AlarmConfig.ErrorRateThreshold,
			EvaluationPeriods:  jsii.Number(2),
			TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		})
		alarms["errors"] = errorAlarm

		if props.AlarmConfig.AlarmTopic != nil {
			errorAlarm.AddAlarmAction(awscloudwatchactions.NewSnsAction(props.AlarmConfig.AlarmTopic))
		}
	}

	// Latency alarm
	if *props.AlarmConfig.EnableLatencyAlarm {
		latencyAlarm := liftFn.Function.MetricDuration(&awscloudwatch.MetricOptions{
			Period:    awscdk.Duration_Minutes(jsii.Number(5)),
			Statistic: jsii.String("Average"),
		}).CreateAlarm(this, jsii.String("LatencyAlarm"), &awscloudwatch.CreateAlarmOptions{
			AlarmName:          jsii.String(fmt.Sprintf("%s-latency", *liftFn.Function.FunctionName())),
			AlarmDescription:   jsii.String("Lambda function latency too high"),
			Threshold:          props.AlarmConfig.LatencyThreshold,
			EvaluationPeriods:  jsii.Number(2),
			TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		})
		alarms["latency"] = latencyAlarm

		if props.AlarmConfig.AlarmTopic != nil {
			latencyAlarm.AddAlarmAction(awscloudwatchactions.NewSnsAction(props.AlarmConfig.AlarmTopic))
		}
	}

	// Throttles alarm
	if *props.AlarmConfig.EnableThrottleAlarm {
		throttleAlarm := liftFn.Function.MetricThrottles(&awscloudwatch.MetricOptions{
			Period: awscdk.Duration_Minutes(jsii.Number(5)),
		}).CreateAlarm(this, jsii.String("ThrottleAlarm"), &awscloudwatch.CreateAlarmOptions{
			AlarmName:          jsii.String(fmt.Sprintf("%s-throttles", *liftFn.Function.FunctionName())),
			AlarmDescription:   jsii.String("Lambda function throttling detected"),
			Threshold:          props.AlarmConfig.ThrottleThreshold,
			EvaluationPeriods:  jsii.Number(1),
			TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		})
		alarms["throttles"] = throttleAlarm

		if props.AlarmConfig.AlarmTopic != nil {
			throttleAlarm.AddAlarmAction(awscloudwatchactions.NewSnsAction(props.AlarmConfig.AlarmTopic))
		}
	}

	// Concurrent executions alarm
	if props.AlarmConfig.EnableConcurrentAlarm != nil && *props.AlarmConfig.EnableConcurrentAlarm {
		// Use custom metric for concurrent executions
		concurrentMetric := awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("AWS/Lambda"),
			MetricName: jsii.String("ConcurrentExecutions"),
			DimensionsMap: &map[string]*string{
				"FunctionName": liftFn.Function.FunctionName(),
			},
			Period: awscdk.Duration_Minutes(jsii.Number(5)),
		})
		
		concurrentAlarm := concurrentMetric.CreateAlarm(this, jsii.String("ConcurrentAlarm"), &awscloudwatch.CreateAlarmOptions{
			AlarmName:          jsii.String(fmt.Sprintf("%s-concurrent", *liftFn.Function.FunctionName())),
			AlarmDescription:   jsii.String("Lambda function concurrent executions too high"),
			Threshold:          props.AlarmConfig.ConcurrentThreshold,
			EvaluationPeriods:  jsii.Number(2),
			TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		})
		alarms["concurrent"] = concurrentAlarm

		if props.AlarmConfig.AlarmTopic != nil {
			concurrentAlarm.AddAlarmAction(awscloudwatchactions.NewSnsAction(props.AlarmConfig.AlarmTopic))
		}
	}

	monitored := &MonitoredFunction{
		Construct: this,
		Function:  liftFn,
		LogGroup:  logGroup,
		Dashboard: dashboard,
		Alarms:    alarms,
	}

	// Add log insights queries if enabled
	if *props.EnableDashboard && props.EnableLogInsightsQueries != nil && *props.EnableLogInsightsQueries {
		monitored.AddCommonLogInsightsQueries()
	}

	return monitored
}

// GetFunction returns the underlying Lambda function
func (f *MonitoredFunction) GetFunction() awslambda.Function {
	return f.Function.Function
}

// GetLogGroup returns the CloudWatch log group
func (f *MonitoredFunction) GetLogGroup() awslogs.LogGroup {
	return f.LogGroup
}

// GetDashboard returns the CloudWatch dashboard
func (f *MonitoredFunction) GetDashboard() awscloudwatch.Dashboard {
	return f.Dashboard
}

// GetAlarm returns a specific alarm by name
func (f *MonitoredFunction) GetAlarm(name string) awscloudwatch.Alarm {
	return f.Alarms[name]
}

// AddCustomMetric adds a custom metric to the dashboard
func (f *MonitoredFunction) AddCustomMetric(metricName *string, namespace *string, dimensions *map[string]*string) awscloudwatch.Metric {
	metric := awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		MetricName: metricName,
		Namespace:  namespace,
		DimensionsMap: dimensions,
	})

	if f.Dashboard != nil {
		f.Dashboard.AddWidgets(awscloudwatch.NewGraphWidget(&awscloudwatch.GraphWidgetProps{
			Title:   metricName,
			Left:    &[]awscloudwatch.IMetric{metric},
			Width:   jsii.Number(12),
			Height:  jsii.Number(6),
		}))
	}

	return metric
}

// AddLogInsightsQuery adds a CloudWatch Logs Insights query to the dashboard
func (f *MonitoredFunction) AddLogInsightsQuery(queryName *string, queryString *string) {
	if f.Dashboard == nil {
		return
	}

	// Create a Logs Insights widget
	logsWidget := awscloudwatch.NewLogQueryWidget(&awscloudwatch.LogQueryWidgetProps{
		Title:        queryName,
		LogGroupNames: &[]*string{f.LogGroup.LogGroupName()},
		QueryString:  queryString,
		Width:        jsii.Number(24),
		Height:       jsii.Number(6),
	})

	f.Dashboard.AddWidgets(logsWidget)
}

// AddCommonLogInsightsQueries adds common CloudWatch Logs Insights queries
func (f *MonitoredFunction) AddCommonLogInsightsQueries() {
	if f.Dashboard == nil {
		return
	}

	// Error analysis query
	errorQuery := `fields @timestamp, @message
| filter @message like /ERROR/
| sort @timestamp desc
| limit 100`
	f.AddLogInsightsQuery(jsii.String("Recent Errors"), jsii.String(errorQuery))

	// Performance analysis query
	performanceQuery := `filter @type = "REPORT"
| stats avg(@duration), max(@duration), min(@duration), 
        pct(@duration, 50) as p50,
        pct(@duration, 95) as p95,
        pct(@duration, 99) as p99
by bin(5m)`
	f.AddLogInsightsQuery(jsii.String("Performance Metrics"), jsii.String(performanceQuery))

	// Cold start analysis query
	coldStartQuery := `filter @type = "REPORT"
| filter @message like /Init Duration/
| parse @message /Init Duration: (?<initDuration>[\d.]+) ms/
| stats count() as coldStarts, avg(initDuration) as avgInitDuration, max(initDuration) as maxInitDuration
by bin(5m)`
	f.AddLogInsightsQuery(jsii.String("Cold Start Analysis"), jsii.String(coldStartQuery))

	// Memory usage query
	memoryQuery := `filter @type = "REPORT"
| parse @message /Memory Size: (?<memSize>\d+) MB\s+Max Memory Used: (?<memUsed>\d+) MB/
| stats avg(memUsed), max(memUsed), avg(memUsed/memSize*100) as avgMemoryUtilization
by bin(5m)`
	f.AddLogInsightsQuery(jsii.String("Memory Usage"), jsii.String(memoryQuery))

	// Request patterns query
	requestPatternsQuery := `fields @timestamp, @message
| parse @message /\[(?<logLevel>\w+)\].*path=(?<path>[^\s]+).*method=(?<method>\w+)/
| filter ispresent(path)
| stats count() by path, method
| sort count() desc
| limit 20`
	f.AddLogInsightsQuery(jsii.String("Top Request Patterns"), jsii.String(requestPatternsQuery))

	// Slow requests query
	slowRequestsQuery := `filter @type = "REPORT"
| filter @duration > 3000
| fields @timestamp, @requestId, @duration
| sort @duration desc
| limit 50`
	f.AddLogInsightsQuery(jsii.String("Slow Requests"), jsii.String(slowRequestsQuery))

	// Error rate by status code
	errorRateQuery := `fields @timestamp, @message
| parse @message /status=(?<statusCode>\d+)/
| filter ispresent(statusCode)
| stats count() by statusCode
| sort statusCode asc`
	f.AddLogInsightsQuery(jsii.String("Response Status Codes"), jsii.String(errorRateQuery))

	// Tenant activity (for multi-tenant apps)
	tenantActivityQuery := `fields @timestamp, @message
| parse @message /tenant=(?<tenantId>[^\s]+)/
| filter ispresent(tenantId)
| stats count() as requests by tenantId
| sort requests desc
| limit 20`
	f.AddLogInsightsQuery(jsii.String("Tenant Activity"), jsii.String(tenantActivityQuery))
}

// Helper functions for creating dashboard widgets
func createInvocationsWidget(fn awslambda.Function) awscloudwatch.GraphWidget {
	return awscloudwatch.NewGraphWidget(&awscloudwatch.GraphWidgetProps{
		Title: jsii.String("Invocations"),
		Left: &[]awscloudwatch.IMetric{
			fn.MetricInvocations(nil),
		},
		Width:  jsii.Number(12),
		Height: jsii.Number(6),
	})
}

func createErrorsWidget(fn awslambda.Function) awscloudwatch.GraphWidget {
	return awscloudwatch.NewGraphWidget(&awscloudwatch.GraphWidgetProps{
		Title: jsii.String("Errors"),
		Left: &[]awscloudwatch.IMetric{
			fn.MetricErrors(nil),
		},
		Width:  jsii.Number(12),
		Height: jsii.Number(6),
	})
}

func createLatencyWidget(fn awslambda.Function) awscloudwatch.GraphWidget {
	return awscloudwatch.NewGraphWidget(&awscloudwatch.GraphWidgetProps{
		Title: jsii.String("Duration"),
		Left: &[]awscloudwatch.IMetric{
			fn.MetricDuration(&awscloudwatch.MetricOptions{Statistic: jsii.String("Average")}),
			fn.MetricDuration(&awscloudwatch.MetricOptions{Statistic: jsii.String("p99")}),
		},
		Width:  jsii.Number(12),
		Height: jsii.Number(6),
	})
}

func createConcurrentExecutionsWidget(fn awslambda.Function) awscloudwatch.GraphWidget {
	// Create custom metric for concurrent executions
	concurrentMetric := awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("AWS/Lambda"),
		MetricName: jsii.String("ConcurrentExecutions"),
		DimensionsMap: &map[string]*string{
			"FunctionName": fn.FunctionName(),
		},
	})
	
	return awscloudwatch.NewGraphWidget(&awscloudwatch.GraphWidgetProps{
		Title: jsii.String("Concurrent Executions"),
		Left: &[]awscloudwatch.IMetric{
			concurrentMetric,
		},
		Width:  jsii.Number(12),
		Height: jsii.Number(6),
	})
}