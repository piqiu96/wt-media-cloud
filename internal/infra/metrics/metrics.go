// Package metrics owns Cloud's private metric recorder resource.
package metrics

import (
	"errors"
	"sync"
)

type Recorder interface {
	Count(name string, value int64, labels map[string]string)
}

// Noop records metrics without a backend.
type Noop struct{}

func (Noop) Count(string, int64, map[string]string) {}

var state struct {
	sync.RWMutex
	initialized bool
	recorder    Recorder
}

// Initialize publishes the current no-op recorder for process bootstrap.
func Initialize() error {
	state.Lock()
	defer state.Unlock()
	if state.initialized {
		return errors.New("metrics: resources already initialized")
	}
	state.recorder = Noop{}
	state.initialized = true
	return nil
}

// Get returns a non-nil recorder, defaulting to Noop before initialization.
func Get() Recorder {
	state.RLock()
	defer state.RUnlock()
	if !state.initialized || state.recorder == nil {
		return Noop{}
	}
	return state.recorder
}

// Close restores the no-op default and is safe to call repeatedly.
func Close() error {
	state.Lock()
	defer state.Unlock()
	state.recorder = nil
	state.initialized = false
	return nil
}
