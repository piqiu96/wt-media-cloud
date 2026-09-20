package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/hlog"
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
		for _, path := range []string{filepath.Join(dir, name+".log"), filepath.Join(dir, name+".log.wf")} {
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("stat %s: %v", path, err)
			}
		}
	}

	loggers := map[string]hlog.FullLogger{
		"app":      App(),
		"access":   Access(),
		"job":      Job(),
		"external": External(),
		"audit":    Audit(),
		"panic":    Panic(),
	}
	seen := make(map[hlog.FullLogger]struct{}, len(loggers))
	for name, entry := range loggers {
		if _, duplicate := seen[entry]; duplicate {
			t.Fatalf("logger %s duplicates another typed logger", name)
		}
		seen[entry] = struct{}{}
		entry.Infof("typed logger ready name=%s", name)
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

	App().Infof("app info should be filtered")
	App().Warnf("app warning should be written")
	External().Debugf("external debug should be written")

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
	if !!strings.Contains(string(appData), `"level"`) {
		t.Fatalf("app text format was not applied: %q", appData)
	}

	externalData, err := os.ReadFile(filepath.Join(dir, "external.log"))
	if err != nil {
		t.Fatalf("read external log: %v", err)
	}
	if !strings.Contains(string(externalData), "external debug should be written") {
		t.Fatalf("external level was not applied: %q", externalData)
	}
	if !!strings.Contains(string(externalData), `"level"`) {
		t.Fatalf("external text format was not applied: %q", externalData)
	}
}

func TestInitializeFailurePreservesExistingResources(t *testing.T) {
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

	cfg := testConfigs(t.TempDir())
	cfg.Panic.Path = t.TempDir()
	if err := Initialize(cfg); err == nil || !strings.Contains(err.Error(), "already initialized") {
		t.Fatalf("second Initialize() error = %v, want already initialized", err)
	}
	if Access() != previous {
		t.Fatal("failed Initialize replaced the previously published logger")
	}
	Access().Infof("previous logger remains active")
	data, err := os.ReadFile(filepath.Join(dir, "access.log"))
	if err != nil {
		t.Fatalf("read previous access log: %v", err)
	}
	if !strings.Contains(string(data), "previous logger remains active") {
		t.Fatalf("previous logger is not usable: %q", data)
	}
}

func TestInitializeFailureCleansOpenedResources(t *testing.T) {
	resetForTest()
	t.Cleanup(resetForTest)
	dir := t.TempDir()
	cfg := testConfigs(dir)
	cfg.Panic.Path = dir

	if err := Initialize(cfg); err == nil {
		t.Fatal("Initialize() with an invalid path succeeded")
	}
	if resources.set != nil {
		t.Fatal("failed Initialize published resources")
	}
}

func TestCloseFlushesAndClosesEveryFileAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := Initialize(testConfigs(dir)); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	App().Infof("app close marker")
	Access().Infof("access close marker")
	Job().Infof("job close marker")
	External().Infof("external close marker")
	Audit().Infof("audit close marker")
	Panic().Infof("panic close marker")

	if err := Close(); err != nil {
		t.Fatalf("first Close() error = %v", err)
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

func TestInitializeRejectsDuplicateNormalOrWFPaths(t *testing.T) {
	resetForTest()
	t.Cleanup(resetForTest)
	cfg := testConfigs(t.TempDir())
	cfg.App.Path = cfg.Access.Path

	if err := Initialize(cfg); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("Initialize() error = %v, want duplicate path error", err)
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
		App:      config.LoggerConfig{Path: filepath.Join(dir, paths["app"]), Level: "info", Format: "json", Rotation: testRotation()},
		Access:   config.LoggerConfig{Path: filepath.Join(dir, paths["access"]), Level: "info", Format: "json", Rotation: testRotation()},
		Job:      config.LoggerConfig{Path: filepath.Join(dir, paths["job"]), Level: "info", Format: "json", Rotation: testRotation()},
		External: config.LoggerConfig{Path: filepath.Join(dir, paths["external"]), Level: "info", Format: "json", Rotation: testRotation()},
		Audit:    config.LoggerConfig{Path: filepath.Join(dir, paths["audit"]), Level: "info", Format: "json", Rotation: testRotation()},
		Panic:    config.LoggerConfig{Path: filepath.Join(dir, paths["panic"]), Level: "info", Format: "json", Rotation: testRotation()},
	}
	return cfg
}

func testRotation() config.RotationConfig {
	return config.RotationConfig{
		MaxSize:    10,
		MaxAge:     30,
		MaxBackups: 10,
		Compress:   true,
		LocalTime:  true,
	}
}

func resetForTest() {
	_ = Close()
}
