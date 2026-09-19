package api

import (
	"context"
	"encoding/json"
	"testing"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
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
