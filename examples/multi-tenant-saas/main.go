package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/validation"
)

// Domain Models

// Tenant represents a tenant in the multi-tenant system
type Tenant struct {
	ID        string    `json:"id" dynamodbav:"id"`
	Name      string    `json:"name" dynamodbav:"name"`
	Email     string    `json:"email" dynamodbav:"email"`
	Plan      string    `json:"plan" dynamodbav:"plan"`
	Status    string    `json:"status" dynamodbav:"status"`
	CreatedAt time.Time `json:"created_at" dynamodbav:"created_at"`
	UpdatedAt time.Time `json:"updated_at" dynamodbav:"updated_at"`

	// Rate limiting configuration
	RateLimit  int `json:"rate_limit" dynamodbav:"rate_limit"`
	BurstLimit int `json:"burst_limit" dynamodbav:"burst_limit"`
}

// User represents a user within a tenant
type User struct {
	ID        string    `json:"id" dynamodbav:"id"`
	TenantID  string    `json:"tenant_id" dynamodbav:"tenant_id"`
	Email     string    `json:"email" dynamodbav:"email"`
	Name      string    `json:"name" dynamodbav:"name"`
	Role      string    `json:"role" dynamodbav:"role"`
	Status    string    `json:"status" dynamodbav:"status"`
	CreatedAt time.Time `json:"created_at" dynamodbav:"created_at"`
	UpdatedAt time.Time `json:"updated_at" dynamodbav:"updated_at"`
}

// Project represents a project within a tenant
type Project struct {
	ID          string    `json:"id" dynamodbav:"id"`
	TenantID    string    `json:"tenant_id" dynamodbav:"tenant_id"`
	Name        string    `json:"name" dynamodbav:"name"`
	Description string    `json:"description" dynamodbav:"description"`
	Status      string    `json:"status" dynamodbav:"status"`
	OwnerID     string    `json:"owner_id" dynamodbav:"owner_id"`
	CreatedAt   time.Time `json:"created_at" dynamodbav:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" dynamodbav:"updated_at"`
}

// Task represents a task within a project
type Task struct {
	ID          string     `json:"id" dynamodbav:"id"`
	TenantID    string     `json:"tenant_id" dynamodbav:"tenant_id"`
	ProjectID   string     `json:"project_id" dynamodbav:"project_id"`
	Title       string     `json:"title" dynamodbav:"title"`
	Description string     `json:"description" dynamodbav:"description"`
	Status      string     `json:"status" dynamodbav:"status"`
	Priority    string     `json:"priority" dynamodbav:"priority"`
	AssigneeID  string     `json:"assignee_id" dynamodbav:"assignee_id"`
	DueDate     *time.Time `json:"due_date,omitempty" dynamodbav:"due_date,omitempty"`
	CreatedAt   time.Time  `json:"created_at" dynamodbav:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" dynamodbav:"updated_at"`
}

// Request/Response DTOs

// CreateTenantRequest represents a request to create a tenant
type CreateTenantRequest struct {
	Name  string `json:"name" validate:"required,min=2,max=100"`
	Email string `json:"email" validate:"required,email"`
	Plan  string `json:"plan" validate:"required,oneof=free pro enterprise"`
}

// CreateUserRequest represents a request to create a user
type CreateUserRequest struct {
	Email string `json:"email" validate:"required,email"`
	Name  string `json:"name" validate:"required,min=2,max=100"`
	Role  string `json:"role" validate:"required,oneof=admin user viewer"`
}

// CreateProjectRequest represents a request to create a project
type CreateProjectRequest struct {
	Name        string `json:"name" validate:"required,min=2,max=100"`
	Description string `json:"description" validate:"max=500"`
}

// CreateTaskRequest represents a request to create a task
type CreateTaskRequest struct {
	ProjectID   string     `json:"project_id" validate:"required"`
	Title       string     `json:"title" validate:"required,min=2,max=200"`
	Description string     `json:"description" validate:"max=1000"`
	Priority    string     `json:"priority" validate:"required,oneof=low medium high critical"`
	AssigneeID  string     `json:"assignee_id,omitempty"`
	DueDate     *time.Time `json:"due_date,omitempty"`
}

// UpdateTaskRequest represents a request to update a task
type UpdateTaskRequest struct {
	Title       *string    `json:"title,omitempty" validate:"omitempty,min=2,max=200"`
	Description *string    `json:"description,omitempty" validate:"omitempty,max=1000"`
	Status      *string    `json:"status,omitempty" validate:"omitempty,oneof=todo in_progress done"`
	Priority    *string    `json:"priority,omitempty" validate:"omitempty,oneof=low medium high critical"`
	AssigneeID  *string    `json:"assignee_id,omitempty"`
	DueDate     *time.Time `json:"due_date,omitempty"`
}

// PaginatedResponse represents a paginated response
type PaginatedResponse struct {
	Data       any `json:"data"`
	Pagination Pagination  `json:"pagination"`
}

// Pagination represents pagination metadata
type Pagination struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
	NextPage   *int  `json:"next_page,omitempty"`
	PrevPage   *int  `json:"prev_page,omitempty"`
}

// Mock database interface for demonstration
type MockDB interface {
	Put(ctx context.Context, item any) error
	Get(ctx context.Context, id string, item any) error
}

type mockDB struct {
	data map[string]any
}

func newMockDB() MockDB {
	return &mockDB{
		data: make(map[string]any),
	}
}

func (db *mockDB) Put(ctx context.Context, item any) error {
	// Mock implementation
	return nil
}

func (db *mockDB) Get(ctx context.Context, id string, item any) error {
	// Mock implementation
	return nil
}

// Services

// TenantService handles tenant operations
type TenantService struct {
	db MockDB
}

func NewTenantService(db MockDB) *TenantService {
	return &TenantService{db: db}
}

func (s *TenantService) CreateTenant(ctx context.Context, req CreateTenantRequest) (*Tenant, error) {
	tenant := &Tenant{
		ID:         generateID(),
		Name:       req.Name,
		Email:      req.Email,
		Plan:       req.Plan,
		Status:     "active",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		RateLimit:  getRateLimitForPlan(req.Plan),
		BurstLimit: getBurstLimitForPlan(req.Plan),
	}

	if err := s.db.Put(ctx, tenant); err != nil {
		return nil, fmt.Errorf("failed to create tenant: %w", err)
	}

	return tenant, nil
}

func (s *TenantService) GetTenant(ctx context.Context, id string) (*Tenant, error) {
	tenant := &Tenant{}
	if err := s.db.Get(ctx, id, tenant); err != nil {
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}
	return tenant, nil
}

// UserService handles user operations
type UserService struct {
	db MockDB
}

func NewUserService(db MockDB) *UserService {
	return &UserService{db: db}
}

func (s *UserService) CreateUser(ctx context.Context, tenantID string, req CreateUserRequest) (*User, error) {
	user := &User{
		ID:        generateID(),
		TenantID:  tenantID,
		Email:     req.Email,
		Name:      req.Name,
		Role:      req.Role,
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.db.Put(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

func (s *UserService) GetUsersByTenant(ctx context.Context, tenantID string, page, perPage int) ([]*User, int64, error) {
	// This would use DynamORM's query capabilities
	// For now, return mock data
	users := []*User{
		{
			ID:       "user-1",
			TenantID: tenantID,
			Email:    "user1@example.com",
			Name:     "User One",
			Role:     "admin",
			Status:   "active",
		},
	}

	return users, 1, nil
}

// ProjectService handles project operations
type ProjectService struct {
	db MockDB
}

func NewProjectService(db MockDB) *ProjectService {
	return &ProjectService{db: db}
}

func (s *ProjectService) CreateProject(ctx context.Context, tenantID, userID string, req CreateProjectRequest) (*Project, error) {
	project := &Project{
		ID:          generateID(),
		TenantID:    tenantID,
		Name:        req.Name,
		Description: req.Description,
		Status:      "active",
		OwnerID:     userID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.db.Put(ctx, project); err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	return project, nil
}

func (s *ProjectService) GetProjectsByTenant(ctx context.Context, tenantID string, page, perPage int) ([]*Project, int64, error) {
	// This would use DynamORM's query capabilities
	// For now, return mock data
	projects := []*Project{
		{
			ID:          "project-1",
			TenantID:    tenantID,
			Name:        "Sample Project",
			Description: "A sample project",
			Status:      "active",
			OwnerID:     "user-1",
		},
	}

	return projects, 1, nil
}

// TaskService handles task operations
type TaskService struct {
	db MockDB
}

func NewTaskService(db MockDB) *TaskService {
	return &TaskService{db: db}
}

func (s *TaskService) CreateTask(ctx context.Context, tenantID, userID string, req CreateTaskRequest) (*Task, error) {
	task := &Task{
		ID:          generateID(),
		TenantID:    tenantID,
		ProjectID:   req.ProjectID,
		Title:       req.Title,
		Description: req.Description,
		Status:      "todo",
		Priority:    req.Priority,
		AssigneeID:  req.AssigneeID,
		DueDate:     req.DueDate,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.db.Put(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	return task, nil
}

func (s *TaskService) UpdateTask(ctx context.Context, tenantID, taskID string, req UpdateTaskRequest) (*Task, error) {
	task := &Task{}
	if err := s.db.Get(ctx, taskID, task); err != nil {
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	// Verify tenant isolation
	if task.TenantID != tenantID {
		return nil, fmt.Errorf("task not found")
	}

	// Update fields
	if req.Title != nil {
		task.Title = *req.Title
	}
	if req.Description != nil {
		task.Description = *req.Description
	}
	if req.Status != nil {
		task.Status = *req.Status
	}
	if req.Priority != nil {
		task.Priority = *req.Priority
	}
	if req.AssigneeID != nil {
		task.AssigneeID = *req.AssigneeID
	}
	if req.DueDate != nil {
		task.DueDate = req.DueDate
	}

	task.UpdatedAt = time.Now()

	if err := s.db.Put(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to update task: %w", err)
	}

	return task, nil
}

func (s *TaskService) GetTasksByProject(ctx context.Context, tenantID, projectID string, page, perPage int) ([]*Task, int64, error) {
	// This would use DynamORM's query capabilities
	// For now, return mock data
	tasks := []*Task{
		{
			ID:          "task-1",
			TenantID:    tenantID,
			ProjectID:   projectID,
			Title:       "Sample Task",
			Description: "A sample task",
			Status:      "todo",
			Priority:    "medium",
		},
	}

	return tasks, 1, nil
}

// Handlers

// TenantHandlers contains handlers for tenant operations
type TenantHandlers struct {
	service *TenantService
}

func NewTenantHandlers(service *TenantService) *TenantHandlers {
	return &TenantHandlers{service: service}
}

func (h *TenantHandlers) CreateTenant(ctx *lift.Context) error {
	var req CreateTenantRequest
	if err := ctx.ParseRequest(&req); err != nil {
		return lift.BadRequest("Invalid request body")
	}

	if err := validation.Validate(req); err != nil {
		return lift.ValidationError("validation", err.Error())
	}

	tenant, err := h.service.CreateTenant(ctx.Context, req)
	if err != nil {
		if logger := ctx.Logger; logger != nil {
			logger.WithField("error", err.Error()).Error("Failed to create tenant")
		}
		return lift.InternalError("Failed to create tenant")
	}

	return ctx.Status(201).JSON(tenant)
}

func (h *TenantHandlers) GetTenant(ctx *lift.Context) error {
	tenantID := ctx.Param("id")
	if tenantID == "" {
		return lift.BadRequest("Tenant ID is required")
	}

	tenant, err := h.service.GetTenant(ctx.Context, tenantID)
	if err != nil {
		if logger := ctx.Logger; logger != nil {
			logger.WithField("error", err.Error()).Error("Failed to get tenant")
		}
		return lift.NotFound("Tenant not found")
	}

	return ctx.JSON(tenant)
}

// UserHandlers contains handlers for user operations
type UserHandlers struct {
	service *UserService
}

func NewUserHandlers(service *UserService) *UserHandlers {
	return &UserHandlers{service: service}
}

func (h *UserHandlers) CreateUser(ctx *lift.Context) error {
	tenantID := ctx.TenantID()
	if tenantID == "" {
		return lift.BadRequest("Tenant ID is required")
	}

	var req CreateUserRequest
	if err := ctx.ParseRequest(&req); err != nil {
		return lift.BadRequest("Invalid request body")
	}

	if err := validation.Validate(req); err != nil {
		return lift.ValidationError("validation", err.Error())
	}

	user, err := h.service.CreateUser(ctx.Context, tenantID, req)
	if err != nil {
		if logger := ctx.Logger; logger != nil {
			logger.WithField("error", err.Error()).Error("Failed to create user")
		}
		return lift.InternalError("Failed to create user")
	}

	return ctx.Status(201).JSON(user)
}

func (h *UserHandlers) ListUsers(ctx *lift.Context) error {
	tenantID := ctx.TenantID()
	if tenantID == "" {
		return lift.BadRequest("Tenant ID is required")
	}

	page, _ := strconv.Atoi(ctx.Query("page"))
	if page < 1 {
		page = 1
	}

	perPage, _ := strconv.Atoi(ctx.Query("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 10
	}

	users, total, err := h.service.GetUsersByTenant(ctx.Context, tenantID, page, perPage)
	if err != nil {
		if logger := ctx.Logger; logger != nil {
			logger.WithField("error", err.Error()).Error("Failed to list users")
		}
		return lift.InternalError("Failed to list users")
	}

	totalPages := int((total + int64(perPage) - 1) / int64(perPage))

	pagination := Pagination{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	}

	if page < totalPages {
		nextPage := page + 1
		pagination.NextPage = &nextPage
	}

	if page > 1 {
		prevPage := page - 1
		pagination.PrevPage = &prevPage
	}

	response := PaginatedResponse{
		Data:       users,
		Pagination: pagination,
	}

	return ctx.JSON(response)
}

// ProjectHandlers contains handlers for project operations
type ProjectHandlers struct {
	service *ProjectService
}

func NewProjectHandlers(service *ProjectService) *ProjectHandlers {
	return &ProjectHandlers{service: service}
}

func (h *ProjectHandlers) CreateProject(ctx *lift.Context) error {
	tenantID := ctx.TenantID()
	userID := ctx.UserID()

	if tenantID == "" {
		return lift.BadRequest("Tenant ID is required")
	}
	if userID == "" {
		return lift.BadRequest("User ID is required")
	}

	var req CreateProjectRequest
	if err := ctx.ParseRequest(&req); err != nil {
		return lift.BadRequest("Invalid request body")
	}

	if err := validation.Validate(req); err != nil {
		return lift.ValidationError("validation", err.Error())
	}

	project, err := h.service.CreateProject(ctx.Context, tenantID, userID, req)
	if err != nil {
		if logger := ctx.Logger; logger != nil {
			logger.WithField("error", err.Error()).Error("Failed to create project")
		}
		return lift.InternalError("Failed to create project")
	}

	return ctx.Status(201).JSON(project)
}

func (h *ProjectHandlers) ListProjects(ctx *lift.Context) error {
	tenantID := ctx.TenantID()
	if tenantID == "" {
		return lift.BadRequest("Tenant ID is required")
	}

	page, _ := strconv.Atoi(ctx.Query("page"))
	if page < 1 {
		page = 1
	}

	perPage, _ := strconv.Atoi(ctx.Query("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 10
	}

	projects, total, err := h.service.GetProjectsByTenant(ctx.Context, tenantID, page, perPage)
	if err != nil {
		if logger := ctx.Logger; logger != nil {
			logger.WithField("error", err.Error()).Error("Failed to list projects")
		}
		return lift.InternalError("Failed to list projects")
	}

	totalPages := int((total + int64(perPage) - 1) / int64(perPage))

	pagination := Pagination{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	}

	if page < totalPages {
		nextPage := page + 1
		pagination.NextPage = &nextPage
	}

	if page > 1 {
		prevPage := page - 1
		pagination.PrevPage = &prevPage
	}

	response := PaginatedResponse{
		Data:       projects,
		Pagination: pagination,
	}

	return ctx.JSON(response)
}

// TaskHandlers contains handlers for task operations
type TaskHandlers struct {
	service *TaskService
}

func NewTaskHandlers(service *TaskService) *TaskHandlers {
	return &TaskHandlers{service: service}
}

func (h *TaskHandlers) CreateTask(ctx *lift.Context) error {
	tenantID := ctx.TenantID()
	userID := ctx.UserID()

	if tenantID == "" {
		return lift.BadRequest("Tenant ID is required")
	}
	if userID == "" {
		return lift.BadRequest("User ID is required")
	}

	var req CreateTaskRequest
	if err := ctx.ParseRequest(&req); err != nil {
		return lift.BadRequest("Invalid request body")
	}

	if err := validation.Validate(req); err != nil {
		return lift.ValidationError("validation", err.Error())
	}

	task, err := h.service.CreateTask(ctx.Context, tenantID, userID, req)
	if err != nil {
		if logger := ctx.Logger; logger != nil {
			logger.WithField("error", err.Error()).Error("Failed to create task")
		}
		return lift.InternalError("Failed to create task")
	}

	return ctx.Status(201).JSON(task)
}

func (h *TaskHandlers) UpdateTask(ctx *lift.Context) error {
	tenantID := ctx.TenantID()
	taskID := ctx.Param("id")

	if tenantID == "" {
		return lift.BadRequest("Tenant ID is required")
	}
	if taskID == "" {
		return lift.BadRequest("Task ID is required")
	}

	var req UpdateTaskRequest
	if err := ctx.ParseRequest(&req); err != nil {
		return lift.BadRequest("Invalid request body")
	}

	if err := validation.Validate(req); err != nil {
		return lift.ValidationError("validation", err.Error())
	}

	task, err := h.service.UpdateTask(ctx.Context, tenantID, taskID, req)
	if err != nil {
		if logger := ctx.Logger; logger != nil {
			logger.WithField("error", err.Error()).Error("Failed to update task")
		}
		return lift.InternalError("Failed to update task")
	}

	return ctx.JSON(task)
}

func (h *TaskHandlers) ListTasks(ctx *lift.Context) error {
	tenantID := ctx.TenantID()
	projectID := ctx.Query("project_id")

	if tenantID == "" {
		return lift.BadRequest("Tenant ID is required")
	}
	if projectID == "" {
		return lift.BadRequest("Project ID is required")
	}

	page, _ := strconv.Atoi(ctx.Query("page"))
	if page < 1 {
		page = 1
	}

	perPage, _ := strconv.Atoi(ctx.Query("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 10
	}

	tasks, total, err := h.service.GetTasksByProject(ctx.Context, tenantID, projectID, page, perPage)
	if err != nil {
		if logger := ctx.Logger; logger != nil {
			logger.WithField("error", err.Error()).Error("Failed to list tasks")
		}
		return lift.InternalError("Failed to list tasks")
	}

	totalPages := int((total + int64(perPage) - 1) / int64(perPage))

	pagination := Pagination{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	}

	if page < totalPages {
		nextPage := page + 1
		pagination.NextPage = &nextPage
	}

	if page > 1 {
		prevPage := page - 1
		pagination.PrevPage = &prevPage
	}

	response := PaginatedResponse{
		Data:       tasks,
		Pagination: pagination,
	}

	return ctx.JSON(response)
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

func getBurstLimitForPlan(plan string) int {
	switch plan {
	case "free":
		return 10
	case "pro":
		return 50
	case "enterprise":
		return 200
	default:
		return 10
	}
}

// Main application setup

func main() {
	// Initialize mock database
	db := newMockDB()

	// Initialize services
	tenantService := NewTenantService(db)
	userService := NewUserService(db)
	projectService := NewProjectService(db)
	taskService := NewTaskService(db)

	// Initialize handlers
	tenantHandlers := NewTenantHandlers(tenantService)
	userHandlers := NewUserHandlers(userService)
	projectHandlers := NewProjectHandlers(projectService)
	taskHandlers := NewTaskHandlers(taskService)

	// Create Lift app
	app := lift.New()

	// Public routes (no authentication required)
	app.POST("/api/tenants", tenantHandlers.CreateTenant)
	app.GET("/api/health", func(ctx *lift.Context) error {
		return ctx.JSON(map[string]string{
			"status":    "healthy",
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0",
		})
	})

	// Protected routes (simplified for demo)
	app.GET("/api/tenants/:id", tenantHandlers.GetTenant)
	app.POST("/api/users", userHandlers.CreateUser)
	app.GET("/api/users", userHandlers.ListUsers)
	app.POST("/api/projects", projectHandlers.CreateProject)
	app.GET("/api/projects", projectHandlers.ListProjects)
	app.POST("/api/tasks", taskHandlers.CreateTask)
	app.PUT("/api/tasks/:id", taskHandlers.UpdateTask)
	app.GET("/api/tasks", taskHandlers.ListTasks)

	// Metrics and monitoring endpoints
	app.GET("/metrics", func(ctx *lift.Context) error {
		// Return application metrics
		metrics := map[string]any{
			"uptime":           time.Since(time.Now()).String(),
			"memory_usage":     "N/A", // Would implement actual memory tracking
			"active_tenants":   "N/A", // Would implement actual tenant counting
			"requests_per_min": "N/A", // Would implement actual request tracking
		}
		return ctx.JSON(metrics)
	})

	// Start the Lambda handler
	lambda.Start(app.HandleRequest)
}

// Simple logger implementation for demo
type simpleLogger struct{}

func (l *simpleLogger) WithField(key string, value any) lift.Logger {
	return l
}

func (l *simpleLogger) WithFields(fields map[string]any) lift.Logger {
	return l
}

func (l *simpleLogger) Debug(msg string, fields ...map[string]any) {}
func (l *simpleLogger) Info(msg string, fields ...map[string]any)  {}
func (l *simpleLogger) Warn(msg string, fields ...map[string]any)  {}
func (l *simpleLogger) Error(msg string, fields ...map[string]any) {}
func (l *simpleLogger) Fatal(msg string, fields ...map[string]any) {}
