package image

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/wt-media/wt-media-cloud/internal/modules/image/service"
	"gopkg.in/yaml.v3"
)

func TestImageContractRoutesRequireSession(t *testing.T) {
	h := server.New()
	RegisterRoutes(h)
	_, file, _, _ := runtime.Caller(0)
	raw, e := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../contracts/cloud-api/v1/image.openapi.yaml"))
	if e != nil {
		t.Fatal(e)
	}
	var doc struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if e = yaml.Unmarshal(raw, &doc); e != nil {
		t.Fatal(e)
	}
	for path, methods := range doc.Paths {
		for method := range methods {
			if method != "get" && method != "put" && method != "post" {
				continue
			}
			response := ut.PerformRequest(h.Engine, strings.ToUpper(method), strings.ReplaceAll(path, "{id}", "1"), nil)
			if response.Result().StatusCode() != 401 {
				t.Fatalf("%s %s did not require session: %d", method, path, response.Result().StatusCode())
			}
		}
	}
}
func TestImageInputRejectsUnknownFieldsTrailingJSONAndNumericIDs(t *testing.T) {
	for _, raw := range []string{`{"file_id":9007199254740993}`, `{"file_id":"7","owner_id":"99"}`, `{} {}`, `{"category_ids":[1]}`} {
		c := app.NewContext(0)
		c.Request.SetBodyString(raw)
		var in service.RegisterInput
		if e := decodeInput(c, &in); e == nil {
			t.Fatalf("unsafe input accepted: %s", raw)
		}
	}
	c := app.NewContext(0)
	c.Request.SetBodyString(`{"file_id":"9007199254740993","category_ids":["9007199254740995"]}`)
	var in service.RegisterInput
	if e := decodeInput(c, &in); e != nil || in.FileID != 9007199254740993 {
		t.Fatalf("ID precision lost %+v %v", in, e)
	}
	b, e := json.Marshal(in)
	if e != nil || !strings.Contains(string(b), `"file_id":"9007199254740993"`) {
		t.Fatalf("ID serialization %s %v", b, e)
	}
}

func TestImageFailureIncludesMachineCodeAndRecovery(t *testing.T) {
	for _, tc := range []struct {
		err            error
		status         int
		code, recovery string
		retryable      bool
	}{
		{service.ErrConflict, 409, "image_conflict", "reload", false},
		{errors.New("storage unavailable"), 503, "image_unavailable", "retry", true},
	} {
		h := server.New()
		h.GET("/probe", func(_ context.Context, c *app.RequestContext) { fail(c, tc.err) })
		r := ut.PerformRequest(h.Engine, "GET", "/probe", nil)
		var body struct {
			Error struct {
				Code      string `json:"code"`
				Recovery  string `json:"recovery_action"`
				Retryable bool   `json:"retryable"`
			} `json:"error"`
		}
		if e := json.Unmarshal(r.Result().Body(), &body); e != nil {
			t.Fatal(e)
		}
		if r.Result().StatusCode() != tc.status || body.Error.Code != tc.code || body.Error.Recovery != tc.recovery || body.Error.Retryable != tc.retryable {
			t.Fatalf("failure recovery lost: %s", r.Result().Body())
		}
	}
}
