package lift

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/aws/aws-lambda-go/events"
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

// SSEResponse configures a streaming response that emits SSE-formatted events
// from eventChan. This requires the Lambda integration to be configured for
// response streaming (for example API Gateway REST API with STREAM transfer).
//
// Note: AWS Lambda response streaming in Go requires compiling with
// `-tags lambda.norpc` (or using a provided runtime) when deployed.
func SSEResponse(ctx *Context, eventChan <-chan SSEEvent) error {
	if ctx == nil {
		return errors.New("lift: nil context")
	}
	if eventChan == nil {
		return errors.New("lift: nil event channel")
	}

	pipeReader, pipeWriter := io.Pipe()

	headers := map[string]string{
		HeaderContentType: "text/event-stream",
		"Cache-Control":   "no-cache",
		"Connection":      "keep-alive",
	}

	streamingResp := &events.LambdaFunctionURLStreamingResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       pipeReader,
	}

	// Keep the standard Lift response in sync for middleware that inspects it
	// (e.g. logging).
	if ctx.Response != nil {
		ctx.Response.StatusCode = 200
		if ctx.Response.Headers == nil {
			ctx.Response.Headers = make(map[string]string)
		}
		for k, v := range headers {
			ctx.Response.Headers[k] = v
		}
	}

	var startOnce sync.Once
	cancelAny := ctx.Get(requestCancelKey)
	cancel, ok := cancelAny.(context.CancelFunc)
	if !ok {
		cancel = nil
	}
	startFn := func() {
		startOnce.Do(func() {
			go func() {
				streamSSE(ctx.Context, pipeWriter, eventChan)
				if cancel != nil {
					cancel()
				}
			}()
		})
	}

	abortFn := func(err error) {
		if err == nil {
			if closeErr := pipeWriter.Close(); closeErr != nil {
				_ = closeErr
			}
			return
		}
		if closeErr := pipeWriter.CloseWithError(err); closeErr != nil {
			_ = closeErr
		}
	}

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
