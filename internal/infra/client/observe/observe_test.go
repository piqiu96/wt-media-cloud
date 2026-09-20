package observe_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	hertzclient "github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/client/observe"
	"github.com/wt-media/wt-media-cloud/internal/infra/logger"
	"github.com/wt-media/wt-media-cloud/internal/shared/requestctx"
)

func TestExternalLogsSuccessWithoutResponseBody(t *testing.T) {
	logDir := initializeLogger(t)
	middleware := observe.External("douyin")
	request := protocol.AcquireRequest()
	request.SetRequestURI("https://api.example.test/v1/items?apiKey=credential-should-not-log")
	called := false
	err := middleware(func(_ context.Context, _ *protocol.Request, response *protocol.Response) error {
		called = true
		response.SetStatusCode(200)
		response.SetBodyString(`{"secret":"should-not-log"}`)
		return nil
	})(requestctx.WithTraceID(context.Background(), "tr-observe"), request, protocol.AcquireResponse())
	if err != nil || !called {
		t.Fatalf("called=%v err=%v", called, err)
	}

	data := read(t, filepath.Join(logDir, "external.log"))
	for _, field := range []string{"name=douyin", "host=api.example.test", "path=/v1/items", "status=200", `"trace_id":"tr-observe"`} {
		if !strings.Contains(data, field) {
			t.Fatalf("external log missing %s: %q", field, data)
		}
	}
	if strings.Contains(data, "credential-should-not-log") || strings.Contains(data, "should-not-log") {
		t.Fatalf("external log leaked credential or response body: %q", data)
	}
	if strings.Contains(data, "apiKey=") {
		t.Fatalf("external log leaked query string: %q", data)
	}
}

func TestExternalErrorsGoToNormalAndWFLogs(t *testing.T) {
	logDir := initializeLogger(t)
	ctx := requestctx.WithRequestID(context.Background(), "rq-observe")
	err := observe.External("agent")(func(_ context.Context, _ *protocol.Request, _ *protocol.Response) error {
		return errors.New("transport failed")
	})(ctx, protocol.AcquireRequest(), protocol.AcquireResponse())
	if err == nil || !strings.Contains(err.Error(), "transport failed") {
		t.Fatalf("err=%v", err)
	}

	normal := read(t, filepath.Join(logDir, "external.log"))
	wf := read(t, filepath.Join(logDir, "external.log.wf"))
	for _, data := range []string{normal, wf} {
		if !strings.Contains(data, "name=agent") || !strings.Contains(data, "transport failed") || !strings.Contains(data, `"request_id":"rq-observe"`) {
			t.Fatalf("external error log = %q", data)
		}
	}
}

func initializeLogger(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	rotation := config.RotationConfig{MaxSize: 10, MaxAge: 30, MaxBackups: 10}
	cfg := config.LoggerConfigs{}
	for _, name := range []string{"app", "access", "job", "external", "audit", "panic"} {
		cfgValue := config.LoggerConfig{
			Path:     filepath.Join(dir, name+".log"),
			Level:    "info",
			Format:   "json",
			Rotation: rotation,
		}
		switch name {
		case "app":
			cfg.App = cfgValue
		case "access":
			cfg.Access = cfgValue
		case "job":
			cfg.Job = cfgValue
		case "external":
			cfg.External = cfgValue
		case "audit":
			cfg.Audit = cfgValue
		case "panic":
			cfg.Panic = cfgValue
		}
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

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

var _ hertzclient.Middleware = observe.External("douyin")
