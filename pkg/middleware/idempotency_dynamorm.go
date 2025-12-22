package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/pay-theory/dynamorm/pkg/core"
	dynamormerrors "github.com/pay-theory/dynamorm/pkg/errors"

	"github.com/pay-theory/lift/pkg/dynamorm"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/models"
)

const (
	statusCompleted  = "completed"
	statusProcessing = "processing"
)

// DynamORMIdempotencyStore implements IdempotencyStore using DynamORM
type DynamORMIdempotencyStore struct {
	db      core.ExtendedDB
	wrapper *dynamorm.DynamORMWrapper
}

// NewDynamORMIdempotencyStore creates a new DynamORM-based idempotency store
// This assumes the DynamORM middleware has been configured in the Lift app
func NewDynamORMIdempotencyStore() *DynamORMIdempotencyStore {
	return &DynamORMIdempotencyStore{}
}

// NewDynamORMIdempotencyStoreWithDB creates a store using a provided DynamORM core DB.
func NewDynamORMIdempotencyStoreWithDB(db core.ExtendedDB) *DynamORMIdempotencyStore {
	return &DynamORMIdempotencyStore{
		db: db,
	}
}

// NewDynamORMIdempotencyStoreWithWrapper creates a store with a specific DynamORM wrapper
func NewDynamORMIdempotencyStoreWithWrapper(wrapper *dynamorm.DynamORMWrapper) *DynamORMIdempotencyStore {
	return &DynamORMIdempotencyStore{
		wrapper: wrapper,
	}
}

func (d *DynamORMIdempotencyStore) getDB(ctx context.Context) (core.ExtendedDB, error) {
	if d.db != nil {
		return d.db, nil
	}

	if d.wrapper != nil {
		db, ok := d.wrapper.GetCoreDB().(core.ExtendedDB)
		if !ok {
			return nil, fmt.Errorf("DynamORM wrapper does not expose core.ExtendedDB")
		}
		return db, nil
	}

	// Try to get from Lift context if available (preferred so middleware can inject DBs).
	if liftCtx, ok := ctx.(*lift.Context); ok {
		if db, ok := liftCtx.DB.(core.ExtendedDB); ok && db != nil {
			return db, nil
		}

		wrapper, err := dynamorm.TenantDB(liftCtx)
		if err == nil && wrapper != nil {
			db, ok := wrapper.GetCoreDB().(core.ExtendedDB)
			if ok && db != nil {
				return db, nil
			}
		}
	}

	return nil, fmt.Errorf("DynamORM not available - provide a core.ExtendedDB or configure DynamORM middleware")
}

// Get retrieves a stored response by key
func (d *DynamORMIdempotencyStore) Get(ctx context.Context, key string) (*IdempotencyRecord, error) {
	db, err := d.getDB(ctx)
	if err != nil {
		return nil, err
	}

	record := &models.IdempotencyRecord{}
	err = db.WithContext(ctx).
		Model(&models.IdempotencyRecord{}).
		Where("IdempotencyKey", "=", key).
		Where("SK", "=", "IDEMPOTENCY").
		First(record)
	if err != nil {
		if errors.Is(err, dynamormerrors.ErrItemNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get idempotency record: %w", err)
	}

	// Convert DynamORM model to middleware record
	middlewareRecord := &IdempotencyRecord{
		Key:          record.IdempotencyKey,
		Status:       record.Status,
		StatusCode:   record.StatusCode,
		Error:        record.ErrorMessage,
		CreatedAt:    record.CreatedAt,
		ExpiresAt:    record.ExpiresAt,
		RequestHash:  record.RequestHash,
		FunctionName: record.FunctionName,
		TenantID:     record.TenantID,
	}

	// Unmarshal response if present
	if record.Response != "" {
		if err := json.Unmarshal([]byte(record.Response), &middlewareRecord.Response); err != nil {
			// If unmarshal fails, store as string
			middlewareRecord.Response = record.Response
		}
	}

	return middlewareRecord, nil
}

// Set stores a response with the given key
func (d *DynamORMIdempotencyStore) Set(ctx context.Context, key string, record *IdempotencyRecord) error {
	db, err := d.getDB(ctx)
	if err != nil {
		return err
	}

	// Convert middleware record to DynamORM model
	dynamormRecord := &models.IdempotencyRecord{
		IdempotencyKey: key,
		SK:             "IDEMPOTENCY",
		FunctionName:   record.FunctionName,
		TenantID:       record.TenantID,
		Status:         record.Status,
		Timestamp:      record.CreatedAt,
		RequestHash:    record.RequestHash,
		StatusCode:     record.StatusCode,
		ErrorMessage:   record.Error,
		CreatedAt:      record.CreatedAt,
		UpdatedAt:      time.Now(),
		ExpiresAt:      record.ExpiresAt,
	}

	// Marshal response to JSON if present
	if record.Response != nil {
		data, err := json.Marshal(record.Response)
		if err != nil {
			return err
		}
		dynamormRecord.Response = string(data)
	}

	// If this is a completed record, set completion time
	if record.Status == statusCompleted {
		dynamormRecord.CompletedAt = time.Now()
	}

	// Use DynamORM to upsert the record
	return db.WithContext(ctx).Model(dynamormRecord).CreateOrUpdate()
}

// SetProcessing marks a key as being processed
func (d *DynamORMIdempotencyStore) SetProcessing(ctx context.Context, key string, expiresAt time.Time) error {
	db, err := d.getDB(ctx)
	if err != nil {
		return err
	}

	record := &models.IdempotencyRecord{
		IdempotencyKey: key,
		SK:             "IDEMPOTENCY",
		Status:         statusProcessing,
		Timestamp:      time.Now(),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		ExpiresAt:      expiresAt,
		LockedUntil:    expiresAt,
	}

	return db.WithContext(ctx).Model(record).IfNotExists().Create()
}

// Delete removes a key from the store
func (d *DynamORMIdempotencyStore) Delete(ctx context.Context, key string) error {
	db, err := d.getDB(ctx)
	if err != nil {
		return err
	}

	return db.WithContext(ctx).
		Model(&models.IdempotencyRecord{}).
		Where("IdempotencyKey", "=", key).
		Where("SK", "=", "IDEMPOTENCY").
		Delete()
}
