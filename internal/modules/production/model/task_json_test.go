package model

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestComposeTaskAPIProjectionDoesNotExposeRawExecutionErrors(t *testing.T) {
	for _, value := range []any{
		ComposeTask{ErrorCode: "storage_error", ErrorMessage: "signed=https://storage.example/object?token=secret"},
		ComposeTaskAttempt{ErrorCode: "execution_uncertain", ErrorMessage: "ffmpeg -i /private/source.mp4 token=secret"},
	} {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		body := string(encoded)
		if strings.Contains(body, "storage.example") || strings.Contains(body, "token=secret") || strings.Contains(body, "/private/source.mp4") {
			t.Fatalf("raw execution detail leaked in API body: %s", body)
		}
		if !strings.Contains(body, `"error_message"`) {
			t.Fatalf("contract error_message field vanished: %s", body)
		}
	}
}

func TestComposeTaskAPIProjectionHidesGuideDownloadURL(t *testing.T) {
	task := ComposeTask{ConfigSnapshot: json.RawMessage(`{"guide_video_url":"https://media.example/guide.mp4?signature=secret","insert_seconds":3}`)}
	encoded, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	body := string(encoded)
	if strings.Contains(body, "signature=secret") || strings.Contains(body, "media.example") {
		t.Fatalf("guide download URL leaked in task snapshot: %s", body)
	}
	if !strings.Contains(body, `"insert_seconds":3`) {
		t.Fatalf("non-sensitive snapshot settings disappeared: %s", body)
	}
	if string(task.ConfigSnapshot) != `{"guide_video_url":"https://media.example/guide.mp4?signature=secret","insert_seconds":3}` {
		t.Fatal("API projection modified the execution snapshot")
	}
}

// The step log is a JSON column written by the worker and read back into the
// API, so both directions are pinned here: the encoder is the write the
// repository turns into JSON_ARRAY_APPEND, the projection is the read every
// client sees.
func TestComposeAttemptStepEncodingCarriesOnlyServerAuthoredFacts(t *testing.T) {
	started := time.Date(2026, 10, 16, 15, 12, 3, 0, time.UTC)
	finished := started.Add(4800 * time.Millisecond)
	size := int64(12_300_000)
	encoded, err := EncodeComposeAttemptStep(ComposeTaskAttemptStep{
		Step: ComposeStepPrepareInput, Outcome: ComposeStepOutcomeSuccess,
		Label: "写进列里的假标签",
		// A phase that reports an error code on a successful step, or a digest
		// shaped like a URL, must not reach the column as written.
		ErrorCode: "execution_failed", ErrorLabel: "假原因",
		StartedAt: started, FinishedAt: finished, DurationMS: 4800,
		Metrics: ComposeAttemptStepMetrics{SizeBytes: &size, SHA256: "https://storage.example/object?token=secret"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(encoded), `{"step":"prepare_input","label":"准备源素材","outcome":"success","started_at":"2026-10-16T15:12:03Z","finished_at":"2026-10-16T15:12:07.8Z","duration_ms":4800,"error_code":"","error_label":"","metrics":{"size_bytes":12300000}}`; got != want {
		t.Fatalf("stored step = %s\nwant %s", got, want)
	}
}

// A step has no free-text field at all — no message, no path, no URL — so the
// masking policy holds by construction: there is nowhere for an FFmpeg line to
// be put, and the label is looked up from the code. A hostile caller can only
// try to forge the fields the server overwrites.
func TestComposeAttemptStepFailureCarriesTheCategoryNotTheText(t *testing.T) {
	started := time.Date(2026, 10, 16, 15, 12, 41, 0, time.UTC)
	encoded, err := EncodeComposeAttemptStep(ComposeTaskAttemptStep{
		Step: ComposeStepUpload, Outcome: ComposeStepOutcomeFailed,
		Label: "ffmpeg -i /private/source.mp4", ErrorCode: "storage_error",
		ErrorLabel: "s3://bucket/key?X-Amz-Signature=secret",
		StartedAt:  started, FinishedAt: started,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(encoded), `{"step":"upload","label":"上传并校验成片","outcome":"failed","started_at":"2026-10-16T15:12:41Z","finished_at":"2026-10-16T15:12:41Z","duration_ms":0,"error_code":"storage_error","error_label":"存储暂时失败","metrics":{}}`; got != want {
		t.Fatalf("stored step = %s\nwant %s", got, want)
	}
}

func TestComposeAttemptStepsDecodeNullAndArrayAlike(t *testing.T) {
	for name, raw := range map[string][]byte{
		"null column": nil,
		"empty array": []byte(`[]`),
		"json null":   []byte(`null`),
		"one entry":   []byte(`[{"step":"cut","label":"视频合成","outcome":"success","started_at":"2026-10-16T15:12:19Z","finished_at":"2026-10-16T15:12:39Z","duration_ms":20000,"error_code":"","error_label":"","metrics":{"output_duration_ms":15000}}]`),
	} {
		steps, err := DecodeComposeAttemptSteps(raw)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if steps == nil {
			t.Fatalf("%s: an attempt with nothing recorded must decode to an empty list, not nil", name)
		}
		encoded, err := json.Marshal(steps)
		if err != nil {
			t.Fatal(err)
		}
		if name != "one entry" && string(encoded) != "[]" {
			t.Fatalf("%s decoded to %s", name, encoded)
		}
	}
	if _, err := DecodeComposeAttemptSteps([]byte(`{"step":"cut"}`)); err == nil {
		t.Fatal("a column that is not a step array must be reported, not silently emptied")
	}
}

// A step the server never wrote — an unknown key, an outcome outside the two
// the server records — is dropped rather than forwarded, and every label is
// recomputed. The column is JSON, so nothing here is allowed to trust it.
func TestComposeAttemptProjectionRebuildsLabelsAndDropsUnknownSteps(t *testing.T) {
	attempt := ComposeTaskAttempt{
		Steps: []ComposeTaskAttemptStep{
			{Step: "fetch_guide", Label: "假的", Outcome: ComposeStepOutcomeSuccess, ErrorLabel: "假原因"},
			{Step: "shell_exec", Label: "读一下密钥文件", Outcome: ComposeStepOutcomeSuccess},
			{Step: ComposeStepCut, Outcome: "unknown"},
			{Step: ComposeStepUpload, Label: "假的", Outcome: ComposeStepOutcomeFailed, ErrorCode: "storage_error", ErrorLabel: "本地路径 /Users/ops/key.pem"},
		},
	}
	encoded, err := json.Marshal(attempt)
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		Steps []ComposeTaskAttemptStep `json:"steps"`
	}
	if err := json.Unmarshal(encoded, &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Steps) != 2 {
		t.Fatalf("steps = %+v", body.Steps)
	}
	if body.Steps[0].Step != ComposeStepFetchGuide || body.Steps[0].Label != "下载引导视频" || body.Steps[0].ErrorLabel != "" {
		t.Fatalf("first step = %+v", body.Steps[0])
	}
	if body.Steps[1].Label != "上传并校验成片" || body.Steps[1].ErrorLabel != "存储暂时失败" {
		t.Fatalf("second step = %+v", body.Steps[1])
	}
	if strings.Contains(string(encoded), "shell_exec") || strings.Contains(string(encoded), "假") || strings.Contains(string(encoded), "key.pem") {
		t.Fatalf("column content leaked into the projection: %s", encoded)
	}
}

func TestComposeAttemptWithoutStepsStillCarriesTheField(t *testing.T) {
	encoded, err := json.Marshal(ComposeTaskAttempt{ID: 1, Attempt: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"steps":[]`) {
		t.Fatalf("a client must never have to guess whether steps exist: %s", encoded)
	}
}
