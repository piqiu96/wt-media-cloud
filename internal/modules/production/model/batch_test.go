package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBatchProgressMeasuresTerminalTasks(t *testing.T) {
	for _, tt := range []struct {
		counts ComposeBatchCounts
		want   int
	}{
		{ComposeBatchCounts{Running: 1}, 0}, {ComposeBatchCounts{Success: 1, Failed: 1, Pending: 2}, 50},
		{ComposeBatchCounts{Cancelled: 2}, 100}, {ComposeBatchCounts{}, 0},
	} {
		if got := tt.counts.ProgressPercent(); got != tt.want {
			t.Fatalf("%+v: got %d want %d", tt.counts, got, tt.want)
		}
	}
}

func TestBatchSnapshotHidesGuideAddress(t *testing.T) {
	b := ComposeTaskBatch{ConfigSnapshot: json.RawMessage(`[{"key":"a","name":"A","copies":1,"platforms":["baidu"],"params":{"guide_video_url":"https://secret.example/token","insert_seconds":2}}]`)}
	raw, err := json.Marshal(b)
	if err != nil || strings.Contains(string(raw), "secret.example") {
		t.Fatalf("leaked %s %v", raw, err)
	}
	if !strings.Contains(string(raw), "insert_seconds") {
		t.Fatal("nonsecret fields lost")
	}
}

func TestManualBatchObjectSnapshotRedactsAndPreservesAcceptance(t *testing.T) {
	b := ComposeTaskBatch{ConfigSnapshot: json.RawMessage(`{"settings":[{"key":"a","params":{"guide_video_url":"https://secret.example/key","insert_seconds":2}}],"acceptance":[{"material_id":3,"code":"missing"}]}`)}
	raw, err := json.Marshal(b)
	if err != nil || strings.Contains(string(raw), "secret.example") || !strings.Contains(string(raw), "acceptance") || !strings.Contains(string(raw), "insert_seconds") {
		t.Fatalf("%s %v", raw, err)
	}
}

func TestSourceBatchRedactsParametersAndKeepsHistoricalLinks(t *testing.T) {
	b := ComposeTaskBatch{ConfigSnapshot: json.RawMessage(`{"source_output_id":91,"source_task_id":71,"settings":[{"key":"a","params":{"guide_video_url":"https://secret.example/key","insert_seconds":2}}],"acceptance":[]}`)}
	raw, err := json.Marshal(b)
	if err != nil || strings.Contains(string(raw), "secret.example") || !strings.Contains(string(raw), `"source_output_id":91`) || !strings.Contains(string(raw), `"source_task_id":71`) {
		t.Fatalf("source links lost or private address exposed: %s err=%v", raw, err)
	}
}
