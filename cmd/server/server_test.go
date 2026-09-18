package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestServerRunClosesEngineBeforeBootstrapResources(t *testing.T) {
	var order []string
	ctx, cancel := context.WithCancel(context.Background())
	lifecycle := serverLifecycle{
		runEngine: func() error {
			<-ctx.Done()
			return nil
		},
		shutdownEngine: func(context.Context) error {
			order = append(order, "engine")
			return nil
		},
		closeResources: func() error {
			order = append(order, "resources")
			return nil
		},
	}

	done := make(chan error, 1)
	go func() { done <- runServerLifecycle(ctx, lifecycle) }()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runServerLifecycle() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("runServerLifecycle() did not return")
	}
	if len(order) != 2 || order[0] != "engine" || order[1] != "resources" {
		t.Fatalf("close order = %v, want [engine resources]", order)
	}
}

func TestServerRunReturnsEngineAndResourceErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	lifecycle := serverLifecycle{
		runEngine:      func() error { <-ctx.Done(); return nil },
		shutdownEngine: func(context.Context) error { return errors.New("engine close failed") },
		closeResources: func() error { return errors.New("resource close failed") },
	}
	cancel()
	err := runServerLifecycle(ctx, lifecycle)
	if err == nil {
		t.Fatal("runServerLifecycle() error = nil, want joined errors")
	}
}
