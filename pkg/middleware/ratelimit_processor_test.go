package middleware

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	dynamormmocks "github.com/pay-theory/dynamorm/pkg/mocks"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRateLimitMiddleware_SkipsOptionsRequests(t *testing.T) {
	db := dynamormmocks.NewMockExtendedDB()
	wrapper := newTestDynamORMWrapper(t, db)

	mw := RateLimitMiddleware(RateLimitConfig{
		DynamORM:     wrapper,
		DefaultLimit: 10,
		SkipOptions:  true,
	})

	req := lift.NewRequest(&adapters.Request{Method: "OPTIONS", Path: "/test"})
	ctx := lift.NewContext(context.Background(), req)

	called := false
	handler := mw(lift.HandlerFunc(func(_ *lift.Context) error {
		called = true
		return nil
	}))

	require.NoError(t, handler.Handle(ctx))
	require.True(t, called)
}

func TestRateLimitMiddleware_NewEntry_AllowsAndDecrementsOnSuccess(t *testing.T) {
	db := dynamormmocks.NewMockExtendedDB()

	getQuery := new(dynamormmocks.MockQuery)
	putQuery := new(dynamormmocks.MockQuery)
	decGetQuery := new(dynamormmocks.MockQuery)
	decPutQuery := new(dynamormmocks.MockQuery)

	db.On("Model", mock.Anything).Return(getQuery).Once()
	getQuery.On("Where", "ID", "=", mock.Anything).Return(getQuery).Once()
	getQuery.On("First", mock.Anything).Return(errors.New("not found")).Once()

	db.On("Model", mock.Anything).Return(putQuery).Once()
	putQuery.On("Create").Return(nil).Once()

	db.On("Model", mock.Anything).Return(decGetQuery).Once()
	decGetQuery.On("Where", "ID", "=", mock.Anything).Return(decGetQuery).Once()
	decGetQuery.On("First", mock.Anything).Run(func(args mock.Arguments) {
		entry := args.Get(0).(*RateLimitEntry)
		entry.Count = 2
	}).Return(nil).Once()

	db.On("Model", mock.Anything).Return(decPutQuery).Once()
	decPutQuery.On("Create").Return(nil).Once()

	wrapper := newTestDynamORMWrapper(t, db)
	mw := RateLimitMiddleware(RateLimitConfig{
		DynamORM:       wrapper,
		DefaultLimit:   5,
		DefaultWindow:  time.Minute,
		TTL:            time.Hour,
		SkipSuccessful: true,
		IncludeMethod:  true,
		IncludePath:    true,
	})

	req := lift.NewRequest(&adapters.Request{
		Method:  "GET",
		Path:    "/test",
		Headers: map[string]string{"X-Forwarded-For": "10.0.0.1"},
	})
	ctx := lift.NewContext(context.Background(), req)
	ctx.SetTenantID("tenant-1")
	ctx.SetUserID("user-1")

	handler := mw(lift.HandlerFunc(func(ctx *lift.Context) error {
		ctx.Response.StatusCode = 200
		return nil
	}))

	require.NoError(t, handler.Handle(ctx))
	require.Equal(t, "5", ctx.Response.Headers["X-RateLimit-Limit"])
	require.Contains(t, ctx.Response.Headers, "X-RateLimit-Remaining")
	require.Contains(t, ctx.Response.Headers, "X-RateLimit-Reset")

	db.AssertExpectations(t)
	getQuery.AssertExpectations(t)
	putQuery.AssertExpectations(t)
	decGetQuery.AssertExpectations(t)
	decPutQuery.AssertExpectations(t)
}

func TestRateLimitMiddleware_LimitExceeded_Returns429(t *testing.T) {
	db := dynamormmocks.NewMockExtendedDB()

	getQuery := new(dynamormmocks.MockQuery)
	db.On("Model", mock.Anything).Return(getQuery).Once()
	getQuery.On("Where", "ID", "=", mock.Anything).Return(getQuery).Once()
	getQuery.On("First", mock.Anything).Run(func(args mock.Arguments) {
		entry := args.Get(0).(*RateLimitEntry)
		entry.Count = 5
		entry.WindowStart = time.Now()
	}).Return(nil).Once()

	wrapper := newTestDynamORMWrapper(t, db)
	mw := RateLimitMiddleware(RateLimitConfig{
		DynamORM:      wrapper,
		DefaultLimit:  5,
		DefaultWindow: time.Minute,
	})

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/test"})
	ctx := lift.NewContext(context.Background(), req)

	handler := mw(lift.HandlerFunc(func(_ *lift.Context) error {
		t.Fatalf("next handler should not run when rate limit is exceeded")
		return nil
	}))

	require.NoError(t, handler.Handle(ctx))
	require.Equal(t, 429, ctx.Response.StatusCode)
	require.Contains(t, ctx.Response.Headers, "Retry-After")
}

func TestRateLimitMiddleware_CheckError_AllowsRequest(t *testing.T) {
	logger := &mockLogger{}
	db := dynamormmocks.NewMockExtendedDB()

	getQuery := new(dynamormmocks.MockQuery)
	putQuery := new(dynamormmocks.MockQuery)

	db.On("Model", mock.Anything).Return(getQuery).Once()
	getQuery.On("Where", "ID", "=", mock.Anything).Return(getQuery).Once()
	getQuery.On("First", mock.Anything).Return(errors.New("not found")).Once()

	db.On("Model", mock.Anything).Return(putQuery).Once()
	putQuery.On("Create").Return(errors.New("write failed")).Once()

	wrapper := newTestDynamORMWrapper(t, db)
	mw := RateLimitMiddleware(RateLimitConfig{
		DynamORM:      wrapper,
		DefaultLimit:  5,
		DefaultWindow: time.Minute,
	})

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/test"})
	ctx := lift.NewContext(context.Background(), req)
	ctx.Logger = logger

	called := false
	handler := mw(lift.HandlerFunc(func(_ *lift.Context) error {
		called = true
		return nil
	}))

	require.NoError(t, handler.Handle(ctx))
	require.True(t, called)
	require.Greater(t, len(logger.logs), 0)
}

func TestRateLimitMiddleware_ExistingEntry_AllowsAndUpdatesEntry(t *testing.T) {
	db := dynamormmocks.NewMockExtendedDB()

	getQuery := new(dynamormmocks.MockQuery)
	putQuery := new(dynamormmocks.MockQuery)

	window := time.Hour
	windowStart := time.Now().Truncate(window)

	db.On("Model", mock.Anything).Return(getQuery).Once()
	getQuery.On("Where", "ID", "=", mock.Anything).Return(getQuery).Once()
	getQuery.On("First", mock.Anything).Run(func(args mock.Arguments) {
		entry := args.Get(0).(*RateLimitEntry)
		entry.Count = 1
		entry.WindowStart = windowStart
	}).Return(nil).Once()

	db.On("Model", mock.Anything).Return(putQuery).Once()
	putQuery.On("Create").Return(nil).Once()

	wrapper := newTestDynamORMWrapper(t, db)
	mw := RateLimitMiddleware(RateLimitConfig{
		DynamORM:      wrapper,
		DefaultLimit:  5,
		DefaultWindow: window,
		TTL:           time.Hour,
	})

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/test"})
	ctx := lift.NewContext(context.Background(), req)

	called := false
	handler := mw(lift.HandlerFunc(func(ctx *lift.Context) error {
		called = true
		ctx.Response.StatusCode = 200
		return nil
	}))

	require.NoError(t, handler.Handle(ctx))
	require.True(t, called)
	require.Equal(t, "5", ctx.Response.Headers["X-RateLimit-Limit"])
	require.Contains(t, ctx.Response.Headers, "X-RateLimit-Remaining")
}

func TestRateLimitMiddleware_WindowReset_ResetsEntry(t *testing.T) {
	db := dynamormmocks.NewMockExtendedDB()

	getQuery := new(dynamormmocks.MockQuery)
	putQuery := new(dynamormmocks.MockQuery)

	window := time.Hour
	windowStart := time.Now().Truncate(window)

	db.On("Model", mock.Anything).Return(getQuery).Once()
	getQuery.On("Where", "ID", "=", mock.Anything).Return(getQuery).Once()
	getQuery.On("First", mock.Anything).Run(func(args mock.Arguments) {
		entry := args.Get(0).(*RateLimitEntry)
		entry.Count = 99
		entry.WindowStart = windowStart.Add(-2 * window)
	}).Return(nil).Once()

	db.On("Model", mock.Anything).Return(putQuery).Once()
	putQuery.On("Create").Return(nil).Once()

	wrapper := newTestDynamORMWrapper(t, db)
	mw := RateLimitMiddleware(RateLimitConfig{
		DynamORM:      wrapper,
		DefaultLimit:  5,
		DefaultWindow: window,
		TTL:           time.Hour,
	})

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/test"})
	ctx := lift.NewContext(context.Background(), req)

	called := false
	handler := mw(lift.HandlerFunc(func(ctx *lift.Context) error {
		called = true
		ctx.Response.StatusCode = 200
		return nil
	}))

	require.NoError(t, handler.Handle(ctx))
	require.True(t, called)
	require.Equal(t, "5", ctx.Response.Headers["X-RateLimit-Limit"])
	require.Contains(t, ctx.Response.Headers, "X-RateLimit-Remaining")
}

func TestRateLimiter_GetLimit_UserTenantAndDefault(t *testing.T) {
	limiter := &rateLimiter{config: RateLimitConfig{
		DefaultLimit:  100,
		UserLimits:    map[string]int{"user-1": 5},
		TenantLimits:  map[string]int{"tenant-1": 10},
		IncludeMethod: true,
		IncludePath:   true,
	}}

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/test"})
	ctx := lift.NewContext(context.Background(), req)
	require.Equal(t, 100, limiter.getLimit(ctx))

	ctx.SetTenantID("tenant-1")
	require.Equal(t, 10, limiter.getLimit(ctx))

	ctx.SetUserID("user-1")
	require.Equal(t, 5, limiter.getLimit(ctx))
}

func TestRateLimitStats_GetAndUpdate(t *testing.T) {
	_, err := GetRateLimitStats(RateLimitConfig{})
	require.Error(t, err)

	db := dynamormmocks.NewMockExtendedDB()
	wrapper := newTestDynamORMWrapper(t, db)

	getQuery := new(dynamormmocks.MockQuery)
	db.On("Model", mock.Anything).Return(getQuery).Once()
	getQuery.On("Where", "ID", "=", mock.Anything).Return(getQuery).Once()
	getQuery.On("First", mock.Anything).Return(errors.New("missing")).Once()

	stats, err := GetRateLimitStats(RateLimitConfig{DynamORM: wrapper, KeyPrefix: "ratelimit"})
	require.NoError(t, err)
	require.Equal(t, int64(0), stats.TotalRequests)

	updateGetQuery := new(dynamormmocks.MockQuery)
	updatePutQuery := new(dynamormmocks.MockQuery)

	db.On("Model", mock.Anything).Return(updateGetQuery).Once()
	updateGetQuery.On("Where", "ID", "=", mock.Anything).Return(updateGetQuery).Once()
	updateGetQuery.On("First", mock.Anything).Return(errors.New("missing")).Once()

	db.On("Model", mock.Anything).Return(updatePutQuery).Once()
	updatePutQuery.On("Create").Return(nil).Once()

	require.NoError(t, UpdateRateLimitStats(context.Background(), RateLimitConfig{
		DynamORM:  wrapper,
		KeyPrefix: "ratelimit",
	}, true, true))
}

func TestRateLimitHelpersAndKeyFuncs(t *testing.T) {
	require.NoError(t, CleanupExpiredEntries(context.Background(), RateLimitConfig{}))
	require.NotNil(t, BurstRateLimitMiddleware(RateLimitConfig{}))
	require.NotNil(t, AdaptiveRateLimitMiddleware(RateLimitConfig{}))

	req := lift.NewRequest(&adapters.Request{
		Method:  "GET",
		Path:    "/test",
		Headers: map[string]string{},
	})
	ctx := lift.NewContext(context.Background(), req)

	require.Equal(t, defaultTenant, tenantKeyFunc(ctx).Identifier)
	require.Equal(t, "anonymous", userKeyFunc(ctx).Identifier)
	require.Equal(t, "unknown", ipKeyFunc(ctx).Identifier)
	require.Equal(t, "GET:/test", endpointKeyFunc(ctx).Identifier)

	require.Equal(t, "", joinParts(nil, ":"))
	require.Equal(t, "a", joinParts([]string{"a"}, ":"))
	require.Equal(t, "a:b", joinParts([]string{"a", "b"}, ":"))

	require.Equal(t, "unknown", defaultKeyFunc(ctx).Identifier)
}

func TestFillRateLimitStatsEntry_ReflectionHelper(t *testing.T) {
	dest := &struct {
		TotalRequests   int64
		AllowedRequests int64
		BlockedRequests int64
		ErrorCount      int64
	}{}

	fillStatsEntry(dest, 1, 2, 3, 4)
	require.Equal(t, int64(1), dest.TotalRequests)
	require.Equal(t, int64(2), dest.AllowedRequests)
	require.Equal(t, int64(3), dest.BlockedRequests)
	require.Equal(t, int64(4), dest.ErrorCount)
}

func fillStatsEntry(dest any, total, allowed, blocked, errCount int64) {
	v := reflect.ValueOf(dest)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return
	}
	elem := v.Elem()
	if elem.Kind() != reflect.Struct {
		return
	}

	setInt64Field(elem, "TotalRequests", total)
	setInt64Field(elem, "AllowedRequests", allowed)
	setInt64Field(elem, "BlockedRequests", blocked)
	setInt64Field(elem, "ErrorCount", errCount)
}

func setInt64Field(v reflect.Value, field string, value int64) {
	f := v.FieldByName(field)
	if !f.IsValid() || !f.CanSet() || f.Kind() != reflect.Int64 {
		return
	}
	f.SetInt(value)
}
