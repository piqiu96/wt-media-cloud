// Package tracing owns Cloud's private tracer resource.
package tracing

import (
	"context"
	"errors"
	"sync"
)

type Tracer interface {
	Start(context.Context, string) (context.Context, func(error))
}

// Noop traces operations without a backend.
type Noop struct{}

func (Noop) Start(ctx context.Context, _ string) (context.Context, func(error)) {
	return ctx, func(error) {}
}

var state struct {
	sync.RWMutex
	initialized bool
	tracer      Tracer
}

// Initialize publishes the current no-op tracer for process bootstrap.
func Initialize() error {
	state.Lock()
	defer state.Unlock()
	if state.initialized {
		return errors.New("tracing: resources already initialized")
	}
	state.tracer = Noop{}
	state.initialized = true
	return nil
}

// Get returns a non-nil tracer, defaulting to Noop before initialization.
func Get() Tracer {
	state.RLock()
	defer state.RUnlock()
	if !state.initialized || state.tracer == nil {
		return Noop{}
	}
	return state.tracer
}

// Close restores the no-op default and is safe to call repeatedly.
func Close() error {
	state.Lock()
	defer state.Unlock()
	state.tracer = nil
	state.initialized = false
	return nil
}
