package lift

import (
	"context"
	"fmt"
	"reflect"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pay-theory/dynamorm"
)

var (
	eventBusHandlerErrorType = reflect.TypeOf((*error)(nil)).Elem()
	eventBusHandlerCtxType   = reflect.TypeOf((*Context)(nil))
)

type eventBusRoute struct {
	handler      reflect.Value
	eventPtrType reflect.Type
	pattern      string
}

// EventBus registers a handler for EventBus stream events matching an `event_type` pattern.
//
// The handler must have the signature:
//
//	func(*lift.Context, *T) error
//
// Where `T` is typically `services.Event` (from `github.com/pay-theory/lift/pkg/services`).
//
// Pattern matching supports `*` wildcards (e.g. `partner.*`, `*.created`).
func (a *App) EventBus(pattern string, handler any) error {
	if a == nil {
		return fmt.Errorf("app is nil")
	}
	if handler == nil {
		return fmt.Errorf("handler is required")
	}

	t := reflect.TypeOf(handler)
	if t.Kind() != reflect.Func {
		return fmt.Errorf("handler must be a function, got %T", handler)
	}
	if t.NumIn() != 2 {
		return fmt.Errorf("handler must accept 2 arguments (got %d)", t.NumIn())
	}
	if t.In(0) != eventBusHandlerCtxType {
		return fmt.Errorf("handler first argument must be *lift.Context")
	}

	eventPtrType := t.In(1)
	if eventPtrType.Kind() != reflect.Pointer || eventPtrType.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("handler second argument must be a pointer to a struct")
	}

	if t.NumOut() != 1 || t.Out(0) != eventBusHandlerErrorType {
		return fmt.Errorf("handler must return a single error value")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.eventBusRoutes = append(a.eventBusRoutes, &eventBusRoute{
		pattern:      pattern,
		handler:      reflect.ValueOf(handler),
		eventPtrType: eventPtrType,
	})

	return nil
}

func (a *App) eventBusRoutesSnapshot() []*eventBusRoute {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return append([]*eventBusRoute(nil), a.eventBusRoutes...)
}

func (b *requestHandlerBuilder) routeEventBus() error {
	routes := b.app.eventBusRoutesSnapshot()
	records, err := b.liftCtx.EventBusRecords()
	if err != nil {
		return err
	}

	failures := make([]events.DynamoDBBatchItemFailure, 0)
	middlewareChain := b.app.eventMiddlewareChain()

	for _, record := range records {
		image, err := eventBusStreamImage(record)
		if err != nil {
			failures = append(failures, events.DynamoDBBatchItemFailure{ItemIdentifier: eventBusItemIdentifier(record)})
			continue
		}

		eventType := eventBusEventType(image)
		if eventType == "" {
			continue
		}
		if eventBusEventID(image) == "" {
			// Skip non-event rows co-located in the EventBus table (e.g., schedules/checkpoints/quarantine).
			continue
		}

		route := findEventBusRoute(routes, eventType)
		if route == nil {
			continue
		}

		eventPtr := reflect.New(route.eventPtrType.Elem())
		if err := dynamorm.UnmarshalStreamImage(image, eventPtr.Interface()); err != nil {
			failures = append(failures, events.DynamoDBBatchItemFailure{ItemIdentifier: eventBusItemIdentifier(record)})
			continue
		}

		recordCtx := b.eventBusRecordContext(record)
		recordCtx.Set("eventbus.event_type", eventType)
		recordCtx.Set("eventbus.record", record)

		handler := HandlerFunc(func(ctx *Context) error {
			results := route.handler.Call([]reflect.Value{reflect.ValueOf(ctx), eventPtr})
			if len(results) != 1 {
				return SystemError("EventBus handler must return exactly one value")
			}
			if results[0].IsNil() {
				return nil
			}
			err, ok := results[0].Interface().(error)
			if !ok {
				return SystemError("EventBus handler return value must be error")
			}
			return err
		})

		finalHandler := Handler(handler)
		for i := len(middlewareChain) - 1; i >= 0; i-- {
			finalHandler = middlewareChain[i](finalHandler)
		}

		if err := finalHandler.Handle(recordCtx); err != nil {
			failures = append(failures, events.DynamoDBBatchItemFailure{ItemIdentifier: eventBusItemIdentifier(record)})
		}
	}

	b.liftCtx.Response.Body = events.DynamoDBEventResponse{BatchItemFailures: failures}
	return nil
}

func (b *requestHandlerBuilder) eventBusRecordContext(record events.DynamoDBEventRecord) *Context {
	if b == nil || b.liftCtx == nil || b.request == nil {
		return NewContext(context.Background(), &Request{})
	}

	baseCtx := b.liftCtx.Context
	if baseCtx == nil {
		baseCtx = context.Background()
	}

	reqCopy := *b.request
	if b.request.Request != nil {
		adapterCopy := *b.request.Request
		adapterCopy.Records = []any{record}
		adapterCopy.EventID = record.EventID
		reqCopy.Request = &adapterCopy
	}
	reqCopy.SetContext(baseCtx)

	ctx := NewContext(baseCtx, &reqCopy)
	ctx.Logger = b.liftCtx.Logger
	ctx.Metrics = b.liftCtx.Metrics
	ctx.DB = b.liftCtx.DB
	if tracer := b.liftCtx.GetTracer(); tracer != nil {
		ctx.SetTracer(tracer)
	}
	if b.cancel != nil {
		ctx.Set(requestCancelKey, b.cancel)
	}
	if b.app != nil && b.app.hasInterceptingMiddleware {
		ctx.EnableResponseBuffering()
	}

	return ctx
}

func findEventBusRoute(routes []*eventBusRoute, eventType string) *eventBusRoute {
	if eventType == "" {
		return nil
	}

	for _, route := range routes {
		if route == nil {
			continue
		}
		if route.pattern == "" || route.pattern == "*" {
			return route
		}
		if newWildcardMatcher(eventType, route.pattern).match() {
			return route
		}
	}

	return nil
}

func eventBusItemIdentifier(record events.DynamoDBEventRecord) string {
	if record.EventID != "" {
		return record.EventID
	}
	return record.Change.SequenceNumber
}

func eventBusStreamImage(record events.DynamoDBEventRecord) (map[string]events.DynamoDBAttributeValue, error) {
	switch record.EventName {
	case string(events.DynamoDBOperationTypeRemove):
		if len(record.Change.OldImage) == 0 {
			return nil, fmt.Errorf("REMOVE record missing OldImage")
		}
		return record.Change.OldImage, nil
	default:
		if len(record.Change.NewImage) > 0 {
			return record.Change.NewImage, nil
		}
		if len(record.Change.OldImage) > 0 {
			return record.Change.OldImage, nil
		}
		return nil, fmt.Errorf("record missing NewImage and OldImage")
	}
}

func eventBusEventType(image map[string]events.DynamoDBAttributeValue) string {
	av, ok := image["event_type"]
	if !ok {
		return ""
	}
	return av.String()
}

func eventBusEventID(image map[string]events.DynamoDBAttributeValue) string {
	av, ok := image["id"]
	if !ok {
		return ""
	}
	return av.String()
}
