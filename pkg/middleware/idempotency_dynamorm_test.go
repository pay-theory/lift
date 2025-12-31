package middleware

import (
	"context"
	"errors"
	"testing"
	"time"

	dynamormcore "github.com/pay-theory/dynamorm/pkg/core"
	dynamormerrors "github.com/pay-theory/dynamorm/pkg/errors"
	dynamormmocks "github.com/pay-theory/dynamorm/pkg/mocks"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/pay-theory/lift/pkg/models"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDynamORMIdempotencyStore_getDB_RequiresConfiguration(t *testing.T) {
	store := NewDynamORMIdempotencyStore()
	_, err := store.getDB(context.Background())
	require.Error(t, err)
}

func TestDynamORMIdempotencyStore_getDB_UsesProvidedDB(t *testing.T) {
	db := dynamormmocks.NewMockExtendedDB()
	store := NewDynamORMIdempotencyStoreWithDB(db)

	got, err := store.getDB(context.Background())
	require.NoError(t, err)
	require.Same(t, db, got)
}

func TestDynamORMIdempotencyStore_getDB_UsesWrapper(t *testing.T) {
	db := dynamormmocks.NewMockExtendedDB()
	wrapper := newTestDynamORMWrapper(t, db)
	store := NewDynamORMIdempotencyStoreWithWrapper(wrapper)

	got, err := store.getDB(context.Background())
	require.NoError(t, err)
	require.NotNil(t, got)
}

func TestDynamORMIdempotencyStore_getDB_UsesLiftContextDBOrTenantWrapper(t *testing.T) {
	db := dynamormmocks.NewMockExtendedDB()
	store := NewDynamORMIdempotencyStore()

	ctx := lift.NewContext(context.Background(), lift.NewRequest(&adapters.Request{Method: "GET", Path: "/"}))
	ctx.DB = dynamormcore.ExtendedDB(db)

	got, err := store.getDB(ctx)
	require.NoError(t, err)
	require.Same(t, db, got)

	ctx2 := lift.NewContext(context.Background(), lift.NewRequest(&adapters.Request{Method: "GET", Path: "/"}))
	ctx2.DB = "not-a-db"
	ctx2.Set("dynamorm_tenant", newTestDynamORMWrapper(t, db))

	got, err = store.getDB(ctx2)
	require.NoError(t, err)
	require.NotNil(t, got)
}

func TestDynamORMIdempotencyStore_Get_NotFound(t *testing.T) {
	db := dynamormmocks.NewMockExtendedDB()
	q := new(dynamormmocks.MockQuery)

	db.On("Model", mock.Anything).Return(q)
	q.On("Where", "IdempotencyKey", "=", "k1").Return(q)
	q.On("Where", "SK", "=", "IDEMPOTENCY").Return(q)
	q.On("First", mock.Anything).Return(dynamormerrors.ErrItemNotFound)

	store := NewDynamORMIdempotencyStoreWithDB(db)
	record, err := store.Get(context.Background(), "k1")
	require.NoError(t, err)
	require.Nil(t, record)
}

func TestDynamORMIdempotencyStore_Get_Success_UnmarshalsResponse(t *testing.T) {
	db := dynamormmocks.NewMockExtendedDB()
	q := new(dynamormmocks.MockQuery)

	db.On("Model", mock.Anything).Return(q)
	q.On("Where", "IdempotencyKey", "=", "k1").Return(q)
	q.On("Where", "SK", "=", "IDEMPOTENCY").Return(q)
	q.On("First", mock.Anything).Run(func(args mock.Arguments) {
		rec := args.Get(0).(*models.IdempotencyRecord)
		rec.IdempotencyKey = "k1"
		rec.Status = statusCompleted
		rec.StatusCode = 200
		rec.Response = `{"ok":true}`
		rec.CreatedAt = time.Now().Add(-time.Minute)
		rec.ExpiresAt = time.Now().Add(time.Hour)
	}).Return(nil)

	store := NewDynamORMIdempotencyStoreWithDB(db)
	record, err := store.Get(context.Background(), "k1")
	require.NoError(t, err)
	require.Equal(t, "k1", record.Key)
	_, ok := record.Response.(map[string]any)
	require.True(t, ok)
}

func TestDynamORMIdempotencyStore_Get_Success_InvalidJSONFallsBackToString(t *testing.T) {
	db := dynamormmocks.NewMockExtendedDB()
	q := new(dynamormmocks.MockQuery)

	db.On("Model", mock.Anything).Return(q)
	q.On("Where", "IdempotencyKey", "=", "k1").Return(q)
	q.On("Where", "SK", "=", "IDEMPOTENCY").Return(q)
	q.On("First", mock.Anything).Run(func(args mock.Arguments) {
		rec := args.Get(0).(*models.IdempotencyRecord)
		rec.IdempotencyKey = "k1"
		rec.Status = statusCompleted
		rec.Response = `not-json`
		rec.CreatedAt = time.Now()
		rec.ExpiresAt = time.Now().Add(time.Hour)
	}).Return(nil)

	store := NewDynamORMIdempotencyStoreWithDB(db)
	record, err := store.Get(context.Background(), "k1")
	require.NoError(t, err)
	require.Equal(t, "not-json", record.Response)
}

func TestDynamORMIdempotencyStore_Get_ReturnsWrappedErrors(t *testing.T) {
	db := dynamormmocks.NewMockExtendedDB()
	q := new(dynamormmocks.MockQuery)

	db.On("Model", mock.Anything).Return(q)
	q.On("Where", "IdempotencyKey", "=", "k1").Return(q)
	q.On("Where", "SK", "=", "IDEMPOTENCY").Return(q)
	q.On("First", mock.Anything).Return(errors.New("boom"))

	store := NewDynamORMIdempotencyStoreWithDB(db)
	_, err := store.Get(context.Background(), "k1")
	require.Error(t, err)
}

func TestDynamORMIdempotencyStore_Set_SetProcessing_Delete(t *testing.T) {
	db := dynamormmocks.NewMockExtendedDB()
	q := new(dynamormmocks.MockQuery)
	q2 := new(dynamormmocks.MockQuery)
	q3 := new(dynamormmocks.MockQuery)

	db.On("Model", mock.Anything).Return(q).Once()
	q.On("CreateOrUpdate").Return(nil).Once()

	db.On("Model", mock.Anything).Return(q2).Once()
	q2.On("IfNotExists").Return(q2).Once()
	q2.On("Create").Return(nil).Once()

	db.On("Model", mock.Anything).Return(q3).Once()
	q3.On("Where", "IdempotencyKey", "=", "k1").Return(q3).Once()
	q3.On("Where", "SK", "=", "IDEMPOTENCY").Return(q3).Once()
	q3.On("Delete").Return(nil).Once()

	store := NewDynamORMIdempotencyStoreWithDB(db)

	require.NoError(t, store.Set(context.Background(), "k1", &IdempotencyRecord{
		Status:     statusCompleted,
		StatusCode: 200,
		Response:   map[string]any{"ok": true},
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(time.Hour),
	}))

	require.NoError(t, store.SetProcessing(context.Background(), "k1", time.Now().Add(time.Minute)))
	require.NoError(t, store.Delete(context.Background(), "k1"))

	db.AssertExpectations(t)
	q.AssertExpectations(t)
	q2.AssertExpectations(t)
	q3.AssertExpectations(t)
}

