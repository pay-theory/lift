package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/jsii-runtime-go"
)

func TestNewDynamORMCache(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create a DynamORM table
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("sk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		TableName: jsii.String("test-table"),
	})

	// Create cache with default settings
	cache := NewDynamORMCache(stack, jsii.String("TestCache"), &DynamORMCacheProps{
		DynamORMTable: table,
	})

	// Test that the cache was created successfully
	if cache == nil {
		t.Fatal("DynamORMCache should not be nil")
	}

	// Test that the table reference is correct
	if cache.Table != table {
		t.Fatal("Table reference should match the provided table")
	}

	// Test that default strategy is IN_MEMORY
	if cache.props.CacheStrategy != CacheStrategy_IN_MEMORY {
		t.Fatal("Default cache strategy should be IN_MEMORY")
	}

	// Test that metrics were created
	if cache.Metrics == nil {
		t.Fatal("Metrics should be created by default")
	}

	// Synth the stack to validate
	app.Synth(nil)
}

func TestDynamORMCacheMemoryDB(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create a DynamORM table
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("sk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		TableName: jsii.String("test-table"),
	})

	// Create cache with MemoryDB strategy
	cache := NewDynamORMCache(stack, jsii.String("TestCache"), &DynamORMCacheProps{
		DynamORMTable:    table,
		CacheStrategy:    CacheStrategy_MEMORYDB,
		EnableMetrics:    jsii.Bool(true),
		DefaultTTL:       awscdk.Duration_Minutes(jsii.Number(30)),
		MemoryDBConfig: &MemoryDBCacheConfig{
			ClusterName:      jsii.String("test-cache-cluster"),
			NodeType:         jsii.String("db.t4g.small"),
			NumShards:        intPtr(2),
			ReplicasPerShard: intPtr(1),
		},
	})

	// Test that MemoryDB cluster was created
	if cache.MemoryDBCluster == nil {
		t.Fatal("MemoryDB cluster should be created")
	}

	// Test that the cache strategy is correct
	if cache.props.CacheStrategy != CacheStrategy_MEMORYDB {
		t.Fatal("Cache strategy should be MEMORYDB")
	}

	// Synth the stack to validate
	app.Synth(nil)
}

func TestDynamORMCacheMultiTenant(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create a multi-tenant DynamORM table
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("sk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		TableName:         jsii.String("test-multi-tenant-table"),
		EnableMultiTenant: jsii.Bool(true),
		TenantAttribute:   jsii.String("tenant_id"),
	})

	// Create cache with multi-tenant support
	cache := NewDynamORMCache(stack, jsii.String("TestCache"), &DynamORMCacheProps{
		DynamORMTable:         table,
		CacheStrategy:         CacheStrategy_HYBRID,
		EnableTenantIsolation: jsii.Bool(true),
		TenantAttribute:       jsii.String("tenant_id"),
		InvalidationStrategy:  CacheInvalidationStrategy_STREAM_BASED,
		EnableDetailedMetrics: jsii.Bool(true),
		AlertThresholds: &CacheAlertThresholds{
			LowCacheHitRatio: jsii.Number(0.75),
			HighEvictionRate: jsii.Number(200),
			HighMemoryUsage:  jsii.Number(0.85),
		},
	})

	// Test that tenant isolation is enabled
	if cache.props.EnableTenantIsolation == nil || !*cache.props.EnableTenantIsolation {
		t.Fatal("Tenant isolation should be enabled")
	}

	// Test that tenant metrics exist
	if cache.Metrics["TenantCacheHitRatio"] == nil {
		t.Fatal("Tenant cache hit ratio metric should exist")
	}

	// Synth the stack to validate
	app.Synth(nil)
}

func TestDynamORMCacheRedis(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create a DynamORM table
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("sk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		TableName: jsii.String("test-table"),
	})

	// Create cache with Redis strategy
	cache := NewDynamORMCache(stack, jsii.String("TestCache"), &DynamORMCacheProps{
		DynamORMTable:      table,
		CacheStrategy:      CacheStrategy_REDIS,
		EnableCompression:  jsii.Bool(true),
		PrefetchPatterns:   []string{"user:*", "tenant:*"},
		WarmupQueries:      []string{"GetUser", "ListTenants"},
		RedisConfig: &RedisCacheConfig{
			Host:              jsii.String("redis.example.com"),
			Port:              intPtr(6379),
			Database:          intPtr(0),
			MaxConnections:    intPtr(100),
			ConnectionTimeout: awscdk.Duration_Seconds(jsii.Number(5)),
			EnableTLS:         jsii.Bool(true),
		},
	})

	// Test that the cache strategy is correct
	if cache.props.CacheStrategy != CacheStrategy_REDIS {
		t.Fatal("Cache strategy should be REDIS")
	}

	// Test that compression is enabled
	if cache.props.EnableCompression == nil || !*cache.props.EnableCompression {
		t.Fatal("Compression should be enabled")
	}

	// Synth the stack to validate
	app.Synth(nil)
}

func TestDynamORMCacheEnvironmentVariables(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create a DynamORM table
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("sk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		TableName: jsii.String("test-table"),
	})

	// Create cache
	cache := NewDynamORMCache(stack, jsii.String("TestCache"), &DynamORMCacheProps{
		DynamORMTable:         table,
		CacheStrategy:         CacheStrategy_IN_MEMORY,
		EnableTenantIsolation: jsii.Bool(true),
		EnableCompression:     jsii.Bool(true),
		PrefetchPatterns:      []string{"user:*", "order:*"},
		WarmupQueries:         []string{"GetUserProfile", "ListActiveOrders"},
	})

	// Get environment variables
	env := cache.GetEnvironmentVariables()

	// Test required environment variables
	if (*env)["DYNAMORM_CACHE_ENABLED"] == nil || *(*env)["DYNAMORM_CACHE_ENABLED"] != "true" {
		t.Fatal("DYNAMORM_CACHE_ENABLED should be true")
	}

	if (*env)["DYNAMORM_CACHE_STRATEGY"] == nil || *(*env)["DYNAMORM_CACHE_STRATEGY"] != string(CacheStrategy_IN_MEMORY) {
		t.Fatal("DYNAMORM_CACHE_STRATEGY should be IN_MEMORY")
	}

	if (*env)["DYNAMORM_CACHE_TENANT_ISOLATION"] == nil || *(*env)["DYNAMORM_CACHE_TENANT_ISOLATION"] != "true" {
		t.Fatal("DYNAMORM_CACHE_TENANT_ISOLATION should be true")
	}

	if (*env)["DYNAMORM_CACHE_COMPRESSION"] == nil || *(*env)["DYNAMORM_CACHE_COMPRESSION"] != "true" {
		t.Fatal("DYNAMORM_CACHE_COMPRESSION should be true")
	}

	if (*env)["DYNAMORM_CACHE_PREFETCH_PATTERNS"] == nil || *(*env)["DYNAMORM_CACHE_PREFETCH_PATTERNS"] != "user:*,order:*" {
		t.Fatal("DYNAMORM_CACHE_PREFETCH_PATTERNS should contain patterns")
	}

	if (*env)["DYNAMORM_CACHE_WARMUP_QUERIES"] == nil || *(*env)["DYNAMORM_CACHE_WARMUP_QUERIES"] != "GetUserProfile,ListActiveOrders" {
		t.Fatal("DYNAMORM_CACHE_WARMUP_QUERIES should contain queries")
	}

	// Synth the stack to validate
	app.Synth(nil)
}

func TestDynamORMCachePermissions(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create a DynamORM table
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("sk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		TableName: jsii.String("test-table"),
	})

	// Create cache
	cache := NewDynamORMCache(stack, jsii.String("TestCache"), &DynamORMCacheProps{
		DynamORMTable: table,
		CacheStrategy: CacheStrategy_IN_MEMORY,
	})

	// Create a test Lambda function
	testFunction := awslambda.NewFunction(stack, jsii.String("TestFunction"), &awslambda.FunctionProps{
		FunctionName: jsii.String("test-function"),
		Code:         awslambda.Code_FromInline(jsii.String("exports.handler = async () => {};")),
		Handler:      jsii.String("index.handler"),
		Runtime:      awslambda.Runtime_NODEJS_18_X(),
	})

	// Grant cache access
	cache.GrantCacheAccess(testFunction)

	// Test that cache access role was created
	if cache.CacheAccessRole == nil {
		t.Fatal("Cache access role should be created")
	}

	// Synth the stack to validate
	app.Synth(nil)
}

func TestDynamORMCacheStreamInvalidation(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create a DynamORM table
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("sk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		TableName: jsii.String("test-table"),
		Stream:   awsdynamodb.StreamViewType_NEW_AND_OLD_IMAGES,
	})

	// Create cache with stream-based invalidation
	cache := NewDynamORMCache(stack, jsii.String("TestCache"), &DynamORMCacheProps{
		DynamORMTable:        table,
		CacheStrategy:        CacheStrategy_IN_MEMORY,
		InvalidationStrategy: CacheInvalidationStrategy_STREAM_BASED,
	})

	// Create stream processor
	streamProcessor := NewDynamORMStreamProcessor(stack, jsii.String("StreamProcessor"), &DynamORMStreamProcessorProps{
		DynamORMTable: table,
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String("test-stream-processor"),
			Code:         awslambda.Code_FromInline(jsii.String("exports.handler = async (event) => console.log(event);")),
			Handler:      jsii.String("index.handler"),
			Runtime:      awslambda.Runtime_NODEJS_18_X(),
		},
	})

	// Configure cache invalidation
	cache.ConfigureCacheInvalidation(streamProcessor)

	// Test that invalidation strategy is correct
	if cache.props.InvalidationStrategy != CacheInvalidationStrategy_STREAM_BASED {
		t.Fatal("Invalidation strategy should be STREAM_BASED")
	}

	// Synth the stack to validate
	app.Synth(nil)
}

func TestDynamORMCacheNilProps(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Test that creating with nil props panics
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Expected panic when props is nil")
		}
	}()

	NewDynamORMCache(stack, jsii.String("TestCache"), nil)
}

func TestDynamORMCacheNilTable(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Test that creating with nil table panics
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Expected panic when DynamORMTable is nil")
		}
	}()

	NewDynamORMCache(stack, jsii.String("TestCache"), &DynamORMCacheProps{
		DynamORMTable: nil,
	})
}

func TestDynamORMCacheGetters(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create a DynamORM table
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("sk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		TableName: jsii.String("test-table"),
	})

	// Create cache
	cache := NewDynamORMCache(stack, jsii.String("TestCache"), &DynamORMCacheProps{
		DynamORMTable: table,
		CacheStrategy: CacheStrategy_MEMORYDB,
	})

	// Test getters
	if cache.GetTable() != table {
		t.Fatal("GetTable should return the correct table")
	}

	if cache.GetCacheMetrics() == nil {
		t.Fatal("GetCacheMetrics should not return nil")
	}

	if cache.GetCacheAccessRole() == nil {
		t.Fatal("GetCacheAccessRole should not return nil")
	}

	if cache.GetMemoryDBCluster() == nil {
		t.Fatal("GetMemoryDBCluster should not return nil when using MemoryDB strategy")
	}

	// Synth the stack to validate
	app.Synth(nil)
}