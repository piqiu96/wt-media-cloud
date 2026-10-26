package model

import (
	"encoding/json"
	"testing"
)

// attempt、external_side_effect、retryable 即使为默认值也必须出现，
// Agent 才能按同一份结构判断重试与副作用。
func TestCloudAgentContractJSONTagsRemainStable(t *testing.T) {
	task := Task{TaskID: "task-1", TaskType: "noop_task", Status: "pending", Payload: map[string]any{"ok": true}}
	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal task: %v", err)
	}
	want := `"task_id":"task-1","task_type":"noop_task","status":"pending","attempt":0,"created_at":"","external_side_effect":false,"retryable":false,"payload":{"ok":true}`
	if string(data) != "{"+want+"}" {
		t.Fatalf("task JSON = %s, want %s", data, "{"+want+"}")
	}
}
