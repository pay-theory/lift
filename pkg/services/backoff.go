package services

import (
	"encoding/binary"
	"hash/fnv"
	"time"
)

// ExponentialBackoff returns base*2^(attempt-1) capped by maxDelay.
//
// attempt is 1-based (attempt=1 returns base). When attempt <= 0, 0 is returned.
func ExponentialBackoff(attempt int, base time.Duration, maxDelay time.Duration) time.Duration {
	if attempt <= 0 {
		return 0
	}
	if base <= 0 {
		return 0
	}

	exp := minInt(attempt-1, 30)
	delay := base * time.Duration(1<<exp)
	if maxDelay > 0 && delay > maxDelay {
		delay = maxDelay
	}
	return delay
}

// ExponentialBackoffWithJitter applies equal jitter to ExponentialBackoff using a deterministic seed.
//
// The returned delay is in the range [baseDelay/2, baseDelay] where baseDelay is the exponential value.
func ExponentialBackoffWithJitter(attempt int, base time.Duration, maxDelay time.Duration, seed string) time.Duration {
	delay := ExponentialBackoff(attempt, base, maxDelay)
	if delay <= 0 {
		return 0
	}

	half := delay / 2
	if half <= 0 {
		return delay
	}

	h := fnv.New64a()
	_, _ = h.Write([]byte(seed))
	var buf [8]byte
	if attempt < 0 {
		attempt = 0
	}
	binary.LittleEndian.PutUint64(buf[:], uint64(attempt)) //nolint:gosec // attempt is sanitized to be >= 0
	_, _ = h.Write(buf[:])

	jitterRange := half.Nanoseconds()
	if jitterRange <= 0 {
		return delay
	}

	jitter := int64(h.Sum64() % uint64(jitterRange)) //nolint:gosec // result is mod positive int64, so it fits in int64
	return half + time.Duration(jitter)*time.Nanosecond
}
