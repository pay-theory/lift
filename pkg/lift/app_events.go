package lift

import (
	"fmt"
)

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
