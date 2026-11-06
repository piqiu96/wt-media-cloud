package executor

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestRegistryLookupReturnsTheDefinitionItWasBuiltWith(t *testing.T) {
	registry, err := NewRegistry(DynamicPIPEndingV1())
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	definition, err := registry.Lookup("dynamic_pip_ending", 1)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if definition.Code != "dynamic_pip_ending" || definition.Version != 1 {
		t.Fatalf("Lookup() = (%q, %d), want (basic_cut, 1)", definition.Code, definition.Version)
	}
	if definition.Executor == nil {
		t.Fatal("Lookup().Executor = nil, want the executor code shipped with the version")
	}
	if len(definition.Schema) == 0 {
		t.Fatal("Lookup().Schema is empty, want the schema published with the version")
	}
	if definition.ExecutorID != "dynamic_pip_ending_v1" {
		t.Fatalf("Lookup().ExecutorID = %q, want dynamic_pip_ending_v1", definition.ExecutorID)
	}
}

func TestRegistryRejectsDuplicateTemplateVersions(t *testing.T) {
	_, err := NewRegistry(DynamicPIPEndingV1(), DynamicPIPEndingV1())
	if err == nil {
		t.Fatal("NewRegistry() = nil error, want a rejection when one (code, version) is registered twice")
	}
	if !strings.Contains(err.Error(), "dynamic_pip_ending") || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("NewRegistry() error = %v, want it to name the duplicated version", err)
	}
}

// An unknown version is the failure the four-layer model exists to make
// impossible to paper over: no database record can claim an executable
// capability that no shipped code provides.
func TestRegistryRejectsAnUnknownTemplateVersion(t *testing.T) {
	registry, err := NewRegistry(DynamicPIPEndingV1())
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	for _, lookup := range []struct {
		code    string
		version uint32
	}{
		{"dynamic_pip_ending", 2},
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

func TestDeployedTemplateTaskEditabilityIsExplicitAndGuideURLLocked(t *testing.T) {
	for _, definition := range DeployedDefinitions() {
		var schema struct {
			Properties map[string]struct {
				Editable *bool `json:"editable"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(definition.Schema, &schema); err != nil {
			t.Fatal(err)
		}
		for name, property := range schema.Properties {
			want := name == "start_seconds" || name == "end_seconds" || name == "insert_seconds" || name == "max_duration_seconds" || name == "guide_duration_seconds"
			if property.Editable == nil || *property.Editable != want {
				t.Errorf("%s.%s editable=%v want %v", definition.Code, name, property.Editable, want)
			}
		}
	}
}

func TestOnlyCurrentCompositionTemplateIsDeployed(t *testing.T) {
	definitions := DeployedDefinitions()
	if len(definitions) != 1 || definitions[0].Code != "dynamic_pip_ending" {
		t.Fatalf("retired templates are still executable: %+v", definitions)
	}
	for _, retired := range []string{"basic_cut", "guide_insert", "guide_pip"} {
		if _, err := DeployedDefinition(retired, 1); !errors.Is(err, ErrUnknownTemplateVersion) {
			t.Fatalf("retired %s lookup=%v", retired, err)
		}
	}
}
