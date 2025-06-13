package lift

import (
	"context"
	"sync"

	"github.com/pay-theory/lift/pkg/lift/adapters"
)

// Config represents the application configuration
type Config struct {
	// Performance settings
	MaxRequestSize  int64 `json:"max_request_size"`
	MaxResponseSize int64 `json:"max_response_size"`
	Timeout         int   `json:"timeout_seconds"`

	// Observability
	LogLevel       string `json:"log_level"`
	MetricsEnabled bool   `json:"metrics_enabled"`
	TracingEnabled bool   `json:"tracing_enabled"`

	// Security
	CORSEnabled    bool     `json:"cors_enabled"`
	AllowedOrigins []string `json:"allowed_origins"`

	// Multi-tenant
	RequireTenantID bool `json:"require_tenant_id"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		MaxRequestSize:  10 * 1024 * 1024, // 10MB
		MaxResponseSize: 6 * 1024 * 1024,  // 6MB (Lambda limit)
		Timeout:         30,               // 30 seconds
		LogLevel:        "INFO",
		MetricsEnabled:  true,
		TracingEnabled:  false,
		CORSEnabled:     true,
		AllowedOrigins:  []string{"*"},
		RequireTenantID: false,
	}
}

// AppOption is a function that configures an App
type AppOption func(*App)

// App represents the main application container
type App struct {
	// Core components
	router     *Router
	middleware []Middleware
	config     *Config

	// Event handling
	adapterRegistry *adapters.AdapterRegistry

	// WebSocket support
	wsRoutes  map[string]WebSocketHandler
	wsOptions *WebSocketOptions

	// Optional integrations
	db       interface{}
	logger   Logger
	metrics  MetricsCollector
	features map[string]bool

	// Runtime state
	started bool
	mu      sync.RWMutex
}

// New creates a new Lift application
func New(options ...AppOption) *App {
	app := &App{
		router:          NewRouter(),
		middleware:      make([]Middleware, 0),
		config:          DefaultConfig(),
		adapterRegistry: adapters.NewAdapterRegistry(),
		features:        make(map[string]bool),
		started:         false,
	}

	// Apply options
	for _, opt := range options {
		opt(app)
	}

	return app
}

// Use adds middleware to the application
func (a *App) Use(middleware Middleware) *App {
	a.middleware = append(a.middleware, middleware)
	return a
}

// GET registers a GET route
func (a *App) GET(path string, handler interface{}) *App {
	return a.Handle("GET", path, handler)
}

// POST registers a POST route
func (a *App) POST(path string, handler interface{}) *App {
	return a.Handle("POST", path, handler)
}

// PUT registers a PUT route
func (a *App) PUT(path string, handler interface{}) *App {
	return a.Handle("PUT", path, handler)
}

// DELETE registers a DELETE route
func (a *App) DELETE(path string, handler interface{}) *App {
	return a.Handle("DELETE", path, handler)
}

// PATCH registers a PATCH route
func (a *App) PATCH(path string, handler interface{}) *App {
	return a.Handle("PATCH", path, handler)
}

// Handle registers a route with the specified method and path
func (a *App) Handle(method, path string, handler interface{}) *App {
	// Convert various handler types to the Handler interface
	var h Handler
	switch v := handler.(type) {
	case Handler:
		h = v
	case func(*Context) error:
		h = HandlerFunc(v)
	default:
		// TODO: Add support for typed handlers via reflection
		panic("unsupported handler type")
	}

	a.router.AddRoute(method, path, h)
	return a
}

// WithConfig sets the application configuration
func (a *App) WithConfig(config *Config) *App {
	a.config = config
	return a
}

// WithLogger sets the application logger
func (a *App) WithLogger(logger Logger) *App {
	a.logger = logger
	return a
}

// WithMetrics sets the metrics collector
func (a *App) WithMetrics(metrics MetricsCollector) *App {
	a.metrics = metrics
	return a
}

// WithDatabase sets the database connection
func (a *App) WithDatabase(db interface{}) *App {
	a.db = db
	return a
}

// Group creates a new route group with the specified prefix
func (a *App) Group(prefix string) *RouteGroup {
	return &RouteGroup{
		app:    a,
		prefix: prefix,
	}
}

// RouteGroup represents a group of routes with a common prefix
type RouteGroup struct {
	app    *App
	prefix string
}

// GET registers a GET route in this group
func (rg *RouteGroup) GET(path string, handler interface{}) *RouteGroup {
	rg.app.GET(rg.prefix+path, handler)
	return rg
}

// POST registers a POST route in this group
func (rg *RouteGroup) POST(path string, handler interface{}) *RouteGroup {
	rg.app.POST(rg.prefix+path, handler)
	return rg
}

// PUT registers a PUT route in this group
func (rg *RouteGroup) PUT(path string, handler interface{}) *RouteGroup {
	rg.app.PUT(rg.prefix+path, handler)
	return rg
}

// DELETE registers a DELETE route in this group
func (rg *RouteGroup) DELETE(path string, handler interface{}) *RouteGroup {
	rg.app.DELETE(rg.prefix+path, handler)
	return rg
}

// PATCH registers a PATCH route in this group
func (rg *RouteGroup) PATCH(path string, handler interface{}) *RouteGroup {
	rg.app.PATCH(rg.prefix+path, handler)
	return rg
}

// Group creates a sub-group with an additional prefix
func (rg *RouteGroup) Group(prefix string) *RouteGroup {
	return &RouteGroup{
		app:    rg.app,
		prefix: rg.prefix + prefix,
	}
}

// Start prepares the application for handling requests
func (a *App) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.started {
		return nil
	}

	// Apply global middleware to router
	a.router.SetMiddleware(a.middleware)

	a.started = true
	return nil
}

// HandleRequest processes an incoming Lambda request
func (a *App) HandleRequest(ctx context.Context, event interface{}) (interface{}, error) {
	// Parse the event into a Request
	req, err := a.parseEvent(event)
	if err != nil {
		return nil, err
	}

	// Create enhanced context
	liftCtx := NewContext(ctx, req)

	// Set dependencies if available
	if a.logger != nil {
		liftCtx.Logger = a.logger
	}
	if a.metrics != nil {
		liftCtx.Metrics = a.metrics
	}
	if a.db != nil {
		liftCtx.DB = a.db
	}

	// Find and execute handler
	if err := a.router.Handle(liftCtx); err != nil {
		// Handle error response
		return a.handleError(liftCtx, err)
	}

	// Return the response
	return liftCtx.Response, nil
}

// parseEvent converts a Lambda event to our Request structure
func (a *App) parseEvent(event interface{}) (*Request, error) {
	// Use the adapter registry to automatically detect and parse the event
	adapterRequest, err := a.adapterRegistry.DetectAndAdapt(event)
	if err != nil {
		return nil, err
	}

	// Wrap the adapter request in our Request type
	return &Request{Request: adapterRequest}, nil
}

// handleError processes errors and returns appropriate responses
func (a *App) handleError(ctx *Context, err error) (interface{}, error) {
	// Set error response
	ctx.Status(500).JSON(map[string]string{
		"error": "Internal server error",
	})

	return ctx.Response, nil
}

// HandleTestRequest processes a test request directly through the router
// This is used by the testing framework to bypass event parsing
func (a *App) HandleTestRequest(ctx *Context) error {
	// Ensure the app is started
	if err := a.Start(); err != nil {
		return err
	}

	// Use the router directly to handle the request
	return a.router.Handle(ctx)
}
