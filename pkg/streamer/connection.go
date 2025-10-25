package streamer

import "time"

// ConnectionInfo represents information about a WebSocket connection.
type ConnectionInfo struct {
	Identity     map[string]any
	ConnectedAt  time.Time
	LastActiveAt time.Time
	ConnectionID string
	SourceIP     string
	UserAgent    string
}

// ConnectionState represents the state of a WebSocket connection.
type ConnectionState string

const (
	// ConnectionStateActive indicates an active, connected WebSocket
	ConnectionStateActive ConnectionState = "ACTIVE"

	// ConnectionStateDisconnected indicates a disconnected WebSocket
	ConnectionStateDisconnected ConnectionState = "DISCONNECTED"

	// ConnectionStateStale indicates a connection that has expired
	ConnectionStateStale ConnectionState = "STALE"
)

// IsActive returns true if the connection is in an active state.
func (c ConnectionInfo) IsActive() bool {
	// If LastActiveAt is zero (not set by API Gateway), treat as active
	// This happens for newly created connections
	if c.LastActiveAt.IsZero() {
		return true
	}

	// Connection is considered active if LastActiveAt is recent
	// This is a heuristic - actual state is tracked by AWS
	return time.Since(c.LastActiveAt) < 2*time.Hour
}

// Age returns how long the connection has been alive.
func (c ConnectionInfo) Age() time.Duration {
	// If ConnectedAt is zero, return 0 duration
	if c.ConnectedAt.IsZero() {
		return 0
	}
	return time.Since(c.ConnectedAt)
}

// IdleDuration returns how long the connection has been idle.
func (c ConnectionInfo) IdleDuration() time.Duration {
	// If LastActiveAt is zero, fall back to ConnectedAt
	timestamp := c.LastActiveAt
	if timestamp.IsZero() {
		timestamp = c.ConnectedAt
	}

	// If both are zero, return 0 duration
	if timestamp.IsZero() {
		return 0
	}

	return time.Since(timestamp)
}
