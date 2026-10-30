package proxy

import (
	"encoding/json"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	proxymodel "github.com/wt-media/wt-media-cloud/internal/modules/proxy/model"
	proxyservice "github.com/wt-media/wt-media-cloud/internal/modules/proxy/service"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

func TestProxyViewUsesNumericIDOnTheWire(t *testing.T) {
	encoded, err := json.Marshal(proxyView{Proxy: proxymodel.Proxy{ID: "42"}, ID: sharedidentity.ProxyID("42")})
	if err != nil {
		t.Fatal(err)
	}
	var view map[string]any
	if err := json.Unmarshal(encoded, &view); err != nil {
		t.Fatal(err)
	}
	if view["id"] != float64(42) {
		t.Fatalf("proxy view id = %#v", view["id"])
	}
}

func TestRegisterRoutesBindsProxyEndpoints(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine)
	response := ut.PerformRequest(engine.Engine, consts.MethodGet, "/api/v1/proxies", nil)
	if response.Result().StatusCode() != consts.StatusUnauthorized {
		t.Fatalf("unauthenticated proxy list status = %d, body = %s", response.Result().StatusCode(), response.Result().Body())
	}
}

func TestSavedExtractionURLRequiresSameTeamSeniorOperator(t *testing.T) {
	team := sharedidentity.TeamID(7)
	other := sharedidentity.TeamID(8)
	proxy := proxyservice.Proxy{TeamID: team}
	for _, tc := range []struct {
		role  identityservice.Role
		team  *sharedidentity.TeamID
		allow bool
	}{
		{identityservice.RoleSeniorOperator, &team, true},
		{identityservice.RoleOperator, &team, false},
		{identityservice.RoleAdmin, &team, false},
		{identityservice.RoleSeniorOperator, &other, false},
	} {
		if got := canExtractProxy(identityservice.PublicUser{Role: tc.role, TeamID: tc.team}, proxy); got != tc.allow {
			t.Fatalf("role=%s team=%v allowed=%v, want %v", tc.role, tc.team, got, tc.allow)
		}
	}
}

func TestDynamicExtractionOnlyHasTheNewCloudRoutes(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine)
	config := ut.PerformRequest(engine.Engine, consts.MethodGet, "/api/v1/proxies/p1/extraction-config", nil)
	if config.Result().StatusCode() != consts.StatusUnauthorized {
		t.Fatalf("extraction config status = %d", config.Result().StatusCode())
	}
	old := ut.PerformRequest(engine.Engine, consts.MethodPost, "/api/v1/proxies/extract-preview", nil)
	if old.Result().StatusCode() != consts.StatusNotFound {
		t.Fatalf("old Cloud extraction route status = %d", old.Result().StatusCode())
	}
}

func TestRegisterRoutesBindsProfileProxyOperation(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine)
	response := ut.PerformRequest(engine.Engine, consts.MethodPost, "/api/v1/browser-profiles/1/proxy-operation", nil)
	if response.Result().StatusCode() != consts.StatusUnauthorized {
		t.Fatalf("unauthenticated profile proxy operation status = %d, body = %s", response.Result().StatusCode(), response.Result().Body())
	}
}
