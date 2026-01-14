package zap

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sns"
	"go.uber.org/zap/zapcore"

	"github.com/pay-theory/lift/pkg/observability"
	"github.com/stretchr/testify/require"
)

type fakeSNSClient struct {
	err error
}

func (c fakeSNSClient) Publish(_ context.Context, _ *sns.PublishInput, _ ...func(*sns.Options)) (*sns.PublishOutput, error) {
	return nil, c.err
}

func TestBuildZapConfig_LevelAndEncoding(t *testing.T) {
	cfg := observability.LoggerConfig{Level: "debug", Format: "json"}
	zcfg := buildZapConfig(cfg)
	require.Equal(t, zapcore.DebugLevel, zcfg.Level.Level())
	require.Equal(t, "json", zcfg.Encoding)

	cfg.Level = "warn"
	zcfg = buildZapConfig(cfg)
	require.Equal(t, zapcore.WarnLevel, zcfg.Level.Level())

	cfg.Level = "error"
	zcfg = buildZapConfig(cfg)
	require.Equal(t, zapcore.ErrorLevel, zcfg.Level.Level())

	cfg.Level = "unknown"
	zcfg = buildZapConfig(cfg)
	require.Equal(t, zapcore.InfoLevel, zcfg.Level.Level())

	cfg.Level = "info"
	cfg.Format = "console"
	zcfg = buildZapConfig(cfg)
	require.Equal(t, "console", zcfg.Encoding)
}

func TestZapLogger_ContextAndStats(t *testing.T) {
	logger, err := NewZapLogger(observability.LoggerConfig{Level: "debug", Format: "json"})
	require.NoError(t, err)

	logger.Debug("d", map[string]any{"password": "secret"})
	logger.Info("i", map[string]any{"k": "v"})
	logger.Warn("w", map[string]any{"k": "v"})

	stats := logger.GetStats()
	require.Equal(t, int64(3), stats.EntriesLogged)
	require.True(t, logger.IsHealthy())

	// Context methods keep returning structured loggers.
	with := logger.WithRequestID("r1").WithTenantID("t1").WithUserID("u1").WithTraceID("tr1").WithSpanID("sp1")
	zl, ok := with.(*ZapLogger)
	require.True(t, ok)
	require.Equal(t, "r1", zl.contextFields["request_id"])
	require.Equal(t, "t1", zl.contextFields["tenant_id"])
	require.Equal(t, "u1", zl.contextFields["user_id"])
	require.Equal(t, "tr1", zl.contextFields["trace_id"])
	require.Equal(t, "sp1", zl.contextFields["span_id"])

	// Flush updates stats even if Sync reports an error on some platforms.
	_ = logger.Flush(context.Background())
	require.GreaterOrEqual(t, logger.stats.flushCount, int64(1))
	require.NotZero(t, logger.stats.lastFlush)

	_ = logger.Close()
}

func TestZapLogger_ErrorWithSNSNotifierUpdatesHealth(t *testing.T) {
	notifier := observability.NewSNSNotifier(observability.SNSConfig{
		Client:   fakeSNSClient{err: errors.New("publish failed")},
		TopicARN: "arn:aws:sns:us-east-1:123456789012:topic",
	})

	logger, err := NewZapLogger(observability.LoggerConfig{Level: "debug", Format: "json"}, WithSNSNotifier(notifier))
	require.NoError(t, err)

	logger.Error("boom", map[string]any{"ts": time.Unix(1, 0).UTC()})

	stats := logger.GetStats()
	require.Equal(t, int64(1), stats.ErrorCount)
	require.Contains(t, stats.LastError, "SNS notification failed")
	require.False(t, logger.IsHealthy())
}

func TestZapLoggerFactory_CreatesLoggers(t *testing.T) {
	factory := NewZapLoggerFactory()
	require.NotNil(t, factory)

	console, err := factory.CreateConsoleLogger(observability.LoggerConfig{Level: "info"})
	require.NoError(t, err)
	require.NotNil(t, console)

	cw, err := factory.CreateCloudWatchLogger(observability.LoggerConfig{Level: "info"}, nil)
	require.NoError(t, err)
	require.NotNil(t, cw)

	testLogger := factory.CreateTestLogger()
	require.NotNil(t, testLogger)

	noOp := factory.CreateNoOpLogger()
	require.NotNil(t, noOp)
	_ = noOp.WithRequestID("r").WithTenantID("t").WithUserID("u").WithTraceID("tr").WithSpanID("sp")
	require.NoError(t, noOp.Flush(context.Background()))
	require.NoError(t, noOp.Close())
	require.True(t, noOp.IsHealthy())
	require.Equal(t, observability.LoggerStats{}, noOp.GetStats())
}

func TestZapLoggerOptions_Constructors(t *testing.T) {
	t.Setenv("ERROR_NOTIFICATION_SNS_TOPIC_ARN", "")

	require.NotNil(t, WithSNSNotifier(nil))
	require.NotNil(t, WithEnvironmentErrorNotifications(nil))
	require.NotNil(t, WithPartnerErrorNotifications(nil))
	require.NotNil(t, WithErrorNotifications(nil, "arn:aws:sns:us-east-1:123456789012:topic"))
}
