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
	productionservice "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
)

// Response bodies must match the Cloud-owned business schema consumed by both frontends.

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

func schemaPropertyNames[T any](properties map[string]T) []string {
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
		ViewCount:       12000,
		LikeCount:       72313,
		FavoriteCount:   882,
		CommentCount:    4028,
		ShareCount:      8323,
		PublishedAt:     &published,
		Status:          model.MaterialAvailable,
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
	if len(want) != 25 {
		t.Fatalf("Material declares %d properties, want the measured 25: %v", len(want), want)
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
	if len(want) != 10 {
		t.Fatalf("MaterialUsage declares %d properties, want the measured 10: %v", len(want), want)
	}

	removed := activeUsage()
	removed.Status = model.MaterialUsageRemoved
	removedAt := removed.UpdatedAt
	removed.RemovedAt = &removedAt
	// download_status is derived and optional: the removed row carries one so the
	// union below proves the marshalled set reaches the whole declared schema.
	removed.DownloadStatus = "downloaded"

	// Three optional properties (`material`, `removed_at`, `download_status`), so
	// the active row and the removed row together have to account for the whole
	// declared set — and only for it. A never-downloaded active row stays at the
	// required set plus the embedded material, which the second assertion pins.
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

func TestComposeTemplateBodyMatchesContract(t *testing.T) {
	schemas := readContentProductionSchema(t)
	schema, ok := schemas["ComposeTemplate"]
	if !ok {
		t.Fatal("missing ComposeTemplate schema")
	}
	if got, want := marshalledKeys(t, model.ComposeTemplate{}), schemaPropertyNames(schema.Properties); !reflect.DeepEqual(got, want) {
		t.Errorf("ComposeTemplate keys = %v, contract = %v", got, want)
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

// The ComposeTask the API returns is the model's own JSON; the OpenAPI file is
// the contract both frontends code against. This reads the schema and asserts
// the two key sets are equal, so a projection the worker starts writing — or a
// schema line someone adds — fails here instead of drifting in a client.
// The ComposeTask lives only in the OpenAPI file (M4-C5), not the business
// schema, so it is read from there directly.
func TestTheComposeTaskResponseMatchesTheOpenAPISchema(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "cloud-api", "v1", "content-production.openapi.yaml"))
	if err != nil {
		t.Fatalf("read compose task openapi: %v", err)
	}
	var parsed struct {
		Components struct {
			Schemas struct {
				ComposeTask struct {
					Properties map[string]yaml.Node `yaml:"properties"`
				} `yaml:"ComposeTask"`
			} `yaml:"schemas"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("parse compose task openapi: %v", err)
	}
	schema := parsed.Components.Schemas.ComposeTask.Properties
	if len(schema) == 0 {
		t.Fatal("the OpenAPI file declares no ComposeTask properties")
	}
	if got, want := marshalledKeys(t, model.ComposeTask{}), schemaPropertyNames(schema); !reflect.DeepEqual(got, want) {
		t.Errorf("ComposeTask keys = %v, contract = %v", got, want)
	}
}

// The compose output projection must never carry the object key — the address
// stays server-side and previews go through the presigned video-url endpoint.
func TestTheComposeOutputResponseMatchesTheOpenAPISchema(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "cloud-api", "v1", "content-production.openapi.yaml"))
	if err != nil {
		t.Fatalf("read compose output openapi: %v", err)
	}
	var parsed struct {
		Components struct {
			Schemas struct {
				ComposeOutput struct {
					Properties map[string]yaml.Node `yaml:"properties"`
				} `yaml:"ComposeOutput"`
			} `yaml:"schemas"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("parse compose output openapi: %v", err)
	}
	schema := parsed.Components.Schemas.ComposeOutput.Properties
	if len(schema) == 0 {
		t.Fatal("the OpenAPI file declares no ComposeOutput properties")
	}
	if got, want := marshalledKeys(t, model.CompositeOutput{}), schemaPropertyNames(schema); !reflect.DeepEqual(got, want) {
		t.Errorf("ComposeOutput keys = %v, contract = %v", got, want)
	}
}

func TestTheComposeOutputDetailMatchesTheOpenAPISchema(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "cloud-api", "v1", "content-production.openapi.yaml"))
	if err != nil {
		t.Fatalf("read compose output openapi: %v", err)
	}
	var parsed struct {
		Components struct {
			Schemas struct {
				Detail struct {
					Properties map[string]yaml.Node `yaml:"properties"`
				} `yaml:"ComposeOutputDetail"`
				Trace struct {
					Properties map[string]yaml.Node `yaml:"properties"`
				} `yaml:"ComposeOutputTrace"`
			} `yaml:"schemas"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("parse compose output openapi: %v", err)
	}
	detail := parsed.Components.Schemas.Detail.Properties
	if len(detail) == 0 {
		t.Fatal("the OpenAPI file declares no ComposeOutputDetail properties")
	}
	if got, want := marshalledKeys(t, productionservice.ComposeOutputDetail{}), schemaPropertyNames(detail); !reflect.DeepEqual(got, want) {
		t.Errorf("ComposeOutputDetail keys = %v, contract = %v", got, want)
	}
	trace := parsed.Components.Schemas.Trace.Properties
	if len(trace) == 0 {
		t.Fatal("the OpenAPI file declares no ComposeOutputTrace properties")
	}
	if got, want := marshalledKeys(t, productionservice.ComposeOutputTrace{}), schemaPropertyNames(trace); !reflect.DeepEqual(got, want) {
		t.Errorf("ComposeOutputTrace keys = %v, contract = %v", got, want)
	}
}

// CHG-20261013-085 (M4-C7 T-01): the output download endpoint mirrors the
// material one — a session POST that queues a local-agent user-download task
// and refuses an out-of-scope output with the same 404 shape the page reads.
func TestTheOutputDownloadEndpointIsInTheContract(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "cloud-api", "v1", "content-production.openapi.yaml"))
	if err != nil {
		t.Fatalf("read content-production openapi: %v", err)
	}
	var parsed struct {
		Paths map[string]struct {
			Post struct {
				OperationID string `yaml:"operationId"`
				Responses   map[string]yaml.Node
			} `yaml:"post"`
		} `yaml:"paths"`
		Components struct {
			Parameters map[string]yaml.Node `yaml:"parameters"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("parse content-production openapi: %v", err)
	}
	path, ok := parsed.Paths["/api/v1/compose-outputs/{output_id}/downloads"]
	if !ok {
		t.Fatal("the output download path is not declared")
	}
	if path.Post.OperationID != "createOutputDownload" {
		t.Fatalf("operationId = %q", path.Post.OperationID)
	}
	for _, status := range []string{"202", "404"} {
		if _, ok := path.Post.Responses[status]; !ok {
			t.Fatalf("missing %s response on the output download endpoint", status)
		}
	}
	if _, ok := parsed.Components.Parameters["OutputID"]; !ok {
		t.Fatal("contract declares no OutputID parameter")
	}
}
