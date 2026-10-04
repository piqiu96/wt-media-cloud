package config

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func mkdirForTest(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create directory %s: %v", dir, err)
	}
}

func TestHomeHonoursReleaseHomeOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv(HomeEnvVar, home)

	if got, want := Home(), home; got != want {
		t.Fatalf("Home() = %q, want %q", got, want)
	}
	if got, want := ConfigDir(), filepath.Join(home, "config"); got != want {
		t.Fatalf("ConfigDir() = %q, want %q", got, want)
	}
	if got, want := LogDir(), filepath.Join(home, "logs"); got != want {
		t.Fatalf("LogDir() = %q, want %q", got, want)
	}
	if got, want := WebDir(), filepath.Join(home, "web"); got != want {
		t.Fatalf("WebDir() = %q, want %q", got, want)
	}
}

func TestHomePathsHonourIndividualOverrides(t *testing.T) {
	home := t.TempDir()
	config := filepath.Join(t.TempDir(), "cfg")
	logs := filepath.Join(t.TempDir(), "var-log")
	web := filepath.Join(t.TempDir(), "static")
	t.Setenv(HomeEnvVar, home)
	t.Setenv(ConfigPathEnvVar, config)
	t.Setenv(LogPathEnvVar, logs)
	t.Setenv(WebPathEnvVar, web)

	if got := ConfigDir(); got != config {
		t.Fatalf("ConfigDir() = %q, want %q", got, config)
	}
	if got := LogDir(); got != logs {
		t.Fatalf("LogDir() = %q, want %q", got, logs)
	}
	if got := WebDir(); got != web {
		t.Fatalf("WebDir() = %q, want %q", got, web)
	}
}

// A process manager such as BaoTa starts the server from <home>/bin. With the
// release home set, Load must read <home>/config and must not depend on cwd.
func TestLoadReadsConfigFromReleaseHomeRegardlessOfWorkingDirectory(t *testing.T) {
	home := t.TempDir()
	writeValidConfig(t, filepath.Join(home, "config"))
	mkdirForTest(t, filepath.Join(home, "bin"))
	t.Setenv(HomeEnvVar, home)
	t.Chdir(filepath.Join(home, "bin"))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got, want := cfg.App.Name, "wt-media-cloud"; got != want {
		t.Fatalf("App.Name = %q, want %q", got, want)
	}
}

func TestLoadAnchorsRelativeLoggerPathsToLogDir(t *testing.T) {
	home := t.TempDir()
	logs := filepath.Join(t.TempDir(), "logs-out")
	writeValidConfig(t, filepath.Join(home, "config"))
	mkdirForTest(t, filepath.Join(home, "bin"))
	t.Setenv(HomeEnvVar, home)
	t.Setenv(LogPathEnvVar, logs)
	t.Chdir(filepath.Join(home, "bin"))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	loggers := map[string]string{
		"app":      cfg.Loggers.App.Path,
		"access":   cfg.Loggers.Access.Path,
		"job":      cfg.Loggers.Job.Path,
		"external": cfg.Loggers.External.Path,
		"audit":    cfg.Loggers.Audit.Path,
		"panic":    cfg.Loggers.Panic.Path,
	}
	for name, got := range loggers {
		want := filepath.Join(logs, name+".log")
		if got != want {
			t.Errorf("logger %s path = %q, want %q", name, got, want)
		}
	}
}

func TestLoadKeepsAbsoluteLoggerPaths(t *testing.T) {
	home := t.TempDir()
	writeValidConfig(t, filepath.Join(home, "config"))
	absolute := filepath.Join(t.TempDir(), "app.log")
	writeConfigFile(t, filepath.Join(home, "config"), "logger/app.toml",
		"path = "+strconv.Quote(absolute)+"\nlevel = \"info\"\nformat = \"json\"\n\n[rotation]\nmax_size = 500\nmax_age = 30\nmax_backups = 10\ncompress = true\nlocal_time = true\n")
	t.Setenv(HomeEnvVar, home)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := cfg.Loggers.App.Path; got != absolute {
		t.Fatalf("absolute logger path = %q, want %q", got, absolute)
	}
}

// Without an override and without an executable-relative config tree, the
// working directory becomes the release root so local development is unchanged,
// but every derived path is absolute.
func TestHomeFallsBackToWorkingDirectory(t *testing.T) {
	home := t.TempDir()
	writeValidConfig(t, filepath.Join(home, "config"))
	t.Chdir(home)
	t.Setenv(HomeEnvVar, "")

	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if got, want := Home(), workingDirectory; got != want {
		t.Fatalf("Home() = %q, want %q", got, want)
	}
	if got, want := ConfigDir(), filepath.Join(workingDirectory, "config"); got != want {
		t.Fatalf("ConfigDir() = %q, want %q", got, want)
	}
}
