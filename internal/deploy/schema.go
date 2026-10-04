package deploy

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

var variableNamePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

type VariableSchema struct {
	SchemaVersion int            `toml:"schema_version"`
	Variables     []VariableSpec `toml:"variables"`
}

type VariableSpec struct {
	Name       string `toml:"name"`
	Type       string `toml:"type"`
	Required   bool   `toml:"required"`
	Secret     bool   `toml:"secret"`
	AllowEmpty bool   `toml:"allow_empty"`
}

func LoadVariableSchema(path string) (VariableSchema, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return VariableSchema{}, fmt.Errorf("read variable schema: %w", err)
	}
	var schema VariableSchema
	if err := toml.Unmarshal(raw, &schema); err != nil {
		return VariableSchema{}, fmt.Errorf("parse variable schema: %w", err)
	}
	if schema.SchemaVersion != 1 {
		return VariableSchema{}, errors.New("variable schema_version must be 1")
	}
	seen := make(map[string]bool, len(schema.Variables))
	for _, spec := range schema.Variables {
		if !variableNamePattern.MatchString(spec.Name) {
			return VariableSchema{}, fmt.Errorf("invalid schema variable name %q", spec.Name)
		}
		if seen[spec.Name] {
			return VariableSchema{}, fmt.Errorf("duplicate schema variable %q", spec.Name)
		}
		seen[spec.Name] = true
		switch spec.Type {
		case "string", "integer", "boolean":
		default:
			return VariableSchema{}, fmt.Errorf("unsupported type for %s: %s", spec.Name, spec.Type)
		}
		if spec.Required && spec.AllowEmpty {
			return VariableSchema{}, fmt.Errorf("required variable %s cannot allow empty", spec.Name)
		}
	}
	return schema, nil
}

func (s VariableSchema) Names() []string {
	names := make([]string, 0, len(s.Variables))
	for _, spec := range s.Variables {
		names = append(names, spec.Name)
	}
	sort.Strings(names)
	return names
}

func (s VariableSchema) Spec(name string) (VariableSpec, bool) {
	for _, spec := range s.Variables {
		if spec.Name == name {
			return spec, true
		}
	}
	return VariableSpec{}, false
}

func VerifySchemaMatchesTemplates(schema VariableSchema, templateNames []string) error {
	want := make(map[string]bool, len(templateNames))
	for _, name := range templateNames {
		want[name] = true
	}
	have := make(map[string]bool, len(schema.Variables))
	for _, spec := range schema.Variables {
		have[spec.Name] = true
	}
	var missingSchema, missingTemplate []string
	for name := range want {
		if !have[name] {
			missingSchema = append(missingSchema, name)
		}
	}
	for name := range have {
		if !want[name] {
			missingTemplate = append(missingTemplate, name)
		}
	}
	sort.Strings(missingSchema)
	sort.Strings(missingTemplate)
	if len(missingSchema) > 0 || len(missingTemplate) > 0 {
		var parts []string
		if len(missingSchema) > 0 {
			parts = append(parts, "missing schema: "+strings.Join(missingSchema, ", "))
		}
		if len(missingTemplate) > 0 {
			parts = append(parts, "missing templates: "+strings.Join(missingTemplate, ", "))
		}
		return errors.New("variable schema/template mismatch; " + strings.Join(parts, "; "))
	}
	return nil
}
