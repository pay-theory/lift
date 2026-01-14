package dev

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pay-theory/lift/pkg/features"
	"github.com/stretchr/testify/require"
)

func TestLogService_Basics(t *testing.T) {
	ls := NewLogService("", nil)

	ls.AddLog(LogEntry{Timestamp: time.Unix(1, 0).UTC(), Level: "INFO", Message: "one"})
	ls.AddLog(LogEntry{Timestamp: time.Unix(2, 0).UTC(), Level: "INFO", Message: "two"})

	require.Len(t, ls.GetRecentLogs(1), 1)
	require.Len(t, ls.GetRecentLogs(0), 2)
	require.Len(t, ls.GetRecentLogs(999), 2)

	since := time.Unix(1500, 0).UTC()
	ls.AddLog(LogEntry{Timestamp: since.Add(1 * time.Second), Level: "INFO", Message: "after"})
	require.Len(t, ls.GetLogsSince(since), 1)

	ls.AddLog(LogEntry{
		Timestamp: time.Unix(3, 0).UTC(),
		Level:     "INFO",
		Message:   "hello",
		Fields:    map[string]interface{}{"note": "world"},
	})
	require.Len(t, ls.SearchLogs("HELLO"), 1)
	require.Len(t, ls.SearchLogs("world"), 1)

	ls.Clear()
	require.Len(t, ls.GetRecentLogs(10), 0)

	// Stop closes the channel only when watching is set.
	ls.watching = true
	ls.Stop()
	require.False(t, ls.watching)
}

func TestLogService_GetRecentLogsHardCap(t *testing.T) {
	ls := NewLogService("", nil)
	for i := 0; i < 10; i++ {
		ls.AddLog(LogEntry{Timestamp: time.Unix(int64(i+1), 0).UTC(), Level: "INFO", Message: "m"})
	}

	ls.maxLogs = 3
	got := ls.GetRecentLogs(10)
	require.Len(t, got, 3)
}

func TestLogService_ParseAndReadFile(t *testing.T) {
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "app.log")

	lines := []string{
		`{"timestamp":"2026-01-01T00:00:00Z","level":"INFO","message":"json"}`,
		`2026-01-01T00:00:01Z INFO plain`,
		`not-a-time INFO fallback-time`,
		`just a line`,
	}
	require.NoError(t, os.WriteFile(logFile, []byte(lines[0]+"\n"+lines[1]+"\n"+lines[2]+"\n"+lines[3]+"\n"), 0o600))

	ls := NewLogService("", nil)
	ls.logFile = logFile
	ls.readExistingLogs()
	require.Greater(t, len(ls.logs), 0)

	info, err := os.Stat(logFile)
	require.NoError(t, err)
	offset := info.Size()

	// Append a line and read only new content.
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	_, err = f.WriteString(`2026-01-01T00:00:02Z INFO appended` + "\n")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	ls.readNewLogs(offset)
	require.Greater(t, len(ls.logs), 0)

	// parseLogLine fallbacks
	require.NotNil(t, ls.parseLogLine(lines[0]))
	require.NotNil(t, ls.parseLogLine(lines[1]))
	require.NotNil(t, ls.parseLogLine(lines[2]))
	require.NotNil(t, ls.parseLogLine(lines[3]))
}

func TestLogService_WatchLogFileStops(t *testing.T) {
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "watch.log")
	require.NoError(t, os.WriteFile(logFile, []byte("line\n"), 0o600))

	ls := NewLogService("", nil)
	ls.logFile = logFile

	done := make(chan struct{})
	go func() {
		ls.watchLogFile()
		close(done)
	}()

	require.Eventually(t, func() bool { return ls.watching }, 1*time.Second, 10*time.Millisecond)
	ls.Stop()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatalf("watchLogFile did not stop")
	}
}

func TestMockLogService_GeneratesLogsAndStops(t *testing.T) {
	mls := NewMockLogService(nil)
	require.Greater(t, len(mls.GetRecentLogs(10)), 0)

	// Stop() only closes stopWatching when watching is set.
	mls.watching = true
	mls.Stop()
}

func TestLogServiceFactory_ReturnsRealServiceWhenMockDisabled(t *testing.T) {
	t.Setenv("LIFT_ENV", "prod")
	ff, err := features.NewFeatureFlags(features.FeatureFlagConfig{LocalOnly: true})
	require.NoError(t, err)

	ls := LogServiceFactory("", ff)
	require.NotNil(t, ls)
	require.Equal(t, "", ls.logFile)
}
