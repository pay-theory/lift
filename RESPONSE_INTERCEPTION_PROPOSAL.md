# Proposal: Response Interception Support in Lift Framework

## Executive Summary

The Lift framework currently lacks proper support for middleware to intercept and capture response data after handler execution. This limitation prevents the implementation of critical middleware patterns such as idempotency, response caching, and response transformation.

**Critical Finding**: The current implementation approach of accessing `ctx.Response.Body` after handler execution is fundamentally flawed because the Response.Body is not reliably populated after `ctx.JSON()` is called.

## Current State Analysis

### The Problem

1. **Response Body Inaccessibility**: After a handler calls `ctx.JSON(data)`, the response body (`ctx.Response.Body`) is not reliably accessible to middleware.

2. **Write-Once Constraint**: The `Response.JSON()` method marks the response as "written" and prevents subsequent modifications.

3. **Caching Middleware Issues**: The existing caching middleware in `pkg/features/caching.go` appears to have a design flaw:
   ```go
   // executeAndCapture returns ctx.Response.Body after handler execution
   return ctx.Response.Body, nil  // This often returns nil
   
   // Then serveResult tries to write the response again
   return ctx.JSON(result)  // This would fail if response was already written
   ```

### Impact

This limitation affects several critical use cases:
- **Idempotency**: Cannot cache successful responses for duplicate request handling
- **Response Caching**: Cannot reliably cache response bodies
- **Response Logging/Auditing**: Cannot log complete responses
- **Response Transformation**: Cannot modify responses after handler execution
- **Metrics Collection**: Cannot measure response sizes or content

## Proposed Solution

### Option 1: Response Buffer Wrapper (Recommended)

Implement a response buffer that captures data before it's written to the underlying response writer.

```go
// pkg/lift/response_buffer.go
type ResponseBuffer struct {
    *Response
    body       []byte
    statusCode int
    written    bool
    mu         sync.Mutex
}

func (r *ResponseBuffer) JSON(data any) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    if r.written {
        return NewLiftError("RESPONSE_WRITTEN", "Response has already been written", 500)
    }
    
    // Marshal data
    jsonData, err := json.Marshal(data)
    if err != nil {
        return err
    }
    
    // Store in buffer
    r.body = jsonData
    r.Header("Content-Type", "application/json")
    
    // Store the original data as well
    r.Response.Body = data
    
    // Mark as written but don't actually write yet
    r.written = true
    return nil
}

func (r *ResponseBuffer) Flush() error {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    if !r.written {
        return nil
    }
    
    // Now actually write to the underlying response
    return r.Response.writeBody(r.body)
}
```

### Option 2: Handler Wrapper Pattern

Create a handler wrapper that intercepts the response at the handler level.

```go
// pkg/lift/interceptor.go
type ResponseInterceptor interface {
    BeforeResponse(ctx *Context, data any, statusCode int) (any, int, error)
    AfterResponse(ctx *Context, data any, statusCode int, err error)
}

func InterceptingHandler(handler Handler, interceptor ResponseInterceptor) Handler {
    return HandlerFunc(func(ctx *Context) error {
        // Create wrapped context with intercepting methods
        wrapped := &InterceptingContext{
            Context:     ctx,
            interceptor: interceptor,
        }
        
        // Execute handler
        err := handler.Handle(wrapped)
        
        // Flush any buffered response
        if flusher, ok := wrapped.Response.(interface{ Flush() error }); ok {
            if flushErr := flusher.Flush(); flushErr != nil && err == nil {
                err = flushErr
            }
        }
        
        return err
    })
}
```

### Option 3: Context Enhancement

Enhance the Context to support response callbacks.

```go
// pkg/lift/context.go
type Context struct {
    // ... existing fields ...
    
    responseCallbacks []func(data any, statusCode int)
}

func (c *Context) OnResponse(callback func(data any, statusCode int)) {
    c.responseCallbacks = append(c.responseCallbacks, callback)
}

func (c *Context) JSON(data any) error {
    // Execute callbacks before writing
    for _, cb := range c.responseCallbacks {
        cb(data, c.Response.StatusCode)
    }
    
    return c.Response.JSON(data)
}
```

## Implementation Recommendations

### 1. Immediate Fix: Response Buffer

Implement the ResponseBuffer approach as it:
- Requires minimal changes to existing code
- Maintains backward compatibility
- Provides full response access to middleware

### 2. Framework Enhancement

Update the core framework to use response buffering by default:

```go
// pkg/lift/app.go
func (a *App) executeHandler(ctx *Context, handler Handler) error {
    // Wrap response with buffer if middleware needs interception
    if a.hasInterceptingMiddleware {
        buffer := NewResponseBuffer(ctx.Response)
        ctx.Response = buffer
        defer buffer.Flush()
    }
    
    return handler.Handle(ctx)
}
```

### 3. Update Existing Middleware

Fix the caching middleware to properly use the response interception:

```go
func (c *CacheMiddleware) Handle(ctx *lift.Context, next lift.Handler) error {
    // ... cache lookup logic ...
    
    // Use response buffer
    buffer := lift.NewResponseBuffer(ctx.Response)
    ctx.Response = buffer
    
    // Execute handler
    err := next.Handle(ctx)
    if err != nil {
        return err
    }
    
    // Access captured response
    if buffer.Body != nil && c.config.ShouldCache(ctx, buffer.Body) {
        c.store.Set(ctx.Context, key, buffer.Body, c.config.DefaultTTL)
    }
    
    // Flush response to client
    return buffer.Flush()
}
```

## Migration Strategy

1. **Phase 1**: Implement ResponseBuffer as an opt-in feature
2. **Phase 2**: Update documentation and examples
3. **Phase 3**: Migrate core middleware to use ResponseBuffer
4. **Phase 4**: Make ResponseBuffer the default for middleware chains

## Testing Requirements

1. Unit tests for ResponseBuffer implementation
2. Integration tests for middleware using response interception
3. Performance benchmarks to ensure minimal overhead
4. Compatibility tests with existing handlers

## Detailed Implementation Guide

### Step 1: Add Response Buffering to Core

Create a new file `pkg/lift/response_buffer.go` that implements buffered response writing:

1. Define the ResponseBuffer struct that wraps the existing Response
2. Implement all Response methods to buffer data instead of writing immediately
3. Add a Flush() method that writes buffered data to the actual response
4. Ensure thread safety with proper mutex usage

### Step 2: Modify Context Creation

Update `pkg/lift/context.go` to support response buffering:

1. Add a field to track if response buffering is enabled
2. Modify NewContext to optionally wrap Response with ResponseBuffer
3. Update the JSON, Text, HTML, and Binary methods to handle buffering

### Step 3: Update Middleware Chain Execution

Modify `pkg/lift/app.go` to handle response buffering:

1. Detect if any middleware in the chain needs response interception
2. Automatically wrap responses with ResponseBuffer when needed
3. Ensure proper flushing of buffered responses after handler execution

### Step 4: Create Middleware Interface

Define a new interface for middleware that needs response interception:

```go
type ResponseInterceptingMiddleware interface {
    NeedsResponseInterception() bool
}
```

### Step 5: Update Existing Middleware

1. Fix the caching middleware to properly use response interception
2. Update any other middleware that attempts to access response bodies
3. Add the ResponseInterceptingMiddleware interface where needed

## Implementation Priority

1. **Critical**: Fix the fundamental issue where `ctx.Response.Body` is not populated
2. **High**: Implement response buffering infrastructure
3. **Medium**: Update existing middleware to use the new pattern
4. **Low**: Add convenience methods and helpers

## Backward Compatibility Requirements

1. Existing handlers must continue to work without modification
2. Response buffering should be opt-in at the middleware level
3. Performance impact must be minimal for non-intercepting middleware
4. API surface changes should be additive only

## Testing Strategy

### Unit Tests Required

1. Test ResponseBuffer correctly buffers all response types (JSON, Text, HTML, Binary)
2. Test thread safety of ResponseBuffer under concurrent access
3. Test that responses are properly flushed after handler execution
4. Test error handling when response is written multiple times

### Integration Tests Required

1. Test idempotency middleware with response buffering
2. Test caching middleware with response buffering
3. Test middleware chain with mixed intercepting/non-intercepting middleware
4. Test performance impact of response buffering

### Benchmarks Required

1. Measure overhead of response buffering vs direct writing
2. Compare memory usage with and without buffering
3. Test impact on request latency
4. Measure throughput impact under load

## Risk Mitigation

### Performance Risks

- **Risk**: Response buffering adds memory overhead
- **Mitigation**: Only buffer when middleware requires it, use object pooling for buffers

### Compatibility Risks

- **Risk**: Changes break existing middleware
- **Mitigation**: Make all changes backward compatible, extensive testing

### Complexity Risks

- **Risk**: Implementation adds complexity to core request handling
- **Mitigation**: Clear separation of concerns, comprehensive documentation

## Success Criteria

1. Idempotency middleware can successfully cache and replay responses
2. Caching middleware works correctly without double-writing responses
3. No performance regression for handlers not using response interception
4. All existing tests continue to pass
5. New middleware can be written that reliably accesses response data

## Conclusion

The lack of response interception in Lift is a significant limitation that prevents implementation of enterprise-grade middleware patterns. The proposed ResponseBuffer solution provides a clean, backward-compatible approach to address this gap while maintaining the framework's performance characteristics.

This enhancement would bring Lift to parity with other modern web frameworks and enable critical features like idempotency, which are essential for production API services.

**Next Steps**: 
1. Get approval for the proposed approach
2. Implement ResponseBuffer as a proof of concept
3. Update core framework to support response buffering
4. Migrate existing middleware to use the new pattern
5. Document the new capabilities for middleware authors