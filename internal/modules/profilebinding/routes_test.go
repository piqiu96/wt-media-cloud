package profilebinding

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
	"github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding"
)

func TestProfileRoutesStageReviewAndConfirm(t *testing.T) {
	engine, cookie, store := newProfileRouteTest(t)
	created := performProfileJSON(engine, "POST", "/api/v1/bit-browser/profile-scans", `{"node_id":"node-trusted","main_user_id":"main-user-1","profiles":[{"bit_profile_id":"p1","main_user_id":"main-user-1","profile_user_id":"bit-user-1","name":"窗口一"}]}`, cookie)
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

func TestProfileRoutesConfirmMainIdentityDoesNotApplyProfiles(t *testing.T) {
	engine, cookie, store := newProfileRouteTest(t)
	created := performProfileJSON(engine, "POST", "/api/v1/bit-browser/profile-scans", `{"node_id":"node-trusted","main_user_id":"main-user-1","profiles":[{"bit_profile_id":"p1","main_user_id":"main-user-1","profile_user_id":"bit-user-1","name":"窗口一"}]}`, cookie)
	if created.Result().StatusCode() != consts.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Result().StatusCode(), created.Result().Body())
	}
	var envelope struct {
		Data ProfileScan `json:"data"`
	}
	if err := json.Unmarshal(created.Result().Body(), &envelope); err != nil {
		t.Fatal(err)
	}

	confirmed := performProfileJSON(engine, "POST", "/api/v1/bit-browser/profile-scans/"+envelope.Data.ID+"/confirm-main-identity", `{}`, cookie)
	if confirmed.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("confirm-main-identity status=%d body=%s", confirmed.Result().StatusCode(), confirmed.Result().Body())
	}
	if len(store.profiles) != 0 {
		t.Fatalf("identity-only route applied profiles: %v", store.profiles)
	}
	binding := store.bindings[identity.UserID(2)]
	if binding.MainUserID != "main-user-1" {
		t.Fatalf("binding=%#v", binding)
	}
}

func TestProfileRoutesConfirmMainIdentityDirectDoesNotApplyProfiles(t *testing.T) {
	engine, cookie, store := newProfileRouteTest(t)
	confirmed := performProfileJSON(engine, "POST", "/api/v1/bit-browser/main-identity", `{"main_user_id":"main-user-1"}`, cookie)
	if confirmed.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("main-identity status=%d body=%s", confirmed.Result().StatusCode(), confirmed.Result().Body())
	}
	if len(store.profiles) != 0 || len(store.scans) != 0 {
		t.Fatalf("direct identity route applied profiles=%v scans=%v", store.profiles, store.scans)
	}
	binding := store.bindings[identity.UserID(2)]
	if binding.MainUserID != "main-user-1" {
		t.Fatalf("binding=%#v", binding)
	}
}

func TestProfileRoutesRejectSilentMainIdentityRebind(t *testing.T) {
	engine, cookie, store := newProfileRouteTest(t)
	ok := performProfileJSON(engine, "POST", "/api/v1/bit-browser/main-identity", `{"main_user_id":"main-user-1"}`, cookie)
	if ok.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("first main-identity status=%d body=%s", ok.Result().StatusCode(), ok.Result().Body())
	}

	rebind := performProfileJSON(engine, "POST", "/api/v1/bit-browser/main-identity", `{"main_user_id":"main-user-2"}`, cookie)
	if rebind.Result().StatusCode() != consts.StatusConflict || !strings.Contains(string(rebind.Result().Body()), "当前比特浏览器登录账号与系统绑定账号不一致") {
		t.Fatalf("rebind status=%d body=%s", rebind.Result().StatusCode(), rebind.Result().Body())
	}
	if got := store.bindings[identity.UserID(2)].MainUserID; got != "main-user-1" {
		t.Fatalf("binding changed to %q", got)
	}
}

func TestProfileRoutesAdminCanClearMainIdentity(t *testing.T) {
	engine, operatorCookie, store := newProfileRouteTest(t)
	confirmed := performProfileJSON(engine, "POST", "/api/v1/bit-browser/main-identity", `{"main_user_id":"main-user-1"}`, operatorCookie)
	if confirmed.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("main-identity status=%d body=%s", confirmed.Result().StatusCode(), confirmed.Result().Body())
	}
	store.profiles["profile-1"] = BrowserProfile{ID: "profile-1", UserID: identity.UserID(2), BitProfileID: "bit-profile-1", MainUserID: "main-user-1"}

	adminLogin := performProfileJSON(engine, "POST", "/api/v1/auth/login", `{"username":"admin","password":"a-long-initial-password"}`, "")
	adminCookie := string(adminLogin.Result().Header.Peek("Set-Cookie"))
	cleared := performProfileJSON(engine, "DELETE", "/api/v1/users/2/bit-browser-main-identity", `{}`, adminCookie)
	if cleared.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("clear status=%d body=%s", cleared.Result().StatusCode(), cleared.Result().Body())
	}
	if _, ok := store.bindings[identity.UserID(2)]; ok {
		t.Fatal("binding was not cleared")
	}
	if _, ok := store.profiles["profile-1"]; !ok {
		t.Fatal("clearing main identity deleted profile")
	}
}

func TestProfileRoutesOperatorCannotClearMainIdentity(t *testing.T) {
	engine, cookie, _ := newProfileRouteTest(t)
	response := performProfileJSON(engine, "DELETE", "/api/v1/users/2/bit-browser-main-identity", `{}`, cookie)
	if response.Result().StatusCode() != consts.StatusForbidden {
		t.Fatalf("operator clear status=%d body=%s", response.Result().StatusCode(), response.Result().Body())
	}
}

func TestProfileRoutesRejectMixedIdentity(t *testing.T) {
	engine, cookie, _ := newProfileRouteTest(t)
	response := performProfileJSON(engine, "POST", "/api/v1/bit-browser/profile-scans", `{"node_id":"node-trusted","main_user_id":"main-user-1","profiles":[{"bit_profile_id":"p1","main_user_id":"main-user-2","profile_user_id":"bit-user-2"}]}`, cookie)
	if response.Result().StatusCode() != consts.StatusConflict || !strings.Contains(string(response.Result().Body()), `"errcode":23002`) {
		t.Fatalf("status=%d body=%s", response.Result().StatusCode(), response.Result().Body())
	}
}

func TestProfileRoutesRequireTrustedNodeForScanSnapshot(t *testing.T) {
	engine, cookie, store, _, trust := newProfileRouteTestWithRuntime(t)
	missingNode := performProfileJSON(engine, "POST", "/api/v1/bit-browser/profile-scans", `{"main_user_id":"main-user-1","profiles":[{"bit_profile_id":"p1","main_user_id":"main-user-1","profile_user_id":"bit-user-1"}]}`, cookie)
	if missingNode.Result().StatusCode() != consts.StatusBadRequest || len(store.scans) != 0 {
		t.Fatalf("missing node status=%d body=%s scans=%d", missingNode.Result().StatusCode(), missingNode.Result().Body(), len(store.scans))
	}

	trust.err = runtimebinding.ErrLocalTrustUnavailable
	untrusted := performProfileJSON(engine, "POST", "/api/v1/bit-browser/profile-scans", `{"node_id":"node-untrusted","main_user_id":"main-user-1","profiles":[{"bit_profile_id":"p1","main_user_id":"main-user-1","profile_user_id":"bit-user-1"}]}`, cookie)
	if untrusted.Result().StatusCode() != consts.StatusConflict || len(store.scans) != 0 {
		t.Fatalf("untrusted status=%d body=%s scans=%d", untrusted.Result().StatusCode(), untrusted.Result().Body(), len(store.scans))
	}
}

func TestProfileRoutesBlockLocalSensitiveTaskWhenNodeIsUntrusted(t *testing.T) {
	engine, cookie, store, tasks, trust := newProfileRouteTestWithRuntime(t)
	store.profiles["profile-1"] = BrowserProfile{ID: "profile-1", UserID: identity.UserID(2), TeamID: actorTeamID(t, engine, cookie), BitProfileID: "bit-profile-1", MainUserID: "main-user-1", ProfileUserID: "bit-user-1", LocalStatus: ProfileActive, LastSyncedAt: time.Now().UTC()}
	trust.err = runtimebinding.ErrLocalTrustUnavailable

	blocked := performProfileJSON(engine, "POST", "/api/v1/browser-profiles/profile-1/open", `{"node_id":"node-untrusted"}`, cookie)
	if blocked.Result().StatusCode() != consts.StatusConflict || tasks.created != 0 {
		t.Fatalf("blocked status=%d body=%s tasks=%d", blocked.Result().StatusCode(), blocked.Result().Body(), tasks.created)
	}

	trust.err = nil
	allowed := performProfileJSON(engine, "POST", "/api/v1/browser-profiles/profile-1/open", `{"node_id":"node-trusted"}`, cookie)
	if allowed.Result().StatusCode() != consts.StatusCreated || tasks.created != 1 {
		t.Fatalf("allowed status=%d body=%s tasks=%d", allowed.Result().StatusCode(), allowed.Result().Body(), tasks.created)
	}
	if trust.lastNodeID != "node-trusted" {
		t.Fatalf("trust node = %q", trust.lastNodeID)
	}
}

func newProfileRouteTest(t *testing.T) (*server.Hertz, string, *memoryStore) {
	t.Helper()
	engine, cookie, store, _, _ := newProfileRouteTestWithRuntime(t)
	return engine, cookie, store
}

func newProfileRouteTestWithRuntime(t *testing.T) (*server.Hertz, string, *memoryStore, *fakeTaskCreator, *fakeTrustChecker) {
	t.Helper()
	identityStore := identity.NewMemoryStore()
	identityService := identity.NewService(identityStore)
	admin, _ := identityService.BootstrapAdmin("admin", "a-long-initial-password")
	team, err := identityService.CreateTeam(admin.ID, "运营一组")
	if err != nil {
		t.Fatal(err)
	}
	_, err = identityService.CreateUser(admin.ID, identity.CreateUserInput{Username: "operator", Password: "a-long-operator-password", Role: identity.RoleOperator, TeamID: &team.ID, GameIDs: []string{"game-a"}})
	if err != nil {
		t.Fatal(err)
	}
	engine := server.New()
	identity.RegisterRoutes(engine, identityService, identity.RouteConfig{CookieSecure: false})
	store := newMemoryStore()
	tasks := &fakeTaskCreator{}
	trust := &fakeTrustChecker{}
	RegisterRoutes(engine, NewService(store), identityService, tasks, trust)
	login := performProfileJSON(engine, "POST", "/api/v1/auth/login", `{"username":"operator","password":"a-long-operator-password"}`, "")
	return engine, string(login.Result().Header.Peek("Set-Cookie")), store, tasks, trust
}

func performProfileJSON(engine *server.Hertz, method, path, payload, cookie string) *ut.ResponseRecorder {
	headers := []ut.Header{{Key: "Content-Type", Value: "application/json"}}
	if cookie != "" {
		headers = append(headers, ut.Header{Key: "Cookie", Value: cookie})
	}
	return ut.PerformRequest(engine.Engine, method, path, &ut.Body{Body: bytes.NewBufferString(payload), Len: len(payload)}, headers...)
}

type fakeTaskCreator struct {
	created int
	last    cloudagent.CreateTaskRequest
}

func (t *fakeTaskCreator) Create(req cloudagent.CreateTaskRequest) cloudagent.Task {
	t.created++
	t.last = req
	return cloudagent.Task{TaskID: "task-1", TaskType: req.TaskType, Status: cloudagent.TaskStatusPending.String(), Payload: req.Payload}
}

type fakeTrustChecker struct {
	err        error
	lastUserID identity.UserID
	lastNodeID string
}

func (c *fakeTrustChecker) CheckLocalTrust(userID identity.UserID, nodeID string) error {
	c.lastUserID = userID
	c.lastNodeID = nodeID
	if c.err != nil {
		return c.err
	}
	if strings.TrimSpace(nodeID) == "" {
		return runtimebinding.ErrInvalidInput
	}
	return nil
}

func actorTeamID(t *testing.T, engine *server.Hertz, cookie string) *identity.TeamID {
	t.Helper()
	me := performProfileJSON(engine, "GET", "/api/v1/auth/me", `{}`, cookie)
	if me.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("me status=%d body=%s", me.Result().StatusCode(), me.Result().Body())
	}
	var envelope struct {
		Data identity.PublicUser `json:"data"`
	}
	if err := json.Unmarshal(me.Result().Body(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.TeamID == nil {
		t.Fatal("operator team id missing")
	}
	return envelope.Data.TeamID
}
