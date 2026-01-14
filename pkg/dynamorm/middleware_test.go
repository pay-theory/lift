package dynamorm

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/pay-theory/dynamorm/pkg/core"
	dynamormmocks "github.com/pay-theory/dynamorm/pkg/mocks"
	"github.com/pay-theory/dynamorm/pkg/session"
	"github.com/pay-theory/lift/pkg/dynamorm/mocks"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	require.NotNil(t, cfg)
	require.Equal(t, "lift_data", cfg.TableName)
	require.Equal(t, "us-east-1", cfg.Region)
	require.True(t, cfg.TenantIsolation)
	require.Equal(t, "tenant_id", cfg.TenantKey)
}

func TestIsWriteOperation(t *testing.T) {
	require.True(t, isWriteOperation("POST"))
	require.True(t, isWriteOperation("PUT"))
	require.True(t, isWriteOperation("PATCH"))
	require.True(t, isWriteOperation("DELETE"))
	require.False(t, isWriteOperation("GET"))
	require.False(t, isWriteOperation("HEAD"))
}

func TestWithDynamORM_TenantIsolationRequiresTenant(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoTransaction = false
	cfg.TenantIsolation = true

	factory := NewMockDBFactory()

	mw := WithDynamORM(cfg, factory)

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/items"})
	ctx := lift.NewContext(context.Background(), req)

	handler := mw(lift.HandlerFunc(func(_ *lift.Context) error {
		t.Fatalf("next handler should not run when tenant is missing")
		return nil
	}))

	err := handler.Handle(ctx)
	require.Error(t, err)
	var liftErr *lift.LiftError
	require.ErrorAs(t, err, &liftErr)
	require.Equal(t, 401, liftErr.StatusCode)

	require.NotNil(t, ctx.Get("dynamorm"))
}

func TestWithDynamORM_SetsTenantScopedDB(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoTransaction = false
	cfg.TenantIsolation = true

	factory := NewMockDBFactory()

	mw := WithDynamORM(cfg, factory)

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/items"})
	ctx := lift.NewContext(context.Background(), req)
	ctx.SetTenantID("tenant-123")

	called := false
	handler := mw(lift.HandlerFunc(func(ctx *lift.Context) error {
		called = true

		db, err := DB(ctx)
		require.NoError(t, err)
		require.Empty(t, db.tenantID)

		tenantDB, err := TenantDB(ctx)
		require.NoError(t, err)
		require.Equal(t, "tenant-123", tenantDB.tenantID)
		require.NotSame(t, db, tenantDB)

		return nil
	}))

	require.NoError(t, handler.Handle(ctx))
	require.True(t, called)
}

func TestWithDynamORM_AutoTransactionCommitsOnSuccess(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoTransaction = true
	cfg.TenantIsolation = false

	dbMock := mocks.NewMockExtendedDB()
	queryMock := new(dynamormmocks.MockQuery)

	dbMock.On("Model", mock.Anything).Return(queryMock).Maybe()
	queryMock.On("Create").Return(nil).Maybe()
	queryMock.On("Delete").Return(nil).Maybe()

	dbMock.On("Transaction", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		fn := args.Get(0).(func(tx *core.Tx) error)
		tx := &core.Tx{}
		tx.SetDB(dbMock)
		_ = fn(tx)
	}).Once()

	factory := &MockDBFactory{MockDB: dbMock}
	mw := WithDynamORM(cfg, factory)

	req := lift.NewRequest(&adapters.Request{Method: "POST", Path: "/items"})
	ctx := lift.NewContext(context.Background(), req)

	handler := mw(lift.HandlerFunc(func(ctx *lift.Context) error {
		raw := ctx.Get("dynamorm_transaction")
		require.NotNil(t, raw)

		tx, ok := raw.(*Transaction)
		require.True(t, ok)
		require.NoError(t, tx.Put(ctx.Context, map[string]string{"ok": "true"}))
		require.NoError(t, tx.Delete(ctx.Context, "k"))
		return nil
	}))

	require.NoError(t, handler.Handle(ctx))
	dbMock.AssertExpectations(t)
	queryMock.AssertExpectations(t)
}

func TestWithDynamORM_AutoTransactionRollsBackOnError(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoTransaction = true
	cfg.TenantIsolation = false

	dbMock := mocks.NewMockExtendedDB()
	factory := &MockDBFactory{MockDB: dbMock}
	mw := WithDynamORM(cfg, factory)

	req := lift.NewRequest(&adapters.Request{Method: "POST", Path: "/items"})
	ctx := lift.NewContext(context.Background(), req)

	sentinel := errors.New("handler failed")
	handler := mw(lift.HandlerFunc(func(_ *lift.Context) error {
		return sentinel
	}))

	err := handler.Handle(ctx)
	require.ErrorIs(t, err, sentinel)
	dbMock.AssertNotCalled(t, "Transaction", mock.Anything)
}

func TestDBHelpers_ErrorWhenNotInitialized(t *testing.T) {
	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/"})
	ctx := lift.NewContext(context.Background(), req)

	_, err := DB(ctx)
	require.Error(t, err)

	_, err = TenantDB(ctx)
	require.Error(t, err)
}

func TestMockDBFactory_CreateDB_HookAndError(t *testing.T) {
	db := mocks.NewMockExtendedDB()
	sentinel := errors.New("create failed")

	called := false
	factory := &MockDBFactory{
		MockDB: db,
		Error:  sentinel,
		OnCreateDB: func(cfg session.Config) {
			called = true
			require.Equal(t, "us-east-1", cfg.Region)
		},
	}

	got, err := factory.CreateDB(session.Config{Region: "us-east-1"})
	require.ErrorIs(t, err, sentinel)
	require.Nil(t, got)
	require.True(t, called)

	factory.Error = nil
	got, err = factory.CreateDB(session.Config{Region: "us-east-1"})
	require.NoError(t, err)
	require.Same(t, db, got)
}

func TestDefaultDBFactory_CreateDB(t *testing.T) {
	factory := &DefaultDBFactory{}

	// Use static credentials to avoid depending on environment setup.
	cfg := session.Config{
		Region:              "us-east-1",
		CredentialsProvider: credentials.NewStaticCredentialsProvider("AKID", "SECRET", ""),
	}

	db, err := factory.CreateDB(cfg)
	require.NoError(t, err)
	require.NotNil(t, db)
}

func TestWithDynamORM_AutoTransactionCommitFailureSurfacesSystemError(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoTransaction = true
	cfg.TenantIsolation = false

	sentinel := errors.New("tx failed")
	dbMock := mocks.NewMockExtendedDB()
	dbMock.On("Transaction", mock.Anything).Return(sentinel).Once()

	factory := &MockDBFactory{MockDB: dbMock}
	mw := WithDynamORM(cfg, factory)

	req := lift.NewRequest(&adapters.Request{Method: "POST", Path: "/items"})
	ctx := lift.NewContext(context.Background(), req)

	handler := mw(lift.HandlerFunc(func(_ *lift.Context) error { return nil }))

	err := handler.Handle(ctx)
	require.Error(t, err)
	var liftErr *lift.LiftError
	require.ErrorAs(t, err, &liftErr)
	require.Equal(t, 500, liftErr.StatusCode)
	require.ErrorIs(t, liftErr.Cause, sentinel)
}

func TestWithDynamORM_AutoTransactionPanicRollbackErrorIsCaptured(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoTransaction = true
	cfg.TenantIsolation = false

	dbMock := mocks.NewMockExtendedDB()
	queryMock := new(dynamormmocks.MockQuery)
	dbMock.On("Model", mock.Anything).Return(queryMock).Maybe()
	queryMock.On("Create").Return(nil).Maybe()
	queryMock.On("Delete").Return(nil).Maybe()

	dbMock.On("Transaction", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		fn := args.Get(0).(func(tx *core.Tx) error)
		tx := &core.Tx{}
		tx.SetDB(dbMock)
		_ = fn(tx)
	}).Once()

	factory := &MockDBFactory{MockDB: dbMock}
	mw := WithDynamORM(cfg, factory)

	req := lift.NewRequest(&adapters.Request{Method: "POST", Path: "/items"})
	ctx := lift.NewContext(context.Background(), req)

	handler := mw(lift.HandlerFunc(func(ctx *lift.Context) error {
		tx, ok := ctx.Get("dynamorm_transaction").(*Transaction)
		require.True(t, ok)
		require.NoError(t, tx.Put(ctx.Context, map[string]string{"ok": "true"}))
		require.NoError(t, tx.Commit())
		panic("boom")
	}))

	require.Panics(t, func() {
		_ = handler.Handle(ctx)
	})

	require.NotNil(t, ctx.Get("rollback_error"))
}

func TestDynamORMWrapper_CRUDAndQuery(t *testing.T) {
	dbMock := mocks.NewMockExtendedDB()
	queryMock := new(dynamormmocks.MockQuery)

	dbMock.On("WithContext", mock.Anything).Return(dbMock).Maybe()
	dbMock.On("Model", mock.Anything).Return(queryMock).Maybe()

	queryMock.On("Where", mock.Anything, mock.Anything, mock.Anything).Return(queryMock).Maybe()
	queryMock.On("Index", mock.Anything).Return(queryMock).Maybe()
	queryMock.On("Limit", mock.Anything).Return(queryMock).Maybe()

	queryMock.On("Create").Return(nil).Maybe()
	queryMock.On("First", mock.Anything).Return(nil).Maybe()
	queryMock.On("Delete").Return(nil).Maybe()
	queryMock.On("All", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		dest, ok := args.Get(0).(*[]any)
		if ok {
			*dest = []any{"a", "b"}
		}
	}).Maybe()

	wrapper := &DynamORMWrapper{
		db:     dbMock,
		config: DefaultConfig(),
	}

	type item struct {
		ID string
	}

	require.NoError(t, wrapper.Put(context.Background(), &item{ID: "1"}))

	var out item
	require.NoError(t, wrapper.Get(context.Background(), "1", &out))

	require.NoError(t, wrapper.Delete(context.Background(), "1"))

	res, err := wrapper.Query(context.Background(), &Query{
		PartitionKey: "pk",
		SortKey:      "sk",
		IndexName:    "gsi",
		Limit:        10,
		Filters: map[string]any{
			"status": "ok",
		},
	})
	require.NoError(t, err)
	require.Equal(t, 2, res.Count)
	require.Equal(t, 2, res.ScannedCount)
	require.Len(t, res.Items, 2)

	require.Same(t, dbMock, wrapper.GetCoreDB())

	tenant := wrapper.WithTenant("tenant-1")
	require.Equal(t, "tenant-1", tenant.tenantID)
	require.Same(t, wrapper.db, tenant.db)
}

func TestTransaction_CommitAndStateGuards(t *testing.T) {
	dbMock := mocks.NewMockExtendedDB()
	queryMock := new(dynamormmocks.MockQuery)

	dbMock.On("Model", mock.Anything).Return(queryMock).Maybe()
	queryMock.On("Create").Return(nil).Maybe()
	queryMock.On("Delete").Return(nil).Maybe()

	dbMock.On("Transaction", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		fn := args.Get(0).(func(tx *core.Tx) error)
		tx := &core.Tx{}
		tx.SetDB(dbMock)
		_ = fn(tx)
	}).Once()

	wrapper := &DynamORMWrapper{db: dbMock}
	tx, err := wrapper.BeginTransaction()
	require.NoError(t, err)

	require.NoError(t, tx.Put(context.Background(), map[string]string{"ok": "true"}))
	require.NoError(t, tx.Delete(context.Background(), "k"))
	require.NoError(t, tx.Commit())

	require.Error(t, tx.Put(context.Background(), "x"))
	require.Error(t, tx.Delete(context.Background(), "x"))
	require.Error(t, tx.Commit())
	require.Error(t, tx.Rollback())

	dbMock.AssertExpectations(t)

	// Underlying transaction failure marks rollback state.
	sentinel := errors.New("tx failed")
	dbMock2 := mocks.NewMockExtendedDB()
	dbMock2.On("Transaction", mock.Anything).Return(sentinel).Once()
	wrapper2 := &DynamORMWrapper{db: dbMock2}
	tx2, err := wrapper2.BeginTransaction()
	require.NoError(t, err)
	require.NoError(t, tx2.Put(context.Background(), "x"))
	require.ErrorIs(t, tx2.Commit(), sentinel)
	require.Error(t, tx2.Put(context.Background(), "x"))
	require.Error(t, tx2.Rollback())
}
