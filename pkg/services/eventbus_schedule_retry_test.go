package services

import (
	"context"
	"errors"
	"testing"
	"time"

	dynamormmocks "github.com/pay-theory/dynamorm/pkg/mocks"
	liftmocks "github.com/pay-theory/lift/pkg/dynamorm/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestEventBusBackoffScheduled_UpdatesRetryAndLease(t *testing.T) {
	db := liftmocks.NewMockExtendedDB()
	query := new(dynamormmocks.MockQuery)
	update := new(dynamormmocks.MockUpdateBuilder)

	item := &EventBusScheduledEvent{
		PK:         eventBusSchedulePK,
		SK:         "00000000000000000000#evt_123",
		EventID:    "evt_123",
		RetryCount: 0,
	}
	now := time.Unix(1_700_000_000, 0).UTC()

	db.On("WithContext", mock.Anything).Return(db)
	db.On("Model", mock.Anything).Return(query)
	query.On("Where", mock.Anything, mock.Anything, mock.Anything).Return(query)
	query.On("WithCondition", "LeaseID", "=", "lease-1").Return(query).Once()
	query.On("UpdateBuilder").Return(update).Once()

	update.On("Add", "RetryCount", 1).Return(update).Once()
	update.On("Set", "LeaseUntil", mock.Anything).Return(update).Once()
	update.On("Set", "LastAttemptAt", now).Return(update).Once()
	update.On("Set", "LastError", "boom").Return(update).Once()
	update.On("Execute").Return(nil).Once()

	require.NoError(t, EventBusBackoffScheduled(context.Background(), db, item, "lease-1", now, 0, errors.New("boom")))

	require.Equal(t, 1, item.RetryCount)
	require.NotZero(t, item.LeaseUntil)
	require.Equal(t, now, item.LastAttemptAt)
	require.Equal(t, "boom", item.LastError)

	db.AssertExpectations(t)
	query.AssertExpectations(t)
	update.AssertExpectations(t)
}

func TestEventBusBackoffScheduled_ValidatesInputs(t *testing.T) {
	db := liftmocks.NewMockExtendedDB()
	item := &EventBusScheduledEvent{PK: "pk", SK: "sk"}

	require.Error(t, EventBusBackoffScheduled(context.Background(), nil, item, "lease", time.Now(), time.Second, nil))
	require.Error(t, EventBusBackoffScheduled(context.Background(), db, nil, "lease", time.Now(), time.Second, nil))
	require.Error(t, EventBusBackoffScheduled(context.Background(), db, &EventBusScheduledEvent{}, "lease", time.Now(), time.Second, nil))
	require.Error(t, EventBusBackoffScheduled(context.Background(), db, item, "", time.Now(), time.Second, nil))
	require.Error(t, EventBusBackoffScheduled(context.Background(), db, item, "lease", time.Now(), -time.Second, nil))
}
