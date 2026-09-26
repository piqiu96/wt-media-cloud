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
	"github.com/wt-media/wt-media-cloud/internal/infra/storage"
	"github.com/wt-media/wt-media-cloud/internal/infra/tracing"
	httpclient "github.com/wt-media/wt-media-cloud/pkg/clients/http"
)

type clientInitializer func(name string, credentials config.CredentialsConfig) (func() error, error)

var clientInitializers = map[string]clientInitializer{
	"agent":  agentclient.Initialize,
	"douyin": douyinclient.Initialize,
}

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
		storageResource(),
		clientsResource(),
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
		storageResource(),
		clientsResource("douyin"),
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

// storageResource publishes the object-storage store. It is in the server plan
// because a claim mints the lease's download grant, and in the worker plan
// because the preparation worker writes the object that grant points at; the
// scheduler and the migration runner do neither.
//
// It does not fail when no credential is configured — that is the state every
// tree in this repository is committed in — so this step is one that always
// succeeds and either publishes a signing store or one that refuses.
func storageResource() resourceStep {
	return resourceStep{
		name: "storage",
		open: func() (func() error, error) {
			cfg := config.Get()
			if err := storage.Initialize(cfg.Storage.ObjectStorage, cfg.Credentials.ObjectStorage); err != nil {
				return nil, err
			}
			return storage.Close, nil
		},
	}
}

func clientsResource(names ...string) resourceStep {
	return resourceStep{
		name: "clients",
		open: func() (func() error, error) {
			cfg := config.Get()
			selected, err := selectHTTPClientConfigs(cfg.Clients.HTTP, names...)
			if err != nil {
				return nil, err
			}

			middlewares := make(map[string][]client.Middleware, len(selected))
			for _, clientConfig := range selected {
				middlewares[clientConfig.Name] = []client.Middleware{observe.External(clientConfig.Name)}
			}
			transportCloser, err := httpclient.Initialize(selected, middlewares)
			if err != nil {
				return nil, err
			}

			semanticClosers := make([]func() error, 0, len(selected))
			closeSemanticClients := func() {
				for index := len(semanticClosers) - 1; index >= 0; index-- {
					_ = semanticClosers[index]()
				}
			}
			for _, clientConfig := range selected {
				initializer, exists := clientInitializers[clientConfig.Name]
				if !exists {
					closeSemanticClients()
					_ = transportCloser()
					return nil, fmt.Errorf("HTTP client %q has no semantic initializer", clientConfig.Name)
				}
				closer, err := initializer(clientConfig.Name, cfg.Credentials)
				if err != nil {
					closeSemanticClients()
					_ = transportCloser()
					return nil, err
				}
				semanticClosers = append(semanticClosers, closer)
			}

			return func() error {
				closeSemanticClients()
				return transportCloser()
			}, nil
		},
	}
}

func selectHTTPClientConfigs(configs []httpclient.Config, names ...string) ([]httpclient.Config, error) {
	selected := append([]httpclient.Config(nil), configs...)
	sort.Slice(selected, func(left, right int) bool {
		return selected[left].Name < selected[right].Name
	})
	if len(names) == 0 {
		return selected, nil
	}

	wanted := make(map[string]struct{}, len(names))
	for _, name := range names {
		wanted[name] = struct{}{}
	}
	filtered := make([]httpclient.Config, 0, len(names))
	for _, clientConfig := range selected {
		if _, exists := wanted[clientConfig.Name]; exists {
			filtered = append(filtered, clientConfig)
			delete(wanted, clientConfig.Name)
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
	return filtered, nil
}
