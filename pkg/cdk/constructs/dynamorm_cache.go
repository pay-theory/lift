package constructs

import (
	"fmt"
	
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsmemorydb"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// CacheStrategy defines the caching strategy to use
type CacheStrategy string

const (
	CacheStrategy_IN_MEMORY     CacheStrategy = "IN_MEMORY"
	CacheStrategy_REDIS         CacheStrategy = "REDIS"
	CacheStrategy_MEMORYDB      CacheStrategy = "MEMORYDB"
	CacheStrategy_HYBRID        CacheStrategy = "HYBRID"
)

// CacheInvalidationStrategy defines how cache invalidation works
type CacheInvalidationStrategy string

const (
	CacheInvalidationStrategy_TTL           CacheInvalidationStrategy = "TTL"
	CacheInvalidationStrategy_WRITE_THROUGH CacheInvalidationStrategy = "WRITE_THROUGH"
	CacheInvalidationStrategy_STREAM_BASED  CacheInvalidationStrategy = "STREAM_BASED"
	CacheInvalidationStrategy_MANUAL        CacheInvalidationStrategy = "MANUAL"
)

// DynamORMCacheProps defines properties for DynamORM caching
type DynamORMCacheProps struct {
	// Required: The DynamORM table to cache
	DynamORMTable *DynamORMTable

	// Caching strategy
	CacheStrategy CacheStrategy

	// Cache invalidation strategy
	InvalidationStrategy CacheInvalidationStrategy

	// In-memory cache configuration
	InMemoryConfig *InMemoryCacheConfig

	// Redis configuration (for external Redis)
	RedisConfig *RedisCacheConfig

	// MemoryDB configuration (for AWS MemoryDB)
	MemoryDBConfig *MemoryDBCacheConfig

	// Cache behavior settings
	DefaultTTL           awscdk.Duration  // Default TTL for cached items
	MaxCacheSize         *int             // Maximum cache size (in-memory only)
	EnableMetrics        *bool            // Enable cache metrics
	EnableCompression    *bool            // Enable cache value compression
	
	// Multi-tenant settings
	EnableTenantIsolation *bool           // Enable tenant-specific caching
	TenantAttribute       *string         // Tenant attribute name
	
	// Performance settings
	PrefetchPatterns     []string         // Patterns to prefetch
	WarmupQueries        []string         // Queries to run on startup
	
	// Monitoring
	EnableDetailedMetrics *bool           // Enable detailed cache metrics
	AlertThresholds      *CacheAlertThresholds
}

// InMemoryCacheConfig defines in-memory cache configuration
type InMemoryCacheConfig struct {
	MaxSize              *int             // Maximum number of items
	TTL                  awscdk.Duration  // Time to live
	EvictionPolicy       *string          // LRU, LFU, FIFO
	ConcurrencyLevel     *int             // Concurrency level for map
	EnableStatistics     *bool            // Enable cache statistics
}

// RedisCacheConfig defines Redis cache configuration
type RedisCacheConfig struct {
	Host                 *string          // Redis host
	Port                 *int             // Redis port
	Password             *string          // Redis password
	Database             *int             // Redis database number
	MaxConnections       *int             // Maximum connections
	ConnectionTimeout    awscdk.Duration  // Connection timeout
	EnableTLS            *bool            // Enable TLS
}

// MemoryDBCacheConfig defines AWS MemoryDB configuration
type MemoryDBCacheConfig struct {
	ClusterName          *string          // MemoryDB cluster name
	NodeType             *string          // Node type (e.g., db.t4g.small)
	NumShards            *int             // Number of shards
	ReplicasPerShard     *int             // Replicas per shard
	ParameterGroup       *string          // Parameter group
	SecurityGroupIds     *[]*string       // Security group IDs
	SubnetGroupName      *string          // Subnet group name
}

// CacheAlertThresholds defines alert thresholds for cache monitoring
type CacheAlertThresholds struct {
	HighCacheHitRatio    *float64         // Alert if hit ratio above this
	LowCacheHitRatio     *float64         // Alert if hit ratio below this
	HighEvictionRate     *float64         // Alert if eviction rate above this
	HighMemoryUsage      *float64         // Alert if memory usage above this
	HighLatency          *float64         // Alert if latency above this (ms)
}

// DynamORMCache provides caching capabilities for DynamORM tables
type DynamORMCache struct {
	constructs.Construct

	// The DynamORM table being cached
	Table *DynamORMTable

	// Cache configuration
	props *DynamORMCacheProps

	// MemoryDB cluster (if using MemoryDB)
	MemoryDBCluster awsmemorydb.CfnCluster

	// CloudWatch metrics
	Metrics map[string]awscloudwatch.Metric

	// IAM role for cache access
	CacheAccessRole awsiam.Role
}

// validateCacheProps validates the required properties for DynamORMCache
func validateCacheProps(props *DynamORMCacheProps) error {
	if props == nil {
		return fmt.Errorf("DynamORMCacheProps cannot be nil")
	}
	if props.DynamORMTable == nil {
		return fmt.Errorf("DynamORMTable is required for DynamORMCache")
	}
	return nil
}

// NewDynamORMCache creates a new DynamORM cache construct
func NewDynamORMCache(scope constructs.Construct, id *string, props *DynamORMCacheProps) *DynamORMCache {
	this := &DynamORMCache{}
	constructs.NewConstruct_Override(this, scope, id)

	// Validate required properties
	if err := validateCacheProps(props); err != nil {
		// For CDK constructs, panic is acceptable during construction with clear error messages
		panic(fmt.Sprintf("DynamORMCache validation failed: %v", err))
	}

	// Set defaults
	props = this.applyDefaults(props)

	this.Table = props.DynamORMTable
	this.props = props

	// Create cache infrastructure based on strategy
	switch props.CacheStrategy {
	case CacheStrategy_MEMORYDB:
		this.createMemoryDBCluster()
	case CacheStrategy_REDIS:
		// Redis configuration is handled by the application
	case CacheStrategy_IN_MEMORY, CacheStrategy_HYBRID:
		// In-memory caching is handled by the application
	}

	// Create IAM role for cache access
	this.createCacheAccessRole()

	// Set up monitoring if enabled
	if props.EnableMetrics != nil && *props.EnableMetrics {
		this.createCacheMetrics()
	}

	// Set up detailed monitoring if enabled
	if props.EnableDetailedMetrics != nil && *props.EnableDetailedMetrics {
		this.createDetailedMonitoring()
	}

	return this
}

// applyDefaults applies default values to cache properties
func (d *DynamORMCache) applyDefaults(props *DynamORMCacheProps) *DynamORMCacheProps {
	if props.CacheStrategy == "" {
		props.CacheStrategy = CacheStrategy_IN_MEMORY
	}
	if props.InvalidationStrategy == "" {
		props.InvalidationStrategy = CacheInvalidationStrategy_TTL
	}
	if props.DefaultTTL == nil {
		props.DefaultTTL = awscdk.Duration_Minutes(jsii.Number(15))
	}
	if props.MaxCacheSize == nil {
		props.MaxCacheSize = intPtr(10000)
	}
	if props.EnableMetrics == nil {
		props.EnableMetrics = jsii.Bool(true)
	}
	if props.EnableCompression == nil {
		props.EnableCompression = jsii.Bool(false)
	}
	if props.EnableTenantIsolation == nil {
		props.EnableTenantIsolation = jsii.Bool(false)
	}
	if props.TenantAttribute == nil {
		props.TenantAttribute = jsii.String("TenantID")
	}
	if props.EnableDetailedMetrics == nil {
		props.EnableDetailedMetrics = jsii.Bool(false)
	}

	// Set in-memory cache defaults
	if props.InMemoryConfig == nil {
		props.InMemoryConfig = &InMemoryCacheConfig{}
	}
	if props.InMemoryConfig.MaxSize == nil {
		props.InMemoryConfig.MaxSize = props.MaxCacheSize
	}
	if props.InMemoryConfig.TTL == nil {
		props.InMemoryConfig.TTL = props.DefaultTTL
	}
	if props.InMemoryConfig.EvictionPolicy == nil {
		props.InMemoryConfig.EvictionPolicy = jsii.String("LRU")
	}
	if props.InMemoryConfig.ConcurrencyLevel == nil {
		props.InMemoryConfig.ConcurrencyLevel = intPtr(16)
	}
	if props.InMemoryConfig.EnableStatistics == nil {
		props.InMemoryConfig.EnableStatistics = jsii.Bool(true)
	}

	return props
}

// createMemoryDBCluster creates AWS MemoryDB cluster for caching
func (d *DynamORMCache) createMemoryDBCluster() {
	if d.props.MemoryDBConfig == nil {
		d.props.MemoryDBConfig = &MemoryDBCacheConfig{
			NodeType:         jsii.String("db.t4g.small"),
			NumShards:        intPtr(1),
			ReplicasPerShard: intPtr(1),
		}
	}

	config := d.props.MemoryDBConfig
	clusterName := "dynamorm-cache"
	if config.ClusterName != nil {
		clusterName = *config.ClusterName
	}

	// Create MemoryDB cluster
	d.MemoryDBCluster = awsmemorydb.NewCfnCluster(d, jsii.String("MemoryDBCluster"), &awsmemorydb.CfnClusterProps{
		ClusterName:      jsii.String(clusterName),
		NodeType:         config.NodeType,
		NumShards:        float64Ptr(float64(*config.NumShards)),
		NumReplicasPerShard: float64Ptr(float64(*config.ReplicasPerShard)),
		AclName:          jsii.String("open-access"),
		Engine:           jsii.String("redis"),
		EngineVersion:    jsii.String("7.0"),
		Port:             jsii.Number(6379),
		TlsEnabled:       jsii.Bool(true),
	})

	// Set parameter group if specified
	if config.ParameterGroup != nil {
		d.MemoryDBCluster.SetParameterGroupName(config.ParameterGroup)
	}

	// Set security groups if specified
	if config.SecurityGroupIds != nil {
		d.MemoryDBCluster.SetSecurityGroupIds(config.SecurityGroupIds)
	}

	// Set subnet group if specified
	if config.SubnetGroupName != nil {
		d.MemoryDBCluster.SetSubnetGroupName(config.SubnetGroupName)
	}

	// Add tags using CDK tagging
	awscdk.Tags_Of(d).Add(jsii.String("Purpose"), jsii.String("DynamORMCache"), nil)
	awscdk.Tags_Of(d).Add(jsii.String("Table"), d.Table.GetTableName(), nil)
}

// createCacheAccessRole creates IAM role for cache access
func (d *DynamORMCache) createCacheAccessRole() {
	d.CacheAccessRole = awsiam.NewRole(d, jsii.String("CacheAccessRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("lambda.amazonaws.com"), nil),
		ManagedPolicies: &[]awsiam.IManagedPolicy{
			awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("service-role/AWSLambdaBasicExecutionRole")),
		},
	})

	// Add DynamORM table permissions
	d.Table.AddDynamORMPermissions(d.CacheAccessRole)

	// Add MemoryDB permissions if using MemoryDB
	if d.props.CacheStrategy == CacheStrategy_MEMORYDB && d.MemoryDBCluster != nil {
		d.CacheAccessRole.AddToPrincipalPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
			Effect: awsiam.Effect_ALLOW,
			Actions: &[]*string{
				jsii.String("memorydb:Connect"),
				jsii.String("memorydb:Describe*"),
				jsii.String("memorydb:List*"),
			},
			Resources: &[]*string{
				jsii.String(fmt.Sprintf("arn:aws:memorydb:%s:%s:cluster/%s", 
					*awscdk.Stack_Of(d).Region(),
					*awscdk.Stack_Of(d).Account(),
					*d.MemoryDBCluster.ClusterName())),
			},
		}))
	}

	// Add CloudWatch permissions for metrics
	d.CacheAccessRole.AddToPrincipalPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Effect: awsiam.Effect_ALLOW,
		Actions: &[]*string{
			jsii.String("cloudwatch:PutMetricData"),
			jsii.String("cloudwatch:GetMetricStatistics"),
		},
		Resources: &[]*string{
			jsii.String("*"),
		},
	}))
}

// createCacheMetrics creates CloudWatch metrics for cache monitoring
func (d *DynamORMCache) createCacheMetrics() {
	d.Metrics = make(map[string]awscloudwatch.Metric)
	tableName := *d.Table.GetTableName()

	// Cache hit ratio
	d.Metrics["CacheHitRatio"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("DynamORM/Cache"),
		MetricName: jsii.String("HitRatio"),
		DimensionsMap: &map[string]*string{
			"TableName": jsii.String(tableName),
			"CacheStrategy": jsii.String(string(d.props.CacheStrategy)),
		},
		Statistic: jsii.String(string(awscloudwatch.Statistic_AVERAGE)),
		Period:    awscdk.Duration_Minutes(jsii.Number(5)),
	})

	// Cache miss ratio
	d.Metrics["CacheMissRatio"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("DynamORM/Cache"),
		MetricName: jsii.String("MissRatio"),
		DimensionsMap: &map[string]*string{
			"TableName": jsii.String(tableName),
			"CacheStrategy": jsii.String(string(d.props.CacheStrategy)),
		},
		Statistic: jsii.String(string(awscloudwatch.Statistic_AVERAGE)),
		Period:    awscdk.Duration_Minutes(jsii.Number(5)),
	})

	// Cache eviction rate
	d.Metrics["EvictionRate"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("DynamORM/Cache"),
		MetricName: jsii.String("EvictionRate"),
		DimensionsMap: &map[string]*string{
			"TableName": jsii.String(tableName),
			"CacheStrategy": jsii.String(string(d.props.CacheStrategy)),
		},
		Statistic: jsii.String(string(awscloudwatch.Statistic_SUM)),
		Period:    awscdk.Duration_Minutes(jsii.Number(5)),
	})

	// Cache operations per second
	d.Metrics["OperationsPerSecond"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("DynamORM/Cache"),
		MetricName: jsii.String("OperationsPerSecond"),
		DimensionsMap: &map[string]*string{
			"TableName": jsii.String(tableName),
			"CacheStrategy": jsii.String(string(d.props.CacheStrategy)),
		},
		Statistic: jsii.String(string(awscloudwatch.Statistic_SUM)),
		Period:    awscdk.Duration_Minutes(jsii.Number(1)),
	})

	// Cache latency
	d.Metrics["CacheLatency"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("DynamORM/Cache"),
		MetricName: jsii.String("CacheLatency"),
		DimensionsMap: &map[string]*string{
			"TableName": jsii.String(tableName),
			"CacheStrategy": jsii.String(string(d.props.CacheStrategy)),
		},
		Statistic: jsii.String(string(awscloudwatch.Statistic_AVERAGE)),
		Period:    awscdk.Duration_Minutes(jsii.Number(5)),
	})

	// Memory usage (for in-memory caches)
	if d.props.CacheStrategy == CacheStrategy_IN_MEMORY || d.props.CacheStrategy == CacheStrategy_HYBRID {
		d.Metrics["MemoryUsage"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("DynamORM/Cache"),
			MetricName: jsii.String("MemoryUsage"),
			DimensionsMap: &map[string]*string{
				"TableName": jsii.String(tableName),
				"CacheStrategy": jsii.String(string(d.props.CacheStrategy)),
			},
			Statistic: jsii.String(string(awscloudwatch.Statistic_AVERAGE)),
			Period:    awscdk.Duration_Minutes(jsii.Number(5)),
		})
	}

	// Tenant-specific metrics if multi-tenant is enabled
	if d.props.EnableTenantIsolation != nil && *d.props.EnableTenantIsolation {
		d.Metrics["TenantCacheHitRatio"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("DynamORM/Cache/Tenant"),
			MetricName: jsii.String("HitRatio"),
			DimensionsMap: &map[string]*string{
				"TableName": jsii.String(tableName),
				"TenantAttribute": d.props.TenantAttribute,
			},
			Statistic: jsii.String(string(awscloudwatch.Statistic_AVERAGE)),
			Period:    awscdk.Duration_Minutes(jsii.Number(5)),
		})
	}
}

// createDetailedMonitoring creates detailed monitoring with alarms
func (d *DynamORMCache) createDetailedMonitoring() {
	if d.Metrics == nil {
		d.createCacheMetrics()
	}

	tableName := *d.Table.GetTableName()
	thresholds := d.props.AlertThresholds

	// Set default thresholds if not provided
	if thresholds == nil {
		thresholds = &CacheAlertThresholds{
			LowCacheHitRatio: jsii.Number(0.8),   // Alert if hit ratio below 80%
			HighEvictionRate: jsii.Number(100),   // Alert if more than 100 evictions/min
			HighMemoryUsage:  jsii.Number(0.9),   // Alert if memory usage above 90%
			HighLatency:      jsii.Number(50),    // Alert if latency above 50ms
		}
	}

	// Low cache hit ratio alarm
	if thresholds.LowCacheHitRatio != nil {
		awscloudwatch.NewAlarm(d, jsii.String("LowCacheHitRatioAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:         jsii.String(fmt.Sprintf("%s-cache-low-hit-ratio", tableName)),
			AlarmDescription:  jsii.String("Cache hit ratio is below threshold"),
			Metric:           d.Metrics["CacheHitRatio"],
			Threshold:         thresholds.LowCacheHitRatio,
			EvaluationPeriods: jsii.Number(3),
			ComparisonOperator: awscloudwatch.ComparisonOperator_LESS_THAN_THRESHOLD,
			TreatMissingData:  awscloudwatch.TreatMissingData_NOT_BREACHING,
		})
	}

	// High eviction rate alarm
	if thresholds.HighEvictionRate != nil {
		awscloudwatch.NewAlarm(d, jsii.String("HighEvictionRateAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:         jsii.String(fmt.Sprintf("%s-cache-high-eviction", tableName)),
			AlarmDescription:  jsii.String("Cache eviction rate is too high"),
			Metric:           d.Metrics["EvictionRate"],
			Threshold:         thresholds.HighEvictionRate,
			EvaluationPeriods: jsii.Number(2),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
			TreatMissingData:  awscloudwatch.TreatMissingData_NOT_BREACHING,
		})
	}

	// High latency alarm
	if thresholds.HighLatency != nil && d.Metrics["CacheLatency"] != nil {
		awscloudwatch.NewAlarm(d, jsii.String("HighCacheLatencyAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:         jsii.String(fmt.Sprintf("%s-cache-high-latency", tableName)),
			AlarmDescription:  jsii.String("Cache latency is too high"),
			Metric:           d.Metrics["CacheLatency"],
			Threshold:         thresholds.HighLatency,
			EvaluationPeriods: jsii.Number(2),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
			TreatMissingData:  awscloudwatch.TreatMissingData_NOT_BREACHING,
		})
	}

	// High memory usage alarm (for in-memory caches)
	if thresholds.HighMemoryUsage != nil && d.Metrics["MemoryUsage"] != nil {
		awscloudwatch.NewAlarm(d, jsii.String("HighMemoryUsageAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:         jsii.String(fmt.Sprintf("%s-cache-high-memory", tableName)),
			AlarmDescription:  jsii.String("Cache memory usage is too high"),
			Metric:           d.Metrics["MemoryUsage"],
			Threshold:         thresholds.HighMemoryUsage,
			EvaluationPeriods: jsii.Number(2),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
			TreatMissingData:  awscloudwatch.TreatMissingData_NOT_BREACHING,
		})
	}
}

// GetEnvironmentVariables returns environment variables for Lambda functions
func (d *DynamORMCache) GetEnvironmentVariables() *map[string]*string {
	env := make(map[string]*string)

	// Basic cache configuration
	env["DYNAMORM_CACHE_ENABLED"] = jsii.String("true")
	env["DYNAMORM_CACHE_STRATEGY"] = jsii.String(string(d.props.CacheStrategy))
	env["DYNAMORM_CACHE_INVALIDATION"] = jsii.String(string(d.props.InvalidationStrategy))
	if d.props.DefaultTTL != nil {
		seconds := d.props.DefaultTTL.ToSeconds(nil)
		if seconds != nil {
			env["DYNAMORM_CACHE_TTL"] = jsii.String(fmt.Sprintf("%d", int(*seconds)))
		}
	}

	// In-memory cache configuration
	if d.props.CacheStrategy == CacheStrategy_IN_MEMORY || d.props.CacheStrategy == CacheStrategy_HYBRID {
		config := d.props.InMemoryConfig
		env["DYNAMORM_CACHE_MAX_SIZE"] = jsii.String(fmt.Sprintf("%d", *config.MaxSize))
		env["DYNAMORM_CACHE_EVICTION_POLICY"] = config.EvictionPolicy
		env["DYNAMORM_CACHE_CONCURRENCY"] = jsii.String(fmt.Sprintf("%d", *config.ConcurrencyLevel))
		
		if config.EnableStatistics != nil && *config.EnableStatistics {
			env["DYNAMORM_CACHE_STATISTICS"] = jsii.String("true")
		}
	}

	// MemoryDB configuration
	if d.props.CacheStrategy == CacheStrategy_MEMORYDB && d.MemoryDBCluster != nil {
		env["DYNAMORM_MEMORYDB_CLUSTER"] = d.MemoryDBCluster.ClusterName()
		env["DYNAMORM_MEMORYDB_ENDPOINT"] = d.MemoryDBCluster.AttrClusterEndpointAddress()
		env["DYNAMORM_MEMORYDB_PORT"] = jsii.String("6379")
	}

	// Redis configuration
	if d.props.CacheStrategy == CacheStrategy_REDIS && d.props.RedisConfig != nil {
		config := d.props.RedisConfig
		if config.Host != nil {
			env["DYNAMORM_REDIS_HOST"] = config.Host
		}
		if config.Port != nil {
			env["DYNAMORM_REDIS_PORT"] = jsii.String(fmt.Sprintf("%d", *config.Port))
		}
		if config.Database != nil {
			env["DYNAMORM_REDIS_DB"] = jsii.String(fmt.Sprintf("%d", *config.Database))
		}
		if config.EnableTLS != nil && *config.EnableTLS {
			env["DYNAMORM_REDIS_TLS"] = jsii.String("true")
		}
	}

	// Multi-tenant configuration
	if d.props.EnableTenantIsolation != nil && *d.props.EnableTenantIsolation {
		env["DYNAMORM_CACHE_TENANT_ISOLATION"] = jsii.String("true")
		env["DYNAMORM_CACHE_TENANT_ATTRIBUTE"] = d.props.TenantAttribute
	}

	// Compression configuration
	if d.props.EnableCompression != nil && *d.props.EnableCompression {
		env["DYNAMORM_CACHE_COMPRESSION"] = jsii.String("true")
	}

	// Metrics configuration
	if d.props.EnableMetrics != nil && *d.props.EnableMetrics {
		env["DYNAMORM_CACHE_METRICS"] = jsii.String("true")
	}

	// Prefetch patterns
	if len(d.props.PrefetchPatterns) > 0 {
		patterns := ""
		for i, pattern := range d.props.PrefetchPatterns {
			if i > 0 {
				patterns += ","
			}
			patterns += pattern
		}
		env["DYNAMORM_CACHE_PREFETCH_PATTERNS"] = jsii.String(patterns)
	}

	// Warmup queries
	if len(d.props.WarmupQueries) > 0 {
		queries := ""
		for i, query := range d.props.WarmupQueries {
			if i > 0 {
				queries += ","
			}
			queries += query
		}
		env["DYNAMORM_CACHE_WARMUP_QUERIES"] = jsii.String(queries)
	}

	return &env
}

// ConfigureCacheInvalidation configures cache invalidation based on DynamoDB streams
func (d *DynamORMCache) ConfigureCacheInvalidation(streamProcessor *DynamORMStreamProcessor) {
	if d.props.InvalidationStrategy != CacheInvalidationStrategy_STREAM_BASED {
		return
	}

	// Add cache invalidation environment variables to the stream processor
	streamProcessor.AddEnvironmentVariable("DYNAMORM_CACHE_INVALIDATION_ENABLED", "true")
	streamProcessor.AddEnvironmentVariable("DYNAMORM_CACHE_STRATEGY", string(d.props.CacheStrategy))

	// Add MemoryDB access if using MemoryDB
	if d.props.CacheStrategy == CacheStrategy_MEMORYDB && d.MemoryDBCluster != nil {
		streamProcessor.AddEnvironmentVariable("DYNAMORM_MEMORYDB_CLUSTER", *d.MemoryDBCluster.ClusterName())
		streamProcessor.AddEnvironmentVariable("DYNAMORM_MEMORYDB_ENDPOINT", *d.MemoryDBCluster.AttrClusterEndpointAddress())
	}
}

// GrantCacheAccess grants cache access permissions to a Lambda function
func (d *DynamORMCache) GrantCacheAccess(grantee awslambda.IFunction) {
	// Grant DynamORM table access
	d.Table.AddDynamORMPermissions(grantee)

	// Grant MemoryDB access if using MemoryDB
	if d.props.CacheStrategy == CacheStrategy_MEMORYDB && d.MemoryDBCluster != nil {
		grantee.GrantPrincipal().AddToPrincipalPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
			Effect: awsiam.Effect_ALLOW,
			Actions: &[]*string{
				jsii.String("memorydb:Connect"),
				jsii.String("memorydb:Describe*"),
			},
			Resources: &[]*string{
				jsii.String(fmt.Sprintf("arn:aws:memorydb:%s:%s:cluster/%s", 
					*awscdk.Stack_Of(d).Region(),
					*awscdk.Stack_Of(d).Account(),
					*d.MemoryDBCluster.ClusterName())),
			},
		}))
	}

	// Grant CloudWatch permissions for metrics
	grantee.GrantPrincipal().AddToPrincipalPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Effect: awsiam.Effect_ALLOW,
		Actions: &[]*string{
			jsii.String("cloudwatch:PutMetricData"),
		},
		Resources: &[]*string{
			jsii.String("*"),
		},
		Conditions: &map[string]interface{}{
			"StringEquals": map[string]interface{}{
				"cloudwatch:namespace": []string{"DynamORM/Cache", "DynamORM/Cache/Tenant"},
			},
		},
	}))
}

// GetCacheMetrics returns cache CloudWatch metrics
func (d *DynamORMCache) GetCacheMetrics() map[string]awscloudwatch.Metric {
	return d.Metrics
}

// GetMemoryDBCluster returns the MemoryDB cluster if using MemoryDB strategy
func (d *DynamORMCache) GetMemoryDBCluster() awsmemorydb.CfnCluster {
	return d.MemoryDBCluster
}

// GetCacheAccessRole returns the IAM role for cache access
func (d *DynamORMCache) GetCacheAccessRole() awsiam.Role {
	return d.CacheAccessRole
}

// GetTable returns the DynamORM table being cached
func (d *DynamORMCache) GetTable() *DynamORMTable {
	return d.Table
}