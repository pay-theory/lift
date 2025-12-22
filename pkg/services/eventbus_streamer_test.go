package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/pay-theory/lift/pkg/lift"
	lifttesting "github.com/pay-theory/lift/pkg/testing"
	"github.com/stretchr/testify/require"
)

func TestFanoutEventBusEvent_SendsAndTracksGone(t *testing.T) {
	client := lifttesting.NewStreamerClientMock().
		WithConnection("c1", nil)

	event := &Event{
		ID:        "evt_123",
		EventType: "partner.created",
		TenantID:  "tenant-1",
		Payload:   json.RawMessage(`{"ok":true}`),
	}

	result, err := FanoutEventBusEvent(context.Background(), client, event, EventBusFanoutOptions{
		ResolveConnectionIDs: func(context.Context, *Event) ([]string, error) {
			return []string{"c1", "c2"}, nil // c2 is missing → gone
		},
	})
	require.NoError(t, err)
	require.Equal(t, 2, result.Attempted)
	require.Equal(t, 1, result.Sent)
	require.Equal(t, []string{"c2"}, result.Gone)
	require.Empty(t, result.Failed)

	msgs := client.GetMessages("c1")
	require.Len(t, msgs, 1)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(msgs[0], &decoded))
	require.Equal(t, "partner.created", decoded["event_type"])
	require.Equal(t, "evt_123", decoded["event_id"])
}

func TestFanoutEventBusEvent_FailOnError(t *testing.T) {
	client := lifttesting.NewStreamerClientMock().
		WithConnection("c1", nil).
		WithConnection("c2", nil).
		WithForbiddenError("c2", "nope")

	event := &Event{
		ID:        "evt_123",
		EventType: "partner.created",
		TenantID:  "tenant-1",
		Payload:   json.RawMessage(`{"ok":true}`),
	}

	result, err := FanoutEventBusEvent(context.Background(), client, event, EventBusFanoutOptions{
		ResolveConnectionIDs: func(context.Context, *Event) ([]string, error) {
			return []string{"c1", "c2"}, nil
		},
		FailOnError: true,
	})
	require.Error(t, err)
	require.Equal(t, 2, result.Attempted)
	require.Equal(t, 1, result.Sent)
	require.Empty(t, result.Gone)
	require.Contains(t, result.Failed, "c2")
}

type staticConnectionStore struct {
	connections map[string]*lift.Connection
}

func (s *staticConnectionStore) Save(_ context.Context, conn *lift.Connection) error {
	s.connections[conn.ID] = conn
	return nil
}
func (s *staticConnectionStore) Get(_ context.Context, connectionID string) (*lift.Connection, error) {
	return s.connections[connectionID], nil
}
func (s *staticConnectionStore) Delete(_ context.Context, connectionID string) error {
	delete(s.connections, connectionID)
	return nil
}
func (s *staticConnectionStore) ListByUser(_ context.Context, userID string) ([]*lift.Connection, error) {
	var out []*lift.Connection
	for _, conn := range s.connections {
		if conn.UserID == userID {
			out = append(out, conn)
		}
	}
	return out, nil
}
func (s *staticConnectionStore) ListByTenant(_ context.Context, tenantID string) ([]*lift.Connection, error) {
	var out []*lift.Connection
	for _, conn := range s.connections {
		if conn.TenantID == tenantID {
			out = append(out, conn)
		}
	}
	return out, nil
}
func (s *staticConnectionStore) CountActive(_ context.Context) (int64, error) {
	return int64(len(s.connections)), nil
}

func TestTenantConnectionsResolver_ReturnsIDs(t *testing.T) {
	store := &staticConnectionStore{
		connections: map[string]*lift.Connection{
			"c1": {ID: "c1", TenantID: "tenant-1"},
			"c2": {ID: "c2", TenantID: "tenant-1"},
			"c3": {ID: "c3", TenantID: "tenant-2"},
		},
	}

	event := &Event{TenantID: "tenant-1"}
	resolver := TenantConnectionsResolver(store)

	ids, err := resolver(context.Background(), event)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"c1", "c2"}, ids)
}

func TestUserConnectionsResolverFromMetadata_ReturnsIDs(t *testing.T) {
	store := &staticConnectionStore{
		connections: map[string]*lift.Connection{
			"c1": {ID: "c1", UserID: "user-1"},
			"c2": {ID: "c2", UserID: "user-1"},
			"c3": {ID: "c3", UserID: "user-2"},
		},
	}

	event := &Event{
		Metadata: map[string]string{"user_id": "user-1"},
	}
	resolver := UserConnectionsResolverFromMetadata(store, "user_id")

	ids, err := resolver(context.Background(), event)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"c1", "c2"}, ids)
}

type staticSubscriptionStore struct {
	byTopic map[string][]string
}

func (s staticSubscriptionStore) Subscribe(context.Context, string, string, string) error {
	return nil
}
func (s staticSubscriptionStore) Unsubscribe(context.Context, string, string, string) error {
	return nil
}
func (s staticSubscriptionStore) ListByTopic(_ context.Context, tenantID string, topic string) ([]string, error) {
	return append([]string(nil), s.byTopic[tenantID+"#"+topic]...), nil
}
func (s staticSubscriptionStore) ListByConnection(context.Context, string) ([]lift.Subscription, error) {
	return nil, nil
}
func (s staticSubscriptionStore) DeleteByConnection(context.Context, string) (int, error) {
	return 0, nil
}

func TestTopicConnectionsResolver_Dedupes(t *testing.T) {
	subscriptions := staticSubscriptionStore{
		byTopic: map[string][]string{
			"tenant-1#home":   {"c1", "c2"},
			"tenant-1#public": {"c2", "c3"},
		},
	}

	event := &Event{
		TenantID: "tenant-1",
	}

	resolver := TopicConnectionsResolver(subscriptions, func(*Event) []string {
		return []string{"home", "public"}
	})

	ids, err := resolver(context.Background(), event)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"c1", "c2", "c3"}, ids)
}
