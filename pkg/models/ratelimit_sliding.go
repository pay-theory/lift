package models

import (
	"fmt"
	"time"
)

// SlidingWindowEntry represents a single request in the sliding window
type SlidingWindowEntry struct {
	// Composite key: RateLimitKey#Timestamp
	PK string `dynamorm:"pk" json:"-"`
	SK string `dynamorm:"sk" json:"-"`

	// Attributes
	RateLimitKey string    `json:"rate_limit_key"`
	Timestamp    time.Time `json:"timestamp"`
	Weight       int       `json:"weight,omitempty"` // For weighted rate limiting
	RequestID    string    `json:"request_id"`

	// TTL for automatic cleanup (set to window duration + buffer)
	ExpiresAt int64 `dynamorm:"ttl" json:"-"`
}

// Key structure for efficient queries
func (s *SlidingWindowEntry) Key(rateLimitKey string, timestamp time.Time) {
	s.PK = fmt.Sprintf("RATELIMIT#%s", rateLimitKey)
	s.SK = fmt.Sprintf("TS#%d", timestamp.UnixNano())
}

// GetTableName returns the DynamoDB table name for this model
func (s *SlidingWindowEntry) GetTableName() string {
	return "rate_limit_sliding_window"
}
