// Package logger owns Cloud's six independent Hertz-compatible log resources.
package logger

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/wt-media/wt-media-cloud/internal/config"
	pkglogger "github.com/wt-media/wt-media-cloud/pkg/logger"
)

type resourceSet struct {
	app      *pkglogger.Logger
	access   *pkglogger.Logger
	job      *pkglogger.Logger
	external *pkglogger.Logger
	audit    *pkglogger.Logger
	panic    *pkglogger.Logger
}

type resourceState struct {
	mu  sync.RWMutex
	set *resourceSet
}

var resources resourceState

type loggerDefinition struct {
	name   string
	config config.LoggerConfig
}

// Initialize opens all six configured loggers and atomically publishes them.
func Initialize(cfg config.LoggerConfigs) error {
	resources.mu.Lock()
	defer resources.mu.Unlock()
	if resources.set != nil {
		return errors.New("logger: resources already initialized")
	}

	definitions := []loggerDefinition{
		{name: "app", config: cfg.App},
		{name: "access", config: cfg.Access},
		{name: "job", config: cfg.Job},
		{name: "external", config: cfg.External},
		{name: "audit", config: cfg.Audit},
		{name: "panic", config: cfg.Panic},
	}

	next := &resourceSet{}
	opened := make([]*pkglogger.Logger, 0, len(definitions))
	seenPaths := make(map[string]struct{}, len(definitions)*2)
	claimPath := func(name, path string) error {
		if _, duplicate := seenPaths[path]; duplicate {
			return fmt.Errorf("logger.%s.path %q duplicates another logger path", name, path)
		}
		seenPaths[path] = struct{}{}
		return nil
	}

	for index, definition := range definitions {
		if strings.TrimSpace(definition.config.Path) == "" {
			closeLoggers(opened)
			return fmt.Errorf("logger.%s.path is required", definition.name)
		}
		if err := claimPath(definition.name, definition.config.Path); err != nil {
			closeLoggers(opened)
			return err
		}
		if err := claimPath(definition.name, definition.config.Path+".wf"); err != nil {
			closeLoggers(opened)
			return err
		}

		instance, err := pkglogger.New(pkglogger.Config{
			Path:     definition.config.Path,
			Level:    definition.config.Level,
			Format:   definition.config.Format,
			Rotation: pkglogger.RotationConfig(definition.config.Rotation),
		})
		if err != nil {
			closeLoggers(opened)
			return fmt.Errorf("logger.%s: %w", definition.name, err)
		}
		opened = append(opened, instance)

		switch index {
		case 0:
			next.app = instance
		case 1:
			next.access = instance
		case 2:
			next.job = instance
		case 3:
			next.external = instance
		case 4:
			next.audit = instance
		case 5:
			next.panic = instance
		}
	}

	resources.set = next
	return nil
}

// App returns the application logger.
func App() hlog.FullLogger { return currentSet().app }

// Access returns the HTTP access logger.
func Access() hlog.FullLogger { return currentSet().access }

// Job returns the background job logger.
func Job() hlog.FullLogger { return currentSet().job }

// External returns the external API logger.
func External() hlog.FullLogger { return currentSet().external }

// Audit returns the audit logger.
func Audit() hlog.FullLogger { return currentSet().audit }

// Panic returns the panic logger.
func Panic() hlog.FullLogger { return currentSet().panic }

// Close syncs and closes every logger. It is safe to call repeatedly.
func Close() error {
	resources.mu.Lock()
	set := resources.set
	resources.set = nil
	resources.mu.Unlock()
	if set == nil {
		return nil
	}
	return closeLoggers([]*pkglogger.Logger{
		set.app, set.access, set.job, set.external, set.audit, set.panic,
	})
}

func currentSet() *resourceSet {
	resources.mu.RLock()
	defer resources.mu.RUnlock()
	if resources.set == nil {
		panic("logger: resources called before Initialize")
	}
	return resources.set
}

func closeLoggers(loggers []*pkglogger.Logger) error {
	var closeErrors []error
	for _, instance := range loggers {
		if instance == nil {
			continue
		}
		if err := instance.Close(); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	return errors.Join(closeErrors...)
}
