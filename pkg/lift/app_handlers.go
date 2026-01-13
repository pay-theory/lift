package lift

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/pay-theory/lift/pkg/utils/stdio"
)

// HandleTestRequest processes a test request directly through the router.
// This is used by the testing framework to bypass event parsing.
//
// Parameters:
//   - ctx: The context for the request
//
// Returns:
//   - An error if the request handling fails
//
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

// convertHandlerUsingReflection converts various handler function types to the Handler interface using reflection.
//
// Parameters:
//   - handler: The handler to convert
//
// Returns:
//   - The converted Handler
//   - An error if the conversion fails
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

// validateHandlerSignature validates that the handler function has a supported signature.
//
// Parameters:
//   - t: The reflect.Type of the handler function
//
// Returns:
//   - An error if the signature is unsupported
//
// handlerPattern represents the different handler signature patterns supported
type handlerPattern int

const (
	patternContextError    handlerPattern = iota // func(*Context) error
	patternContextResponse                       // func(*Context) (any, error)
	patternSimpleError                           // func() error
	patternSimpleResponse                        // func() (any, error)
	patternModelError                            // func(RequestModel) error
	patternModelResponse                         // func(RequestModel) (ResponseModel, error)
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

// identifyPattern determines which handler pattern a function type matches.
//
// Parameters:
//   - t: The reflect.Type of the handler function
//
// Returns:
//   - The handlerPattern
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

// validateSingleInOut validates patterns: func(*Context) error OR func(RequestModel) error.
//
// Parameters:
//   - t: The reflect.Type of the handler function
//
// Returns:
//   - The handlerPattern
func (v *handlerValidator) validateSingleInOut(t reflect.Type) handlerPattern {
	if !isErrorType(t.Out(0)) {
		return patternUnsupported
	}

	if isContextType(t.In(0)) {
		return patternContextError
	}

	return patternModelError
}

// validateSingleInDoubleOut validates patterns: func(*Context) (any, error) OR func(RequestModel) (ResponseModel, error).
//
// Parameters:
//   - t: The reflect.Type of the handler function
//
// Returns:
//   - The handlerPattern
func (v *handlerValidator) validateSingleInDoubleOut(t reflect.Type) handlerPattern {
	if !isInterfaceType(t.Out(0)) || !isErrorType(t.Out(1)) {
		return patternUnsupported
	}

	if isContextType(t.In(0)) {
		return patternContextResponse
	}

	return patternModelResponse
}

// validateNoInSingleOut validates pattern: func() error.
//
// Parameters:
//   - t: The reflect.Type of the handler function
//
// Returns:
//   - The handlerPattern
func (v *handlerValidator) validateNoInSingleOut(t reflect.Type) handlerPattern {
	if isErrorType(t.Out(0)) {
		return patternSimpleError
	}
	return patternUnsupported
}

// validateNoInDoubleOut validates pattern: func() (any, error).
//
// Parameters:
//   - t: The reflect.Type of the handler function
//
// Returns:
//   - The handlerPattern
func (v *handlerValidator) validateNoInDoubleOut(t reflect.Type) handlerPattern {
	if isInterfaceType(t.Out(0)) && isErrorType(t.Out(1)) {
		return patternSimpleResponse
	}
	return patternUnsupported
}

// handlerExecutor handles the execution of different handler patterns.
// It is responsible for executing the handler function based on its signature.
// Memory optimized: struct with 48 pointer bytes could be 32
type handlerExecutor struct {
	// reflect.Type (16 bytes - interface)
	funcType reflect.Type
	// reflect.Value (24 bytes)
	value reflect.Value
	// enum (8 bytes on 64-bit)
	pattern handlerPattern
}

// createReflectedHandler creates a Handler from a reflected function.
//
// Parameters:
//   - v: The reflect.Value of the handler function
//   - t: The reflect.Type of the handler function
//
// Returns:
//   - The Handler
func createReflectedHandler(v reflect.Value, t reflect.Type) Handler {
	validator := &handlerValidator{}
	pattern := validator.identifyPattern(t)

	executor := &handlerExecutor{
		value:    v,
		pattern:  pattern,
		funcType: t,
	}

	return HandlerFunc(func(ctx *Context) error {
		return executor.execute(ctx)
	})
}

// execute runs the handler function based on its identified pattern.
//
// Parameters:
//   - ctx: The context for the request
//
// Returns:
//   - An error if the execution fails
func (e *handlerExecutor) execute(ctx *Context) error {
	callArgs, err := e.prepareArgs(ctx)
	if err != nil {
		return err
	}

	results := e.value.Call(callArgs)
	return e.handleResults(ctx, results)
}

// prepareArgs prepares the arguments for the function call based on the pattern.
//
// Parameters:
//   - ctx: The context for the request
//
// Returns:
//   - The arguments for the function call
//   - An error if the preparation fails
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

// prepareModelArgs prepares arguments for model-based handlers.
//
// Parameters:
//   - ctx: The context for the request
//
// Returns:
//   - The arguments for the function call
//   - An error if the preparation fails
func (e *handlerExecutor) prepareModelArgs(ctx *Context) ([]reflect.Value, error) {
	requestType := e.funcType.In(0)
	requestValue := reflect.New(requestType).Interface()

	if err := ctx.ParseRequest(requestValue); err != nil {
		return nil, err
	}

	return []reflect.Value{reflect.ValueOf(requestValue).Elem()}, nil
}

// handleResults processes the return values from the handler function.
//
// Parameters:
//   - ctx: The context for the request
//   - results: The return values from the handler function
//
// Returns:
//   - An error if the result handling fails
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

// handleSingleResult handles functions that return only an error.
//
// Parameters:
//   - result: The return value from the handler function
//
// Returns:
//   - An error if the result handling fails
func (e *handlerExecutor) handleSingleResult(result reflect.Value) error {
	if result.IsNil() {
		return nil
	}

	if err, ok := result.Interface().(error); ok {
		return err
	}

	return fmt.Errorf("handler returned non-error value: %v", result.Interface())
}

// handleDoubleResult handles functions that return (value, error).
//
// Parameters:
//   - ctx: The context for the request
//   - valueResult: The value returned by the handler function
//   - errorResult: The error returned by the handler function
//
// Returns:
//   - An error if the result handling fails
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

// Helper functions for type checking.
// These functions are used to validate the types of handler functions.

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

// parseTriggerType converts a string to a TriggerType.
//
// Parameters:
//   - s: The string to convert
//
// Returns:
//   - The TriggerType
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
			stdio.Stdoutln("Debug: Error handling request during local test")
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
