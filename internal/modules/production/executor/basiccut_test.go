package executor

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestBasicCutV1PublishesItsSchema(t *testing.T) {
	definition := BasicCutV1()
	var schema map[string]any
	if err := json.Unmarshal(definition.Schema, &schema); err != nil {
		t.Fatalf("published schema is not JSON: %v", err)
	}
	properties, _ := schema["properties"].(map[string]any)
	if properties == nil {
		t.Fatalf("published schema has no properties: %s", definition.Schema)
	}
	if _, ok := properties["start_seconds"]; !ok {
		t.Fatalf("published schema has no start_seconds: %s", definition.Schema)
	}
	if _, ok := properties["end_seconds"]; !ok {
		t.Fatalf("published schema has no end_seconds: %s", definition.Schema)
	}
	if len(properties) != 2 {
		t.Fatalf("published schema has %d properties, want exactly the two v1 can execute", len(properties))
	}
	if additional, ok := schema["additionalProperties"].(bool); !ok || additional {
		t.Fatalf("published schema additionalProperties = %v, want false", schema["additionalProperties"])
	}
	required, _ := schema["required"].([]any)
	if len(required) != 2 || required[0] != "start_seconds" || required[1] != "end_seconds" {
		t.Fatalf("published schema required = %v, want both parameters", required)
	}
}

func TestBasicCutV1ValidatesParamsAgainstTheSchema(t *testing.T) {
	definition := BasicCutV1()
	plan, err := definition.Executor.ValidateParams(json.RawMessage(`{"start_seconds": 5, "end_seconds": 12}`))
	if err != nil {
		t.Fatalf("ValidateParams() error = %v", err)
	}
	if args := plan.FFmpegArguments("in.mp4", "out.mp4"); args == nil {
		t.Fatal("plan.FFmpegArguments() = nil, want a runnable command")
	}
}

// Params that the v1 schema does not declare must not parse at all — an asset
// key without a contract is absent from the schema, not present-but-rejected.
func TestBasicCutV1RejectsParamsOutsideTheSchema(t *testing.T) {
	definition := BasicCutV1()
	for name, raw := range map[string]string{
		"asset key":        `{"start_seconds": 5, "end_seconds": 12, "intro": "/tmp/intro.mp4"}`,
		"missing end":      `{"start_seconds": 5}`,
		"end before start": `{"start_seconds": 12, "end_seconds": 5}`,
		"zero length":      `{"start_seconds": 5, "end_seconds": 5}`,
		"negative start":   `{"start_seconds": -1, "end_seconds": 5}`,
		"fractional":       `{"start_seconds": 5.5, "end_seconds": 12}`,
		"string param":     `{"start_seconds": "5", "end_seconds": 12}`,
		"malformed json":   `{"start_seconds": 5, "end_seconds": `,
		"empty params":     `{}`,
	} {
		_, err := definition.Executor.ValidateParams(json.RawMessage(raw))
		if err == nil {
			t.Fatalf("ValidateParams(%s) = nil error, want a rejection", name)
		}
	}
}

// The plan reports the window it asks for so the pipeline can check the input
// covers it and the output approximates it.
func TestBasicCutV1ReportsItsSegmentWindow(t *testing.T) {
	definition := BasicCutV1()
	plan, err := definition.Executor.ValidateParams(json.RawMessage(`{"start_seconds": 5, "end_seconds": 12}`))
	if err != nil {
		t.Fatalf("ValidateParams() error = %v", err)
	}

	startMS, endMS := plan.SegmentMS()
	if startMS != 5000 || endMS != 12000 {
		t.Fatalf("SegmentMS() = (%d, %d), want (5000, 12000)", startMS, endMS)
	}
}

// The command is a fixed construction: the whole argument slice is asserted,
// so no future edit can quietly widen what the executor runs.
func TestBasicCutV1BuildsTheFixedFFmpegCommand(t *testing.T) {
	definition := BasicCutV1()
	plan, err := definition.Executor.ValidateParams(json.RawMessage(`{"start_seconds": 5, "end_seconds": 12}`))
	if err != nil {
		t.Fatalf("ValidateParams() error = %v", err)
	}

	got := plan.FFmpegArguments("/work/input.mp4", "/work/output.mp4")
	want := []string{
		"-hide_banner", "-loglevel", "error",
		"-ss", "5", "-to", "12", "-i", "/work/input.mp4",
		"-c:v", "libx264", "-c:a", "aac",
		"-movflags", "+faststart",
		"-y", "/work/output.mp4",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FFmpegArguments() =\n%v\nwant\n%v", got, want)
	}
	for _, forbidden := range []string{"-filter_complex", "-vf", "-af", "-f lavfi"} {
		if strings.Contains(strings.Join(got, " "), forbidden) {
			t.Fatalf("FFmpegArguments() contains %q, want no filter graph in v1", forbidden)
		}
	}
}
