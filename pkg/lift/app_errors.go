package lift

import (
	"fmt"
	"time"

	"github.com/pay-theory/lift/pkg/lift/adapters"
)

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
