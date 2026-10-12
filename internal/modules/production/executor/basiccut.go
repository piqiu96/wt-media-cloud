package executor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

// basicCutSchema is the schema published with basic_cut v1. It declares exactly
// the two parameters v1 can execute; asset keys (intro, outro, bgm) belong to
// later template versions, after their resource contracts and execution
// capabilities exist (ADR-0021 ruling 5).
const basicCutSchema = `{
	"type": "object",
	"properties": {
		"start_seconds": {"type": "integer", "minimum": 0},
		"end_seconds": {"type": "integer", "minimum": 1}
	},
	"required": ["start_seconds", "end_seconds"],
	"additionalProperties": false
}`

// BasicCut is the executor for the basic_cut template: cut one input video to a
// segment and re-encode it as H.264/AAC MP4.
type BasicCut struct{}

// BasicCutV1 returns the definition of basic_cut version 1: the identity, the
// schema published with it, and this code as its executor.
func BasicCutV1() Definition {
	return Definition{
		Code:     "basic_cut",
		Version:  1,
		Schema:   json.RawMessage(basicCutSchema),
		Executor: BasicCut{},
	}
}

// cutPlan is the validated form of basic_cut parameters.
type cutPlan struct {
	startSeconds int64
	endSeconds   int64
}

// ValidateParams accepts only the object the v1 schema declares, with integer
// bounds the schema states: a non-negative start strictly before the end.
func (BasicCut) ValidateParams(raw json.RawMessage) (Plan, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("basic_cut v1 params are not a JSON object: %w", err)
	}
	for name := range fields {
		if name != "start_seconds" && name != "end_seconds" {
			return nil, fmt.Errorf("basic_cut v1 has no parameter %q", name)
		}
	}
	if _, ok := fields["start_seconds"]; !ok {
		return nil, fmt.Errorf("basic_cut v1 requires start_seconds")
	}
	if _, ok := fields["end_seconds"]; !ok {
		return nil, fmt.Errorf("basic_cut v1 requires end_seconds")
	}

	start, err := integerParam(fields["start_seconds"], "start_seconds")
	if err != nil {
		return nil, err
	}
	end, err := integerParam(fields["end_seconds"], "end_seconds")
	if err != nil {
		return nil, err
	}
	if start < 0 {
		return nil, fmt.Errorf("basic_cut v1 start_seconds must not be negative, got %d", start)
	}
	if start >= end {
		return nil, fmt.Errorf("basic_cut v1 needs start_seconds (%d) strictly before end_seconds (%d)", start, end)
	}
	return cutPlan{startSeconds: start, endSeconds: end}, nil
}

func integerParam(raw json.RawMessage, name string) (int64, error) {
	// The raw bytes must be a number literal, not merely parse as one: the
	// decoder accepts a quoted "5" into json.Number, and a schema type of
	// integer must not.
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || (trimmed[0] != '-' && (trimmed[0] < '0' || trimmed[0] > '9')) {
		return 0, fmt.Errorf("basic_cut v1 %s must be an integer, got %s", name, string(trimmed))
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err != nil {
		return 0, fmt.Errorf("basic_cut v1 %s must be an integer: %w", name, err)
	}
	value, err := number.Int64()
	if err != nil {
		return 0, fmt.Errorf("basic_cut v1 %s must be an integer, got %s", name, number.String())
	}
	return value, nil
}

// FFmpegArguments is the fixed command for a cut run. It is the entire command:
// input seeking on the input timeline, a re-encode to H.264/AAC, and a
// faststart MP4. No filter graph, no script, no caller-supplied tokens.
func (p cutPlan) FFmpegArguments(inputPath, outputPath string) []string {
	return []string{
		"-hide_banner", "-loglevel", "error",
		"-ss", strconv.FormatInt(p.startSeconds, 10),
		"-to", strconv.FormatInt(p.endSeconds, 10),
		"-i", inputPath,
		"-c:v", "libx264", "-c:a", "aac",
		"-movflags", "+faststart",
		"-y", outputPath,
	}
}
