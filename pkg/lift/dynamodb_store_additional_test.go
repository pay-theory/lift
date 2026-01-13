package lift

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	dynamormerrors "github.com/pay-theory/dynamorm/pkg/errors"
	"github.com/pay-theory/lift/pkg/naming"
	"github.com/stretchr/testify/require"
)

type fakeDynamormDB struct {
	mu sync.Mutex

	items map[string]any

	failCreateOrUpdate error
	failDelete         error
	failFirst          error
	failAll            error
	failUpdateExecute  error
	failBatchWrite     error
	failTransactWrite  error

	createTableCalls int
}

func newFakeDynamormDB() *fakeDynamormDB {
	return &fakeDynamormDB{items: make(map[string]any)}
}

func (db *fakeDynamormDB) WithContext(_ context.Context) dynamormDB {
	return db
}

func (db *fakeDynamormDB) Model(model any) dynamormModel {
	return &fakeDynamormModel{db: db, model: model}
}

func (db *fakeDynamormDB) CreateTable(_ any) error {
	db.mu.Lock()
	db.createTableCalls++
	db.mu.Unlock()
	return nil
}

func (db *fakeDynamormDB) TransactWrite(_ context.Context, fn func(dynamormTransaction) error) error {
	if db.failTransactWrite != nil {
		return db.failTransactWrite
	}

	tx := &fakeDynamormTx{db: db}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.commit()
}

type fakeDynamormTx struct {
	db      *fakeDynamormDB
	puts    []any
	deletes []any
}

func (tx *fakeDynamormTx) Put(model any) {
	tx.puts = append(tx.puts, model)
}

func (tx *fakeDynamormTx) Delete(model any) {
	tx.deletes = append(tx.deletes, model)
}

func (tx *fakeDynamormTx) commit() error {
	tx.db.mu.Lock()
	defer tx.db.mu.Unlock()

	for _, model := range tx.puts {
		key, stored, ok := dynamormKeyAndValue(model)
		if !ok {
			return fmt.Errorf("unsupported transact put model: %T", model)
		}
		tx.db.items[key] = stored
	}

	for _, model := range tx.deletes {
		key, _, ok := dynamormKeyAndValue(model)
		if !ok {
			return fmt.Errorf("unsupported transact delete model: %T", model)
		}
		delete(tx.db.items, key)
	}

	return nil
}

type fakeDynamormModel struct {
	db    *fakeDynamormDB
	model any

	index  string
	wheres []whereCond
}

type whereCond struct {
	field string
	op    string
	value any
}

func (m *fakeDynamormModel) Where(field string, op string, value any) dynamormModel {
	m.wheres = append(m.wheres, whereCond{field: field, op: op, value: value})
	return m
}

func (m *fakeDynamormModel) Index(indexName string) dynamormModel {
	m.index = indexName
	return m
}

func (m *fakeDynamormModel) First(dest any) error {
	if m.db.failFirst != nil {
		return m.db.failFirst
	}

	pk, sk, ok := m.pkSkEquals()
	if !ok {
		return dynamormerrors.ErrItemNotFound
	}

	m.db.mu.Lock()
	item, found := m.db.items[compositeKey(pk, sk)]
	m.db.mu.Unlock()

	if !found {
		return dynamormerrors.ErrItemNotFound
	}

	switch d := dest.(type) {
	case *dynamormConnectionRecord:
		*d = item.(dynamormConnectionRecord)
		return nil
	case *dynamormSubscriptionRecord:
		*d = item.(dynamormSubscriptionRecord)
		return nil
	default:
		return fmt.Errorf("unsupported First dest: %T", dest)
	}
}

func (m *fakeDynamormModel) All(dest any) error {
	if m.db.failAll != nil {
		return m.db.failAll
	}

	m.db.mu.Lock()
	defer m.db.mu.Unlock()

	switch d := dest.(type) {
	case *[]dynamormConnectionRecord:
		var out []dynamormConnectionRecord
		for _, item := range m.db.items {
			rec, ok := item.(dynamormConnectionRecord)
			if !ok {
				continue
			}
			if !m.matchesConnection(rec) {
				continue
			}
			out = append(out, rec)
		}
		*d = out
		return nil

	case *[]dynamormSubscriptionRecord:
		var out []dynamormSubscriptionRecord
		for _, item := range m.db.items {
			rec, ok := item.(dynamormSubscriptionRecord)
			if !ok {
				continue
			}
			if !m.matchesSubscription(rec) {
				continue
			}
			out = append(out, rec)
		}
		*d = out
		return nil

	default:
		return fmt.Errorf("unsupported All dest: %T", dest)
	}
}

func (m *fakeDynamormModel) CreateOrUpdate() error {
	if m.db.failCreateOrUpdate != nil {
		return m.db.failCreateOrUpdate
	}

	key, stored, ok := dynamormKeyAndValue(m.model)
	if !ok {
		return fmt.Errorf("unsupported CreateOrUpdate model: %T", m.model)
	}

	m.db.mu.Lock()
	m.db.items[key] = stored
	m.db.mu.Unlock()
	return nil
}

func (m *fakeDynamormModel) Delete() error {
	if m.db.failDelete != nil {
		return m.db.failDelete
	}

	pk, sk, ok := m.pkSkEquals()
	if !ok {
		return nil
	}

	m.db.mu.Lock()
	delete(m.db.items, compositeKey(pk, sk))
	m.db.mu.Unlock()
	return nil
}

func (m *fakeDynamormModel) BatchWrite(_ []any, deleteKeys []any) error {
	if m.db.failBatchWrite != nil {
		return m.db.failBatchWrite
	}

	m.db.mu.Lock()
	defer m.db.mu.Unlock()

	for _, key := range deleteKeys {
		k, _, ok := dynamormKeyAndValue(key)
		if !ok {
			return fmt.Errorf("unsupported batch delete key: %T", key)
		}
		delete(m.db.items, k)
	}

	return nil
}

func (m *fakeDynamormModel) UpdateBuilder() dynamormUpdateBuilder {
	return &fakeDynamormUpdateBuilder{db: m.db, model: m}
}

func (m *fakeDynamormModel) pkSkEquals() (string, string, bool) {
	var pk string
	var sk string
	for _, cond := range m.wheres {
		if cond.op != "=" {
			continue
		}
		switch cond.field {
		case "PK":
			pk, _ = cond.value.(string)
		case "SK":
			sk, _ = cond.value.(string)
		}
	}
	if pk == "" || sk == "" {
		return "", "", false
	}
	return pk, sk, true
}

func (m *fakeDynamormModel) matchesConnection(rec dynamormConnectionRecord) bool {
	for _, cond := range m.wheres {
		value, _ := cond.value.(string)
		switch {
		case cond.field == "GSI1PK" && value != "" && rec.GSI1PK != value:
			return false
		case cond.field == "GSI2PK" && value != "" && rec.GSI2PK != value:
			return false
		case cond.field == "PK" && cond.op == "=" && value != "" && rec.PK != value:
			return false
		case cond.field == "SK" && cond.op == "=" && value != "" && rec.SK != value:
			return false
		}
	}
	return true
}

func (m *fakeDynamormModel) matchesSubscription(rec dynamormSubscriptionRecord) bool {
	for _, cond := range m.wheres {
		value, _ := cond.value.(string)
		switch cond.field {
		case "PK":
			if cond.op == "=" && value != "" && rec.PK != value {
				return false
			}
		case "SK":
			if cond.op == "=" && value != "" && rec.SK != value {
				return false
			}
			if cond.op == "BEGINS_WITH" && value != "" && len(rec.SK) >= len(value) && rec.SK[:len(value)] != value {
				return false
			}
		}
	}
	return true
}

type fakeDynamormUpdateBuilder struct {
	db    *fakeDynamormDB
	model *fakeDynamormModel

	adds map[string]any
}

func (b *fakeDynamormUpdateBuilder) Add(field string, value any) dynamormUpdateBuilder {
	if b.adds == nil {
		b.adds = make(map[string]any)
	}
	b.adds[field] = value
	return b
}

func (b *fakeDynamormUpdateBuilder) Execute() error {
	if b.db.failUpdateExecute != nil {
		return b.db.failUpdateExecute
	}

	pk, sk, ok := b.model.pkSkEquals()
	if !ok {
		return nil
	}

	delta := int64(0)
	if v, ok := b.adds["Count"]; ok {
		switch n := v.(type) {
		case int64:
			delta = n
		case int:
			delta = int64(n)
		}
	}

	key := compositeKey(pk, sk)

	b.db.mu.Lock()
	defer b.db.mu.Unlock()

	item, found := b.db.items[key]
	if !found {
		b.db.items[key] = dynamormConnectionRecord{PK: pk, SK: sk, Count: delta}
		return nil
	}

	rec, ok := item.(dynamormConnectionRecord)
	if !ok {
		return fmt.Errorf("unexpected counter record type: %T", item)
	}
	rec.Count += delta
	b.db.items[key] = rec
	return nil
}

func compositeKey(pk, sk string) string {
	return pk + "|" + sk
}

func dynamormKeyAndValue(model any) (string, any, bool) {
	switch v := model.(type) {
	case *dynamormConnectionRecord:
		if v == nil || v.PK == "" || v.SK == "" {
			return "", nil, false
		}
		return compositeKey(v.PK, v.SK), *v, true
	case *dynamormSubscriptionRecord:
		if v == nil || v.PK == "" || v.SK == "" {
			return "", nil, false
		}
		return compositeKey(v.PK, v.SK), *v, true
	default:
		return "", nil, false
	}
}

func setWebSocketTableOverrideForTest(t *testing.T, value string) {
	t.Helper()

	webSocketConnectionsTableNameMu.Lock()
	prev := webSocketConnectionsTableNameOverride
	webSocketConnectionsTableNameOverride = value
	webSocketConnectionsTableNameMu.Unlock()

	t.Cleanup(func() {
		webSocketConnectionsTableNameMu.Lock()
		webSocketConnectionsTableNameOverride = prev
		webSocketConnectionsTableNameMu.Unlock()
	})
}

func TestWebSocketConnectionsTableNameOverride_AndEnvNaming(t *testing.T) {
	setWebSocketTableOverrideForTest(t, "")

	t.Setenv(naming.EnvAppName, "app")
	t.Setenv(naming.EnvStage, "lab")
	t.Setenv(naming.EnvTenant, "tenant")

	require.Equal(t, "app-tenant-websocket-connections-lab", (&dynamormConnectionRecord{}).TableName())

	require.NoError(t, setWebSocketConnectionsTableNameOverride("override-table"))
	require.Equal(t, "override-table", getWebSocketConnectionsTableNameOverride())
	require.Equal(t, "override-table", (&dynamormConnectionRecord{}).TableName())

	require.NoError(t, setWebSocketConnectionsTableNameOverride("override-table"))
	require.Error(t, setWebSocketConnectionsTableNameOverride("different-table"))
}

func TestDynamoDBConnectionStore_CRUDAndQueries(t *testing.T) {
	setWebSocketTableOverrideForTest(t, "")

	db := newFakeDynamormDB()
	store := &DynamoDBConnectionStore{db: db, ttlHours: 1}

	require.Error(t, store.Save(context.Background(), nil))
	require.Error(t, store.Save(context.Background(), &Connection{}))

	conn := &Connection{
		ID:        "conn-1",
		UserID:    "user-1",
		TenantID:  "tenant-1",
		CreatedAt: "now",
		Metadata:  map[string]any{"k": "v"},
	}
	require.NoError(t, store.Save(context.Background(), conn))

	require.Error(t, func() error { _, err := store.Get(context.Background(), ""); return err }())

	got, err := store.Get(context.Background(), "missing")
	require.NoError(t, err)
	require.Nil(t, got)

	got, err = store.Get(context.Background(), "conn-1")
	require.NoError(t, err)
	require.Equal(t, conn.ID, got.ID)
	require.Equal(t, conn.UserID, got.UserID)
	require.Equal(t, conn.TenantID, got.TenantID)
	require.Equal(t, conn.Metadata, got.Metadata)

	require.Error(t, func() error { _, err := store.ListByUser(context.Background(), ""); return err }())
	conns, err := store.ListByUser(context.Background(), "user-1")
	require.NoError(t, err)
	require.Len(t, conns, 1)
	require.Equal(t, "conn-1", conns[0].ID)

	require.Error(t, func() error { _, err := store.ListByTenant(context.Background(), ""); return err }())
	conns, err = store.ListByTenant(context.Background(), "tenant-1")
	require.NoError(t, err)
	require.Len(t, conns, 1)
	require.Equal(t, "conn-1", conns[0].ID)

	count, err := store.CountActive(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(1), count)

	require.Error(t, store.Delete(context.Background(), ""))
	require.NoError(t, store.Delete(context.Background(), "conn-1"))

	count, err = store.CountActive(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(0), count)

	db.mu.Lock()
	db.items[compositeKey(connectionCounterPK, connectionCounterSK)] = dynamormConnectionRecord{PK: connectionCounterPK, SK: connectionCounterSK, Count: -3}
	db.mu.Unlock()

	count, err = store.CountActive(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(0), count)

	require.NoError(t, store.CreateTable(context.Background()))
	require.Equal(t, 1, db.createTableCalls)
}

func TestDynamoDBConnectionStore_CounterFailuresAreNonFatal(t *testing.T) {
	db := newFakeDynamormDB()
	db.failUpdateExecute = errors.New("fail update")

	store := &DynamoDBConnectionStore{db: db, ttlHours: 1}
	require.NoError(t, store.Save(context.Background(), &Connection{ID: "conn-1"}))
	require.NoError(t, store.Delete(context.Background(), "conn-1"))
}

func TestDynamoDBConnectionStore_CountActive_MissingCounter(t *testing.T) {
	db := newFakeDynamormDB()
	store := &DynamoDBConnectionStore{db: db, ttlHours: 1}

	count, err := store.CountActive(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(0), count)
}

func TestDynamoDBSubscriptionStore_SubscribeListUnsubscribeAndDelete(t *testing.T) {
	db := newFakeDynamormDB()
	store := &DynamoDBSubscriptionStore{db: db, ttlHours: 1}

	var nilStore *DynamoDBSubscriptionStore
	require.Error(t, nilStore.Subscribe(context.Background(), "t", "c", "topic"))
	require.Error(t, (&DynamoDBSubscriptionStore{}).Subscribe(context.Background(), "t", "c", "topic"))

	require.Error(t, store.Subscribe(context.Background(), "", "c", "topic"))
	require.Error(t, store.Subscribe(context.Background(), "t", "", "topic"))
	require.Error(t, store.Subscribe(context.Background(), "t", "c", "   "))

	require.NoError(t, store.Subscribe(context.Background(), "t", "c", "  topic "))

	ids, err := store.ListByTopic(context.Background(), "t", "topic")
	require.NoError(t, err)
	require.Equal(t, []string{"c"}, ids)

	subs, err := store.ListByConnection(context.Background(), "c")
	require.NoError(t, err)
	require.Equal(t, []Subscription{{TenantID: "t", Topic: "topic", ConnectionID: "c"}}, subs)

	require.NoError(t, store.Unsubscribe(context.Background(), "t", "c", "topic"))
	ids, err = store.ListByTopic(context.Background(), "t", "topic")
	require.NoError(t, err)
	require.Empty(t, ids)

	// Create enough subscriptions to cross the 25-item batch limit (each sub creates 2 keys).
	for i := 0; i < 13; i++ {
		require.NoError(t, store.Subscribe(context.Background(), "t", "c", fmt.Sprintf("topic-%02d", i)))
	}

	deleted, err := store.DeleteByConnection(context.Background(), "c")
	require.NoError(t, err)
	require.Equal(t, 26, deleted)

	subs, err = store.ListByConnection(context.Background(), "c")
	require.NoError(t, err)
	require.Empty(t, subs)
}

func TestDynamoDBSubscriptionStore_DeleteByConnection_BatchErrorReturnsPartialCount(t *testing.T) {
	db := newFakeDynamormDB()
	store := &DynamoDBSubscriptionStore{db: db, ttlHours: 1}

	for i := 0; i < 2; i++ {
		require.NoError(t, store.Subscribe(context.Background(), "t", "c", fmt.Sprintf("topic-%02d", i)))
	}

	db.failBatchWrite = errors.New("batch failed")
	deleted, err := store.DeleteByConnection(context.Background(), "c")
	require.Error(t, err)
	require.Equal(t, 0, deleted)
}

func TestDynamoDBSubscriptionStore_TransactErrorsPropagate(t *testing.T) {
	db := newFakeDynamormDB()
	db.failTransactWrite = errors.New("tx failed")
	store := &DynamoDBSubscriptionStore{db: db, ttlHours: 1}

	err := store.Subscribe(context.Background(), "t", "c", "topic")
	require.Error(t, err)
}

func TestDynamoDBSubscriptionStore_TTLIsSet(t *testing.T) {
	db := newFakeDynamormDB()
	store := &DynamoDBSubscriptionStore{db: db, ttlHours: 1}

	require.NoError(t, store.Subscribe(context.Background(), "t", "c", "topic"))

	db.mu.Lock()
	defer db.mu.Unlock()
	item, ok := db.items[compositeKey(subscriptionTopicPK("t", "topic"), subscriptionTopicSK("c"))]
	require.True(t, ok)
	rec := item.(dynamormSubscriptionRecord)
	require.Greater(t, rec.TTL, time.Now().Unix())
}
