package lift

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/pay-theory/lift/pkg/lift/adapters"
)

// Config represents the application configuration
type Config struct {
	LogLevel        string   `json:"log_level"`
	AllowedOrigins  []string `json:"allowed_origins"`
	MaxRequestSize  int64    `json:"max_request_size"`
	MaxResponseSize int64    `json:"max_response_size"`
	Timeout         int      `json:"timeout_seconds"`
	MetricsEnabled  bool     `json:"metrics_enabled"`
	TracingEnabled  bool     `json:"tracing_enabled"`
	Debug           bool     `json:"debug"`
	CORSEnabled     bool     `json:"cors_enabled"`
	RequireTenantID bool     `json:"require_tenant_id"`
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
		Debug:           false,
		CORSEnabled:     true,
		AllowedOrigins:  []string{"*"},
		RequireTenantID: false,
	}
}

// AppOption is a function that configures an App
type AppOption func(*App)

// App represents the main application container
type App struct { //nolint:govet // fieldalignment: keep readable order; negligible impact
    // Interfaces (16 bytes) first
    logger            Logger
    metrics           MetricsCollector
    db                any

    // Pointers (8 bytes)
    router            *Router
    eventRouter       *EventRouter
    config            *Config
    adapterRegistry   *adapters.AdapterRegistry
    wsOptions         *WebSocketOptions

    // Maps/slices (24 bytes)
    wsRoutes          map[string]WebSocketHandler
    middleware        []Middleware
    preferredAdapters []adapters.TriggerType
    features          map[string]bool

    // Sync primitives and small scalars last
    mu                        sync.RWMutex
    started                   bool
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

// Use adds middleware to the application. Middleware is executed in the order
// it is added (last added runs closest to the handler), and applies to all
// routes (HTTP and non‑HTTP) handled by this App.
func (a *App) Use(mw func(Handler) Handler) *App {
	// Accept generic middleware func type for better interop across packages
	a.middleware = append(a.middleware, Middleware(mw))

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

// WithPreferredAdapters sets an ordered list of preferred event adapters for parsing Lambda events.
// If set, the app will try these adapters (in order) before falling back to auto-detection.
func (a *App) WithPreferredAdapters(order ...adapters.TriggerType) *App {
    a.preferredAdapters = append([]adapters.TriggerType(nil), order...)
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

// Use attaches middleware to this route group. Delegates to app-level chain.
func (rg *RouteGroup) Use(mw func(Handler) Handler) *RouteGroup {
    rg.app.Use(mw)
    return rg
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
    builder := newRequestHandlerBuilder(ctx, a, event)
	return builder.build()
}

// requestHandlerBuilder builds and executes Lambda request handling
type requestHandlerBuilder struct {
	app      *App
	ctx      context.Context
	event    any
	liftCtx  *Context
	request  *Request
	routeErr error
}

// newRequestHandlerBuilder creates a new request handler builder
func newRequestHandlerBuilder(ctx context.Context, app *App, event any) *requestHandlerBuilder {
    return &requestHandlerBuilder{
        app:   app,
        ctx:   ctx,
        event: event,
    }
}

// build executes the complete request handling pipeline
func (b *requestHandlerBuilder) build() (any, error) {
	if err := b.ensureAppStarted(); err != nil {
		return nil, err
	}
	
	if err := b.parseEvent(); err != nil {
		return nil, err
	}
	
	b.createContext()
	b.configureContext()
	b.routeRequest()
	
	if b.routeErr != nil {
		return b.app.handleError(b.liftCtx, b.routeErr)
	}
	
	if err := b.liftCtx.FlushResponse(); err != nil {
		return nil, err
	}
	
	return b.liftCtx.Response, nil
}

// ensureAppStarted ensures the app is properly initialized
func (b *requestHandlerBuilder) ensureAppStarted() error {
	return b.app.Start()
}

// parseEvent converts the Lambda event to a Request
func (b *requestHandlerBuilder) parseEvent() error {
	req, err := b.app.parseEvent(b.event)
	if err != nil {
		return err
	}
	b.request = req
	return nil
}

// createContext creates the enhanced Lift context
func (b *requestHandlerBuilder) createContext() {
	b.liftCtx = NewContext(b.ctx, b.request)
}

// configureContext sets up the context with app dependencies
func (b *requestHandlerBuilder) configureContext() {
	if b.app.hasInterceptingMiddleware {
		b.liftCtx.EnableResponseBuffering()
	}
	
	if b.app.logger != nil {
		b.liftCtx.Logger = b.app.logger
	}
	if b.app.metrics != nil {
		b.liftCtx.Metrics = b.app.metrics
	}
	if b.app.db != nil {
		b.liftCtx.DB = b.app.db
	}
}

// routeRequest routes the request based on trigger type
func (b *requestHandlerBuilder) routeRequest() {
	switch {
	case b.request.TriggerType == adapters.TriggerWebSocket:
		b.routeWebSocket()
	case b.isEventTrigger():
		b.routeEvent()
	default:
		b.routeHTTP()
	}
}

// isEventTrigger checks if this is a non-HTTP event trigger
func (b *requestHandlerBuilder) isEventTrigger() bool {
	return b.request.TriggerType != adapters.TriggerAPIGateway && 
		b.request.TriggerType != adapters.TriggerAPIGatewayV2 && 
		b.request.TriggerType != adapters.TriggerUnknown
}

// routeWebSocket handles WebSocket routing
func (b *requestHandlerBuilder) routeWebSocket() {
	routeKey := b.extractRouteKey()
	handler := b.app.RouteWebSocket(routeKey)
	
	if handler == nil {
		b.routeErr = NewLiftError("WEBSOCKET_ROUTE_NOT_FOUND", 
			fmt.Sprintf("No handler for WebSocket route: %s", routeKey), 404)
		return
	}
	
	finalHandler := b.applyMiddleware(handler)
	finalHandler = b.applyConnectionManagement(finalHandler)
	
	if err := finalHandler.Handle(b.liftCtx); err != nil {
		b.routeErr = err
	}
}

// extractRouteKey gets the WebSocket route key from metadata
func (b *requestHandlerBuilder) extractRouteKey() string {
	if metadata, ok := b.request.Metadata["routeKey"].(string); ok {
		return metadata
	}
	return ""
}

// applyMiddleware wraps the handler with middleware
func (b *requestHandlerBuilder) applyMiddleware(handler Handler) Handler {
	finalHandler := handler
	for i := len(b.app.middleware) - 1; i >= 0; i-- {
		finalHandler = b.app.middleware[i](finalHandler)
	}
	return finalHandler
}

// applyConnectionManagement adds WebSocket connection management if enabled
func (b *requestHandlerBuilder) applyConnectionManagement(handler Handler) Handler {
	if b.app.wsOptions != nil && b.app.wsOptions.EnableAutoConnectionManagement {
		return wrapWithConnectionManagement(handler, b.app.wsOptions.ConnectionStore)
	}
	return handler
}

// routeEvent handles non-HTTP event routing
func (b *requestHandlerBuilder) routeEvent() {
	if err := b.app.eventRouter.HandleEvent(b.liftCtx); err != nil {
		b.routeErr = err
	}
}

// routeHTTP handles HTTP request routing
func (b *requestHandlerBuilder) routeHTTP() {
	if err := b.app.router.Handle(b.liftCtx); err != nil {
		b.routeErr = err
	}
}

// parseEvent converts a Lambda event to our Request structure
func (a *App) parseEvent(event any) (*Request, error) {
    a.logEventDebug(event)

    if req, ok := a.tryPreferredAdapters(event); ok {
        return req, nil
    }

    return a.detectAndAdaptEvent(event)
}

// logEventDebug logs useful debug information about the incoming event
func (a *App) logEventDebug(event any) {
    if !a.config.Debug || a.logger == nil {
        return
    }
    // Avoid logging user-controlled content; just record that an event arrived.
    a.logger.Debug("Parsing Lambda event")
    if eventMap, ok := event.(map[string]any); ok {
        a.logger.WithField("field_count", len(eventMap)).Debug("Event fields detected")
    }
}

// tryPreferredAdapters attempts to parse using preferred adapters first
func (a *App) tryPreferredAdapters(event any) (*Request, bool) {
    if len(a.preferredAdapters) == 0 {
        return nil, false
    }
    for _, tt := range a.preferredAdapters {
        adapter, ok := a.adapterRegistry.GetAdapter(tt)
        if !ok || !adapter.CanHandle(event) {
            continue
        }
        if req, err := adapter.Adapt(event); err == nil {
            // Do not log method/path which can be user-controlled.
            return NewRequest(req), true
        }
    }
    return nil, false
}

// detectAndAdaptEvent uses the registry to detect and adapt events
func (a *App) detectAndAdaptEvent(event any) (*Request, error) {
    adapterRequest, err := a.adapterRegistry.DetectAndAdapt(event)
    if err != nil {
        // Avoid echoing error text that may include user input.
        if a.config.Debug && a.logger != nil {
            a.logger.Error("Failed to parse Lambda event")
        }
        return nil, err
    }
    // Do not log method/path; success is enough for debug tracing.
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

		if err := ctx.Status(liftErr.StatusCode).JSON(resp); err != nil {
			// Log error but continue - we're already in error handling
			return nil, fmt.Errorf("failed to send error response: %w", err)
		}
		return ctx.Response, nil
	}

	// For non-Lift errors, set 500 status
	if err := ctx.Status(500).JSON(map[string]string{
		"error": "Internal server error",
	}); err != nil {
		return nil, fmt.Errorf("failed to send internal server error response: %w", err)
	}

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
			if jsonErr := ctx.Status(liftErr.StatusCode).JSON(map[string]any{
				"error":   liftErr.Code,
				"message": liftErr.Message,
			}); jsonErr != nil {
				return fmt.Errorf("failed to send error response: %w", jsonErr)
			}
			return nil // Don't return error, status is set in response
		}

		// For non-Lift errors, set 500 status
		if jsonErr := ctx.Status(500).JSON(map[string]any{
			"error":   "Internal Server Error",
			"message": err.Error(),
		}); jsonErr != nil {
			return fmt.Errorf("failed to send internal server error response: %w", jsonErr)
		}
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
// handlerPattern represents the different handler signature patterns supported
type handlerPattern int

const (
	patternContextError handlerPattern = iota // func(*Context) error
	patternContextResponse                    // func(*Context) (any, error)
	patternSimpleError                        // func() error
	patternSimpleResponse                     // func() (any, error)
	patternModelError                         // func(RequestModel) error
	patternModelResponse                      // func(RequestModel) (ResponseModel, error)
	patternUnsupported
)

// handlerValidator provides validation for handler signatures
type handlerValidator struct{}

// validateHandlerSignature validates that a handler function has a supported signature
func validateHandlerSignature(t reflect.Type) error {
	validator := &handlerValidator{}
	pattern := validator.identifyPattern(t)
	
	if pattern == patternUnsupported {
		return fmt.Errorf("unsupported handler signature: %s", t.String())
	}
	
	return nil
}

// identifyPattern determines which handler pattern a function type matches
func (v *handlerValidator) identifyPattern(t reflect.Type) handlerPattern {
	numIn := t.NumIn()
	numOut := t.NumOut()
	
	switch {
	case numIn == 1 && numOut == 1:
		return v.validateSingleInOut(t)
	case numIn == 1 && numOut == 2:
		return v.validateSingleInDoubleOut(t)
	case numIn == 0 && numOut == 1:
		return v.validateNoInSingleOut(t)
	case numIn == 0 && numOut == 2:
		return v.validateNoInDoubleOut(t)
	default:
		return patternUnsupported
	}
}

// validateSingleInOut validates patterns: func(*Context) error OR func(RequestModel) error
func (v *handlerValidator) validateSingleInOut(t reflect.Type) handlerPattern {
	if !isErrorType(t.Out(0)) {
		return patternUnsupported
	}
	
	if isContextType(t.In(0)) {
		return patternContextError
	}
	
	return patternModelError
}

// validateSingleInDoubleOut validates patterns: func(*Context) (any, error) OR func(RequestModel) (ResponseModel, error)
func (v *handlerValidator) validateSingleInDoubleOut(t reflect.Type) handlerPattern {
	if !isInterfaceType(t.Out(0)) || !isErrorType(t.Out(1)) {
		return patternUnsupported
	}
	
	if isContextType(t.In(0)) {
		return patternContextResponse
	}
	
	return patternModelResponse
}

// validateNoInSingleOut validates pattern: func() error
func (v *handlerValidator) validateNoInSingleOut(t reflect.Type) handlerPattern {
	if isErrorType(t.Out(0)) {
		return patternSimpleError
	}
	return patternUnsupported
}

// validateNoInDoubleOut validates pattern: func() (any, error)
func (v *handlerValidator) validateNoInDoubleOut(t reflect.Type) handlerPattern {
	if isInterfaceType(t.Out(0)) && isErrorType(t.Out(1)) {
		return patternSimpleResponse
	}
	return patternUnsupported
}

// handlerExecutor handles the execution of different handler patterns
// Memory optimized: struct with 48 pointer bytes could be 32
type handlerExecutor struct {
    // reflect.Type (16 bytes - interface)
    funcType reflect.Type
    // reflect.Value (24 bytes)
    value reflect.Value
    // enum (8 bytes on 64-bit)
    pattern handlerPattern
}

// createReflectedHandler creates a Handler from a reflected function
func createReflectedHandler(v reflect.Value, t reflect.Type) Handler {
	validator := &handlerValidator{}
	pattern := validator.identifyPattern(t)
	
	executor := &handlerExecutor{
		value:   v,
		pattern: pattern,
		funcType: t,
	}
	
	return HandlerFunc(func(ctx *Context) error {
		return executor.execute(ctx)
	})
}

// execute runs the handler function based on its identified pattern
func (e *handlerExecutor) execute(ctx *Context) error {
	callArgs, err := e.prepareArgs(ctx)
	if err != nil {
		return err
	}
	
	results := e.value.Call(callArgs)
	return e.handleResults(ctx, results)
}

// prepareArgs prepares the arguments for the function call based on the pattern
func (e *handlerExecutor) prepareArgs(ctx *Context) ([]reflect.Value, error) {
	switch e.pattern {
	case patternContextError, patternContextResponse:
		return []reflect.Value{reflect.ValueOf(ctx)}, nil
	
	case patternSimpleError, patternSimpleResponse:
		return []reflect.Value{}, nil
	
	case patternModelError, patternModelResponse:
		return e.prepareModelArgs(ctx)
	
	default:
		return nil, fmt.Errorf("unsupported handler pattern during execution")
	}
}

// prepareModelArgs prepares arguments for model-based handlers
func (e *handlerExecutor) prepareModelArgs(ctx *Context) ([]reflect.Value, error) {
	requestType := e.funcType.In(0)
	requestValue := reflect.New(requestType).Interface()
	
	if err := ctx.ParseRequest(requestValue); err != nil {
		return nil, err
	}
	
	return []reflect.Value{reflect.ValueOf(requestValue).Elem()}, nil
}

// handleResults processes the return values from the handler function
func (e *handlerExecutor) handleResults(ctx *Context, results []reflect.Value) error {
	switch len(results) {
	case 1:
		return e.handleSingleResult(results[0])
	case 2:
		return e.handleDoubleResult(ctx, results[0], results[1])
	default:
		return fmt.Errorf("unexpected number of return values: %d", len(results))
	}
}

// handleSingleResult handles functions that return only an error
func (e *handlerExecutor) handleSingleResult(result reflect.Value) error {
	if result.IsNil() {
		return nil
	}
	
	if err, ok := result.Interface().(error); ok {
		return err
	}
	
	return fmt.Errorf("handler returned non-error value: %v", result.Interface())
}

// handleDoubleResult handles functions that return (value, error)
func (e *handlerExecutor) handleDoubleResult(ctx *Context, valueResult, errorResult reflect.Value) error {
	if !errorResult.IsNil() {
		if err, ok := errorResult.Interface().(error); ok {
			return err
		}
		return fmt.Errorf("handler returned non-error value in error position: %v", errorResult.Interface())
	}
	
	responseValue := valueResult.Interface()
	if err := ctx.JSON(responseValue); err != nil {
		return fmt.Errorf("failed to send JSON response: %w", err)
	}
	return nil
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

func isInterfaceType(_ reflect.Type) bool {
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

// RunLocalTest runs local testing logic when not in Lambda environment
func (a *App) RunLocalTest() {
	if a.IsLambda() {
		return
	}

	// Load test event from file
	testFile := os.Getenv("TEST_FILE")
	if testFile == "" {
		if a.logger != nil {
			a.logger.Info("No test file defined", nil)
		}
		return
	}

	// Validate file path to prevent directory traversal
	testFile = filepath.Clean(testFile)
	if strings.Contains(testFile, "..") {
		if a.logger != nil {
			a.logger.Error("Invalid test file path", map[string]any{
				"path": testFile,
			})
		}
		return
	}

	eventData, err := os.ReadFile(testFile)
	if err != nil {
		if a.logger != nil {
			a.logger.Error("Error reading test event file", map[string]any{
				"error": err,
			})
		}
		return
	}

	// First unmarshal into a generic map to determine the event type
	var rawEvent map[string]any
	if err := json.Unmarshal(eventData, &rawEvent); err != nil {
		if a.logger != nil {
			a.logger.Error("Error unmarshaling test event", map[string]any{
				"error": err,
			})
		}
		return
	}

	// Create context with timeout for local testing
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// Run the test event locally
    if _, err := a.HandleRequest(ctx, rawEvent); err != nil {
        // Log generic debug message without echoing user input.
        if a.logger != nil {
            a.logger.Debug("Error handling request during local test")
        } else {
            fmt.Println("Debug: Error handling request during local test")
        }
    }
}

// WithDebug enables debug mode for the application
func WithDebug() AppOption {
	return func(app *App) {
		app.config.Debug = true
		// Also set log level to DEBUG if using the default logger
		if app.config.LogLevel == "INFO" {
			app.config.LogLevel = "DEBUG"
		}
	}
}

// WithConfig sets a custom configuration for the application
func WithConfig(config *Config) AppOption {
    return func(app *App) {
        app.config = config
    }
}
