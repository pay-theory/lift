package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pay-theory/dynamorm/pkg/core"
)

// EventBusBackoffScheduled increments RetryCount and extends LeaseUntil to delay the next publish attempt.
//
// The update is conditional on the provided leaseID to ensure only the current claimant can reschedule.
func EventBusBackoffScheduled(ctx context.Context, db core.ExtendedDB, item *EventBusScheduledEvent, leaseID string, now time.Time, delay time.Duration, cause error) error {
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
	if delay < 0 {
		return fmt.Errorf("delay must be >= 0")
	}

	now = now.UTC()
	nowUnix := now.Unix()
	leaseUntil := now.Add(delay).Unix()
	if leaseUntil <= nowUnix {
		leaseUntil = nowUnix + 1
	}

	lastError := ""
	if cause != nil {
		lastError = strings.TrimSpace(cause.Error())
	}

	q := db.WithContext(ctx).
		Model(&EventBusScheduledEvent{}).
		Where("PK", "=", item.PK).
		Where("SK", "=", item.SK).
		WithCondition("LeaseID", "=", leaseID).
		UpdateBuilder().
		Add("RetryCount", 1).
		Set("LeaseUntil", leaseUntil).
		Set("LastAttemptAt", now)

	if lastError != "" {
		q = q.Set("LastError", lastError)
	}

	if err := q.Execute(); err != nil {
		return fmt.Errorf("failed to backoff scheduled item: %w", err)
	}

	item.RetryCount++
	item.LeaseUntil = leaseUntil
	item.LastAttemptAt = now
	item.LastError = lastError

	return nil
}
