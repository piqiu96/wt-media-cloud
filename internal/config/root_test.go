package config

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

func resetRuntimePathsForTest(t *testing.T) {
	t.Helper()
	runtimePaths.Lock()
	previousValue, previousInitialized := runtimePaths.value, runtimePaths.initialized
	runtimePaths.value, runtimePaths.initialized = RuntimePaths{}, false
	runtimePaths.Unlock()
	t.Cleanup(func() {
		runtimePaths.Lock()
		runtimePaths.value, runtimePaths.initialized = previousValue, previousInitialized
		runtimePaths.Unlock()
	})
}

func initializeRuntimePathsForTest(t *testing.T) {
	t.Helper()
	resetRuntimePathsForTest(t)
	MustInitializeRuntimePaths()
}

func mkdirForTest(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create directory %s: %v", dir, err)
	}
}

func TestHomeHonoursReleaseHomeOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv(HomeEnvVar, home)
	paths, err := resolveRuntimePaths()
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct{ name, got, want string }{
		{"home", paths.Home, home},
		{"config", paths.Config, filepath.Join(home, "config")},
		{"logs", paths.Logs, filepath.Join(home, "logs")},
		{"web", paths.Web, filepath.Join(home, "web")},
		{"migrations", paths.Migrations, filepath.Join(home, "migrations")},
	} {
		if check.got != check.want {
			t.Errorf("%s path = %q, want %q", check.name, check.got, check.want)
		}
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

	paths, err := resolveRuntimePaths()
	if err != nil {
		t.Fatal(err)
	}
	if paths.Config != config || paths.Logs != logs || paths.Web != web {
		t.Fatalf("runtime paths = %+v, want config=%q logs=%q web=%q", paths, config, logs, web)
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
	initializeRuntimePathsForTest(t)

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
	initializeRuntimePathsForTest(t)

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
	initializeRuntimePathsForTest(t)

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
	paths, err := resolveRuntimePaths()
	if err != nil {
		t.Fatal(err)
	}
	if paths.Home != workingDirectory || paths.Config != filepath.Join(workingDirectory, "config") {
		t.Fatalf("runtime paths = %+v, want cwd home and config", paths)
	}
}

// A released process must keep its root when config is absent, so the missing
// file error identifies the release instead of an arbitrary process-manager cwd.
func TestHomeUsesReleasedBinaryRootEvenWhenConfigIsMissing(t *testing.T) {
	if os.Getenv("WT_MEDIA_TEST_RELEASE_BINARY_CHILD") == "1" {
		paths, err := resolveRuntimePaths()
		if err != nil {
			t.Fatal(err)
		}
		if got, want := paths.Home, os.Getenv("WT_MEDIA_TEST_RELEASE_HOME"); got != want {
			t.Fatalf("Home() = %q, want released binary root %q", got, want)
		}
		return
	}

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	mkdirForTest(t, bin)
	source, err := os.Open(executable)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	released := filepath.Join(bin, "cloud-test")
	target, err := os.OpenFile(released, os.O_CREATE|os.O_WRONLY, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(target, source); err != nil {
		t.Fatal(err)
	}
	if err := target.Close(); err != nil {
		t.Fatal(err)
	}
	resolvedHome, err := filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(released, "-test.run=^TestHomeUsesReleasedBinaryRootEvenWhenConfigIsMissing$")
	command.Dir = t.TempDir()
	command.Env = append(os.Environ(), HomeEnvVar+"=", "WT_MEDIA_TEST_RELEASE_BINARY_CHILD=1", "WT_MEDIA_TEST_RELEASE_HOME="+resolvedHome)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("released binary from unrelated cwd: %v\n%s", err, output)
	}
}

func TestRelativeChildPathOverridesFollowHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv(HomeEnvVar, home)
	t.Setenv(ConfigPathEnvVar, "private/config")
	t.Setenv(LogPathEnvVar, "private/logs")
	t.Setenv(WebPathEnvVar, "assets/web")
	t.Chdir(t.TempDir())
	paths, err := resolveRuntimePaths()
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct{ name, got, want string }{
		{"config", paths.Config, filepath.Join(home, "private/config")},
		{"logs", paths.Logs, filepath.Join(home, "private/logs")},
		{"web", paths.Web, filepath.Join(home, "assets/web")},
	} {
		if check.got != check.want {
			t.Errorf("%s path = %q, want %q", check.name, check.got, check.want)
		}
	}
}

func TestPathInitializationRejectsRelativeHomeOverride(t *testing.T) {
	t.Setenv(HomeEnvVar, "relative-release")
	resetRuntimePathsForTest(t)
	assertPathPanic(t, HomeEnvVar, MustInitializeRuntimePaths)
}
