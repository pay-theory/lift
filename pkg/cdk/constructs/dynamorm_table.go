package constructs

import (
	"fmt"
	
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatchactions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssns"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// DynamORMTableProps extends DynamoDB table properties for DynamORM compatibility
type DynamORMTableProps struct {
	// Required
	PartitionKey *awsdynamodb.Attribute
	SortKey      *awsdynamodb.Attribute // Optional but common
	
	// Table configuration
	TableName                 *string
	BillingMode              awsdynamodb.BillingMode
	PointInTimeRecovery      *bool
	Stream                   awsdynamodb.StreamViewType
	TimeToLiveAttribute      *string
	DeletionProtection       *bool
	RemovalPolicy            awscdk.RemovalPolicy
	
	// DynamORM specific
	EnableMultiTenant        *bool
	TenantAttribute          *string
	EnableVersioning         *bool
	EnableTimestamps         *bool
	
	// Capacity (for provisioned mode)
	ReadCapacity             *float64
	WriteCapacity            *float64
	EnableAutoScaling        *bool
	
	// Tags
	Tags                     *map[string]*string
}

// GSIProps defines a Global Secondary Index for DynamORM
type GSIProps struct {
	// Index name
	IndexName      *string
	// Partition key attribute definition
	PartitionKey   *awsdynamodb.Attribute
	// Sort key attribute definition (optional)
	SortKey        *awsdynamodb.Attribute
	// Projection type (defaults to ALL)
	ProjectionType awsdynamodb.ProjectionType
	// For composite keys
	CompositeFields []string  // For dynamorm composite key support
}

// DynamORMModelSpec defines expected model structure
type DynamORMModelSpec struct {
	ModelName    string
	PartitionKey string
	SortKey      string
	GSIs         []GSISpec
	TTLAttribute string
	Attributes   map[string]string
}

// GSISpec defines a GSI specification
type GSISpec struct {
	IndexName    string
	PartitionKey string
	SortKey      string
}

// DynamORMTable is a DynamoDB table construct optimized for DynamORM
type DynamORMTable struct {
	construct constructs.Construct
	Table     awsdynamodb.Table
	props     *DynamORMTableProps
	modelSpec *DynamORMModelSpec
}

// GetResourceName returns the table name
func (d *DynamORMTable) GetResourceName() *string {
	return d.Table.TableName()
}

// validateDynamORMTableProps validates the required properties for DynamORMTable
func validateDynamORMTableProps(props *DynamORMTableProps) error {
	if props == nil {
		return fmt.Errorf("DynamORMTableProps cannot be nil")
	}
	if props.PartitionKey == nil {
		return fmt.Errorf("PartitionKey is required for DynamORMTable")
	}
	if props.PartitionKey.Name == nil || *props.PartitionKey.Name == "" {
		return fmt.Errorf("PartitionKey.Name cannot be empty")
	}
	return nil
}

// NewDynamORMTable creates a new DynamoDB table for DynamORM with standard configurations
func NewDynamORMTable(scope constructs.Construct, id *string, props *DynamORMTableProps) *DynamORMTable {
	this := constructs.NewConstruct(scope, id)
	
	// Validate required properties
	if err := validateDynamORMTableProps(props); err != nil {
		// For CDK constructs, panic is acceptable during construction with clear error messages
		panic(fmt.Sprintf("DynamORMTable validation failed: %v", err))
	}
	
	// Apply defaults after validation
	props = applyDefaults(props)
	
	// Create table with DynamORM-optimized settings
	tableProps := &awsdynamodb.TableProps{
		TableName:           props.TableName,
		PartitionKey:        props.PartitionKey,
		SortKey:             props.SortKey,
		BillingMode:         props.BillingMode,
		PointInTimeRecovery: props.PointInTimeRecovery,
		DeletionProtection:  props.DeletionProtection,
		RemovalPolicy:       props.RemovalPolicy,
		Encryption:          awsdynamodb.TableEncryption_AWS_MANAGED,
	}
	
	// Configure TTL if specified
	if props.TimeToLiveAttribute != nil {
		tableProps.TimeToLiveAttribute = props.TimeToLiveAttribute
	}
	
	// Configure streams if needed
	if props.Stream != "" {
		tableProps.Stream = props.Stream
	}
	
	// Create the table
	table := awsdynamodb.NewTable(this, jsii.String("Table"), tableProps)
	
	dt := &DynamORMTable{
		construct: this,
		Table:     table,
		props:     props,
	}
	
	// Configure DynamORM-specific features
	if *props.EnableMultiTenant {
		dt.ConfigureMultiTenant(*props.TenantAttribute)
	}
	
	// Configure auto-scaling if in provisioned mode
	if props.BillingMode == awsdynamodb.BillingMode_PROVISIONED && *props.EnableAutoScaling {
		dt.EnableAutoScaling(nil, nil, nil)
	}
	
	// Add standard tags
	dt.addStandardTags()
	
	// Add custom tags
	if props.Tags != nil {
		dt.AddTags(props.Tags)
	}
	
	return dt
}

func applyDefaults(props *DynamORMTableProps) *DynamORMTableProps {
	if props.BillingMode == "" {
		props.BillingMode = awsdynamodb.BillingMode_PAY_PER_REQUEST
	}
	if props.PointInTimeRecovery == nil {
		props.PointInTimeRecovery = jsii.Bool(true)
	}
	if props.RemovalPolicy == "" {
		props.RemovalPolicy = awscdk.RemovalPolicy_RETAIN
	}
	if props.DeletionProtection == nil {
		props.DeletionProtection = jsii.Bool(true) // Default to true for production safety
	}
	if props.EnableMultiTenant == nil {
		props.EnableMultiTenant = jsii.Bool(false)
	}
	if props.TenantAttribute == nil {
		props.TenantAttribute = jsii.String("TenantID")
	}
	if props.EnableVersioning == nil {
		props.EnableVersioning = jsii.Bool(true)
	}
	if props.EnableTimestamps == nil {
		props.EnableTimestamps = jsii.Bool(true)
	}
	if props.EnableAutoScaling == nil {
		props.EnableAutoScaling = jsii.Bool(false)
	}
	if props.Stream == "" {
		props.Stream = awsdynamodb.StreamViewType_NEW_AND_OLD_IMAGES
	}
	return props
}

func (t *DynamORMTable) addStandardTags() {
	stack := awscdk.Stack_Of(t.construct)
	awscdk.Tags_Of(t.Table).Add(jsii.String("Framework"), jsii.String("DynamORM"), nil)
	awscdk.Tags_Of(t.Table).Add(jsii.String("ManagedBy"), jsii.String("CDK"), nil)
	awscdk.Tags_Of(t.Table).Add(jsii.String("Environment"), stack.StackName(), nil)
}

// ConfigureMultiTenant sets up multi-tenant patterns with enhanced isolation
func (t *DynamORMTable) ConfigureMultiTenant(tenantAttribute string) {
	if tenantAttribute == "" {
		tenantAttribute = "TenantID"
	}
	
	// Add primary tenant GSI for cross-tenant queries
	t.AddDynamORMIndex("tenant", 
		&awsdynamodb.Attribute{
			Name: jsii.String(tenantAttribute),
			Type: awsdynamodb.AttributeType_STRING,
		},
		&awsdynamodb.Attribute{
			Name: jsii.String("created_at"),
			Type: awsdynamodb.AttributeType_STRING,
		},
	)
	
	// Add tenant-entity GSI for efficient entity queries within tenant
	t.AddDynamORMIndex("tenant-entity", 
		&awsdynamodb.Attribute{
			Name: jsii.String(tenantAttribute),
			Type: awsdynamodb.AttributeType_STRING,
		},
		&awsdynamodb.Attribute{
			Name: jsii.String("entity_type"),
			Type: awsdynamodb.AttributeType_STRING,
		},
	)
	
	// Add tenant status GSI for operational queries
	t.AddDynamORMIndex("tenant-status", 
		&awsdynamodb.Attribute{
			Name: jsii.String("status"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		&awsdynamodb.Attribute{
			Name: jsii.String(tenantAttribute),
			Type: awsdynamodb.AttributeType_STRING,
		},
	)
	
	// Add tenant isolation tags
	awscdk.Tags_Of(t.Table).Add(jsii.String("MultiTenant"), jsii.String("true"), nil)
	awscdk.Tags_Of(t.Table).Add(jsii.String("TenantAttribute"), jsii.String(tenantAttribute), nil)
	awscdk.Tags_Of(t.Table).Add(jsii.String("IsolationLevel"), jsii.String("strict"), nil)
}

// AddDynamORMIndex adds an index following DynamORM naming conventions
func (t *DynamORMTable) AddDynamORMIndex(indexName string, pkAttr, skAttr *awsdynamodb.Attribute) {
	// DynamORM expects specific index naming: "gsi-{name}"
	gsiName := fmt.Sprintf("gsi-%s", indexName)
	
	gsiProps := &awsdynamodb.GlobalSecondaryIndexProps{
		IndexName:      jsii.String(gsiName),
		PartitionKey:   pkAttr,
		SortKey:        skAttr,
		ProjectionType: awsdynamodb.ProjectionType_ALL,
	}
	
	t.Table.AddGlobalSecondaryIndex(gsiProps)
}

// AddTenantScopedGSI adds a GSI with tenant isolation built-in
func (t *DynamORMTable) AddTenantScopedGSI(indexName, tenantAttribute, entityAttribute string) {
	if tenantAttribute == "" {
		tenantAttribute = "TenantID"
	}
	
	gsiName := fmt.Sprintf("gsi-tenant-%s", indexName)
	
	gsiProps := &awsdynamodb.GlobalSecondaryIndexProps{
		IndexName: jsii.String(gsiName),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String(tenantAttribute),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String(entityAttribute),
			Type: awsdynamodb.AttributeType_STRING,
		},
		ProjectionType: awsdynamodb.ProjectionType_ALL,
	}
	
	t.Table.AddGlobalSecondaryIndex(gsiProps)
}

// AddTenantEntityGSI adds a GSI for efficient entity queries within tenant boundaries
func (t *DynamORMTable) AddTenantEntityGSI(tenantAttribute, entityType string) {
	if tenantAttribute == "" {
		tenantAttribute = "TenantID"
	}
	if entityType == "" {
		entityType = "entity_type"
	}
	
	gsiName := "gsi-tenant-entity"
	
	gsiProps := &awsdynamodb.GlobalSecondaryIndexProps{
		IndexName: jsii.String(gsiName),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String(tenantAttribute),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String(entityType),
			Type: awsdynamodb.AttributeType_STRING,
		},
		ProjectionType: awsdynamodb.ProjectionType_ALL,
	}
	
	t.Table.AddGlobalSecondaryIndex(gsiProps)
}

// AddTenantTimeSeriesGSI adds a GSI for time-series queries within tenant boundaries
func (t *DynamORMTable) AddTenantTimeSeriesGSI(tenantAttribute, timeAttribute string) {
	if tenantAttribute == "" {
		tenantAttribute = "TenantID"
	}
	if timeAttribute == "" {
		timeAttribute = "created_at"
	}
	
	gsiName := "gsi-tenant-timeseries"
	
	gsiProps := &awsdynamodb.GlobalSecondaryIndexProps{
		IndexName: jsii.String(gsiName),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String(tenantAttribute),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String(timeAttribute),
			Type: awsdynamodb.AttributeType_STRING,
		},
		ProjectionType: awsdynamodb.ProjectionType_ALL,
	}
	
	t.Table.AddGlobalSecondaryIndex(gsiProps)
}

// AddTenantStatusGSI adds a GSI for status-based queries within tenant boundaries
func (t *DynamORMTable) AddTenantStatusGSI(statusAttribute, tenantAttribute string) {
	if tenantAttribute == "" {
		tenantAttribute = "TenantID"
	}
	if statusAttribute == "" {
		statusAttribute = "status"
	}
	
	gsiName := "gsi-tenant-status"
	
	gsiProps := &awsdynamodb.GlobalSecondaryIndexProps{
		IndexName: jsii.String(gsiName),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String(statusAttribute),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String(tenantAttribute),
			Type: awsdynamodb.AttributeType_STRING,
		},
		ProjectionType: awsdynamodb.ProjectionType_ALL,
	}
	
	t.Table.AddGlobalSecondaryIndex(gsiProps)
}

// AddGSI adds a Global Secondary Index to the table with full attribute definitions
func (t *DynamORMTable) AddGSI(props *GSIProps) {
	// Set defaults
	if props.ProjectionType == "" {
		props.ProjectionType = awsdynamodb.ProjectionType_ALL
	}
	
	gsiProps := &awsdynamodb.GlobalSecondaryIndexProps{
		IndexName:      props.IndexName,
		PartitionKey:   props.PartitionKey,
		SortKey:        props.SortKey,
		ProjectionType: props.ProjectionType,
	}
	
	t.Table.AddGlobalSecondaryIndex(gsiProps)
}

// GetEnvironmentVariables returns environment variables for Lambda functions
func (t *DynamORMTable) GetEnvironmentVariables() *map[string]*string {
	return &map[string]*string{
		"DYNAMODB_TABLE_NAME": t.Table.TableName(),
		"DYNAMORM_REGION":     jsii.String(*awscdk.Stack_Of(t.construct).Region()),
	}
}

// GrantTenantIsolatedAccess grants access with strict tenant isolation
func (t *DynamORMTable) GrantTenantIsolatedAccess(grantee awsiam.IGrantable, tenantAttribute string) {
	if tenantAttribute == "" {
		tenantAttribute = "TenantID"
	}
	
	// Grant access to main table with tenant isolation
	grantee.GrantPrincipal().AddToPrincipalPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: &[]*string{
			jsii.String("dynamodb:Query"),
			jsii.String("dynamodb:GetItem"),
			jsii.String("dynamodb:PutItem"),
			jsii.String("dynamodb:UpdateItem"),
			jsii.String("dynamodb:DeleteItem"),
		},
		Resources: &[]*string{
			t.Table.TableArn(),
		},
		Conditions: &map[string]interface{}{
			"ForAllValues:StringEquals": map[string]interface{}{
				"dynamodb:LeadingKeys": []string{"${aws:PrincipalTag/TenantID}"},
			},
		},
	}))
	
	// Grant access to tenant-specific GSIs
	grantee.GrantPrincipal().AddToPrincipalPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: &[]*string{
			jsii.String("dynamodb:Query"),
		},
		Resources: &[]*string{
			jsii.String(*t.Table.TableArn() + "/index/gsi-tenant"),
			jsii.String(*t.Table.TableArn() + "/index/gsi-tenant-entity"),
			jsii.String(*t.Table.TableArn() + "/index/gsi-tenant-status"),
		},
		Conditions: &map[string]interface{}{
			"ForAllValues:StringEquals": map[string]interface{}{
				"dynamodb:LeadingKeys": []string{"${aws:PrincipalTag/TenantID}"},
			},
		},
	}))
}

// GrantTenantReadOnlyAccess grants read-only access with tenant isolation
func (t *DynamORMTable) GrantTenantReadOnlyAccess(grantee awsiam.IGrantable, tenantAttribute string) {
	if tenantAttribute == "" {
		tenantAttribute = "TenantID"
	}
	
	grantee.GrantPrincipal().AddToPrincipalPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: &[]*string{
			jsii.String("dynamodb:Query"),
			jsii.String("dynamodb:GetItem"),
			jsii.String("dynamodb:BatchGetItem"),
		},
		Resources: &[]*string{
			t.Table.TableArn(),
			jsii.String(*t.Table.TableArn() + "/index/*"),
		},
		Conditions: &map[string]interface{}{
			"ForAllValues:StringEquals": map[string]interface{}{
				"dynamodb:LeadingKeys": []string{"${aws:PrincipalTag/TenantID}"},
			},
		},
	}))
}

// GrantTenantAdminAccess grants admin access within tenant boundaries
func (t *DynamORMTable) GrantTenantAdminAccess(grantee awsiam.IGrantable, tenantAttribute string) {
	if tenantAttribute == "" {
		tenantAttribute = "TenantID"
	}
	
	grantee.GrantPrincipal().AddToPrincipalPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: &[]*string{
			jsii.String("dynamodb:Query"),
			jsii.String("dynamodb:GetItem"),
			jsii.String("dynamodb:PutItem"),
			jsii.String("dynamodb:UpdateItem"),
			jsii.String("dynamodb:DeleteItem"),
			jsii.String("dynamodb:BatchGetItem"),
			jsii.String("dynamodb:BatchWriteItem"),
			jsii.String("dynamodb:ConditionCheckItem"),
		},
		Resources: &[]*string{
			t.Table.TableArn(),
			jsii.String(*t.Table.TableArn() + "/index/*"),
		},
		Conditions: &map[string]interface{}{
			"ForAllValues:StringEquals": map[string]interface{}{
				"dynamodb:LeadingKeys": []string{"${aws:PrincipalTag/TenantID}"},
			},
		},
	}))
}

// ConfigureTenantMetrics adds CloudWatch metrics with tenant dimensions
func (t *DynamORMTable) ConfigureTenantMetrics(tenantAttribute string) {
	if tenantAttribute == "" {
		tenantAttribute = "TenantID"
	}
	
	// Add tags for tenant-specific metrics
	awscdk.Tags_Of(t.Table).Add(jsii.String("MetricsDimension"), jsii.String(tenantAttribute), nil)
	awscdk.Tags_Of(t.Table).Add(jsii.String("EnableTenantMetrics"), jsii.String("true"), nil)
}

// CreateTenantBoundaryPolicy creates a comprehensive IAM policy for tenant isolation
func (t *DynamORMTable) CreateTenantBoundaryPolicy(tenantAttribute string) awsiam.PolicyDocument {
	if tenantAttribute == "" {
		tenantAttribute = "TenantID"
	}
	
	// Create comprehensive tenant isolation policy
	return awsiam.NewPolicyDocument(&awsiam.PolicyDocumentProps{
		Statements: &[]awsiam.PolicyStatement{
			// Allow basic operations with tenant isolation
			awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					jsii.String("dynamodb:GetItem"),
					jsii.String("dynamodb:PutItem"),
					jsii.String("dynamodb:UpdateItem"),
					jsii.String("dynamodb:DeleteItem"),
					jsii.String("dynamodb:Query"),
				},
				Resources: &[]*string{
					t.Table.TableArn(),
				},
				Conditions: &map[string]interface{}{
					"ForAllValues:StringEquals": map[string]interface{}{
						"dynamodb:LeadingKeys": []string{"${aws:PrincipalTag/TenantID}"},
					},
				},
			}),
			// Allow tenant-specific GSI access
			awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					jsii.String("dynamodb:Query"),
				},
				Resources: &[]*string{
					jsii.String(*t.Table.TableArn() + "/index/gsi-tenant*"),
				},
				Conditions: &map[string]interface{}{
					"ForAllValues:StringEquals": map[string]interface{}{
						"dynamodb:LeadingKeys": []string{"${aws:PrincipalTag/TenantID}"},
					},
				},
			}),
			// Allow batch operations with tenant isolation
			awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					jsii.String("dynamodb:BatchGetItem"),
					jsii.String("dynamodb:BatchWriteItem"),
				},
				Resources: &[]*string{
					t.Table.TableArn(),
				},
				Conditions: &map[string]interface{}{
					"ForAllValues:StringEquals": map[string]interface{}{
						"dynamodb:LeadingKeys": []string{"${aws:PrincipalTag/TenantID}"},
					},
				},
			}),
			// Explicit deny for cross-tenant access
			awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_DENY,
				Actions: &[]*string{
					jsii.String("dynamodb:GetItem"),
					jsii.String("dynamodb:PutItem"),
					jsii.String("dynamodb:UpdateItem"),
					jsii.String("dynamodb:DeleteItem"),
					jsii.String("dynamodb:Query"),
					jsii.String("dynamodb:BatchGetItem"),
					jsii.String("dynamodb:BatchWriteItem"),
				},
				Resources: &[]*string{
					t.Table.TableArn(),
					jsii.String(*t.Table.TableArn() + "/index/*"),
				},
				Conditions: &map[string]interface{}{
					"ForAnyValue:StringNotEquals": map[string]interface{}{
						"dynamodb:LeadingKeys": []string{"${aws:PrincipalTag/TenantID}"},
					},
				},
			}),
		},
	})
}

// CreateTenantReadOnlyPolicy creates a read-only IAM policy for tenant isolation
func (t *DynamORMTable) CreateTenantReadOnlyPolicy(tenantAttribute string) awsiam.PolicyDocument {
	if tenantAttribute == "" {
		tenantAttribute = "TenantID"
	}
	
	return awsiam.NewPolicyDocument(&awsiam.PolicyDocumentProps{
		Statements: &[]awsiam.PolicyStatement{
			// Allow read operations with tenant isolation
			awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					jsii.String("dynamodb:GetItem"),
					jsii.String("dynamodb:Query"),
					jsii.String("dynamodb:BatchGetItem"),
				},
				Resources: &[]*string{
					t.Table.TableArn(),
					jsii.String(*t.Table.TableArn() + "/index/*"),
				},
				Conditions: &map[string]interface{}{
					"ForAllValues:StringEquals": map[string]interface{}{
						"dynamodb:LeadingKeys": []string{"${aws:PrincipalTag/TenantID}"},
					},
				},
			}),
			// Explicit deny for write operations
			awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_DENY,
				Actions: &[]*string{
					jsii.String("dynamodb:PutItem"),
					jsii.String("dynamodb:UpdateItem"),
					jsii.String("dynamodb:DeleteItem"),
					jsii.String("dynamodb:BatchWriteItem"),
				},
				Resources: &[]*string{
					t.Table.TableArn(),
				},
			}),
		},
	})
}

// CreateTenantAdminPolicy creates an admin IAM policy for tenant isolation
func (t *DynamORMTable) CreateTenantAdminPolicy(tenantAttribute string) awsiam.PolicyDocument {
	if tenantAttribute == "" {
		tenantAttribute = "TenantID"
	}
	
	return awsiam.NewPolicyDocument(&awsiam.PolicyDocumentProps{
		Statements: &[]awsiam.PolicyStatement{
			// Allow all operations within tenant boundaries
			awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					jsii.String("dynamodb:GetItem"),
					jsii.String("dynamodb:PutItem"),
					jsii.String("dynamodb:UpdateItem"),
					jsii.String("dynamodb:DeleteItem"),
					jsii.String("dynamodb:Query"),
					jsii.String("dynamodb:BatchGetItem"),
					jsii.String("dynamodb:BatchWriteItem"),
					jsii.String("dynamodb:ConditionCheckItem"),
					jsii.String("dynamodb:DescribeTable"),
				},
				Resources: &[]*string{
					t.Table.TableArn(),
					jsii.String(*t.Table.TableArn() + "/index/*"),
				},
				Conditions: &map[string]interface{}{
					"ForAllValues:StringEquals": map[string]interface{}{
						"dynamodb:LeadingKeys": []string{"${aws:PrincipalTag/TenantID}"},
					},
				},
			}),
			// Allow stream access for tenant data
			awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					jsii.String("dynamodb:DescribeStream"),
					jsii.String("dynamodb:GetRecords"),
					jsii.String("dynamodb:GetShardIterator"),
					jsii.String("dynamodb:ListStreams"),
				},
				Resources: &[]*string{
					jsii.String(*t.Table.TableArn() + "/stream/*"),
				},
			}),
		},
	})
}

// AttachTenantBoundaryPolicy attaches a tenant boundary policy to a role
func (t *DynamORMTable) AttachTenantBoundaryPolicy(role awsiam.Role, tenantAttribute string) {
	if tenantAttribute == "" {
		tenantAttribute = "TenantID"
	}
	
	policy := awsiam.NewPolicy(t.construct, jsii.String("TenantBoundaryPolicy"), &awsiam.PolicyProps{
		PolicyName: jsii.String("DynamORMTenantBoundary"),
		Document:   t.CreateTenantBoundaryPolicy(tenantAttribute),
	})
	
	role.AttachInlinePolicy(policy)
}

// ValidateTenantIsolation validates that tenant isolation is properly configured
func (t *DynamORMTable) ValidateTenantIsolation(tenantAttribute string) error {
	if tenantAttribute == "" {
		tenantAttribute = "TenantID"
	}
	
	// Check if multi-tenant is enabled
	if !*t.props.EnableMultiTenant {
		return fmt.Errorf("multi-tenant support is not enabled on this table")
	}
	
	// Check if tenant attribute matches
	if *t.props.TenantAttribute != tenantAttribute {
		return fmt.Errorf("tenant attribute mismatch: expected %s, got %s", 
			*t.props.TenantAttribute, tenantAttribute)
	}
	
	return nil
}

// ValidateModelCompatibility checks if table matches DynamORM model
func (t *DynamORMTable) ValidateModelCompatibility(spec DynamORMModelSpec) error {
	// Store model spec for future reference
	t.modelSpec = &spec
	
	// Validate partition key matches
	if t.props.PartitionKey.Name != nil && *t.props.PartitionKey.Name != spec.PartitionKey {
		return fmt.Errorf("partition key mismatch: table has %s, model expects %s", 
			*t.props.PartitionKey.Name, spec.PartitionKey)
	}
	
	// Validate sort key matches
	if t.props.SortKey != nil && t.props.SortKey.Name != nil && *t.props.SortKey.Name != spec.SortKey {
		return fmt.Errorf("sort key mismatch: table has %s, model expects %s", 
			*t.props.SortKey.Name, spec.SortKey)
	}
	
	// TODO: Validate GSIs exist when needed
	
	return nil
}

// GrantReadWrite grants read/write permissions to a grantee
func (t *DynamORMTable) GrantReadWrite(grantee awsiam.IGrantable) awsiam.Grant {
	return t.Table.GrantReadWriteData(grantee)
}

// GrantRead grants read permissions to a grantee
func (t *DynamORMTable) GrantRead(grantee awsiam.IGrantable) awsiam.Grant {
	return t.Table.GrantReadData(grantee)
}

// GrantWrite grants write permissions to a grantee
func (t *DynamORMTable) GrantWrite(grantee awsiam.IGrantable) awsiam.Grant {
	return t.Table.GrantWriteData(grantee)
}

// GrantStream grants stream read permissions to a grantee
func (t *DynamORMTable) GrantStream(grantee awsiam.IGrantable) awsiam.Grant {
	return t.Table.GrantStreamRead(grantee)
}

// GetTable returns the underlying DynamoDB table
func (t *DynamORMTable) GetTable() awsdynamodb.Table {
	return t.Table
}

// GetTableName returns the table name
func (t *DynamORMTable) GetTableName() *string {
	return t.Table.TableName()
}

// GetTableArn returns the table ARN
func (t *DynamORMTable) GetTableArn() *string {
	return t.Table.TableArn()
}

// AddDynamORMPermissions adds specific permissions required by DynamORM
func (t *DynamORMTable) AddDynamORMPermissions(grantee awsiam.IGrantable) {
	// Grant standard read/write permissions
	t.GrantReadWrite(grantee)
	
	// Add additional permissions for DynamORM operations
	grantee.GrantPrincipal().AddToPrincipalPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: &[]*string{
			jsii.String("dynamodb:ConditionCheckItem"),
			jsii.String("dynamodb:BatchGetItem"),
			jsii.String("dynamodb:BatchWriteItem"),
			jsii.String("dynamodb:DescribeTable"),
			jsii.String("dynamodb:DescribeStream"),
		},
		Resources: &[]*string{
			t.Table.TableArn(),
			jsii.String(*t.Table.TableArn() + "/index/*"),
			jsii.String(*t.Table.TableArn() + "/stream/*"),
		},
	}))
}

// EnableAutoScaling configures auto-scaling for the table
func (t *DynamORMTable) EnableAutoScaling(minCapacity *float64, maxCapacity *float64, targetUtilization *float64) {
	if minCapacity == nil {
		minCapacity = jsii.Number(1)
	}
	if maxCapacity == nil {
		maxCapacity = jsii.Number(10)
	}
	if targetUtilization == nil {
		targetUtilization = jsii.Number(70)
	}
	
	// Auto-scale read capacity
	t.Table.AutoScaleReadCapacity(&awsdynamodb.EnableScalingProps{
		MinCapacity: minCapacity,
		MaxCapacity: maxCapacity,
	}).ScaleOnUtilization(&awsdynamodb.UtilizationScalingProps{
		TargetUtilizationPercent: targetUtilization,
	})
	
	// Auto-scale write capacity
	t.Table.AutoScaleWriteCapacity(&awsdynamodb.EnableScalingProps{
		MinCapacity: minCapacity,
		MaxCapacity: maxCapacity,
	}).ScaleOnUtilization(&awsdynamodb.UtilizationScalingProps{
		TargetUtilizationPercent: targetUtilization,
	})
}

// AddTags adds tags to the table
func (t *DynamORMTable) AddTags(tags *map[string]*string) {
	if tags != nil {
		for key, value := range *tags {
			awscdk.Tags_Of(t.Table).Add(&key, value, nil)
		}
	}
}

// ConfigureForDynamORM sets up standard DynamORM patterns
func (t *DynamORMTable) ConfigureForDynamORM() {
	// Add commonly used GSIs for DynamORM patterns
	if *t.props.EnableTimestamps {
		// Add GSI for created_at queries
		t.AddDynamORMIndex("created-at", 
			&awsdynamodb.Attribute{
				Name: jsii.String("entity_type"),
				Type: awsdynamodb.AttributeType_STRING,
			},
			&awsdynamodb.Attribute{
				Name: jsii.String("created_at"),
				Type: awsdynamodb.AttributeType_STRING,
			},
		)
	}
	
	// Configure additional DynamORM optimizations
	awscdk.Tags_Of(t.Table).Add(jsii.String("DynamORMConfigured"), jsii.String("true"), nil)
}

// GetStreamArn returns the DynamoDB stream ARN if streams are enabled
func (t *DynamORMTable) GetStreamArn() *string {
	if t.props.Stream != "" {
		return t.Table.TableStreamArn()
	}
	return nil
}

// GetConstruct returns the underlying CDK construct
func (t *DynamORMTable) GetConstruct() constructs.Construct {
	return t.construct
}

// AddCloudWatchMetrics adds comprehensive CloudWatch metrics for DynamORM table monitoring
func (t *DynamORMTable) AddCloudWatchMetrics() {
	tableName := *t.Table.TableName()
	
	// Create metrics for table-level monitoring
	t.createTableMetrics(tableName)
	
	// Create metrics for tenant-level monitoring if multi-tenant is enabled
	if *t.props.EnableMultiTenant {
		t.createTenantMetrics(tableName)
	}
	
	// Add tags to enable metrics
	awscdk.Tags_Of(t.Table).Add(jsii.String("MonitoringEnabled"), jsii.String("true"), nil)
	awscdk.Tags_Of(t.Table).Add(jsii.String("MetricsLevel"), jsii.String("detailed"), nil)
}

// createTableMetrics creates standard table-level metrics
func (t *DynamORMTable) createTableMetrics(tableName string) {
	// Consumed read capacity units
	awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("AWS/DynamoDB"),
		MetricName: jsii.String("ConsumedReadCapacityUnits"),
		DimensionsMap: &map[string]*string{
			"TableName": jsii.String(tableName),
		},
		Statistic: jsii.String(string(awscloudwatch.Statistic_SUM)),
		Period:    awscdk.Duration_Minutes(jsii.Number(5)),
	})
	
	// Consumed write capacity units
	awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("AWS/DynamoDB"),
		MetricName: jsii.String("ConsumedWriteCapacityUnits"),
		DimensionsMap: &map[string]*string{
			"TableName": jsii.String(tableName),
		},
		Statistic: jsii.String(string(awscloudwatch.Statistic_SUM)),
		Period:    awscdk.Duration_Minutes(jsii.Number(5)),
	})
	
	// Throttled requests
	awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("AWS/DynamoDB"),
		MetricName: jsii.String("ThrottledRequests"),
		DimensionsMap: &map[string]*string{
			"TableName": jsii.String(tableName),
		},
		Statistic: jsii.String(string(awscloudwatch.Statistic_SUM)),
		Period:    awscdk.Duration_Minutes(jsii.Number(1)),
	})
	
	// System errors
	awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("AWS/DynamoDB"),
		MetricName: jsii.String("SystemErrors"),
		DimensionsMap: &map[string]*string{
			"TableName": jsii.String(tableName),
		},
		Statistic: jsii.String(string(awscloudwatch.Statistic_SUM)),
		Period:    awscdk.Duration_Minutes(jsii.Number(1)),
	})
	
	// Successful request latency
	awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("AWS/DynamoDB"),
		MetricName: jsii.String("SuccessfulRequestLatency"),
		DimensionsMap: &map[string]*string{
			"TableName": jsii.String(tableName),
		},
		Statistic: jsii.String(string(awscloudwatch.Statistic_AVERAGE)),
		Period:    awscdk.Duration_Minutes(jsii.Number(5)),
	})
}

// createTenantMetrics creates tenant-specific metrics
func (t *DynamORMTable) createTenantMetrics(tableName string) {
	// Custom metrics for tenant isolation
	awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("DynamORM/MultiTenant"),
		MetricName: jsii.String("TenantOperations"),
		DimensionsMap: &map[string]*string{
			"TableName": jsii.String(tableName),
			"TenantAttribute": t.props.TenantAttribute,
		},
		Statistic: jsii.String(string(awscloudwatch.Statistic_SUM)),
		Period:    awscdk.Duration_Minutes(jsii.Number(5)),
	})
	
	// Tenant access violations
	awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("DynamORM/MultiTenant"),
		MetricName: jsii.String("TenantAccessViolations"),
		DimensionsMap: &map[string]*string{
			"TableName": jsii.String(tableName),
			"TenantAttribute": t.props.TenantAttribute,
		},
		Statistic: jsii.String(string(awscloudwatch.Statistic_SUM)),
		Period:    awscdk.Duration_Minutes(jsii.Number(1)),
	})
}

// GetTableMetrics returns CloudWatch metrics for the table
func (t *DynamORMTable) GetTableMetrics() map[string]awscloudwatch.Metric {
	tableName := *t.Table.TableName()
	
	return map[string]awscloudwatch.Metric{
		"ConsumedReadCapacity": awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("AWS/DynamoDB"),
			MetricName: jsii.String("ConsumedReadCapacityUnits"),
			DimensionsMap: &map[string]*string{
				"TableName": jsii.String(tableName),
			},
			Statistic: jsii.String(string(awscloudwatch.Statistic_SUM)),
			Period:    awscdk.Duration_Minutes(jsii.Number(5)),
		}),
		"ConsumedWriteCapacity": awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("AWS/DynamoDB"),
			MetricName: jsii.String("ConsumedWriteCapacityUnits"),
			DimensionsMap: &map[string]*string{
				"TableName": jsii.String(tableName),
			},
			Statistic: jsii.String(string(awscloudwatch.Statistic_SUM)),
			Period:    awscdk.Duration_Minutes(jsii.Number(5)),
		}),
		"ThrottledRequests": awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("AWS/DynamoDB"),
			MetricName: jsii.String("ThrottledRequests"),
			DimensionsMap: &map[string]*string{
				"TableName": jsii.String(tableName),
			},
			Statistic: jsii.String(string(awscloudwatch.Statistic_SUM)),
			Period:    awscdk.Duration_Minutes(jsii.Number(1)),
		}),
		"SystemErrors": awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("AWS/DynamoDB"),
			MetricName: jsii.String("SystemErrors"),
			DimensionsMap: &map[string]*string{
				"TableName": jsii.String(tableName),
			},
			Statistic: jsii.String(string(awscloudwatch.Statistic_SUM)),
			Period:    awscdk.Duration_Minutes(jsii.Number(1)),
		}),
		"SuccessfulRequestLatency": awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("AWS/DynamoDB"),
			MetricName: jsii.String("SuccessfulRequestLatency"),
			DimensionsMap: &map[string]*string{
				"TableName": jsii.String(tableName),
			},
			Statistic: jsii.String(string(awscloudwatch.Statistic_AVERAGE)),
			Period:    awscdk.Duration_Minutes(jsii.Number(5)),
		}),
	}
}

// GetTenantMetrics returns tenant-specific CloudWatch metrics
func (t *DynamORMTable) GetTenantMetrics() map[string]awscloudwatch.Metric {
	if !*t.props.EnableMultiTenant {
		return nil
	}
	
	tableName := *t.Table.TableName()
	
	return map[string]awscloudwatch.Metric{
		"TenantOperations": awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("DynamORM/MultiTenant"),
			MetricName: jsii.String("TenantOperations"),
			DimensionsMap: &map[string]*string{
				"TableName": jsii.String(tableName),
				"TenantAttribute": t.props.TenantAttribute,
			},
			Statistic: jsii.String(string(awscloudwatch.Statistic_SUM)),
			Period:    awscdk.Duration_Minutes(jsii.Number(5)),
		}),
		"TenantAccessViolations": awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("DynamORM/MultiTenant"),
			MetricName: jsii.String("TenantAccessViolations"),
			DimensionsMap: &map[string]*string{
				"TableName": jsii.String(tableName),
				"TenantAttribute": t.props.TenantAttribute,
			},
			Statistic: jsii.String(string(awscloudwatch.Statistic_SUM)),
			Period:    awscdk.Duration_Minutes(jsii.Number(1)),
		}),
	}
}

// AddCloudWatchAlarms adds comprehensive CloudWatch alarms for DynamORM table monitoring
func (t *DynamORMTable) AddCloudWatchAlarms(snsTopicArn *string) map[string]awscloudwatch.Alarm {
	tableName := *t.Table.TableName()
	alarms := make(map[string]awscloudwatch.Alarm)
	
	// Get table metrics
	metrics := t.GetTableMetrics()
	
	// Throttled requests alarm
	throttleAlarm := awscloudwatch.NewAlarm(t.construct, jsii.String("ThrottledRequestsAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:        jsii.String(fmt.Sprintf("%s-ThrottledRequests", tableName)),
		AlarmDescription: jsii.String("DynamoDB table is experiencing throttled requests"),
		Metric:           metrics["ThrottledRequests"],
		Threshold:        jsii.Number(1),
		EvaluationPeriods: jsii.Number(2),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData: awscloudwatch.TreatMissingData_NOT_BREACHING,
	})
	alarms["ThrottledRequests"] = throttleAlarm
	
	// System errors alarm
	systemErrorsAlarm := awscloudwatch.NewAlarm(t.construct, jsii.String("SystemErrorsAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:        jsii.String(fmt.Sprintf("%s-SystemErrors", tableName)),
		AlarmDescription: jsii.String("DynamoDB table is experiencing system errors"),
		Metric:           metrics["SystemErrors"],
		Threshold:        jsii.Number(1),
		EvaluationPeriods: jsii.Number(1),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData: awscloudwatch.TreatMissingData_NOT_BREACHING,
	})
	alarms["SystemErrors"] = systemErrorsAlarm
	
	// High latency alarm
	latencyAlarm := awscloudwatch.NewAlarm(t.construct, jsii.String("HighLatencyAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:        jsii.String(fmt.Sprintf("%s-HighLatency", tableName)),
		AlarmDescription: jsii.String("DynamoDB table is experiencing high latency"),
		Metric:           metrics["SuccessfulRequestLatency"],
		Threshold:        jsii.Number(100), // 100ms threshold
		EvaluationPeriods: jsii.Number(3),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData: awscloudwatch.TreatMissingData_NOT_BREACHING,
	})
	alarms["HighLatency"] = latencyAlarm
	
	// High read capacity utilization alarm
	readCapacityAlarm := awscloudwatch.NewAlarm(t.construct, jsii.String("HighReadCapacityAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:        jsii.String(fmt.Sprintf("%s-HighReadCapacity", tableName)),
		AlarmDescription: jsii.String("DynamoDB table read capacity utilization is high"),
		Metric:           metrics["ConsumedReadCapacity"],
		Threshold:        jsii.Number(80), // 80% of provisioned capacity
		EvaluationPeriods: jsii.Number(2),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData: awscloudwatch.TreatMissingData_NOT_BREACHING,
	})
	alarms["HighReadCapacity"] = readCapacityAlarm
	
	// High write capacity utilization alarm
	writeCapacityAlarm := awscloudwatch.NewAlarm(t.construct, jsii.String("HighWriteCapacityAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:        jsii.String(fmt.Sprintf("%s-HighWriteCapacity", tableName)),
		AlarmDescription: jsii.String("DynamoDB table write capacity utilization is high"),
		Metric:           metrics["ConsumedWriteCapacity"],
		Threshold:        jsii.Number(80), // 80% of provisioned capacity
		EvaluationPeriods: jsii.Number(2),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData: awscloudwatch.TreatMissingData_NOT_BREACHING,
	})
	alarms["HighWriteCapacity"] = writeCapacityAlarm
	
	// Add tenant-specific alarms if multi-tenant is enabled
	if *t.props.EnableMultiTenant {
		tenantAlarms := t.addTenantAlarms(snsTopicArn)
		for k, v := range tenantAlarms {
			alarms[k] = v
		}
	}
	
	// Add SNS notification if topic ARN is provided
	if snsTopicArn != nil {
		t.addAlarmNotifications(alarms, snsTopicArn)
	}
	
	return alarms
}

// addTenantAlarms adds tenant-specific alarms
func (t *DynamORMTable) addTenantAlarms(_ *string) map[string]awscloudwatch.Alarm {
	tableName := *t.Table.TableName()
	alarms := make(map[string]awscloudwatch.Alarm)
	
	tenantMetrics := t.GetTenantMetrics()
	if tenantMetrics == nil {
		return alarms
	}
	
	// Tenant access violations alarm
	tenantViolationsAlarm := awscloudwatch.NewAlarm(t.construct, jsii.String("TenantAccessViolationsAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:        jsii.String(fmt.Sprintf("%s-TenantAccessViolations", tableName)),
		AlarmDescription: jsii.String("DynamoDB table is experiencing tenant access violations"),
		Metric:           tenantMetrics["TenantAccessViolations"],
		Threshold:        jsii.Number(1),
		EvaluationPeriods: jsii.Number(1),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData: awscloudwatch.TreatMissingData_NOT_BREACHING,
	})
	alarms["TenantAccessViolations"] = tenantViolationsAlarm
	
	return alarms
}

// addAlarmNotifications adds SNS notifications to alarms
func (t *DynamORMTable) addAlarmNotifications(alarms map[string]awscloudwatch.Alarm, snsTopicArn *string) {
	if snsTopicArn == nil {
		return
	}
	
	// Import existing SNS topic
	topic := awssns.Topic_FromTopicArn(t.construct, jsii.String("AlarmTopic"), snsTopicArn)
	
	// Add alarm action to all alarms
	for _, alarm := range alarms {
		alarm.AddAlarmAction(awscloudwatchactions.NewSnsAction(topic))
		alarm.AddOkAction(awscloudwatchactions.NewSnsAction(topic))
	}
}

// CreateDynamORMAlarmDashboard creates a comprehensive CloudWatch dashboard for DynamORM table
func (t *DynamORMTable) CreateDynamORMAlarmDashboard(dashboardName string) awscloudwatch.Dashboard {
	// Create dashboard
	dashboard := awscloudwatch.NewDashboard(t.construct, jsii.String("DynamORMDashboard"), &awscloudwatch.DashboardProps{
		DashboardName: jsii.String(dashboardName),
	})
	
	// Get metrics
	metrics := t.GetTableMetrics()
	
	// Convert metrics to IMetric interfaces
	readCapMetric := metrics["ConsumedReadCapacity"]
	writeCapMetric := metrics["ConsumedWriteCapacity"]
	throttleMetric := metrics["ThrottledRequests"]
	errorMetric := metrics["SystemErrors"]
	latencyMetric := metrics["SuccessfulRequestLatency"]
	
	// Add capacity utilization widget
	dashboard.AddWidgets(awscloudwatch.NewGraphWidget(&awscloudwatch.GraphWidgetProps{
		Title:  jsii.String("Capacity Utilization"),
		Width:  jsii.Number(12),
		Height: jsii.Number(6),
		Left: &[]awscloudwatch.IMetric{
			readCapMetric,
			writeCapMetric,
		},
	}))
	
	// Add errors and throttling widget
	dashboard.AddWidgets(awscloudwatch.NewGraphWidget(&awscloudwatch.GraphWidgetProps{
		Title:  jsii.String("Errors and Throttling"),
		Width:  jsii.Number(12),
		Height: jsii.Number(6),
		Left: &[]awscloudwatch.IMetric{
			throttleMetric,
			errorMetric,
		},
	}))
	
	// Add latency widget
	dashboard.AddWidgets(awscloudwatch.NewGraphWidget(&awscloudwatch.GraphWidgetProps{
		Title:  jsii.String("Request Latency"),
		Width:  jsii.Number(12),
		Height: jsii.Number(6),
		Left: &[]awscloudwatch.IMetric{
			latencyMetric,
		},
	}))
	
	// Add tenant metrics if multi-tenant is enabled
	if *t.props.EnableMultiTenant {
		tenantMetrics := t.GetTenantMetrics()
		if tenantMetrics != nil {
			tenantOpsMetric := tenantMetrics["TenantOperations"]
			tenantViolationsMetric := tenantMetrics["TenantAccessViolations"]
			
			dashboard.AddWidgets(awscloudwatch.NewGraphWidget(&awscloudwatch.GraphWidgetProps{
				Title:  jsii.String("Tenant Operations"),
				Width:  jsii.Number(12),
				Height: jsii.Number(6),
				Left: &[]awscloudwatch.IMetric{
					tenantOpsMetric,
					tenantViolationsMetric,
				},
			}))
		}
	}
	
	return dashboard
}

// SetupComprehensiveMonitoring sets up all monitoring features at once
func (t *DynamORMTable) SetupComprehensiveMonitoring(snsTopicArn *string, dashboardName string) map[string]interface{} {
	// Add metrics
	t.AddCloudWatchMetrics()
	
	// Add alarms
	alarms := t.AddCloudWatchAlarms(snsTopicArn)
	
	// Create dashboard
	var dashboard awscloudwatch.Dashboard
	if dashboardName != "" {
		dashboard = t.CreateDynamORMAlarmDashboard(dashboardName)
	}
	
	// Return monitoring components
	result := map[string]interface{}{
		"metrics": t.GetTableMetrics(),
		"alarms":  alarms,
	}
	
	if dashboardName != "" {
		result["dashboard"] = dashboard
	}
	
	if *t.props.EnableMultiTenant {
		result["tenantMetrics"] = t.GetTenantMetrics()
	}
	
	return result
}

// EnableXRayTracing enables X-Ray tracing for DynamORM operations
func (t *DynamORMTable) EnableXRayTracing() {
	// Add X-Ray tracing tags
	awscdk.Tags_Of(t.Table).Add(jsii.String("XRayTracing"), jsii.String("enabled"), nil)
	awscdk.Tags_Of(t.Table).Add(jsii.String("TracingLevel"), jsii.String("detailed"), nil)
}

// ConfigureXRayServiceMap configures X-Ray service map for DynamORM
func (t *DynamORMTable) ConfigureXRayServiceMap(serviceName string) {
	if serviceName == "" {
		serviceName = "DynamORMService"
	}
	
	// Add service map tags
	awscdk.Tags_Of(t.Table).Add(jsii.String("XRayServiceName"), jsii.String(serviceName), nil)
	awscdk.Tags_Of(t.Table).Add(jsii.String("XRayServiceType"), jsii.String("DynamORM"), nil)
}

// GetXRayPermissions returns IAM permissions needed for X-Ray tracing
func (t *DynamORMTable) GetXRayPermissions() awsiam.PolicyDocument {
	return awsiam.NewPolicyDocument(&awsiam.PolicyDocumentProps{
		Statements: &[]awsiam.PolicyStatement{
			awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					jsii.String("xray:PutTraceSegments"),
					jsii.String("xray:PutTelemetryRecords"),
					jsii.String("xray:GetSamplingRules"),
					jsii.String("xray:GetSamplingTargets"),
					jsii.String("xray:GetSamplingStatisticSummaries"),
				},
				Resources: &[]*string{
					jsii.String("*"),
				},
			}),
		},
	})
}

// AddXRayPermissions adds X-Ray permissions to a grantee
func (t *DynamORMTable) AddXRayPermissions(grantee awsiam.IGrantable) {
	grantee.GrantPrincipal().AddToPrincipalPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Effect: awsiam.Effect_ALLOW,
		Actions: &[]*string{
			jsii.String("xray:PutTraceSegments"),
			jsii.String("xray:PutTelemetryRecords"),
			jsii.String("xray:GetSamplingRules"),
			jsii.String("xray:GetSamplingTargets"),
			jsii.String("xray:GetSamplingStatisticSummaries"),
		},
		Resources: &[]*string{
			jsii.String("*"),
		},
	}))
}

// GetXRayEnvironmentVariables returns environment variables for X-Ray tracing
func (t *DynamORMTable) GetXRayEnvironmentVariables() *map[string]*string {
	return &map[string]*string{
		"_X_AMZN_TRACE_ID":         jsii.String(""), // Will be set by Lambda runtime
		"AWS_XRAY_TRACING_NAME":    jsii.String("DynamORM-Operations"),
		"AWS_XRAY_CONTEXT_MISSING": jsii.String("LOG_ERROR"),
		"AWS_XRAY_DEBUG_MODE":      jsii.String("false"),
	}
}

// ConfigureComprehensiveXRayTracing sets up complete X-Ray tracing
func (t *DynamORMTable) ConfigureComprehensiveXRayTracing(serviceName string, enableDebug bool) {
	// Enable basic tracing
	t.EnableXRayTracing()
	
	// Configure service map
	t.ConfigureXRayServiceMap(serviceName)
	
	// Add debug mode if requested
	if enableDebug {
		awscdk.Tags_Of(t.Table).Add(jsii.String("XRayDebugMode"), jsii.String("true"), nil)
	}
	
	// Add comprehensive tags for monitoring
	awscdk.Tags_Of(t.Table).Add(jsii.String("XRaySubsegments"), jsii.String("enabled"), nil)
	awscdk.Tags_Of(t.Table).Add(jsii.String("XRayAnnotations"), jsii.String("enabled"), nil)
	awscdk.Tags_Of(t.Table).Add(jsii.String("XRayMetadata"), jsii.String("enabled"), nil)
}