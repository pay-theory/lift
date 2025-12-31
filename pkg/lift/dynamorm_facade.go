package lift

import (
	"context"

	"github.com/pay-theory/dynamorm/pkg/core"
)

// dynamormDB is a small façade over DynamORM's core interfaces, allowing unit tests
// to provide lightweight fakes without implementing the full core.ExtendedDB surface.
type dynamormDB interface {
	WithContext(ctx context.Context) dynamormDB
	Model(model any) dynamormModel
	CreateTable(model any) error
	TransactWrite(ctx context.Context, fn func(dynamormTransaction) error) error
}

type dynamormModel interface {
	Where(field string, op string, value any) dynamormModel
	Index(indexName string) dynamormModel
	First(dest any) error
	All(dest any) error
	CreateOrUpdate() error
	Delete() error
	BatchWrite(putItems []any, deleteKeys []any) error
	UpdateBuilder() dynamormUpdateBuilder
}

type dynamormUpdateBuilder interface {
	Add(field string, value any) dynamormUpdateBuilder
	Execute() error
}

type dynamormTransaction interface {
	Put(model any)
	Delete(model any)
}

func wrapDynamormDB(db core.ExtendedDB) dynamormDB {
	return &coreDynamormDB{db: db}
}

type coreDynamormDB struct {
	db  core.ExtendedDB
	ctx context.Context
}

func (d *coreDynamormDB) WithContext(ctx context.Context) dynamormDB {
	return &coreDynamormDB{db: d.db, ctx: ctx}
}

func (d *coreDynamormDB) Model(model any) dynamormModel {
	if d.ctx == nil {
		return &coreDynamormModel{query: d.db.Model(model)}
	}
	return &coreDynamormModel{query: d.db.WithContext(d.ctx).Model(model)}
}

func (d *coreDynamormDB) CreateTable(model any) error {
	return d.db.CreateTable(model)
}

func (d *coreDynamormDB) TransactWrite(ctx context.Context, fn func(dynamormTransaction) error) error {
	return d.db.TransactWrite(ctx, func(tx core.TransactionBuilder) error {
		return fn(coreDynamormTransaction{tx: tx})
	})
}

type coreDynamormTransaction struct {
	tx core.TransactionBuilder
}

func (t coreDynamormTransaction) Put(model any) {
	t.tx.Put(model)
}

func (t coreDynamormTransaction) Delete(model any) {
	t.tx.Delete(model)
}

type coreDynamormModel struct {
	query core.Query
}

func (m *coreDynamormModel) Where(field string, op string, value any) dynamormModel {
	m.query = m.query.Where(field, op, value)
	return m
}

func (m *coreDynamormModel) Index(indexName string) dynamormModel {
	m.query = m.query.Index(indexName)
	return m
}

func (m *coreDynamormModel) First(dest any) error {
	return m.query.First(dest)
}

func (m *coreDynamormModel) All(dest any) error {
	return m.query.All(dest)
}

func (m *coreDynamormModel) CreateOrUpdate() error {
	return m.query.CreateOrUpdate()
}

func (m *coreDynamormModel) Delete() error {
	return m.query.Delete()
}

func (m *coreDynamormModel) BatchWrite(putItems []any, deleteKeys []any) error {
	return m.query.BatchWrite(putItems, deleteKeys)
}

func (m *coreDynamormModel) UpdateBuilder() dynamormUpdateBuilder {
	return &coreDynamormUpdateBuilder{builder: m.query.UpdateBuilder()}
}

type coreDynamormUpdateBuilder struct {
	builder core.UpdateBuilder
}

func (b *coreDynamormUpdateBuilder) Add(field string, value any) dynamormUpdateBuilder {
	b.builder = b.builder.Add(field, value)
	return b
}

func (b *coreDynamormUpdateBuilder) Execute() error {
	return b.builder.Execute()
}
