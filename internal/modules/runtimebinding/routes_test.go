package runtimebinding

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
)

func TestRuntimeRoutesBindCurrentSessionAndRequireNodeCredential(t *testing.T) {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	identityService := identity.NewService(identity.NewMemoryStore(), identity.WithTokenGenerator(func() string { return "session-token" }))
	if _, err := identityService.BootstrapTechnician("tech", "a-long-initial-password"); err != nil {
		t.Fatal(err)
	}
	login, err := identityService.Login("tech", "a-long-initial-password")
	if err != nil {
		t.Fatal(err)
	}
	store := newMemoryStore()
	service := testService(store, &now)
	engine := server.Default()
	RegisterRoutes(engine, service, identityService)

	unauthenticated := ut.PerformRequest(engine.Engine, "POST", "/api/v1/local-agent/binding-tickets", nil)
	if unauthenticated.Result().StatusCode() != consts.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d", unauthenticated.Result().StatusCode())
	}

	cookie := identity.SessionCookieName + "=" + login.Token
	ticketResponse := ut.PerformRequest(engine.Engine, "POST", "/api/v1/local-agent/binding-tickets", nil, ut.Header{Key: "Cookie", Value: cookie})
	if ticketResponse.Result().StatusCode() != consts.StatusCreated {
		t.Fatalf("ticket status=%d body=%s", ticketResponse.Result().StatusCode(), ticketResponse.Result().Body())
	}
	var ticketEnvelope struct {
		Data BindingTicketGrant `json:"data"`
	}
	if err := json.Unmarshal(ticketResponse.Result().Body(), &ticketEnvelope); err != nil {
		t.Fatal(err)
	}
	for _, ticket := range store.tickets {
		store.activeSessions[ticket.SessionID+":"+ticket.UserID] = true
	}

	registerBody, _ := json.Marshal(RegisterLocalInput{
		BindingToken: ticketEnvelope.Data.BindingToken, AgentID: "agent-1", DeviceID: "device-1", AgentVersion: "0.2.0",
		ContractMajorVersion: cloudagent.MajorVersion, ContractRevision: cloudagent.ContractRevision,
	})
	registerResponse := ut.PerformRequest(engine.Engine, "POST", "/api/v1/local-agent/nodes/register", &ut.Body{Body: bytes.NewReader(registerBody), Len: len(registerBody)}, ut.Header{Key: "Content-Type", Value: "application/json"})
	if registerResponse.Result().StatusCode() != consts.StatusCreated {
		t.Fatalf("register status=%d body=%s", registerResponse.Result().StatusCode(), registerResponse.Result().Body())
	}
	var registrationEnvelope struct {
		Data Registration `json:"data"`
	}
	if err := json.Unmarshal(registerResponse.Result().Body(), &registrationEnvelope); err != nil {
		t.Fatal(err)
	}

	reportBody, _ := json.Marshal(validRuntimeReport())
	wrong := ut.PerformRequest(engine.Engine, "POST", "/api/v1/local-agent/nodes/"+registrationEnvelope.Data.Node.ID+"/runtime-report", &ut.Body{Body: bytes.NewReader(reportBody), Len: len(reportBody)}, ut.Header{Key: "Content-Type", Value: "application/json"}, ut.Header{Key: "Authorization", Value: "Bearer wrong"})
	if wrong.Result().StatusCode() != consts.StatusUnauthorized {
		t.Fatalf("wrong credential status=%d body=%s", wrong.Result().StatusCode(), wrong.Result().Body())
	}

	store.validProfiles[login.User.ID+":profile-1"] = true
	store.validProfiles[login.User.ID+":profile-2"] = true
	reportResponse := ut.PerformRequest(engine.Engine, "POST", "/api/v1/local-agent/nodes/"+registrationEnvelope.Data.Node.ID+"/runtime-report", &ut.Body{Body: bytes.NewReader(reportBody), Len: len(reportBody)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
		ut.Header{Key: "Authorization", Value: "Bearer " + registrationEnvelope.Data.NodeCredential},
	)
	if reportResponse.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("report status=%d body=%s", reportResponse.Result().StatusCode(), reportResponse.Result().Body())
	}
	if got := string(reportResponse.Result().Header.ContentType()); got == "" {
		t.Fatal("runtime report response has no content type")
	}
}
