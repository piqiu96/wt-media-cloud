package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	// HomeEnvVar sets the Cloud release root. It must be absolute.
	HomeEnvVar = "WT_MEDIA_CLOUD_HOME"
	// ConfigPathEnvVar overrides <home>/config.
	ConfigPathEnvVar = "WT_MEDIA_CLOUD_CONFIG_PATH"
	// LogPathEnvVar overrides <home>/logs.
	LogPathEnvVar = "WT_MEDIA_CLOUD_LOG_PATH"
	// WebPathEnvVar overrides <home>/web.
	WebPathEnvVar = "WT_MEDIA_CLOUD_WEB_PATH"
)

// RuntimePaths contains the absolute paths needed by all Cloud processes.
// Relative child overrides are anchored to Home, never to the process cwd.
type RuntimePaths struct {
	Home       string
	Config     string
	Logs       string
	Web        string
	Migrations string
}

var runtimePaths struct {
	sync.RWMutex
	value       RuntimePaths
	initialized bool
}

// MustInitializeRuntimePaths resolves and publishes paths once per process.
// A bad release root is a startup failure, so initialization panics.
func MustInitializeRuntimePaths() {
	runtimePaths.Lock()
	defer runtimePaths.Unlock()
	if runtimePaths.initialized {
		return
	}
	paths, err := resolveRuntimePaths()
	if err != nil {
		panic(fmt.Errorf("initialize Cloud runtime paths: %w", err))
	}
	runtimePaths.value = paths
	runtimePaths.initialized = true
}

// GetRuntimePaths returns the paths published during startup.
// Reading them before initialization is a programming error.
func GetRuntimePaths() RuntimePaths {
	runtimePaths.RLock()
	defer runtimePaths.RUnlock()
	if !runtimePaths.initialized {
		panic("config: GetRuntimePaths called before initialization")
	}
	return runtimePaths.value
}

// resolveRuntimePaths uses the explicit home, a released <home>/bin/<binary>
// layout, then cwd for local development. The released layout wins even when
// config/app.toml is missing, so errors identify the actual release directory.
func resolveRuntimePaths() (RuntimePaths, error) {
	home, err := resolveHome()
	if err != nil {
		return RuntimePaths{}, err
	}
	return RuntimePaths{
		Home:       home,
		Config:     childPath(home, ConfigPathEnvVar, "config"),
		Logs:       childPath(home, LogPathEnvVar, "logs"),
		Web:        childPath(home, WebPathEnvVar, "web"),
		Migrations: filepath.Join(home, "migrations"),
	}, nil
}

func resolveHome() (string, error) {
	if value := strings.TrimSpace(os.Getenv(HomeEnvVar)); value != "" {
		if !filepath.IsAbs(value) {
			return "", fmt.Errorf("%s must be an absolute path, got %q", HomeEnvVar, value)
		}
		return filepath.Clean(value), nil
	}
	if executable, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(executable); err == nil {
			executable = resolved
		}
		bin := filepath.Dir(executable)
		if filepath.Base(bin) == "bin" {
			return filepath.Dir(bin), nil
		}
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve Cloud root from working directory: %w", err)
	}
	return workingDirectory, nil
}

func childPath(home, variable, suffix string) string {
	value := strings.TrimSpace(os.Getenv(variable))
	if value == "" {
		value = suffix
	}
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return filepath.Join(home, value)
}
