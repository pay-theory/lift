package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/pay-theory/dynamorm/pkg/core"
	dynamormerrors "github.com/pay-theory/dynamorm/pkg/errors"
)

const (
	eventBusSchedulePK = "SCHEDULE"
)

// EventBusScheduledEvent represents a delayed publish request stored in the EventBus table.
//
// Scheduled events are stored in the same DynamoDB table as EventBus events to avoid requiring
// additional infrastructure. They are keyed by a fixed partition key with a sort key ordered by
// due time.
type EventBusScheduledEvent struct {
	PK string `dynamorm:"pk,attr:pk" dynamodb:"pk" json:"-"`
	SK string `dynamorm:"sk,attr:sk" dynamodb:"sk" json:"-"`

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

	Version    int `dynamodb:"version" json:"version"`
	RetryCount int `dynamodb:"retry_count" json:"retry_count"`

	LastAttemptAt time.Time `dynamodb:"last_attempt_at,omitempty" json:"last_attempt_at,omitempty"`
	LastError     string    `dynamodb:"last_error,omitempty" json:"last_error,omitempty"`

	// Lease fields are used to safely drain scheduled events with multiple concurrent drainers.
	// They are optional and only used by EventBusDrainDueScheduled.
	LeaseID    string `dynamodb:"lease_id,omitempty" json:"lease_id,omitempty"`
	LeaseOwner string `dynamodb:"lease_owner,omitempty" json:"lease_owner,omitempty"`
	LeaseUntil int64  `dynamodb:"lease_until,omitempty" json:"lease_until,omitempty"`

	TTL int64 `dynamorm:"ttl,omitempty" dynamodb:"ttl,omitempty" json:"-"`
}

func (*EventBusScheduledEvent) TableName() string {
	return (&Event{}).TableName()
}

// Event returns a services.Event suitable for publishing to the EventBus.
func (e *EventBusScheduledEvent) Event() *Event {
	if e == nil {
		return nil
	}

	return &Event{
		ID:            e.EventID,
		EventType:     e.EventType,
		TenantID:      e.TenantID,
		SourceID:      e.SourceID,
		CorrelationID: e.CorrelationID,
		Payload:       e.Payload,
		Metadata:      e.Metadata,
		Tags:          e.Tags,
		Version:       e.Version,
		RetryCount:    e.RetryCount,
	}
}

func eventBusScheduleSK(dueAt time.Time, eventID string) string {
	return fmt.Sprintf("%020d#%s", dueAt.UTC().UnixNano(), eventID)
}

func eventBusScheduleMaxSK(now time.Time) string {
	return fmt.Sprintf("%020d#~", now.UTC().UnixNano())
}

// EventBusSchedule schedules an Event to be published after `dueAt`.
//
// This is an idempotent write: if an identical scheduled item already exists, (false, nil) is returned.
func EventBusSchedule(ctx context.Context, db core.ExtendedDB, event *Event, dueAt time.Time, retention time.Duration) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("db is required")
	}
	if event == nil {
		return false, fmt.Errorf("event is required")
	}
	if event.EventType == "" {
		return false, fmt.Errorf("event.EventType is required")
	}
	if dueAt.IsZero() {
		return false, fmt.Errorf("dueAt is required")
	}

	if event.ID == "" {
		event.ID = ulid.Make().String()
	}

	now := time.Now().UTC()
	item := &EventBusScheduledEvent{
		PK:            eventBusSchedulePK,
		SK:            eventBusScheduleSK(dueAt, event.ID),
		DueAt:         dueAt.UTC(),
		CreatedAt:     now,
		EventID:       event.ID,
		EventType:     event.EventType,
		TenantID:      event.TenantID,
		SourceID:      event.SourceID,
		CorrelationID: event.CorrelationID,
		Payload:       event.Payload,
		Metadata:      event.Metadata,
		Tags:          event.Tags,
		Version:       event.Version,
		RetryCount:    event.RetryCount,
	}
	if retention > 0 {
		item.TTL = now.Add(retention).Unix()
	}

	err := db.WithContext(ctx).
		Model(item).
		IfNotExists().
		Create()
	if err != nil {
		if errors.Is(err, dynamormerrors.ErrConditionFailed) {
			return false, nil
		}
		return false, fmt.Errorf("failed to schedule event: %w", err)
	}

	return true, nil
}

// EventBusDueScheduled returns scheduled events due at or before `now`.
func EventBusDueScheduled(ctx context.Context, db core.ExtendedDB, now time.Time, limit int) ([]EventBusScheduledEvent, error) {
	if db == nil {
		return nil, fmt.Errorf("db is required")
	}

	var items []EventBusScheduledEvent
	q := db.WithContext(ctx).
		Model(&EventBusScheduledEvent{}).
		Where("PK", "=", eventBusSchedulePK).
		Where("SK", "<=", eventBusScheduleMaxSK(now))

	if limit > 0 {
		q = q.Limit(limit)
	}

	if err := q.All(&items); err != nil {
		return nil, fmt.Errorf("failed to query scheduled events: %w", err)
	}

	return items, nil
}

// EventBusDeleteScheduled deletes a scheduled event item.
func EventBusDeleteScheduled(ctx context.Context, db core.ExtendedDB, item *EventBusScheduledEvent) error {
	if db == nil {
		return fmt.Errorf("db is required")
	}
	if item == nil {
		return fmt.Errorf("item is required")
	}
	if item.PK == "" || item.SK == "" {
		return fmt.Errorf("item keys are required")
	}

	err := db.WithContext(ctx).
		Model(&EventBusScheduledEvent{}).
		Where("PK", "=", item.PK).
		Where("SK", "=", item.SK).
		Delete()
	if err != nil {
		return fmt.Errorf("failed to delete scheduled item: %w", err)
	}

	return nil
}

// EventBusClaimScheduled attempts to lease a scheduled item so only one drainer will publish it.
//
// This is safe to run concurrently: the lease update is guarded by a conditional expression that
// only succeeds when the item is currently unleased or the previous lease is expired.
func EventBusClaimScheduled(ctx context.Context, db core.ExtendedDB, item *EventBusScheduledEvent, now time.Time, leaseDuration time.Duration, owner string) (string, bool, error) {
	if db == nil {
		return "", false, fmt.Errorf("db is required")
	}
	if item == nil {
		return "", false, fmt.Errorf("item is required")
	}
	if item.PK == "" || item.SK == "" {
		return "", false, fmt.Errorf("item keys are required")
	}

	if leaseDuration <= 0 {
		leaseDuration = 2 * time.Minute
	}

	leaseID := ulid.Make().String()
	nowUnix := now.UTC().Unix()
	leaseUntil := now.UTC().Add(leaseDuration).Unix()

	leaseCondition := "attribute_not_exists(lease_until) OR lease_until < :now"
	q := db.WithContext(ctx).
		Model(&EventBusScheduledEvent{}).
		Where("PK", "=", item.PK).
		Where("SK", "=", item.SK).
		WithConditionExpression(leaseCondition, map[string]any{
			":now": nowUnix,
		})

	update := q.UpdateBuilder().
		Set("LeaseID", leaseID).
		Set("LeaseUntil", leaseUntil)
	if owner != "" {
		update = update.Set("LeaseOwner", owner)
	}

	if err := update.Execute(); err != nil {
		if errors.Is(err, dynamormerrors.ErrConditionFailed) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("failed to claim scheduled item: %w", err)
	}

	item.LeaseID = leaseID
	item.LeaseUntil = leaseUntil
	item.LeaseOwner = owner

	return leaseID, true, nil
}

// EventBusDeleteScheduledClaimed deletes a scheduled item only if it is still held by leaseID.
func EventBusDeleteScheduledClaimed(ctx context.Context, db core.ExtendedDB, item *EventBusScheduledEvent, leaseID string) error {
	if db == nil {
		return fmt.Errorf("db is required")
	}
	if item == nil {
		return fmt.Errorf("item is required")
	}
	if item.PK == "" || item.SK == "" {
		return fmt.Errorf("item keys are required")
	}
	if leaseID == "" {
		return fmt.Errorf("leaseID is required")
	}

	err := db.WithContext(ctx).
		Model(&EventBusScheduledEvent{}).
		Where("PK", "=", item.PK).
		Where("SK", "=", item.SK).
		WithCondition("LeaseID", "=", leaseID).
		Delete()
	if err != nil {
		return fmt.Errorf("failed to delete claimed scheduled item: %w", err)
	}

	return nil
}

// EventBusScheduleDrainResult summarizes the work performed by EventBusDrainDueScheduled.
type EventBusScheduleDrainResult struct {
	Due              int
	Claimed          int
	Published        int
	AlreadyPublished int
	Deleted          int
	Rescheduled      int
	Quarantined      int
}

// EventBusScheduleDrainOptions configures EventBusDrainDueScheduledWithOptions.
type EventBusScheduleDrainOptions struct {
	// LeaseDuration controls how long a drainer claims an item while publishing.
	LeaseDuration time.Duration
	// LeaseOwner is optional metadata stored alongside the lease.
	LeaseOwner string

	// ContinueOnPublishError enables retry/backoff semantics for publish failures.
	// When false, the drainer stops and returns the publish error.
	ContinueOnPublishError bool

	// MaxPublishAttempts controls when a scheduled item is quarantined after repeated publish failures.
	// When == 0, a default is applied. When < 0, items are never quarantined and will retry indefinitely.
	MaxPublishAttempts int

	// RetryBaseDelay is the base delay for exponential backoff after a publish failure.
	RetryBaseDelay time.Duration
	// RetryMaxDelay caps exponential backoff delay.
	RetryMaxDelay time.Duration

	// QuarantineRetention controls TTL on quarantine records.
	// When == 0, a default is applied. When < 0, quarantine records do not expire.
	QuarantineRetention time.Duration
}

func (o EventBusScheduleDrainOptions) withDefaults() EventBusScheduleDrainOptions {
	out := o
	if out.LeaseDuration <= 0 {
		out.LeaseDuration = 2 * time.Minute
	}
	if out.RetryBaseDelay <= 0 {
		out.RetryBaseDelay = 5 * time.Second
	}
	if out.RetryMaxDelay <= 0 {
		out.RetryMaxDelay = 5 * time.Minute
	}
	if out.MaxPublishAttempts == 0 {
		out.MaxPublishAttempts = 10
	}
	if out.QuarantineRetention == 0 {
		out.QuarantineRetention = 14 * 24 * time.Hour
	}
	return out
}

// EventBusDrainDueScheduled publishes and deletes scheduled events that are due at or before `now`.
//
// It is safe to run this on a schedule (e.g., via EventBridge) because published events use a stable
// primary key derived from (tenant_id, event_type, due_at, event_id). Scheduled rows are first
// leased/claimed so concurrent drainers won't both publish the same item. If the event already
// exists, it is treated as already published and the scheduled item is deleted.
func EventBusDrainDueScheduled(ctx context.Context, db core.ExtendedDB, bus EventBus, now time.Time, limit int) (EventBusScheduleDrainResult, error) {
	return EventBusDrainDueScheduledWithOptions(ctx, db, bus, now, limit, EventBusScheduleDrainOptions{
		ContinueOnPublishError: false,
	})
}

// EventBusDrainDueScheduledWithOptions publishes and deletes scheduled events that are due at or before `now`.
//
// When ContinueOnPublishError is enabled, publish failures are handled by:
//   - incrementing RetryCount
//   - extending LeaseUntil with exponential backoff + jitter
//   - quarantining items after MaxPublishAttempts (when > 0)
func EventBusDrainDueScheduledWithOptions(ctx context.Context, db core.ExtendedDB, bus EventBus, now time.Time, limit int, opts EventBusScheduleDrainOptions) (EventBusScheduleDrainResult, error) {
	if db == nil {
		return EventBusScheduleDrainResult{}, fmt.Errorf("db is required")
	}
	if bus == nil {
		return EventBusScheduleDrainResult{}, fmt.Errorf("bus is required")
	}

	items, err := EventBusDueScheduled(ctx, db, now, limit)
	if err != nil {
		return EventBusScheduleDrainResult{}, err
	}

	opts = opts.withDefaults()

	result := EventBusScheduleDrainResult{Due: len(items)}
	for i := range items {
		item := items[i]

		leaseID, claimed, err := EventBusClaimScheduled(ctx, db, &item, now, opts.LeaseDuration, opts.LeaseOwner)
		if err != nil {
			return result, err
		}
		if !claimed {
			continue
		}
		result.Claimed++

		event := item.Event()
		if event == nil {
			continue
		}

		// Ensure stable keys so replays are safe.
		if event.CreatedAt.IsZero() && !item.CreatedAt.IsZero() {
			event.CreatedAt = item.CreatedAt
		}
		if event.PublishedAt.IsZero() && !item.DueAt.IsZero() {
			event.PublishedAt = item.DueAt
		}
		if event.PartitionKey == "" {
			event.PartitionKey = fmt.Sprintf("%s#%s", event.TenantID, event.EventType)
		}
		if event.SortKey == "" {
			event.SortKey = fmt.Sprintf("%d#%s", item.DueAt.UTC().UnixNano(), event.ID)
		}

		if _, err := bus.Publish(ctx, event); err != nil {
			if errors.Is(err, dynamormerrors.ErrConditionFailed) {
				result.AlreadyPublished++
				if err := EventBusDeleteScheduledClaimed(ctx, db, &item, leaseID); err != nil {
					if errors.Is(err, dynamormerrors.ErrConditionFailed) {
						continue
					}
					return result, err
				}
				result.Deleted++
				continue
			}

			if !opts.ContinueOnPublishError {
				return result, fmt.Errorf("failed to publish scheduled event %q: %w", item.EventID, err)
			}

			attempt := item.RetryCount + 1
			if opts.MaxPublishAttempts > 0 && attempt >= opts.MaxPublishAttempts {
				quarantined, qErr := EventBusQuarantineScheduled(ctx, db, &item, leaseID, err, attempt, opts.QuarantineRetention)
				if qErr != nil {
					return result, qErr
				}
				if quarantined {
					result.Quarantined++
				}
				continue
			}

			delay := ExponentialBackoffWithJitter(attempt, opts.RetryBaseDelay, opts.RetryMaxDelay, item.EventID)
			if delay <= 0 {
				delay = opts.RetryBaseDelay
			}

			if err := EventBusBackoffScheduled(ctx, db, &item, leaseID, now, delay, err); err != nil {
				if errors.Is(err, dynamormerrors.ErrConditionFailed) {
					continue
				}
				return result, err
			}
			result.Rescheduled++
			continue
		}

		result.Published++
		if err := EventBusDeleteScheduledClaimed(ctx, db, &item, leaseID); err != nil {
			if errors.Is(err, dynamormerrors.ErrConditionFailed) {
				continue
			}
			return result, err
		}
		result.Deleted++
	}

	return result, nil
}
