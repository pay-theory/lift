package services

import (
	"context"
	"errors"
	"testing"
	"time"

	dynamormerrors "github.com/pay-theory/dynamorm/pkg/errors"
	liftmocks "github.com/pay-theory/lift/pkg/dynamorm/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/pay-theory/dynamorm/pkg/core"
)

type recordingTx struct {
	puts    []recordedPut
	deletes []recordedDelete
}

type recordedPut struct {
	model      any
	conditions []core.TransactCondition
}

type recordedDelete struct {
	model      any
	conditions []core.TransactCondition
}

func (r *recordingTx) Put(model any, conditions ...core.TransactCondition) core.TransactionBuilder {
	r.puts = append(r.puts, recordedPut{model: model, conditions: conditions})
	return r
}
func (r *recordingTx) Create(model any, conditions ...core.TransactCondition) core.TransactionBuilder {
	return r.Put(model, conditions...)
}
func (r *recordingTx) Update(_ any, _ []string, _ ...core.TransactCondition) core.TransactionBuilder {
	return r
}
func (r *recordingTx) UpdateWithBuilder(_ any, _ func(core.UpdateBuilder) error, _ ...core.TransactCondition) core.TransactionBuilder {
	return r
}
func (r *recordingTx) Delete(model any, conditions ...core.TransactCondition) core.TransactionBuilder {
	r.deletes = append(r.deletes, recordedDelete{model: model, conditions: conditions})
	return r
}
func (r *recordingTx) ConditionCheck(_ any, _ ...core.TransactCondition) core.TransactionBuilder {
	return r
}
func (r *recordingTx) WithContext(_ context.Context) core.TransactionBuilder { return r }
func (r *recordingTx) Execute() error                                        { return nil }
func (r *recordingTx) ExecuteWithContext(_ context.Context) error            { return nil }

var _ core.TransactionBuilder = (*recordingTx)(nil)

func TestEventBusQuarantineScheduled_ErrorsOnInvalidInputs(t *testing.T) {
	item := &EventBusScheduledEvent{PK: "pk", SK: "sk"}

	_, err := EventBusQuarantineScheduled(context.Background(), nil, item, "lease", errors.New("x"), 1, 0)
	require.Error(t, err)

	db := new(liftmocks.MockExtendedDB)

	_, err = EventBusQuarantineScheduled(context.Background(), db, nil, "lease", errors.New("x"), 1, 0)
	require.Error(t, err)

	_, err = EventBusQuarantineScheduled(context.Background(), db, &EventBusScheduledEvent{}, "lease", errors.New("x"), 1, 0)
	require.Error(t, err)

	_, err = EventBusQuarantineScheduled(context.Background(), db, item, "", errors.New("x"), 1, 0)
	require.Error(t, err)
}

func TestEventBusQuarantineScheduled_HappyPathAndLeaseLost(t *testing.T) {
	db := new(liftmocks.MockExtendedDB)
	tx := &recordingTx{}

	item := &EventBusScheduledEvent{
		PK:        eventBusSchedulePK,
		SK:        eventBusScheduleSK(time.Unix(1_700_000_000, 0).UTC(), "evt_123"),
		EventID:   "evt_123",
		EventType: "partner.created",
		TenantID:  "tenant-1",
		Payload:   []byte(`{"ok":true}`),
		Version:   1,
	}

	db.On("TransactWrite", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(core.TransactionBuilder) error)
		require.NoError(t, fn(tx))
	}).Return(nil).Once()

	quarantined, err := EventBusQuarantineScheduled(context.Background(), db, item, "lease-1", errors.New("boom"), 3, time.Hour)
	require.NoError(t, err)
	require.True(t, quarantined)

	require.Len(t, tx.puts, 1)
	record, ok := tx.puts[0].model.(*EventBusQuarantinedScheduledEvent)
	require.True(t, ok)
	require.Equal(t, eventBusQuarantineScheduledPK, record.PK)
	require.Equal(t, item.SK, record.SK)
	require.Equal(t, item.EventID, record.EventID)
	require.Equal(t, "boom", record.Cause)
	require.Equal(t, 3, record.Attempts)
	require.NotZero(t, record.QuarantinedAt)
	require.NotZero(t, record.TTL)

	require.Len(t, tx.deletes, 1)
	deleted, ok := tx.deletes[0].model.(*EventBusScheduledEvent)
	require.True(t, ok)
	require.Equal(t, item.PK, deleted.PK)
	require.Equal(t, item.SK, deleted.SK)
	require.Len(t, tx.deletes[0].conditions, 1)
	require.Equal(t, "LeaseID", tx.deletes[0].conditions[0].Field)
	require.Equal(t, "=", tx.deletes[0].conditions[0].Operator)
	require.Equal(t, "lease-1", tx.deletes[0].conditions[0].Value)

	db.AssertExpectations(t)

	// Lease lost → treat as no-op.
	db2 := new(liftmocks.MockExtendedDB)
	db2.On("TransactWrite", mock.Anything, mock.Anything).Return(dynamormerrors.ErrConditionFailed).Once()

	quarantined, err = EventBusQuarantineScheduled(context.Background(), db2, item, "lease-2", errors.New("boom"), 3, 0)
	require.NoError(t, err)
	require.False(t, quarantined)
	db2.AssertExpectations(t)
}

func TestEventBusQuarantineScheduled_PropagatesErrors(t *testing.T) {
	db := new(liftmocks.MockExtendedDB)
	db.On("TransactWrite", mock.Anything, mock.Anything).Return(errors.New("write failed")).Once()

	_, err := EventBusQuarantineScheduled(context.Background(), db, &EventBusScheduledEvent{PK: "pk", SK: "sk"}, "lease", errors.New("boom"), 1, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to quarantine")
	db.AssertExpectations(t)
}

func TestEventBusReplayQuarantinedScheduled_RestoresToSchedule(t *testing.T) {
	db := new(liftmocks.MockExtendedDB)
	tx := &recordingTx{}

	item := &EventBusQuarantinedScheduledEvent{
		PK:        eventBusQuarantineScheduledPK,
		SK:        eventBusScheduleSK(time.Unix(1_700_000_000, 0).UTC(), "evt_123"),
		EventID:   "evt_123",
		EventType: "partner.created",
		TenantID:  "tenant-1",
		Payload:   []byte(`{"ok":true}`),
		Version:   1,
	}

	db.On("TransactWrite", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(core.TransactionBuilder) error)
		require.NoError(t, fn(tx))
	}).Return(nil).Once()

	require.NoError(t, EventBusReplayQuarantinedScheduled(context.Background(), db, item))

	require.Len(t, tx.puts, 1)
	scheduled, ok := tx.puts[0].model.(*EventBusScheduledEvent)
	require.True(t, ok)
	require.Equal(t, eventBusSchedulePK, scheduled.PK)
	require.Equal(t, item.SK, scheduled.SK)
	require.Equal(t, 0, scheduled.RetryCount)

	require.Len(t, tx.deletes, 1)
	deleted, ok := tx.deletes[0].model.(*EventBusQuarantinedScheduledEvent)
	require.True(t, ok)
	require.Equal(t, item.PK, deleted.PK)
	require.Equal(t, item.SK, deleted.SK)

	db.AssertExpectations(t)
}

func TestEventBusReplayQuarantinedScheduled_ErrorsOnInvalidInputsAndTransactFailure(t *testing.T) {
	db := new(liftmocks.MockExtendedDB)

	require.Error(t, EventBusReplayQuarantinedScheduled(context.Background(), nil, &EventBusQuarantinedScheduledEvent{PK: "pk", SK: "sk"}))
	require.Error(t, EventBusReplayQuarantinedScheduled(context.Background(), db, nil))
	require.Error(t, EventBusReplayQuarantinedScheduled(context.Background(), db, &EventBusQuarantinedScheduledEvent{}))

	db.On("TransactWrite", mock.Anything, mock.Anything).Return(errors.New("fail")).Once()
	require.Error(t, EventBusReplayQuarantinedScheduled(context.Background(), db, &EventBusQuarantinedScheduledEvent{PK: "pk", SK: "sk"}))
	db.AssertExpectations(t)
}
