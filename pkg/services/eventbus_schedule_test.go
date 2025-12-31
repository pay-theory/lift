package services

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	dynamormerrors "github.com/pay-theory/dynamorm/pkg/errors"
	dynamormmocks "github.com/pay-theory/dynamorm/pkg/mocks"
	liftmocks "github.com/pay-theory/lift/pkg/dynamorm/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestEventBusSchedule_CreatesOnce(t *testing.T) {
	db := liftmocks.NewMockExtendedDB()
	query := new(dynamormmocks.MockQuery)

	event := &Event{
		ID:        "evt_123",
		EventType: "partner.created",
		TenantID:  "tenant-1",
	}
	dueAt := time.Unix(1_700_000_000, 0).UTC()

	db.On("WithContext", mock.Anything).Return(db)
	db.On("Model", mock.MatchedBy(func(model any) bool {
		item, ok := model.(*EventBusScheduledEvent)
		return ok &&
			item.PK == eventBusSchedulePK &&
			item.SK == eventBusScheduleSK(dueAt, event.ID) &&
			item.DueAt.Equal(dueAt) &&
			item.EventID == event.ID &&
			item.EventType == event.EventType &&
			item.TenantID == event.TenantID
	})).Return(query)
	query.On("IfNotExists").Return(query)
	query.On("Create").Return(nil)

	created, err := EventBusSchedule(context.Background(), db, event, dueAt, 0)
	require.NoError(t, err)
	require.True(t, created)
}

func TestEventBusSchedule_AlreadyExists(t *testing.T) {
	db := liftmocks.NewMockExtendedDB()
	query := new(dynamormmocks.MockQuery)

	event := &Event{
		ID:        "evt_123",
		EventType: "partner.created",
	}
	dueAt := time.Unix(1_700_000_000, 0).UTC()

	db.On("WithContext", mock.Anything).Return(db)
	db.On("Model", mock.Anything).Return(query)
	query.On("IfNotExists").Return(query)
	query.On("Create").Return(dynamormerrors.ErrConditionFailed)

	created, err := EventBusSchedule(context.Background(), db, event, dueAt, 0)
	require.NoError(t, err)
	require.False(t, created)
}

func TestEventBusDueScheduled_QueriesByDueTime(t *testing.T) {
	db := liftmocks.NewMockExtendedDB()
	query := new(dynamormmocks.MockQuery)

	now := time.Unix(1_700_000_000, 0).UTC()

	db.On("WithContext", mock.Anything).Return(db)
	db.On("Model", mock.Anything).Return(query)
	query.On("Where", "PK", "=", eventBusSchedulePK).Return(query)
	query.On("Where", "SK", "<=", eventBusScheduleMaxSK(now)).Return(query)
	query.On("Limit", 10).Return(query)
	query.On("All", mock.Anything).Return(nil)

	_, err := EventBusDueScheduled(context.Background(), db, now, 10)
	require.NoError(t, err)
}

func TestEventBusDeleteScheduled_DeletesByKeys(t *testing.T) {
	db := liftmocks.NewMockExtendedDB()
	query := new(dynamormmocks.MockQuery)

	item := &EventBusScheduledEvent{
		PK: eventBusSchedulePK,
		SK: "00000000000000000000#evt_123",
	}

	db.On("WithContext", mock.Anything).Return(db)
	db.On("Model", mock.Anything).Return(query)
	query.On("Where", "PK", "=", item.PK).Return(query)
	query.On("Where", "SK", "=", item.SK).Return(query)
	query.On("Delete").Return(nil)

	err := EventBusDeleteScheduled(context.Background(), db, item)
	require.NoError(t, err)
}

type testEventBus struct {
	publish func(context.Context, *Event) (string, error)
}

func (b testEventBus) Publish(ctx context.Context, event *Event) (string, error) {
	return b.publish(ctx, event)
}

func (testEventBus) Query(context.Context, *EventQuery) ([]*Event, error)  { return nil, nil }
func (testEventBus) Subscribe(context.Context, string, EventHandler) error { return nil }
func (testEventBus) GetEvent(context.Context, string) (*Event, error)      { return nil, nil }
func (testEventBus) DeleteEvent(context.Context, string) error             { return nil }

func TestEventBusDrainDueScheduled_PublishesAndDeletes(t *testing.T) {
	db := liftmocks.NewMockExtendedDB()
	query := new(dynamormmocks.MockQuery)
	update := new(dynamormmocks.MockUpdateBuilder)

	dueAt := time.Unix(1_700_000_000, 0).UTC()
	createdAt := dueAt.Add(-time.Minute)

	db.On("WithContext", mock.Anything).Return(db)
	db.On("Model", mock.Anything).Return(query)
	query.On("Where", mock.Anything, mock.Anything, mock.Anything).Return(query)
	query.On("Limit", mock.Anything).Return(query).Maybe()
	query.On("All", mock.Anything).Run(func(args mock.Arguments) {
		out := args.Get(0).(*[]EventBusScheduledEvent)
		*out = []EventBusScheduledEvent{
			{
				PK:        eventBusSchedulePK,
				SK:        eventBusScheduleSK(dueAt, "evt_123"),
				DueAt:     dueAt,
				CreatedAt: createdAt,
				EventID:   "evt_123",
				EventType: "partner.created",
				TenantID:  "tenant-1",
				Payload:   []byte(`{"ok":true}`),
				Version:   1,
			},
		}
	}).Return(nil).Once()

	query.On("WithConditionExpression", mock.Anything, mock.Anything).Return(query).Once()
	query.On("UpdateBuilder").Return(update).Once()
	update.On("Set", "LeaseID", mock.Anything).Return(update).Once()
	update.On("Set", "LeaseUntil", mock.Anything).Return(update).Once()
	update.On("Execute").Return(nil).Once()

	query.On("WithCondition", "LeaseID", "=", mock.Anything).Return(query).Once()
	query.On("Delete").Return(nil).Once()

	var published *Event
	bus := testEventBus{
		publish: func(_ context.Context, event *Event) (string, error) {
			copied := *event
			published = &copied
			return event.ID, nil
		},
	}

	result, err := EventBusDrainDueScheduled(context.Background(), db, bus, dueAt.Add(time.Second), 10)
	require.NoError(t, err)
	require.Equal(t, EventBusScheduleDrainResult{
		Due:       1,
		Claimed:   1,
		Published: 1,
		Deleted:   1,
	}, result)
	require.NotNil(t, published)
	require.Equal(t, "tenant-1#partner.created", published.PartitionKey)
	require.Equal(t, dueAt, published.PublishedAt)
	require.Equal(t, createdAt, published.CreatedAt)
	require.Equal(t, eventBusSortKey(dueAt, "evt_123"), published.SortKey)
}

func TestEventBusDrainDueScheduled_AlreadyPublished(t *testing.T) {
	db := liftmocks.NewMockExtendedDB()
	query := new(dynamormmocks.MockQuery)
	update := new(dynamormmocks.MockUpdateBuilder)

	dueAt := time.Unix(1_700_000_000, 0).UTC()

	db.On("WithContext", mock.Anything).Return(db)
	db.On("Model", mock.Anything).Return(query)
	query.On("Where", mock.Anything, mock.Anything, mock.Anything).Return(query)
	query.On("Limit", mock.Anything).Return(query).Maybe()
	query.On("All", mock.Anything).Run(func(args mock.Arguments) {
		out := args.Get(0).(*[]EventBusScheduledEvent)
		*out = []EventBusScheduledEvent{
			{
				PK:        eventBusSchedulePK,
				SK:        eventBusScheduleSK(dueAt, "evt_123"),
				DueAt:     dueAt,
				EventID:   "evt_123",
				EventType: "partner.created",
				TenantID:  "tenant-1",
				Payload:   []byte(`{}`),
				Version:   1,
			},
		}
	}).Return(nil).Once()

	query.On("WithConditionExpression", mock.Anything, mock.Anything).Return(query).Once()
	query.On("UpdateBuilder").Return(update).Once()
	update.On("Set", "LeaseID", mock.Anything).Return(update).Once()
	update.On("Set", "LeaseUntil", mock.Anything).Return(update).Once()
	update.On("Execute").Return(nil).Once()

	query.On("WithCondition", "LeaseID", "=", mock.Anything).Return(query).Once()
	query.On("Delete").Return(nil).Once()

	bus := testEventBus{
		publish: func(context.Context, *Event) (string, error) {
			return "", dynamormerrors.ErrConditionFailed
		},
	}

	result, err := EventBusDrainDueScheduled(context.Background(), db, bus, dueAt.Add(time.Second), 10)
	require.NoError(t, err)
	require.Equal(t, EventBusScheduleDrainResult{
		Due:              1,
		Claimed:          1,
		AlreadyPublished: 1,
		Deleted:          1,
	}, result)
}

func TestEventBusDrainDueScheduled_StopsOnPublishError(t *testing.T) {
	db := liftmocks.NewMockExtendedDB()
	query := new(dynamormmocks.MockQuery)
	update := new(dynamormmocks.MockUpdateBuilder)

	dueAt := time.Unix(1_700_000_000, 0).UTC()

	db.On("WithContext", mock.Anything).Return(db)
	db.On("Model", mock.Anything).Return(query)
	query.On("Where", mock.Anything, mock.Anything, mock.Anything).Return(query)
	query.On("Limit", mock.Anything).Return(query).Maybe()
	query.On("All", mock.Anything).Run(func(args mock.Arguments) {
		out := args.Get(0).(*[]EventBusScheduledEvent)
		*out = []EventBusScheduledEvent{
			{
				PK:        eventBusSchedulePK,
				SK:        eventBusScheduleSK(dueAt, "evt_123"),
				DueAt:     dueAt,
				EventID:   "evt_123",
				EventType: "partner.created",
				TenantID:  "tenant-1",
				Payload:   []byte(`{}`),
				Version:   1,
			},
		}
	}).Return(nil).Once()

	query.On("WithConditionExpression", mock.Anything, mock.Anything).Return(query).Once()
	query.On("UpdateBuilder").Return(update).Once()
	update.On("Set", "LeaseID", mock.Anything).Return(update).Once()
	update.On("Set", "LeaseUntil", mock.Anything).Return(update).Once()
	update.On("Execute").Return(nil).Once()

	query.On("Delete").Return(nil).Maybe()

	bus := testEventBus{
		publish: func(context.Context, *Event) (string, error) {
			return "", errors.New("boom")
		},
	}

	_, err := EventBusDrainDueScheduled(context.Background(), db, bus, dueAt.Add(time.Second), 10)
	require.Error(t, err)
	query.AssertNotCalled(t, "Delete")
}

func TestEventBusDrainDueScheduledWithOptions_BackoffsOnPublishError(t *testing.T) {
	db := liftmocks.NewMockExtendedDB()
	query := new(dynamormmocks.MockQuery)
	claimUpdate := new(dynamormmocks.MockUpdateBuilder)
	backoffUpdate := new(dynamormmocks.MockUpdateBuilder)

	dueAt := time.Unix(1_700_000_000, 0).UTC()

	db.On("WithContext", mock.Anything).Return(db)
	db.On("Model", mock.Anything).Return(query)
	query.On("Where", mock.Anything, mock.Anything, mock.Anything).Return(query)
	query.On("Limit", mock.Anything).Return(query).Maybe()
	query.On("All", mock.Anything).Run(func(args mock.Arguments) {
		out := args.Get(0).(*[]EventBusScheduledEvent)
		*out = []EventBusScheduledEvent{
			{
				PK:        eventBusSchedulePK,
				SK:        eventBusScheduleSK(dueAt, "evt_123"),
				DueAt:     dueAt,
				EventID:   "evt_123",
				EventType: "partner.created",
				TenantID:  "tenant-1",
				Payload:   []byte(`{}`),
				Version:   1,
			},
		}
	}).Return(nil).Once()

	var leaseID string
	query.On("WithConditionExpression", mock.Anything, mock.Anything).Return(query).Once()
	query.On("UpdateBuilder").Return(claimUpdate).Once()
	claimUpdate.On("Set", "LeaseID", mock.Anything).Run(func(args mock.Arguments) {
		if v, ok := args.Get(1).(string); ok {
			leaseID = v
		}
	}).Return(claimUpdate).Once()
	claimUpdate.On("Set", "LeaseUntil", mock.Anything).Return(claimUpdate).Once()
	claimUpdate.On("Execute").Return(nil).Once()

	query.On("WithCondition", "LeaseID", "=", mock.MatchedBy(func(v any) bool {
		s, ok := v.(string)
		return ok && s != "" && s == leaseID
	})).Return(query).Once()
	query.On("UpdateBuilder").Return(backoffUpdate).Once()
	backoffUpdate.On("Add", "RetryCount", 1).Return(backoffUpdate).Once()
	backoffUpdate.On("Set", "LeaseUntil", mock.Anything).Return(backoffUpdate).Once()
	backoffUpdate.On("Set", "LastAttemptAt", mock.Anything).Return(backoffUpdate).Once()
	backoffUpdate.On("Set", "LastError", mock.Anything).Return(backoffUpdate).Once()
	backoffUpdate.On("Execute").Return(nil).Once()

	bus := testEventBus{
		publish: func(context.Context, *Event) (string, error) {
			return "", errors.New("boom")
		},
	}

	result, err := EventBusDrainDueScheduledWithOptions(context.Background(), db, bus, dueAt.Add(time.Second), 10, EventBusScheduleDrainOptions{
		ContinueOnPublishError: true,
		MaxPublishAttempts:     5,
		RetryBaseDelay:         time.Second,
		RetryMaxDelay:          10 * time.Second,
	})
	require.NoError(t, err)
	require.Equal(t, EventBusScheduleDrainResult{
		Due:         1,
		Claimed:     1,
		Rescheduled: 1,
	}, result)
	query.AssertNotCalled(t, "Delete")
}

func TestEventBusDrainDueScheduled_SkipsWhenLeased(t *testing.T) {
	db := liftmocks.NewMockExtendedDB()
	query := new(dynamormmocks.MockQuery)
	update := new(dynamormmocks.MockUpdateBuilder)

	dueAt := time.Unix(1_700_000_000, 0).UTC()

	db.On("WithContext", mock.Anything).Return(db)
	db.On("Model", mock.Anything).Return(query)
	query.On("Where", mock.Anything, mock.Anything, mock.Anything).Return(query)
	query.On("Limit", mock.Anything).Return(query).Maybe()
	query.On("All", mock.Anything).Run(func(args mock.Arguments) {
		out := args.Get(0).(*[]EventBusScheduledEvent)
		*out = []EventBusScheduledEvent{
			{
				PK:        eventBusSchedulePK,
				SK:        eventBusScheduleSK(dueAt, "evt_123"),
				DueAt:     dueAt,
				EventID:   "evt_123",
				EventType: "partner.created",
				TenantID:  "tenant-1",
				Payload:   []byte(`{}`),
				Version:   1,
			},
		}
	}).Return(nil).Once()

	query.On("WithConditionExpression", mock.Anything, mock.Anything).Return(query).Once()
	query.On("UpdateBuilder").Return(update).Once()
	update.On("Set", "LeaseID", mock.Anything).Return(update).Once()
	update.On("Set", "LeaseUntil", mock.Anything).Return(update).Once()
	update.On("Execute").Return(dynamormerrors.ErrConditionFailed).Once()

	bus := testEventBus{
		publish: func(context.Context, *Event) (string, error) {
			return "", errors.New("unexpected publish")
		},
	}

	result, err := EventBusDrainDueScheduled(context.Background(), db, bus, dueAt.Add(time.Second), 10)
	require.NoError(t, err)
	require.Equal(t, EventBusScheduleDrainResult{
		Due: 1,
	}, result)
	query.AssertNotCalled(t, "Delete")
}

func eventBusSortKey(dueAt time.Time, eventID string) string {
	return fmt.Sprintf("%d#%s", dueAt.UTC().UnixNano(), eventID)
}
