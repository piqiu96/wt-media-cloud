package api

import (
	"context"
	"encoding/json"
	"testing"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestSuccessPreservesUnifiedEnvelope(t *testing.T) {
	engine := server.New()
	engine.GET("/ok", func(_ context.Context, c *hertzapp.RequestContext) {
		c.Set("logid", "trace-1")
		Success(c, map[string]string{"status": "ok"})
	})
	result := ut.PerformRequest(engine.Engine, "GET", "/ok", nil)
	var body struct {
		ErrCode int               `json:"errcode"`
		Message string            `json:"message"`
		Data    map[string]string `json:"data"`
		LogID   string            `json:"logid"`
	}
	if err := json.Unmarshal(result.Result().Body(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ErrCode != 0 || body.Message != "success" || body.Data["status"] != "ok" || body.LogID != "trace-1" {
		t.Fatalf("response = %+v", body)
	}
}

// The transfer API answers a created-but-not-yet-started download with 202, and
// the frontend distinguishes "queued" from "finished" on that status alone. 200
// would be readable as a completed download.
func TestAcceptedSignalsAQueuedTransfer(t *testing.T) {
	engine := server.New()
	engine.POST("/downloads", func(_ context.Context, c *hertzapp.RequestContext) {
		Accepted(c, map[string]string{"id": "transfer-1", "status": "pending"})
	})
	result := ut.PerformRequest(engine.Engine, "POST", "/downloads", nil)

	if got := result.Result().StatusCode(); got != consts.StatusAccepted {
		t.Fatalf("status = %d, want 202", got)
	}
	var body struct {
		ErrCode int               `json:"errcode"`
		Data    map[string]string `json:"data"`
	}
	if err := json.Unmarshal(result.Result().Body(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ErrCode != 0 || body.Data["status"] != "pending" {
		t.Fatalf("response = %+v", body)
	}
}

// `DELETE /material-usages/{id}` is the one frozen contract in this repo that
// says 204, and the reason `NoContent` cannot serve it: `NoContent` answers 200
// with a `data:null` envelope, which the frontend's `parseResponse` unwraps into
// a value. A 204 must carry no body at all, and that is the difference this
// pins — both halves, because a helper that answered 204 *with* a body would
// satisfy the status assertion alone.
func TestNoContentEmptyWritesAnActual204(t *testing.T) {
	engine := server.New()
	engine.DELETE("/thing", func(_ context.Context, c *hertzapp.RequestContext) {
		NoContentEmpty(c)
	})
	result := ut.PerformRequest(engine.Engine, "DELETE", "/thing", nil)

	if got := result.Result().StatusCode(); got != consts.StatusNoContent {
		t.Fatalf("status = %d, want 204", got)
	}
	if body := result.Result().Body(); len(body) != 0 {
		t.Fatalf("a 204 must carry no body, got %q", body)
	}
}

// The two 409s this change introduces are indistinguishable by errcode alone to
// a caller holding only the frozen name list, so the name travels in
// `error.type`. Pinning it here keeps the naming from drifting away from
// contracts/cloud-error-codes/.
func TestUnprocessableEntityCarriesTheFrozenErrorName(t *testing.T) {
	engine := server.New()
	engine.POST("/complete", func(_ context.Context, c *hertzapp.RequestContext) {
		UnprocessableEntity(c, 15106, "传输完整性校验失败", "transfer_integrity_failed")
	})
	result := ut.PerformRequest(engine.Engine, "POST", "/complete", nil)

	if got := result.Result().StatusCode(); got != consts.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", got)
	}
	var body struct {
		ErrCode int `json:"errcode"`
		Error   *struct {
			Type string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(result.Result().Body(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ErrCode != 15106 {
		t.Fatalf("errcode = %d, want 15106", body.ErrCode)
	}
	if body.Error == nil || body.Error.Type != "transfer_integrity_failed" {
		t.Fatalf("error = %+v, want type transfer_integrity_failed", body.Error)
	}
}

// A 409 whose type is absent is not the same answer as one whose type is set:
// the frontend branches on the type to tell "the source is not ready yet" from
// "there is no Local Agent to send this to", and those need different copy.
func TestConflictNamedDistinguishesTheTwoConflicts(t *testing.T) {
	engine := server.New()
	engine.POST("/conflict", func(_ context.Context, c *hertzapp.RequestContext) {
		ConflictNamed(c, 15105, "没有可用的本机下载节点", "local_transfer_node_unavailable")
	})
	result := ut.PerformRequest(engine.Engine, "POST", "/conflict", nil)

	if got := result.Result().StatusCode(); got != consts.StatusConflict {
		t.Fatalf("status = %d, want 409", got)
	}
	var body struct {
		ErrCode int `json:"errcode"`
		Error   *struct {
			Type string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(result.Result().Body(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ErrCode != 15105 || body.Error == nil || body.Error.Type != "local_transfer_node_unavailable" {
		t.Fatalf("response = %+v", body)
	}
}
