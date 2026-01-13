package lift

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/events"

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

// requestHandlerBuilder builds and executes Lambda request handling.
// It is responsible for constructing the request handling pipeline.
type requestHandlerBuilder struct {
	app     *App
	ctx     context.Context
	event   any
	liftCtx *Context
	request *Request
	cancel  context.CancelFunc

	streamingResponse bool
}

// newRequestHandlerBuilder creates a new request handler builder.
//
// Parameters:
//   - ctx: The context for the request
//   - app: The App instance
//   - event: The Lambda event
//
// Returns:
//   - A pointer to the requestHandlerBuilder
func newRequestHandlerBuilder(ctx context.Context, app *App, event any) *requestHandlerBuilder {
	return &requestHandlerBuilder{
		app:   app,
		ctx:   ctx,
		event: event,
	}
}

// build executes the complete request handling pipeline.
//
// Returns:
//   - The response from the handler
//   - An error if the request handling fails
func (b *requestHandlerBuilder) build() (any, error) {
	if err := b.ensureAppStarted(); err != nil {
		return nil, err
	}

	if err := b.parseEvent(); err != nil {
		return nil, err
	}

	b.createContext()
	if err := b.applyRequestGuardrails(); err != nil {
		return b.app.handleError(b.liftCtx, err)
	}
	b.configureContext()
	defer b.cancelTimeoutUnlessStreaming()

	if err := b.routeRequest(); err != nil {
		b.abortStreaming(err)
		return b.app.handleError(b.liftCtx, err)
	}

	if err := b.enforceTenantRequirement(); err != nil {
		b.abortStreaming(err)
		return b.app.handleError(b.liftCtx, err)
	}

	if resp, ok := b.takeStreamingResponse(); ok {
		return resp, nil
	}

	if err := b.validateResponseGuardrails(); err != nil {
		return b.app.handleError(b.liftCtx, err)
	}

	if err := b.liftCtx.FlushResponse(); err != nil {
		return nil, err
	}

	// For AppSync Lambda resolvers, return the unwrapped body directly
	// AppSync expects the actual data, not the HTTP proxy response format
	if b.request.TriggerType == adapters.TriggerAppSync {
		return b.liftCtx.Response.Body, nil
	}

	// DynamoDB stream processors expect the raw batch response (e.g. BatchItemFailures),
	// not an API Gateway proxy response wrapper.
	if b.request.TriggerType == adapters.TriggerEventBus {
		return b.liftCtx.Response.Body, nil
	}

	// SQS batch processors expect the raw batch response (e.g. BatchItemFailures),
	// not an API Gateway proxy response wrapper, when ReportBatchItemFailures is enabled.
	if b.request.TriggerType == adapters.TriggerSQS {
		switch b.liftCtx.Response.Body.(type) {
		case events.SQSEventResponse, *events.SQSEventResponse:
			return b.liftCtx.Response.Body, nil
		}
	}

	return b.liftCtx.Response, nil
}

func (b *requestHandlerBuilder) cancelTimeoutUnlessStreaming() {
	if b.streamingResponse {
		return
	}
	b.cancelTimeout()
}

func (b *requestHandlerBuilder) abortStreaming(err error) {
	if b.liftCtx == nil {
		return
	}
	state := b.liftCtx.streamingState()
	if state == nil || state.abort == nil {
		return
	}
	state.abort(err)
	b.liftCtx.clearStreamingState()
}

func (b *requestHandlerBuilder) takeStreamingResponse() (any, bool) {
	if b.liftCtx == nil {
		return nil, false
	}

	state := b.liftCtx.streamingState()
	if state == nil || state.response == nil {
		return nil, false
	}

	if state.start != nil {
		state.start()
	}
	b.streamingResponse = true
	b.liftCtx.clearStreamingState()

	return state.response, true
}

// ensureAppStarted ensures the app is properly initialized.
//
// Returns:
//   - An error if the application fails to start
func (b *requestHandlerBuilder) ensureAppStarted() error {
	return b.app.Start()
}

// parseEvent converts the Lambda event to a Request.
//
// Returns:
//   - An error if the event parsing fails
func (b *requestHandlerBuilder) parseEvent() error {
	req, err := b.app.parseEvent(b.event)
	if err != nil {
		return err
	}
	b.request = req
	return nil
}

// createContext creates the enhanced Lift context.
// It initializes the context with the request and other dependencies.
func (b *requestHandlerBuilder) createContext() {
	// Set the Lambda context on the request for proper context propagation
	if b.request != nil {
		b.request.SetContext(b.ctx)
	}
	b.liftCtx = NewContext(b.ctx, b.request)
}

func (b *requestHandlerBuilder) cancelTimeout() {
	if b.cancel != nil {
		b.cancel()
	}
}

// configureContext sets up the context with app dependencies.
// It configures the context with the logger, metrics, and database.
func (b *requestHandlerBuilder) configureContext() {
	if timeoutSeconds := b.app.config.Timeout; timeoutSeconds > 0 {
		timeout := time.Duration(timeoutSeconds) * time.Second
		ctxWithTimeout, cancel := context.WithTimeout(b.ctx, timeout)
		b.liftCtx.Context = ctxWithTimeout
		b.cancel = cancel
	} else {
		b.liftCtx.Context = b.ctx
		b.cancel = nil
	}

	if b.cancel != nil {
		b.liftCtx.Set(requestCancelKey, b.cancel)
	}

	if b.app.hasInterceptingMiddleware {
		b.liftCtx.EnableResponseBuffering()
	}

	if b.app.logger != nil {
		b.liftCtx.Logger = b.app.logger
	}
	if b.app.metrics != nil {
		b.liftCtx.Metrics = b.app.metrics
	}
	if b.app.tracer != nil {
		b.liftCtx.SetTracer(b.app.tracer)
	}
	if b.app.db != nil {
		b.liftCtx.DB = b.app.db
	}
}

func (b *requestHandlerBuilder) applyRequestGuardrails() error {
	maxSize := b.app.config.MaxRequestSize
	if maxSize <= 0 || b.request == nil {
		return nil
	}

	if bodyLen := int64(len(b.request.Body)); bodyLen > 0 && bodyLen > maxSize {
		details := map[string]any{
			"size_bytes":   bodyLen,
			"limit_bytes":  maxSize,
			"trigger_type": string(b.request.TriggerType),
		}
		return b.guardrailViolation(ErrorCodePayloadTooLarge, 413, "Request body exceeds configured limit", details)
	}

	switch b.request.TriggerType {
	case adapters.TriggerSQS:
		for idx, record := range b.request.Records {
			recordSize := estimateSQSRecordSize(record)
			if recordSize > maxSize {
				details := map[string]any{
					"size_bytes":   recordSize,
					"limit_bytes":  maxSize,
					"record_index": idx,
					"trigger_type": string(b.request.TriggerType),
				}
				return b.guardrailViolation(ErrorCodePayloadTooLarge, 413, "SQS record body exceeds configured limit", details)
			}
		}
	case adapters.TriggerS3:
		for idx, record := range b.request.Records {
			recordSize := estimateS3RecordSize(record)
			if recordSize > maxSize {
				details := map[string]any{
					"size_bytes":   recordSize,
					"limit_bytes":  maxSize,
					"record_index": idx,
					"trigger_type": string(b.request.TriggerType),
				}
				return b.guardrailViolation(ErrorCodePayloadTooLarge, 413, "S3 object size exceeds configured limit", details)
			}
		}
	}

	return nil
}

func (b *requestHandlerBuilder) enforceTenantRequirement() error {
	if !b.app.config.RequireTenantID {
		return nil
	}

	tenantID := b.liftCtx.GetTenantID()
	if tenantID == "" && b.liftCtx.Request != nil {
		if header := b.liftCtx.Request.GetHeader("X-Tenant-ID"); header != "" {
			b.liftCtx.SetTenantID(header)
			tenantID = header
		}
	}

	if tenantID != "" {
		return nil
	}

	details := map[string]any{
		"trigger_type": string(b.request.TriggerType),
	}
	return b.guardrailViolation(ErrorCodeTenantRequired, 400, "Tenant ID is required", details)
}

func (b *requestHandlerBuilder) validateResponseGuardrails() error {
	maxSize := b.app.config.MaxResponseSize
	if maxSize <= 0 {
		return nil
	}

	size, err := b.responsePayloadSize()
	if err != nil {
		return err
	}

	if size > maxSize {
		details := map[string]any{
			"size_bytes":   size,
			"limit_bytes":  maxSize,
			"trigger_type": string(b.request.TriggerType),
		}
		return b.guardrailViolation(ErrorCodePayloadTooLarge, 413, "Response body exceeds configured limit", details)
	}

	return nil
}

func (b *requestHandlerBuilder) responsePayloadSize() (int64, error) {
	if b.liftCtx == nil || b.liftCtx.Response == nil {
		return 0, nil
	}

	body := b.liftCtx.Response.Body
	if body == nil {
		return 0, nil
	}

	switch v := body.(type) {
	case string:
		return int64(len(v)), nil
	case []byte:
		return int64(len(v)), nil
	default:
		serialized, err := json.Marshal(v)
		if err != nil {
			return 0, NewLiftError(ErrorCodeMarshalError, "Failed to marshal response body", 500).WithCause(err)
		}
		return int64(len(serialized)), nil
	}
}

func (b *requestHandlerBuilder) guardrailViolation(code string, status int, message string, details map[string]any) *LiftError {
	b.prepareErrorResponse()
	if details == nil {
		details = make(map[string]any)
	}
	if _, exists := details["trigger_type"]; !exists && b.request != nil {
		details["trigger_type"] = string(b.request.TriggerType)
	}

	if b.app.logger != nil {
		fields := map[string]any{
			"code": code,
		}
		for k, v := range details {
			fields[k] = v
		}
		b.app.logger.Warn(message, fields)
	}

	if b.app.metrics != nil {
		tags := map[string]string{
			"code": code,
		}
		if trigger, ok := details["trigger_type"].(string); ok {
			tags["trigger_type"] = trigger
		}
		b.app.metrics.Counter("lift_guardrail_violation_total", tags).Inc()
	}

	return NewLiftError(code, message, status).WithDetails(details)
}

func (b *requestHandlerBuilder) prepareErrorResponse() {
	if b.liftCtx == nil {
		return
	}
	b.liftCtx.Response = NewResponse()
	if b.app.hasInterceptingMiddleware {
		b.liftCtx.EnableResponseBuffering()
	}
}

func estimateSQSRecordSize(record any) int64 {
	recordMap, ok := record.(map[string]any)
	if !ok {
		return 0
	}

	if body, ok := recordMap["body"].(string); ok {
		return int64(len(body))
	}

	return 0
}

func estimateS3RecordSize(record any) int64 {
	recordMap, ok := record.(map[string]any)
	if !ok {
		return 0
	}

	s3Field, ok := recordMap["s3"].(map[string]any)
	if !ok {
		return 0
	}

	object, ok := s3Field["object"].(map[string]any)
	if !ok {
		return 0
	}

	sizeVal, exists := object["size"]
	if !exists {
		return 0
	}

	switch v := sizeVal.(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	case json.Number:
		if parsed, err := v.Int64(); err == nil {
			return parsed
		}
	}

	return 0
}

// routeRequest routes the request based on trigger type.
// It determines the type of request and routes it accordingly.
func (b *requestHandlerBuilder) routeRequest() error {
	err := b.executeWithTimeout(func() error {
		switch {
		case b.request.TriggerType == adapters.TriggerWebSocket:
			return b.routeWebSocket()
		case b.request.TriggerType == adapters.TriggerEventBus && b.app != nil && b.app.hasEventBusRoutes():
			return b.routeEventBus()
		case b.isEventTrigger():
			return b.routeEvent()
		default:
			return b.routeHTTP()
		}
	})
	return err
}

func (b *requestHandlerBuilder) executeWithTimeout(fn func() error) error {
	if b.app.config.Timeout <= 0 {
		return fn()
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- fn()
	}()

	select {
	case err := <-errCh:
		return err
	case <-b.liftCtx.Done():
		err := b.liftCtx.Err()
		if errors.Is(err, context.DeadlineExceeded) {
			details := map[string]any{
				"timeout_seconds": b.app.config.Timeout,
			}
			return b.guardrailViolation(ErrorCodeTimeout, 504, "Request timed out", details)
		}
		return err
	}
}

// isEventTrigger checks if this is a non-HTTP event trigger.
//
// Returns:
//   - A boolean indicating if the request is a non-HTTP event
func (b *requestHandlerBuilder) isEventTrigger() bool {
	return b.request.TriggerType != adapters.TriggerAPIGateway &&
		b.request.TriggerType != adapters.TriggerAPIGatewayV2 &&
		b.request.TriggerType != adapters.TriggerAppSync && // AppSync is HTTP-like
		b.request.TriggerType != adapters.TriggerUnknown
}

// routeWebSocket handles WebSocket routing.
// It routes WebSocket requests to the appropriate handler.

func (b *requestHandlerBuilder) routeWebSocket() error {
	routeKey := b.extractRouteKey()
	handler := b.app.RouteWebSocket(routeKey)

	if handler == nil {
		return NewLiftError("WEBSOCKET_ROUTE_NOT_FOUND",
			fmt.Sprintf("No handler for WebSocket route: %s", routeKey), 404)
	}

	finalHandler := b.applyMiddleware(handler)
	finalHandler = b.applyConnectionManagement(finalHandler)

	return finalHandler.Handle(b.liftCtx)
}

// extractRouteKey gets the WebSocket route key from metadata.
//
// Returns:
//   - The route key as a string
func (b *requestHandlerBuilder) extractRouteKey() string {
	if metadata, ok := b.request.Metadata["routeKey"].(string); ok {
		return metadata
	}
	return ""
}

// applyMiddleware wraps the handler with middleware.
//
// Parameters:
//   - handler: The handler to wrap
//
// Returns:
//   - The wrapped handler
func (b *requestHandlerBuilder) applyMiddleware(handler Handler) Handler {
	finalHandler := handler
	chain := b.app.httpMiddlewareChain()
	for i := len(chain) - 1; i >= 0; i-- {
		finalHandler = chain[i](finalHandler)
	}
	return finalHandler
}

// applyConnectionManagement adds WebSocket connection management if enabled.
//
// Parameters:
//   - handler: The handler to wrap
//
// Returns:
//   - The wrapped handler
func (b *requestHandlerBuilder) applyConnectionManagement(handler Handler) Handler {
	if b.app.wsOptions != nil && b.app.wsOptions.EnableAutoConnectionManagement {
		return wrapWithConnectionManagement(handler, b.app.wsOptions.ConnectionStore)
	}
	return handler
}

// routeEvent handles non-HTTP event routing.
// It routes non-HTTP events to the appropriate handler.
func (b *requestHandlerBuilder) routeEvent() error {
	return b.app.eventRouter.HandleEvent(b.liftCtx, b.app.eventMiddlewareChain())
}

// routeHTTP handles HTTP request routing.
// It routes HTTP requests to the appropriate handler.
func (b *requestHandlerBuilder) routeHTTP() error {
	return b.app.router.Handle(b.liftCtx)
}

// parseEvent converts a Lambda event to our Request structure.
//
// Parameters:
//   - event: The Lambda event
//
// Returns:
//   - The parsed Request
//   - An error if the event parsing fails
func (a *App) parseEvent(event any) (*Request, error) {
	a.logEventDebug(event)

	if req, ok := a.tryPreferredAdapters(event); ok {
		return req, nil
	}

	return a.detectAndAdaptEvent(event)
}

// logEventDebug logs useful debug information about the incoming event.
// It logs debug information if debug mode is enabled.
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

// tryPreferredAdapters attempts to parse using preferred adapters first.
//
// Parameters:
//   - event: The Lambda event
//
// Returns:
//   - The parsed Request
//   - A boolean indicating if the parsing was successful
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

// detectAndAdaptEvent uses the registry to detect and adapt events.
//
// Parameters:
//   - event: The Lambda event
//
// Returns:
//   - The parsed Request
//   - An error if the event parsing fails
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

// handleError processes errors and returns appropriate responses.
func (a *App) handleError(ctx *Context, err error) (any, error) {
	// DynamoDB stream processors should generally surface errors so Lambda retries (or sends to DLQ).
	// EventBus handlers use BatchItemFailures to avoid returning errors for per-record failures.
	if ctx != nil && ctx.Request != nil && ctx.Request.TriggerType == adapters.TriggerEventBus {
		return nil, err
	}
	// SQS handlers should surface errors so Lambda retries the batch (or sends to DLQ).
	if ctx != nil && ctx.Request != nil && ctx.Request.TriggerType == adapters.TriggerSQS {
		return nil, err
	}
	if a.isAppSyncRequest(ctx) {
		return a.handleAppSyncError(ctx, err)
	}
	return a.handleStandardError(ctx, err)
}

func (a *App) isAppSyncRequest(ctx *Context) bool {
	return ctx.Request != nil && ctx.Request.TriggerType == adapters.TriggerAppSync
}

func (a *App) handleAppSyncError(ctx *Context, err error) (any, error) {
	if liftErr, ok := err.(*LiftError); ok {
		a.enrichAppSyncLiftError(ctx, liftErr)
		return a.buildAppSyncErrorResponse(liftErr), nil
	}
	return map[string]any{
		"pay_theory_error": true,
		"error_message":    err.Error(),
		"error_type":       "SYSTEM_ERROR",
		"error_data":       map[string]any{},
		"error_info":       map[string]any{},
	}, nil
}

func (a *App) enrichAppSyncLiftError(ctx *Context, liftErr *LiftError) {
	if liftErr.ErrorData == nil {
		liftErr.ErrorData = make(map[string]any)
	}
	if liftErr.ErrorInfo == nil {
		liftErr.ErrorInfo = make(map[string]any)
	}

	if ctx.Request != nil {
		liftErr.ErrorInfo["trigger_type"] = string(ctx.Request.TriggerType)
		liftErr.ErrorInfo["path"] = ctx.Request.Path
		liftErr.ErrorInfo["method"] = ctx.Request.Method
		if ctx.Request.EventID != "" && liftErr.RequestID == "" {
			liftErr.RequestID = ctx.Request.EventID
		}
	}

	if liftErr.Timestamp == "" {
		liftErr.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	liftErr.ErrorData["status_code"] = liftErr.StatusCode
	liftErr.ErrorData["timestamp"] = liftErr.Timestamp
	if liftErr.RequestID != "" {
		liftErr.ErrorData["request_id"] = liftErr.RequestID
	}
	if liftErr.TraceID != "" {
		liftErr.ErrorData["trace_id"] = liftErr.TraceID
	}

	liftErr.ErrorInfo["code"] = liftErr.Code
	if len(liftErr.Details) > 0 {
		liftErr.ErrorInfo["details"] = liftErr.Details
	}
}

func (a *App) buildAppSyncErrorResponse(liftErr *LiftError) map[string]any {
	return map[string]any{
		"pay_theory_error": true,
		"error_message":    liftErr.Message,
		"error_type":       determineAppSyncErrorType(liftErr.StatusCode),
		"error_data":       liftErr.ErrorData,
		"error_info":       liftErr.ErrorInfo,
	}
}

func determineAppSyncErrorType(statusCode int) string {
	switch {
	case statusCode >= 500:
		return "SYSTEM_ERROR"
	case statusCode >= 400:
		return "CLIENT_ERROR"
	default:
		return "SYSTEM_ERROR"
	}
}

func (a *App) handleStandardError(ctx *Context, err error) (any, error) {
	if liftErr, ok := err.(*LiftError); ok {
		return a.respondWithLiftError(ctx, liftErr)
	}
	return a.respondWithInternalError(ctx)
}

func (a *App) respondWithLiftError(ctx *Context, liftErr *LiftError) (any, error) {
	resp := map[string]any{
		"code":    liftErr.Code,
		"message": liftErr.Message,
	}
	if len(liftErr.Details) > 0 {
		resp["details"] = liftErr.Details
	}

	if err := ctx.Status(liftErr.StatusCode).JSON(resp); err != nil {
		return nil, fmt.Errorf("failed to send error response: %w", err)
	}
	return ctx.Response, nil
}

func (a *App) respondWithInternalError(ctx *Context) (any, error) {
	if err := ctx.Status(500).JSON(map[string]string{
		"error": "Internal server error",
	}); err != nil {
		return nil, fmt.Errorf("failed to send internal server error response: %w", err)
	}
	return ctx.Response, nil
}
