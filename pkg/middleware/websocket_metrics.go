package middleware

import (
	"strings"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
)

// WebSocketMetrics creates metrics middleware for WebSocket operations
func WebSocketMetrics(metrics lift.MetricsCollector) lift.Middleware {
	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			// Check if this is a WebSocket context
			wsCtx, err := ctx.AsWebSocket()
			if err != nil {
				// Not a WebSocket context, pass through
				return next.Handle(ctx)
			}

			start := time.Now()
			routeKey := wsCtx.RouteKey()
			eventType := wsCtx.EventType()
			connectionID := wsCtx.ConnectionID()

			// Add metadata to context for tracing
			ctx.Set("websocket.route_key", routeKey)
			ctx.Set("websocket.event_type", eventType)
			ctx.Set("websocket.connection_id", connectionID)

			// Process request
			err = next.Handle(ctx)

			// Record metrics
			duration := time.Since(start)

			dimensions := map[string]string{
				"route_key":  routeKey,
				"event_type": eventType,
			}

			if err != nil {
				dimensions["error"] = "true"
				dimensions["error_type"] = getErrorType(err)
				metrics.Counter("websocket.errors", dimensions).Inc()
			}

			metrics.Histogram("websocket.latency", dimensions).Observe(duration.Seconds())
			metrics.Counter("websocket.requests", dimensions).Inc()

			// Connection-specific metrics
			switch eventType {
			case "CONNECT":
				metrics.Counter("websocket.connections.new", dimensions).Inc()
				if err == nil {
					metrics.Gauge("websocket.connections.active", dimensions).Inc()
				}
			case "DISCONNECT":
				metrics.Counter("websocket.connections.closed", dimensions).Inc()
				metrics.Gauge("websocket.connections.active", dimensions).Dec()
			case "MESSAGE":
				metrics.Counter("websocket.messages", dimensions).Inc()
				// Track message size if available
				if ctx.Request != nil && len(ctx.Request.Body) > 0 {
					metrics.Histogram("websocket.message.size", dimensions).Observe(float64(len(ctx.Request.Body)))
				}
			}

			// Log the request
			if ctx.Logger != nil {
				logFields := map[string]interface{}{
					"route_key":     routeKey,
					"event_type":    eventType,
					"connection_id": connectionID,
					"duration_ms":   duration.Milliseconds(),
				}

				if err != nil {
					logFields["error"] = err.Error()
					ctx.Logger.Error("WebSocket request failed", logFields)
				} else {
					ctx.Logger.Info("WebSocket request completed", logFields)
				}
			}

			return err
		})
	}
}

// WebSocketConnectionMetrics creates middleware that tracks connection lifecycle
func WebSocketConnectionMetrics(metrics lift.MetricsCollector, store lift.ConnectionStore) lift.Middleware {
	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			wsCtx, err := ctx.AsWebSocket()
			if err != nil {
				return next.Handle(ctx)
			}

			// Track connection count periodically
			if wsCtx.IsConnectEvent() {
				go func() {
					// Count active connections every minute
					ticker := time.NewTicker(1 * time.Minute)
					defer ticker.Stop()

					for range ticker.C {
						if store != nil {
							// This would need to be implemented in the store
							// count, err := store.CountActive(context.Background())
							// if err == nil {
							//     metrics.RecordGauge("websocket.connections.total", float64(count), nil)
							// }
						}
					}
				}()
			}

			return next.Handle(ctx)
		})
	}
}

// getErrorType extracts a categorized error type for metrics
func getErrorType(err error) string {
	if err == nil {
		return ""
	}

	// Check for Lift errors
	if liftErr, ok := err.(*lift.LiftError); ok {
		return liftErr.Code
	}

	// Categorize by error message patterns
	errStr := err.Error()
	switch {
	case contains(errStr, "unauthorized", "authentication", "401"):
		return "unauthorized"
	case contains(errStr, "forbidden", "permission", "403"):
		return "forbidden"
	case contains(errStr, "not found", "404"):
		return "not_found"
	case contains(errStr, "timeout"):
		return "timeout"
	case contains(errStr, "connection"):
		return "connection_error"
	default:
		return "internal_error"
	}
}

// contains checks if any of the substrings exist in the string (case-insensitive)
func contains(s string, substrs ...string) bool {
	s = strings.ToLower(s)
	for _, substr := range substrs {
		if strings.Contains(s, strings.ToLower(substr)) {
			return true
		}
	}
	return false
}
