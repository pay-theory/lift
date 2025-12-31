package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestExponentialBackoff_EdgeCasesAndCap(t *testing.T) {
	require.Equal(t, time.Duration(0), ExponentialBackoff(0, time.Second, time.Minute))
	require.Equal(t, time.Duration(0), ExponentialBackoff(1, 0, time.Minute))

	// Caps at maxDelay when computed delay exceeds it.
	require.Equal(t, 5*time.Second, ExponentialBackoff(10, 10*time.Second, 5*time.Second))

	// No cap when maxDelay <= 0.
	require.Equal(t, 2*time.Second, ExponentialBackoff(2, 1*time.Second, 0))
}

func TestExponentialBackoffWithJitter_EdgeCasesAndDeterminism(t *testing.T) {
	require.Equal(t, time.Duration(0), ExponentialBackoffWithJitter(0, time.Second, 0, "seed"))

	// When delay is 1ns, half is 0 so the delay is returned without jitter.
	require.Equal(t, 1*time.Nanosecond, ExponentialBackoffWithJitter(1, 1*time.Nanosecond, 0, "seed"))

	base := 10 * time.Millisecond
	delay := ExponentialBackoff(2, base, 0) // 20ms
	half := delay / 2                       // 10ms

	j1 := ExponentialBackoffWithJitter(2, base, 0, "seed")
	j2 := ExponentialBackoffWithJitter(2, base, 0, "seed")

	require.Equal(t, j1, j2)
	require.GreaterOrEqual(t, j1, half)
	require.LessOrEqual(t, j1, delay)
}
