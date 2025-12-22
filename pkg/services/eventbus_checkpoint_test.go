package services

import (
	"context"
	"testing"

	dynamormerrors "github.com/pay-theory/dynamorm/pkg/errors"
	dynamormmocks "github.com/pay-theory/dynamorm/pkg/mocks"
	liftmocks "github.com/pay-theory/lift/pkg/dynamorm/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestEventBusIsProcessed_NotFound(t *testing.T) {
	db := liftmocks.NewMockExtendedDB()
	query := new(dynamormmocks.MockQuery)

	pk, sk := eventBusCheckpointKey("consumer-a", "evt_123")
	db.On("WithContext", mock.Anything).Return(db)
	db.On("Model", mock.Anything).Return(query)
	query.On("Where", "PK", "=", pk).Return(query)
	query.On("Where", "SK", "=", sk).Return(query)
	query.On("First", mock.Anything).Return(dynamormerrors.ErrItemNotFound)

	processed, err := EventBusIsProcessed(context.Background(), db, "consumer-a", "evt_123")
	require.NoError(t, err)
	require.False(t, processed)
}

func TestEventBusIsProcessed_Exists(t *testing.T) {
	db := liftmocks.NewMockExtendedDB()
	query := new(dynamormmocks.MockQuery)

	pk, sk := eventBusCheckpointKey("consumer-a", "evt_123")
	db.On("WithContext", mock.Anything).Return(db)
	db.On("Model", mock.Anything).Return(query)
	query.On("Where", "PK", "=", pk).Return(query)
	query.On("Where", "SK", "=", sk).Return(query)
	query.On("First", mock.Anything).Return(nil)

	processed, err := EventBusIsProcessed(context.Background(), db, "consumer-a", "evt_123")
	require.NoError(t, err)
	require.True(t, processed)
}

func TestEventBusMarkProcessed_CreatesOnce(t *testing.T) {
	db := liftmocks.NewMockExtendedDB()
	query := new(dynamormmocks.MockQuery)

	event := &Event{
		ID:        "evt_123",
		EventType: "partner.created",
	}
	pk, sk := eventBusCheckpointKey("consumer-a", event.ID)

	db.On("WithContext", mock.Anything).Return(db)
	db.On("Model", mock.MatchedBy(func(model any) bool {
		cp, ok := model.(*EventBusCheckpoint)
		return ok && cp.PK == pk && cp.SK == sk && cp.Consumer == "consumer-a" && cp.EventID == event.ID && cp.EventType == event.EventType && !cp.ProcessedAt.IsZero() && cp.TTL == 0
	})).Return(query)
	query.On("IfNotExists").Return(query)
	query.On("Create").Return(nil)

	created, err := EventBusMarkProcessed(context.Background(), db, "consumer-a", event, 0)
	require.NoError(t, err)
	require.True(t, created)
}

func TestEventBusMarkProcessed_AlreadyProcessed(t *testing.T) {
	db := liftmocks.NewMockExtendedDB()
	query := new(dynamormmocks.MockQuery)

	event := &Event{
		ID:        "evt_123",
		EventType: "partner.created",
	}
	pk, sk := eventBusCheckpointKey("consumer-a", event.ID)

	db.On("WithContext", mock.Anything).Return(db)
	db.On("Model", mock.MatchedBy(func(model any) bool {
		cp, ok := model.(*EventBusCheckpoint)
		return ok && cp.PK == pk && cp.SK == sk
	})).Return(query)
	query.On("IfNotExists").Return(query)
	query.On("Create").Return(dynamormerrors.ErrConditionFailed)

	created, err := EventBusMarkProcessed(context.Background(), db, "consumer-a", event, 0)
	require.NoError(t, err)
	require.False(t, created)
}
