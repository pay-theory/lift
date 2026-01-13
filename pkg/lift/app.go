package lift

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

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
	shutdownHooks     []func()
	preferredAdapters []adapters.TriggerType
	features          map[string]bool

	// Functions (8 bytes)
	metricsFactory func(*Config) MetricsCollector
	tracerFactory  func(*Config) any

	// Mutex (8 bytes)
	mu sync.RWMutex

	// Bools (1 byte)
	started                   bool
	hasInterceptingMiddleware bool
}

// New creates a new App instance with the provided options.
// It initializes the App with default configuration and applies the provided options.
//
// Parameters:
//   - options: A variadic list of AppOption functions
//
// Returns:
//   - A pointer to the newly created App
func New(options ...AppOption) *App {
	app := &App{
		config:          DefaultConfig(),
		router:          NewRouter(),
		eventRouter:     NewEventRouter(),
		adapterRegistry: adapters.NewAdapterRegistry(),
		features:        make(map[string]bool),
		wsRoutes:        make(map[string]WebSocketHandler),
	}

	for _, option := range options {
		option(app)
	}

	return app
}

func (a *App) ensureLifecycleContextLocked() {
	if a.lifecycleCtx == nil {
		a.lifecycleCtx, a.lifecycleCancel = context.WithCancel(context.Background())
	}
}

// LifecycleContext returns a context that is canceled when the app is stopped.
//
// Returns:
//   - The lifecycle context
func (a *App) LifecycleContext() context.Context {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ensureLifecycleContextLocked()
	return a.lifecycleCtx
}

// RegisterShutdownHook registers a function to be called when the app is stopped.
//
// Parameters:
//   - hook: The function to call on shutdown
func (a *App) RegisterShutdownHook(hook func()) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.shutdownHooks = append(a.shutdownHooks, hook)
}

// Stop stops the application and runs shutdown hooks.
func (a *App) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.lifecycleCancel != nil {
		a.lifecycleCancel()
	}

	for i := len(a.shutdownHooks) - 1; i >= 0; i-- {
		a.shutdownHooks[i]()
	}
	a.started = false
}

// Use adds a middleware to the application.
//
// Parameters:
//   - mw: The middleware function
//
// Returns:
//   - A pointer to the App
func (a *App) Use(mw func(Handler) Handler) *App {
	m := Middleware(mw)
	a.addMiddleware(m, middlewareAppliesToEvents(m))
	return a
}

func (a *App) addMiddleware(mw Middleware, appliesToEvents bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.middleware = append(a.middleware, middlewareEntry{
		handler:         mw,
		appliesToEvents: appliesToEvents,
	})
	// Check if this middleware might intercept/modify responses (heuristic)
	a.hasInterceptingMiddleware = true
}

func (a *App) httpMiddlewareChain() []Middleware {
	chain := make([]Middleware, 0, len(a.middleware))
	for _, mw := range a.middleware {
		chain = append(chain, mw.handler)
	}
	return chain
}

func (a *App) eventMiddlewareChain() []Middleware {
	var chain []Middleware
	for _, mw := range a.middleware {
		if mw.appliesToEvents {
			chain = append(chain, mw.handler)
		}
	}
	return chain
}

func (a *App) ensureTelemetry() {
	if a.config.MetricsEnabled && a.metrics == nil && a.metricsFactory != nil {
		a.metrics = a.metricsFactory(a.config)
	}
	if a.config.TracingEnabled && a.tracer == nil && a.tracerFactory != nil {
		a.tracer = a.tracerFactory(a.config)
	}
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

// RunLocalTest runs local testing logic when not in Lambda environment.
// It is used to run tests locally without deploying to AWS Lambda.
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
