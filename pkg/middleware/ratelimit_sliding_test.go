package middleware

import (
	"context"
	"reflect"
	"testing"
	"time"

	dynamormcore "github.com/pay-theory/dynamorm/pkg/core"
	dynamormmocks "github.com/pay-theory/dynamorm/pkg/mocks"
	liftdynamorm "github.com/pay-theory/lift/pkg/dynamorm"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewSlidingWindowRateLimiter_ValidatesConfig(t *testing.T) {
	_, err := NewSlidingWindowRateLimiter(RateLimitConfig{DefaultWindow: 0})
	require.Error(t, err)

	_, err = NewSlidingWindowRateLimiter(RateLimitConfig{DefaultWindow: time.Second, DefaultLimit: 0})
	require.Error(t, err)

	_, err = NewSlidingWindowRateLimiter(RateLimitConfig{DefaultWindow: time.Second, DefaultLimit: 1})
	require.Error(t, err)
}

func TestSlidingWindowRateLimiter_KeyExtraction(t *testing.T) {
	mockDB := dynamormmocks.NewMockExtendedDB()
	wrapper := newTestDynamORMWrapper(t, mockDB)

	limiter, err := NewSlidingWindowRateLimiter(RateLimitConfig{
		DynamORM:      wrapper,
		DefaultWindow: time.Minute,
		DefaultLimit:  10,
		IncludePath:   true,
		IncludeMethod: true,
	})
	require.NoError(t, err)

	req := lift.NewRequest(&adapters.Request{
		Method:  "POST",
		Path:    "/v1/test",
		Headers: map[string]string{},
	})
	ctx := lift.NewContext(context.Background(), req)
	ctx.SetTenantID("tenant-1")
	ctx.SetUserID("user-1")

	key := limiter.keyExtractor(ctx)
	require.Contains(t, key, "tenant:tenant-1")
	require.Contains(t, key, "user:user-1")
	require.Contains(t, key, "path:/v1/test")
	require.Contains(t, key, "method:POST")

	ipReq := lift.NewRequest(&adapters.Request{
		Method:  "GET",
		Path:    "/v1/test",
		Headers: map[string]string{"X-Forwarded-For": "10.0.0.1"},
	})
	ipCtx := lift.NewContext(context.Background(), ipReq)
	require.Contains(t, limiter.keyExtractor(ipCtx), "ip:10.0.0.1")
}

func TestSlidingWindowRateLimiter_CheckRateLimit_AllowsWhenUnderLimit(t *testing.T) {
	mockDB := dynamormmocks.NewMockExtendedDB()
	mockQuery := new(dynamormmocks.MockQuery)

	mockDB.On("Model", mock.Anything).Return(mockQuery)
	mockQuery.On("Where", "ID", "=", mock.Anything).Return(mockQuery)
	mockQuery.On("First", mock.Anything).Run(func(args mock.Arguments) {
		fillWindowEntry(args.Get(0), time.Now(), 2)
	}).Return(nil)

	wrapper := newTestDynamORMWrapper(t, mockDB)
	limiter, err := NewSlidingWindowRateLimiter(RateLimitConfig{
		DynamORM:      wrapper,
		DefaultWindow: time.Minute,
		DefaultLimit:  5,
	})
	require.NoError(t, err)

	allowed, remaining, _, err := limiter.checkRateLimit(context.Background(), "k")
	require.NoError(t, err)
	require.True(t, allowed)
	require.Equal(t, 3, remaining)
}

func TestSlidingWindowRateLimiter_CheckRateLimit_DeniesWhenAtLimit(t *testing.T) {
	mockDB := dynamormmocks.NewMockExtendedDB()
	mockQuery := new(dynamormmocks.MockQuery)

	mockDB.On("Model", mock.Anything).Return(mockQuery)
	mockQuery.On("Where", "ID", "=", mock.Anything).Return(mockQuery)
	mockQuery.On("First", mock.Anything).Run(func(args mock.Arguments) {
		fillWindowEntry(args.Get(0), time.Now(), 5)
	}).Return(nil)

	wrapper := newTestDynamORMWrapper(t, mockDB)
	limiter, err := NewSlidingWindowRateLimiter(RateLimitConfig{
		DynamORM:      wrapper,
		DefaultWindow: time.Minute,
		DefaultLimit:  5,
	})
	require.NoError(t, err)

	allowed, remaining, _, err := limiter.checkRateLimit(context.Background(), "k")
	require.NoError(t, err)
	require.False(t, allowed)
	require.Equal(t, 0, remaining)
}

func TestSlidingWindowRateLimiter_RecordRequest_PersistsEntry(t *testing.T) {
	mockDB := dynamormmocks.NewMockExtendedDB()
	mockQuery := new(dynamormmocks.MockQuery)

	mockDB.On("Model", mock.AnythingOfType("*models.SlidingWindowEntry")).Return(mockQuery)
	mockQuery.On("Create").Return(nil)

	wrapper := newTestDynamORMWrapper(t, mockDB)
	limiter, err := NewSlidingWindowRateLimiter(RateLimitConfig{
		DynamORM:      wrapper,
		DefaultWindow: time.Minute,
		DefaultLimit:  5,
	})
	require.NoError(t, err)

	require.NoError(t, limiter.recordRequest(context.Background(), "k"))
	mockDB.AssertExpectations(t)
	mockQuery.AssertExpectations(t)
}

func TestSlidingWindowRateLimit_ReturnsHelpfulError(t *testing.T) {
	_, err := SlidingWindowRateLimit(1, time.Second)
	require.Error(t, err)
}

func newTestDynamORMWrapper(t *testing.T, db dynamormcore.ExtendedDB) *liftdynamorm.DynamORMWrapper {
	t.Helper()

	config := &liftdynamorm.DynamORMConfig{
		TableName:       "test",
		Region:          "us-east-1",
		TenantIsolation: false,
		AutoTransaction: false,
	}

	mw := liftdynamorm.WithDynamORM(config, &liftdynamorm.MockDBFactory{MockDB: db})

	var wrapper *liftdynamorm.DynamORMWrapper
	ctx := lift.NewContext(context.Background(), lift.NewRequest(&adapters.Request{Method: "GET", Path: "/"}))

	require.NoError(t, mw(lift.HandlerFunc(func(ctx *lift.Context) error {
		var err error
		wrapper, err = liftdynamorm.DB(ctx)
		require.NoError(t, err)
		return nil
	})).Handle(ctx))

	require.NotNil(t, wrapper)
	return wrapper
}

func fillWindowEntry(dest any, timestamp time.Time, count int) {
	v := reflect.ValueOf(dest)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return
	}
	elem := v.Elem()
	if elem.Kind() != reflect.Struct {
		return
	}

	ts := elem.FieldByName("Timestamp")
	if ts.IsValid() && ts.CanSet() && ts.Type() == reflect.TypeOf(time.Time{}) {
		ts.Set(reflect.ValueOf(timestamp))
	}

	c := elem.FieldByName("Count")
	if c.IsValid() && c.CanSet() && c.Kind() == reflect.Int {
		c.SetInt(int64(count))
	}
}
