package models

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIdempotencyRecord_TableName(t *testing.T) {
	t.Run("uses explicit env override", func(t *testing.T) {
		t.Setenv("IDEMPOTENCY_TABLE_NAME", "idempotency_override")
		t.Setenv("APP_NAME", "")
		t.Setenv("STAGE", "")

		rec := &IdempotencyRecord{}
		require.Equal(t, "idempotency_override", rec.TableName())
	})

	t.Run("uses naming env when available", func(t *testing.T) {
		t.Setenv("IDEMPOTENCY_TABLE_NAME", "")
		t.Setenv("APP_NAME", "myapp")
		t.Setenv("STAGE", "lab")
		t.Setenv("PARTNER", "")

		rec := &IdempotencyRecord{}
		require.Equal(t, "myapp-idempotency-lab", rec.TableName())
	})

	t.Run("falls back to default", func(t *testing.T) {
		t.Setenv("IDEMPOTENCY_TABLE_NAME", "")
		t.Setenv("APP_NAME", "")
		t.Setenv("STAGE", "")

		rec := &IdempotencyRecord{}
		require.Equal(t, "idempotency", rec.TableName())
	})
}

func TestIdempotencyRecord_RequestAndResponseLifecycle(t *testing.T) {
	rec := NewIdempotencyRecord("key", "fn")
	require.Equal(t, "key", rec.IdempotencyKey)
	require.Equal(t, "fn", rec.FunctionName)
	require.Equal(t, IdempotencyStatusPending, rec.Status)
	require.False(t, rec.CreatedAt.IsZero())
	require.False(t, rec.ExpiresAt.IsZero())
	require.True(t, rec.ExpiresAt.After(rec.CreatedAt))

	type req struct {
		A string `json:"a"`
	}
	require.NoError(t, rec.SetRequest(req{A: "b"}))
	require.NotEmpty(t, rec.RequestHash)
	require.Contains(t, rec.RequestBody, `"a":"b"`)

	type resp struct {
		OK bool `json:"ok"`
	}
	require.NoError(t, rec.SetResponse(resp{OK: true}, 201))
	require.Equal(t, IdempotencyStatusCompleted, rec.Status)
	require.Equal(t, 201, rec.StatusCode)
	require.Contains(t, rec.Response, `"ok":true`)

	var gotResp resp
	require.NoError(t, rec.GetResponse(&gotResp))
	require.Equal(t, resp{OK: true}, gotResp)

	var gotReq req
	require.NoError(t, rec.GetRequest(&gotReq))
	require.Equal(t, req{A: "b"}, gotReq)
}

func TestIdempotencyRecord_ErrorAndRetryLogic(t *testing.T) {
	rec := NewIdempotencyRecord("key", "fn")

	rec.SetError(errors.New("boom"))
	require.Equal(t, IdempotencyStatusFailed, rec.Status)
	require.Equal(t, 1, rec.RetryCount)
	require.Equal(t, "boom", rec.ErrorMessage)
	require.True(t, rec.CanRetry())

	rec.RetryCount = 3
	require.False(t, rec.CanRetry())

	rec.Status = IdempotencyStatusProcessing
	rec.LockedUntil = time.Now().Add(1 * time.Minute)
	require.True(t, rec.IsLocked())

	rec.LockedUntil = time.Now().Add(-1 * time.Minute)
	require.False(t, rec.IsLocked())
}

func TestHashRequest_ErrorOnUnmarshalable(t *testing.T) {
	_, err := HashRequest(map[string]any{"bad": make(chan int)})
	require.Error(t, err)
}

func TestRateLimitRecord_TableNameAndLifecycle(t *testing.T) {
	t.Run("table name env override", func(t *testing.T) {
		t.Setenv("RATE_LIMIT_TABLE_NAME", "rate_limits_override")
		t.Setenv("APP_NAME", "")
		t.Setenv("STAGE", "")

		rec := &RateLimitRecord{}
		require.Equal(t, "rate_limits_override", rec.TableName())
	})

	t.Run("table name from naming env", func(t *testing.T) {
		t.Setenv("RATE_LIMIT_TABLE_NAME", "")
		t.Setenv("APP_NAME", "myapp")
		t.Setenv("STAGE", "lab")
		t.Setenv("PARTNER", "")

		rec := &RateLimitRecord{}
		require.Equal(t, "myapp-rate-limits-lab", rec.TableName())
	})

	t.Run("table name default", func(t *testing.T) {
		t.Setenv("RATE_LIMIT_TABLE_NAME", "")
		t.Setenv("APP_NAME", "")
		t.Setenv("STAGE", "")

		rec := &RateLimitRecord{}
		require.Equal(t, "rate-limits", rec.TableName())
	})

	window := time.Date(2026, 1, 1, 1, 2, 3, 0, time.UTC)
	rec := NewRateLimitRecord("id", window)
	require.Equal(t, "id", rec.Identifier)
	require.Equal(t, window.Format(time.RFC3339), rec.WindowTime)
	require.Equal(t, 0, rec.Count)
	require.False(t, rec.CreatedAt.IsZero())
	require.False(t, rec.ExpiresAt.IsZero())

	rec.IncrementCount()
	require.Equal(t, 1, rec.Count)

	rec.SetIdentifierMetadata("ip", "1.1.1.1")
	rec.SetIdentifierMetadata("user", "u1")
	rec.SetIdentifierMetadata("tenant", "t1")
	require.Equal(t, "1.1.1.1", rec.IPAddress)
	require.Equal(t, "u1", rec.UserID)
	require.Equal(t, "t1", rec.TenantID)

	rec.ExpiresAt = time.Now().Add(-1 * time.Second)
	require.True(t, rec.IsExpired())
}

func TestSlidingWindowEntry_KeyAndTableName(t *testing.T) {
	entry := &SlidingWindowEntry{}
	ts := time.Date(2026, 1, 1, 1, 2, 3, 4, time.UTC)
	entry.Key("rk", ts)

	require.Equal(t, "RATELIMIT#rk", entry.PK)
	require.Contains(t, entry.SK, "TS#")
	require.Equal(t, "rate_limit_sliding_window", entry.GetTableName())
}
