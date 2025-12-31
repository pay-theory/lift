package constructs

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssns"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// MonitorableResource interface for resources that can be monitored
type MonitorableResource interface {
	// GetResourceName returns the name of the resource.
	// Returns:
	//   - A pointer to the name of the resource
	GetResourceName() *string
}

// MetricConfiguration defines advanced metric configuration
type MetricConfiguration struct {
	// Enable detailed metrics
	DetailedMetrics *bool
	// Custom dimensions
	Dimensions *map[string]*string
	// Metric resolution (1 or 60 seconds)
	Resolution *float64
	// Percentiles to track
	Percentiles *[]*float64
	// Enable custom business metrics
	EnableBusinessMetrics *bool
}

// AlarmThresholds defines threshold configuration for alarms
type AlarmThresholds struct {
	// Error rate threshold (percentage)
	ErrorRate *float64
	// Latency threshold (milliseconds)
	LatencyP99 *float64
	// Throttle count threshold
	ThrottleCount *float64
	// Concurrent executions threshold
	ConcurrentExecutions *float64
	// Custom thresholds
	CustomThresholds *map[string]*float64
}

// EnhancedMonitoringProps defines properties for enhanced monitoring
type EnhancedMonitoringProps struct {
	// Resource to monitor
	Resource MonitorableResource
	// Custom namespace for metrics
	Namespace *string
	// Alert configuration
	AlertTopic awssns.ITopic
	// Dashboard configuration
	DashboardName *string
	// Metric configuration
	MetricConfig *MetricConfiguration
	// Alarm thresholds
	AlarmThresholds *AlarmThresholds
	// Enable real-time streaming
	EnableRealTimeStreaming *bool
	// Environment tag
	Environment *string
}

// EnhancedMonitoring provides comprehensive monitoring with real CloudWatch metrics
type EnhancedMonitoring struct {
	constructs.Construct
	Metrics       map[string]awscloudwatch.IMetric
	Alarms        map[string]awscloudwatch.IAlarm
	Dashboard     awscloudwatch.Dashboard
	LogGroup      awslogs.LogGroup
	MetricFilters map[string]awslogs.MetricFilter
}

// NewEnhancedMonitoring creates a comprehensive monitoring construct
func NewEnhancedMonitoring(scope constructs.Construct, id *string, props *EnhancedMonitoringProps) *EnhancedMonitoring {
	this := constructs.NewConstruct(scope, id)

	monitoring := &EnhancedMonitoring{
		Construct:     this,
		Metrics:       make(map[string]awscloudwatch.IMetric),
		Alarms:        make(map[string]awscloudwatch.IAlarm),
		MetricFilters: make(map[string]awslogs.MetricFilter),
	}

	// Set defaults
	monitoring.setDefaults(props)

	// Create metrics based on resource type
	monitoring.createMetrics(props)

	// Create alarms
	monitoring.createAlarms(props)

	// Create dashboard
	monitoring.createDashboard(props)

	// Set up metric streams for real-time monitoring
	if props.EnableRealTimeStreaming != nil && *props.EnableRealTimeStreaming {
		monitoring.createMetricStreams(props)
	}

	return monitoring
}

func (m *EnhancedMonitoring) setDefaults(props *EnhancedMonitoringProps) {
	if props.Namespace == nil {
		props.Namespace = jsii.String("Lift/Application")
	}
	if props.MetricConfig == nil {
		props.MetricConfig = &MetricConfiguration{}
	}
	if props.MetricConfig.DetailedMetrics == nil {
		props.MetricConfig.DetailedMetrics = jsii.Bool(true)
	}
	if props.MetricConfig.EnableBusinessMetrics == nil {
		props.MetricConfig.EnableBusinessMetrics = jsii.Bool(true)
	}
	if props.MetricConfig.Resolution == nil {
		props.MetricConfig.Resolution = jsii.Number(1) // 1-second resolution
	}
	if props.MetricConfig.Percentiles == nil {
		props.MetricConfig.Percentiles = &[]*float64{
			jsii.Number(50),
			jsii.Number(95),
			jsii.Number(99),
		}
	}
	if props.AlarmThresholds == nil {
		props.AlarmThresholds = &AlarmThresholds{
			ErrorRate:            jsii.Number(5.0),  // 5% error rate
			LatencyP99:           jsii.Number(3000), // 3 seconds
			ThrottleCount:        jsii.Number(5),    // 5 throttles
			ConcurrentExecutions: jsii.Number(100),  // 100 concurrent
		}
	}
	if props.Environment == nil {
		props.Environment = jsii.String("production")
	}
}

func (m *EnhancedMonitoring) createMetrics(props *EnhancedMonitoringProps) {
	// Lambda function metrics
	if fn, ok := props.Resource.(*LiftFunction); ok {
		m.createLambdaMetrics(fn, props)
	}

	// DynamoDB table metrics
	if table, ok := props.Resource.(*LiftTable); ok {
		m.createDynamoDBMetrics(table, props)
	}

	// API Gateway metrics
	if api, ok := props.Resource.(*LiftAPI); ok {
		m.createAPIMetrics(api, props)
	}
}

func (m *EnhancedMonitoring) createLambdaMetrics(fn *LiftFunction, props *EnhancedMonitoringProps) {
	baseDimensions := &map[string]*string{
		"FunctionName": fn.Function.FunctionName(),
		"Environment":  props.Environment,
	}

	// Request metrics with detailed dimensions
	m.Metrics["Requests"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:     props.Namespace,
		MetricName:    jsii.String("Requests"),
		DimensionsMap: baseDimensions,
		Statistic:     jsii.String("Sum"),
		Period:        awscdk.Duration_Minutes(jsii.Number(1)),
	})

	// Error metrics with categorization
	m.Metrics["Errors"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:     props.Namespace,
		MetricName:    jsii.String("Errors"),
		DimensionsMap: baseDimensions,
		Statistic:     jsii.String("Sum"),
		Period:        awscdk.Duration_Minutes(jsii.Number(1)),
	})

	// Latency percentiles
	for _, p := range *props.MetricConfig.Percentiles {
		m.Metrics[fmt.Sprintf("LatencyP%v", *p)] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:     props.Namespace,
			MetricName:    jsii.String("Duration"),
			DimensionsMap: baseDimensions,
			Statistic:     jsii.String(fmt.Sprintf("p%v", *p)),
			Period:        awscdk.Duration_Minutes(jsii.Number(1)),
		})
	}

	// Cold start metrics
	m.createColdStartMetrics(fn, props)

	// Business metrics
	if props.MetricConfig.EnableBusinessMetrics != nil && *props.MetricConfig.EnableBusinessMetrics {
		m.createBusinessMetrics(fn, props)
	}

	// Standard Lambda metrics
	m.Metrics["ConcurrentExecutions"] = awslambda.Function_MetricAllConcurrentExecutions(&awscloudwatch.MetricOptions{
		Period: awscdk.Duration_Minutes(jsii.Number(1)),
	})

	m.Metrics["Throttles"] = fn.Function.MetricThrottles(&awscloudwatch.MetricOptions{
		Period: awscdk.Duration_Minutes(jsii.Number(1)),
	})

	// Memory utilization
	m.Metrics["MemoryUtilization"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("AWS/Lambda"),
		MetricName: jsii.String("MemoryUtilization"),
		DimensionsMap: &map[string]*string{
			"FunctionName": fn.Function.FunctionName(),
		},
		Statistic: jsii.String("Maximum"),
		Period:    awscdk.Duration_Minutes(jsii.Number(1)),
	})
}

func (m *EnhancedMonitoring) createColdStartMetrics(fn *LiftFunction, props *EnhancedMonitoringProps) {
	// Create log group for the function
	logGroupName := fmt.Sprintf("/aws/lambda/%s", *fn.Function.FunctionName())
	m.LogGroup = awslogs.NewLogGroup(m.Construct, jsii.String("LogGroup"), &awslogs.LogGroupProps{
		LogGroupName:  jsii.String(logGroupName),
		Retention:     awslogs.RetentionDays_ONE_MONTH,
		RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
	})

	// Cold start filter
	coldStartFilter := awslogs.NewMetricFilter(m.Construct, jsii.String("ColdStartFilter"), &awslogs.MetricFilterProps{
		LogGroup:        m.LogGroup,
		MetricNamespace: props.Namespace,
		MetricName:      jsii.String("ColdStarts"),
		FilterPattern:   awslogs.FilterPattern_Literal(jsii.String("[timestamp, requestId, initType=\"INIT_START\", ...]")),
		MetricValue:     jsii.String("1"),
		DefaultValue:    jsii.Number(0),
		Dimensions: &map[string]*string{
			"FunctionName": fn.Function.FunctionName(),
			"Environment":  props.Environment,
		},
	})

	// Cold start duration filter
	// Use a literal filter pattern to capture duration from Lambda REPORT line
	durationFilter := awslogs.NewMetricFilter(m.Construct, jsii.String("ColdStartDurationFilter"), &awslogs.MetricFilterProps{
		LogGroup:        m.LogGroup,
		MetricNamespace: props.Namespace,
		MetricName:      jsii.String("ColdStartDuration"),
		FilterPattern:   awslogs.FilterPattern_Literal(jsii.String("REPORT RequestId: ? Duration: $duration")),
		MetricValue:     jsii.String("$duration"),
		Dimensions: &map[string]*string{
			"FunctionName": fn.Function.FunctionName(),
			"Environment":  props.Environment,
		},
	})

	m.MetricFilters["ColdStart"] = coldStartFilter
	m.MetricFilters["ColdStartDuration"] = durationFilter
	m.Metrics["ColdStarts"] = coldStartFilter.Metric(&awscloudwatch.MetricOptions{})
	m.Metrics["ColdStartDuration"] = durationFilter.Metric(&awscloudwatch.MetricOptions{})
}

func (m *EnhancedMonitoring) createBusinessMetrics(fn *LiftFunction, props *EnhancedMonitoringProps) {
	// Parameters are used for context but not directly in metric creation
	_ = fn
	_ = props

	// Success rate metric
	m.Metrics["SuccessRate"] = awscloudwatch.NewMathExpression(&awscloudwatch.MathExpressionProps{
		Expression: jsii.String("100 * (requests - errors) / requests"),
		UsingMetrics: &map[string]awscloudwatch.IMetric{
			"requests": m.Metrics["Requests"],
			"errors":   m.Metrics["Errors"],
		},
		Label:  jsii.String("Success Rate (%)"),
		Period: awscdk.Duration_Minutes(jsii.Number(1)),
	})

	// Request rate metrics
	m.Metrics["RequestRate"] = awscloudwatch.NewMathExpression(&awscloudwatch.MathExpressionProps{
		Expression: jsii.String("requests / PERIOD(requests)"),
		UsingMetrics: &map[string]awscloudwatch.IMetric{
			"requests": m.Metrics["Requests"],
		},
		Label:  jsii.String("Requests per Second"),
		Period: awscdk.Duration_Minutes(jsii.Number(1)),
	})

	// Error rate metric
	m.Metrics["ErrorRate"] = awscloudwatch.NewMathExpression(&awscloudwatch.MathExpressionProps{
		Expression: jsii.String("100 * errors / requests"),
		UsingMetrics: &map[string]awscloudwatch.IMetric{
			"requests": m.Metrics["Requests"],
			"errors":   m.Metrics["Errors"],
		},
		Label:  jsii.String("Error Rate (%)"),
		Period: awscdk.Duration_Minutes(jsii.Number(1)),
	})
}

func (m *EnhancedMonitoring) createDynamoDBMetrics(table *LiftTable, props *EnhancedMonitoringProps) {
	// Props parameter is reserved for future use
	_ = props

	// Consumed capacity metrics
	m.Metrics["ConsumedReadCapacity"] = table.Table.MetricConsumedReadCapacityUnits(&awscloudwatch.MetricOptions{
		Period: awscdk.Duration_Minutes(jsii.Number(1)),
	})

	m.Metrics["ConsumedWriteCapacity"] = table.Table.MetricConsumedWriteCapacityUnits(&awscloudwatch.MetricOptions{
		Period: awscdk.Duration_Minutes(jsii.Number(1)),
	})

	// Throttled requests
	m.Metrics["ReadThrottles"] = table.Table.MetricThrottledRequestsForOperations(&awsdynamodb.OperationsMetricOptions{
		Operations: &[]awsdynamodb.Operation{awsdynamodb.Operation_GET_ITEM, awsdynamodb.Operation_QUERY, awsdynamodb.Operation_SCAN},
		Period:     awscdk.Duration_Minutes(jsii.Number(1)),
	})

	m.Metrics["WriteThrottles"] = table.Table.MetricThrottledRequestsForOperations(&awsdynamodb.OperationsMetricOptions{
		Operations: &[]awsdynamodb.Operation{awsdynamodb.Operation_PUT_ITEM, awsdynamodb.Operation_UPDATE_ITEM, awsdynamodb.Operation_DELETE_ITEM},
		Period:     awscdk.Duration_Minutes(jsii.Number(1)),
	})

	// System errors
	m.Metrics["SystemErrors"] = table.Table.MetricSystemErrorsForOperations(&awsdynamodb.SystemErrorsForOperationsMetricOptions{
		Operations: &[]awsdynamodb.Operation{
			awsdynamodb.Operation_GET_ITEM,
			awsdynamodb.Operation_PUT_ITEM,
			awsdynamodb.Operation_QUERY,
			awsdynamodb.Operation_SCAN,
		},
		Period: awscdk.Duration_Minutes(jsii.Number(1)),
	})

	// Latency metrics by operation
	operations := []awsdynamodb.Operation{
		awsdynamodb.Operation_GET_ITEM,
		awsdynamodb.Operation_PUT_ITEM,
		awsdynamodb.Operation_QUERY,
		awsdynamodb.Operation_SCAN,
	}

	for _, op := range operations {
		opName := string(op)
		m.Metrics[fmt.Sprintf("%sLatency", opName)] = table.Table.MetricSuccessfulRequestLatency(&awscloudwatch.MetricOptions{
			DimensionsMap: &map[string]*string{
				"Operation": jsii.String(opName),
			},
			Statistic: jsii.String("Average"),
			Period:    awscdk.Duration_Minutes(jsii.Number(1)),
		})
	}
}

func (m *EnhancedMonitoring) createAPIMetrics(api *LiftAPI, props *EnhancedMonitoringProps) {
	// Props parameter is reserved for future use
	_ = props

	// API Gateway v2 metrics
	stageName := jsii.String("$default")

	// Request count
	m.Metrics["APIRequests"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("AWS/ApiGatewayV2"),
		MetricName: jsii.String("Count"),
		DimensionsMap: &map[string]*string{
			"ApiId": api.HttpAPI.ApiId(),
			"Stage": stageName,
		},
		Statistic: jsii.String("Sum"),
		Period:    awscdk.Duration_Minutes(jsii.Number(1)),
	})

	// Integration latency
	m.Metrics["APIIntegrationLatency"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("AWS/ApiGatewayV2"),
		MetricName: jsii.String("IntegrationLatency"),
		DimensionsMap: &map[string]*string{
			"ApiId": api.HttpAPI.ApiId(),
			"Stage": stageName,
		},
		Statistic: jsii.String("Average"),
		Period:    awscdk.Duration_Minutes(jsii.Number(1)),
	})

	// 4xx errors
	m.Metrics["API4xxErrors"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("AWS/ApiGatewayV2"),
		MetricName: jsii.String("4xx"),
		DimensionsMap: &map[string]*string{
			"ApiId": api.HttpAPI.ApiId(),
			"Stage": stageName,
		},
		Statistic: jsii.String("Sum"),
		Period:    awscdk.Duration_Minutes(jsii.Number(1)),
	})

	// 5xx errors
	m.Metrics["API5xxErrors"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("AWS/ApiGatewayV2"),
		MetricName: jsii.String("5xx"),
		DimensionsMap: &map[string]*string{
			"ApiId": api.HttpAPI.ApiId(),
			"Stage": stageName,
		},
		Statistic: jsii.String("Sum"),
		Period:    awscdk.Duration_Minutes(jsii.Number(1)),
	})
}

func (m *EnhancedMonitoring) createAlarms(props *EnhancedMonitoringProps) {
	// Error rate alarm
	if errorMetric, ok := m.Metrics["ErrorRate"]; ok {
		alarmProps := &awscloudwatch.AlarmProps{
			Metric:            errorMetric,
			AlarmName:         jsii.String("High Error Rate"),
			AlarmDescription:  jsii.String("Error rate exceeds threshold"),
			Threshold:         props.AlarmThresholds.ErrorRate,
			EvaluationPeriods: jsii.Number(2),
			DatapointsToAlarm: jsii.Number(2),
			TreatMissingData:  awscloudwatch.TreatMissingData_NOT_BREACHING,
		}

		if props.AlertTopic != nil {
			alarmProps.ActionsEnabled = jsii.Bool(true)
		}

		m.Alarms["HighErrorRate"] = awscloudwatch.NewAlarm(m.Construct, jsii.String("HighErrorRate"), alarmProps)
	}

	// Latency alarm (p99)
	if latencyMetric, ok := m.Metrics["LatencyP99"]; ok {
		m.Alarms["HighLatency"] = awscloudwatch.NewAlarm(m.Construct, jsii.String("HighLatency"), &awscloudwatch.AlarmProps{
			Metric:            latencyMetric,
			AlarmName:         jsii.String("High Latency"),
			AlarmDescription:  jsii.String("P99 latency exceeds threshold"),
			Threshold:         props.AlarmThresholds.LatencyP99,
			EvaluationPeriods: jsii.Number(3),
			DatapointsToAlarm: jsii.Number(2),
			TreatMissingData:  awscloudwatch.TreatMissingData_NOT_BREACHING,
		})

		// NOTE: Alert topic integration available when AddAlarmAction interface is available
	}

	// Throttling alarm
	if throttleMetric, ok := m.Metrics["Throttles"]; ok {
		m.Alarms["Throttling"] = awscloudwatch.NewAlarm(m.Construct, jsii.String("Throttling"), &awscloudwatch.AlarmProps{
			Metric:            throttleMetric,
			AlarmName:         jsii.String("Function Throttling"),
			AlarmDescription:  jsii.String("Function is being throttled"),
			Threshold:         props.AlarmThresholds.ThrottleCount,
			EvaluationPeriods: jsii.Number(1),
			TreatMissingData:  awscloudwatch.TreatMissingData_NOT_BREACHING,
		})

		// NOTE: Alert topic integration available when AddAlarmAction interface is available
	}

	// Concurrent executions alarm
	if concurrentMetric, ok := m.Metrics["ConcurrentExecutions"]; ok && props.AlarmThresholds != nil && props.AlarmThresholds.ConcurrentExecutions != nil {
		m.Alarms["HighConcurrency"] = awscloudwatch.NewAlarm(m.Construct, jsii.String("HighConcurrency"), &awscloudwatch.AlarmProps{
			Metric:            concurrentMetric,
			AlarmName:         jsii.String("High Concurrent Executions"),
			AlarmDescription:  jsii.String("Concurrent executions exceed threshold"),
			Threshold:         props.AlarmThresholds.ConcurrentExecutions,
			EvaluationPeriods: jsii.Number(2),
			TreatMissingData:  awscloudwatch.TreatMissingData_NOT_BREACHING,
		})

		// NOTE: Alert topic integration available when AddAlarmAction interface is available
	}
}

func (m *EnhancedMonitoring) createDashboard(props *EnhancedMonitoringProps) {
	dashboardName := props.DashboardName
	if dashboardName == nil {
		dashboardName = jsii.String(fmt.Sprintf("%s-Enhanced-Dashboard", *awscdk.Stack_Of(m.Construct).StackName()))
	}

	rows := make([]*[]awscloudwatch.IWidget, 0)

	switch {
	case m.Metrics["Requests"] != nil:
		rows = append(rows, m.lambdaDashboardRows()...)
	case m.Metrics["APIRequests"] != nil:
		rows = append(rows, m.apiDashboardRows()...)
	case m.Metrics["ConsumedReadCapacity"] != nil:
		rows = append(rows, m.dynamoDashboardRows()...)
	}

	if len(rows) == 0 {
		fallback := []awscloudwatch.IWidget{
			awscloudwatch.NewTextWidget(&awscloudwatch.TextWidgetProps{
				Markdown: jsii.String("## Enhanced Monitoring\nNo widgets configured for this resource."),
				Width:    jsii.Number(24),
				Height:   jsii.Number(3),
			}),
		}
		rows = append(rows, &fallback)
	}

	m.Dashboard = awscloudwatch.NewDashboard(m.Construct, jsii.String("Dashboard"), &awscloudwatch.DashboardProps{
		DashboardName: dashboardName,
		Widgets:       &rows,
	})
}

func (m *EnhancedMonitoring) createMetricStreams(_ *EnhancedMonitoringProps) {
	// Implementation for real-time metric streaming would go here
	// This would typically involve setting up Kinesis Data Firehose
	// and CloudWatch Metric Streams for real-time processing
}

// getAllAlarms is reserved for future use
// func (m *EnhancedMonitoring) getAllAlarms() *[]awscloudwatch.IAlarm {
// 	var alarms []awscloudwatch.IAlarm
// 	for _, alarm := range m.Alarms {
// 		alarms = append(alarms, alarm)
// 	}
// 	return &alarms
// }

// GetMetric returns a specific metric by name
func (m *EnhancedMonitoring) GetMetric(name string) awscloudwatch.IMetric {
	return m.Metrics[name]
}

// GetAlarm returns a specific alarm by name
func (m *EnhancedMonitoring) GetAlarm(name string) awscloudwatch.IAlarm {
	return m.Alarms[name]
}

// AddCustomMetric adds a custom metric to the monitoring
func (m *EnhancedMonitoring) AddCustomMetric(name string, metric awscloudwatch.IMetric) {
	m.Metrics[name] = metric
}

// AddCustomAlarm adds a custom alarm to the monitoring
func (m *EnhancedMonitoring) AddCustomAlarm(name string, alarm awscloudwatch.IAlarm) {
	m.Alarms[name] = alarm
}

type graphWidgetDefinition struct {
	MetricKey string
	Title     string
}

func (m *EnhancedMonitoring) graphRow(definitions []graphWidgetDefinition) *[]awscloudwatch.IWidget {
	row := make([]awscloudwatch.IWidget, 0, len(definitions))

	for _, definition := range definitions {
		if metric := m.Metrics[definition.MetricKey]; metric != nil {
			row = append(row, awscloudwatch.NewGraphWidget(&awscloudwatch.GraphWidgetProps{
				Title:  jsii.String(definition.Title),
				Left:   &[]awscloudwatch.IMetric{metric},
				Width:  jsii.Number(12),
				Height: jsii.Number(6),
			}))
		}
	}

	if len(row) == 0 {
		return nil
	}

	return &row
}

func (m *EnhancedMonitoring) graphRows(definitions ...[]graphWidgetDefinition) []*[]awscloudwatch.IWidget {
	rows := make([]*[]awscloudwatch.IWidget, 0, len(definitions))

	for _, rowDefinitions := range definitions {
		if row := m.graphRow(rowDefinitions); row != nil {
			rows = append(rows, row)
		}
	}

	return rows
}

func appendRowIfNotEmpty(rows []*[]awscloudwatch.IWidget, row *[]awscloudwatch.IWidget) []*[]awscloudwatch.IWidget {
	if row == nil || len(*row) == 0 {
		return rows
	}

	return append(rows, row)
}

func (m *EnhancedMonitoring) lambdaDashboardRows() []*[]awscloudwatch.IWidget {
	rows := make([]*[]awscloudwatch.IWidget, 0, 5)

	rows = appendRowIfNotEmpty(rows, m.lambdaSummaryRow())
	rows = appendRowIfNotEmpty(rows, m.lambdaRequestAndErrorsRow())
	rows = appendRowIfNotEmpty(rows, m.lambdaLatencyPercentilesRow())
	rows = appendRowIfNotEmpty(rows, m.lambdaOperationalRow())
	rows = appendRowIfNotEmpty(rows, m.lambdaActiveAlarmsRow())

	return rows
}

func (m *EnhancedMonitoring) lambdaSummaryRow() *[]awscloudwatch.IWidget {
	definitions := []struct {
		MetricKey string
		Title     string
	}{
		{MetricKey: "SuccessRate", Title: "Success Rate"},
		{MetricKey: "ErrorRate", Title: "Error Rate"},
		{MetricKey: "RequestRate", Title: "Request Rate"},
		{MetricKey: "LatencyP99", Title: "P99 Latency"},
	}

	row := make([]awscloudwatch.IWidget, 0, len(definitions))
	for _, definition := range definitions {
		if metric := m.Metrics[definition.MetricKey]; metric != nil {
			row = append(row, awscloudwatch.NewSingleValueWidget(&awscloudwatch.SingleValueWidgetProps{
				Title:   jsii.String(definition.Title),
				Metrics: &[]awscloudwatch.IMetric{metric},
				Width:   jsii.Number(6),
				Height:  jsii.Number(6),
			}))
		}
	}

	if len(row) == 0 {
		return nil
	}

	return &row
}

func (m *EnhancedMonitoring) lambdaRequestAndErrorsRow() *[]awscloudwatch.IWidget {
	return m.graphRow([]graphWidgetDefinition{
		{MetricKey: "Requests", Title: "Request Volume"},
		{MetricKey: "Errors", Title: "Errors"},
	})
}

func (m *EnhancedMonitoring) lambdaLatencyPercentilesRow() *[]awscloudwatch.IWidget {
	metricKeys := []string{"LatencyP50", "LatencyP95", "LatencyP99"}

	metrics := make([]awscloudwatch.IMetric, 0, len(metricKeys))
	for _, metricKey := range metricKeys {
		if metric := m.Metrics[metricKey]; metric != nil {
			metrics = append(metrics, metric)
		}
	}

	if len(metrics) == 0 {
		return nil
	}

	row := []awscloudwatch.IWidget{
		awscloudwatch.NewGraphWidget(&awscloudwatch.GraphWidgetProps{
			Title:  jsii.String("Latency Percentiles"),
			Left:   &metrics,
			Width:  jsii.Number(24),
			Height: jsii.Number(6),
		}),
	}

	return &row
}

func (m *EnhancedMonitoring) lambdaOperationalRow() *[]awscloudwatch.IWidget {
	row := make([]awscloudwatch.IWidget, 0, 2)

	if widget := m.lambdaColdStartsWidget(); widget != nil {
		row = append(row, widget)
	}
	if widget := m.lambdaResourceUtilizationWidget(); widget != nil {
		row = append(row, widget)
	}

	if len(row) == 0 {
		return nil
	}

	return &row
}

func (m *EnhancedMonitoring) lambdaColdStartsWidget() awscloudwatch.IWidget {
	left := make([]awscloudwatch.IMetric, 0, 1)
	if metric := m.Metrics["ColdStarts"]; metric != nil {
		left = append(left, metric)
	}

	right := make([]awscloudwatch.IMetric, 0, 1)
	if metric := m.Metrics["ColdStartDuration"]; metric != nil {
		right = append(right, metric)
	}

	if len(left) == 0 && len(right) == 0 {
		return nil
	}

	return awscloudwatch.NewGraphWidget(&awscloudwatch.GraphWidgetProps{
		Title:  jsii.String("Cold Starts"),
		Left:   &left,
		Right:  &right,
		Width:  jsii.Number(12),
		Height: jsii.Number(6),
	})
}

func (m *EnhancedMonitoring) lambdaResourceUtilizationWidget() awscloudwatch.IWidget {
	left := make([]awscloudwatch.IMetric, 0, 2)
	if metric := m.Metrics["ConcurrentExecutions"]; metric != nil {
		left = append(left, metric)
	}
	if metric := m.Metrics["MemoryUtilization"]; metric != nil {
		left = append(left, metric)
	}

	if len(left) == 0 {
		return nil
	}

	return awscloudwatch.NewGraphWidget(&awscloudwatch.GraphWidgetProps{
		Title:  jsii.String("Resource Utilization"),
		Left:   &left,
		Width:  jsii.Number(12),
		Height: jsii.Number(6),
	})
}

func (m *EnhancedMonitoring) lambdaActiveAlarmsRow() *[]awscloudwatch.IWidget {
	if alarm := m.Alarms["HighErrorRate"]; alarm != nil {
		row := []awscloudwatch.IWidget{
			awscloudwatch.NewAlarmWidget(&awscloudwatch.AlarmWidgetProps{
				Title:  jsii.String("Active Alarms"),
				Alarm:  alarm,
				Width:  jsii.Number(24),
				Height: jsii.Number(4),
			}),
		}
		return &row
	}

	return nil
}

func (m *EnhancedMonitoring) apiDashboardRows() []*[]awscloudwatch.IWidget {
	return m.graphRows(
		[]graphWidgetDefinition{
			{MetricKey: "APIRequests", Title: "API Requests"},
			{MetricKey: "APIIntegrationLatency", Title: "Integration Latency"},
		},
		[]graphWidgetDefinition{
			{MetricKey: "API4xxErrors", Title: "4xx Errors"},
			{MetricKey: "API5xxErrors", Title: "5xx Errors"},
		},
	)
}

func (m *EnhancedMonitoring) dynamoDashboardRows() []*[]awscloudwatch.IWidget {
	return m.graphRows(
		[]graphWidgetDefinition{
			{MetricKey: "ConsumedReadCapacity", Title: "Consumed Read Capacity"},
			{MetricKey: "ConsumedWriteCapacity", Title: "Consumed Write Capacity"},
		},
		[]graphWidgetDefinition{
			{MetricKey: "ReadThrottles", Title: "Read Throttles"},
			{MetricKey: "WriteThrottles", Title: "Write Throttles"},
		},
	)
}

// getMonitoringRetentionDays is reserved for future use
// Helper function to get retention days enum
/*
func getMonitoringRetentionDays(days float64) awslogs.RetentionDays {
	switch days {
	case 1:
		return awslogs.RetentionDays_ONE_DAY
	case 3:
		return awslogs.RetentionDays_THREE_DAYS
	case 5:
		return awslogs.RetentionDays_FIVE_DAYS
	case 7:
		return awslogs.RetentionDays_ONE_WEEK
	case 14:
		return awslogs.RetentionDays_TWO_WEEKS
	case 30:
		return awslogs.RetentionDays_ONE_MONTH
	case 60:
		return awslogs.RetentionDays_TWO_MONTHS
	case 90:
		return awslogs.RetentionDays_THREE_MONTHS
	case 120:
		return awslogs.RetentionDays_FOUR_MONTHS
	case 150:
		return awslogs.RetentionDays_FIVE_MONTHS
	case 180:
		return awslogs.RetentionDays_SIX_MONTHS
	case 365:
		return awslogs.RetentionDays_ONE_YEAR
	case 400:
		return awslogs.RetentionDays_THIRTEEN_MONTHS
	case 545:
		return awslogs.RetentionDays_EIGHTEEN_MONTHS
	case 731:
		return awslogs.RetentionDays_TWO_YEARS
	case 1827:
		return awslogs.RetentionDays_FIVE_YEARS
	case 3653:
		return awslogs.RetentionDays_TEN_YEARS
	default:
		return awslogs.RetentionDays_ONE_MONTH
	}
}
*/
