package main

import (
	"context"
	"errors"
	"os/signal"
	"syscall"

	"github.com/wt-media/wt-media-cloud/internal/bootstrap"
)

type serverLifecycle struct {
	runEngine      func() error
	shutdownEngine func(context.Context) error
	closeResources func() error
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	engine, closeResources, err := bootstrap.InitializeServer()
	if err != nil {
		return err
	}
	return runServerLifecycle(ctx, serverLifecycle{
		runEngine:      engine.Run,
		shutdownEngine: engine.Shutdown,
		closeResources: closeResources,
	})
}

func runServerLifecycle(ctx context.Context, lifecycle serverLifecycle) error {
	engineDone := make(chan error, 1)
	go func() { engineDone <- lifecycle.runEngine() }()

	var engineError error
	select {
	case <-ctx.Done():
	case engineError = <-engineDone:
	}

	shutdownError := lifecycle.shutdownEngine(context.Background())
	resourceError := lifecycle.closeResources()
	return errors.Join(engineError, shutdownError, resourceError)
}
