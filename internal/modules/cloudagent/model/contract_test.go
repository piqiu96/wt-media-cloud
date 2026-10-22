package model

import (
	"encoding/json"
	"testing"
)

func TestCloudAgentContractJSONTagsRemainStable(t *testing.T) {
	task := Task{TaskID: "task-1", TaskType: "noop_task", Status: "pending", Payload: map[string]any{"ok": true}}
	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal task: %v", err)
	}
	want := `"task_id":"task-1","task_type":"noop_task","status":"pending","created_at":"","payload":{"ok":true}`
	if string(data) != "{"+want+"}" {
		t.Fatalf("task JSON = %s, want %s", data, "{"+want+"}")
	}
}
