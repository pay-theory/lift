package lift

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/aws/aws-lambda-go/events"

	"github.com/pay-theory/lift/pkg/lift/adapters"
)

// SSEEvent represents a single Server-Sent Event.
type SSEEvent struct {
	// Event is the event type (optional).
	Event string
	// Data is the event payload (required).
	Data string
	// ID is the event identifier (optional).
	ID string
	// Retry is the reconnection time in milliseconds (optional).
	Retry int
}

// StreamingContext wraps a lift.Context with an io.Writer for streaming bodies.
type StreamingContext struct {
	*Context
	Writer io.Writer
}

type streamingState struct {
	response any
	start    func()
	abort    func(error)
}

const streamingStateKey = "__lift_streaming_state"
const requestCancelKey = "__lift_request_cancel"
const responseMultiValueHeadersKey = "__lift_response_multi_value_headers"

// AddMultiValueHeader appends a response header value under the same header
// name (multi-value headers).
//
// This is currently applied only when Lift returns an API Gateway REST API (v1)
// streaming response (events.APIGatewayProxyStreamingResponse), which supports
// the `multiValueHeaders` response metadata field.
func (c *Context) AddMultiValueHeader(key, value string) {
	if c == nil {
		return
	}
	if key == "" {
		return
	}
	headers := c.getOrCreateMultiValueResponseHeaders()
	headers[key] = append(headers[key], value)
}

func (c *Context) getOrCreateMultiValueResponseHeaders() map[string][]string {
	if c == nil {
		return nil
	}
	existing := c.Get(responseMultiValueHeadersKey)
	headers, ok := existing.(map[string][]string)
	if ok && headers != nil {
		return headers
	}

	headers = make(map[string][]string)
	c.Set(responseMultiValueHeadersKey, headers)
	return headers
}

func (c *Context) multiValueResponseHeaders() map[string][]string {
	if c == nil {
		return nil
	}
	existing := c.Get(responseMultiValueHeadersKey)
	headers, ok := existing.(map[string][]string)
	if !ok {
		return nil
	}
	return headers
}

func (c *Context) setStreamingState(state *streamingState) {
	if c == nil {
		return
	}
	c.Set(streamingStateKey, state)
}

func (c *Context) streamingState() *streamingState {
	if c == nil {
		return nil
	}
	val := c.Get(streamingStateKey)
	state, ok := val.(*streamingState)
	if !ok {
		return nil
	}
	return state
}

func (c *Context) clearStreamingState() {
	if c == nil {
		return
	}
	if c.values == nil {
		return
	}
	delete(c.values, streamingStateKey)
}

func validateSSEResponseInputs(ctx *Context, eventChan <-chan SSEEvent) error {
	if ctx == nil {
		return errors.New("lift: nil context")
	}
	if eventChan == nil {
		return errors.New("lift: nil event channel")
	}
	return nil
}

func buildSSEHeaders(ctx *Context) map[string]string {
	headers := map[string]string{}
	if ctx != nil && ctx.Response != nil && ctx.Response.Headers != nil {
		for k, v := range ctx.Response.Headers {
			headers[k] = v
		}
	}

	headers[HeaderContentType] = "text/event-stream"
	headers["Cache-Control"] = "no-cache"

	return headers
}

func buildStreamingResponse(ctx *Context, reader io.Reader, headers map[string]string, multiValueHeaders map[string][]string) any {
	if ctx != nil && ctx.Request != nil && ctx.Request.TriggerType == adapters.TriggerAPIGateway {
		return &events.APIGatewayProxyStreamingResponse{
			StatusCode:        200,
			Headers:           headers,
			MultiValueHeaders: multiValueHeaders,
			Body:              reader,
		}
	}

	return &events.LambdaFunctionURLStreamingResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       reader,
	}
}

func syncStreamingHeadersToLiftResponse(ctx *Context, headers map[string]string) {
	if ctx == nil || ctx.Response == nil {
		return
	}

	ctx.Response.StatusCode = 200
	if ctx.Response.Headers == nil {
		ctx.Response.Headers = make(map[string]string)
	}
	for k, v := range headers {
		ctx.Response.Headers[k] = v
	}
}

func requestCancelFunc(ctx *Context) context.CancelFunc {
	if ctx == nil {
		return nil
	}
	cancelAny := ctx.Get(requestCancelKey)
	cancel, ok := cancelAny.(context.CancelFunc)
	if !ok {
		return nil
	}
	return cancel
}

func newSSEStartFn(ctx context.Context, writer *io.PipeWriter, eventChan <-chan SSEEvent, cancel context.CancelFunc) func() {
	var startOnce sync.Once
	return func() {
		startOnce.Do(func() {
			go func() {
				streamSSE(ctx, writer, eventChan)
				if cancel != nil {
					cancel()
				}
			}()
		})
	}
}

func newSSEAbortFn(writer *io.PipeWriter) func(error) {
	return func(err error) {
		if err == nil {
			if closeErr := writer.Close(); closeErr != nil {
				_ = closeErr
			}
			return
		}
		if closeErr := writer.CloseWithError(err); closeErr != nil {
			_ = closeErr
		}
	}
}

// SSEResponse configures a streaming response that emits SSE-formatted events
// from eventChan. This requires the Lambda integration to be configured for
// response streaming (for example API Gateway REST API v1 with STREAM transfer).
//
// When invoked via API Gateway REST API (v1), Lift returns an
// events.APIGatewayProxyStreamingResponse (supports multiValueHeaders). For
// other trigger types, Lift returns an events.LambdaFunctionURLStreamingResponse
// for compatibility with Function URL response streaming.
//
// Note: AWS Lambda response streaming in Go requires compiling with
// `-tags lambda.norpc` (or using a provided runtime) when deployed.
func SSEResponse(ctx *Context, eventChan <-chan SSEEvent) error {
	if err := validateSSEResponseInputs(ctx, eventChan); err != nil {
		return err
	}

	pipeReader, pipeWriter := io.Pipe()

	headers := buildSSEHeaders(ctx)
	multiValueHeaders := ctx.multiValueResponseHeaders()
	streamingResp := buildStreamingResponse(ctx, pipeReader, headers, multiValueHeaders)

	// Keep the standard Lift response in sync for middleware that inspects it
	// (e.g. logging).
	syncStreamingHeadersToLiftResponse(ctx, headers)

	startFn := newSSEStartFn(ctx.Context, pipeWriter, eventChan, requestCancelFunc(ctx))
	abortFn := newSSEAbortFn(pipeWriter)

	ctx.setStreamingState(&streamingState{
		response: streamingResp,
		start:    startFn,
		abort:    abortFn,
	})

	return nil
}

func streamSSE(ctx context.Context, writer *io.PipeWriter, eventChan <-chan SSEEvent) {
	if writer == nil {
		return
	}

	defer func() {
		if err := writer.Close(); err != nil {
			_ = err
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-eventChan:
			if !ok {
				return
			}
			if err := writeSSEEvent(writer, event); err != nil {
				_ = writer.CloseWithError(err)
				return
			}
		}
	}
}

func writeSSEEvent(w io.Writer, event SSEEvent) error {
	if w == nil {
		return errors.New("lift: nil SSE writer")
	}

	var builder strings.Builder
	if event.ID != "" {
		builder.WriteString("id: ")
		builder.WriteString(event.ID)
		builder.WriteByte('\n')
	}
	if event.Event != "" {
		builder.WriteString("event: ")
		builder.WriteString(event.Event)
		builder.WriteByte('\n')
	}
	if event.Retry > 0 {
		builder.WriteString("retry: ")
		builder.WriteString(strconv.Itoa(event.Retry))
		builder.WriteByte('\n')
	}

	// Each line in data is its own "data:" line per SSE spec.
	for _, line := range strings.Split(event.Data, "\n") {
		builder.WriteString("data: ")
		builder.WriteString(line)
		builder.WriteByte('\n')
	}

	builder.WriteByte('\n')

	_, err := io.WriteString(w, builder.String())
	return err
}
