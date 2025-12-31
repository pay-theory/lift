package cloudwatch

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/pay-theory/lift/pkg/observability"
	"github.com/stretchr/testify/require"
)

func emptyDatum(name string) types.MetricDatum {
	return types.MetricDatum{MetricName: aws.String(name)}
}

func TestCloudWatchMetrics_AdditionalPaths(t *testing.T) {
	t.Run("MetricsBuffer Size reports current length", func(t *testing.T) {
		buffer := NewMetricsBuffer(10, 5)
		require.Equal(t, 0, buffer.Size())
		buffer.Add(emptyDatum("a"))
		require.Equal(t, 1, buffer.Size())
	})

	t.Run("WithTag and RecordBatch and lift helpers", func(t *testing.T) {
		metrics := NewCloudWatchMetrics(nil, CloudWatchMetricsConfig{
			Namespace:     "TestNamespace",
			BufferSize:    10,
			FlushSize:     100,
			FlushInterval: time.Hour,
		})
		defer func() { _ = metrics.Close() }()

		tagged := metrics.WithTag("Tenant_ID", "t1")
		_, ok := tagged.(*CloudWatchMetrics)
		require.True(t, ok)

		now := time.Now()
		require.NoError(t, metrics.RecordBatch([]*observability.MetricEntry{
			nil,
			{Name: "a", Value: 1, Unit: "Count", Timestamp: now, Tags: map[string]string{"tenant_id": "t1"}},
			{Name: "b", Value: 2, Unit: "Milliseconds", Timestamp: now, Tags: map[string]string{"user_id": "u1"}},
			{Name: "c", Value: 3, Unit: "Unknown", Timestamp: now},
		}))

		metrics.RecordLatency("op", 123*time.Millisecond)
		metrics.RecordError("op")
		metrics.RecordSuccess("op")

		// Flush should be a no-op when the underlying client is nil, but should not error.
		require.NoError(t, metrics.Flush())

		counter := metrics.Counter("c", map[string]string{"k": "v"})
		counter.Inc()
		counter.Add(2)

		hist := metrics.Histogram("h")
		hist.Observe(1)

		gauge := metrics.Gauge("g")
		gauge.Set(1)
		gauge.Inc()
		gauge.Dec()
		gauge.Add(2)
	})
}
