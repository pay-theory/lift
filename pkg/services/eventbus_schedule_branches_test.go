package services

import (
	"context"
	"errors"
	"testing"
	"time"

	dynamormerrors "github.com/pay-theory/dynamorm/pkg/errors"
	dynamormmocks "github.com/pay-theory/dynamorm/pkg/mocks"
	liftmocks "github.com/pay-theory/lift/pkg/dynamorm/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestEventBusScheduledEvent_Event_NilReceiverAndFieldMapping(t *testing.T) {
	var nilItem *EventBusScheduledEvent
	require.Nil(t, nilItem.Event())

	item := &EventBusScheduledEvent{
		EventID:       "evt_123",
		EventType:     "partner.created",
		TenantID:      "tenant-1",
		SourceID:      "src",
		CorrelationID: "corr",
		Payload:       []byte(`{"ok":true}`),
		Metadata:      map[string]string{"k": "v"},
		Tags:          []string{"t1"},
		Version:       2,
		RetryCount:    3,
	}
	ev := item.Event()
	require.NotNil(t, ev)
	require.Equal(t, item.EventID, ev.ID)
	require.Equal(t, item.EventType, ev.EventType)
	require.Equal(t, item.TenantID, ev.TenantID)
	require.Equal(t, item.SourceID, ev.SourceID)
	require.Equal(t, item.CorrelationID, ev.CorrelationID)
	require.Equal(t, item.Payload, ev.Payload)
	require.Equal(t, item.Metadata, ev.Metadata)
	require.Equal(t, item.Tags, ev.Tags)
	require.Equal(t, item.Version, ev.Version)
	require.Equal(t, item.RetryCount, ev.RetryCount)
}

func TestEventBusSchedule_ValidationsRetentionAndErrorWrapping(t *testing.T) {
	db := new(liftmocks.MockExtendedDB)
	query := new(dynamormmocks.MockQuery)

	dueAt := time.Unix(1_700_000_000, 0).UTC()
	event := &Event{EventType: "partner.created", TenantID: "tenant-1"}

	require.Error(t, func() error { _, err := EventBusSchedule(context.Background(), nil, event, dueAt, 0); return err }())
	require.Error(t, func() error { _, err := EventBusSchedule(context.Background(), db, nil, dueAt, 0); return err }())
	require.Error(t, func() error { _, err := EventBusSchedule(context.Background(), db, &Event{}, dueAt, 0); return err }())
	require.Error(t, func() error { _, err := EventBusSchedule(context.Background(), db, event, time.Time{}, 0); return err }())

	db.On("WithContext", mock.Anything).Return(db)
	db.On("Model", mock.MatchedBy(func(model any) bool {
		item, ok := model.(*EventBusScheduledEvent)
		return ok && item.TTL > 0 && item.EventID != "" && item.EventType == event.EventType
	})).Return(query).Once()
	query.On("IfNotExists").Return(query).Once()
	query.On("Create").Return(errors.New("boom")).Once()

	_, err := EventBusSchedule(context.Background(), db, &Event{EventType: "partner.created"}, dueAt, time.Hour)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to schedule event")
}

func TestEventBusDueScheduled_SkipsLimitWhenZero(t *testing.T) {
	db := new(liftmocks.MockExtendedDB)
	query := new(dynamormmocks.MockQuery)

	now := time.Unix(1_700_000_000, 0).UTC()
	db.On("WithContext", mock.Anything).Return(db)
	db.On("Model", mock.Anything).Return(query)
	query.On("Where", mock.Anything, mock.Anything, mock.Anything).Return(query)
	query.On("All", mock.Anything).Return(nil).Once()

	_, err := EventBusDueScheduled(context.Background(), db, now, 0)
	require.NoError(t, err)
}

func TestEventBusDeleteScheduledAndClaimed_DeleteBranches(t *testing.T) {
	t.Run("validates inputs", func(t *testing.T) {
		db := new(liftmocks.MockExtendedDB)
		require.Error(t, EventBusDeleteScheduled(context.Background(), nil, &EventBusScheduledEvent{PK: "pk", SK: "sk"}))
		require.Error(t, EventBusDeleteScheduled(context.Background(), db, nil))
		require.Error(t, EventBusDeleteScheduled(context.Background(), db, &EventBusScheduledEvent{}))

		require.Error(t, EventBusDeleteScheduledClaimed(context.Background(), nil, &EventBusScheduledEvent{PK: "pk", SK: "sk"}, "lease"))
		require.Error(t, EventBusDeleteScheduledClaimed(context.Background(), db, nil, "lease"))
		require.Error(t, EventBusDeleteScheduledClaimed(context.Background(), db, &EventBusScheduledEvent{}, "lease"))
		require.Error(t, EventBusDeleteScheduledClaimed(context.Background(), db, &EventBusScheduledEvent{PK: "pk", SK: "sk"}, ""))
	})

	t.Run("delete wraps errors", func(t *testing.T) {
		db := new(liftmocks.MockExtendedDB)
		query := new(dynamormmocks.MockQuery)

		item := &EventBusScheduledEvent{PK: "pk", SK: "sk"}

		db.On("WithContext", mock.Anything).Return(db)
		db.On("Model", mock.Anything).Return(query)
		query.On("Where", mock.Anything, mock.Anything, mock.Anything).Return(query)
		query.On("Delete").Return(errors.New("boom")).Once()

		err := EventBusDeleteScheduled(context.Background(), db, item)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to delete scheduled item")
	})
}

func TestEventBusClaimScheduled_OwnerConditionFailedAndSuccess(t *testing.T) {
	t.Run("condition failed returns not claimed", func(t *testing.T) {
		db := new(liftmocks.MockExtendedDB)
		query := new(dynamormmocks.MockQuery)
		update := new(dynamormmocks.MockUpdateBuilder)

		item := &EventBusScheduledEvent{PK: "pk", SK: "sk", EventID: "evt"}

		db.On("WithContext", mock.Anything).Return(db)
		db.On("Model", mock.Anything).Return(query)
		query.On("Where", mock.Anything, mock.Anything, mock.Anything).Return(query)
		query.On("WithConditionExpression", mock.Anything, mock.Anything).Return(query).Once()
		query.On("UpdateBuilder").Return(update).Once()
		update.On("Set", mock.Anything, mock.Anything).Return(update)
		update.On("Execute").Return(dynamormerrors.ErrConditionFailed).Once()

		leaseID, claimed, err := EventBusClaimScheduled(context.Background(), db, item, time.Now(), 0, "")
		require.NoError(t, err)
		require.False(t, claimed)
		require.Empty(t, leaseID)
	})

	t.Run("success sets owner and updates item", func(t *testing.T) {
		db := new(liftmocks.MockExtendedDB)
		query := new(dynamormmocks.MockQuery)
		update := new(dynamormmocks.MockUpdateBuilder)

		item := &EventBusScheduledEvent{PK: "pk", SK: "sk", EventID: "evt"}
		now := time.Unix(1_700_000_000, 0).UTC()

		db.On("WithContext", mock.Anything).Return(db)
		db.On("Model", mock.Anything).Return(query)
		query.On("Where", mock.Anything, mock.Anything, mock.Anything).Return(query)
		query.On("WithConditionExpression", mock.Anything, mock.Anything).Return(query).Once()
		query.On("UpdateBuilder").Return(update).Once()
		update.On("Set", "LeaseID", mock.Anything).Return(update).Once()
		update.On("Set", "LeaseUntil", mock.Anything).Return(update).Once()
		update.On("Set", "LeaseOwner", "owner-1").Return(update).Once()
		update.On("Execute").Return(nil).Once()

		leaseID, claimed, err := EventBusClaimScheduled(context.Background(), db, item, now, 0, "owner-1")
		require.NoError(t, err)
		require.True(t, claimed)
		require.NotEmpty(t, leaseID)
		require.Equal(t, leaseID, item.LeaseID)
		require.Equal(t, "owner-1", item.LeaseOwner)
		require.Greater(t, item.LeaseUntil, now.Unix())
	})
}

func TestDeleteScheduledItemIfLeaseHeld_SuccessAndLeaseLoss(t *testing.T) {
	item := &EventBusScheduledEvent{PK: "pk", SK: "sk"}

	t.Run("lease lost is ignored", func(t *testing.T) {
		db := new(liftmocks.MockExtendedDB)
		query := new(dynamormmocks.MockQuery)

		db.On("WithContext", mock.Anything).Return(db)
		db.On("Model", mock.Anything).Return(query)
		query.On("Where", mock.Anything, mock.Anything, mock.Anything).Return(query)
		query.On("WithCondition", mock.Anything, mock.Anything, mock.Anything).Return(query)
		query.On("Delete").Return(dynamormerrors.ErrConditionFailed).Once()

		var result EventBusScheduleDrainResult
		require.NoError(t, deleteScheduledItemIfLeaseHeld(context.Background(), db, item, "lease", &result))
		require.Equal(t, 0, result.Deleted)
	})

	t.Run("success increments deleted", func(t *testing.T) {
		db := new(liftmocks.MockExtendedDB)
		query := new(dynamormmocks.MockQuery)

		db.On("WithContext", mock.Anything).Return(db)
		db.On("Model", mock.Anything).Return(query)
		query.On("Where", mock.Anything, mock.Anything, mock.Anything).Return(query)
		query.On("WithCondition", mock.Anything, mock.Anything, mock.Anything).Return(query)
		query.On("Delete").Return(nil).Once()

		var result EventBusScheduleDrainResult
		require.NoError(t, deleteScheduledItemIfLeaseHeld(context.Background(), db, item, "lease", &result))
		require.Equal(t, 1, result.Deleted)
	})
}

func TestEventBusDrainDueScheduledWithOptions_ValidatesInputs(t *testing.T) {
	_, err := EventBusDrainDueScheduledWithOptions(context.Background(), nil, testEventBus{}, time.Now(), 1, EventBusScheduleDrainOptions{})
	require.Error(t, err)

	db := new(liftmocks.MockExtendedDB)
	_, err = EventBusDrainDueScheduledWithOptions(context.Background(), db, nil, time.Now(), 1, EventBusScheduleDrainOptions{})
	require.Error(t, err)
}
