package main

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/middleware"
	"github.com/pay-theory/lift/pkg/validation"
)

// Tenant represents a tenant in the multi-tenant DynamORM system
type Tenant struct {
	PK         string    `dynamorm:"pk" json:"pk"`                              // tenant#{id}
	SK         string    `dynamorm:"sk" json:"sk"`                              // tenant#{id}
	TenantID   string    `dynamorm:"index:tenant-entity,pk" json:"tenant_id"`   // For GSI
	EntityType string    `dynamorm:"index:tenant-entity,sk" json:"entity_type"` // "tenant"
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	Plan       string    `json:"plan"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	RateLimit  int       `json:"rate_limit"`
}

// User represents a user within a tenant using DynamORM patterns
type User struct {
	PK         string    `dynamorm:"pk" json:"pk"`                              // tenant#{tenant_id}
	SK         string    `dynamorm:"sk" json:"sk"`                              // user#{id}
	TenantID   string    `dynamorm:"index:tenant-entity,pk" json:"tenant_id"`   // For tenant isolation
	EntityType string    `dynamorm:"index:tenant-entity,sk" json:"entity_type"` // "user"
	ID         string    `json:"id"`
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	Role       string    `json:"role"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Project represents a project within a tenant using DynamORM patterns
type Project struct {
	PK          string    `dynamorm:"pk" json:"pk"`                              // tenant#{tenant_id}
	SK          string    `dynamorm:"sk" json:"sk"`                              // project#{id}
	TenantID    string    `dynamorm:"index:tenant-entity,pk" json:"tenant_id"`   // For tenant isolation
	EntityType  string    `dynamorm:"index:tenant-entity,sk" json:"entity_type"` // "project"
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	OwnerID     string    `json:"owner_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Request DTOs
type CreateTenantRequest struct {
	Name  string `json:"name" validate:"required,min=2,max=100"`
	Email string `json:"email" validate:"required,email"`
	Plan  string `json:"plan" validate:"required,oneof=free pro enterprise"`
}

type CreateUserRequest struct {
	Email string `json:"email" validate:"required,email"`
	Name  string `json:"name" validate:"required,min=2,max=100"`
	Role  string `json:"role" validate:"required,oneof=admin user viewer"`
}

type CreateProjectRequest struct {
	Name        string `json:"name" validate:"required,min=2,max=100"`
	Description string `json:"description" validate:"max=500"`
}

// Mock DynamORM service to demonstrate patterns
type DynamORMService struct {
	tableName string
}

func NewDynamORMService(tableName string) *DynamORMService {
	return &DynamORMService{tableName: tableName}
}

func (s *DynamORMService) PutItem(ctx context.Context, item interface{}) error {
	// In real implementation, this would use DynamORM client
	fmt.Printf("DynamORM: Putting item to table %s\n", s.tableName)
	return nil
}

func (s *DynamORMService) GetItem(ctx context.Context, pk, sk string, item interface{}) error {
	// In real implementation, this would use DynamORM client
	fmt.Printf("DynamORM: Getting item from table %s with PK=%s, SK=%s\n", s.tableName, pk, sk)
	return nil
}

func (s *DynamORMService) QueryByTenant(ctx context.Context, tenantID string, entityType string, items interface{}) error {
	// In real implementation, this would query the tenant GSI
	fmt.Printf("DynamORM: Querying tenant %s for entity type %s\n", tenantID, entityType)
	return nil
}

// Services implementing DynamORM patterns
type TenantService struct {
	db *DynamORMService
}

func NewTenantService(db *DynamORMService) *TenantService {
	return &TenantService{db: db}
}

func (s *TenantService) CreateTenant(ctx context.Context, req CreateTenantRequest) (*Tenant, error) {
	id := generateID()
	tenant := &Tenant{
		PK:         fmt.Sprintf("tenant#%s", id),
		SK:         fmt.Sprintf("tenant#%s", id),
		TenantID:   id,
		EntityType: "tenant",
		ID:         id,
		Name:       req.Name,
		Email:      req.Email,
		Plan:       req.Plan,
		Status:     "active",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		RateLimit:  getRateLimitForPlan(req.Plan),
	}

	return tenant, s.db.PutItem(ctx, tenant)
}

func (s *TenantService) GetTenant(ctx context.Context, id string) (*Tenant, error) {
	tenant := &Tenant{}
	pk := fmt.Sprintf("tenant#%s", id)
	sk := fmt.Sprintf("tenant#%s", id)

	if err := s.db.GetItem(ctx, pk, sk, tenant); err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}

	return tenant, nil
}

type UserService struct {
	db *DynamORMService
}

func NewUserService(db *DynamORMService) *UserService {
	return &UserService{db: db}
}

func (s *UserService) CreateUser(ctx context.Context, tenantID string, req CreateUserRequest) (*User, error) {
	id := generateID()
	user := &User{
		PK:         fmt.Sprintf("tenant#%s", tenantID),
		SK:         fmt.Sprintf("user#%s", id),
		TenantID:   tenantID,
		EntityType: "user",
		ID:         id,
		Email:      req.Email,
		Name:       req.Name,
		Role:       req.Role,
		Status:     "active",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	return user, s.db.PutItem(ctx, user)
}

func (s *UserService) GetUsersByTenant(ctx context.Context, tenantID string) ([]*User, error) {
	var users []*User
	err := s.db.QueryByTenant(ctx, tenantID, "user", &users)
	return users, err
}

type ProjectService struct {
	db *DynamORMService
}

func NewProjectService(db *DynamORMService) *ProjectService {
	return &ProjectService{db: db}
}

func (s *ProjectService) CreateProject(ctx context.Context, tenantID, ownerID string, req CreateProjectRequest) (*Project, error) {
	id := generateID()
	project := &Project{
		PK:          fmt.Sprintf("tenant#%s", tenantID),
		SK:          fmt.Sprintf("project#%s", id),
		TenantID:    tenantID,
		EntityType:  "project",
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Status:      "active",
		OwnerID:     ownerID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	return project, s.db.PutItem(ctx, project)
}

func (s *ProjectService) GetProjectsByTenant(ctx context.Context, tenantID string) ([]*Project, error) {
	var projects []*Project
	err := s.db.QueryByTenant(ctx, tenantID, "project", &projects)
	return projects, err
}

// Handlers with tenant isolation
type TenantHandlers struct {
	service *TenantService
}

func NewTenantHandlers(service *TenantService) *TenantHandlers {
	return &TenantHandlers{service: service}
}

func (h *TenantHandlers) CreateTenant(ctx *lift.Context) error {
	var req CreateTenantRequest
	if err := ctx.ParseRequest(&req); err != nil {
		return lift.NewLiftError("BAD_REQUEST", "Invalid request body", 400)
	}

	if err := validation.Validate(req); err != nil {
		return lift.ValidationError(err.Error())
	}

	tenant, err := h.service.CreateTenant(ctx.Context, req)
	if err != nil {
		return lift.NewLiftError("INTERNAL_ERROR", "Failed to create tenant", 500)
	}

	// Set tenant ID in response headers for client tracking
	ctx.Response.Header("X-Tenant-ID", tenant.ID)
	return ctx.Status(201).JSON(tenant)
}

func (h *TenantHandlers) GetTenant(ctx *lift.Context) error {
	tenantID := ctx.Param("id")
	if tenantID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Tenant ID is required", 400)
	}

	tenant, err := h.service.GetTenant(ctx.Context, tenantID)
	if err != nil {
		return lift.NotFound("Tenant not found")
	}

	return ctx.JSON(tenant)
}

type UserHandlers struct {
	service *UserService
}

func NewUserHandlers(service *UserService) *UserHandlers {
	return &UserHandlers{service: service}
}

func (h *UserHandlers) CreateUser(ctx *lift.Context) error {
	// Extract tenant ID from context (would be set by middleware)
	tenantID := ctx.TenantID()
	if tenantID == "" {
		return lift.NewLiftError("UNAUTHORIZED", "Tenant context required", 401)
	}

	var req CreateUserRequest
	if err := ctx.ParseRequest(&req); err != nil {
		return lift.NewLiftError("BAD_REQUEST", "Invalid request body", 400)
	}

	if err := validation.Validate(req); err != nil {
		return lift.ValidationError(err.Error())
	}

	user, err := h.service.CreateUser(ctx.Context, tenantID, req)
	if err != nil {
		return lift.NewLiftError("INTERNAL_ERROR", "Failed to create user", 500)
	}

	return ctx.Status(201).JSON(user)
}

func (h *UserHandlers) ListUsers(ctx *lift.Context) error {
	tenantID := ctx.TenantID()
	if tenantID == "" {
		return lift.NewLiftError("UNAUTHORIZED", "Tenant context required", 401)
	}

	users, err := h.service.GetUsersByTenant(ctx.Context, tenantID)
	if err != nil {
		return lift.NewLiftError("INTERNAL_ERROR", "Failed to list users", 500)
	}

	return ctx.JSON(map[string]interface{}{
		"users":     users,
		"tenant_id": tenantID,
		"count":     len(users),
	})
}

type ProjectHandlers struct {
	service *ProjectService
}

func NewProjectHandlers(service *ProjectService) *ProjectHandlers {
	return &ProjectHandlers{service: service}
}

func (h *ProjectHandlers) CreateProject(ctx *lift.Context) error {
	tenantID := ctx.TenantID()
	userID := ctx.UserID()

	if tenantID == "" || userID == "" {
		return lift.NewLiftError("UNAUTHORIZED", "Tenant and user context required", 401)
	}

	var req CreateProjectRequest
	if err := ctx.ParseRequest(&req); err != nil {
		return lift.NewLiftError("BAD_REQUEST", "Invalid request body", 400)
	}

	if err := validation.Validate(req); err != nil {
		return lift.ValidationError(err.Error())
	}

	project, err := h.service.CreateProject(ctx.Context, tenantID, userID, req)
	if err != nil {
		return lift.NewLiftError("INTERNAL_ERROR", "Failed to create project", 500)
	}

	return ctx.Status(201).JSON(project)
}

func (h *ProjectHandlers) ListProjects(ctx *lift.Context) error {
	tenantID := ctx.TenantID()
	if tenantID == "" {
		return lift.NewLiftError("UNAUTHORIZED", "Tenant context required", 401)
	}

	projects, err := h.service.GetProjectsByTenant(ctx.Context, tenantID)
	if err != nil {
		return lift.NewLiftError("INTERNAL_ERROR", "Failed to list projects", 500)
	}

	return ctx.JSON(map[string]interface{}{
		"projects":  projects,
		"tenant_id": tenantID,
		"count":     len(projects),
	})
}

// Tenant isolation middleware
func TenantIsolationMiddleware() lift.Middleware {
	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			// Extract tenant ID from header or JWT token
			tenantID := ctx.Header("X-Tenant-ID")
			if tenantID == "" {
				// In a real app, you'd extract this from a JWT token
				tenantID = "demo-tenant"
			}

			// Set tenant ID in context
			ctx.Set("tenant_id", tenantID)

			// Log tenant access for monitoring
			if logger := ctx.Logger; logger != nil {
				logger.WithField("tenant_id", tenantID).Info("Tenant access")
			}

			return next.Handle(ctx)
		})
	}
}

// Utility functions
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func getRateLimitForPlan(plan string) int {
	switch plan {
	case "free":
		return 100
	case "pro":
		return 1000
	case "enterprise":
		return 10000
	default:
		return 100
	}
}

// Main application demonstrating DynamORM multi-tenant patterns
func main() {
	// Initialize DynamORM service (in real app, this would be properly configured)
	tableName := "DynamORMMultiTenantTable"
	db := NewDynamORMService(tableName)

	// Initialize services
	tenantService := NewTenantService(db)
	userService := NewUserService(db)
	projectService := NewProjectService(db)

	// Initialize handlers
	tenantHandlers := NewTenantHandlers(tenantService)
	userHandlers := NewUserHandlers(userService)
	projectHandlers := NewProjectHandlers(projectService)

	// Create Lift app
	app := lift.New()

	// Add middleware
	app.Use(lift.Middleware(middleware.Logger()))
	app.Use(lift.Middleware(middleware.Recover()))
	app.Use(lift.Middleware(middleware.CORS([]string{"*"})))

	// Add tenant-specific rate limiting
	rateLimiter, err := middleware.TenantRateLimitWithLimited(1000, time.Hour)
	if err == nil {
		app.Use(rateLimiter)
	}

	// Public routes
	app.POST("/api/tenants", tenantHandlers.CreateTenant)
	app.GET("/api/health", func(ctx *lift.Context) error {
		return ctx.JSON(map[string]interface{}{
			"status":     "healthy",
			"timestamp":  time.Now().Format(time.RFC3339),
			"version":    "1.0.0",
			"table_name": tableName,
			"features": []string{
				"multi-tenant",
				"dynamorm",
				"tenant-isolation",
				"rate-limiting",
				"monitoring",
			},
		})
	})

	// Add tenant isolation middleware
	app.Use(TenantIsolationMiddleware())

	// Tenant-scoped routes
	tenantGroup := app.Group("/api")

	// Tenant management
	tenantGroup.GET("/tenants/:id", tenantHandlers.GetTenant)

	// User management (tenant-scoped)
	tenantGroup.POST("/users", userHandlers.CreateUser)
	tenantGroup.GET("/users", userHandlers.ListUsers)

	// Project management (tenant-scoped)
	tenantGroup.POST("/projects", projectHandlers.CreateProject)
	tenantGroup.GET("/projects", projectHandlers.ListProjects)

	// Metrics endpoint showing tenant-specific data
	tenantGroup.GET("/metrics", func(ctx *lift.Context) error {
		tenantID := ctx.TenantID()
		return ctx.JSON(map[string]interface{}{
			"tenant_id":  tenantID,
			"table_name": tableName,
			"access_patterns": []string{
				fmt.Sprintf("PK: tenant#%s", tenantID),
				"SK: user#{id}, project#{id}",
				"GSI1: tenant_id, entity_type",
				"GSI2: status, tenant_id",
			},
			"isolation_features": []string{
				"IAM policies with tenant boundary enforcement",
				"DynamORM tenant-scoped GSIs",
				"CloudWatch metrics with tenant dimensions",
				"X-Ray tracing with tenant context",
			},
		})
	})

	// Admin endpoints (would require admin authentication in real app)
	app.GET("/admin/tenants", func(ctx *lift.Context) error {
		// This would query all tenants (admin only)
		return ctx.JSON(map[string]interface{}{
			"message": "Admin endpoint - would list all tenants",
			"note":    "Requires admin authentication in production",
		})
	})

	// Start the Lambda handler
	lambda.Start(app.HandleRequest)
}
