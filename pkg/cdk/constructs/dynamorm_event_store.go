package constructs

// DynamORMEventStore provides event sourcing capabilities using DynamORM
//
// IMPORTANT: This construct now uses standard pk/sk naming for DynamORM compatibility.
// Instead of aggregate_id/event_sequence, data should be stored as:
//   - Events: pk="event#{aggregate_id}", sk="seq#{event_sequence}"
//   - Snapshots: pk="snapshot#{aggregate_id}", sk="ver#{snapshot_version}"
//
// Example DynamORM models:
//
// type Event struct {
//     PK         string    `dynamorm:"pk"`                          // event#{aggregate_id}
//     SK         string    `dynamorm:"sk"`                          // seq#{sequence_number}
//
//     // Indexes
//     EventType  string    `dynamorm:"index:type-index,pk"`         // event_type
//     Timestamp  string    `dynamorm:"index:type-index,sk"`         // ISO timestamp
//     TenantID   string    `dynamorm:"index:tenant-index,pk"`       // tenant_id (if multi-tenant)
//
//     // Event data
//     AggregateID    string `json:"aggregate_id"`
//     EventSequence  int64  `json:"event_sequence"`
//     EventData      string `json:"event_data"`
//     TTL            int64  `json:"ttl,omitempty"`
// }

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// EventStorePattern defines the event store pattern to use
type EventStorePattern string

const (
	EventStorePattern_SINGLE_TABLE    EventStorePattern = "SINGLE_TABLE"
	EventStorePattern_MULTI_TABLE     EventStorePattern = "MULTI_TABLE"
	EventStorePattern_AGGREGATE_TABLE EventStorePattern = "AGGREGATE_TABLE"
)

// SnapshotStrategy defines how snapshots are handled
type SnapshotStrategy string

const (
	SnapshotStrategy_DISABLED   SnapshotStrategy = "DISABLED"
	SnapshotStrategy_FREQUENCY  SnapshotStrategy = "FREQUENCY"
	SnapshotStrategy_SIZE_BASED SnapshotStrategy = "SIZE_BASED"
	SnapshotStrategy_TIME_BASED SnapshotStrategy = "TIME_BASED"
)

// DynamORMEventStoreProps defines properties for DynamORM event store
type DynamORMEventStoreProps struct {
	// Event store pattern
	Pattern EventStorePattern

	// Table configuration
	EventTableName    *string
	SnapshotTableName *string

	// Multi-tenant configuration
	EnableMultiTenant *bool
	TenantAttribute   *string

	// Event configuration
	EnableEventVersioning  *bool
	EnableEventEncryption  *bool
	EnableEventCompression *bool
	EventTTL               awscdk.Duration // TTL for old events

	// Snapshot configuration
	SnapshotStrategy     SnapshotStrategy
	SnapshotFrequency    *int            // Number of events between snapshots
	SnapshotSizeLimit    *int            // Size limit in KB for snapshots
	SnapshotTimeInterval awscdk.Duration // Time interval for snapshots
	SnapshotRetention    awscdk.Duration // How long to keep snapshots

	// Performance configuration
	EventStreamEnabled    *bool    // Enable DynamoDB streams for events
	SnapshotStreamEnabled *bool    // Enable DynamoDB streams for snapshots
	EnableAutoScaling     *bool    // Enable auto-scaling
	ReadCapacity          *float64 // Read capacity units
	WriteCapacity         *float64 // Write capacity units

	// Archival configuration
	EnableArchival *bool           // Enable event archival to S3
	ArchivalBucket awss3.IBucket   // S3 bucket for archival
	ArchivalAfter  awscdk.Duration // Archive events after this duration

	// Monitoring configuration
	EnableMetrics         *bool // Enable CloudWatch metrics
	EnableDetailedMetrics *bool // Enable detailed monitoring
	AlertThresholds       *EventStoreAlertThresholds

	// Security configuration
	EnableEncryption *bool   // Enable encryption at rest
	KMSKey           *string // KMS key for encryption

	// Query optimization
	EnableGSIs        *bool    // Enable Global Secondary Indexes
	ProjectionQueries []string // Queries for projection views

	// Tags
	Tags *map[string]*string
}

// EventStoreAlertThresholds defines alert thresholds for event store monitoring
type EventStoreAlertThresholds struct {
	HighEventRate        *float64 // Events per second threshold
	HighErrorRate        *float64 // Error rate threshold
	HighLatency          *float64 // Latency threshold (ms)
	LowSnapshotFrequency *float64 // Minimum snapshot frequency
	HighStorageUsage     *float64 // Storage usage threshold (GB)
}

// DynamORMEventStore provides event sourcing capabilities using DynamORM
type DynamORMEventStore struct {
	constructs.Construct

	// Event table for storing events
	EventTable *LiftTable

	// Snapshot table for storing snapshots
	SnapshotTable *LiftTable

	// S3 bucket for archival (if enabled)
	ArchivalBucket awss3.IBucket

	// Configuration
	props *DynamORMEventStoreProps

	// CloudWatch metrics
	Metrics map[string]awscloudwatch.Metric

	// IAM roles for different access patterns
	EventReaderRole     awsiam.Role
	EventWriterRole     awsiam.Role
	SnapshotManagerRole awsiam.Role
}

// NewDynamORMEventStore creates a new DynamORM event store construct
func NewDynamORMEventStore(scope constructs.Construct, id *string, props *DynamORMEventStoreProps) *DynamORMEventStore {
	this := &DynamORMEventStore{}
	constructs.NewConstruct_Override(this, scope, id)

	// Validate and set defaults
	if props == nil {
		props = &DynamORMEventStoreProps{}
	}
	props = this.applyDefaults(props)

	this.props = props

	// Create event table
	this.createEventTable()

	// Create snapshot table if snapshots are enabled
	if props.SnapshotStrategy != SnapshotStrategy_DISABLED {
		this.createSnapshotTable()
	}

	// Create archival bucket if archival is enabled
	if props.EnableArchival != nil && *props.EnableArchival {
		this.createArchivalBucket()
	}

	// Create IAM roles
	this.createIAMRoles()

	// Set up monitoring if enabled
	if props.EnableMetrics != nil && *props.EnableMetrics {
		this.createEventStoreMetrics()
	}

	// Set up detailed monitoring if enabled
	if props.EnableDetailedMetrics != nil && *props.EnableDetailedMetrics {
		this.createDetailedMonitoring()
	}

	return this
}

// applyDefaults applies default values to event store properties
func (e *DynamORMEventStore) applyDefaults(props *DynamORMEventStoreProps) *DynamORMEventStoreProps {
	if props.Pattern == "" {
		props.Pattern = EventStorePattern_SINGLE_TABLE
	}
	if props.EventTableName == nil {
		props.EventTableName = jsii.String("event-store")
	}
	if props.SnapshotTableName == nil {
		props.SnapshotTableName = jsii.String("event-snapshots")
	}
	if props.EnableMultiTenant == nil {
		props.EnableMultiTenant = jsii.Bool(false)
	}
	if props.TenantAttribute == nil {
		props.TenantAttribute = jsii.String("TenantID")
	}
	if props.EnableEventVersioning == nil {
		props.EnableEventVersioning = jsii.Bool(true)
	}
	if props.EnableEventEncryption == nil {
		props.EnableEventEncryption = jsii.Bool(true)
	}
	if props.EnableEventCompression == nil {
		props.EnableEventCompression = jsii.Bool(false)
	}
	if props.EventTTL == nil {
		props.EventTTL = awscdk.Duration_Days(jsii.Number(365 * 7)) // 7 years default
	}
	if props.SnapshotStrategy == "" {
		props.SnapshotStrategy = SnapshotStrategy_FREQUENCY
	}
	if props.SnapshotFrequency == nil {
		i := 100 // Every 100 events
		props.SnapshotFrequency = &i
	}
	if props.SnapshotSizeLimit == nil {
		i := 1024 // 1MB
		props.SnapshotSizeLimit = &i
	}
	if props.SnapshotTimeInterval == nil {
		props.SnapshotTimeInterval = awscdk.Duration_Hours(jsii.Number(24)) // Daily
	}
	if props.SnapshotRetention == nil {
		props.SnapshotRetention = awscdk.Duration_Days(jsii.Number(90)) // 90 days
	}
	if props.EventStreamEnabled == nil {
		props.EventStreamEnabled = jsii.Bool(true)
	}
	if props.SnapshotStreamEnabled == nil {
		props.SnapshotStreamEnabled = jsii.Bool(false)
	}
	if props.EnableAutoScaling == nil {
		props.EnableAutoScaling = jsii.Bool(true)
	}
	if props.EnableArchival == nil {
		props.EnableArchival = jsii.Bool(false)
	}
	if props.ArchivalAfter == nil {
		props.ArchivalAfter = awscdk.Duration_Days(jsii.Number(365)) // 1 year
	}
	if props.EnableMetrics == nil {
		props.EnableMetrics = jsii.Bool(true)
	}
	if props.EnableDetailedMetrics == nil {
		props.EnableDetailedMetrics = jsii.Bool(false)
	}
	if props.EnableEncryption == nil {
		props.EnableEncryption = jsii.Bool(true)
	}
	if props.EnableGSIs == nil {
		props.EnableGSIs = jsii.Bool(true)
	}

	return props
}

// createEventTable creates the main event table
func (e *DynamORMEventStore) createEventTable() {
	// Create table props using standard pk/sk naming
	tableProps := &LiftTableProps{
		TableName:                 e.props.EventTableName,
		EnableAutoScaling:         e.props.EnableAutoScaling,
		ReadCapacity:              e.props.ReadCapacity,
		WriteCapacity:             e.props.WriteCapacity,
		EnablePointInTimeRecovery: jsii.Bool(true),
	}

	// Configure streams if enabled
	if e.props.EventStreamEnabled != nil && *e.props.EventStreamEnabled {
		tableProps.EnableStreams = jsii.Bool(true)
		tableProps.StreamViewType = awsdynamodb.StreamViewType_NEW_AND_OLD_IMAGES
	}

	// Configure TTL if specified
	if e.props.EventTTL != nil {
		tableProps.TimeToLiveAttribute = jsii.String("ttl")
	}

	// Create the event table with field names from Event struct
	tableProps.PartitionKeyName = jsii.String("PK")
	tableProps.SortKeyName = jsii.String("SK")
	e.EventTable = NewLiftTable(e, jsii.String("EventTable"), tableProps)

	// Add GSIs if enabled
	if e.props.EnableGSIs != nil && *e.props.EnableGSIs {
		e.addEventTableGSIs()
	}

	// DynamORM configuration is now handled through model struct tags
	// Multi-tenant GSIs are defined in DynamORM models
}

// addEventTableGSIs adds Global Secondary Indexes to the event table
func (e *DynamORMEventStore) addEventTableGSIs() {
	// GSIs are now defined in DynamORM models using struct tags
	// Example model for events:
	//
	// type Event struct {
	//     PK            string `dynamorm:"pk"`                          // event#{aggregate_id}
	//     SK            string `dynamorm:"sk"`                          // seq#{sequence}
	//     EventType     string `dynamorm:"index:event-type,pk"`         // For querying by type
	//     CreatedAt     string `dynamorm:"index:event-type,sk"`         // For sorting by time
	//     AggregateType string `dynamorm:"index:aggregate-type,pk"`     // For aggregate queries
	//     CorrelationID string `dynamorm:"index:correlation,pk"`        // For correlation tracking
	//     EventDay      string `dynamorm:"index:timeline,pk"`          // For timeline queries
	//     // ... other fields
	// }
}

// createSnapshotTable creates the snapshot table
func (e *DynamORMEventStore) createSnapshotTable() {
	// For AGGREGATE_TABLE pattern, reuse the event table
	if e.props.Pattern == EventStorePattern_AGGREGATE_TABLE {
		e.SnapshotTable = e.EventTable
		return
	}
	// Create table props using standard pk/sk naming
	tableProps := &LiftTableProps{
		TableName:                 e.props.SnapshotTableName,
		EnableAutoScaling:         e.props.EnableAutoScaling,
		EnablePointInTimeRecovery: jsii.Bool(true),
	}

	// Configure streams if enabled
	if e.props.SnapshotStreamEnabled != nil && *e.props.SnapshotStreamEnabled {
		tableProps.EnableStreams = jsii.Bool(true)
		tableProps.StreamViewType = awsdynamodb.StreamViewType_NEW_AND_OLD_IMAGES
	}

	// Configure TTL for snapshot retention
	if e.props.SnapshotRetention != nil {
		tableProps.TimeToLiveAttribute = jsii.String("ttl")
	}

	// Configure capacity if specified
	if e.props.ReadCapacity != nil && e.props.WriteCapacity != nil {
		tableProps.ReadCapacity = jsii.Number(*e.props.ReadCapacity * 0.3)   // 30% of event table capacity
		tableProps.WriteCapacity = jsii.Number(*e.props.WriteCapacity * 0.1) // 10% of event table capacity
	}

	// Create the snapshot table with field names from Snapshot struct
	tableProps.PartitionKeyName = jsii.String("PK")
	tableProps.SortKeyName = jsii.String("SK")
	e.SnapshotTable = NewLiftTable(e, jsii.String("SnapshotTable"), tableProps)

	// Add snapshot-specific GSIs
	e.addSnapshotTableGSIs()

	// Only configure if it's a separate table (not AGGREGATE_TABLE pattern)
	// DynamORM configuration is now handled through model struct tags
	// Multi-tenant GSIs are defined in DynamORM models using tags like:
	// TenantID string `dynamorm:"index:tenant-entity,pk"`
	// Future table-specific configuration can be added here for non-aggregate patterns
}

// addSnapshotTableGSIs adds Global Secondary Indexes to the snapshot table
func (e *DynamORMEventStore) addSnapshotTableGSIs() {
	// Skip for AGGREGATE_TABLE pattern as GSIs are already added to event table
	if e.props.Pattern == EventStorePattern_AGGREGATE_TABLE {
		return
	}
	// GSIs are now defined in DynamORM models using struct tags
	// Example model for snapshots:
	//
	// type Snapshot struct {
	//     PK           string `dynamorm:"pk"`                        // snapshot#{aggregate_id}
	//     SK           string `dynamorm:"sk"`                        // ver#{version}
	//     AggregateType string `dynamorm:"index:aggregate-type,pk"`  // For querying by type
	//     CreatedAt    string `dynamorm:"index:aggregate-type,sk"`   // For sorting by time
	//     IsLatest     string `dynamorm:"index:latest,pk"`          // For finding latest snapshots
	//     // ... other fields
	// }
}

// createArchivalBucket creates S3 bucket for event archival
func (e *DynamORMEventStore) createArchivalBucket() {
	if e.props.ArchivalBucket != nil {
		e.ArchivalBucket = e.props.ArchivalBucket
		return
	}

	// Create new S3 bucket for archival
	e.ArchivalBucket = awss3.NewBucket(e, jsii.String("ArchivalBucket"), &awss3.BucketProps{
		BucketName:        jsii.String(fmt.Sprintf("%s-event-archive", *e.props.EventTableName)),
		Versioned:         jsii.Bool(true),
		Encryption:        awss3.BucketEncryption_S3_MANAGED,
		BlockPublicAccess: awss3.BlockPublicAccess_BLOCK_ALL(),
		LifecycleRules: &[]*awss3.LifecycleRule{
			{
				Id:      jsii.String("archive-lifecycle"),
				Enabled: jsii.Bool(true),
				Transitions: &[]*awss3.Transition{
					{
						StorageClass:    awss3.StorageClass_INFREQUENT_ACCESS(),
						TransitionAfter: awscdk.Duration_Days(jsii.Number(30)),
					},
					{
						StorageClass:    awss3.StorageClass_GLACIER(),
						TransitionAfter: awscdk.Duration_Days(jsii.Number(90)),
					},
					{
						StorageClass:    awss3.StorageClass_DEEP_ARCHIVE(),
						TransitionAfter: awscdk.Duration_Days(jsii.Number(365)),
					},
				},
			},
		},
	})
}

// createIAMRoles creates IAM roles for different access patterns
func (e *DynamORMEventStore) createIAMRoles() {
	// Event reader role
	e.EventReaderRole = awsiam.NewRole(e, jsii.String("EventReaderRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("lambda.amazonaws.com"), nil),
		ManagedPolicies: &[]awsiam.IManagedPolicy{
			awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("service-role/AWSLambdaBasicExecutionRole")),
		},
	})

	// Grant read access to event table
	e.EventTable.Table.GrantReadData(e.EventReaderRole)
	if e.SnapshotTable != nil {
		e.SnapshotTable.Table.GrantReadData(e.EventReaderRole)
	}

	// Event writer role
	e.EventWriterRole = awsiam.NewRole(e, jsii.String("EventWriterRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("lambda.amazonaws.com"), nil),
		ManagedPolicies: &[]awsiam.IManagedPolicy{
			awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("service-role/AWSLambdaBasicExecutionRole")),
		},
	})

	// Grant write access to event table
	e.EventTable.Table.GrantWriteData(e.EventWriterRole)

	// Snapshot manager role
	e.SnapshotManagerRole = awsiam.NewRole(e, jsii.String("SnapshotManagerRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("lambda.amazonaws.com"), nil),
		ManagedPolicies: &[]awsiam.IManagedPolicy{
			awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("service-role/AWSLambdaBasicExecutionRole")),
		},
	})

	// Grant read access to event table and read/write access to snapshot table
	e.EventTable.Table.GrantReadData(e.SnapshotManagerRole)
	if e.SnapshotTable != nil {
		e.SnapshotTable.Table.GrantReadWriteData(e.SnapshotManagerRole)
	}

	// Grant archival permissions if archival is enabled
	if e.props.EnableArchival != nil && *e.props.EnableArchival && e.ArchivalBucket != nil {
		e.ArchivalBucket.GrantReadWrite(e.SnapshotManagerRole, nil)
	}

	// Add CloudWatch permissions for metrics
	for _, role := range []awsiam.Role{e.EventReaderRole, e.EventWriterRole, e.SnapshotManagerRole} {
		role.AddToPrincipalPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
			Effect: awsiam.Effect_ALLOW,
			Actions: &[]*string{
				jsii.String("cloudwatch:PutMetricData"),
			},
			Resources: &[]*string{
				jsii.String("*"),
			},
			Conditions: &map[string]interface{}{
				"StringEquals": map[string]interface{}{
					"cloudwatch:namespace": []string{"DynamORM/EventStore"},
				},
			},
		}))
	}
}

// createEventStoreMetrics creates CloudWatch metrics for event store monitoring
func (e *DynamORMEventStore) createEventStoreMetrics() {
	e.Metrics = make(map[string]awscloudwatch.Metric)
	tableName := *e.EventTable.GetTableName()

	// Events written per second
	e.Metrics["EventsWritten"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("DynamORM/EventStore"),
		MetricName: jsii.String("EventsWritten"),
		DimensionsMap: &map[string]*string{
			"TableName": jsii.String(tableName),
			"Pattern":   jsii.String(string(e.props.Pattern)),
		},
		Statistic: jsii.String(string(awscloudwatch.Statistic_SUM)),
		Period:    awscdk.Duration_Minutes(jsii.Number(1)),
	})

	// Events read per second
	e.Metrics["EventsRead"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("DynamORM/EventStore"),
		MetricName: jsii.String("EventsRead"),
		DimensionsMap: &map[string]*string{
			"TableName": jsii.String(tableName),
			"Pattern":   jsii.String(string(e.props.Pattern)),
		},
		Statistic: jsii.String(string(awscloudwatch.Statistic_SUM)),
		Period:    awscdk.Duration_Minutes(jsii.Number(1)),
	})

	// Snapshot operations
	if e.SnapshotTable != nil {
		e.Metrics["SnapshotsCreated"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("DynamORM/EventStore"),
			MetricName: jsii.String("SnapshotsCreated"),
			DimensionsMap: &map[string]*string{
				"TableName":        jsii.String(tableName),
				"SnapshotStrategy": jsii.String(string(e.props.SnapshotStrategy)),
			},
			Statistic: jsii.String(string(awscloudwatch.Statistic_SUM)),
			Period:    awscdk.Duration_Minutes(jsii.Number(5)),
		})

		e.Metrics["SnapshotLatency"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("DynamORM/EventStore"),
			MetricName: jsii.String("SnapshotLatency"),
			DimensionsMap: &map[string]*string{
				"TableName":        jsii.String(tableName),
				"SnapshotStrategy": jsii.String(string(e.props.SnapshotStrategy)),
			},
			Statistic: jsii.String(string(awscloudwatch.Statistic_AVERAGE)),
			Period:    awscdk.Duration_Minutes(jsii.Number(5)),
		})
	}

	// Event store latency
	e.Metrics["EventStoreLatency"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("DynamORM/EventStore"),
		MetricName: jsii.String("EventStoreLatency"),
		DimensionsMap: &map[string]*string{
			"TableName": jsii.String(tableName),
			"Operation": jsii.String("Write"),
		},
		Statistic: jsii.String(string(awscloudwatch.Statistic_AVERAGE)),
		Period:    awscdk.Duration_Minutes(jsii.Number(5)),
	})

	// Event store errors
	e.Metrics["EventStoreErrors"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("DynamORM/EventStore"),
		MetricName: jsii.String("EventStoreErrors"),
		DimensionsMap: &map[string]*string{
			"TableName": jsii.String(tableName),
		},
		Statistic: jsii.String(string(awscloudwatch.Statistic_SUM)),
		Period:    awscdk.Duration_Minutes(jsii.Number(1)),
	})

	// Storage metrics
	e.Metrics["StorageSize"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("DynamORM/EventStore"),
		MetricName: jsii.String("StorageSize"),
		DimensionsMap: &map[string]*string{
			"TableName": jsii.String(tableName),
		},
		Statistic: jsii.String(string(awscloudwatch.Statistic_AVERAGE)),
		Period:    awscdk.Duration_Hours(jsii.Number(1)),
	})

	// Multi-tenant metrics if enabled
	if e.props.EnableMultiTenant != nil && *e.props.EnableMultiTenant {
		e.Metrics["TenantEventRate"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("DynamORM/EventStore/Tenant"),
			MetricName: jsii.String("EventRate"),
			DimensionsMap: &map[string]*string{
				"TableName":       jsii.String(tableName),
				"TenantAttribute": e.props.TenantAttribute,
			},
			Statistic: jsii.String(string(awscloudwatch.Statistic_SUM)),
			Period:    awscdk.Duration_Minutes(jsii.Number(5)),
		})
	}
}

// createDetailedMonitoring creates detailed monitoring with alarms
func (e *DynamORMEventStore) createDetailedMonitoring() {
	if e.Metrics == nil {
		e.createEventStoreMetrics()
	}

	tableName := *e.EventTable.GetTableName()
	thresholds := e.props.AlertThresholds

	// Set default thresholds if not provided
	if thresholds == nil {
		thresholds = &EventStoreAlertThresholds{
			HighEventRate:        jsii.Number(1000), // 1000 events/second
			HighErrorRate:        jsii.Number(5),    // 5 errors/minute
			HighLatency:          jsii.Number(100),  // 100ms
			LowSnapshotFrequency: jsii.Number(1),    // At least 1 snapshot/hour
			HighStorageUsage:     jsii.Number(100),  // 100GB
		}
	}

	// High event rate alarm
	if thresholds.HighEventRate != nil {
		awscloudwatch.NewAlarm(e, jsii.String("HighEventRateAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:          jsii.String(fmt.Sprintf("%s-event-store-high-rate", tableName)),
			AlarmDescription:   jsii.String("Event store is receiving high event rate"),
			Metric:             e.Metrics["EventsWritten"],
			Threshold:          thresholds.HighEventRate,
			EvaluationPeriods:  jsii.Number(2),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
			TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		})
	}

	// High error rate alarm
	if thresholds.HighErrorRate != nil {
		awscloudwatch.NewAlarm(e, jsii.String("HighErrorRateAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:          jsii.String(fmt.Sprintf("%s-event-store-errors", tableName)),
			AlarmDescription:   jsii.String("Event store is experiencing high error rate"),
			Metric:             e.Metrics["EventStoreErrors"],
			Threshold:          thresholds.HighErrorRate,
			EvaluationPeriods:  jsii.Number(2),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
			TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		})
	}

	// High latency alarm
	if thresholds.HighLatency != nil {
		awscloudwatch.NewAlarm(e, jsii.String("HighLatencyAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:          jsii.String(fmt.Sprintf("%s-event-store-latency", tableName)),
			AlarmDescription:   jsii.String("Event store latency is too high"),
			Metric:             e.Metrics["EventStoreLatency"],
			Threshold:          thresholds.HighLatency,
			EvaluationPeriods:  jsii.Number(3),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
			TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		})
	}

	// Low snapshot frequency alarm
	if thresholds.LowSnapshotFrequency != nil && e.Metrics["SnapshotsCreated"] != nil {
		awscloudwatch.NewAlarm(e, jsii.String("LowSnapshotFrequencyAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:          jsii.String(fmt.Sprintf("%s-low-snapshot-frequency", tableName)),
			AlarmDescription:   jsii.String("Snapshot frequency is too low"),
			Metric:             e.Metrics["SnapshotsCreated"],
			Threshold:          thresholds.LowSnapshotFrequency,
			EvaluationPeriods:  jsii.Number(3),
			ComparisonOperator: awscloudwatch.ComparisonOperator_LESS_THAN_THRESHOLD,
			TreatMissingData:   awscloudwatch.TreatMissingData_BREACHING,
		})
	}

	// High storage usage alarm
	if thresholds.HighStorageUsage != nil {
		awscloudwatch.NewAlarm(e, jsii.String("HighStorageUsageAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:          jsii.String(fmt.Sprintf("%s-high-storage", tableName)),
			AlarmDescription:   jsii.String("Event store storage usage is high"),
			Metric:             e.Metrics["StorageSize"],
			Threshold:          thresholds.HighStorageUsage,
			EvaluationPeriods:  jsii.Number(2),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
			TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		})
	}
}

// GetEnvironmentVariables returns environment variables for Lambda functions
func (e *DynamORMEventStore) GetEnvironmentVariables() *map[string]*string {
	env := make(map[string]*string)

	// Basic event store configuration
	env["DYNAMORM_EVENT_STORE_ENABLED"] = jsii.String("true")
	env["DYNAMORM_EVENT_STORE_PATTERN"] = jsii.String(string(e.props.Pattern))
	env["DYNAMORM_EVENT_TABLE_NAME"] = e.EventTable.GetTableName()
	env["DYNAMORM_EVENT_TABLE_ARN"] = e.EventTable.GetTableArn()

	if e.SnapshotTable != nil {
		env["DYNAMORM_SNAPSHOT_TABLE_NAME"] = e.SnapshotTable.GetTableName()
		env["DYNAMORM_SNAPSHOT_TABLE_ARN"] = e.SnapshotTable.GetTableArn()
		env["DYNAMORM_SNAPSHOT_STRATEGY"] = jsii.String(string(e.props.SnapshotStrategy))
		env["DYNAMORM_SNAPSHOT_FREQUENCY"] = jsii.String(fmt.Sprintf("%d", *e.props.SnapshotFrequency))
	}

	// Event configuration
	if e.props.EnableEventVersioning != nil && *e.props.EnableEventVersioning {
		env["DYNAMORM_EVENT_VERSIONING"] = jsii.String("true")
	}
	if e.props.EnableEventEncryption != nil && *e.props.EnableEventEncryption {
		env["DYNAMORM_EVENT_ENCRYPTION"] = jsii.String("true")
	}
	if e.props.EnableEventCompression != nil && *e.props.EnableEventCompression {
		env["DYNAMORM_EVENT_COMPRESSION"] = jsii.String("true")
	}

	// Multi-tenant configuration
	if e.props.EnableMultiTenant != nil && *e.props.EnableMultiTenant {
		env["DYNAMORM_EVENT_STORE_MULTI_TENANT"] = jsii.String("true")
		env["DYNAMORM_TENANT_ATTRIBUTE"] = e.props.TenantAttribute
	}

	// Streams configuration
	if e.props.EventStreamEnabled != nil && *e.props.EventStreamEnabled {
		if e.EventTable.GetStreamArn() != nil {
			env["DYNAMORM_EVENT_STREAM_ARN"] = e.EventTable.GetStreamArn()
		}
	}
	if e.props.SnapshotStreamEnabled != nil && *e.props.SnapshotStreamEnabled && e.SnapshotTable != nil {
		if e.SnapshotTable.GetStreamArn() != nil {
			env["DYNAMORM_SNAPSHOT_STREAM_ARN"] = e.SnapshotTable.GetStreamArn()
		}
	}

	// Archival configuration
	if e.props.EnableArchival != nil && *e.props.EnableArchival && e.ArchivalBucket != nil {
		env["DYNAMORM_ARCHIVAL_ENABLED"] = jsii.String("true")
		env["DYNAMORM_ARCHIVAL_BUCKET"] = e.ArchivalBucket.BucketName()
		if e.props.ArchivalAfter != nil {
			days := e.props.ArchivalAfter.ToDays(nil)
			if days != nil {
				env["DYNAMORM_ARCHIVAL_AFTER_DAYS"] = jsii.String(fmt.Sprintf("%.0f", *days))
			}
		}
	}

	// Metrics configuration
	if e.props.EnableMetrics != nil && *e.props.EnableMetrics {
		env["DYNAMORM_EVENT_STORE_METRICS"] = jsii.String("true")
	}

	return &env
}

// GrantEventReaderAccess grants event reader access to a Lambda function
func (e *DynamORMEventStore) GrantEventReaderAccess(grantee awslambda.IFunction) {
	e.EventTable.Table.GrantReadData(awsiam.IGrantable(grantee))
	if e.SnapshotTable != nil {
		e.SnapshotTable.Table.GrantReadData(awsiam.IGrantable(grantee))
	}
}

// GrantEventWriterAccess grants event writer access to a Lambda function
func (e *DynamORMEventStore) GrantEventWriterAccess(grantee awslambda.IFunction) {
	e.EventTable.Table.GrantWriteData(awsiam.IGrantable(grantee))
}

// GrantSnapshotManagerAccess grants snapshot manager access to a Lambda function
func (e *DynamORMEventStore) GrantSnapshotManagerAccess(grantee awslambda.IFunction) {
	e.EventTable.Table.GrantReadData(awsiam.IGrantable(grantee))
	if e.SnapshotTable != nil {
		e.SnapshotTable.GrantReadWrite(grantee)
	}
	if e.props.EnableArchival != nil && *e.props.EnableArchival && e.ArchivalBucket != nil {
		e.ArchivalBucket.GrantReadWrite(grantee, nil)
	}
}

// GrantFullAccess grants full event store access to a Lambda function
func (e *DynamORMEventStore) GrantFullAccess(grantee awslambda.IFunction) {
	e.EventTable.GrantReadWrite(grantee)
	if e.SnapshotTable != nil {
		e.SnapshotTable.GrantReadWrite(grantee)
	}
	if e.props.EnableArchival != nil && *e.props.EnableArchival && e.ArchivalBucket != nil {
		e.ArchivalBucket.GrantReadWrite(grantee, nil)
	}
}

// GetEventTable returns the event table
func (e *DynamORMEventStore) GetEventTable() *LiftTable {
	return e.EventTable
}

// GetSnapshotTable returns the snapshot table
func (e *DynamORMEventStore) GetSnapshotTable() *LiftTable {
	return e.SnapshotTable
}

// GetArchivalBucket returns the archival bucket
func (e *DynamORMEventStore) GetArchivalBucket() awss3.IBucket {
	return e.ArchivalBucket
}

// GetEventStoreMetrics returns event store CloudWatch metrics
func (e *DynamORMEventStore) GetEventStoreMetrics() map[string]awscloudwatch.Metric {
	return e.Metrics
}

// GetEventReaderRole returns the event reader IAM role
func (e *DynamORMEventStore) GetEventReaderRole() awsiam.Role {
	return e.EventReaderRole
}

// GetEventWriterRole returns the event writer IAM role
func (e *DynamORMEventStore) GetEventWriterRole() awsiam.Role {
	return e.EventWriterRole
}

// GetSnapshotManagerRole returns the snapshot manager IAM role
func (e *DynamORMEventStore) GetSnapshotManagerRole() awsiam.Role {
	return e.SnapshotManagerRole
}
