package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/pay-theory/dynamorm"
	"github.com/pay-theory/dynamorm/pkg/core"
	dynamormerrors "github.com/pay-theory/dynamorm/pkg/errors"
)

const (
	eventBusQuarantineScheduledPK = "QUARANTINE#SCHEDULE"
)

// EventBusQuarantinedScheduledEvent stores a poison scheduled publish request after repeated failures.
//
// Records are co-located in the EventBus table to avoid additional infrastructure.
type EventBusQuarantinedScheduledEvent struct {
	PK string `dynamorm:"pk,attr:pk" dynamodb:"pk" json:"-"`
	SK string `dynamorm:"sk,attr:sk" dynamodb:"sk" json:"-"`

	QuarantinedAt time.Time `dynamodb:"quarantined_at" json:"quarantined_at"`
	Attempts      int       `dynamodb:"attempts" json:"attempts"`
	Cause         string    `dynamodb:"cause,omitempty" json:"cause,omitempty"`

	// Copied from the scheduled request.
	DueAt     time.Time `dynamodb:"due_at" json:"due_at"`
	CreatedAt time.Time `dynamodb:"created_at" json:"created_at"`

	EventID       string `dynamodb:"event_id" json:"event_id"`
	EventType     string `dynamodb:"event_type" json:"event_type"`
	TenantID      string `dynamodb:"tenant_id" json:"tenant_id"`
	SourceID      string `dynamodb:"source_id" json:"source_id"`
	CorrelationID string `dynamodb:"correlation_id,omitempty" json:"correlation_id,omitempty"`

	Payload  json.RawMessage   `dynamodb:"payload" json:"payload"`
	Metadata map[string]string `dynamodb:"metadata,omitempty" json:"metadata,omitempty"`
	Tags     []string          `dynamodb:"tags,omitempty" json:"tags,omitempty"`

	Version       int       `dynamodb:"version" json:"version"`
	RetryCount    int       `dynamodb:"retry_count" json:"retry_count"`
	LastAttemptAt time.Time `dynamodb:"last_attempt_at,omitempty" json:"last_attempt_at,omitempty"`
	LastError     string    `dynamodb:"last_error,omitempty" json:"last_error,omitempty"`

	TTL int64 `dynamorm:"ttl,omitempty" dynamodb:"ttl,omitempty" json:"-"`
}

func (*EventBusQuarantinedScheduledEvent) TableName() string {
	return (&Event{}).TableName()
}

// EventBusQuarantineScheduled moves a scheduled item to quarantine and deletes it from the schedule queue.
//
// The delete is conditional on the provided leaseID to ensure only the current claimant can quarantine.
// When the lease is lost, (false, nil) is returned.
func EventBusQuarantineScheduled(ctx context.Context, db core.ExtendedDB, item *EventBusScheduledEvent, leaseID string, cause error, attempts int, retention time.Duration) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("db is required")
	}
	if item == nil {
		return false, fmt.Errorf("item is required")
	}
	if item.PK == "" || item.SK == "" {
		return false, fmt.Errorf("item keys are required")
	}
	if leaseID == "" {
		return false, fmt.Errorf("leaseID is required")
	}

	now := time.Now().UTC()
	record := &EventBusQuarantinedScheduledEvent{
		PK:            eventBusQuarantineScheduledPK,
		SK:            item.SK,
		QuarantinedAt: now,
		Attempts:      attempts,
		DueAt:         item.DueAt,
		CreatedAt:     item.CreatedAt,
		EventID:       item.EventID,
		EventType:     item.EventType,
		TenantID:      item.TenantID,
		SourceID:      item.SourceID,
		CorrelationID: item.CorrelationID,
		Payload:       item.Payload,
		Metadata:      item.Metadata,
		Tags:          item.Tags,
		Version:       item.Version,
		RetryCount:    item.RetryCount,
		LastAttemptAt: item.LastAttemptAt,
		LastError:     item.LastError,
	}
	if cause != nil {
		record.Cause = cause.Error()
	}
	if retention > 0 {
		record.TTL = now.Add(retention).Unix()
	}

	err := db.TransactWrite(ctx, func(tx core.TransactionBuilder) error {
		tx.Put(record)
		tx.Delete(&EventBusScheduledEvent{
			PK: item.PK,
			SK: item.SK,
		}, dynamorm.Condition("LeaseID", "=", leaseID))
		return nil
	})
	if err != nil {
		if errors.Is(err, dynamormerrors.ErrConditionFailed) {
			return false, nil
		}
		return false, fmt.Errorf("failed to quarantine scheduled item: %w", err)
	}

	return true, nil
}

// EventBusReplayQuarantinedScheduled restores a quarantined scheduled item back into the schedule queue and removes the quarantine record.
func EventBusReplayQuarantinedScheduled(ctx context.Context, db core.ExtendedDB, item *EventBusQuarantinedScheduledEvent) error {
	if db == nil {
		return fmt.Errorf("db is required")
	}
	if item == nil {
		return fmt.Errorf("item is required")
	}
	if item.PK == "" || item.SK == "" {
		return fmt.Errorf("item keys are required")
	}

	scheduled := &EventBusScheduledEvent{
		PK:            eventBusSchedulePK,
		SK:            item.SK,
		DueAt:         item.DueAt,
		CreatedAt:     item.CreatedAt,
		EventID:       item.EventID,
		EventType:     item.EventType,
		TenantID:      item.TenantID,
		SourceID:      item.SourceID,
		CorrelationID: item.CorrelationID,
		Payload:       item.Payload,
		Metadata:      item.Metadata,
		Tags:          item.Tags,
		Version:       item.Version,
		RetryCount:    0,
	}

	err := db.TransactWrite(ctx, func(tx core.TransactionBuilder) error {
		tx.Put(scheduled)
		tx.Delete(&EventBusQuarantinedScheduledEvent{
			PK: item.PK,
			SK: item.SK,
		})
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to replay quarantined scheduled item: %w", err)
	}

	return nil
}
