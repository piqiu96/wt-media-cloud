package production

import (
	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"testing"
)

func TestTaskBatchListParserKeepsServerPaginationAndFilters(t *testing.T) {
	ctx := hertzapp.NewContext(0)
	ctx.Request.SetRequestURI("/api/v1/compose-task-batches?page=3&page_size=7&batch_id=8&compose_strategy_id=9&status=failed&trigger_type=manual&search=%E6%89%B9%E6%AC%A1&created_from=2026-11-01T00:00:00Z&created_before=2026-11-05T00:00:00Z")
	input, err := parseTaskListInput(ctx)
	if err != nil || input.Page != 3 || input.PageSize != 7 || input.BatchID != 8 || input.StrategyID != 9 || input.Status != "failed" || input.TriggerType != "manual" || input.Search != "批次" || input.CreatedFrom.IsZero() || input.CreatedBefore.IsZero() {
		t.Fatalf("input=%+v err=%v", input, err)
	}
	for _, query := range []string{"batch_id=0", "batch_id=-1", "compose_strategy_id=bad", "page=x", "page_size=x", "created_from=not-a-date"} {
		ctx.Request.SetRequestURI("/api/v1/compose-task-batches?" + query)
		if _, err = parseTaskListInput(ctx); err == nil {
			t.Fatalf("accepted malformed query %s", query)
		}
	}
}
func TestTaskBatchRoutesRequireAuthentication(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine)
	for _, request := range []struct{ method, path string }{{consts.MethodGet, "/api/v1/compose-task-batches"}, {consts.MethodGet, "/api/v1/compose-task-batches/8"}, {consts.MethodPost, "/api/v1/compose-tasks/batch-retry"}, {consts.MethodPost, "/api/v1/compose-tasks/batch-cancel"}} {
		response := ut.PerformRequest(engine.Engine, request.method, request.path, nil)
		if response.Result().StatusCode() != consts.StatusUnauthorized {
			t.Fatalf("%s %s status=%d", request.method, request.path, response.Result().StatusCode())
		}
	}
}
