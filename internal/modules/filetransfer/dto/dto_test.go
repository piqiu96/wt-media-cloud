package dto

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The key sets below are read out of the frozen contracts rather than repeated
// here. A hardcoded list would only pin what the DTO does today; reading the
// schema is what makes "the body matches the contract" a statement about the two
// together, and it is the drift this package's whole job is to prevent — the
// frontend and the Agent are both written against these files, not against this
// Go type.

func readContract(t *testing.T, relative string) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "..", relative))
	if err != nil {
		t.Fatalf("read %s: %v", relative, err)
	}
	return content
}

type businessSchema struct {
	Schemas map[string]struct {
		Required            []string       `yaml:"required"`
		Properties          map[string]any `yaml:"properties"`
		ForbiddenProperties []string       `yaml:"forbidden_properties"`
	} `yaml:"schemas"`
}

func businessFileTransferSchema(t *testing.T) businessSchema {
	t.Helper()
	var parsed businessSchema
	if err := yaml.Unmarshal(readContract(t, "contracts/business-schemas/v1/file-transfer.yaml"), &parsed); err != nil {
		t.Fatalf("parse business schema: %v", err)
	}
	schema, ok := parsed.Schemas["FileTransferTask"]
	if !ok {
		// Loud rather than empty: an empty property map would make the comparison
		// below pass against a DTO whose keys had all been renamed.
		t.Fatal("contract has no FileTransferTask schema")
	}
	if len(schema.Properties) == 0 {
		t.Fatal("FileTransferTask declares no properties; the comparison below would be vacuous")
	}
	return parsed
}

// marshalledKeys returns the keys of a marshalled struct as a sorted slice.
func marshalledKeys(t *testing.T, value any) []string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal %T: %v", value, err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatalf("decode %T: %v", value, err)
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedCopy(values []string) []string {
	// The schema's `required` and `forbidden_properties` lists are written in
	// reading order; the marshalled keys are sorted, because Go's map iteration is
	// random and a flaky order would report a difference that is not one.
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

// The zero value is what a pending task marshals to, and it must still carry the
// whole property set: a reader indexing into the body gets `0` rather than
// `undefined`, and a body missing `total_bytes` is not the same body as one
// saying there is nothing to transfer.
func TestTaskMarshalsExactlyTheFrozenPropertySet(t *testing.T) {
	schema := businessFileTransferSchema(t).Schemas["FileTransferTask"]

	want := make([]string, 0, len(schema.Properties))
	for key := range schema.Properties {
		want = append(want, key)
	}
	sort.Strings(want)

	// The denominator, measured here. A property quietly dropped from the schema would
	// otherwise keep this comparison green while shrinking what the frontend is promised.
	if len(want) != 19 || len(schema.Required) != 12 {
		t.Fatalf("contract declares %d properties / %d required, want the measured 19 / 12", len(want), len(schema.Required))
	}

	if got := marshalledKeys(t, Task{}); !reflect.DeepEqual(got, want) {
		t.Fatalf("Task keys = %v, want %v", got, want)
	}

	// Every required property has to be among the emitted keys, which the equality
	// above implies — but asserted separately so that a future decision to omit
	// optional keys cannot quietly drop a required one.
	emitted := make(map[string]bool, len(want))
	for _, key := range want {
		emitted[key] = true
	}
	for _, key := range schema.Required {
		if !emitted[key] {
			t.Errorf("required property %q is not emitted", key)
		}
	}
}

// The forbidden properties are absent by construction — nothing in `Task`
// declares them — and this asserts it against the contract's own list rather
// than against a copy, so a property added to both would still be caught.
func TestTaskNeverCarriesAnAddressOrACredential(t *testing.T) {
	schema := businessFileTransferSchema(t).Schemas["FileTransferTask"]
	if len(schema.ForbiddenProperties) == 0 {
		t.Fatal("contract declares no forbidden_properties; this check would be vacuous")
	}
	emitted := make(map[string]bool)
	for _, key := range marshalledKeys(t, Task{}) {
		emitted[key] = true
	}
	for _, forbidden := range schema.ForbiddenProperties {
		if emitted[forbidden] {
			t.Errorf("Task must not carry %q", forbidden)
		}
	}
}

// A nullable property has to marshal as null rather than as a zero value: "" and
// `null` are both schema-valid strings, and only one of them says "Cloud does not
// know this yet".
func TestTaskMarshalsUnknownNullableFieldsAsNull(t *testing.T) {
	data, err := json.Marshal(Task{})
	if err != nil {
		t.Fatalf("marshal Task: %v", err)
	}
	for _, key := range []string{"estimated_remaining_seconds", "checksum_sha256", "file_name", "error_code", "error_message"} {
		if !strings.Contains(string(data), `"`+key+`":null`) {
			t.Errorf("%s = not null in %s", key, data)
		}
	}
}

func readCloudAgentSchemas(t *testing.T) map[string]struct {
	Required   []string       `yaml:"required"`
	Properties map[string]any `yaml:"properties"`
} {
	t.Helper()
	var parsed struct {
		Components struct {
			Schemas map[string]struct {
				Required   []string       `yaml:"required"`
				Properties map[string]any `yaml:"properties"`
			} `yaml:"schemas"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(readContract(t, "contracts/cloud-agent-api/v1/file-transfer.openapi.yaml"), &parsed); err != nil {
		t.Fatalf("parse cloud-agent contract: %v", err)
	}
	if len(parsed.Components.Schemas) == 0 {
		t.Fatal("cloud-agent contract declares no component schemas; the checks below would be vacuous")
	}
	return parsed.Components.Schemas
}

func TestLeaseAndTerminalMatchTheFrozenExecutorContract(t *testing.T) {
	schemas := readCloudAgentSchemas(t)

	for _, testCase := range []struct {
		schema string
		value  any
	}{
		{"LocalLease", LocalLease{}},
		{"TransferTerminal", TransferTerminal{}},
	} {
		properties := schemas[testCase.schema].Properties
		if len(properties) == 0 {
			t.Fatalf("contract has no %s schema", testCase.schema)
		}
		want := make([]string, 0, len(properties))
		for key := range properties {
			want = append(want, key)
		}
		sort.Strings(want)

		got := marshalledKeys(t, testCase.value)
		if testCase.schema == "TransferTerminal" {
			// `file_name` is optional and not nullable, so it is omitted until the
			// executor reports one.
			got = append(got, "file_name")
			sort.Strings(got)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s keys = %v, want %v", testCase.schema, got, want)
		}
	}
}

// The no-task result has to be a null `task` inside a 200, not a missing key and
// not an error status: the executor polls this endpoint, and a poll that finds
// nothing to do is the normal case.
func TestClaimResultAnswersNoTaskAsAnExplicitNull(t *testing.T) {
	data, err := json.Marshal(ClaimResult{})
	if err != nil {
		t.Fatalf("marshal ClaimResult: %v", err)
	}
	if string(data) != `{"task":null}` {
		t.Fatalf("ClaimResult = %s, want {\"task\":null}", data)
	}
}
