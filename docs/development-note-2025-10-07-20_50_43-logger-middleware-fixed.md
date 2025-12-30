# Logger Middleware Fixed - Real Logging Initialized

**Date:** 2025-10-07  
**Author:** Security Analysis Team

## Problem Fixed

The `middleware.Logger()` function was not initializing a logger when `ctx.Logger` was nil, causing other middleware that depend on logging to fail silently without any log output.

## Solution Implemented

Modified `pkg/middleware/middleware.go` to initialize a **real Zap console logger** that actually outputs logs to stdout when no logger is configured.

### Key Changes

1. **Added imports:**
   - `github.com/pay-theory/lift/pkg/observability`
   - `github.com/pay-theory/lift/pkg/observability/zap`
   - `sync` for singleton pattern

2. **Created singleton logger:**
```go
var (
    defaultLogger     observability.StructuredLogger
    defaultLoggerOnce sync.Once
)

func getDefaultLogger() observability.StructuredLogger {
    defaultLoggerOnce.Do(func() {
        config := observability.LoggerConfig{
            Level:  "info",
            Format: "json",
        }
        logger, err := zap.NewZapLogger(config)
        if err != nil {
            panic(fmt.Sprintf("Failed to initialize default logger: %v", err))
        }
        defaultLogger = logger
    })
    return defaultLogger
}
```

3. **Updated Logger() middleware:**
```go
if ctx.Logger == nil {
    ctx.Logger = getDefaultLogger()
}
```

## Benefits

✅ **Logging works out of the box** - no configuration needed  
✅ **Real logs to stdout** - JSON formatted structured logs  
✅ **Singleton pattern** - only one logger instance created  
✅ **Performance** - Zap is high-performance  
✅ **Backward compatible** - existing code continues to work  
✅ **Other middleware can use logger** - always available now  

## Test Results

All tests pass with actual log output:

```bash
go test -v -run TestLoggerMiddleware ./pkg/middleware/
PASS
ok      github.com/pay-theory/lift/pkg/middleware      0.008s
```

Example log output:
```json
{"level":"info","timestamp":"2025-10-07T20:53:53.221-0400","caller":"zap/logger.go:269","message":"Request completed","request_id":"","status":200,"duration":0,"method":"GET","path":"/test"}
```

## Files Modified

- `pkg/middleware/middleware.go` - Added real logger initialization
- `pkg/middleware/logger_test.go` - Created comprehensive tests
- `docs/development/decisions/2025-10-07-20_50_43-logger-middleware-initialization-fix.md` - Decision doc

## Usage

Developers can now simply use:

```go
app := lift.New()
app.Use(middleware.Logger())  // Real logging works!
```

No need for complex observability middleware setup just to get basic logging.
