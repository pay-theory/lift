package lift

import (
	"context"
	"testing"

	"github.com/pay-theory/dynamorm/pkg/core"
	"github.com/stretchr/testify/require"
)

type fakeExtendedDB struct {
	core.ExtendedDB

	modelCalls          int
	withContextCalls    int
	createTableCalls    int
	transactWriteCalls  int
	lastModel           any
	lastWithContext     context.Context
	lastCreateTable     any
	lastTransactContext context.Context

	query *fakeQuery
	tx    *fakeTxBuilder
}

func (f *fakeExtendedDB) WithContext(ctx context.Context) core.DB {
	f.withContextCalls++
	f.lastWithContext = ctx
	return f
}

func (f *fakeExtendedDB) Model(model any) core.Query {
	f.modelCalls++
	f.lastModel = model
	if f.query == nil {
		f.query = &fakeQuery{}
	}
	return f.query
}

func (f *fakeExtendedDB) CreateTable(model any, _ ...any) error {
	f.createTableCalls++
	f.lastCreateTable = model
	return nil
}

func (f *fakeExtendedDB) TransactWrite(ctx context.Context, fn func(core.TransactionBuilder) error) error {
	f.transactWriteCalls++
	f.lastTransactContext = ctx
	if f.tx == nil {
		f.tx = &fakeTxBuilder{}
	}
	return fn(f.tx)
}

type fakeTxBuilder struct {
	core.TransactionBuilder

	puts    []any
	deletes []any
}

func (t *fakeTxBuilder) Put(model any, _ ...core.TransactCondition) core.TransactionBuilder {
	t.puts = append(t.puts, model)
	return t
}

func (t *fakeTxBuilder) Delete(model any, _ ...core.TransactCondition) core.TransactionBuilder {
	t.deletes = append(t.deletes, model)
	return t
}

type fakeQuery struct {
	core.Query

	whereCalls []struct {
		field string
		op    string
		value any
	}
	indexCalls          []string
	firstCalls          int
	allCalls            int
	createOrUpdateCalls int
	deleteCalls         int
	batchWriteCalls     int
	updateBuilderCalls  int

	lastFirstDest any
	lastAllDest   any
	lastPutItems  []any
	lastDelKeys   []any

	updateBuilder *fakeUpdateBuilder
}

func (q *fakeQuery) Where(field string, op string, value any) core.Query {
	q.whereCalls = append(q.whereCalls, struct {
		field string
		op    string
		value any
	}{field: field, op: op, value: value})
	return q
}

func (q *fakeQuery) Index(indexName string) core.Query {
	q.indexCalls = append(q.indexCalls, indexName)
	return q
}

func (q *fakeQuery) First(dest any) error {
	q.firstCalls++
	q.lastFirstDest = dest
	return nil
}

func (q *fakeQuery) All(dest any) error {
	q.allCalls++
	q.lastAllDest = dest
	return nil
}

func (q *fakeQuery) CreateOrUpdate() error {
	q.createOrUpdateCalls++
	return nil
}

func (q *fakeQuery) Delete() error {
	q.deleteCalls++
	return nil
}

func (q *fakeQuery) BatchWrite(putItems []any, deleteKeys []any) error {
	q.batchWriteCalls++
	q.lastPutItems = putItems
	q.lastDelKeys = deleteKeys
	return nil
}

func (q *fakeQuery) UpdateBuilder() core.UpdateBuilder {
	q.updateBuilderCalls++
	if q.updateBuilder == nil {
		q.updateBuilder = &fakeUpdateBuilder{}
	}
	return q.updateBuilder
}

type fakeUpdateBuilder struct {
	core.UpdateBuilder

	addCalls []struct {
		field string
		value any
	}
	executeCalls int
}

func (b *fakeUpdateBuilder) Add(field string, value any) core.UpdateBuilder {
	b.addCalls = append(b.addCalls, struct {
		field string
		value any
	}{field: field, value: value})
	return b
}

func (b *fakeUpdateBuilder) Execute() error {
	b.executeCalls++
	return nil
}

func TestWrapDynamormDB_Model_ContextHandling(t *testing.T) {
	underlying := &fakeExtendedDB{}
	db := wrapDynamormDB(underlying)

	model := db.Model("model-1")
	require.NotNil(t, model)
	require.Equal(t, 1, underlying.modelCalls)
	require.Equal(t, 0, underlying.withContextCalls)

	ctx := context.WithValue(context.Background(), "k", "v")
	model = db.WithContext(ctx).Model("model-2")
	require.NotNil(t, model)
	require.Equal(t, 2, underlying.modelCalls)
	require.Equal(t, 1, underlying.withContextCalls)
	require.Equal(t, ctx, underlying.lastWithContext)
}

func TestWrapDynamormDB_CreateTable_Forwards(t *testing.T) {
	underlying := &fakeExtendedDB{}
	db := wrapDynamormDB(underlying)

	require.NoError(t, db.CreateTable("table-model"))
	require.Equal(t, 1, underlying.createTableCalls)
	require.Equal(t, "table-model", underlying.lastCreateTable)
}

func TestWrapDynamormDB_TransactWrite_WrapsTransactionBuilder(t *testing.T) {
	underlying := &fakeExtendedDB{}
	db := wrapDynamormDB(underlying)

	ctx := context.WithValue(context.Background(), "tx", "1")
	require.NoError(t, db.TransactWrite(ctx, func(tx dynamormTransaction) error {
		tx.Put("put-1")
		tx.Delete("del-1")
		return nil
	}))

	require.Equal(t, 1, underlying.transactWriteCalls)
	require.Equal(t, ctx, underlying.lastTransactContext)
	require.Equal(t, []any{"put-1"}, underlying.tx.puts)
	require.Equal(t, []any{"del-1"}, underlying.tx.deletes)
}

func TestCoreDynamormModel_ForwardsQueryCalls(t *testing.T) {
	underlying := &fakeExtendedDB{}
	db := wrapDynamormDB(underlying)

	model := db.Model("model")
	firstDest := &struct{}{}
	allDest := &[]struct{}{}

	require.NoError(t, model.Where("field", "=", "value").
		Index("gsi1").
		First(firstDest))
	require.NoError(t, model.All(allDest))
	require.NoError(t, model.CreateOrUpdate())
	require.NoError(t, model.Delete())
	require.NoError(t, model.BatchWrite([]any{"put"}, []any{"del"}))

	ub := model.UpdateBuilder()
	require.NoError(t, ub.Add("count", 1).Execute())

	q := underlying.query
	require.Len(t, q.whereCalls, 1)
	require.Equal(t, "field", q.whereCalls[0].field)
	require.Equal(t, "=", q.whereCalls[0].op)
	require.Equal(t, "value", q.whereCalls[0].value)
	require.Equal(t, []string{"gsi1"}, q.indexCalls)
	require.Equal(t, 1, q.firstCalls)
	require.Equal(t, firstDest, q.lastFirstDest)
	require.Equal(t, 1, q.allCalls)
	require.Equal(t, allDest, q.lastAllDest)
	require.Equal(t, 1, q.createOrUpdateCalls)
	require.Equal(t, 1, q.deleteCalls)
	require.Equal(t, 1, q.batchWriteCalls)
	require.Equal(t, []any{"put"}, q.lastPutItems)
	require.Equal(t, []any{"del"}, q.lastDelKeys)
	require.Equal(t, 1, q.updateBuilderCalls)
	require.Len(t, q.updateBuilder.addCalls, 1)
	require.Equal(t, "count", q.updateBuilder.addCalls[0].field)
	require.Equal(t, 1, q.updateBuilder.addCalls[0].value)
	require.Equal(t, 1, q.updateBuilder.executeCalls)
}
