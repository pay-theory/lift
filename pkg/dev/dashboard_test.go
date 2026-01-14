package dev

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDevDashboard_Handlers(t *testing.T) {
	t.Setenv("LIFT_ENV", "prod") // avoid dev-mode mock services

	server := &DevServer{
		startTime: time.Unix(1, 0).UTC(),
		stats:     &DevStats{},
		config: &DevServerConfig{
			Port:          1234,
			ProfilerPort:  5678,
			DashboardPort: 3000,
			HotReload:     true,
			DebugMode:     true,
		},
		restartCh: make(chan struct{}, 1),
	}

	d := NewDevDashboard(server, 3000)
	require.NotNil(t, d)
	require.NotNil(t, d.logService)

	t.Run("handleStatic serves assets", func(t *testing.T) {
		rr := httptest.NewRecorder()
		d.handleStatic(rr, httptest.NewRequest(http.MethodGet, "/static/style.css", nil))
		require.Equal(t, "text/css", rr.Header().Get("Content-Type"))

		rr = httptest.NewRecorder()
		d.handleStatic(rr, httptest.NewRequest(http.MethodGet, "/static/script.js", nil))
		require.Equal(t, "application/javascript", rr.Header().Get("Content-Type"))

		rr = httptest.NewRecorder()
		d.handleStatic(rr, httptest.NewRequest(http.MethodGet, "/static/does-not-exist", nil))
		require.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("handleDashboard renders HTML", func(t *testing.T) {
		rr := httptest.NewRecorder()
		d.handleDashboard(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		require.Equal(t, http.StatusOK, rr.Code)
		require.Contains(t, rr.Header().Get("Content-Type"), "text/html")
		require.NotEmpty(t, rr.Body.String())
	})

	t.Run("handleAPILogs supports limit and search", func(t *testing.T) {
		d.logService.AddLog(LogEntry{Timestamp: time.Unix(2, 0).UTC(), Level: "INFO", Message: "hello", Fields: map[string]interface{}{"note": "world"}})
		d.logService.AddLog(LogEntry{Timestamp: time.Unix(3, 0).UTC(), Level: "INFO", Message: "other"})

		rr := httptest.NewRecorder()
		d.handleAPILogs(rr, httptest.NewRequest(http.MethodGet, "/api/logs?limit=1", nil))
		var logs []LogEntry
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &logs))
		require.Len(t, logs, 1)

		rr = httptest.NewRecorder()
		d.handleAPILogs(rr, httptest.NewRequest(http.MethodGet, "/api/logs?search=world", nil))
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &logs))
		require.Len(t, logs, 1)
	})

	t.Run("handleAPIRestart method guard + triggers", func(t *testing.T) {
		rr := httptest.NewRecorder()
		d.handleAPIRestart(rr, httptest.NewRequest(http.MethodGet, "/api/restart", nil))
		require.Equal(t, http.StatusMethodNotAllowed, rr.Code)

		rr = httptest.NewRecorder()
		d.handleAPIRestart(rr, httptest.NewRequest(http.MethodPost, "/api/restart", nil))
		require.Equal(t, "application/json", rr.Header().Get("Content-Type"))

		select {
		case <-server.restartCh:
		default:
			t.Fatalf("expected restart signal")
		}
	})

	t.Run("handleAPIStats + handleAPIHealth return JSON", func(t *testing.T) {
		rr := httptest.NewRecorder()
		d.handleAPIStats(rr, httptest.NewRequest(http.MethodGet, "/api/stats", nil))
		require.Equal(t, "application/json", rr.Header().Get("Content-Type"))

		rr = httptest.NewRecorder()
		d.handleAPIHealth(rr, httptest.NewRequest(http.MethodGet, "/api/health", nil))
		require.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	})

	t.Run("stop is safe without server", func(t *testing.T) {
		d.httpServer = nil
		require.NoError(t, d.Stop())
	})
}
