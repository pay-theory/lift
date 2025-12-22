package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/streamer"
)

// EventBusConnectionResolver resolves target connection IDs for a given EventBus event.
type EventBusConnectionResolver func(ctx context.Context, event *Event) ([]string, error)

// EventBusFanoutOptions configures FanoutEventBusEvent.
type EventBusFanoutOptions struct {
	// ResolveConnectionIDs is required and is used to pick target connections (IDs or subscriptions).
	ResolveConnectionIDs EventBusConnectionResolver

	// Encode builds the WebSocket payload. Defaults to DefaultEventBusFanoutMessage.
	Encode func(event *Event) ([]byte, error)

	// FailOnError causes FanoutEventBusEvent to return an error when any non-gone send fails.
	FailOnError bool
}

// EventBusFanoutResult summarizes a fanout attempt.
type EventBusFanoutResult struct {
	Attempted int
	Sent      int
	Gone      []string
	Failed    map[string]error
}

type eventBusFanoutEnvelope struct {
	EventType     string            `json:"event_type"`
	EventID       string            `json:"event_id"`
	TenantID      string            `json:"tenant_id,omitempty"`
	SourceID      string            `json:"source_id,omitempty"`
	CorrelationID string            `json:"correlation_id,omitempty"`
	Payload       json.RawMessage   `json:"payload,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	Tags          []string          `json:"tags,omitempty"`
}

// DefaultEventBusFanoutMessage encodes a small, stable envelope for pushing EventBus events to WebSockets.
func DefaultEventBusFanoutMessage(event *Event) ([]byte, error) {
	if event == nil {
		return nil, fmt.Errorf("event is required")
	}

	envelope := eventBusFanoutEnvelope{
		EventType:     event.EventType,
		EventID:       event.ID,
		TenantID:      event.TenantID,
		SourceID:      event.SourceID,
		CorrelationID: event.CorrelationID,
		Payload:       event.Payload,
		Metadata:      event.Metadata,
		Tags:          event.Tags,
	}

	return json.Marshal(envelope)
}

// TenantConnectionsResolver returns a resolver that targets all active connections for event.TenantID.
func TenantConnectionsResolver(store lift.ConnectionStore) EventBusConnectionResolver {
	return func(ctx context.Context, event *Event) ([]string, error) {
		if store == nil {
			return nil, fmt.Errorf("connection store is required")
		}
		if event == nil {
			return nil, fmt.Errorf("event is required")
		}
		if event.TenantID == "" {
			return nil, nil
		}

		conns, err := store.ListByTenant(ctx, event.TenantID)
		if err != nil {
			return nil, err
		}

		ids := make([]string, 0, len(conns))
		for _, conn := range conns {
			if conn == nil || conn.ID == "" {
				continue
			}
			ids = append(ids, conn.ID)
		}

		return ids, nil
	}
}

// UserConnectionsResolverFromMetadata returns a resolver that targets all active connections for the user ID found in event.Metadata[metadataKey].
//
// This is useful for "user scoped" fanout where the EventBus event does not have a first-class user field.
func UserConnectionsResolverFromMetadata(store lift.ConnectionStore, metadataKey string) EventBusConnectionResolver {
	metadataKey = strings.TrimSpace(metadataKey)
	if metadataKey == "" {
		metadataKey = "user_id"
	}

	return func(ctx context.Context, event *Event) ([]string, error) {
		if store == nil {
			return nil, fmt.Errorf("connection store is required")
		}
		if event == nil {
			return nil, fmt.Errorf("event is required")
		}
		if event.Metadata == nil {
			return nil, nil
		}

		userID := strings.TrimSpace(event.Metadata[metadataKey])
		if userID == "" {
			return nil, nil
		}

		conns, err := store.ListByUser(ctx, userID)
		if err != nil {
			return nil, err
		}

		ids := make([]string, 0, len(conns))
		for _, conn := range conns {
			if conn == nil || conn.ID == "" {
				continue
			}
			ids = append(ids, conn.ID)
		}

		return ids, nil
	}
}

// TopicConnectionsResolver returns a resolver that targets all subscribed connections for topics derived from an EventBus event.
//
// The provided topics function should return application-defined topic strings (e.g. "home", "public", "hashtag:golang").
func TopicConnectionsResolver(store lift.SubscriptionStore, topics func(event *Event) []string) EventBusConnectionResolver {
	return func(ctx context.Context, event *Event) ([]string, error) {
		if store == nil {
			return nil, fmt.Errorf("subscription store is required")
		}
		if event == nil {
			return nil, fmt.Errorf("event is required")
		}
		if event.TenantID == "" {
			return nil, nil
		}
		if topics == nil {
			return nil, fmt.Errorf("topics function is required")
		}

		resolvedTopics := topics(event)
		if len(resolvedTopics) == 0 {
			return nil, nil
		}

		seen := make(map[string]struct{})
		out := make([]string, 0)
		for _, topic := range resolvedTopics {
			topic = strings.TrimSpace(topic)
			if topic == "" {
				continue
			}

			connectionIDs, err := store.ListByTopic(ctx, event.TenantID, topic)
			if err != nil {
				return nil, err
			}

			for _, connectionID := range connectionIDs {
				connectionID = strings.TrimSpace(connectionID)
				if connectionID == "" {
					continue
				}
				if _, ok := seen[connectionID]; ok {
					continue
				}
				seen[connectionID] = struct{}{}
				out = append(out, connectionID)
			}
		}

		return out, nil
	}
}

// UnionConnectionsResolver unions multiple resolvers and returns de-duplicated connection IDs.
func UnionConnectionsResolver(resolvers ...EventBusConnectionResolver) EventBusConnectionResolver {
	return func(ctx context.Context, event *Event) ([]string, error) {
		seen := make(map[string]struct{})
		out := make([]string, 0)

		for _, resolver := range resolvers {
			if resolver == nil {
				continue
			}
			ids, err := resolver(ctx, event)
			if err != nil {
				return nil, err
			}
			for _, id := range ids {
				id = strings.TrimSpace(id)
				if id == "" {
					continue
				}
				if _, ok := seen[id]; ok {
					continue
				}
				seen[id] = struct{}{}
				out = append(out, id)
			}
		}

		return out, nil
	}
}

// FanoutEventBusEvent sends a WebSocket message derived from an EventBus event to resolved connection IDs.
//
// This helper is designed for "EventBus stream processor → push to sockets" flows.
// Gone connections are collected in the result and do not count as failures.
func FanoutEventBusEvent(ctx context.Context, client streamer.Client, event *Event, opts EventBusFanoutOptions) (EventBusFanoutResult, error) {
	if client == nil {
		return EventBusFanoutResult{}, fmt.Errorf("streamer client is required")
	}
	if event == nil {
		return EventBusFanoutResult{}, fmt.Errorf("event is required")
	}
	if opts.ResolveConnectionIDs == nil {
		return EventBusFanoutResult{}, fmt.Errorf("ResolveConnectionIDs is required")
	}

	encode := opts.Encode
	if encode == nil {
		encode = DefaultEventBusFanoutMessage
	}

	connectionIDs, err := opts.ResolveConnectionIDs(ctx, event)
	if err != nil {
		return EventBusFanoutResult{}, err
	}

	data, err := encode(event)
	if err != nil {
		return EventBusFanoutResult{}, err
	}

	result := EventBusFanoutResult{
		Attempted: len(connectionIDs),
	}

	for _, connectionID := range connectionIDs {
		if connectionID == "" {
			continue
		}

		if err := client.PostToConnection(ctx, connectionID, data); err != nil {
			if errors.Is(err, streamer.ErrConnectionGone) {
				result.Gone = append(result.Gone, connectionID)
				continue
			}

			if result.Failed == nil {
				result.Failed = make(map[string]error)
			}
			result.Failed[connectionID] = err
			continue
		}

		result.Sent++
	}

	if opts.FailOnError && len(result.Failed) > 0 {
		return result, fmt.Errorf("failed to fanout to %d connections", len(result.Failed))
	}

	return result, nil
}
