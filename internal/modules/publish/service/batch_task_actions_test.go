package service

import (
	"encoding/json"
	"testing"

	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
)

func TestBatchFailedTaskJSONRetainsExplicitEmptyActions(t *testing.T) {
	team := identity.TeamID(7)
	actor := identity.PublicUser{ID: 9, TeamID: &team, Role: identity.RoleOperator, Status: identity.UserStatusEnabled}
	for _, status := range []string{model.BatchPaused, model.BatchEnded, model.BatchRunning, "standalone"} {
		t.Run(status, func(t *testing.T) {
			task := model.Task{ID: 12, Status: model.StatusFailed}
			if status != "standalone" {
				id := uint64(4)
				task.BatchID = &id
				decorateBatchTask(actor, model.Batch{ID: id, TeamID: 7, OperatorID: 9, ExecutionStatus: status}, &task)
			}
			raw, err := json.Marshal(task)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(raw, &fields); err != nil {
				t.Fatal(err)
			}
			actions, present := fields["allowed_actions"]
			switch status {
			case "standalone":
				if present {
					t.Fatalf("standalone legacy action fallback changed: %s", raw)
				}
			case model.BatchRunning:
				if string(actions) != `["retry"]` {
					t.Fatalf("running batch lost permitted retry: %s", raw)
				}
			default:
				if !present || string(actions) != `[]` {
					t.Fatalf("blocked batch must send explicit array, not missing/null actions: %s", raw)
				}
			}
		})
	}
}
