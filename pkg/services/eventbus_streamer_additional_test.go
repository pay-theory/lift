package services

import (
	"context"
	"errors"
	"testing"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/streamer"
	lifttesting "github.com/pay-theory/lift/pkg/testing"
	"github.com/stretchr/testify/require"
)

func TestUnionConnectionsResolver_DedupesAndTrims(t *testing.T) {
	r1 := func(context.Context, *Event) ([]string, error) {
		return []string{" c1 ", "c2", ""}, nil
	}
	r2 := func(context.Context, *Event) ([]string, error) {
		return []string{"c2", "c3", "   "}, nil
	}

	resolver := UnionConnectionsResolver(nil, r1, r2)
	ids, err := resolver(context.Background(), &Event{TenantID: "tenant-1"})
	require.NoError(t, err)
	require.Equal(t, []string{"c1", "c2", "c3"}, ids)
}

func TestUnionConnectionsResolver_PropagatesErrors(t *testing.T) {
	sentinel := errors.New("boom")
	resolver := UnionConnectionsResolver(func(context.Context, *Event) ([]string, error) {
		return nil, sentinel
	})

	_, err := resolver(context.Background(), &Event{})
	require.ErrorIs(t, err, sentinel)
}

func TestEventBusStreamerResolvers_ValidationAndEmptyCases(t *testing.T) {
	t.Run("DefaultEventBusFanoutMessage requires event", func(t *testing.T) {
		_, err := DefaultEventBusFanoutMessage(nil)
		require.Error(t, err)
	})

	t.Run("TenantConnectionsResolver validates inputs and skips empty tenant", func(t *testing.T) {
		resolver := TenantConnectionsResolver(nil)
		_, err := resolver(context.Background(), &Event{TenantID: "tenant-1"})
		require.Error(t, err)

		store := connectionStoreStub{
			listByTenant: func(context.Context, string) ([]*lift.Connection, error) {
				return []*lift.Connection{
					nil,
					{ID: ""},
					{ID: "c1"},
				}, nil
			},
		}
		resolver = TenantConnectionsResolver(store)

		ids, err := resolver(context.Background(), &Event{TenantID: ""})
		require.NoError(t, err)
		require.Empty(t, ids)

		ids, err = resolver(context.Background(), &Event{TenantID: "tenant-1"})
		require.NoError(t, err)
		require.Equal(t, []string{"c1"}, ids)
	})

	t.Run("UserConnectionsResolverFromMetadata defaults key and handles empty metadata/user", func(t *testing.T) {
		store := connectionStoreStub{
			listByUser: func(context.Context, string) ([]*lift.Connection, error) {
				return []*lift.Connection{{ID: "c1"}}, nil
			},
		}

		resolver := UserConnectionsResolverFromMetadata(store, "  ")
		ids, err := resolver(context.Background(), &Event{})
		require.NoError(t, err)
		require.Empty(t, ids)

		ids, err = resolver(context.Background(), &Event{Metadata: map[string]string{"user_id": " "}})
		require.NoError(t, err)
		require.Empty(t, ids)

		ids, err = resolver(context.Background(), &Event{Metadata: map[string]string{"user_id": "user-1"}})
		require.NoError(t, err)
		require.Equal(t, []string{"c1"}, ids)
	})

	t.Run("TopicConnectionsResolver validates inputs and handles empty topics", func(t *testing.T) {
		resolver := TopicConnectionsResolver(nil, func(*Event) []string { return nil })
		_, err := resolver(context.Background(), &Event{TenantID: "tenant-1"})
		require.Error(t, err)

		store := subscriptionStoreStub{}
		resolver = TopicConnectionsResolver(store, nil)
		_, err = resolver(context.Background(), &Event{TenantID: "tenant-1"})
		require.Error(t, err)

		resolver = TopicConnectionsResolver(store, func(*Event) []string { return nil })
		ids, err := resolver(context.Background(), &Event{TenantID: "tenant-1"})
		require.NoError(t, err)
		require.Empty(t, ids)

		store.listByTopic = func(context.Context, string, string) ([]string, error) {
			return []string{" c1 ", "c1", "", "c2"}, nil
		}
		resolver = TopicConnectionsResolver(store, func(*Event) []string {
			return []string{"home", " ", "home"}
		})
		ids, err = resolver(context.Background(), &Event{TenantID: "tenant-1"})
		require.NoError(t, err)
		require.Equal(t, []string{"c1", "c2"}, ids)
	})
}

func TestFanoutEventBusEvent_ValidationAndEncodeAndResolverErrors(t *testing.T) {
	_, err := FanoutEventBusEvent(context.Background(), nil, &Event{}, EventBusFanoutOptions{})
	require.Error(t, err)

	_, err = FanoutEventBusEvent(context.Background(), lifttesting.NewStreamerClientMock(), nil, EventBusFanoutOptions{})
	require.Error(t, err)

	_, err = FanoutEventBusEvent(context.Background(), lifttesting.NewStreamerClientMock(), &Event{}, EventBusFanoutOptions{})
	require.Error(t, err)

	_, err = FanoutEventBusEvent(context.Background(), lifttesting.NewStreamerClientMock(), &Event{}, EventBusFanoutOptions{
		ResolveConnectionIDs: func(context.Context, *Event) ([]string, error) {
			return nil, errors.New("resolver failed")
		},
	})
	require.Error(t, err)

	_, err = FanoutEventBusEvent(context.Background(), lifttesting.NewStreamerClientMock(), &Event{}, EventBusFanoutOptions{
		ResolveConnectionIDs: func(context.Context, *Event) ([]string, error) { return []string{"c1"}, nil },
		Encode:               func(*Event) ([]byte, error) { return nil, errors.New("encode failed") },
	})
	require.Error(t, err)

	client := lifttesting.NewStreamerClientMock().
		WithConnection("c1", nil).
		WithConnection("c2", nil).
		WithForbiddenError("c2", "nope")

	result, err := FanoutEventBusEvent(context.Background(), client, &Event{ID: "e1", EventType: "evt"}, EventBusFanoutOptions{
		ResolveConnectionIDs: func(context.Context, *Event) ([]string, error) { return []string{"", "c1", "c2"}, nil },
		Encode:               func(*Event) ([]byte, error) { return []byte(`{}`), nil },
		FailOnError:          false,
	})
	require.NoError(t, err)
	require.Equal(t, 3, result.Attempted)
	require.Equal(t, 1, result.Sent)
	require.Contains(t, result.Failed, "c2")
}

type connectionStoreStub struct {
	listByTenant func(context.Context, string) ([]*lift.Connection, error)
	listByUser   func(context.Context, string) ([]*lift.Connection, error)
}

func (c connectionStoreStub) Save(context.Context, *lift.Connection) error { return nil }
func (c connectionStoreStub) Get(context.Context, string) (*lift.Connection, error) {
	return nil, nil
}
func (c connectionStoreStub) Delete(context.Context, string) error { return nil }
func (c connectionStoreStub) ListByUser(ctx context.Context, userID string) ([]*lift.Connection, error) {
	if c.listByUser == nil {
		return nil, nil
	}
	return c.listByUser(ctx, userID)
}
func (c connectionStoreStub) ListByTenant(ctx context.Context, tenantID string) ([]*lift.Connection, error) {
	if c.listByTenant == nil {
		return nil, nil
	}
	return c.listByTenant(ctx, tenantID)
}
func (c connectionStoreStub) CountActive(context.Context) (int64, error) { return 0, nil }

var _ lift.ConnectionStore = connectionStoreStub{}

type subscriptionStoreStub struct {
	listByTopic func(context.Context, string, string) ([]string, error)
}

func (subscriptionStoreStub) Subscribe(context.Context, string, string, string) error { return nil }
func (subscriptionStoreStub) Unsubscribe(context.Context, string, string, string) error {
	return nil
}
func (s subscriptionStoreStub) ListByTopic(ctx context.Context, tenantID string, topic string) ([]string, error) {
	if s.listByTopic == nil {
		return nil, nil
	}
	return s.listByTopic(ctx, tenantID, topic)
}
func (subscriptionStoreStub) ListByConnection(context.Context, string) ([]lift.Subscription, error) {
	return nil, nil
}
func (subscriptionStoreStub) DeleteByConnection(context.Context, string) (int, error) { return 0, nil }

var _ lift.SubscriptionStore = subscriptionStoreStub{}

var _ streamer.Client = (*lifttesting.StreamerClientMock)(nil)
