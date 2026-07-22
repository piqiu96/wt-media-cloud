package identity

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestLoginUsesHttpOnlyCookieAndReplacementRejectsOldSession(t *testing.T) {
	service := NewService(NewMemoryStore())
	if _, err := service.BootstrapAdmin("admin", "a-long-initial-password"); err != nil {
		t.Fatalf("BootstrapAdmin() error = %v", err)
	}
	engine := server.New()
	RegisterRoutes(engine, service, RouteConfig{CookieSecure: false})

	first := performJSON(engine, "POST", "/api/v1/auth/login", `{"username":"admin","password":"a-long-initial-password"}`)
	if first.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("first login status = %d, body = %s", first.Result().StatusCode(), first.Result().Body())
	}
	firstCookie := string(first.Result().Header.Peek("Set-Cookie"))
	if !strings.Contains(firstCookie, SessionCookieName+"=") || !strings.Contains(firstCookie, "HttpOnly") || !strings.Contains(firstCookie, "SameSite=Lax") {
		t.Fatalf("first Set-Cookie = %q", firstCookie)
	}
	if strings.Contains(string(first.Result().Body()), "session-first") || strings.Contains(string(first.Result().Body()), "password") {
		t.Fatalf("login response leaked secret material: %s", first.Result().Body())
	}

	idempotent := ut.PerformRequest(
		engine.Engine,
		"POST",
		"/api/v1/auth/login",
		&ut.Body{Body: bytes.NewBufferString(`{"username":"admin","password":"a-long-initial-password"}`), Len: len(`{"username":"admin","password":"a-long-initial-password"}`)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
		ut.Header{Key: "Cookie", Value: firstCookie},
	)
	if idempotent.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("idempotent login status = %d, body = %s", idempotent.Result().StatusCode(), idempotent.Result().Body())
	}
	if setCookie := string(idempotent.Result().Header.Peek("Set-Cookie")); setCookie != "" {
		t.Fatalf("idempotent login unexpectedly replaced cookie: %q", setCookie)
	}
	stillMe := ut.PerformRequest(engine.Engine, "GET", "/api/v1/auth/me", nil, ut.Header{Key: "Cookie", Value: firstCookie})
	if stillMe.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("idempotent login invalidated session status = %d, body = %s", stillMe.Result().StatusCode(), stillMe.Result().Body())
	}

	second := performJSON(engine, "POST", "/api/v1/auth/login", `{"username":"admin","password":"a-long-initial-password"}`)
	secondCookie := string(second.Result().Header.Peek("Set-Cookie"))
	if secondCookie == "" || secondCookie == firstCookie {
		t.Fatalf("replacement Set-Cookie = %q", secondCookie)
	}

	oldMe := ut.PerformRequest(engine.Engine, "GET", "/api/v1/auth/me", nil, ut.Header{Key: "Cookie", Value: firstCookie})
	if oldMe.Result().StatusCode() != consts.StatusUnauthorized {
		t.Fatalf("old session status = %d, body = %s", oldMe.Result().StatusCode(), oldMe.Result().Body())
	}
	newMe := ut.PerformRequest(engine.Engine, "GET", "/api/v1/auth/me", nil, ut.Header{Key: "Cookie", Value: secondCookie})
	if newMe.Result().StatusCode() != consts.StatusOK {
		t.Fatalf("new session status = %d, body = %s", newMe.Result().StatusCode(), newMe.Result().Body())
	}
}

func TestAdminManagesTeamsAndUsersWithOneTimePassword(t *testing.T) {
	service := NewService(NewMemoryStore())
	if _, err := service.BootstrapAdmin("admin", "a-long-initial-password"); err != nil {
		t.Fatal(err)
	}
	engine := server.New()
	RegisterRoutes(engine, service, RouteConfig{CookieSecure: false})
	login := performJSON(engine, "POST", "/api/v1/auth/login", `{"username":"admin","password":"a-long-initial-password"}`)
	cookie := string(login.Result().Header.Peek("Set-Cookie"))

	createdTeam := performIdentityJSON(engine, "POST", "/api/v1/operation-teams", `{"name":"火影组"}`, cookie)
	if createdTeam.Result().StatusCode() != consts.StatusCreated {
		t.Fatalf("create team status=%d body=%s", createdTeam.Result().StatusCode(), createdTeam.Result().Body())
	}
	var teamEnvelope struct {
		Data OperationTeam `json:"data"`
	}
	if err := json.Unmarshal(createdTeam.Result().Body(), &teamEnvelope); err != nil {
		t.Fatal(err)
	}
	teamID := teamEnvelope.Data.ID

	createdUser := performIdentityJSON(engine, "POST", "/api/v1/users", fmt.Sprintf(`{"username":"operator","password":"a-long-operator-password","role":"operator","team_id":%d,"game_ids":["game-a"]}`, teamID), cookie)
	if createdUser.Result().StatusCode() != consts.StatusCreated || !strings.Contains(string(createdUser.Result().Body()), `"one_time_password":"a-long-operator-password"`) {
		t.Fatalf("create user status=%d body=%s", createdUser.Result().StatusCode(), createdUser.Result().Body())
	}
	var userEnvelope struct {
		Data struct {
			User            PublicUser `json:"user"`
			OneTimePassword string     `json:"one_time_password"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createdUser.Result().Body(), &userEnvelope); err != nil {
		t.Fatal(err)
	}
	if userEnvelope.Data.User.ID <= 0 || userEnvelope.Data.User.TeamName != "火影组" {
		t.Fatalf("created user=%+v", userEnvelope.Data.User)
	}

	listed := ut.PerformRequest(engine.Engine, "GET", fmt.Sprintf("/api/v1/users?uid=%d&role=operator&team_id=%d&status=enabled&game_id=game-a", userEnvelope.Data.User.ID, teamID), nil, ut.Header{Key: "Cookie", Value: cookie})
	if listed.Result().StatusCode() != consts.StatusOK || strings.Contains(string(listed.Result().Body()), "one_time_password") || strings.Contains(string(listed.Result().Body()), "a-long-operator-password") {
		t.Fatalf("list users status=%d body=%s", listed.Result().StatusCode(), listed.Result().Body())
	}
	updated := performIdentityJSON(engine, "PATCH", fmt.Sprintf("/api/v1/users/%d", userEnvelope.Data.User.ID), fmt.Sprintf(`{"role":"senior_operator","team_id":%d,"status":"disabled","game_ids":["game-b"]}`, teamID), cookie)
	if updated.Result().StatusCode() != consts.StatusOK || !strings.Contains(string(updated.Result().Body()), `"role":"senior_operator"`) || !strings.Contains(string(updated.Result().Body()), `"status":"disabled"`) {
		t.Fatalf("update user status=%d body=%s", updated.Result().StatusCode(), updated.Result().Body())
	}

	reset := performIdentityJSON(engine, "POST", fmt.Sprintf("/api/v1/users/%d/reset-password", userEnvelope.Data.User.ID), `{"new_password":"a-reset-operator-password"}`, cookie)
	if reset.Result().StatusCode() != consts.StatusOK || !strings.Contains(string(reset.Result().Body()), `"one_time_password":"a-reset-operator-password"`) {
		t.Fatalf("reset status=%d body=%s", reset.Result().StatusCode(), reset.Result().Body())
	}

	renamed := performIdentityJSON(engine, "PATCH", fmt.Sprintf("/api/v1/operation-teams/%d", teamID), `{"name":"火影高级组"}`, cookie)
	if renamed.Result().StatusCode() != consts.StatusOK || !strings.Contains(string(renamed.Result().Body()), "火影高级组") {
		t.Fatalf("rename status=%d body=%s", renamed.Result().StatusCode(), renamed.Result().Body())
	}
	blockedDelete := performIdentityJSON(engine, "DELETE", fmt.Sprintf("/api/v1/operation-teams/%d", teamID), `{}`, cookie)
	if blockedDelete.Result().StatusCode() != consts.StatusConflict {
		t.Fatalf("referenced team delete status=%d body=%s", blockedDelete.Result().StatusCode(), blockedDelete.Result().Body())
	}
}

func TestNonAdminCannotUseUserAndTeamAdministrationRoutes(t *testing.T) {
	service := NewService(NewMemoryStore())
	admin, _ := service.BootstrapAdmin("admin", "a-long-initial-password")
	team, _ := service.CreateTeam(admin.ID, "运营组")
	_, _ = service.CreateUser(admin.ID, CreateUserInput{Username: "operator", Password: "a-long-operator-password", Role: RoleOperator, TeamID: &team.ID, GameIDs: []string{"game-a"}})
	engine := server.New()
	RegisterRoutes(engine, service, RouteConfig{CookieSecure: false})
	login := performJSON(engine, "POST", "/api/v1/auth/login", `{"username":"operator","password":"a-long-operator-password"}`)
	cookie := string(login.Result().Header.Peek("Set-Cookie"))

	for _, path := range []string{"/api/v1/users", "/api/v1/operation-teams", "/api/v1/audit-logs"} {
		response := ut.PerformRequest(engine.Engine, "GET", path, nil, ut.Header{Key: "Cookie", Value: cookie})
		if response.Result().StatusCode() != consts.StatusForbidden {
			t.Fatalf("GET %s status=%d body=%s", path, response.Result().StatusCode(), response.Result().Body())
		}
	}
}

func TestUserAdministrationRejectsNonNumericUID(t *testing.T) {
	service := NewService(NewMemoryStore())
	_, _ = service.BootstrapAdmin("admin", "a-long-initial-password")
	engine := server.New()
	RegisterRoutes(engine, service, RouteConfig{CookieSecure: false})
	login := performJSON(engine, "POST", "/api/v1/auth/login", `{"username":"admin","password":"a-long-initial-password"}`)
	cookie := string(login.Result().Header.Peek("Set-Cookie"))
	response := performIdentityJSON(engine, "DELETE", "/api/v1/users/not-a-number", `{}`, cookie)
	if response.Result().StatusCode() != consts.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Result().StatusCode(), response.Result().Body())
	}
}

func performJSON(engine *server.Hertz, method, path, payload string) *ut.ResponseRecorder {
	return ut.PerformRequest(
		engine.Engine,
		method,
		path,
		&ut.Body{Body: bytes.NewBufferString(payload), Len: len(payload)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
}

func performIdentityJSON(engine *server.Hertz, method, path, payload, cookie string) *ut.ResponseRecorder {
	headers := []ut.Header{{Key: "Content-Type", Value: "application/json"}}
	if cookie != "" {
		headers = append(headers, ut.Header{Key: "Cookie", Value: cookie})
	}
	return ut.PerformRequest(engine.Engine, method, path, &ut.Body{Body: bytes.NewBufferString(payload), Len: len(payload)}, headers...)
}
