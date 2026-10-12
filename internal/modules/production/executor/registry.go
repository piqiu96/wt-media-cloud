// Package executor binds template versions to the code that executes them.
//
// ADR-0021 makes a template version a technical asset: schema and executor are
// released together, and a database record may only claim a capability that
// shipped code provides. This registry is that code-side half. It is built only
// from definitions this process carries — nothing a caller, a request, or a
// database row can do adds an entry, which is what makes an unknown version a
// hard failure rather than a lookup that might succeed later.
package executor

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ErrUnknownTemplateVersion reports a lookup for a (code, version) pair no
// shipped executor provides. Callers must treat it as a business failure: no
// retry, no fallback executor, no success.
var ErrUnknownTemplateVersion = errors.New("unknown template version")

// A Plan is the validated, executable form of one run's parameters. It exists
// as an interface because validation belongs to the executor that will run the
// command: each template version decides what its parameters mean, and the
// pipeline above never needs to know their shape.
type Plan interface {
	// FFmpegArguments is the whole command for the run: a fixed construction,
	// not a starting point the caller may extend.
	FFmpegArguments(inputPath, outputPath string) []string
}

// An Executor is the code released with one template version.
type Executor interface {
	// ValidateParams parses raw configuration against the version's schema and
	// returns the plan to run. Parameters the schema does not declare are an
	// error here, not a value the run ignores.
	ValidateParams(raw json.RawMessage) (Plan, error)
}

// A Definition is one template version: the identity, the schema published
// with it, and the code that executes it.
type Definition struct {
	Code     string
	Version  uint32
	Schema   json.RawMessage
	Executor Executor
}

// A Registry answers which executor code a template version maps to.
type Registry struct {
	definitions map[string]Definition
}

// NewRegistry builds a registry from the definitions the process shipped with.
// An empty registration or two definitions claiming one (code, version) are
// both programming errors and fail here rather than at lookup time.
func NewRegistry(definitions ...Definition) (*Registry, error) {
	if len(definitions) == 0 {
		return nil, errors.New("an executor registry needs at least one template version")
	}
	registry := &Registry{definitions: make(map[string]Definition, len(definitions))}
	for _, definition := range definitions {
		key := fmt.Sprintf("%s/%d", definition.Code, definition.Version)
		if _, exists := registry.definitions[key]; exists {
			return nil, fmt.Errorf("duplicate template version %s", key)
		}
		registry.definitions[key] = definition
	}
	return registry, nil
}

// Lookup resolves a template version to its definition. The error wraps
// ErrUnknownTemplateVersion and names what was asked for.
func (r *Registry) Lookup(code string, version uint32) (Definition, error) {
	key := fmt.Sprintf("%s/%d", code, version)
	definition, exists := r.definitions[key]
	if !exists {
		return Definition{}, fmt.Errorf("%w: %s", ErrUnknownTemplateVersion, key)
	}
	return definition, nil
}
