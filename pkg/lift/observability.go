package lift

// Logger represents a structured logger
type Logger interface {
	Debug(message string, fields ...map[string]interface{})
	Info(message string, fields ...map[string]interface{})
	Warn(message string, fields ...map[string]interface{})
	Error(message string, fields ...map[string]interface{})
	WithField(key string, value interface{}) Logger
	WithFields(fields map[string]interface{}) Logger
}

// MetricsCollector represents a metrics collection interface
type MetricsCollector interface {
	Counter(name string, tags ...map[string]string) Counter
	Histogram(name string, tags ...map[string]string) Histogram
	Gauge(name string, tags ...map[string]string) Gauge
	Flush() error
}

// Counter represents a counter metric
type Counter interface {
	Inc()
	Add(value float64)
}

// Histogram represents a histogram metric
type Histogram interface {
	Observe(value float64)
}

// Gauge represents a gauge metric
type Gauge interface {
	Set(value float64)
	Inc()
	Dec()
	Add(value float64)
}

// NoOpLogger is a logger that does nothing (for testing)
type NoOpLogger struct{}

func (l *NoOpLogger) Debug(message string, fields ...map[string]interface{}) {}
func (l *NoOpLogger) Info(message string, fields ...map[string]interface{})  {}
func (l *NoOpLogger) Warn(message string, fields ...map[string]interface{})  {}
func (l *NoOpLogger) Error(message string, fields ...map[string]interface{}) {}
func (l *NoOpLogger) WithField(key string, value interface{}) Logger         { return l }
func (l *NoOpLogger) WithFields(fields map[string]interface{}) Logger        { return l }

// NoOpMetrics is a metrics collector that does nothing (for testing)
type NoOpMetrics struct{}

func (m *NoOpMetrics) Counter(name string, tags ...map[string]string) Counter { return &NoOpCounter{} }
func (m *NoOpMetrics) Histogram(name string, tags ...map[string]string) Histogram {
	return &NoOpHistogram{}
}
func (m *NoOpMetrics) Gauge(name string, tags ...map[string]string) Gauge { return &NoOpGauge{} }
func (m *NoOpMetrics) Flush() error                                       { return nil }

type NoOpCounter struct{}

func (c *NoOpCounter) Inc()              {}
func (c *NoOpCounter) Add(value float64) {}

type NoOpHistogram struct{}

func (h *NoOpHistogram) Observe(value float64) {}

type NoOpGauge struct{}

func (g *NoOpGauge) Set(value float64) {}
func (g *NoOpGauge) Inc()              {}
func (g *NoOpGauge) Dec()              {}
func (g *NoOpGauge) Add(value float64) {}
