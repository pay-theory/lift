package lift

import "testing"

func TestNoOpObservability_DoesNotPanic(t *testing.T) {
	logger := &NoOpLogger{}
	logger.Debug("d", map[string]any{"k": "v"})
	logger.Info("i")
	logger.Warn("w")
	logger.Error("e")

	logger2 := logger.WithField("k", "v").WithFields(map[string]any{"a": 1})
	logger2.Debug("d2")

	metrics := &NoOpMetrics{}
	metrics.Counter("c").Inc()
	metrics.Counter("c").Add(1)
	metrics.Histogram("h").Observe(1)

	g := metrics.Gauge("g")
	g.Set(1)
	g.Inc()
	g.Dec()
	g.Add(2)

	if err := metrics.Flush(); err != nil {
		t.Fatalf("expected Flush() to return nil, got %v", err)
	}
}
