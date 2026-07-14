package profileguard

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding"
)

func TestProfileGuardRoutesPreflightAndFinish(t *testing.T) {
	now := time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC)
	store := &fakeStore{task: authorizedTask(), taskFound: true}
	service := NewService(
		store,
		fakeNodeAuth{node: runtimebinding.AgentNode{ID: "node-1", UserID: "user-1", Mode: "local", Status: "online"}},
		WithClock(func() time.Time { return now }),
		WithIDGenerator(func(prefix string) string { return "permit-1" }),
		WithSecretGenerator(func() string { return "permit-secret" }),
	)
	engine := server.New()
	RegisterRoutes(engine, service)

	preflight := guardJSON(engine, "/api/v1/local-agent/sensitive-tasks/task-1/preflight", `{"node_id":"node-1"}`, "node-secret", "")
	if preflight.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("preflight status=%d body=%s", preflight.Result().StatusCode(), preflight.Result().Body())
	}
	var envelope struct {
		Data PreflightOutcome `json:"data"`
	}
	if err := json.Unmarshal(preflight.Result().Body(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Outcome != OutcomeGranted || envelope.Data.PermitCredential != "permit-secret" {
		t.Fatalf("preflight data=%+v", envelope.Data)
	}

	finish := guardJSON(engine, "/api/v1/local-agent/sensitive-permits/permit-1/finish", `{"node_id":"node-1","outcome":"completed"}`, "node-secret", "permit-secret")
	if finish.Result().StatusCode() != consts.StatusOK || store.finishOutcome != FinishCompleted {
		t.Fatalf("finish status=%d body=%s outcome=%s", finish.Result().StatusCode(), finish.Result().Body(), store.finishOutcome)
	}
}

func TestProfileGuardRoutesRejectWrongNodeCredential(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine, NewService(&fakeStore{task: authorizedTask(), taskFound: true}, fakeNodeAuth{node: runtimebinding.AgentNode{ID: "node-1", UserID: "user-1"}}))
	response := guardJSON(engine, "/api/v1/local-agent/sensitive-tasks/task-1/preflight", `{"node_id":"node-1"}`, "wrong", "")
	if response.Result().StatusCode() != consts.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", response.Result().StatusCode(), response.Result().Body())
	}
}

func guardJSON(engine *server.Hertz, path, payload, nodeCredential, permitCredential string) *ut.ResponseRecorder {
	headers := []ut.Header{{Key: "Content-Type", Value: "application/json"}, {Key: "Authorization", Value: "Bearer " + nodeCredential}}
	if permitCredential != "" {
		headers = append(headers, ut.Header{Key: "X-Profile-Permit", Value: permitCredential})
	}
	return ut.PerformRequest(engine.Engine, "POST", path, &ut.Body{Body: bytes.NewBufferString(payload), Len: len(payload)}, headers...)
}
