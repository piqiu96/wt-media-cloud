package mediaaccount

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

func TestRoutesRequireAuthentication(t *testing.T) {
	engine, _, _ := newMediaAccountRouteTest(t)
	response := ut.PerformRequest(engine.Engine, "GET", "/api/v1/media-accounts", nil)
	if response.Result().StatusCode() != consts.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", response.Result().StatusCode(), response.Result().Body())
	}
}

func TestRoutesCreateListAndNeverReturnCookie(t *testing.T) {
	engine, cookie, _ := newMediaAccountRouteTest(t)
	created := performMediaJSON(engine, "POST", "/api/v1/media-accounts", `{"game_id":"game-a","platform":"douyin","original_cookie":"top-secret-cookie"}`, cookie)
	if created.Result().StatusCode() != consts.StatusCreated {
		t.Fatalf("create status = %d, body = %s", created.Result().StatusCode(), created.Result().Body())
	}
	if strings.Contains(string(created.Result().Body()), "top-secret-cookie") || strings.Contains(string(created.Result().Body()), "original_cookie") {
		t.Fatalf("create response leaked cookie: %s", created.Result().Body())
	}

	listed := ut.PerformRequest(engine.Engine, "GET", "/api/v1/media-accounts?platform=douyin", nil, ut.Header{Key: "Cookie", Value: cookie})
	if listed.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("list status = %d, body = %s", listed.Result().StatusCode(), listed.Result().Body())
	}
	if !strings.Contains(string(listed.Result().Body()), `"platform":"douyin"`) || strings.Contains(string(listed.Result().Body()), "cookie") {
		t.Fatalf("list response = %s", listed.Result().Body())
	}
}

func TestRoutesEnforceGameScope(t *testing.T) {
	engine, cookie, _ := newMediaAccountRouteTest(t)
	response := performMediaJSON(engine, "POST", "/api/v1/media-accounts", `{"game_id":"game-b","platform":"douyin"}`, cookie)
	if response.Result().StatusCode() != consts.StatusForbidden {
		t.Fatalf("status = %d, body = %s", response.Result().StatusCode(), response.Result().Body())
	}
}

func TestRoutesRejectOutOfScopeGameList(t *testing.T) {
	engine, cookie, _ := newMediaAccountRouteTest(t)
	response := ut.PerformRequest(engine.Engine, "GET", "/api/v1/media-accounts?game_id=game-b", nil, ut.Header{Key: "Cookie", Value: cookie})
	if response.Result().StatusCode() != consts.StatusForbidden {
		t.Fatalf("status = %d, body = %s", response.Result().StatusCode(), response.Result().Body())
	}
}

func TestRoutesIdentifyDuplicateAndManageTags(t *testing.T) {
	engine, cookie, _ := newMediaAccountRouteTest(t)
	firstID := createRouteAccount(t, engine, cookie, "douyin")
	secondID := createRouteAccount(t, engine, cookie, "douyin")

	identified := performMediaJSON(engine, "POST", "/api/v1/media-accounts/"+firstID+"/identify", `{"platform_account_id":"platform-42","name":"main","login_status":"normal"}`, cookie)
	if identified.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("identify status = %d, body = %s", identified.Result().StatusCode(), identified.Result().Body())
	}
	duplicate := performMediaJSON(engine, "POST", "/api/v1/media-accounts/"+secondID+"/identify", `{"platform_account_id":"platform-42","name":"duplicate","login_status":"normal"}`, cookie)
	if duplicate.Result().StatusCode() != consts.StatusConflict || !strings.Contains(string(duplicate.Result().Body()), `"errcode":20009`) {
		t.Fatalf("duplicate status = %d, body = %s", duplicate.Result().StatusCode(), duplicate.Result().Body())
	}

	tagged := performMediaJSON(engine, "POST", "/api/v1/media-accounts/tags/add", `{"account_ids":["`+firstID+`"],"tags":["launch","vip"]}`, cookie)
	if tagged.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("tag status = %d, body = %s", tagged.Result().StatusCode(), tagged.Result().Body())
	}
	filtered := ut.PerformRequest(engine.Engine, "GET", "/api/v1/media-accounts?all_tags=launch,vip", nil, ut.Header{Key: "Cookie", Value: cookie})
	if filtered.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("filtered status = %d, body = %s", filtered.Result().StatusCode(), filtered.Result().Body())
	}
	var list struct {
		Data []Account `json:"data"`
	}
	if err := json.Unmarshal(filtered.Result().Body(), &list); err != nil {
		t.Fatalf("parse filtered = %v, body = %s", err, filtered.Result().Body())
	}
	foundFirst, foundSecond := false, false
	for _, account := range list.Data {
		if account.ID == firstID {
			foundFirst = true
		}
		if account.ID == secondID {
			foundSecond = true
		}
	}
	if !foundFirst || foundSecond {
		t.Fatalf("filtered ids = %#v, want contain %s and not %s", list.Data, firstID, secondID)
	}
}

func TestRoutesUpdateStatuses(t *testing.T) {
	engine, cookie, _ := newMediaAccountRouteTest(t)
	accountID := createRouteAccount(t, engine, cookie, "bilibili")
	response := performMediaJSON(engine, "PATCH", "/api/v1/media-accounts/"+accountID, `{"business_status":"disabled","login_status":"verification_needed"}`, cookie)
	if response.Result().StatusCode() != consts.StatusOK || !strings.Contains(string(response.Result().Body()), `"business_status":"disabled"`) {
		t.Fatalf("status = %d, body = %s", response.Result().StatusCode(), response.Result().Body())
	}
}

func TestRoutesBindValidatedProfile(t *testing.T) {
	engine, cookie, _ := newMediaAccountRouteTest(t)
	accountID := createRouteAccount(t, engine, cookie, "douyin")
	response := performMediaJSON(engine, "PATCH", "/api/v1/media-accounts/"+accountID+"/profile", `{"browser_profile_id":"profile-1"}`, cookie)
	if response.Result().StatusCode() != consts.StatusOK || !strings.Contains(string(response.Result().Body()), `"browser_profile_id":"profile-1"`) {
		t.Fatalf("status = %d, body = %s", response.Result().StatusCode(), response.Result().Body())
	}
}

func TestRoutesCreateAndUpdateAccountName(t *testing.T) {
	engine, cookie, _ := newMediaAccountRouteTest(t)
	created := performMediaJSON(engine, "POST", "/api/v1/media-accounts", `{"game_id":"game-a","platform":"bilibili","name":"  测试昵称  "}`, cookie)
	if created.Result().StatusCode() != consts.StatusCreated || !strings.Contains(string(created.Result().Body()), `"name":"测试昵称"`) {
		t.Fatalf("create status = %d, body = %s", created.Result().StatusCode(), created.Result().Body())
	}
	var envelope struct {
		Data Account `json:"data"`
	}
	if err := json.Unmarshal(created.Result().Body(), &envelope); err != nil {
		t.Fatal(err)
	}
	updated := performMediaJSON(engine, "PATCH", "/api/v1/media-accounts/"+envelope.Data.ID, `{"name":"新名"}`, cookie)
	if updated.Result().StatusCode() != consts.StatusOK || !strings.Contains(string(updated.Result().Body()), `"name":"新名"`) {
		t.Fatalf("update status = %d, body = %s", updated.Result().StatusCode(), updated.Result().Body())
	}
}

func newMediaAccountRouteTest(t *testing.T) (*server.Hertz, string, *memoryStore) {
	t.Helper()
	identityStore := identity.NewMemoryStore()
	identityService := identity.NewService(identityStore)
	admin, err := identityService.BootstrapAdmin("admin", "a-long-initial-password")
	if err != nil {
		t.Fatal(err)
	}
	team, err := identityService.CreateTeam(admin.ID, "运营一组")
	if err != nil {
		t.Fatal(err)
	}
	operator, err := identityService.CreateUser(admin.ID, identity.CreateUserInput{
		Username: "operator", Password: "a-long-operator-password", Role: identity.RoleOperator, TeamID: &team.ID, GameIDs: []string{"game-a"},
	})
	if err != nil {
		t.Fatal(err)
	}
	engine := server.New()
	identity.RegisterRoutes(engine, identityService, identity.RouteConfig{CookieSecure: false})
	store := newMemoryStore()
	resolver := &fakeProfileResolver{profiles: map[string]identity.UserID{"profile-1": operator.ID}, inactive: map[string]bool{}}
	RegisterRoutes(engine, NewService(store, WithProfileResolver(resolver), WithUserResolver(identityService)), identityService)
	login := performMediaJSON(engine, "POST", "/api/v1/auth/login", `{"username":"operator","password":"a-long-operator-password"}`, "")
	if login.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("login status = %d, body = %s", login.Result().StatusCode(), login.Result().Body())
	}
	return engine, string(login.Result().Header.Peek("Set-Cookie")), store
}

func createRouteAccount(t *testing.T, engine *server.Hertz, cookie, platform string) string {
	t.Helper()
	response := performMediaJSON(engine, "POST", "/api/v1/media-accounts", `{"game_id":"game-a","platform":"`+platform+`"}`, cookie)
	if response.Result().StatusCode() != consts.StatusCreated {
		t.Fatalf("create status = %d, body = %s", response.Result().StatusCode(), response.Result().Body())
	}
	var envelope struct {
		Data Account `json:"data"`
	}
	if err := json.Unmarshal(response.Result().Body(), &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data.ID
}

func performMediaJSON(engine *server.Hertz, method, path, payload, cookie string) *ut.ResponseRecorder {
	headers := []ut.Header{{Key: "Content-Type", Value: "application/json"}}
	if cookie != "" {
		headers = append(headers, ut.Header{Key: "Cookie", Value: cookie})
	}
	return ut.PerformRequest(
		engine.Engine, method, path,
		&ut.Body{Body: bytes.NewBufferString(payload), Len: len(payload)},
		headers...,
	)
}
