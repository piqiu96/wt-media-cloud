package profilebinding

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
)

func TestProfileRoutesStageReviewAndConfirm(t *testing.T) {
	engine, cookie, store := newProfileRouteTest(t)
	created := performProfileJSON(engine, "POST", "/api/v1/bit-browser/profile-scans", `{"owner_user_id":"bit-user-1","profiles":[{"bit_profile_id":"p1","owner_user_id":"bit-user-1","name":"窗口一"}]}`, cookie)
	if created.Result().StatusCode() != consts.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Result().StatusCode(), created.Result().Body())
	}
	if len(store.profiles) != 0 {
		t.Fatal("route changed formal profiles before confirmation")
	}
	var envelope struct {
		Data ProfileScan `json:"data"`
	}
	if err := json.Unmarshal(created.Result().Body(), &envelope); err != nil {
		t.Fatal(err)
	}
	review := ut.PerformRequest(engine.Engine, "GET", "/api/v1/bit-browser/profile-scans/"+envelope.Data.ID, nil, ut.Header{Key: "Cookie", Value: cookie})
	if review.Result().StatusCode() != consts.StatusOK || !strings.Contains(string(review.Result().Body()), `"kind":"added"`) {
		t.Fatalf("review status=%d body=%s", review.Result().StatusCode(), review.Result().Body())
	}
	confirmed := performProfileJSON(engine, "POST", "/api/v1/bit-browser/profile-scans/"+envelope.Data.ID+"/confirm", `{}`, cookie)
	if confirmed.Result().StatusCode() != consts.StatusOK || len(store.profiles) != 1 {
		t.Fatalf("confirm status=%d body=%s profiles=%v", confirmed.Result().StatusCode(), confirmed.Result().Body(), store.profiles)
	}
}

func TestProfileRoutesRejectMixedIdentity(t *testing.T) {
	engine, cookie, _ := newProfileRouteTest(t)
	response := performProfileJSON(engine, "POST", "/api/v1/bit-browser/profile-scans", `{"owner_user_id":"bit-user-1","profiles":[{"bit_profile_id":"p1","owner_user_id":"bit-user-2"}]}`, cookie)
	if response.Result().StatusCode() != consts.StatusConflict || !strings.Contains(string(response.Result().Body()), "bitbrowser_identity_unverifiable") {
		t.Fatalf("status=%d body=%s", response.Result().StatusCode(), response.Result().Body())
	}
}

func newProfileRouteTest(t *testing.T) (*server.Hertz, string, *memoryStore) {
	t.Helper()
	identityStore := identity.NewMemoryStore()
	identityService := identity.NewService(identityStore)
	tech, _ := identityService.BootstrapTechnician("tech", "a-long-initial-password")
	_, err := identityService.CreateUser(tech.ID, identity.CreateUserInput{Username: "operator", Password: "a-long-operator-password", Role: identity.RoleOperator, GameIDs: []string{"game-a"}})
	if err != nil {
		t.Fatal(err)
	}
	engine := server.New()
	identity.RegisterRoutes(engine, identityService, identity.RouteConfig{CookieSecure: false})
	store := newMemoryStore()
	RegisterRoutes(engine, NewService(store), identityService)
	login := performProfileJSON(engine, "POST", "/api/v1/auth/login", `{"username":"operator","password":"a-long-operator-password"}`, "")
	return engine, string(login.Result().Header.Peek("Set-Cookie")), store
}

func performProfileJSON(engine *server.Hertz, method, path, payload, cookie string) *ut.ResponseRecorder {
	headers := []ut.Header{{Key: "Content-Type", Value: "application/json"}}
	if cookie != "" {
		headers = append(headers, ut.Header{Key: "Cookie", Value: cookie})
	}
	return ut.PerformRequest(engine.Engine, method, path, &ut.Body{Body: bytes.NewBufferString(payload), Len: len(payload)}, headers...)
}
