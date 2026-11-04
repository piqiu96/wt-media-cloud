package production

import (
	"context"
	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	productionservice "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
	"strings"
	"testing"
)

func TestComposeOutputArchiveRouteIsRetired(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine)
	response := ut.PerformRequest(engine.Engine, "POST", "/api/v1/compose-outputs/5/archive", nil)
	if response.Result().StatusCode() != 404 {
		t.Fatalf("archive route remains callable: %d", response.Result().StatusCode())
	}
}
func TestComposeOutputAvailabilityErrorsHavePublicNames(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		err    error
	}{{"compose_output_unavailable", 409, productionservice.ErrComposeOutputUnavailable}, {"compose_output_verification_failed", 503, productionservice.ErrComposeOutputVerificationFailed}} {
		t.Run(tc.name, func(t *testing.T) {
			engine := server.New()
			engine.GET("/probe", func(_ context.Context, c *hertzapp.RequestContext) { writeComposeOutputError(c, tc.err) })
			response := ut.PerformRequest(engine.Engine, "GET", "/probe", nil)
			if response.Result().StatusCode() != tc.status || !strings.Contains(string(response.Result().Body()), tc.name) {
				t.Fatalf("status=%d body=%s", response.Result().StatusCode(), response.Result().Body())
			}
		})
	}
}
