package logger

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wt-media/wt-media-cloud/internal/config"
)

func TestInitializeCreatesSixIndependentLogFiles(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfigs(dir)
	if err := Initialize(cfg); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	t.Cleanup(func() {
		if err := Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	for _, name := range []string{"app", "access", "job", "external", "audit", "panic"} {
		path := filepath.Join(dir, name+".log")
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
	}

	loggers := map[string]*slog.Logger{
		"app":      App(),
		"access":   Access(),
		"job":      Job(),
		"external": External(),
		"audit":    Audit(),
		"panic":    Panic(),
	}
	seen := make(map[*slog.Logger]struct{}, len(loggers))
	for name, entry := range loggers {
		if _, duplicate := seen[entry]; duplicate {
			t.Fatalf("logger %s duplicates another typed logger", name)
		}
		seen[entry] = struct{}{}
		entry.Info("typed logger ready", "name", name)
	}

	for name := range loggers {
		data, err := os.ReadFile(filepath.Join(dir, name+".log"))
		if err != nil {
			t.Fatalf("read %s log: %v", name, err)
		}
		if !strings.Contains(string(data), "typed logger ready") {
			t.Fatalf("%s log does not contain its own entry: %q", name, data)
		}
	}
}

func TestTypedGettersRespectIndependentFormatsAndLevels(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfigs(dir)
	cfg.App.Level = "warn"
	cfg.App.Format = "text"
	cfg.External.Level = "debug"
	cfg.External.Format = "text"

	if err := Initialize(cfg); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	t.Cleanup(func() {
		if err := Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	App().Info("app info should be filtered")
	App().Warn("app warning should be written")
	External().Debug("external debug should be written")

	appData, err := os.ReadFile(filepath.Join(dir, "app.log"))
	if err != nil {
		t.Fatalf("read app log: %v", err)
	}
	if strings.Contains(string(appData), "app info should be filtered") {
		t.Fatalf("app level was not applied: %q", appData)
	}
	if !strings.Contains(string(appData), "app warning should be written") {
		t.Fatalf("app warn entry was not written: %q", appData)
	}
	if !strings.Contains(string(appData), "time=") {
		t.Fatalf("app text format was not applied: %q", appData)
	}

	externalData, err := os.ReadFile(filepath.Join(dir, "external.log"))
	if err != nil {
		t.Fatalf("read external log: %v", err)
	}
	if !strings.Contains(string(externalData), "external debug should be written") {
		t.Fatalf("external level was not applied: %q", externalData)
	}
	if !strings.Contains(string(externalData), "time=") {
		t.Fatalf("external text format was not applied: %q", externalData)
	}
}

func TestInitializeFailureCleansOpenedFilesAndKeepsPreviousResources(t *testing.T) {
	dir := t.TempDir()
	if err := Initialize(testConfigs(dir)); err != nil {
		t.Fatalf("first Initialize() error = %v", err)
	}
	t.Cleanup(func() {
		if err := Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})
	previous := Access()

	badDir := t.TempDir()
	cfg := testConfigs(badDir)
	cfg.Panic.Path = badDir

	if err := Initialize(cfg); err == nil {
		t.Fatal("Initialize() with an invalid path succeeded")
	}
	if Access() != previous {
		t.Fatal("failed Initialize replaced the previously published logger")
	}
	Access().Info("previous logger remains active")
	data, err := os.ReadFile(filepath.Join(dir, "access.log"))
	if err != nil {
		t.Fatalf("read previous access log: %v", err)
	}
	if !strings.Contains(string(data), "previous logger remains active") {
		t.Fatalf("previous logger is not usable: %q", data)
	}
}

func TestCloseFlushesAndClosesEveryFileAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := Initialize(testConfigs(dir)); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	App().Info("app close marker")
	Access().Info("access close marker")
	Job().Info("job close marker")
	External().Info("external close marker")
	Audit().Info("audit close marker")
	Panic().Info("panic close marker")

	if err := Close(); err != nil {
		t.Fatalf("first Close() error = %v", err)
	}
	resources.mu.RLock()
	set := resources.set
	resources.mu.RUnlock()
	openFiles := 0
	if set != nil {
		for _, file := range []*os.File{set.app, set.access, set.job, set.external, set.audit, set.panic} {
			if file != nil {
				openFiles++
			}
		}
	}
	if openFiles != 0 {
		t.Fatalf("Close left %d file handles open", openFiles)
	}
	if err := Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}

	for _, name := range []string{"app", "access", "job", "external", "audit", "panic"} {
		data, err := os.ReadFile(filepath.Join(dir, name+".log"))
		if err != nil {
			t.Fatalf("read %s log: %v", name, err)
		}
		if !strings.Contains(string(data), "close marker") {
			t.Fatalf("%s log was not flushed: %q", name, data)
		}
	}
}

func TestTypedGettersFailFastBeforeInitialize(t *testing.T) {
	previous := resources.set
	resources.set = nil
	t.Cleanup(func() { resources.set = previous })

	defer func() {
		if recover() == nil {
			t.Fatal("App() did not fail fast before Initialize")
		}
	}()
	App()
}

func testConfigs(dir string) config.LoggerConfigs {
	paths := map[string]string{
		"app":      "app.log",
		"access":   "access.log",
		"job":      "job.log",
		"external": "external.log",
		"audit":    "audit.log",
		"panic":    "panic.log",
	}
	cfg := config.LoggerConfigs{
		App:      config.LoggerConfig{Path: filepath.Join(dir, paths["app"]), Level: "info", Format: "json"},
		Access:   config.LoggerConfig{Path: filepath.Join(dir, paths["access"]), Level: "info", Format: "json"},
		Job:      config.LoggerConfig{Path: filepath.Join(dir, paths["job"]), Level: "info", Format: "json"},
		External: config.LoggerConfig{Path: filepath.Join(dir, paths["external"]), Level: "info", Format: "json"},
		Audit:    config.LoggerConfig{Path: filepath.Join(dir, paths["audit"]), Level: "info", Format: "json"},
		Panic:    config.LoggerConfig{Path: filepath.Join(dir, paths["panic"]), Level: "info", Format: "json"},
	}
	return cfg
}
