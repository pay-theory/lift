package lift

import (
	"os"
	"path/filepath"
	"testing"
)

type recordingLogger struct {
	debugMessages []string
	infoMessages  []string
	errorMessages []string
}

func (l *recordingLogger) Debug(message string, _ ...map[string]any) { l.debugMessages = append(l.debugMessages, message) }
func (l *recordingLogger) Info(message string, _ ...map[string]any)  { l.infoMessages = append(l.infoMessages, message) }
func (l *recordingLogger) Warn(_ string, _ ...map[string]any)        {}
func (l *recordingLogger) Error(message string, _ ...map[string]any) { l.errorMessages = append(l.errorMessages, message) }
func (l *recordingLogger) WithField(_ string, _ any) Logger          { return l }
func (l *recordingLogger) WithFields(_ map[string]any) Logger        { return l }

func TestWithDebug_AppOption(t *testing.T) {
	app := New(WithDebug())
	if !app.config.Debug {
		t.Fatalf("expected debug enabled")
	}
	if app.config.LogLevel != "DEBUG" {
		t.Fatalf("expected log level DEBUG, got %q", app.config.LogLevel)
	}
}

func TestApp_RunLocalTest_Branches(t *testing.T) {
	t.Run("no test file defined", func(t *testing.T) {
		app := New()
		logger := &recordingLogger{}
		app.logger = logger

		t.Setenv("AWS_LAMBDA_FUNCTION_NAME", "")
		t.Setenv("TEST_FILE", "")

		app.RunLocalTest()
		if len(logger.infoMessages) == 0 {
			t.Fatalf("expected info log for missing test file")
		}
	})

	t.Run("invalid test file path is rejected", func(t *testing.T) {
		app := New()
		logger := &recordingLogger{}
		app.logger = logger

		t.Setenv("AWS_LAMBDA_FUNCTION_NAME", "")
		t.Setenv("TEST_FILE", "../secrets.json")

		app.RunLocalTest()
		if len(logger.errorMessages) == 0 {
			t.Fatalf("expected error log for invalid test file path")
		}
	})

	t.Run("missing test file logs error", func(t *testing.T) {
		app := New()
		logger := &recordingLogger{}
		app.logger = logger

		t.Setenv("AWS_LAMBDA_FUNCTION_NAME", "")
		t.Setenv("TEST_FILE", filepath.Join(t.TempDir(), "missing.json"))

		app.RunLocalTest()
		if len(logger.errorMessages) == 0 {
			t.Fatalf("expected error log for missing test file")
		}
	})

	t.Run("invalid JSON logs error", func(t *testing.T) {
		app := New()
		logger := &recordingLogger{}
		app.logger = logger

		t.Setenv("AWS_LAMBDA_FUNCTION_NAME", "")
		dir := t.TempDir()
		path := filepath.Join(dir, "event.json")
		if err := os.WriteFile(path, []byte("not-json"), 0o600); err != nil {
			t.Fatalf("write file: %v", err)
		}
		t.Setenv("TEST_FILE", path)

		app.RunLocalTest()
		if len(logger.errorMessages) == 0 {
			t.Fatalf("expected error log for invalid JSON")
		}
	})

	t.Run("unknown event type logs debug on handle error", func(t *testing.T) {
		app := New()
		logger := &recordingLogger{}
		app.logger = logger

		t.Setenv("AWS_LAMBDA_FUNCTION_NAME", "")
		dir := t.TempDir()
		path := filepath.Join(dir, "event.json")
		if err := os.WriteFile(path, []byte(`{"foo":"bar"}`), 0o600); err != nil {
			t.Fatalf("write file: %v", err)
		}
		t.Setenv("TEST_FILE", path)

		app.RunLocalTest()
		if len(logger.debugMessages) == 0 {
			t.Fatalf("expected debug log for handle error")
		}
	})
}

