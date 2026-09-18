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

	agent := AgentNode{AgentID: "agent-1", Mode: "local", Status: "online", Capabilities: []string{"proxy"}}
	data, err = json.Marshal(agent)
	if err != nil {
		t.Fatalf("marshal agent: %v", err)
	}
	for _, field := range []string{`"agent_id":"agent-1"`, `"contract_major_version"`, `"contract_revision"`, `"last_heartbeat_at"`, `"capabilities":["proxy"]`} {
		if !contains(string(data), field) {
			t.Fatalf("agent JSON = %s, want field %s", data, field)
		}
	}
}

func contains(value, part string) bool {
	for i := 0; i+len(part) <= len(value); i++ {
		if value[i:i+len(part)] == part {
			return true
		}
	}
	return false
}
