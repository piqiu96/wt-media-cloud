package middleware

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/logger"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
	"github.com/wt-media/wt-media-cloud/internal/shared/requestctx"
)

func TestRequestContextUsesIncomingRequestIDAndKeepsLogID(t *testing.T) {
	_ = initializeRequestLogger(t)
	engine := server.New()
	engine.Use(RequestContext())
	engine.GET("/check", func(ctx context.Context, c *hertzapp.RequestContext) {
		traceID, _ := c.Get("trace_id")
		requestID, _ := c.Get("request_id")
		if requestctx.TraceID(ctx) != traceID.(string) || requestctx.RequestID(ctx) != requestID.(string) {
			api.InternalError(c, "context id mismatch")
			return
		}
		api.Success(c, map[string]string{"trace_id": traceID.(string), "request_id": requestID.(string)})
	})

	result := ut.PerformRequest(engine.Engine, consts.MethodGet, "/check", nil, ut.Header{Key: "X-Request-ID", Value: "request-123"})
	if result.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("status = %d, body=%s", result.Result().StatusCode(), result.Result().Body())
	}
	var envelope struct {
		Data  map[string]string `json:"data"`
		LogID string            `json:"logid"`
	}
	if err := json.Unmarshal(result.Result().Body(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got, want := envelope.Data["request_id"], "request-123"; got != want {
		t.Fatalf("request id = %q, want %q", got, want)
	}
	if envelope.Data["trace_id"] == "" {
		t.Fatal("trace ID is empty")
	}
	if got, want := envelope.LogID, envelope.Data["trace_id"]; got != want {
		t.Fatalf("logid = %q, want trace id %q", got, want)
	}
}

func TestRequestLoggerContainsTraceRequestAndModule(t *testing.T) {
	dir := initializeRequestLogger(t)
	engine := server.New()
	engine.Use(RequestContext())
	engine.Use(func(ctx context.Context, c *hertzapp.RequestContext) {
		c.Set("user_id", int64(7))
		c.Next(requestctx.WithUserID(ctx, 7))
	})
	engine.GET("/check", func(ctx context.Context, c *hertzapp.RequestContext) {
		if requestctx.UserID(ctx) != 7 {
			api.InternalError(c, "context id mismatch")
			return
		}
		api.Success(c, map[string]string{"ok": "yes"})
	})

	result := ut.PerformRequest(engine.Engine, consts.MethodGet, "/check", nil, ut.Header{Key: "X-Request-ID", Value: "request-456"})
	if result.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("status = %d, body=%s", result.Result().StatusCode(), result.Result().Body())
	}

	data, err := os.ReadFile(filepath.Join(dir, "access.log"))
	if err != nil {
		t.Fatalf("read access log: %v", err)
	}
	output := string(data)
	for _, field := range []string{`"trace_id":"tr`, `"request_id":"request-456"`, "module=http", `"user_id":7`} {
		if !strings.Contains(output, field) {
			t.Fatalf("request log %q does not contain %s", output, field)
		}
	}
}

func initializeRequestLogger(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cfg := config.LoggerConfigs{
		App:      config.LoggerConfig{Path: filepath.Join(dir, "app.log"), Level: "info", Format: "json"},
		Access:   config.LoggerConfig{Path: filepath.Join(dir, "access.log"), Level: "info", Format: "json"},
		Job:      config.LoggerConfig{Path: filepath.Join(dir, "job.log"), Level: "info", Format: "json"},
		External: config.LoggerConfig{Path: filepath.Join(dir, "external.log"), Level: "info", Format: "json", Rotation: testRotation()},
		Audit:    config.LoggerConfig{Path: filepath.Join(dir, "audit.log"), Level: "info", Format: "json"},
		Panic:    config.LoggerConfig{Path: filepath.Join(dir, "panic.log"), Level: "info", Format: "json"},
	}
	if err := logger.Initialize(cfg); err != nil {
		t.Fatalf("logger.Initialize() error = %v", err)
	}
	t.Cleanup(func() {
		if err := logger.Close(); err != nil {
			t.Fatalf("logger.Close() error = %v", err)
		}
	})
	return dir
}

func testRotation() config.RotationConfig {
	return config.RotationConfig{MaxSize: 10, MaxAge: 30, MaxBackups: 10}
}
