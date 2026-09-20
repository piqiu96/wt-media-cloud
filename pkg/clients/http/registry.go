package httpclient

import (
	"fmt"
	"sync"

	hertzclient "github.com/cloudwego/hertz/pkg/app/client"
	pkgconfig "github.com/wt-media/wt-media-cloud/pkg/config"
)

type resourceState struct {
	mu      sync.RWMutex
	clients map[string]*Client
}

var resources resourceState

// Initialize publishes strongly typed Hertz clients derived from document filenames.
func Initialize(documents []pkgconfig.Document, middlewares map[string][]hertzclient.Middleware) (func() error, error) {
	resources.mu.Lock()
	defer resources.mu.Unlock()
	if resources.clients != nil {
		return nil, fmt.Errorf("http clients already initialized")
	}

	clients := make(map[string]*Client, len(documents))
	closers := make([]func() error, 0, len(documents))
	seen := make(map[string]struct{}, len(documents))
	closeAll := func() {
		for index := len(closers) - 1; index >= 0; index-- {
			_ = closers[index]()
		}
	}

	for _, document := range documents {
		if _, duplicate := seen[document.Name]; duplicate {
			closeAll()
			return nil, fmt.Errorf("duplicate http client name %q", document.Name)
		}
		if document.Format != pkgconfig.FormatTOML {
			closeAll()
			return nil, fmt.Errorf("http client %s must use TOML", document.Name)
		}

		var cfg Config
		if err := document.Decode(&cfg); err != nil {
			closeAll()
			return nil, fmt.Errorf("decode http client %s: %w", document.Name, err)
		}
		instance, closer, err := New(cfg, middlewares[document.Name]...)
		if err != nil {
			closeAll()
			return nil, fmt.Errorf("http client %s: %w", document.Name, err)
		}

		seen[document.Name] = struct{}{}
		clients[document.Name] = instance
		closers = append(closers, closer)
	}

	resources.clients = clients
	return func() error {
		resources.mu.Lock()
		current := resources.clients
		resources.clients = nil
		resources.mu.Unlock()

		var closeErrors []error
		for _, instance := range current {
			instance.CloseIdleConnections()
		}
		for _, closer := range closers {
			if err := closer(); err != nil {
				closeErrors = append(closeErrors, err)
			}
		}
		if len(closeErrors) == 0 {
			return nil
		}
		return fmt.Errorf("close http clients: %v", closeErrors)
	}, nil
}

// Get returns one initialized Hertz client and fails fast before Bootstrap.
func Get(name string) *Client {
	resources.mu.RLock()
	defer resources.mu.RUnlock()
	if resources.clients == nil {
		panic("http client resources called before Initialize")
	}
	instance, ok := resources.clients[name]
	if !ok {
		panic("http client not configured: " + name)
	}
	return instance
}

// Close releases all initialized clients and is safe to call repeatedly.
func Close() error {
	resources.mu.Lock()
	current := resources.clients
	resources.clients = nil
	resources.mu.Unlock()
	if current == nil {
		return nil
	}
	for _, instance := range current {
		instance.CloseIdleConnections()
	}
	return nil
}
