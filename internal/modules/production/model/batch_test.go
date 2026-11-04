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
