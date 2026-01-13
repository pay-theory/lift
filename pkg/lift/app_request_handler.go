package lift

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"

	"github.com/pay-theory/lift/pkg/lift/adapters"
)

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
			fmt.Println("Debug: Error handling request during local test")
		}
	}
}
