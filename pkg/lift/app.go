package lift

import (
	"context"
	"fmt"
	"os"
	"reflect"
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
	router      *Router      // HTTP router
	eventRouter *EventRouter // Non-HTTP event router
	middleware  []Middleware
	config      *Config

	// Event handling
	adapterRegistry *adapters.AdapterRegistry

	// WebSocket support
	wsRoutes  map[string]WebSocketHandler
	wsOptions *WebSocketOptions

	// Optional integrations
	db       any
	logger   Logger
	metrics  MetricsCollector
	features map[string]bool

	// Runtime state
	started bool
	mu      sync.RWMutex

	// Response buffering
	hasInterceptingMiddleware bool
}

// New creates a new Lift application
func New(options ...AppOption) *App {
	app := &App{
		router:          NewRouter(),
		eventRouter:     NewEventRouter(),
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
	
	// Note: Since Middleware is a function type, not an interface,
	// we'll need to handle response interception detection differently
	// For now, we'll assume middleware that needs interception will
	// be wrapped with InterceptingMiddleware
	
	return a
}

// GET registers a GET route
func (a *App) GET(path string, handler any) error {
	return a.Handle("GET", path, handler)
}

// POST registers a POST route
func (a *App) POST(path string, handler any) error {
	return a.Handle("POST", path, handler)
}

// PUT registers a PUT route
func (a *App) PUT(path string, handler any) error {
	return a.Handle("PUT", path, handler)
}

// DELETE registers a DELETE route
func (a *App) DELETE(path string, handler any) error {
	return a.Handle("DELETE", path, handler)
}

// PATCH registers a PATCH route
func (a *App) PATCH(path string, handler any) error {
	return a.Handle("PATCH", path, handler)
}

// Handle registers a route with the specified method and path
func (a *App) Handle(method, path string, handler any) error {
	// Check if this is an event trigger type
	triggerType := parseTriggerType(method)
	if triggerType != TriggerUnknown && triggerType != TriggerAPIGateway {
		// This is a non-HTTP event, use the event router
		var eventHandler EventHandler
		switch v := handler.(type) {
		case EventHandler:
			eventHandler = v
		case func(*Context) error:
			eventHandler = EventHandlerFunc(v)
		default:
			// Convert to event handler
			h, err := convertHandlerUsingReflection(handler)
			if err != nil {
				return fmt.Errorf("unsupported handler type: %w", err)
			}
			eventHandler = EventHandlerFunc(h.Handle)
		}

		a.eventRouter.AddEventRoute(triggerType, path, eventHandler)
		return nil
	}

	// This is an HTTP route
	var h Handler
	switch v := handler.(type) {
	case Handler:
		h = v
	case func(*Context) error:
		h = HandlerFunc(v)
	default:
		// Use reflection to support additional handler types
		reflectedHandler, err := convertHandlerUsingReflection(handler)
		if err != nil {
			return fmt.Errorf("unsupported handler type: %w", err)
		}
		h = reflectedHandler
	}

	a.router.AddRoute(method, path, h)
	return nil
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
func (a *App) WithDatabase(db any) *App {
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
func (rg *RouteGroup) GET(path string, handler any) error {
	return rg.app.GET(rg.prefix+path, handler)
}

// POST registers a POST route in this group
func (rg *RouteGroup) POST(path string, handler any) error {
	return rg.app.POST(rg.prefix+path, handler)
}

// PUT registers a PUT route in this group
func (rg *RouteGroup) PUT(path string, handler any) error {
	return rg.app.PUT(rg.prefix+path, handler)
}

// DELETE registers a DELETE route in this group
func (rg *RouteGroup) DELETE(path string, handler any) error {
	return rg.app.DELETE(rg.prefix+path, handler)
}

// PATCH registers a PATCH route in this group
func (rg *RouteGroup) PATCH(path string, handler any) error {
	return rg.app.PATCH(rg.prefix+path, handler)
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

// IsLambda returns true if the code is running in an AWS Lambda environment
func (a *App) IsLambda() bool {
	// Check for AWS Lambda-specific environment variables
	// AWS_LAMBDA_FUNCTION_NAME is set in all Lambda runtime environments
	// LAMBDA_TASK_ROOT is the path to the Lambda function code
	// AWS_EXECUTION_ENV contains the runtime identifier (e.g., AWS_Lambda_go1.x)
	return os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" ||
		os.Getenv("LAMBDA_TASK_ROOT") != "" ||
		os.Getenv("AWS_EXECUTION_ENV") != ""
}

// HandleRequest processes an incoming Lambda request
func (a *App) HandleRequest(ctx context.Context, event any) (any, error) {
	// Ensure the app is started (transfers middleware to router)
	if err := a.Start(); err != nil {
		return nil, err
	}

	// Parse the event into a Request
	req, err := a.parseEvent(event)
	if err != nil {
		return nil, err
	}

	// Create enhanced context
	liftCtx := NewContext(ctx, req)

	// Enable response buffering if any middleware needs it
	if a.hasInterceptingMiddleware {
		liftCtx.EnableResponseBuffering()
	}

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

	// Route based on trigger type
	var routeErr error
	if req.TriggerType == adapters.TriggerWebSocket {
		// WebSocket event, use WebSocket routing
		routeKey := ""
		if metadata, ok := req.Metadata["routeKey"].(string); ok {
			routeKey = metadata
		}

		handler := a.RouteWebSocket(routeKey)
		if handler == nil {
			routeErr = NewLiftError("WEBSOCKET_ROUTE_NOT_FOUND", fmt.Sprintf("No handler for WebSocket route: %s", routeKey), 404)
		} else {
			// Apply middleware and execute handler
			finalHandler := handler
			for i := len(a.middleware) - 1; i >= 0; i-- {
				finalHandler = a.middleware[i](finalHandler)
			}

			// Execute with automatic connection management if enabled
			if a.wsOptions != nil && a.wsOptions.EnableAutoConnectionManagement {
				finalHandler = wrapWithConnectionManagement(finalHandler, a.wsOptions.ConnectionStore)
			}

			if err := finalHandler.Handle(liftCtx); err != nil {
				routeErr = err
			}
		}
	} else if req.TriggerType != adapters.TriggerAPIGateway && req.TriggerType != adapters.TriggerAPIGatewayV2 && req.TriggerType != adapters.TriggerUnknown {
		// Non-HTTP event, use event router
		if err := a.eventRouter.HandleEvent(liftCtx); err != nil {
			routeErr = err
		}
	} else {
		// HTTP event, use regular router
		if err := a.router.Handle(liftCtx); err != nil {
			routeErr = err
		}
	}

	// Handle any routing errors
	if routeErr != nil {
		return a.handleError(liftCtx, routeErr)
	}

	// Flush response buffer if enabled
	if err := liftCtx.FlushResponse(); err != nil {
		return nil, err
	}

	// Return the response
	return liftCtx.Response, nil
}

// parseEvent converts a Lambda event to our Request structure
func (a *App) parseEvent(event any) (*Request, error) {
	// Use the adapter registry to automatically detect and parse the event
	adapterRequest, err := a.adapterRegistry.DetectAndAdapt(event)
	if err != nil {
		return nil, err
	}

	// Properly wrap the adapter request using NewRequest to copy all fields
	return NewRequest(adapterRequest), nil
}

// handleError processes errors and returns appropriate responses
func (a *App) handleError(ctx *Context, err error) (any, error) {
	// Handle Lift errors properly by setting appropriate status codes
	if liftErr, ok := err.(*LiftError); ok {
		resp := map[string]any{
			"code":    liftErr.Code,
			"message": liftErr.Message,
		}
		
		// Include details if present
		if len(liftErr.Details) > 0 {
			resp["details"] = liftErr.Details
		}
		
		ctx.Status(liftErr.StatusCode).JSON(resp)
		return ctx.Response, nil
	}

	// For non-Lift errors, set 500 status
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
	if err := a.router.Handle(ctx); err != nil {
		// Handle Lift errors properly by setting appropriate status codes
		if liftErr, ok := err.(*LiftError); ok {
			ctx.Status(liftErr.StatusCode).JSON(map[string]any{
				"error":   liftErr.Code,
				"message": liftErr.Message,
			})
			return nil // Don't return error, status is set in response
		}

		// For non-Lift errors, set 500 status
		ctx.Status(500).JSON(map[string]any{
			"error":   "Internal Server Error",
			"message": err.Error(),
		})
		return nil // Don't return error, status is set in response
	}

	return nil
}

// GetEventRouter returns the EventRouter for accessing event routes (mainly for testing)
func (a *App) GetEventRouter() *EventRouter {
	return a.eventRouter
}

// SQS registers a handler for SQS events
func (a *App) SQS(pattern string, handler any) error {
	h, err := a.convertEventHandler(handler)
	if err != nil {
		return fmt.Errorf("invalid SQS handler: %w", err)
	}
	a.eventRouter.AddEventRoute(TriggerSQS, pattern, h)
	return nil
}

// S3 registers a handler for S3 events
func (a *App) S3(pattern string, handler any) error {
	h, err := a.convertEventHandler(handler)
	if err != nil {
		return fmt.Errorf("invalid S3 handler: %w", err)
	}
	a.eventRouter.AddEventRoute(TriggerS3, pattern, h)
	return nil
}

// EventBridge registers a handler for EventBridge events
func (a *App) EventBridge(pattern string, handler any) error {
	h, err := a.convertEventHandler(handler)
	if err != nil {
		return fmt.Errorf("invalid EventBridge handler: %w", err)
	}
	a.eventRouter.AddEventRoute(TriggerEventBridge, pattern, h)
	return nil
}

// convertEventHandler converts various handler types to EventHandler
func (a *App) convertEventHandler(handler any) (EventHandler, error) {
	// Check if it's already an EventHandler
	if eh, ok := handler.(EventHandler); ok {
		return eh, nil
	}
	
	// Check if it's an EventHandlerFunc
	if ehf, ok := handler.(func(*Context) error); ok {
		return EventHandlerFunc(ehf), nil
	}
	
	// Try to convert as HTTP handler and wrap it
	httpHandler, err := convertHandlerUsingReflection(handler)
	if err != nil {
		return nil, err
	}
	
	// Wrap HTTP handler as EventHandler
	return EventHandlerFunc(func(ctx *Context) error {
		return httpHandler.Handle(ctx)
	}), nil
}

// convertHandlerUsingReflection converts various handler function types to the Handler interface using reflection
func convertHandlerUsingReflection(handler any) (Handler, error) {
	v := reflect.ValueOf(handler)
	t := reflect.TypeOf(handler)

	// Ensure handler is a function
	if t.Kind() != reflect.Func {
		return nil, fmt.Errorf("handler must be a function, got %T", handler)
	}

	// Validate handler function signature at registration time for security
	if err := validateHandlerSignature(t); err != nil {
		return nil, err
	}

	// Convert to our Handler interface based on the function signature
	return createReflectedHandler(v, t), nil
}

// validateHandlerSignature validates that the handler function has a supported signature
func validateHandlerSignature(t reflect.Type) error {
	numIn := t.NumIn()
	numOut := t.NumOut()

	// Pattern 1: func(*Context) error
	if numIn == 1 && numOut == 1 {
		// Already handled in the switch statement, but included for completeness
		if isContextType(t.In(0)) && isErrorType(t.Out(0)) {
			return nil
		}
	}

	// Pattern 2: func(*Context) (any, error)
	if numIn == 1 && numOut == 2 {
		if isContextType(t.In(0)) && isInterfaceType(t.Out(0)) && isErrorType(t.Out(1)) {
			return nil
		}
	}

	// Pattern 3: func() error (no context - simple handlers)
	if numIn == 0 && numOut == 1 {
		if isErrorType(t.Out(0)) {
			return nil
		}
	}

	// Pattern 4: func() (any, error) (no context - simple handlers with return value)
	if numIn == 0 && numOut == 2 {
		if isInterfaceType(t.Out(0)) && isErrorType(t.Out(1)) {
			return nil
		}
	}

	// Pattern 5: func(any) error (request model binding)
	if numIn == 1 && numOut == 1 {
		if !isContextType(t.In(0)) && isErrorType(t.Out(0)) {
			return nil
		}
	}

	// Pattern 6: func(any) (any, error) (request/response model binding)
	if numIn == 1 && numOut == 2 {
		if !isContextType(t.In(0)) && isInterfaceType(t.Out(0)) && isErrorType(t.Out(1)) {
			return nil
		}
	}

	return fmt.Errorf("unsupported handler signature: %s", t.String())
}

// createReflectedHandler creates a Handler from a reflected function
func createReflectedHandler(v reflect.Value, t reflect.Type) Handler {
	return HandlerFunc(func(ctx *Context) error {
		// Determine the handler pattern and call appropriately
		numIn := t.NumIn()
		numOut := t.NumOut()

		var callArgs []reflect.Value
		var results []reflect.Value

		switch {
		// Pattern 1: func(*Context) error - already handled by main switch but included here
		case numIn == 1 && numOut == 1 && isContextType(t.In(0)):
			callArgs = []reflect.Value{reflect.ValueOf(ctx)}

		// Pattern 2: func(*Context) (any, error)
		case numIn == 1 && numOut == 2 && isContextType(t.In(0)):
			callArgs = []reflect.Value{reflect.ValueOf(ctx)}

		// Pattern 3: func() error
		case numIn == 0 && numOut == 1:
			callArgs = []reflect.Value{}

		// Pattern 4: func() (any, error)
		case numIn == 0 && numOut == 2:
			callArgs = []reflect.Value{}

		// Pattern 5: func(RequestModel) error
		case numIn == 1 && numOut == 1 && !isContextType(t.In(0)):
			// Create instance of the expected input type
			requestType := t.In(0)
			requestValue := reflect.New(requestType).Interface()

			// Parse request body into the model
			if err := ctx.ParseRequest(requestValue); err != nil {
				return err
			}

			callArgs = []reflect.Value{reflect.ValueOf(requestValue).Elem()}

		// Pattern 6: func(RequestModel) (ResponseModel, error)
		case numIn == 1 && numOut == 2 && !isContextType(t.In(0)):
			// Create instance of the expected input type
			requestType := t.In(0)
			requestValue := reflect.New(requestType).Interface()

			// Parse request body into the model
			if err := ctx.ParseRequest(requestValue); err != nil {
				return err
			}

			callArgs = []reflect.Value{reflect.ValueOf(requestValue).Elem()}

		default:
			return fmt.Errorf("unsupported handler pattern during execution")
		}

		// Call the handler function
		results = v.Call(callArgs)

		// Handle return values
		switch len(results) {
		case 1:
			// Only error return
			if !results[0].IsNil() {
				return results[0].Interface().(error)
			}
			return nil

		case 2:
			// (value, error) return
			errValue := results[1]
			if !errValue.IsNil() {
				return errValue.Interface().(error)
			}

			// Send the response value as JSON
			responseValue := results[0].Interface()
			return ctx.JSON(responseValue)

		default:
			return fmt.Errorf("unexpected number of return values: %d", len(results))
		}
	})
}

// Helper functions for type checking

func isContextType(t reflect.Type) bool {
	// Check if it's a pointer to Context
	if t.Kind() != reflect.Ptr {
		return false
	}
	elem := t.Elem()
	return elem.Name() == "Context" && elem.PkgPath() == "github.com/pay-theory/lift/pkg/lift"
}

func isErrorType(t reflect.Type) bool {
	// Check if it implements the error interface
	errorInterface := reflect.TypeOf((*error)(nil)).Elem()
	return t.Implements(errorInterface)
}

func isInterfaceType(t reflect.Type) bool {
	// Accept any type for response values (any)
	return true
}

// parseTriggerType converts a string to a TriggerType
func parseTriggerType(s string) TriggerType {
	switch s {
	case "SQS":
		return TriggerSQS
	case "S3":
		return TriggerS3
	case "EventBridge":
		return TriggerEventBridge
	case "CONNECT", "DISCONNECT", "MESSAGE":
		return TriggerWebSocket
	default:
		// Check if it's an HTTP method
		if s == "GET" || s == "POST" || s == "PUT" || s == "DELETE" || s == "PATCH" || s == "HEAD" || s == "OPTIONS" {
			return TriggerAPIGateway
		}
		return TriggerUnknown
	}
}
