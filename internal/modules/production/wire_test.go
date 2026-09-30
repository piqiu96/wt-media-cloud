package production

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
)

// The bodies this package serves are pinned to `contracts/business-schemas/v1/content-production.yaml`.
//
// This file is the instrument that was missing. The schema and `model.Material` were
// written from two different sources — the schema from `change.md` §3.1's prose, the
// struct from the table's column names — and gave the same five facts two sets of
// names (`file_size_bytes`/`sha256` versus `video_size_bytes`/`video_sha256`). Nothing
// compared them, so the two were free to disagree and did. The comparison below is the
// point, not the names it currently happens to accept: a rename on either side has to
// be a deliberate, visible act.
//
// The frontend is written against the contract file, not against this Go type. So the
// key sets are read out of the contract here rather than repeated, exactly as
// `filetransfer/dto` does for the transfer bodies.

func readContentProductionSchema(t *testing.T) map[string]struct {
	Required            []string       `yaml:"required"`
	Properties          map[string]any `yaml:"properties"`
	ForbiddenProperties []string       `yaml:"forbidden_properties"`
} {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "business-schemas", "v1", "content-production.yaml"))
	if err != nil {
		t.Fatalf("read content-production schema: %v", err)
	}
	var parsed struct {
		Schemas map[string]struct {
			Required            []string       `yaml:"required"`
			Properties          map[string]any `yaml:"properties"`
			ForbiddenProperties []string       `yaml:"forbidden_properties"`
		} `yaml:"schemas"`
	}
	if err := yaml.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("parse content-production schema: %v", err)
	}
	return parsed.Schemas
}

// marshalledKeys returns the keys of a marshalled value, sorted.
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
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func schemaPropertyNames(properties map[string]any) []string {
	names := make([]string, 0, len(properties))
	for name := range properties {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// readyMaterial carries every optional field, so that a property the schema declares
// but the struct cannot produce shows up as a missing key rather than being hidden by
// a zero value's `omitempty`.
func readyMaterial() model.Material {
	game := "other"
	author := "作者"
	published := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	size := int64(4096)
	prepared := time.Date(2026, 9, 27, 8, 0, 0, 0, time.UTC)
	return model.Material{
		ID:              7,
		TeamID:          3,
		GameID:          &game,
		SourceContentID: 42,
		Title:           "一条素材",
		SourceURL:       "https://example.invalid/video/42",
		Platform:        "douyin",
		AuthorName:      author,
		CoverURL:        "https://example.invalid/cover/42.jpg",
		AuthorHomeURL:   "https://example.invalid/author/42",
		PublishedAt:     &published,
		VideoStatus:     model.VideoReady,
		VideoSizeBytes:  &size,
		VideoSHA256:     "0f343b0931126a20f133d67c2b018a3b",
		VideoMedia:      map[string]any{"duration_ms": 15000},
		VideoError:      "上一次准备失败的原因",
		VideoPreparedAt: &prepared,
		CreatedAt:       published,
		UpdatedAt:       prepared,
	}
}

// The denominator is asserted, not assumed: a property quietly dropped from the schema
// would otherwise keep this comparison green while shrinking what the frontend is
// promised. Changing the contract's property set has to edit these two lines too.
func TestMaterialBodyMarshalsExactlyTheFrozenPropertySet(t *testing.T) {
	schemas := readContentProductionSchema(t)
	schema, ok := schemas["Material"]
	if !ok {
		t.Fatal("contract has no Material schema")
	}
	want := schemaPropertyNames(schema.Properties)
	if len(want) != 19 {
		t.Fatalf("Material declares %d properties, want the measured 19: %v", len(want), want)
	}

	if got := marshalledKeys(t, readyMaterial()); !reflect.DeepEqual(got, want) {
		t.Fatalf("a fully known Material marshals to %v, want %v", got, want)
	}
}

// A required property that gained `omitempty` would vanish from the body whenever its
// value is zero, which is exactly the class of change nobody notices in review: the
// struct still compiles, and a reader indexing into the body gets `undefined`. The
// zero value is what a freshly materialized row marshals to.
func TestMaterialBodyStillCarriesEveryRequiredPropertyWhenNothingIsKnown(t *testing.T) {
	schemas := readContentProductionSchema(t)
	schema := schemas["Material"]
	if len(schema.Required) == 0 {
		t.Fatal("contract declares no required properties; this check would be vacuous")
	}

	got := marshalledKeys(t, model.Material{})
	if want := sortedCopy(schema.Required); !reflect.DeepEqual(got, want) {
		t.Fatalf("a Material with nothing known marshals to %v, want exactly the required set %v", got, want)
	}
}

// `object_key` is the case this check exists for. The contract used to declare it as a
// property while the struct had carried `json:"-"` all along — a schema promising the
// frontend a value that never arrives. It is listed as forbidden now, and this asserts
// the list against the body rather than against a second copy of it.
func TestMaterialBodyCarriesNoObjectReferenceOrAddress(t *testing.T) {
	schemas := readContentProductionSchema(t)
	schema := schemas["Material"]
	if len(schema.ForbiddenProperties) == 0 {
		t.Fatal("contract declares no forbidden_properties; this check would be vacuous")
	}

	emitted := make(map[string]bool)
	for _, key := range marshalledKeys(t, readyMaterial()) {
		emitted[key] = true
	}
	for _, forbidden := range schema.ForbiddenProperties {
		if emitted[forbidden] {
			t.Errorf("the material body must not carry %q", forbidden)
		}
	}
}

func activeUsage() model.MaterialUsage {
	material := readyMaterial()
	return model.MaterialUsage{
		ID:         11,
		TeamID:     3,
		MaterialID: material.ID,
		UserID:     5,
		Status:     model.MaterialUsageActive,
		CreatedAt:  material.CreatedAt,
		UpdatedAt:  material.UpdatedAt,
		Material:   &material,
	}
}

func TestMaterialUsageBodyMarshalsExactlyTheFrozenPropertySet(t *testing.T) {
	schemas := readContentProductionSchema(t)
	schema, ok := schemas["MaterialUsage"]
	if !ok {
		t.Fatal("contract has no MaterialUsage schema")
	}
	want := schemaPropertyNames(schema.Properties)
	if len(want) != 9 {
		t.Fatalf("MaterialUsage declares %d properties, want the measured 9: %v", len(want), want)
	}

	removed := activeUsage()
	removed.Status = model.MaterialUsageRemoved
	removedAt := removed.UpdatedAt
	removed.RemovedAt = &removedAt

	// Two optional properties (`material` and `removed_at`), so the active row and the
	// removed row together have to account for the whole declared set — and only for it.
	active := marshalledKeys(t, activeUsage())
	withRemovals := marshalledKeys(t, removed)
	if got := union(active, withRemovals); !reflect.DeepEqual(got, want) {
		t.Fatalf("MaterialUsage marshals to %v (active) / %v (removed), together %v, want %v", active, withRemovals, got, want)
	}
	if got, want := active, sortedCopy(append(append([]string(nil), schema.Required...), "material")); !reflect.DeepEqual(got, want) {
		t.Fatalf("an active usage marshals to %v, want the required set plus the embedded material %v", got, want)
	}
}

// The embedded material is the one property whose key set this file does not repeat: the
// contract says "its keys are `Material`'s" rather than listing seventeen again, so the
// agreement between the two is asserted here instead of being trusted.
func TestTheEmbeddedMaterialIsTheMaterialSchemaItself(t *testing.T) {
	schemas := readContentProductionSchema(t)
	materialProperties := schemaPropertyNames(schemas["Material"].Properties)
	if len(materialProperties) == 0 {
		t.Fatal("contract has no Material properties; this check would be vacuous")
	}

	usage := activeUsage()
	if usage.Material == nil {
		t.Fatal("the fixture has no embedded material; this check would be vacuous")
	}
	if got := marshalledKeys(t, *usage.Material); !reflect.DeepEqual(got, materialProperties) {
		t.Fatalf("the embedded material marshals to %v, want the Material property set %v", got, materialProperties)
	}
}

// The detail link body is pinned the same way the material body is: the handler
// struct and the contract were written from the same change, and only a test
// that compares them catches one side shrinking without the other.
func TestVideoLinkBodyMarshalsExactlyTheFrozenPropertySet(t *testing.T) {
	schemas := readContentProductionSchema(t)
	schema, ok := schemas["MaterialVideoLink"]
	if !ok {
		t.Fatal("contract has no MaterialVideoLink schema")
	}
	want := schemaPropertyNames(schema.Properties)
	if len(want) != 1 || want[0] != "url" {
		t.Fatalf("MaterialVideoLink declares %v, want exactly [url]", want)
	}
	if len(schema.Required) != 1 || schema.Required[0] != "url" {
		t.Fatalf("MaterialVideoLink requires %v, want exactly [url]", schema.Required)
	}
	if got := marshalledKeys(t, VideoLink{URL: "https://example.invalid/wt-media/materials/42/x.mp4"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("a video link marshals to %v, want %v", got, want)
	}
}

func union(left, right []string) []string {
	seen := make(map[string]bool, len(left)+len(right))
	for _, values := range [][]string{left, right} {
		for _, value := range values {
			seen[value] = true
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
