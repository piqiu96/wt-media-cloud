package publish

import (
	"net/http"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/service"
)

func TestBatchRoutesRequireSession(t *testing.T) {
	h := server.New()
	RegisterRoutes(h)
	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/v1/publication-batch"},
		{"POST", "/api/v1/publication-batch"},
		{"GET", "/api/v1/publication-batch/1"},
		{"GET", "/api/v1/publication-batch/1/copy"},
		{"GET", "/api/v1/publication-batch/1/record"},
		{"PUT", "/api/v1/publication-batch/1/configuration"},
		{"POST", "/api/v1/publication-batch/allocate"},
		{"POST", "/api/v1/publication-batch/validate"},
		{"GET", "/api/v1/publication/account"},
		{"GET", "/api/v1/publication/resource"},
	} {
		if response := ut.PerformRequest(h.Engine, tc.method, tc.path, nil).Result(); response.StatusCode() != http.StatusUnauthorized {
			t.Errorf("%s %s = %d, want 401", tc.method, tc.path, response.StatusCode())
		}
	}
}

func TestBatchConfigurationPresenceDistinguishesExplicitZero(t *testing.T) {
	const valid = `{"name":"batch","expected_revision":"1","platform":"baidu","targets":[{"category_id":"1","target_count":0}],"config":{"candidates":[{"row_key":"r","input_revision":"1","field_revisions":{},"category_id":"1","media_account_id":"1","composite_output_id":"4","order":0,"content":{}}]},"execution_setting":{"concurrency_limit":1,"group_interval_seconds":0,"between_group_policy":"auto_continue","exception_task_limit":1}}`
	for _, field := range []string{"target_count", "group_interval_seconds", "order", "content"} {
		t.Run(field, func(t *testing.T) {
			target := `,"` + field + `":0`
			if field == "content" {
				target = `,"content":{}`
			}
			for _, bad := range []string{"", `,"` + field + `":null`} {
				var in service.EditBatchInput
				ctx := app.NewContext(0)
				ctx.Request.SetBodyString(strings.Replace(valid, target, bad, 1))
				if err := decodeBatch(ctx, &in); err == nil {
					t.Errorf("accepted %s replaced with %q", field, bad)
				}
			}
		})
	}
	var in service.EditBatchInput
	ctx := app.NewContext(0)
	ctx.Request.SetBodyString(valid)
	if err := decodeBatch(ctx, &in); err != nil {
		t.Fatalf("explicit zero/object rejected: %v", err)
	}
}

func TestBatchValidationContextRequiresBothNonzeroValues(t *testing.T) {
	const config = `"platform":"baidu","targets":[{"category_id":"1","target_count":0}],"config":{"candidates":[]},"execution_setting":{"concurrency_limit":1,"group_interval_seconds":0,"between_group_policy":"auto_continue","exception_task_limit":1}`
	for _, tc := range []struct {
		fields string
		valid  bool
	}{
		{"", true},
		{`,"batch_id":"1","expected_revision":"3"`, true},
		{`,"batch_id":"1"`, false},
		{`,"batch_id":null,"expected_revision":"3"`, false},
		{`,"batch_id":"0","expected_revision":"0"`, false},
	} {
		var request service.ValidateBatchInput
		ctx := app.NewContext(0)
		ctx.Request.SetBodyString("{" + config + tc.fields + "}")
		if got := decodeBatch(ctx, &request) == nil; got != tc.valid {
			t.Errorf("context %s accepted = %t, want %t", tc.fields, got, tc.valid)
		}
	}
}
