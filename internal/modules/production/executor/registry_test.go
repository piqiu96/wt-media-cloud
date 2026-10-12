package executor

import (
	"errors"
	"strings"
	"testing"
)

func TestRegistryLookupReturnsTheDefinitionItWasBuiltWith(t *testing.T) {
	registry, err := NewRegistry(BasicCutV1())
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	definition, err := registry.Lookup("basic_cut", 1)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if definition.Code != "basic_cut" || definition.Version != 1 {
		t.Fatalf("Lookup() = (%q, %d), want (basic_cut, 1)", definition.Code, definition.Version)
	}
	if definition.Executor == nil {
		t.Fatal("Lookup().Executor = nil, want the executor code shipped with the version")
	}
	if len(definition.Schema) == 0 {
		t.Fatal("Lookup().Schema is empty, want the schema published with the version")
	}
	if definition.ExecutorID != "basic_cut_v1" {
		t.Fatalf("Lookup().ExecutorID = %q, want basic_cut_v1", definition.ExecutorID)
	}
}

func TestRegistryRejectsDuplicateTemplateVersions(t *testing.T) {
	_, err := NewRegistry(BasicCutV1(), BasicCutV1())
	if err == nil {
		t.Fatal("NewRegistry() = nil error, want a rejection when one (code, version) is registered twice")
	}
	if !strings.Contains(err.Error(), "basic_cut") || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("NewRegistry() error = %v, want it to name the duplicated version", err)
	}
}

// An unknown version is the failure the four-layer model exists to make
// impossible to paper over: no database record can claim an executable
// capability that no shipped code provides.
func TestRegistryRejectsAnUnknownTemplateVersion(t *testing.T) {
	registry, err := NewRegistry(BasicCutV1())
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	for _, lookup := range []struct {
		code    string
		version uint32
	}{
		{"basic_cut", 2},
		{"other_cut", 1},
		{"", 0},
	} {
		_, err := registry.Lookup(lookup.code, lookup.version)
		if !errors.Is(err, ErrUnknownTemplateVersion) {
			t.Fatalf("Lookup(%q, %d) error = %v, want ErrUnknownTemplateVersion", lookup.code, lookup.version, err)
		}
		if !strings.Contains(err.Error(), lookup.code) {
			t.Fatalf("Lookup(%q, %d) error = %v, want it to name the requested version", lookup.code, lookup.version, err)
		}
	}
}

func TestRegistryRejectsEmptyRegistrations(t *testing.T) {
	if _, err := NewRegistry(); err == nil {
		t.Fatal("NewRegistry() = nil error, want a rejection for a registry with no executor at all")
	}
}
