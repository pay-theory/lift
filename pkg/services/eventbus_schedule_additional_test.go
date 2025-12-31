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

func TestEventBusDrainDueScheduledWithOptions_QuarantinesAfterMaxAttempts(t *testing.T) {
	db := liftmocks.NewMockExtendedDB()
	query := new(dynamormmocks.MockQuery)
	update := new(dynamormmocks.MockUpdateBuilder)

	dueAt := time.Unix(1_700_000_000, 0).UTC()

	db.On("WithContext", mock.Anything).Return(db)
	db.On("Model", mock.Anything).Return(query)
	query.On("Where", mock.Anything, mock.Anything, mock.Anything).Return(query)
	query.On("Limit", 10).Return(query).Once()
	query.On("All", mock.Anything).Run(func(args mock.Arguments) {
		out := args.Get(0).(*[]EventBusScheduledEvent)
		*out = []EventBusScheduledEvent{
			{
				PK:         eventBusSchedulePK,
				SK:         eventBusScheduleSK(dueAt, "evt_123"),
				DueAt:      dueAt,
				EventID:    "evt_123",
				EventType:  "partner.created",
				TenantID:   "tenant-1",
				Payload:    []byte(`{}`),
				Version:    1,
				RetryCount: 0,
			},
		}
	}).Return(nil).Once()

	// Claim succeeds.
	query.On("WithConditionExpression", mock.Anything, mock.Anything).Return(query).Once()
	query.On("UpdateBuilder").Return(update).Once()
	update.On("Set", "LeaseID", mock.Anything).Return(update).Once()
	update.On("Set", "LeaseUntil", mock.Anything).Return(update).Once()
	update.On("Execute").Return(nil).Once()

	// Quarantine transaction succeeds.
	db.On("TransactWrite", mock.Anything, mock.Anything).Return(nil).Once()

	bus := testEventBus{
		publish: func(context.Context, *Event) (string, error) {
			return "", errors.New("publish failed")
		},
	}

	result, err := EventBusDrainDueScheduledWithOptions(context.Background(), db, bus, dueAt.Add(time.Second), 10, EventBusScheduleDrainOptions{
		ContinueOnPublishError: true,
		MaxPublishAttempts:     1,
		QuarantineRetention:    time.Hour,
	})
	require.NoError(t, err)
	require.Equal(t, EventBusScheduleDrainResult{
		Due:         1,
		Claimed:     1,
		Quarantined: 1,
	}, result)
}

func TestBackoffScheduledItem_IgnoresLeaseLoss(t *testing.T) {
	db := liftmocks.NewMockExtendedDB()
	query := new(dynamormmocks.MockQuery)
	update := new(dynamormmocks.MockUpdateBuilder)

	item := &EventBusScheduledEvent{
		PK:      eventBusSchedulePK,
		SK:      "00000000000000000000#evt_123",
		EventID: "evt_123",
	}
	now := time.Unix(1_700_000_000, 0).UTC()

	db.On("WithContext", mock.Anything).Return(db)
	db.On("Model", mock.Anything).Return(query)
	query.On("Where", mock.Anything, mock.Anything, mock.Anything).Return(query)
	query.On("WithCondition", "LeaseID", "=", "lease-1").Return(query).Once()
	query.On("UpdateBuilder").Return(update).Once()

	update.On("Add", "RetryCount", 1).Return(update).Once()
	update.On("Set", "LeaseUntil", mock.Anything).Return(update).Once()
	update.On("Set", "LastAttemptAt", mock.Anything).Return(update).Once()
	update.On("Set", "LastError", "boom").Return(update).Once()
	update.On("Execute").Return(dynamormerrors.ErrConditionFailed).Once()

	var result EventBusScheduleDrainResult
	require.NoError(t, backoffScheduledItem(context.Background(), db, item, "lease-1", now, errors.New("boom"), 1, EventBusScheduleDrainOptions{
		RetryBaseDelay: 1 * time.Millisecond,
		RetryMaxDelay:  10 * time.Millisecond,
	}, &result))
	require.Equal(t, 0, result.Rescheduled)
}
