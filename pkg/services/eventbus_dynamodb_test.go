package services

import (
	"context"
	"errors"
	"testing"
	"time"

	dynamormerrors "github.com/pay-theory/dynamorm/pkg/errors"
	dynamormmocks "github.com/pay-theory/dynamorm/pkg/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/pay-theory/dynamorm/pkg/core"
	liftmocks "github.com/pay-theory/lift/pkg/dynamorm/mocks"
)

func TestNewDynamoDBEventBus_DefaultsAndOverrides(t *testing.T) {
	resetEventBusTableNameOverride(t)
	t.Cleanup(func() { resetEventBusTableNameOverride(t) })

	db := liftmocks.NewMockExtendedDB()

	bus := NewDynamoDBEventBus(db, EventBusConfig{})
	require.NotNil(t, bus)
	require.NotNil(t, bus.handlers)
	require.NotZero(t, bus.config.TTL)
	require.NotZero(t, bus.config.RetryAttempts)
	require.NotZero(t, bus.config.RetryBaseDelay)
	require.NotZero(t, bus.config.MaxBatchSize)
	require.NotEmpty(t, bus.config.MetricsNamespace)
	require.NotEmpty(t, bus.config.TableName)

	// Override table name for the process lifetime.
	overrideBus := NewDynamoDBEventBus(db, EventBusConfig{TableName: "custom-table"})
	require.Equal(t, "custom-table", overrideBus.config.TableName)
	require.Equal(t, "custom-table", (&Event{}).TableName())
}

func TestDynamoDBEventBus_WithCloudWatch_EnablesMetrics(t *testing.T) {
	db := liftmocks.NewMockExtendedDB()
	bus := NewDynamoDBEventBus(db, EventBusConfig{})

	bus.WithCloudWatch(nil)
	require.True(t, bus.config.EnableMetrics)
}

func TestDynamoDBEventBus_Publish_SuccessRetryAndDedup(t *testing.T) {
	t.Run("nil event", func(t *testing.T) {
		db := liftmocks.NewMockExtendedDB()
		bus := NewDynamoDBEventBus(db, EventBusConfig{})
		_, err := bus.Publish(context.Background(), nil)
		require.Error(t, err)
	})

	t.Run("success creates missing fields", func(t *testing.T) {
		db := liftmocks.NewMockExtendedDB()
		query := new(dynamormmocks.MockQuery)

		db.On("WithContext", mock.Anything).Return(db).Once()
		db.On("Model", mock.Anything).Return(query).Once()
		query.On("IfNotExists").Return(query).Once()
		query.On("Create").Return(nil).Once()

		bus := NewDynamoDBEventBus(db, EventBusConfig{
			TTL:              time.Hour,
			RetryAttempts:    1,
			RetryBaseDelay:   1 * time.Nanosecond,
			MetricsNamespace: "Lift/EventBus",
		})

		event := &Event{
			EventType: "partner.created",
			TenantID:  "tenant-1",
		}

		id, err := bus.Publish(context.Background(), event)
		require.NoError(t, err)
		require.NotEmpty(t, id)
		require.Equal(t, id, event.ID)
		require.Equal(t, "tenant-1#partner.created", event.PartitionKey)
		require.NotEmpty(t, event.SortKey)
		require.NotZero(t, event.PublishedAt)
		require.NotZero(t, event.CreatedAt)
		require.NotZero(t, event.ExpiresAt)
		require.Equal(t, event.ExpiresAt.Unix(), event.TTL)

		db.AssertExpectations(t)
		query.AssertExpectations(t)
	})

	t.Run("deduped publish treats condition failure as success", func(t *testing.T) {
		db := liftmocks.NewMockExtendedDB()
		query := new(dynamormmocks.MockQuery)

		db.On("WithContext", mock.Anything).Return(db).Once()
		db.On("Model", mock.Anything).Return(query).Once()
		query.On("IfNotExists").Return(query).Once()
		query.On("Create").Return(dynamormerrors.ErrConditionFailed).Once()

		bus := NewDynamoDBEventBus(db, EventBusConfig{})
		event := &Event{EventType: "evt", TenantID: "tenant-1"}

		id, err := bus.Publish(context.Background(), event)
		require.NoError(t, err)
		require.Equal(t, event.ID, id)
	})

	t.Run("retries on retryable errors", func(t *testing.T) {
		db := liftmocks.NewMockExtendedDB()
		query := new(dynamormmocks.MockQuery)

		db.On("WithContext", mock.Anything).Return(db).Twice()
		db.On("Model", mock.Anything).Return(query).Twice()
		query.On("IfNotExists").Return(query).Twice()
		query.On("Create").Return(errors.New("ProvisionedThroughputExceededException")).Once()
		query.On("Create").Return(nil).Once()

		bus := NewDynamoDBEventBus(db, EventBusConfig{
			RetryAttempts:  1,
			RetryBaseDelay: 1 * time.Nanosecond,
		})

		event := &Event{EventType: "evt", TenantID: "tenant-1"}
		_, err := bus.Publish(context.Background(), event)
		require.NoError(t, err)

		db.AssertExpectations(t)
		query.AssertExpectations(t)
	})

	t.Run("does not retry on non-retryable errors", func(t *testing.T) {
		db := liftmocks.NewMockExtendedDB()
		query := new(dynamormmocks.MockQuery)

		db.On("WithContext", mock.Anything).Return(db).Once()
		db.On("Model", mock.Anything).Return(query).Once()
		query.On("IfNotExists").Return(query).Once()
		query.On("Create").Return(errors.New("validation error")).Once()

		bus := NewDynamoDBEventBus(db, EventBusConfig{RetryAttempts: 3})
		event := &Event{EventType: "evt", TenantID: "tenant-1"}

		_, err := bus.Publish(context.Background(), event)
		require.Error(t, err)
	})
}

func TestDynamoDBEventBus_Query_GSIAndPartitionKeyModes(t *testing.T) {
	t.Run("validates inputs", func(t *testing.T) {
		db := liftmocks.NewMockExtendedDB()
		bus := NewDynamoDBEventBus(db, EventBusConfig{})
		_, err := bus.Query(context.Background(), nil)
		require.Error(t, err)
		_, err = bus.Query(context.Background(), &EventQuery{})
		require.Error(t, err)
	})

	t.Run("tenant-wide uses GSI and supports pagination cursor", func(t *testing.T) {
		db := liftmocks.NewMockExtendedDB()
		query := new(dynamormmocks.MockQuery)

		start := time.Unix(1_700_000_000, 0).UTC()
		end := start.Add(time.Hour)

		db.On("WithContext", mock.Anything).Return(db).Once()
		db.On("Model", mock.Anything).Return(query).Once()

		query.On("Index", "tenant-timestamp-index").Return(query).Once()
		query.On("Where", "TenantID", "=", "tenant-1").Return(query).Once()
		query.On("Where", "PublishedAt", "BETWEEN", mock.Anything).Return(query).Once()
		query.On("OrderBy", "PublishedAt", "DESC").Return(query).Once()
		query.On("Filter", "Tags", "CONTAINS", "tag-1").Return(query).Once()
		query.On("Limit", 10).Return(query).Once()
		query.On("Cursor", "cur").Return(query).Once()

		query.On("AllPaginated", mock.Anything).Run(func(args mock.Arguments) {
			dest := args.Get(0).(*[]*Event)
			*dest = []*Event{{ID: "evt_1"}}
		}).Return(&core.PaginatedResult{HasMore: true, NextCursor: "next"}, nil).Once()

		bus := NewDynamoDBEventBus(db, EventBusConfig{})
		q := &EventQuery{
			TenantID:         "tenant-1",
			StartTime:        &start,
			EndTime:          &end,
			Tags:             []string{"tag-1", ""},
			Limit:            10,
			LastEvaluatedKey: map[string]interface{}{"cursor": "cur"},
			NextKey:          map[string]interface{}{"cursor": "stale"},
			EventType:        "",
		}

		events, err := bus.Query(context.Background(), q)
		require.NoError(t, err)
		require.Len(t, events, 1)
		require.Equal(t, map[string]interface{}{"cursor": "next"}, q.NextKey)
	})

	t.Run("event-type uses main table and defaults limit", func(t *testing.T) {
		db := liftmocks.NewMockExtendedDB()
		query := new(dynamormmocks.MockQuery)

		start := time.Unix(1_700_000_000, 0).UTC()

		db.On("WithContext", mock.Anything).Return(db).Once()
		db.On("Model", mock.Anything).Return(query).Once()

		query.On("Where", "PartitionKey", "=", "tenant-1#partner.created").Return(query).Once()
		query.On("Where", "SortKey", ">=", mock.Anything).Return(query).Once()
		query.On("OrderBy", "SortKey", "DESC").Return(query).Once()
		query.On("Limit", 100).Return(query).Once()
		query.On("AllPaginated", mock.Anything).Return(&core.PaginatedResult{HasMore: false}, nil).Once()

		bus := NewDynamoDBEventBus(db, EventBusConfig{})
		q := &EventQuery{
			TenantID:  "tenant-1",
			EventType: "partner.created",
			StartTime: &start,
		}

		events, err := bus.Query(context.Background(), q)
		require.NoError(t, err)
		require.Len(t, events, 0)
		require.Nil(t, q.NextKey)
	})

	t.Run("query errors are wrapped", func(t *testing.T) {
		db := liftmocks.NewMockExtendedDB()
		query := new(dynamormmocks.MockQuery)

		db.On("WithContext", mock.Anything).Return(db).Once()
		db.On("Model", mock.Anything).Return(query).Once()
		query.On("Index", mock.Anything).Return(query).Once()
		query.On("Where", mock.Anything, mock.Anything, mock.Anything).Return(query)
		query.On("OrderBy", mock.Anything, mock.Anything).Return(query).Once()
		query.On("Limit", mock.Anything).Return(query).Once()
		query.On("AllPaginated", mock.Anything).Return((*core.PaginatedResult)(nil), errors.New("boom")).Once()

		bus := NewDynamoDBEventBus(db, EventBusConfig{})
		_, err := bus.Query(context.Background(), &EventQuery{TenantID: "tenant-1"})
		require.Error(t, err)
	})
}

func TestDynamoDBEventBus_SubscribeGetDeleteAndBatchPublish(t *testing.T) {
	db := liftmocks.NewMockExtendedDB()
	bus := NewDynamoDBEventBus(db, EventBusConfig{})

	require.Error(t, bus.Subscribe(context.Background(), "", func(context.Context, *Event) error { return nil }))
	require.Error(t, bus.Subscribe(context.Background(), "evt", nil))
	require.NoError(t, bus.Subscribe(context.Background(), "evt", func(context.Context, *Event) error { return nil }))
	require.Len(t, bus.handlers["evt"], 1)

	_, err := bus.GetEvent(context.Background(), "")
	require.Error(t, err)

	// GetEvent success.
	queryGet := new(dynamormmocks.MockQuery)
	db.On("WithContext", mock.Anything).Return(db)
	db.On("Model", mock.Anything).Return(queryGet).Once()
	queryGet.On("Index", "event-id-index").Return(queryGet).Once()
	queryGet.On("Where", "ID", "=", "evt_123").Return(queryGet).Once()
	queryGet.On("First", mock.Anything).Run(func(args mock.Arguments) {
		dest := args.Get(0).(*Event)
		dest.ID = "evt_123"
		dest.PartitionKey = "tenant-1#partner.created"
		dest.SortKey = "1#evt_123"
	}).Return(nil).Once()

	got, err := bus.GetEvent(context.Background(), "evt_123")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "evt_123", got.ID)

	// DeleteEvent success: GetEvent + Delete by PK/SK.
	db2 := liftmocks.NewMockExtendedDB()
	queryLookup := new(dynamormmocks.MockQuery)
	queryDelete := new(dynamormmocks.MockQuery)

	db2.On("WithContext", mock.Anything).Return(db2)
	db2.On("Model", mock.Anything).Return(queryLookup).Once()
	db2.On("Model", mock.Anything).Return(queryDelete).Once()

	queryLookup.On("Index", mock.Anything).Return(queryLookup).Once()
	queryLookup.On("Where", mock.Anything, mock.Anything, mock.Anything).Return(queryLookup)
	queryLookup.On("First", mock.Anything).Run(func(args mock.Arguments) {
		dest := args.Get(0).(*Event)
		dest.ID = "evt_123"
		dest.PartitionKey = "pk"
		dest.SortKey = "sk"
	}).Return(nil).Once()

	queryDelete.On("Where", "PartitionKey", "=", "pk").Return(queryDelete).Once()
	queryDelete.On("Where", "SortKey", "=", "sk").Return(queryDelete).Once()
	queryDelete.On("Delete").Return(nil).Once()

	bus2 := NewDynamoDBEventBus(db2, EventBusConfig{})
	require.NoError(t, bus2.DeleteEvent(context.Background(), "evt_123"))

	// BatchPublish validations and success.
	_, err = bus.BatchPublish(context.Background(), nil)
	require.Error(t, err)

	e1 := &Event{TenantID: "tenant-1", EventType: "evt"}
	ids, err := bus.BatchPublish(context.Background(), []*Event{e1, nil})
	require.Error(t, err)
	require.Len(t, ids, 1)

	db3 := liftmocks.NewMockExtendedDB()
	queryBatch := new(dynamormmocks.MockQuery)
	db3.On("WithContext", mock.Anything).Return(db3).Once()
	db3.On("Model", mock.Anything).Return(queryBatch).Once()
	queryBatch.On("BatchCreate", mock.Anything).Return(nil).Once()

	bus3 := NewDynamoDBEventBus(db3, EventBusConfig{TTL: time.Hour})
	events := []*Event{
		{TenantID: "tenant-1", EventType: "evt"},
		{TenantID: "tenant-1", EventType: "evt"},
	}
	ids, err = bus3.BatchPublish(context.Background(), events)
	require.NoError(t, err)
	require.Len(t, ids, 2)
	require.NotEmpty(t, events[0].ID)
	require.NotEmpty(t, events[1].ID)
}
