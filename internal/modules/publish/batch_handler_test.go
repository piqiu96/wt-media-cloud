package publish

import (
	"net/http"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
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
