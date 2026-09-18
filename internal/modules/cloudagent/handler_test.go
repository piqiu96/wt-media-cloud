package cloudagent

import (
	"encoding/json"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestRegisterRoutesBindsCompatibilityEndpoint(t *testing.T) {
	engine := server.New()
	RegisterRoutes(engine)
	response := ut.PerformRequest(engine.Engine, consts.MethodGet, "/api/v1/cloud-agent/compatibility", nil)
	if response.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Result().StatusCode(), response.Result().Body())
	}
	var envelope struct {
		Data struct {
			API          string `json:"api"`
			MajorVersion string `json:"major_version"`
			Status       string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Result().Body(), &envelope); err != nil {
		t.Fatalf("decode compatibility: %v", err)
	}
	if envelope.Data.API != "cloud-agent" || envelope.Data.MajorVersion != "v1" || envelope.Data.Status != "compatible" {
		t.Fatalf("compatibility = %#v", envelope.Data)
	}
}
