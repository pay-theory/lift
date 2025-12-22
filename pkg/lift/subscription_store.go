package lift

import "context"

// Subscription represents a WebSocket subscription for fanout (topic → connection).
//
// Subscriptions are typically stored in the same DynamoDB table as WebSocket connections.
// Topic semantics are application-defined (e.g. "tenant:broadcast", "stream:home", "hashtag:golang").
type Subscription struct {
	TenantID     string
	Topic        string
	ConnectionID string
}

// SubscriptionStore persists and queries WebSocket subscriptions without requiring scans.
//
// Implementations should support efficient:
//   - topic → connection IDs fanout (for publish → push patterns)
//   - connection → topics lookup (for cleanup on disconnect)
type SubscriptionStore interface {
	// Subscribe adds a connection to a topic. It should be idempotent.
	Subscribe(ctx context.Context, tenantID string, connectionID string, topic string) error
	// Unsubscribe removes a connection from a topic. It should be idempotent.
	Unsubscribe(ctx context.Context, tenantID string, connectionID string, topic string) error

	// ListByTopic returns connection IDs subscribed to a topic for a tenant.
	ListByTopic(ctx context.Context, tenantID string, topic string) ([]string, error)
	// ListByConnection returns subscriptions for a connection.
	ListByConnection(ctx context.Context, connectionID string) ([]Subscription, error)

	// DeleteByConnection removes all subscriptions for a connection and returns the number of items deleted.
	DeleteByConnection(ctx context.Context, connectionID string) (int, error)
}
