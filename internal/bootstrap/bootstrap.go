// Package bootstrap owns Cloud process initialization and lifecycle order.
package bootstrap

import (
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/wt-media/wt-media-cloud/internal/config"
)

// InitializeServer initializes HTTP-only resources and registers routes.
func InitializeServer() (*server.Hertz, func() error, error) {
	closer, err := initializeResources(serverResourcePlan())
	if err != nil {
		return nil, nil, err
	}

	engine := server.Default(server.WithHostPorts(config.Get().App.Server.HTTPAddr))
	if err := registerRoutes(engine); err != nil {
		_ = closer()
		return nil, nil, err
	}
	return engine, closer, nil
}

// InitializeScheduler initializes only resources required by the scheduler process
// and returns its blocking run function.
func InitializeScheduler() (func() error, error) {
	closer, err := initializeResources(schedulerResourcePlan())
	if err != nil {
		return nil, err
	}
	return func() error {
		defer closer()
		return runSchedulerProcess()
	}, nil
}

// InitializeWorker initializes only resources required by the worker process and
// returns its blocking run function.
func InitializeWorker() (func() error, error) {
	closer, err := initializeResources(workerResourcePlan())
	if err != nil {
		return nil, err
	}
	return func() error {
		defer closer()
		return runWorkerProcess()
	}, nil
}

// InitializeMigration initializes resources required by the migration command.
func InitializeMigration() (func() error, error) {
	return initializeResources(migrationResourcePlan())
}

func initializeServer(
	steps []resourceStep,
	addr string,
	newEngine func(addr string) *server.Hertz,
	register func(*server.Hertz) error,
) (*server.Hertz, func() error, error) {
	closer, err := initializeResources(steps)
	if err != nil {
		return nil, nil, err
	}

	engine := newEngine(addr)
	if err := register(engine); err != nil {
		_ = closer()
		return nil, nil, err
	}
	return engine, closer, nil
}
