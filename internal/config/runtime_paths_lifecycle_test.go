package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimePathsProcessLifecycle(t *testing.T) {
	switch os.Getenv("WT_MEDIA_TEST_PATH_LIFECYCLE") {
	case "before-init":
		assertPathPanic(t, "before initialization", func() { GetRuntimePaths() })
		return
	case "load-before-init":
		assertPathPanic(t, "before initialization", func() { _, _ = Load() })
		return
	case "invalid-home":
		assertPathPanic(t, HomeEnvVar, MustInitializeRuntimePaths)
		assertPathPanic(t, "before initialization", func() { GetRuntimePaths() })
		return
	case "cached":
		first := os.Getenv("WT_MEDIA_TEST_FIRST_HOME")
		second := os.Getenv("WT_MEDIA_TEST_SECOND_HOME")
		MustInitializeRuntimePaths()
		if got := GetRuntimePaths().Home; got != first {
			t.Fatalf("initial home = %q, want %q", got, first)
		}
		if err := os.Setenv(HomeEnvVar, second); err != nil {
			t.Fatal(err)
		}
		if err := os.Setenv(ConfigPathEnvVar, "other-config"); err != nil {
			t.Fatal(err)
		}
		MustInitializeRuntimePaths()
		paths := GetRuntimePaths()
		if paths.Home != first || paths.Config != filepath.Join(first, "config") {
			t.Fatalf("cached paths = %+v, want original home and config after env change", paths)
		}
		return
	}

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(t.TempDir(), "first")
	second := filepath.Join(t.TempDir(), "second")
	for _, test := range []struct {
		name, home string
	}{
		{"before-init", first},
		{"load-before-init", first},
		{"invalid-home", "relative-release"},
		{"cached", first},
	} {
		t.Run(test.name, func(t *testing.T) {
			command := exec.Command(executable, "-test.run=^TestRuntimePathsProcessLifecycle$")
			command.Env = append(os.Environ(),
				"WT_MEDIA_TEST_PATH_LIFECYCLE="+test.name,
				HomeEnvVar+"="+test.home,
				"WT_MEDIA_TEST_FIRST_HOME="+first,
				"WT_MEDIA_TEST_SECOND_HOME="+second,
			)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("child process: %v\n%s", err, output)
			}
		})
	}
}

func assertPathPanic(t *testing.T, want string, run func()) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered == nil || !strings.Contains(panicMessage(recovered), want) {
			t.Errorf("panic = %v, want text %q", recovered, want)
		}
	}()
	run()
}

func panicMessage(value any) string {
	if err, ok := value.(error); ok {
		return err.Error()
	}
	if message, ok := value.(string); ok {
		return message
	}
	return ""
}
