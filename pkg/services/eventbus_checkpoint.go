package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/pay-theory/dynamorm/pkg/core"
	dynamormerrors "github.com/pay-theory/dynamorm/pkg/errors"
)

const (
	eventBusCheckpointPKPrefix = "CHECKPOINT"
	eventBusCheckpointSKPrefix = "EVENT"
)

// EventBusCheckpoint is a lightweight dedupe/checkpoint record stored in the same table as EventBus events.
//
// The primary key is scoped to a consumer name so multiple processors can independently dedupe the same event.
type EventBusCheckpoint struct {
	ProcessedAt time.Time `dynamodb:"processed_at" json:"processed_at"`

	PK string `dynamorm:"pk,attr:pk" dynamodb:"pk" json:"-"`
	SK string `dynamorm:"sk,attr:sk" dynamodb:"sk" json:"-"`

	Consumer  string `dynamodb:"consumer" json:"consumer"`
	EventID   string `dynamodb:"event_id" json:"event_id"`
	EventType string `dynamodb:"event_type,omitempty" json:"event_type,omitempty"`

	TTL int64 `dynamorm:"ttl,omitempty" dynamodb:"ttl,omitempty" json:"-"`
}

func (*EventBusCheckpoint) TableName() string {
	// Co-locate checkpoints with the EventBus table to avoid requiring an additional DynamoDB table.
	// The Event table name can be overridden process-wide via EventBusConfig.TableName.
	return (&Event{}).TableName()
}

func eventBusCheckpointKey(consumer string, eventID string) (string, string) {
	return fmt.Sprintf("%s#%s", eventBusCheckpointPKPrefix, consumer),
		fmt.Sprintf("%s#%s", eventBusCheckpointSKPrefix, eventID)
}

// EventBusIsProcessed reports whether a given consumer has already successfully processed an EventBus event.
func EventBusIsProcessed(ctx context.Context, db core.ExtendedDB, consumer string, eventID string) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("db is required")
	}
	if consumer == "" {
		return false, fmt.Errorf("consumer is required")
	}
	if eventID == "" {
		return false, fmt.Errorf("eventID is required")
	}

	pk, sk := eventBusCheckpointKey(consumer, eventID)

	var checkpoint EventBusCheckpoint
	err := db.WithContext(ctx).
		Model(&EventBusCheckpoint{}).
		Where("PK", "=", pk).
		Where("SK", "=", sk).
		First(&checkpoint)
	if err != nil {
		if errors.Is(err, dynamormerrors.ErrItemNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read checkpoint: %w", err)
	}

	return true, nil
}

// EventBusMarkProcessed records a successful processing checkpoint for an EventBus event.
//
// This uses a conditional write to ensure the checkpoint is only created once per consumer/event.
// If the checkpoint already exists, (false, nil) is returned.
func EventBusMarkProcessed(ctx context.Context, db core.ExtendedDB, consumer string, event *Event, retention time.Duration) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("db is required")
	}
	if consumer == "" {
		return false, fmt.Errorf("consumer is required")
	}
	if event == nil {
		return false, fmt.Errorf("event is required")
	}
	if event.ID == "" {
		return false, fmt.Errorf("event.ID is required")
	}

	pk, sk := eventBusCheckpointKey(consumer, event.ID)
	now := time.Now().UTC()

	checkpoint := &EventBusCheckpoint{
		PK:          pk,
		SK:          sk,
		Consumer:    consumer,
		EventID:     event.ID,
		EventType:   event.EventType,
		ProcessedAt: now,
	}
	if retention > 0 {
		checkpoint.TTL = now.Add(retention).Unix()
	}

	err := db.WithContext(ctx).
		Model(checkpoint).
		IfNotExists().
		Create()
	if err != nil {
		if errors.Is(err, dynamormerrors.ErrConditionFailed) {
			return false, nil
		}
		return false, fmt.Errorf("failed to write checkpoint: %w", err)
	}

	return true, nil
}
