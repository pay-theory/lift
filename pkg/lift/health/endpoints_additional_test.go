package health

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
)

type recordingLogger struct {
	errorCalls int
	lastMsg    string
	lastFields map[string]any
}

func (l *recordingLogger) Debug(string, ...map[string]any) {}
func (l *recordingLogger) Info(string, ...map[string]any)  {}
func (l *recordingLogger) Warn(string, ...map[string]any)  {}
func (l *recordingLogger) Error(message string, fields ...map[string]any) {
	l.errorCalls++
	l.lastMsg = message
	if len(fields) > 0 {
		l.lastFields = fields[0]
	}
}
func (l *recordingLogger) WithField(string, any) lift.Logger     { return l }
func (l *recordingLogger) WithFields(map[string]any) lift.Logger { return l }

type failingResponseWriter struct {
	header     http.Header
	statusCode int
}

func (w *failingResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *failingResponseWriter) WriteHeader(statusCode int) { w.statusCode = statusCode }
func (w *failingResponseWriter) Write([]byte) (int, error)  { return 0, errors.New("write failed") }

func TestHealthEndpoints_logErrorAndWriteFailures(t *testing.T) {
	manager := NewHealthManager(DefaultHealthManagerConfig())

	logger := &recordingLogger{}
	endpoints := NewHealthEndpoints(manager, HealthEndpointsConfig{
		Logger:               logger,
		EnableDetailedErrors: true,
		EnableCORS:           false,
		Timeout:              time.Second,
	})

	w := &failingResponseWriter{}
	endpoints.writeJSONResponse(w, http.StatusOK, HealthStatus{
		Status:    StatusHealthy,
		Timestamp: time.Now(),
		Duration:  time.Microsecond,
		Message:   "ok",
		Details:   map[string]any{"k": "v"},
	})
	if logger.errorCalls == 0 {
		t.Fatal("expected structured logger to record an error on JSON encode failure")
	}

	endpoints.writePlainTextResponse(w, http.StatusOK, HealthStatus{
		Status:    StatusHealthy,
		Timestamp: time.Now(),
		Duration:  time.Microsecond,
		Message:   "ok",
	})

	endpoints.writeError(w, http.StatusInternalServerError, "boom")

	// Cover fallback to std log without asserting output.
	noLogger := NewHealthEndpoints(manager, HealthEndpointsConfig{Timeout: time.Second})
	noLogger.logError("msg", errors.New("boom"))
}

func TestHealthEndpoints_LivenessAndReadiness_PlainTextAndMethodNotAllowed(t *testing.T) {
	manager := NewHealthManager(DefaultHealthManagerConfig())
	config := DefaultHealthEndpointsConfig()
	config.EnableCORS = false
	endpoints := NewHealthEndpoints(manager, config)

	t.Run("liveness plain text", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health/live?format=text", nil)
		w := httptest.NewRecorder()
		endpoints.LivenessHandler(w, req)
		if w.Header().Get("Content-Type") != "text/plain" {
			t.Fatalf("expected text/plain content type, got %q", w.Header().Get("Content-Type"))
		}
	})

	t.Run("liveness method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/health/live", nil)
		w := httptest.NewRecorder()
		endpoints.LivenessHandler(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
		}
	})

	t.Run("readiness plain text", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health/ready?format=text", nil)
		w := httptest.NewRecorder()
		endpoints.ReadinessHandler(w, req)
		if w.Header().Get("Content-Type") != "text/plain" {
			t.Fatalf("expected text/plain content type, got %q", w.Header().Get("Content-Type"))
		}
	})

	t.Run("readiness method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/health/ready", nil)
		w := httptest.NewRecorder()
		endpoints.ReadinessHandler(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
		}
	})
}
