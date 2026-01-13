package lift

import (
	"fmt"
)

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
