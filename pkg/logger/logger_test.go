package logger_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wt-media/wt-media-cloud/pkg/logger"
)

func TestNewWritesNormalAndWarningFatalCompanionLogs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	instance, err := logger.New(logger.Config{
		Path:   path,
		Level:  "info",
		Format: "json",
		Rotation: logger.RotationConfig{
			MaxSize:    10,
			MaxAge:     30,
			MaxBackups: 10,
			Compress:   true,
			LocalTime:  true,
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	instance.Infof("normal info marker")
	instance.Warnf("normal warning marker")
	instance.Errorf("normal error marker")
	if err := instance.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	normal := read(t, path)
	wf := read(t, path+".wf")
	for _, marker := range []string{"normal info marker", "normal warning marker", "normal error marker"} {
		if !strings.Contains(normal, marker) {
			t.Fatalf("normal log missing %q: %q", marker, normal)
		}
	}
	if strings.Contains(wf, "normal info marker") {
		t.Fatalf("wf log unexpectedly contains info: %q", wf)
	}
	for _, marker := range []string{"normal warning marker", "normal error marker"} {
		if !strings.Contains(wf, marker) {
			t.Fatalf("wf log missing %q: %q", marker, wf)
		}
	}
}

func TestNewSupportsJSONConsoleAndTextFormats(t *testing.T) {
	dir := t.TempDir()
	tests := map[string]string{
		"json":    "json.log",
		"console": "console.log",
		"text":    "text.log",
	}
	for format, filename := range tests {
		t.Run(format, func(t *testing.T) {
			path := filepath.Join(dir, filename)
			instance, err := logger.New(logger.Config{Path: path, Level: "info", Format: format})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			instance.Infof("format marker")
			if err := instance.Close(); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
			data := read(t, path)
			if !strings.Contains(data, "format marker") {
				t.Fatalf("log missing marker: %q", data)
			}
			if format == "json" && !strings.Contains(data, `"level":"INFO"`) && !strings.Contains(data, `"level":"info"`) {
				t.Fatalf("json format missing level: %q", data)
			}
			if format != "json" && strings.Contains(data, `{"level"`) {
				t.Fatalf("%s format unexpectedly emitted JSON: %q", format, data)
			}
		})
	}
}

func TestNewExtractsConfiguredContextFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "context.log")
	instance, err := logger.New(logger.Config{Path: path, Level: "info", Format: "json"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer instance.Close()

	ctx := context.WithValue(context.Background(), "trace_id", "tr-test")
	ctx = context.WithValue(ctx, "request_id", "rq-test")
	ctx = context.WithValue(ctx, "user_id", int64(7))
	instance.CtxInfof(ctx, "context marker")
	if err := instance.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	data := read(t, path)
	for _, field := range []string{`"trace_id":"tr-test"`, `"request_id":"rq-test"`, `"user_id":7`} {
		if !strings.Contains(data, field) {
			t.Fatalf("context log missing %s: %q", field, data)
		}
	}
}

func TestNewRejectsInvalidSettings(t *testing.T) {
	dir := t.TempDir()
	tests := []logger.Config{
		{Path: "", Level: "info", Format: "json"},
		{Path: filepath.Join(dir, "level.log"), Level: "verbose", Format: "json"},
		{Path: filepath.Join(dir, "format.log"), Level: "info", Format: "xml"},
	}
	for index, cfg := range tests {
		if _, err := logger.New(cfg); err == nil {
			t.Fatalf("tests[%d] New() error = nil", index)
		}
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "close.log")
	instance, err := logger.New(logger.Config{Path: path, Level: "info", Format: "json"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := instance.Close(); err != nil {
		t.Fatalf("first Close() error = %v", err)
	}
	if err := instance.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
