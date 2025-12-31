package lift

import (
	"encoding/json"
	"fmt"
)

const (
	defaultWebSocketActionField = "action"

	contextKeyWebSocketAction            = "websocket.action"
	contextKeyWebSocketActionRouterError = "websocket.action_router_error"
)

// WebSocketActionExtractor extracts an action name from a WebSocket request context.
type WebSocketActionExtractor func(ctx *Context) (string, error)

// WebSocketActionRouterOption configures a WebSocketActionRouter.
type WebSocketActionRouterOption func(*WebSocketActionRouter)

// WithWebSocketActionField configures the JSON field used to route actions (default: "action").
func WithWebSocketActionField(field string) WebSocketActionRouterOption {
	return func(r *WebSocketActionRouter) {
		if r == nil || field == "" {
			return
		}
		r.actionField = field
	}
}

// WithWebSocketActionExtractor configures a custom action extractor.
func WithWebSocketActionExtractor(extractor WebSocketActionExtractor) WebSocketActionRouterOption {
	return func(r *WebSocketActionRouter) {
		if r == nil {
			return
		}
		r.extractor = extractor
	}
}

// WebSocketActionRouter routes $default WebSocket messages by body.action (or configured field).
//
// This is intended for API Gateway WebSocket APIs that only define $connect, $disconnect, and
// $default routes, with action-based dispatch performed inside the $default handler.
type WebSocketActionRouter struct {
	handlers       map[string]WebSocketHandler
	defaultHandler WebSocketHandler
	extractor      WebSocketActionExtractor
	actionField    string
}

// NewWebSocketActionRouter creates a new action router.
func NewWebSocketActionRouter(options ...WebSocketActionRouterOption) *WebSocketActionRouter {
	router := &WebSocketActionRouter{
		handlers:    make(map[string]WebSocketHandler),
		actionField: defaultWebSocketActionField,
	}
	for _, opt := range options {
		if opt == nil {
			continue
		}
		opt(router)
	}
	return router
}

// On registers a handler for a given action.
func (r *WebSocketActionRouter) On(action string, handler WebSocketHandler) *WebSocketActionRouter {
	if r == nil || action == "" || handler == nil {
		return r
	}
	if r.handlers == nil {
		r.handlers = make(map[string]WebSocketHandler)
	}
	r.handlers[action] = handler
	return r
}

// Default sets the handler used when the action is missing/unknown or extraction fails.
func (r *WebSocketActionRouter) Default(handler WebSocketHandler) *WebSocketActionRouter {
	if r == nil {
		return r
	}
	r.defaultHandler = handler
	return r
}

// Handle implements WebSocketHandler; register this as the $default route handler.
func (r *WebSocketActionRouter) Handle(ctx *Context) error {
	if r == nil {
		return SystemError("WebSocket action router is nil")
	}

	action, err := r.extractAction(ctx)
	if err != nil || action == "" {
		if ctx != nil {
			ctx.Set(contextKeyWebSocketAction, action)
			if err != nil {
				ctx.Set(contextKeyWebSocketActionRouterError, err)
			}
		}
		if r.defaultHandler != nil {
			return r.defaultHandler(ctx)
		}
		if err != nil {
			return err
		}
		return ParameterError(r.actionField, fmt.Sprintf("Missing %s in request body", r.actionField))
	}

	if ctx != nil {
		ctx.Set(contextKeyWebSocketAction, action)
	}

	if handler, ok := r.handlers[action]; ok {
		return handler(ctx)
	}

	if r.defaultHandler != nil {
		return r.defaultHandler(ctx)
	}

	return NotFound(fmt.Sprintf("No handler for action: %s", action))
}

func (r *WebSocketActionRouter) extractAction(ctx *Context) (string, error) {
	if r.extractor != nil {
		return r.extractor(ctx)
	}

	if ctx == nil || ctx.Request == nil {
		return "", SystemError("Context request is required")
	}

	if len(ctx.Request.Body) == 0 {
		return "", nil
	}

	var payload map[string]any
	if err := json.Unmarshal(ctx.Request.Body, &payload); err != nil {
		return "", ParameterError("body", "Invalid JSON body").WithCause(err)
	}

	rawAction, ok := payload[r.actionField]
	if !ok {
		return "", nil
	}

	action, ok := rawAction.(string)
	if !ok || action == "" {
		return "", ParameterError(r.actionField, fmt.Sprintf("%s must be a non-empty string", r.actionField))
	}

	return action, nil
}

// WebSocketActions registers an action router as the $default WebSocket handler and returns it
// so handlers can be added fluently.
func (a *App) WebSocketActions(options ...WebSocketActionRouterOption) *WebSocketActionRouter {
	router := NewWebSocketActionRouter(options...)
	a.WebSocket("$default", router.Handle)
	return router
}
