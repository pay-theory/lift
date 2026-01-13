package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// DynamORMScaffoldCommand scaffolds DynamORM models, CDK constructs, and examples
type DynamORMScaffoldCommand struct{}

func (c *DynamORMScaffoldCommand) Name() string { return "scaffold" }
func (c *DynamORMScaffoldCommand) Description() string {
	return "Generate DynamORM models with CDK constructs"
}
func (c *DynamORMScaffoldCommand) Usage() string {
	return "lift dynamorm-scaffold --model <ModelName> [--table <table-name>] [--multi-tenant] [--enable-ttl] [--enable-streams] [--gsi <name:pk:sk>]"
}

// ScaffoldConfig holds the configuration for scaffolding
type ScaffoldConfig struct {
	ModelName     string
	TableName     string
	GSIs          []GSIConfig
	MultiTenant   bool
	EnableTTL     bool
	EnableStreams bool
}

// GSIConfig holds GSI configuration
type GSIConfig struct {
	IndexName    string
	PartitionKey string
	SortKey      string
}

func (c *DynamORMScaffoldCommand) Execute(_ context.Context, args []string) error {
	// Check if we're in a Lift project
	if !c.isLiftProject() {
		return fmt.Errorf("not in a Lift project directory - run 'lift new' first")
	}

	// Parse arguments
	config, err := c.parseScaffoldArgs(args)
	if err != nil {
		return err
	}

	// Create directories
	if err := c.createDirectories(config); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Generate model file
	if err := c.generateModel(config); err != nil {
		return fmt.Errorf("failed to generate model: %w", err)
	}

	// Generate CDK construct
	if err := c.generateCDKConstruct(config); err != nil {
		return fmt.Errorf("failed to generate CDK construct: %w", err)
	}

	// Generate example usage
	if err := c.generateExampleUsage(config); err != nil {
		return fmt.Errorf("failed to generate example: %w", err)
	}

	fmt.Printf("✅ Successfully generated DynamORM scaffold for %s\n", config.ModelName)
	fmt.Printf("📁 Generated files:\n")
	fmt.Printf("   - models/%s.go\n", strings.ToLower(config.ModelName))
	fmt.Printf("   - cdk/constructs/%s_table.go\n", strings.ToLower(config.ModelName))
	fmt.Printf("   - examples/%s_example.go\n", strings.ToLower(config.ModelName))

	return nil
}

func (c *DynamORMScaffoldCommand) isLiftProject() bool {
	// Check for go.mod file
	if _, err := os.Stat("go.mod"); err != nil {
		return false
	}

	// Check if go.mod contains lift dependency
	content, err := os.ReadFile("go.mod")
	if err != nil {
		return false
	}

	return strings.Contains(string(content), "github.com/pay-theory/lift")
}

func (c *DynamORMScaffoldCommand) parseScaffoldArgs(args []string) (*ScaffoldConfig, error) {
	config := &ScaffoldConfig{
		GSIs: []GSIConfig{},
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--model":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--model requires a value")
			}
			config.ModelName = args[i+1]
			i++ // Skip the value
		case "--table":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--table requires a value")
			}
			config.TableName = args[i+1]
			i++ // Skip the value
		case "--multi-tenant":
			config.MultiTenant = true
		case "--enable-ttl":
			config.EnableTTL = true
		case "--enable-streams":
			config.EnableStreams = true
		case "--gsi":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--gsi requires a value")
			}
			gsi, err := c.parseGSI(args[i+1])
			if err != nil {
				return nil, err
			}
			config.GSIs = append(config.GSIs, gsi)
			i++ // Skip the value
		}
	}

	if config.ModelName == "" {
		return nil, fmt.Errorf("--model is required")
	}

	// Default table name if not provided
	if config.TableName == "" {
		config.TableName = strings.ToLower(config.ModelName) + "s"
	}

	return config, nil
}

func (c *DynamORMScaffoldCommand) parseGSI(gsiArg string) (GSIConfig, error) {
	parts := strings.Split(gsiArg, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return GSIConfig{}, fmt.Errorf("--gsi format should be 'IndexName:PartitionKey' or 'IndexName:PartitionKey:SortKey'")
	}

	gsi := GSIConfig{
		IndexName:    parts[0],
		PartitionKey: parts[1],
	}

	if len(parts) == 3 {
		gsi.SortKey = parts[2]
	}

	return gsi, nil
}

func (c *DynamORMScaffoldCommand) createDirectories(_ *ScaffoldConfig) error {
	dirs := []string{
		"models",
		"cdk/constructs",
		"examples",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return err
		}
	}

	return nil
}

func (c *DynamORMScaffoldCommand) generateModel(config *ScaffoldConfig) error {
	tmpl := `package models

import (
	"time"

	"github.com/pay-theory/lift/pkg/dynamorm"
)

// {{.ModelName}} represents a {{.ModelName}} entity
type {{.ModelName}} struct {
	{{if .MultiTenant}}TenantID string ` + "`" + `json:"tenant_id" dynamodb:"tenant_id,pk"` + "`" + `
	{{end}}ID        string    ` + "`" + `json:"id" dynamodb:"id{{if not .MultiTenant}},pk{{end}}"` + "`" + `
	Name      string    ` + "`" + `json:"name" dynamodb:"name"` + "`" + `
	CreatedAt time.Time ` + "`" + `json:"created_at" dynamodb:"created_at"` + "`" + `
	UpdatedAt time.Time ` + "`" + `json:"updated_at" dynamodb:"updated_at"` + "`" + `
	{{if .EnableTTL}}TTL       int64     ` + "`" + `json:"ttl" dynamodb:"ttl"` + "`" + `
	ExpiresAt time.Time ` + "`" + `json:"expires_at" dynamodb:"expires_at"` + "`" + `
	{{end}}{{range .GSIs}}{{.PartitionKey}} string ` + "`" + `json:"{{.PartitionKey | ToLower}}" dynamodb:"{{.PartitionKey | ToLower}}" gsi:"{{.IndexName}}"` + "`" + `
	{{if .SortKey}}{{.SortKey}} string ` + "`" + `json:"{{.SortKey | ToLower}}" dynamodb:"{{.SortKey | ToLower}}" gsi:"{{.IndexName}}"` + "`" + `
	{{end}}{{end}}
}

// New{{.ModelName}} creates a new {{.ModelName}} instance
func New{{.ModelName}}({{if .MultiTenant}}tenantID, {{end}}name string) *{{.ModelName}} {
	now := time.Now()
	{{if .MultiTenant}}id := tenantID + "#" + dynamorm.GenerateID(){{else}}id := dynamorm.GenerateID(){{end}}
	
	return &{{.ModelName}}{
		{{if .MultiTenant}}TenantID:  tenantID,
		{{end}}ID:        id,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
		{{if .EnableTTL}}TTL:       now.Add(24 * time.Hour).Unix(),
		ExpiresAt: now.Add(24 * time.Hour),
		{{end}}
	}
}

// TableName returns the DynamoDB table name
func (m *{{.ModelName}}) TableName() string {
	return "{{.TableName}}"
}

// Update updates the {{.ModelName}} timestamp
func (m *{{.ModelName}}) Update() {
	m.UpdatedAt = time.Now()
	{{if .EnableTTL}}m.TTL = time.Now().Add(24 * time.Hour).Unix()
	m.ExpiresAt = time.Now().Add(24 * time.Hour)
	{{end}}
}
`

	funcMap := template.FuncMap{
		"ToLower": strings.ToLower,
	}

	t, err := template.New("model").Funcs(funcMap).Parse(tmpl)
	if err != nil {
		return err
	}

	filename := filepath.Join("models", strings.ToLower(config.ModelName)+".go")
	file, err := os.Create(filename) // #nosec G304 - filename is constructed from sanitized input in controlled directories
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Printf("Warning: failed to close file: %v\n", err)
		}
	}()

	return t.Execute(file, config)
}

func (c *DynamORMScaffoldCommand) generateCDKConstruct(config *ScaffoldConfig) error {
	tmpl := `package constructs

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// {{.ModelName}}TableProps defines the properties for the {{.ModelName}} table
type {{.ModelName}}TableProps struct {
	awscdk.StackProps
	TableName         *string
	{{if .MultiTenant}}EnableMultiTenant *bool{{end}}
	{{if .EnableTTL}}EnableTTL        *bool{{end}}
	{{if .EnableStreams}}EnableStreams    *bool{{end}}
	RemovalPolicy     awscdk.RemovalPolicy
}

// {{.ModelName}}Table represents a DynamoDB table for {{.ModelName}} entities
type {{.ModelName}}Table struct {
	constructs.Construct
	Table awsdynamodb.Table
}

// New{{.ModelName}}Table creates a new {{.ModelName}} table construct
func New{{.ModelName}}Table(scope constructs.Construct, id *string, props *{{.ModelName}}TableProps) *{{.ModelName}}Table {
	this := &{{.ModelName}}Table{}
	this.Construct = constructs.NewConstruct(scope, id)

	// Set defaults
	if props.TableName == nil {
		props.TableName = jsii.String("{{.TableName}}")
	}
	if props.RemovalPolicy == "" {
		props.RemovalPolicy = awscdk.RemovalPolicy_RETAIN
	}

	// Table configuration
	tableProps := &awsdynamodb.TableProps{
		TableName:     props.TableName,
		BillingMode:   awsdynamodb.BillingMode_PAY_PER_REQUEST,
		RemovalPolicy: props.RemovalPolicy,
		{{if .MultiTenant}}PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("tenant_id"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("id"),
			Type: awsdynamodb.AttributeType_STRING,
		},{{else}}PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("id"),
			Type: awsdynamodb.AttributeType_STRING,
		},{{end}}
		{{if .EnableTTL}}TimeToLiveAttribute: jsii.String("ttl"),{{end}}
		{{if .EnableStreams}}Stream: awsdynamodb.StreamViewType_NEW_AND_OLD_IMAGES,{{end}}
	}

	// Create table
	this.Table = awsdynamodb.NewTable(this, jsii.String("Table"), tableProps)

	{{range .GSIs}}// Add {{.IndexName}} GSI
	this.Table.AddGlobalSecondaryIndex(&awsdynamodb.GlobalSecondaryIndexProps{
		IndexName: jsii.String("{{.IndexName}}"),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("{{.PartitionKey}}"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		{{if .SortKey}}SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("{{.SortKey}}"),
			Type: awsdynamodb.AttributeType_STRING,
		},{{end}}
	})
	{{end}}

	return this
}

// TableName returns the table name
func (t *{{.ModelName}}Table) TableName() *string {
	return t.Table.TableName()
}

// TableArn returns the table ARN
func (t *{{.ModelName}}Table) TableArn() *string {
	return t.Table.TableArn()
}

// GrantReadData grants read permissions to the table
func (t *{{.ModelName}}Table) GrantReadData(grantee awscdk.IPrincipal) {
	t.Table.GrantReadData(grantee)
}

// GrantWriteData grants write permissions to the table
func (t *{{.ModelName}}Table) GrantWriteData(grantee awscdk.IPrincipal) {
	t.Table.GrantWriteData(grantee)
}

// GrantFullAccess grants full access to the table
func (t *{{.ModelName}}Table) GrantFullAccess(grantee awscdk.IPrincipal) {
	t.Table.GrantFullAccess(grantee)
}
`

	t, err := template.New("cdk").Parse(tmpl)
	if err != nil {
		return err
	}

	filename := filepath.Join("cdk", "constructs", strings.ToLower(config.ModelName)+"_table.go")
	file, err := os.Create(filename) // #nosec G304 - filename is constructed from sanitized input in controlled directories
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Printf("Warning: failed to close file: %v\n", err)
		}
	}()

	return t.Execute(file, config)
}

func (c *DynamORMScaffoldCommand) generateExampleUsage(config *ScaffoldConfig) error {
	tmpl := `package main

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/dynamorm"
	"{{.ModuleName}}/models"
)

// {{.ModelName}}Service handles {{.ModelName}} operations
type {{.ModelName}}Service struct {
	db *dynamorm.DB
}

// New{{.ModelName}}Service creates a new {{.ModelName}}Service
func New{{.ModelName}}Service() *{{.ModelName}}Service {
	db, err := dynamorm.New(context.Background())
	if err != nil {
		log.Fatal("Failed to initialize DynamORM:", err)
	}

	return &{{.ModelName}}Service{
		db: db,
	}
}

// Create{{.ModelName}} creates a new {{.ModelName}}
func (s *{{.ModelName}}Service) Create{{.ModelName}}(ctx *lift.Context) error {
	type CreateRequest struct {
		Name string ` + "`" + `json:"name" validate:"required"` + "`" + `
	}

	var req CreateRequest
	if err := ctx.ParseRequest(&req); err != nil {
		return err
	}

	{{if .MultiTenant}}// Get tenant ID from context
	tenantID := ctx.TenantID()
	if tenantID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Tenant ID is required", 400)
	}

	{{.ModelName | ToLower}} := models.New{{.ModelName}}(tenantID, req.Name){{else}}{{.ModelName | ToLower}} := models.New{{.ModelName}}(req.Name){{end}}

	if err := s.db.Put(ctx.Context(), {{.ModelName | ToLower}}); err != nil {
		return lift.NewLiftError("INTERNAL_ERROR", "Failed to create {{.ModelName | ToLower}}", 500)
	}

	return ctx.JSON({{.ModelName | ToLower}})
}

// Get{{.ModelName}} retrieves a {{.ModelName}} by ID
func (s *{{.ModelName}}Service) Get{{.ModelName}}(ctx *lift.Context) error {
	id := ctx.Param("id")
	{{if .MultiTenant}}tenantID := ctx.TenantID()
	if tenantID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Tenant ID is required", 400)
	}

	var {{.ModelName | ToLower}} models.{{.ModelName}}
	if err := s.db.GetByPK(ctx.Context(), tenantID, id, &{{.ModelName | ToLower}}); err != nil {
		if err == dynamorm.ErrNotFound {
			return lift.NotFound("{{.ModelName}} not found")
		}
		return lift.NewLiftError("INTERNAL_ERROR", "Failed to get {{.ModelName | ToLower}}", 500)
	}{{else}}var {{.ModelName | ToLower}} models.{{.ModelName}}
	if err := s.db.GetByPK(ctx.Context(), id, &{{.ModelName | ToLower}}); err != nil {
		if err == dynamorm.ErrNotFound {
			return lift.NotFound("{{.ModelName}} not found")
		}
		return lift.NewLiftError("INTERNAL_ERROR", "Failed to get {{.ModelName | ToLower}}", 500)
	}{{end}}

	return ctx.JSON({{.ModelName | ToLower}})
}

// List{{.ModelName}}s lists all {{.ModelName}}s
func (s *{{.ModelName}}Service) List{{.ModelName}}s(ctx *lift.Context) error {
	{{if .MultiTenant}}tenantID := ctx.TenantID()
	if tenantID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Tenant ID is required", 400)
	}

	var {{.ModelName | ToLower}}s []models.{{.ModelName}}
	if err := s.db.Query(ctx.Context(), &{{.ModelName | ToLower}}s, dynamorm.WithPartitionKey(tenantID)); err != nil {
		return lift.NewLiftError("INTERNAL_ERROR", "Failed to list {{.ModelName | ToLower}}s", 500)
	}{{else}}var {{.ModelName | ToLower}}s []models.{{.ModelName}}
	if err := s.db.Scan(ctx.Context(), &{{.ModelName | ToLower}}s); err != nil {
		return lift.NewLiftError("INTERNAL_ERROR", "Failed to list {{.ModelName | ToLower}}s", 500)
	}{{end}}

	return ctx.JSON({{.ModelName | ToLower}}s)
}

// Update{{.ModelName}} updates a {{.ModelName}}
func (s *{{.ModelName}}Service) Update{{.ModelName}}(ctx *lift.Context) error {
	id := ctx.Param("id")
	{{if .MultiTenant}}tenantID := ctx.TenantID()
	if tenantID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Tenant ID is required", 400)
	}{{end}}

	type UpdateRequest struct {
		Name string ` + "`" + `json:"name" validate:"required"` + "`" + `
	}

	var req UpdateRequest
	if err := ctx.ParseRequest(&req); err != nil {
		return err
	}

	var {{.ModelName | ToLower}} models.{{.ModelName}}
	{{if .MultiTenant}}if err := s.db.GetByPK(ctx.Context(), tenantID, id, &{{.ModelName | ToLower}}); err != nil {
		if err == dynamorm.ErrNotFound {
			return lift.NotFound("{{.ModelName}} not found")
		}
		return lift.NewLiftError("INTERNAL_ERROR", "Failed to get {{.ModelName | ToLower}}", 500)
	}{{else}}if err := s.db.GetByPK(ctx.Context(), id, &{{.ModelName | ToLower}}); err != nil {
		if err == dynamorm.ErrNotFound {
			return lift.NotFound("{{.ModelName}} not found")
		}
		return lift.NewLiftError("INTERNAL_ERROR", "Failed to get {{.ModelName | ToLower}}", 500)
	}{{end}}

	{{.ModelName | ToLower}}.Name = req.Name
	{{.ModelName | ToLower}}.Update()

	if err := s.db.Put(ctx.Context(), &{{.ModelName | ToLower}}); err != nil {
		return lift.NewLiftError("INTERNAL_ERROR", "Failed to update {{.ModelName | ToLower}}", 500)
	}

	return ctx.JSON({{.ModelName | ToLower}})
}

// Delete{{.ModelName}} deletes a {{.ModelName}}
func (s *{{.ModelName}}Service) Delete{{.ModelName}}(ctx *lift.Context) error {
	id := ctx.Param("id")
	{{if .MultiTenant}}tenantID := ctx.TenantID()
	if tenantID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Tenant ID is required", 400)
	}

	var {{.ModelName | ToLower}} models.{{.ModelName}}
	{{.ModelName | ToLower}}.TenantID = tenantID
	{{.ModelName | ToLower}}.ID = id{{else}}var {{.ModelName | ToLower}} models.{{.ModelName}}
	{{.ModelName | ToLower}}.ID = id{{end}}

	if err := s.db.Delete(ctx.Context(), &{{.ModelName | ToLower}}); err != nil {
		return lift.NewLiftError("INTERNAL_ERROR", "Failed to delete {{.ModelName | ToLower}}", 500)
	}

	return ctx.JSON(map[string]string{"message": "{{.ModelName}} deleted successfully"})
}

func main() {
	// Create Lift app
	app := lift.New()

	// Create {{.ModelName}} service
	{{.ModelName | ToLower}}Service := New{{.ModelName}}Service()

	// Routes
	{{if .MultiTenant}}api := app.Group("/api")
	{{.ModelName | ToLower}}Routes := api.Group("/{{.ModelName | ToLower}}s"){{else}}{{.ModelName | ToLower}}Routes := app.Group("/{{.ModelName | ToLower}}s"){{end}}

	{{.ModelName | ToLower}}Routes.POST("", {{.ModelName | ToLower}}Service.Create{{.ModelName}})
	{{.ModelName | ToLower}}Routes.GET("", {{.ModelName | ToLower}}Service.List{{.ModelName}}s)
	{{.ModelName | ToLower}}Routes.GET("/:id", {{.ModelName | ToLower}}Service.Get{{.ModelName}})
	{{.ModelName | ToLower}}Routes.PUT("/:id", {{.ModelName | ToLower}}Service.Update{{.ModelName}})
	{{.ModelName | ToLower}}Routes.DELETE("/:id", {{.ModelName | ToLower}}Service.Delete{{.ModelName}})

	// Start Lambda handler
	lambda.Start(app.Handler())
}
`

	// Get module name from go.mod
	moduleName := "your-module"
	if content, err := os.ReadFile("go.mod"); err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "module ") {
				moduleName = strings.TrimSpace(strings.TrimPrefix(line, "module "))
				break
			}
		}
	}

	data := struct {
		*ScaffoldConfig
		ModuleName string
	}{
		ScaffoldConfig: config,
		ModuleName:     moduleName,
	}

	funcMap := template.FuncMap{
		"ToLower": strings.ToLower,
	}

	t, err := template.New("example").Funcs(funcMap).Parse(tmpl)
	if err != nil {
		return err
	}

	filename := filepath.Join("examples", strings.ToLower(config.ModelName)+"_example.go")
	file, err := os.Create(filename) // #nosec G304 - filename is constructed from sanitized input in controlled directories
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Printf("Warning: failed to close file: %v\n", err)
		}
	}()

	return t.Execute(file, data)
}
