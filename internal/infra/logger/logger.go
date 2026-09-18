// Package logger owns Cloud's six independent structured log resources.
package logger

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wt-media/wt-media-cloud/internal/config"
)

type resourceSet struct {
	appLogger      *slog.Logger
	accessLogger   *slog.Logger
	jobLogger      *slog.Logger
	externalLogger *slog.Logger
	auditLogger    *slog.Logger
	panicLogger    *slog.Logger

	app      *os.File
	access   *os.File
	job      *os.File
	external *os.File
	audit    *os.File
	panic    *os.File
}

type resourceState struct {
	mu  sync.RWMutex
	set *resourceSet
}

var resources resourceState

// Initialize opens all six configured log files and atomically publishes their loggers.
func Initialize(cfg config.LoggerConfigs) error {
	resources.mu.Lock()
	defer resources.mu.Unlock()
	if resources.set != nil {
		return errors.New("logger: resources already initialized")
	}

	definitions := []struct {
		name   string
		config config.LoggerConfig
	}{
		{name: "app", config: cfg.App},
		{name: "access", config: cfg.Access},
		{name: "job", config: cfg.Job},
		{name: "external", config: cfg.External},
		{name: "audit", config: cfg.Audit},
		{name: "panic", config: cfg.Panic},
	}

	next := &resourceSet{}
	opened := make([]*os.File, 0, len(definitions))
	loggers := make([]*slog.Logger, len(definitions))
	seenPaths := make(map[string]struct{}, len(definitions))

	for index, definition := range definitions {
		if strings.TrimSpace(definition.config.Path) == "" {
			closeFiles(opened)
			return fmt.Errorf("logger.%s.path is required", definition.name)
		}
		if _, duplicate := seenPaths[definition.config.Path]; duplicate {
			closeFiles(opened)
			return fmt.Errorf("logger.%s.path duplicates another logger path", definition.name)
		}
		seenPaths[definition.config.Path] = struct{}{}

		level, err := parseLevel(definition.config.Level)
		if err != nil {
			closeFiles(opened)
			return fmt.Errorf("logger.%s.level is invalid: %w", definition.name, err)
		}
		if !strings.EqualFold(definition.config.Format, "json") && !strings.EqualFold(definition.config.Format, "text") {
			closeFiles(opened)
			return fmt.Errorf("logger.%s.format must be json or text", definition.name)
		}

		file, err := openLogFile(definition.config.Path)
		if err != nil {
			closeFiles(opened)
			return fmt.Errorf("logger.%s.path cannot be opened: %w", definition.name, err)
		}
		opened = append(opened, file)
		loggers[index] = newLogger(file, level, definition.config.Format)
	}

	next.appLogger = loggers[0]
	next.accessLogger = loggers[1]
	next.jobLogger = loggers[2]
	next.externalLogger = loggers[3]
	next.auditLogger = loggers[4]
	next.panicLogger = loggers[5]
	next.app = opened[0]
	next.access = opened[1]
	next.job = opened[2]
	next.external = opened[3]
	next.audit = opened[4]
	next.panic = opened[5]
	resources.set = next
	return nil
}

// App returns the application logger.
func App() *slog.Logger {
	set := currentSet()
	if set == nil || set.appLogger == nil {
		panic("logger: App called before Initialize")
	}
	return set.appLogger
}

// Access returns the HTTP access logger.
func Access() *slog.Logger {
	set := currentSet()
	if set == nil || set.accessLogger == nil {
		panic("logger: Access called before Initialize")
	}
	return set.accessLogger
}

// Job returns the background job logger.
func Job() *slog.Logger {
	set := currentSet()
	if set == nil || set.jobLogger == nil {
		panic("logger: Job called before Initialize")
	}
	return set.jobLogger
}

// External returns the external API logger.
func External() *slog.Logger {
	set := currentSet()
	if set == nil || set.externalLogger == nil {
		panic("logger: External called before Initialize")
	}
	return set.externalLogger
}

// Audit returns the audit logger.
func Audit() *slog.Logger {
	set := currentSet()
	if set == nil || set.auditLogger == nil {
		panic("logger: Audit called before Initialize")
	}
	return set.auditLogger
}

// Panic returns the panic logger.
func Panic() *slog.Logger {
	set := currentSet()
	if set == nil || set.panicLogger == nil {
		panic("logger: Panic called before Initialize")
	}
	return set.panicLogger
}

// Close syncs and closes every logger file. It is safe to call repeatedly.
func Close() error {
	resources.mu.Lock()
	set := resources.set
	resources.set = nil
	resources.mu.Unlock()
	if set == nil {
		return nil
	}

	var closeErrors []error
	for _, file := range []*os.File{
		set.app,
		set.access,
		set.job,
		set.external,
		set.audit,
		set.panic,
	} {
		if file == nil {
			continue
		}
		if err := file.Sync(); err != nil {
			closeErrors = append(closeErrors, err)
		}
		if err := file.Close(); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	return errors.Join(closeErrors...)
}

func currentSet() *resourceSet {
	resources.mu.RLock()
	defer resources.mu.RUnlock()
	return resources.set
}

func openLogFile(path string) (*os.File, error) {
	directory := filepath.Dir(path)
	if directory != "." {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return nil, err
		}
	}
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
}

func newLogger(writer io.Writer, level slog.Level, format string) *slog.Logger {
	options := &slog.HandlerOptions{Level: level}
	if strings.EqualFold(format, "text") {
		return slog.New(slog.NewTextHandler(writer, options))
	}
	return slog.New(slog.NewJSONHandler(writer, options))
}

func parseLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid level %q", value)
	}
}

func closeFiles(files []*os.File) {
	for _, file := range files {
		_ = file.Close()
	}
}
