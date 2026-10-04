package config

import (
	"os"
	"path/filepath"
	"strings"
)

// Path environment variables. Every value is optional; each one falls back to a
// path derived from the release root, so moving the install directory only
// requires changing WT_MEDIA_CLOUD_HOME (or nothing at all when the binary runs
// from <root>/bin).
const (
	// HomeEnvVar sets the Cloud release root (ENV_PATH).
	HomeEnvVar = "WT_MEDIA_CLOUD_HOME"
	// ConfigPathEnvVar sets the runtime configuration directory, default <home>/config.
	ConfigPathEnvVar = "WT_MEDIA_CLOUD_CONFIG_PATH"
	// LogPathEnvVar sets the runtime log directory, default <home>/logs.
	LogPathEnvVar = "WT_MEDIA_CLOUD_LOG_PATH"
	// WebPathEnvVar sets the Cloud Web asset directory, default <home>/web.
	WebPathEnvVar = "WT_MEDIA_CLOUD_WEB_PATH"
)

// Home returns the Cloud release root.
//
// Precedence:
//  1. $WT_MEDIA_CLOUD_HOME when set;
//  2. the parent of the directory holding the running binary when it contains
//     config/app.toml (a released <home>/bin/<binary> layout);
//  3. the current working directory (local development).
//
// Resolving from the binary lets a process manager (for example BaoTa, which
// starts the server from <home>/bin) launch the binary without forcing a
// working directory.
func Home() string {
	if override := pathEnv(HomeEnvVar); override != "" {
		return override
	}
	if executable, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(executable); err == nil {
			executable = resolved
		}
		candidate := filepath.Dir(filepath.Dir(executable))
		if _, err := os.Stat(filepath.Join(candidate, configDirectory, "app.toml")); err == nil {
			return candidate
		}
	}
	return "."
}

// ConfigDir returns the runtime configuration directory.
func ConfigDir() string {
	return pathEnv(ConfigPathEnvVar, filepath.Join(Home(), configDirectory))
}

// LogDir returns the directory that relative logger paths are anchored to.
func LogDir() string {
	return pathEnv(LogPathEnvVar, filepath.Join(Home(), "logs"))
}

// WebDir returns the Cloud Web asset directory.
func WebDir() string {
	return pathEnv(WebPathEnvVar, filepath.Join(Home(), "web"))
}

// pathEnv returns a cleaned environment override, or fallback when it is unset.
func pathEnv(name string, fallback ...string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return filepath.Clean(value)
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return ""
}
