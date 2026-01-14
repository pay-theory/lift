package mocks

import (
	"context"
	"testing"
	"time"

	"github.com/pay-theory/dynamorm/pkg/core"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestMockExtendedDB_DefaultExpectations(t *testing.T) {
	db := NewMockExtendedDB()
	require.NotNil(t, db)

	require.NoError(t, db.AutoMigrateWithOptions(struct{}{}))
	require.NoError(t, db.CreateTable(struct{}{}))
	require.NoError(t, db.EnsureTable(struct{}{}))
	require.NoError(t, db.DeleteTable(struct{}{}))

	_, err := db.DescribeTable(struct{}{})
	require.NoError(t, err)

	require.Same(t, db, db.WithLambdaTimeout(context.Background()))
	require.Same(t, db, db.WithLambdaTimeoutBuffer(1*time.Second))

	require.Nil(t, db.Transact())
	require.NoError(t, db.RegisterTypeConverter(nil, nil))
}

func TestMockExtendedDB_TransactionFunc(t *testing.T) {
	db := NewMockExtendedDB()
	db.On("TransactionFunc", mock.Anything).Return(nil).Once()

	require.NoError(t, db.TransactionFunc(func(_ any) error { return nil }))
	db.AssertExpectations(t)
}

type fakeTransactionBuilder struct{}

func (b *fakeTransactionBuilder) Put(_ any, _ ...core.TransactCondition) core.TransactionBuilder {
	return b
}
func (b *fakeTransactionBuilder) Create(_ any, _ ...core.TransactCondition) core.TransactionBuilder {
	return b
}
func (b *fakeTransactionBuilder) Update(_ any, _ []string, _ ...core.TransactCondition) core.TransactionBuilder {
	return b
}
func (b *fakeTransactionBuilder) UpdateWithBuilder(_ any, _ func(core.UpdateBuilder) error, _ ...core.TransactCondition) core.TransactionBuilder {
	return b
}
func (b *fakeTransactionBuilder) Delete(_ any, _ ...core.TransactCondition) core.TransactionBuilder {
	return b
}
func (b *fakeTransactionBuilder) ConditionCheck(_ any, _ ...core.TransactCondition) core.TransactionBuilder {
	return b
}
func (b *fakeTransactionBuilder) WithContext(_ context.Context) core.TransactionBuilder { return b }
func (b *fakeTransactionBuilder) Execute() error                                        { return nil }
func (b *fakeTransactionBuilder) ExecuteWithContext(_ context.Context) error            { return nil }

func TestMockExtendedDB_BranchCoverage(t *testing.T) {
	db := &MockExtendedDB{}

	db.On("WithLambdaTimeout", mock.Anything).Return(nil).Once()
	require.Nil(t, db.WithLambdaTimeout(context.Background()))

	db.On("WithLambdaTimeout", mock.Anything).Return(123).Once()
	require.Nil(t, db.WithLambdaTimeout(context.Background()))

	db.On("WithLambdaTimeoutBuffer", mock.Anything).Return(nil).Once()
	require.Nil(t, db.WithLambdaTimeoutBuffer(1*time.Second))

	db.On("WithLambdaTimeoutBuffer", mock.Anything).Return(123).Once()
	require.Nil(t, db.WithLambdaTimeoutBuffer(1*time.Second))

	db.On("Transact").Return(&fakeTransactionBuilder{}).Once()
	require.NotNil(t, db.Transact())

	db.On("Transact").Return(123).Once()
	require.Nil(t, db.Transact())

	db.On("TransactWrite", mock.Anything, mock.Anything).Return(nil).Once()
	require.NoError(t, db.TransactWrite(context.Background(), func(core.TransactionBuilder) error { return nil }))

	db.AssertExpectations(t)
}
