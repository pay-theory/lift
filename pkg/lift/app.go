package lift

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/pay-theory/lift/pkg/lift/adapters"
)

// Config represents the application configuration.
// It includes settings for logging, CORS, timeouts, and other application-wide configurations.
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

// DefaultConfig returns a configuration with sensible defaults.
// It provides a baseline configuration that can be customized further.
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

// AppOption is a function that configures an App.
// It allows for functional configuration of the App struct.
type AppOption func(*App)

type middlewareEntry struct {
	handler         Middleware
	appliesToEvents bool
}

// App represents the main application container.
// It is the central structure for configuring and running a Lift application.
// The App struct holds the configuration, middleware, routes, and other application-wide settings.
// App represents a Lift application. It is the main entry point for creating
// and configuring a serverless application. The App struct holds the
// configuration, middleware, routes, and other application-wide settings.
//
// Fields:
//   - logger: The logger used by the application
//   - metrics: The metrics collector used by the application
//   - db: A generic database connection
//   - router: The router used to match incoming requests to handlers
//   - eventRouter: The event router used to match incoming events to handlers
//   - config: The application configuration
//   - adapterRegistry: The registry of event adapters
//   - wsOptions: WebSocket options
//   - wsRoutes: WebSocket routes
//   - middleware: The middleware stack
//   - preferredAdapters: The preferred event adapters
//   - features: Feature flags
//   - mu: A mutex for synchronizing access to the App
//   - started: Whether the application has been started
//   - hasInterceptingMiddleware: Whether the application has intercepting middleware
type App struct { //nolint:govet // fieldalignment: keep readable order; negligible impact
	// Interfaces (16 bytes) first
	logger  Logger
	metrics MetricsCollector
	tracer  any
	db      any

	// Pointers (8 bytes)
	router          *Router
	eventRouter     *EventRouter
	config          *Config
	adapterRegistry *adapters.AdapterRegistry
	wsOptions       *WebSocketOptions
	lifecycleCtx    context.Context
	lifecycleCancel context.CancelFunc

	// Maps/slices (24 bytes)
	wsRoutes          map[string]WebSocketHandler
	eventBusRoutes    []*eventBusRoute
	middleware        []middlewareEntry
	preferredAdapters []adapters.TriggerType
	features          map[string]bool
	shutdownHooks     []func()

	metricsFactory func(*Config) MetricsCollector
	tracerFactory  func(*Config) any

	// Sync primitives and small scalars last
	mu                        sync.RWMutex
	started                   bool
	hasInterceptingMiddleware bool
}

// New creates a new Lift application with the given options.
// The options are applied in the order they are provided.
//
// Parameters:
//   - options: A variadic list of AppOption functions to configure the application
//
// Returns:
//   - A pointer to the newly created App
//
// Example:
//
//	app := lift.New(
//	    lift.WithConfig(&lift.Config{LogLevel: "DEBUG"}),
//	    lift.WithLogger(myLogger),
//	)
//
// New creates a new Lift application with the given options. The options are
// applied in the order they are provided.
//
// Parameters:
//   - options: A variadic list of AppOption functions to configure the application
//
// Returns:
//   - A pointer to the newly created App
//
// Example:
//
//	app := lift.New(
//	    lift.WithConfig(&lift.Config{LogLevel: "DEBUG"}),
//	    lift.WithLogger(myLogger),
//	)
func New(options ...AppOption) *App {
	app := &App{
		router:          NewRouter(),
		eventRouter:     NewEventRouter(),
		middleware:      make([]middlewareEntry, 0),
		config:          DefaultConfig(),
		adapterRegistry: adapters.NewAdapterRegistry(),
		features:        make(map[string]bool),
		started:         false,
	}

	// Apply options
	for _, opt := range options {
		opt(app)
	}

	app.ensureLifecycleContextLocked()

	return app
}

func (a *App) ensureLifecycleContextLocked() {
	if a.lifecycleCtx == nil || a.lifecycleCancel == nil {
		a.lifecycleCtx, a.lifecycleCancel = context.WithCancel(context.Background())
	}
}

// LifecycleContext returns a process-scoped context that is canceled when the
// application stops. Middleware can use it to manage background goroutines.
func (a *App) LifecycleContext() context.Context {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.ensureLifecycleContextLocked()
	return a.lifecycleCtx
}

// RegisterShutdownHook registers a cleanup hook that executes when the
// application stops. Hooks run in LIFO order and panics are recovered to avoid
// interrupting other cleanup tasks.
func (a *App) RegisterShutdownHook(hook func()) {
	if hook == nil {
		return
	}

	a.mu.Lock()
	a.shutdownHooks = append(a.shutdownHooks, hook)
	a.mu.Unlock()
}

// Stop cancels the lifecycle context and executes registered shutdown hooks.
// It is safe to call multiple times; subsequent calls become no-ops.
func (a *App) Stop() {
	a.mu.Lock()
	if a.lifecycleCancel == nil && len(a.shutdownHooks) == 0 {
		a.mu.Unlock()
		return
	}

	cancel := a.lifecycleCancel
	a.lifecycleCancel = nil
	a.lifecycleCtx = nil
	hooks := make([]func(), len(a.shutdownHooks))
	copy(hooks, a.shutdownHooks)
	a.shutdownHooks = nil
	a.started = false
	a.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	for i := len(hooks) - 1; i >= 0; i-- {
		hook := hooks[i]
		if hook == nil {
			continue
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					_ = r // ignore panic to allow other shutdown hooks to complete
				}
			}()
			hook()
		}()
	}
}

// Use adds middleware to the application.
// Middleware is executed in the order it is added (last added runs closest to the handler).
// By default middleware only runs for HTTP/WebSocket requests; global middleware such as
// logging/metrics can opt-in to non-HTTP triggers via lift.MarkGlobalMiddleware.
func (a *App) Use(mw func(Handler) Handler) *App {
	a.addMiddleware(Middleware(mw), false)
	return a
}

type eventScopedMiddleware interface {
	AppliesToEvents() bool
}

func (a *App) addMiddleware(mw Middleware, appliesToEvents bool) {
	if mw == nil {
		return
	}

	if scoped, ok := any(mw).(eventScopedMiddleware); ok {
		appliesToEvents = scoped.AppliesToEvents()
	}

	if !appliesToEvents {
		appliesToEvents = middlewareAppliesToEvents(mw)
	}

	if interceptor, ok := any(mw).(ResponseInterceptor); ok && interceptor.NeedsResponseInterception() {
		a.hasInterceptingMiddleware = true
	}

	a.middleware = append(a.middleware, middlewareEntry{
		handler:         mw,
		appliesToEvents: appliesToEvents,
	})
}

func (a *App) httpMiddlewareChain() []Middleware {
	chain := make([]Middleware, 0, len(a.middleware))
	for _, entry := range a.middleware {
		chain = append(chain, entry.handler)
	}
	return chain
}

func (a *App) eventMiddlewareChain() []Middleware {
	chain := make([]Middleware, 0, len(a.middleware))
	for _, entry := range a.middleware {
		if !entry.appliesToEvents {
			continue
		}
		chain = append(chain, entry.handler)
	}
	return chain
}

func (a *App) ensureTelemetry() {
	if !a.config.MetricsEnabled {
		a.metrics = nil
	} else if a.metrics == nil && a.metricsFactory != nil {
		a.metrics = a.metricsFactory(a.config)
	}

	if !a.config.TracingEnabled {
		a.tracer = nil
	} else if a.tracer == nil && a.tracerFactory != nil {
		a.tracer = a.tracerFactory(a.config)
	}
}

// GET registers a GET route.
//
// Parameters:
//   - path: The URL path for the route
//   - handler: The handler function for the route
//
// Returns:
//   - An error if the handler type is unsupported
func (a *App) GET(path string, handler any) error {
	return a.Handle("GET", path, handler)
}

// POST registers a POST route.
//
// Parameters:
//   - path: The URL path for the route
//   - handler: The handler function for the route
//
// Returns:
//   - An error if the handler type is unsupported
func (a *App) POST(path string, handler any) error {
	return a.Handle("POST", path, handler)
}

// PUT registers a PUT route.
//
// Parameters:
//   - path: The URL path for the route
//   - handler: The handler function for the route
//
// Returns:
//   - An error if the handler type is unsupported
func (a *App) PUT(path string, handler any) error {
	return a.Handle("PUT", path, handler)
}

// DELETE registers a DELETE route.
//
// Parameters:
//   - path: The URL path for the route
//   - handler: The handler function for the route
//
// Returns:
//   - An error if the handler type is unsupported
func (a *App) DELETE(path string, handler any) error {
	return a.Handle("DELETE", path, handler)
}

// PATCH registers a PATCH route.
//
// Parameters:
//   - path: The URL path for the route
//   - handler: The handler function for the route
//
// Returns:
//   - An error if the handler type is unsupported
func (a *App) PATCH(path string, handler any) error {
	return a.Handle("PATCH", path, handler)
}

// Handle registers a route with the specified method and path.
//
// Parameters:
//   - method: The HTTP method for the route
//   - path: The URL path for the route
//   - handler: The handler function for the route
//
// Returns:
//   - An error if the handler type is unsupported
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

// Group creates a new route group with the specified prefix.
//
// Parameters:
//   - prefix: The URL prefix for the route group
//
// Returns:
//   - A pointer to the RouteGroup
func (a *App) Group(prefix string) *RouteGroup {
	return &RouteGroup{
		app:    a,
		prefix: prefix,
	}
}

// RouteGroup represents a group of routes with a common prefix.
// It allows for organizing routes under a common URL prefix.
type RouteGroup struct {
	app    *App
	prefix string
}

// GET registers a GET route in this group.
//
// Parameters:
//   - path: The URL path for the route
//   - handler: The handler function for the route
//
// Returns:
//   - An error if the handler type is unsupported
func (rg *RouteGroup) GET(path string, handler any) error {
	return rg.app.GET(rg.prefix+path, handler)
}

// POST registers a POST route in this group.
//
// Parameters:
//   - path: The URL path for the route
//   - handler: The handler function for the route
//
// Returns:
//   - An error if the handler type is unsupported
func (rg *RouteGroup) POST(path string, handler any) error {
	return rg.app.POST(rg.prefix+path, handler)
}

// PUT registers a PUT route in this group.
//
// Parameters:
//   - path: The URL path for the route
//   - handler: The handler function for the route
//
// Returns:
//   - An error if the handler type is unsupported
func (rg *RouteGroup) PUT(path string, handler any) error {
	return rg.app.PUT(rg.prefix+path, handler)
}

// DELETE registers a DELETE route in this group.
//
// Parameters:
//   - path: The URL path for the route
//   - handler: The handler function for the route
//
// Returns:
//   - An error if the handler type is unsupported
func (rg *RouteGroup) DELETE(path string, handler any) error {
	return rg.app.DELETE(rg.prefix+path, handler)
}

// PATCH registers a PATCH route in this group.
//
// Parameters:
//   - path: The URL path for the route
//   - handler: The handler function for the route
//
// Returns:
//   - An error if the handler type is unsupported
func (rg *RouteGroup) PATCH(path string, handler any) error {
	return rg.app.PATCH(rg.prefix+path, handler)
}

// Group creates a sub-group with an additional prefix.
//
// Parameters:
//   - prefix: The additional URL prefix for the sub-group
//
// Returns:
//   - A pointer to the RouteGroup
func (rg *RouteGroup) Group(prefix string) *RouteGroup {
	return &RouteGroup{
		app:    rg.app,
		prefix: rg.prefix + prefix,
	}
}

// Use attaches middleware to this route group. Delegates to app-level chain.
//
// Parameters:
//   - mw: The middleware function
//
// Returns:
//   - A pointer to the RouteGroup
func (rg *RouteGroup) Use(mw func(Handler) Handler) *RouteGroup {
	rg.app.Use(mw)
	return rg
}

// Start prepares the application for handling requests.
//
// Returns:
//   - An error if the application is already started
func (a *App) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.started {
		return nil
	}

	a.ensureTelemetry()
	a.ensureLifecycleContextLocked()

	// Apply global middleware to router
	a.router.SetMiddleware(a.httpMiddlewareChain())

	a.started = true
	return nil
}

// IsLambda returns true if the code is running in an AWS Lambda environment.
//
// Returns:
//   - A boolean indicating if the code is running in AWS Lambda
func (a *App) IsLambda() bool {
	// Check for AWS Lambda-specific environment variables
	// AWS_LAMBDA_FUNCTION_NAME is set in all Lambda runtime environments
	// LAMBDA_TASK_ROOT is the path to the Lambda function code
	// AWS_EXECUTION_ENV contains the runtime identifier (e.g., AWS_Lambda_go1.x)
	return os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" ||
		os.Getenv("LAMBDA_TASK_ROOT") != "" ||
		os.Getenv("AWS_EXECUTION_ENV") != ""
}

// HandleRequest processes an incoming Lambda request.
//
// Parameters:
//   - ctx: The context for the request
//   - event: The Lambda event
//
// Returns:
//   - The response from the handler
//   - An error if the request handling fails
func (a *App) HandleRequest(ctx context.Context, event any) (any, error) {
	builder := newRequestHandlerBuilder(ctx, a, event)
	return builder.build()
}

// GetEventRouter returns the EventRouter for accessing event routes (mainly for testing).
//
// Returns:
//   - The EventRouter
func (a *App) GetEventRouter() *EventRouter {
	return a.eventRouter
}

// SQS registers a handler for SQS events.
//
// Parameters:
//   - pattern: The pattern for the SQS event
//   - handler: The handler function for the event
//
// Returns:
//   - An error if the handler type is unsupported
func (a *App) SQS(pattern string, handler any) error {
	h, err := a.convertEventHandler(handler)
	if err != nil {
		return fmt.Errorf("invalid SQS handler: %w", err)
	}
	a.eventRouter.AddEventRoute(TriggerSQS, pattern, h)
	return nil
}

// S3 registers a handler for S3 events.
//
// Parameters:
//   - pattern: The pattern for the S3 event
//   - handler: The handler function for the event
//
// Returns:
//   - An error if the handler type is unsupported
func (a *App) S3(pattern string, handler any) error {
	h, err := a.convertEventHandler(handler)
	if err != nil {
		return fmt.Errorf("invalid S3 handler: %w", err)
	}
	a.eventRouter.AddEventRoute(TriggerS3, pattern, h)
	return nil
}

// EventBridge registers a handler for EventBridge events.
//
// Parameters:
//   - pattern: The pattern for the EventBridge event
//   - handler: The handler function for the event
//
// Returns:
//   - An error if the handler type is unsupported
func (a *App) EventBridge(pattern string, handler any) error {
	h, err := a.convertEventHandler(handler)
	if err != nil {
		return fmt.Errorf("invalid EventBridge handler: %w", err)
	}
	a.eventRouter.AddEventRoute(TriggerEventBridge, pattern, h)
	return nil
}

// DynamoDB registers a handler for DynamoDB stream events.
//
// Parameters:
//   - pattern: The pattern for the DynamoDB event
//   - handler: The handler function for the event
//
// Returns:
//   - An error if the handler type is unsupported
func (a *App) DynamoDB(pattern string, handler any) error {
	h, err := a.convertEventHandler(handler)
	if err != nil {
		return fmt.Errorf("invalid DynamoDB handler: %w", err)
	}
	// DynamoDB stream events are currently adapted via the EventBus adapter.
	a.eventRouter.AddEventRoute(TriggerEventBus, pattern, h)
	return nil
}

// convertEventHandler converts various handler types to EventHandler.
//
// Parameters:
//   - handler: The handler to convert
//
// Returns:
//   - The converted EventHandler
//   - An error if the conversion fails
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

// WithConfig sets the application configuration.
//
// Parameters:
//   - config: A pointer to the Config struct
//
// Returns:
//   - A pointer to the App
func (a *App) WithConfig(config *Config) *App {
	a.config = config
	return a
}

// WithLogger sets the application logger.
//
// Parameters:
//   - logger: A Logger instance
//
// Returns:
//   - A pointer to the App
func (a *App) WithLogger(logger Logger) *App {
	a.logger = logger
	return a
}

// WithMetrics sets the metrics collector.
//
// Parameters:
//   - metrics: A MetricsCollector instance
//
// Returns:
//   - A pointer to the App
func (a *App) WithMetrics(metrics MetricsCollector) *App {
	a.metrics = metrics
	return a
}

// WithMetricsFactory registers a factory used to lazily construct a metrics collector when metrics are enabled.
func (a *App) WithMetricsFactory(factory func(*Config) MetricsCollector) *App {
	a.metricsFactory = factory
	return a
}

// WithDatabase sets the database connection.
//
// Parameters:
//   - db: A database connection
//
// Returns:
//   - A pointer to the App
func (a *App) WithDatabase(db any) *App {
	a.db = db
	return a
}

// WithTracer sets the tracer implementation used by the application when tracing is enabled.
func (a *App) WithTracer(tracer any) *App {
	a.tracer = tracer
	return a
}

// WithTracerFactory registers a factory used to lazily construct a tracer when tracing is enabled.
func (a *App) WithTracerFactory(factory func(*Config) any) *App {
	a.tracerFactory = factory
	return a
}

// WithPreferredAdapters sets an ordered list of preferred event adapters for parsing Lambda events.
// If set, the app will try these adapters (in order) before falling back to auto-detection.
//
// Parameters:
//   - order: A variadic list of TriggerType
//
// Returns:
//   - A pointer to the App
func (a *App) WithPreferredAdapters(order ...adapters.TriggerType) *App {
	a.preferredAdapters = append([]adapters.TriggerType(nil), order...)
	return a
}

// WithConfig sets a custom configuration for the application.
//
// Parameters:
//   - config: A pointer to the Config struct
//
// Returns:
//   - An AppOption function that sets the configuration
func WithConfig(config *Config) AppOption {
	return func(app *App) {
		app.config = config
	}
}

// WithDebug enables debug mode for the application.
//
// Returns:
//   - An AppOption function that enables debug mode
func WithDebug() AppOption {
	return func(app *App) {
		app.config.Debug = true
		// Also set log level to DEBUG if using the default logger
		if app.config.LogLevel == "INFO" {
			app.config.LogLevel = "DEBUG"
		}
	}
}

// WithLogger sets the application logger.
//
// Parameters:
//   - logger: A Logger instance
//
// Returns:
//   - An AppOption function that sets the logger
func WithLogger(logger Logger) AppOption {
	return func(app *App) {
		app.logger = logger
	}
}

// WithMetrics sets the metrics collector.
//
// Parameters:
//   - metrics: A MetricsCollector instance
//
// Returns:
//   - An AppOption function that sets the metrics collector
func WithMetrics(metrics MetricsCollector) AppOption {
	return func(app *App) {
		app.metrics = metrics
	}
}

// WithMetricsFactory registers a factory used to lazily construct a metrics collector when metrics are enabled.
//
// Parameters:
//   - factory: A function that takes a Config and returns a MetricsCollector
//
// Returns:
//   - An AppOption function that sets the metrics factory
func WithMetricsFactory(factory func(*Config) MetricsCollector) AppOption {
	return func(app *App) {
		app.metricsFactory = factory
	}
}

// WithDatabase sets the database connection.
//
// Parameters:
//   - db: A database connection
//
// Returns:
//   - An AppOption function that sets the database
func WithDatabase(db any) AppOption {
	return func(app *App) {
		app.db = db
	}
}

// WithTracer sets the tracer implementation used by the application when tracing is enabled.
//
// Parameters:
//   - tracer: A tracer implementation
//
// Returns:
//   - An AppOption function that sets the tracer
func WithTracer(tracer any) AppOption {
	return func(app *App) {
		app.tracer = tracer
	}
}

// WithTracerFactory registers a factory used to lazily construct a tracer when tracing is enabled.
//
// Parameters:
//   - factory: A function that takes a Config and returns a tracer
//
// Returns:
//   - An AppOption function that sets the tracer factory
func WithTracerFactory(factory func(*Config) any) AppOption {
	return func(app *App) {
		app.tracerFactory = factory
	}
}

// WithPreferredAdapters sets an ordered list of preferred event adapters for parsing Lambda events.
// If set, the app will try these adapters (in order) before falling back to auto-detection.
//
// Parameters:
//   - order: A variadic list of TriggerType
//
// Returns:
//   - An AppOption function that sets the preferred adapters
func WithPreferredAdapters(order ...adapters.TriggerType) AppOption {
	return func(app *App) {
		app.preferredAdapters = append([]adapters.TriggerType(nil), order...)
	}
}
