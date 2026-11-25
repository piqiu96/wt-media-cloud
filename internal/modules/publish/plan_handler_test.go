package publish

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/service"
	"github.com/wt-media/wt-media-cloud/internal/shared/api"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestPlanRoutesRequireSession(t *testing.T) {
	h := server.New()
	RegisterRoutes(h)
	for _, c := range []struct{ method, path string }{{"GET", "/api/v1/publication-plan"}, {"POST", "/api/v1/publication-plan"}, {"GET", "/api/v1/publication-plan/1"}, {"PUT", "/api/v1/publication-plan/1/structure"}, {"POST", "/api/v1/publication-plan/validate"}, {"POST", "/api/v1/publication-plan/recommend"}, {"GET", "/api/v1/publication-plan/1/batch"}, {"GET", "/api/v1/publication-plan/1/record"}, {"GET", "/api/v1/publication-plan/1/copy"}} {
		if r := ut.PerformRequest(h.Engine, c.method, c.path, nil).Result(); r.StatusCode() != http.StatusUnauthorized {
			t.Fatalf("%s %d", c.path, r.StatusCode())
		}
	}
}
func TestPlanDecoderRejectsClientResourceFactsAndOwner(t *testing.T) {
	valid := `{"goals":[{"platform":"baidu","category_id":"1","target_count":1}],"batches":[],"instruction":"拆批"}`
	for _, extra := range []string{`,"owner_id":"99"`, `,"resource_counts":{"available":99}`, `,"plan_id":"8"`} {
		c := app.NewContext(0)
		c.Request.SetBodyString(strings.TrimSuffix(valid, "}") + extra + "}")
		var in service.PlanRecommendationInput
		if decodePlan(c, &in) == nil {
			t.Fatalf("accepted %s", extra)
		}
	}
	c := app.NewContext(0)
	c.Request.SetBodyString(valid)
	var in service.PlanRecommendationInput
	if e := decodePlan(c, &in); e != nil {
		t.Fatal(e)
	}
}

func TestPlanRecommendationUsesSharedTypedSSEAndRejectsFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		c := app.NewContext(0)
		api.StreamOneWithTimeout(c, api.StreamItem{UserID: 808}, time.Second, func(ctx context.Context, progress func(string) error) (any, error) {
			if e := progress("generating"); e != nil {
				return nil, e
			}
			if fail {
				return nil, errors.New("model incomplete")
			}
			return service.PlanRecommendationResult{Batches: []model.PlanStructureBatch{{RowKey: "r", Name: "first", Platform: "baidu", GroupOrder: 1, BatchOrder: 1, Targets: []model.BatchTarget{{CategoryID: 1, TargetCount: 1}}}}, ResourceAsOf: time.Now().UTC(), Issues: []service.PlanIssue{}}, nil
		})
		body := c.Response.BodyStream()
		data, e := io.ReadAll(body)
		body.(io.Closer).Close()
		if e != nil {
			t.Fatal(e)
		}
		text := string(data)
		if !strings.Contains(text, "event: start") || !strings.Contains(text, "event: progress") || !strings.Contains(text, "event: done") || !strings.Contains(text, `"input_revision"`) || !strings.Contains(text, `"logid"`) {
			t.Fatal(text)
		}
		if fail {
			if strings.Contains(text, "event: item_result") || !strings.Contains(text, "event: item_error") {
				t.Fatal(text)
			}
		} else if strings.Count(text, "event: item_result") != 1 || !strings.Contains(text, `"batches"`) {
			t.Fatal(text)
		}
	}
}

func TestPlanDuplicateOutputUsesActualHTTPConflictEnvelope(t *testing.T) {
	h := server.New()
	h.POST("/plan-conflict", func(_ context.Context, c *app.RequestContext) {
		respondBatch(c, nil, &repository.PlanConflictError{PlanID: 1, CurrentRevision: 3, Issues: []repository.PlanConflictIssue{{Code: "duplicate_plan_output", Message: "两个已纳入的行使用同平台成片", BatchID: 2, RowKey: "first", RelatedBatchID: 3, RelatedRowKey: "second"}}})
	})
	r := ut.PerformRequest(h.Engine, "POST", "/plan-conflict", nil).Result()
	if r.StatusCode() != 409 {
		t.Fatal(r.StatusCode())
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Details struct {
				PlanID          string `json:"plan_id"`
				CurrentRevision string `json:"current_revision"`
				Issues          []struct {
					BatchID        string `json:"batch_id"`
					RowKey         string `json:"row_key"`
					RelatedBatchID string `json:"related_batch_id"`
					RelatedRowKey  string `json:"related_row_key"`
				} `json:"issues"`
			} `json:"details"`
		} `json:"error"`
	}
	if e := json.Unmarshal(r.Body(), &body); e != nil {
		t.Fatal(e)
	}
	if body.Error.Code != "publication_conflict" || body.Error.Details.PlanID != "1" || body.Error.Details.CurrentRevision != "3" || len(body.Error.Details.Issues) != 1 {
		t.Fatalf("lost issue %s", r.Body())
	}
	issue := body.Error.Details.Issues[0]
	if issue.BatchID != "2" || issue.RowKey != "first" || issue.RelatedBatchID != "3" || issue.RelatedRowKey != "second" {
		t.Fatalf("wrong issue %s", r.Body())
	}
}
