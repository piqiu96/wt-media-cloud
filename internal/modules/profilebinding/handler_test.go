package profilebinding

import (
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestMaskBitMainUserIDKeepsOnlyTheEdges(t *testing.T) {
	cases := map[string]string{
		"":                     "****",
		"short":                "****",
		"12345678":             "****",
		"123456789":            "1234****6789",
		"main-user-abcdef1234": "main****1234",
	}
	for input, want := range cases {
		if got := maskBitMainUserID(input); got != want {
			t.Fatalf("maskBitMainUserID(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestRegisterRoutesBindsProfileEndpoints(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine)

	cases := []struct {
		method string
		path   string
		want   int
	}{
		{consts.MethodGet, "/api/v1/browser-profiles", consts.StatusUnauthorized},
		{consts.MethodGet, "/api/v1/local-agent/bit-account", consts.StatusUnauthorized},
		{consts.MethodPost, "/api/v1/local-agent/bit-account/bind", consts.StatusUnauthorized},
		{consts.MethodPost, "/api/v1/bit-browser/main-identity", consts.StatusNotFound},
		{consts.MethodPost, "/api/v1/bit-browser/profile-scans/scan-1/confirm-main-identity", consts.StatusNotFound},
	}
	for _, tc := range cases {
		response := ut.PerformRequest(engine.Engine, tc.method, tc.path, nil)
		if got := response.Result().StatusCode(); got != tc.want {
			t.Fatalf("%s %s status = %d, want %d, body = %s", tc.method, tc.path, got, tc.want, response.Result().Body())
		}
	}
}
