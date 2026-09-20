package bootstrap

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/wt-media/wt-media-cloud/internal/config"
	agentclient "github.com/wt-media/wt-media-cloud/internal/infra/client/agent"
	"github.com/wt-media/wt-media-cloud/internal/infra/client/observe"
	douyinclient "github.com/wt-media/wt-media-cloud/internal/infra/client/platforms/douyin"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/infra/logger"
	"github.com/wt-media/wt-media-cloud/internal/infra/metrics"
	"github.com/wt-media/wt-media-cloud/internal/infra/tracing"
	httpclient "github.com/wt-media/wt-media-cloud/pkg/clients/http"
	pkgconfig "github.com/wt-media/wt-media-cloud/pkg/config"
)

const httpClientsConfigDirectory = "config/clients/http"

type resourceStep struct {
	name string
	open func() (func() error, error)
}

func serverResourcePlan() []resourceStep {
	return []resourceStep{
		configResource(),
		loggerResource(),
		metricsResource(),
		tracingResource(),
		databaseResource(),
		httpClientsResource("agent", "douyin"),
		agentClientResource(),
		douyinClientResource(),
	}
}

func schedulerResourcePlan() []resourceStep {
	return []resourceStep{
		configResource(),
		loggerResource(),
		metricsResource(),
		tracingResource(),
		databaseResource(),
	}
}

func workerResourcePlan() []resourceStep {
	return []resourceStep{
		configResource(),
		loggerResource(),
		metricsResource(),
		tracingResource(),
		databaseResource(),
		httpClientsResource("douyin"),
		douyinClientResource(),
	}
}

func migrationResourcePlan() []resourceStep {
	return []resourceStep{
		configResource(),
		loggerResource(),
		databaseResource(),
	}
}

func initializeResources(steps []resourceStep) (func() error, error) {
	closers := make([]func() error, 0, len(steps))
	for _, step := range steps {
		closer, err := step.open()
		if err != nil {
			rollbackResources(closers)
			return nil, err
		}
		if closer != nil {
			closers = append(closers, closer)
		}
	}

	stack := &closeStack{closers: closers}
	return stack.close, nil
}

type closeStack struct {
	once    sync.Once
	err     error
	closers []func() error
}

func (s *closeStack) close() error {
	s.once.Do(func() { s.err = rollbackResources(s.closers) })
	return s.err
}

func rollbackResources(closers []func() error) error {
	var closeErrors []error
	for index := len(closers) - 1; index >= 0; index-- {
		if err := closers[index](); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	return errors.Join(closeErrors...)
}

func configResource() resourceStep {
	return resourceStep{
		name: "config",
		open: func() (func() error, error) {
			if err := config.Initialize(); err != nil {
				return nil, err
			}
			return func() error { return nil }, nil
		},
	}
}

func loggerResource() resourceStep {
	return resourceStep{
		name: "logger",
		open: func() (func() error, error) {
			cfg := config.Get()
			if err := logger.Initialize(cfg.Loggers); err != nil {
				return nil, err
			}
			installHertzLoggers()
			return logger.Close, nil
		},
	}
}

func metricsResource() resourceStep {
	return resourceStep{
		name: "metrics",
		open: func() (func() error, error) {
			if err := metrics.Initialize(); err != nil {
				return nil, err
			}
			return metrics.Close, nil
		},
	}
}

func tracingResource() resourceStep {
	return resourceStep{
		name: "tracing",
		open: func() (func() error, error) {
			if err := tracing.Initialize(); err != nil {
				return nil, err
			}
			return tracing.Close, nil
		},
	}
}

func databaseResource() resourceStep {
	return resourceStep{
		name: "database",
		open: func() (func() error, error) {
			cfg := config.Get()
			if err := database.Initialize(cfg.Databases); err != nil {
				return nil, err
			}
			return database.Close, nil
		},
	}
}

func installHertzLoggers() {
	hlog.SetLogger(logger.App())
	hlog.SetSystemLogger(logger.Panic())
}

func httpClientsResource(names ...string) resourceStep {
	return resourceStep{
		name: "http-clients",
		open: func() (func() error, error) {
			documents, err := loadHTTPClientDocuments(httpClientsConfigDirectory, names...)
			if err != nil {
				return nil, err
			}
			middlewares := make(map[string][]client.Middleware, len(names))
			for _, name := range names {
				middlewares[name] = []client.Middleware{observe.External(name)}
			}
			closer, err := httpclient.Initialize(documents, middlewares)
			if err != nil {
				return nil, err
			}
			return closer, nil
		},
	}
}

func loadHTTPClientDocuments(directory string, names ...string) ([]pkgconfig.Document, error) {
	documents, err := pkgconfig.LoadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("load HTTP client config: %w", err)
	}
	wanted := make(map[string]struct{}, len(names))
	for _, name := range names {
		wanted[name] = struct{}{}
	}
	selected := make([]pkgconfig.Document, 0, len(names))
	for _, document := range documents {
		if _, exists := wanted[document.Name]; exists {
			selected = append(selected, document)
			delete(wanted, document.Name)
		}
	}
	if len(wanted) != 0 {
		missing := make([]string, 0, len(wanted))
		for name := range wanted {
			missing = append(missing, name)
		}
		sort.Strings(missing)
		return nil, fmt.Errorf("HTTP client config not found: %s", strings.Join(missing, ", "))
	}
	return selected, nil
}

func agentClientResource() resourceStep {
	return resourceStep{
		name: "agent-client",
		open: func() (func() error, error) {
			cfg := config.Get()
			connection, _ := cfg.HTTPClient("agent")
			if err := agentclient.Initialize(connection, cfg.Credentials.Agent); err != nil {
				return nil, err
			}
			return agentclient.Close, nil
		},
	}
}

func douyinClientResource() resourceStep {
	return resourceStep{
		name: "douyin-client",
		open: func() (func() error, error) {
			cfg := config.Get()
			connection, _ := cfg.HTTPClient("douyin")
			if err := douyinclient.Initialize(connection, cfg.Credentials.Douyin); err != nil {
				return nil, err
			}
			return douyinclient.Close, nil
		},
	}
}
