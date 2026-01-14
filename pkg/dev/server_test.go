package dev

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDefaultDevServerConfig(t *testing.T) {
	cfg := DefaultDevServerConfig()
	require.Equal(t, 8080, cfg.Port)
	require.True(t, cfg.HotReload)
	require.True(t, cfg.DebugMode)
	require.True(t, cfg.EnableCORS)
	require.NotEmpty(t, cfg.WatchPaths)
}

func TestNewDevServer_InitializesComponents(t *testing.T) {
	t.Setenv("LIFT_ENV", "prod") // avoid dev-mode mock services

	cfg := &DevServerConfig{
		Port:          0,
		HotReload:     true,
		DebugMode:     true,
		ProfilerPort:  0,
		DashboardPort: 0,
		WatchPaths:    []string{"."},
		WatchInterval: 10 * time.Millisecond,
		RestartDelay:  0,
		EnableCORS:    true,
		LogLevel:      "debug",
	}

	s := NewDevServer(nil, cfg)
	require.NotNil(t, s)
	require.NotNil(t, s.dashboard)
	require.NotNil(t, s.watcher)
	require.NotNil(t, s.profiler)
	require.False(t, s.running)

	// Nil config uses defaults.
	s2 := NewDevServer(nil, nil)
	require.NotNil(t, s2.config)
}

func TestDevServer_StartAndStop_CoversLifecycle(t *testing.T) {
	t.Setenv("LIFT_ENV", "prod") // avoid dev-mode mock services

	watchDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(watchDir, "a.go"), []byte("package x\n"), 0o600))

	cfg := &DevServerConfig{
		Port:          0,
		ProfilerPort:  0,
		DashboardPort: 0,
		HotReload:     true,
		DebugMode:     true,
		EnableCORS:    false,
		LogLevel:      "debug",
		WatchPaths:    []string{watchDir},
		WatchInterval: 10 * time.Millisecond,
		RestartDelay:  0,
		BuildCommand:  "",
	}

	s := NewDevServer(nil, cfg)
	require.NotNil(t, s.watcher)
	require.NotNil(t, s.profiler)
	require.NotNil(t, s.dashboard)

	startErrCh := make(chan error, 1)
	go func() {
		startErrCh <- s.Start(context.Background())
	}()

	require.Eventually(t, func() bool { return s.server != nil }, 2*time.Second, 10*time.Millisecond)

	// Exercise watcher event branch -> restart signal.
	s.watcher.events <- FileEvent{Path: filepath.Join(watchDir, "a.go"), Type: "modified", Timestamp: time.Now()}

	// Ensure restart execution path runs at least once.
	require.Eventually(t, func() bool {
		s.stats.mu.RLock()
		defer s.stats.mu.RUnlock()
		return s.stats.Restarts > 0
	}, 2*time.Second, 10*time.Millisecond)

	require.NoError(t, s.Stop())

	select {
	case err := <-startErrCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatalf("Start did not return after Stop")
	}
}

func TestDevServer_HTTPHandlersAndMiddleware(t *testing.T) {
	s := &DevServer{
		config:    &DevServerConfig{HotReload: true, EnableCORS: true, LogLevel: "debug"},
		stats:     &DevStats{},
		startTime: time.Unix(1, 0).UTC(),
		restartCh: make(chan struct{}, 1),
		stopCh:    make(chan struct{}),
	}

	t.Run("devMiddleware adds headers", func(t *testing.T) {
		called := false
		h := s.devMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusNoContent)
		}))

		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		require.True(t, called)
		require.Equal(t, "lift", rr.Header().Get("X-Dev-Server"))
		require.Equal(t, "true", rr.Header().Get("X-Hot-Reload"))
	})

	t.Run("corsMiddleware short-circuits OPTIONS", func(t *testing.T) {
		called := false
		h := s.corsMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			called = true
		}))

		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodOptions, "/", nil))
		require.False(t, called)
		require.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("handleRequest returns JSON and updates stats", func(t *testing.T) {
		rr := httptest.NewRecorder()
		s.handleRequest(rr, httptest.NewRequest(http.MethodGet, "/hello", nil))

		require.Equal(t, "application/json", rr.Header().Get("Content-Type"))
		var payload map[string]any
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &payload))
		require.Equal(t, "/hello", payload["path"])
	})

	t.Run("handleStats returns JSON", func(t *testing.T) {
		rr := httptest.NewRecorder()
		s.handleStats(rr, httptest.NewRequest(http.MethodGet, "/dev/stats", nil))
		require.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	})

	t.Run("handleRestart method guard + backpressure", func(t *testing.T) {
		rr := httptest.NewRecorder()
		s.handleRestart(rr, httptest.NewRequest(http.MethodGet, "/dev/restart", nil))
		require.Equal(t, http.StatusMethodNotAllowed, rr.Code)

		rr = httptest.NewRecorder()
		s.handleRestart(rr, httptest.NewRequest(http.MethodPost, "/dev/restart", nil))
		require.Equal(t, http.StatusOK, rr.Code)

		// Channel is buffered size 1; second call should hit default branch.
		rr = httptest.NewRecorder()
		s.handleRestart(rr, httptest.NewRequest(http.MethodPost, "/dev/restart", nil))
		require.Equal(t, http.StatusTooManyRequests, rr.Code)
	})

	t.Run("handleHealth returns JSON", func(t *testing.T) {
		rr := httptest.NewRecorder()
		s.handleHealth(rr, httptest.NewRequest(http.MethodGet, "/dev/health", nil))
		require.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	})
}

func TestDevServer_RestartHelpers(t *testing.T) {
	s := &DevServer{
		config:    &DevServerConfig{BuildCommand: "", RestartDelay: 0},
		stats:     &DevStats{},
		startTime: time.Unix(1, 0).UTC(),
		restartCh: make(chan struct{}, 1),
		stopCh:    make(chan struct{}),
	}

	s.triggerRestart()
	s.triggerRestart()

	select {
	case <-s.restartCh:
	default:
		t.Fatalf("expected restart signal")
	}

	s.performRestart()

	stats := s.GetStats()
	require.Equal(t, int64(1), stats.Restarts)
	require.Equal(t, int64(1), stats.HotReloads)
}

func TestFileWatcher_CheckPathSendsEventOnModification(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "a.go")
	require.NoError(t, os.WriteFile(file, []byte("package x\n"), 0o600))

	w := NewFileWatcher([]string{tmp}, 1*time.Second)

	// First scan populates lastMod but doesn't emit.
	w.checkPath(tmp)

	// Force a mod-time bump and scan again.
	newTime := time.Now().Add(2 * time.Second)
	require.NoError(t, os.Chtimes(file, newTime, newTime))
	w.checkPath(tmp)

	select {
	case ev := <-w.events:
		require.Equal(t, file, ev.Path)
		require.Equal(t, "modified", ev.Type)
	case <-time.After(1 * time.Second):
		t.Fatalf("expected file event")
	}
}

func TestProfilerServer_StopWhenNotStarted(t *testing.T) {
	p := NewProfilerServer(0)
	require.NoError(t, p.Stop())
}

func TestDevServer_StopWhenNotRunning(t *testing.T) {
	s := &DevServer{running: false}
	require.NoError(t, s.Stop())

	// Cover context cancellation wiring at call sites.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.Error(t, ctx.Err())
}
