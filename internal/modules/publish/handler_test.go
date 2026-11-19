package publish

import (
	"context"
	"encoding/json"
	app "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/service"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPublicationRoutesAndContractRequireSession(t *testing.T) {
	h := server.New()
	RegisterRoutes(h)
	_, file, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../contracts/cloud-api/v1/publication.openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err = yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	count := 0
	for path, methods := range doc.Paths {
		for method := range methods {
			if method != "get" && method != "post" && method != "put" {
				continue
			}
			count++
			path = strings.ReplaceAll(path, "{id}", "1")
			r := ut.PerformRequest(h.Engine, strings.ToUpper(method), path, nil)
			if r.Result().StatusCode() != 401 {
				t.Fatalf("%s %s got %d", method, path, r.Result().StatusCode())
			}
		}
	}
	if count != 6 {
		t.Fatalf("route/contract count=%d", count)
	}
	for _, action := range []string{"execute", "authorize", "retry"} {
		if got := ut.PerformRequest(h.Engine, "POST", "/api/v1/publication/1/"+action, nil).Result().StatusCode(); got != 404 {
			t.Fatalf("premature execution endpoint %s=%d", action, got)
		}
	}
}
func TestStrictPublicationInput(t *testing.T) {
	for _, raw := range []string{
		`{"request_key":"r","category_id":9007199254740993}`,
		`{"request_key":"r","category_id":"1","operator_id":"99"}`,
		`{"content":{"description":"a","cover_file_id":"1","execute":true}}`,
		`{} {}`,
	} {
		c := app.NewContext(0)
		c.Request.SetBodyString(raw)
		var in service.CreateInput
		if decode(c, &in) == nil {
			t.Fatalf("unsafe input accepted: %s", raw)
		}
	}
	c := app.NewContext(0)
	c.Request.SetBodyString(`{"category_id":"9007199254740993","media_account_id":"9007199254740994","composite_output_id":"9007199254740995"}`)
	var in service.CreateInput
	if err := decode(c, &in); err != nil || in.CompositeOutputID != 9007199254740995 {
		t.Fatalf("large IDs lost %v %+v", err, in)
	}
}
func TestRevisionConflictIncludesCurrentVersion(t *testing.T) {
	h := server.New()
	h.PUT("/probe", func(_ context.Context, c *app.RequestContext) {
		fail(c, &service.ConflictError{TaskID: 9007199254740993, Revision: 7})
	})
	r := ut.PerformRequest(h.Engine, "PUT", "/probe", nil)
	var body struct {
		Errcode int `json:"errcode"`
		Error   struct {
			Details struct {
				TaskID   string `json:"task_id"`
				Revision string `json:"current_revision"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(r.Result().Body(), &body); err != nil {
		t.Fatal(err)
	}
	if r.Result().StatusCode() != 409 || body.Errcode != 18003 || body.Error.Details.TaskID != "9007199254740993" || body.Error.Details.Revision != "7" {
		t.Fatalf("conflict response %s", r.Result().Body())
	}
}

func TestPublicationAgentRoutesRequireBearer(t *testing.T) {
	h := server.New()
	RegisterRoutes(h)
	for _, path := range []string{"claim", "1/heartbeat", "1/progress", "1/complete"} {
		r := ut.PerformRequest(h.Engine, "POST", "/api/v1/cloud-agent/publication-tasks/"+path, nil)
		if r.Result().StatusCode() != 401 {
			t.Fatalf("%s: %d", path, r.Result().StatusCode())
		}
	}
}

func TestPublicationResourceRouteRequiresSession(t *testing.T) {
	h := server.New()
	RegisterRoutes(h)
	for _, method := range []string{"GET", "POST"} {
		if got := ut.PerformRequest(h.Engine, method, "/api/v1/publication", nil).Result().StatusCode(); got != 401 {
			t.Fatalf("%s publication status=%d", method, got)
		}
	}
}
