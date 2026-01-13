package lift

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-lambda-go/events"

	"github.com/pay-theory/lift/pkg/lift/adapters"
)

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
