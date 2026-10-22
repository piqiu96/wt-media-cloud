package service

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// newTokenSequence returns a token generator that hands out unique tokens, so
// each login in one test gets its own session.
func newTokenSequence(prefix string) func() string {
	i := 0
	return func() string {
		token := fmt.Sprintf("%s-%d", prefix, i)
		i++
		return token
	}
}

func newLoginService() *Service {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	return NewService(
		NewMemoryStore(),
		WithClock(func() time.Time { return now }),
	)
}

func mustCreateOperator(t *testing.T, service *Service, adminID UserID, username string) PublicUser {
	t.Helper()
	team := mustCreateTeam(t, service, adminID, "登录会话组")
	user, err := service.CreateUser(adminID, CreateUserInput{
		Username: username,
		Password: "a-long-operator-password",
		Role:     RoleOperator,
		TeamID:   &team.ID,
		GameIDs:  []string{"game-a"},
	})
	if err != nil {
		t.Fatalf("CreateUser(%s) error = %v", username, err)
	}
	return user
}

func TestWebLoginsCoexistWithoutReplacement(t *testing.T) {
	service := newLoginService()
	if _, err := service.BootstrapAdmin("admin", "a-long-initial-password"); err != nil {
		t.Fatal(err)
	}

	first, err := service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{ReplaceExisting: true})
	if err != nil {
		t.Fatalf("first web login: %v", err)
	}
	second, err := service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{})
	if err != nil {
		t.Fatalf("second web login: %v", err)
	}
	if first.Token == second.Token {
		t.Fatalf("web logins reused token %q", first.Token)
	}
	for name, token := range map[string]string{"first": first.Token, "second": second.Token} {
		if _, err := service.Authenticate(token); err != nil {
			t.Fatalf("%s web session invalid: %v", name, err)
		}
	}
}

func TestDesktopAndWebSessionsCoexist(t *testing.T) {
	service := newLoginService()
	if _, err := service.BootstrapAdmin("admin", "a-long-initial-password"); err != nil {
		t.Fatal(err)
	}

	desktop, err := service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{ClientType: ClientTypeDesktop})
	if err != nil {
		t.Fatalf("desktop login: %v", err)
	}
	web, err := service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{})
	if err != nil {
		t.Fatalf("web login after desktop: %v", err)
	}

	desktopCtx, err := service.AuthenticateContext(desktop.Token)
	if err != nil {
		t.Fatalf("desktop session invalid: %v", err)
	}
	if desktopCtx.Session.ClientType != ClientTypeDesktop || desktopCtx.Session.ExpiresAt != desktop.ExpiresAt {
		t.Fatalf("desktop session = %+v", desktopCtx.Session)
	}
	if _, err := service.Authenticate(web.Token); err != nil {
		t.Fatalf("web session invalid: %v", err)
	}
}

func TestDesktopLoginConflictCancelAndConfirm(t *testing.T) {
	service := newLoginService()
	if _, err := service.BootstrapAdmin("admin", "a-long-initial-password"); err != nil {
		t.Fatal(err)
	}

	first, err := service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{ClientType: ClientTypeDesktop})
	if err != nil {
		t.Fatalf("first desktop login: %v", err)
	}
	web, err := service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{ClientType: ClientTypeDesktop})
	var conflict *DesktopLoginConflictError
	if !errors.As(err, &conflict) || conflict.ActiveSessionID != first.SessionID {
		t.Fatalf("unconfirmed desktop login error = %v, want conflict with %q", err, first.SessionID)
	}
	if _, err := service.Authenticate(first.Token); err != nil {
		t.Fatalf("cancelled conflict invalidated the active desktop session: %v", err)
	}

	_, err = service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{
		ClientType:       ClientTypeDesktop,
		ReplaceExisting:  true,
		ReplaceSessionID: "stale-session",
	})
	if !errors.As(err, &conflict) || conflict.ActiveSessionID != first.SessionID {
		t.Fatalf("stale confirmation error = %v, want conflict with %q", err, first.SessionID)
	}
	if _, err := service.Authenticate(first.Token); err != nil {
		t.Fatalf("stale confirmation invalidated the active desktop session: %v", err)
	}

	second, err := service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{
		ClientType:       ClientTypeDesktop,
		ReplaceExisting:  true,
		ReplaceSessionID: first.SessionID,
	})
	if err != nil {
		t.Fatalf("confirmed desktop replacement: %v", err)
	}
	if _, err := service.Authenticate(first.Token); !errors.Is(err, ErrDesktopReplaced) {
		t.Fatalf("old desktop token error = %v, want ErrDesktopReplaced", err)
	}
	if _, err := service.Authenticate(second.Token); err != nil {
		t.Fatalf("new desktop session invalid: %v", err)
	}
	if _, err := service.Authenticate(web.Token); err != nil {
		t.Fatalf("desktop replacement killed the web session: %v", err)
	}
}

func TestReplaceSessionIDRequiresReplaceExisting(t *testing.T) {
	service := newLoginService()
	if _, err := service.BootstrapAdmin("admin", "a-long-initial-password"); err != nil {
		t.Fatal(err)
	}
	first, err := service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{ClientType: ClientTypeDesktop})
	if err != nil {
		t.Fatal(err)
	}

	// Knowing the active session id is not permission to replace it: the
	// explicit confirmation flag is what authorizes the takeover.
	_, err = service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{
		ClientType:       ClientTypeDesktop,
		ReplaceSessionID: first.SessionID,
	})
	var conflict *DesktopLoginConflictError
	if !errors.As(err, &conflict) || conflict.ActiveSessionID != first.SessionID {
		t.Fatalf("replace_session_id without replace_existing error = %v, want conflict", err)
	}
	if _, err := service.Authenticate(first.Token); err != nil {
		t.Fatalf("unconfirmed replacement invalidated the active desktop session: %v", err)
	}

	_, err = service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{
		ClientType:      ClientTypeDesktop,
		ReplaceExisting: true,
	})
	if !errors.As(err, &conflict) {
		t.Fatalf("replace_existing without session id error = %v, want conflict", err)
	}
	if _, err := service.Authenticate(first.Token); err != nil {
		t.Fatalf("id-less replacement invalidated the active desktop session: %v", err)
	}
}

func TestConcurrentDesktopConfirmationsHaveOneWinner(t *testing.T) {
	service := NewService(NewMemoryStore(), WithTokenGenerator(newTokenSequence("desktop")))
	if _, err := service.BootstrapAdmin("admin", "a-long-initial-password"); err != nil {
		t.Fatal(err)
	}
	first, err := service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{ClientType: ClientTypeDesktop})
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	results := make([]error, 2)
	for i := range results {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, results[index] = service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{
				ClientType:       ClientTypeDesktop,
				ReplaceExisting:  true,
				ReplaceSessionID: first.SessionID,
			})
		}(i)
	}
	wg.Wait()

	winners, conflicts := 0, 0
	for _, err := range results {
		switch {
		case err == nil:
			winners++
		case errors.Is(err, ErrDesktopLoginConflict):
			conflicts++
		default:
			t.Fatalf("concurrent replacement error = %v", err)
		}
	}
	if winners != 1 || conflicts != 1 {
		t.Fatalf("concurrent replacement winners=%d conflicts=%d, want 1/1", winners, conflicts)
	}
}

func TestSessionExpiryIsDistinctFromInvalidation(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	service := NewService(NewMemoryStore(), WithClock(func() time.Time { return now }))
	if _, err := service.BootstrapAdmin("admin", "a-long-initial-password"); err != nil {
		t.Fatal(err)
	}
	login, err := service.Login("admin", "a-long-initial-password")
	if err != nil {
		t.Fatal(err)
	}
	if login.ExpiresAt != now.Add(SessionTTL) {
		t.Fatalf("expires_at = %v, want %v", login.ExpiresAt, now.Add(SessionTTL))
	}

	now = now.Add(SessionTTL)
	if _, err := service.Authenticate(login.Token); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("authenticate at expiry error = %v, want ErrSessionExpired", err)
	}

	// An expired Desktop row no longer counts as the active one, so a fresh
	// login does not need a replacement confirmation.
	desktop, err := service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{ClientType: ClientTypeDesktop})
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(SessionTTL)
	if _, err := service.Authenticate(desktop.Token); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("desktop authenticate at expiry error = %v, want ErrSessionExpired", err)
	}
	if _, err := service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{ClientType: ClientTypeDesktop}); err != nil {
		t.Fatalf("fresh desktop login after expiry: %v", err)
	}
}

func TestPasswordChangeInvalidatesEverySessionWithReason(t *testing.T) {
	service := newLoginService()
	if _, err := service.BootstrapAdmin("admin", "a-long-initial-password"); err != nil {
		t.Fatal(err)
	}
	desktop, err := service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{ClientType: ClientTypeDesktop})
	if err != nil {
		t.Fatal(err)
	}
	web, err := service.Login("admin", "a-long-initial-password")
	if err != nil {
		t.Fatal(err)
	}

	if err := service.ChangeOwnPassword(desktop.User.ID, "a-long-initial-password", "a-new-initial-password"); err != nil {
		t.Fatalf("ChangeOwnPassword() error = %v", err)
	}
	for name, token := range map[string]string{"desktop": desktop.Token, "web": web.Token} {
		if _, err := service.Authenticate(token); !errors.Is(err, ErrSessionInvalidated) {
			t.Fatalf("%s token error = %v, want ErrSessionInvalidated", name, err)
		}
		session, found, err := service.store.FindSessionByTokenHash(tokenHash(token))
		if err != nil || !found {
			t.Fatalf("%s session lookup = found %v, err %v", name, found, err)
		}
		if session.InvalidatedReason != SessionInvalidReasonPasswordChanged {
			t.Fatalf("%s invalidated_reason = %q", name, session.InvalidatedReason)
		}
	}
}

func TestDisableAndReEnableNeverRevivesSessions(t *testing.T) {
	service := newLoginService()
	admin, err := service.BootstrapAdmin("admin", "a-long-initial-password")
	if err != nil {
		t.Fatal(err)
	}
	operator := mustCreateOperator(t, service, admin.ID, "operator-disable")
	login, err := service.Login("operator-disable", "a-long-operator-password")
	if err != nil {
		t.Fatal(err)
	}

	if err := service.SetUserStatus(admin.ID, operator.ID, UserStatusDisabled); err != nil {
		t.Fatalf("SetUserStatus(disabled) error = %v", err)
	}
	if _, err := service.Authenticate(login.Token); !errors.Is(err, ErrSessionInvalidated) {
		t.Fatalf("disabled token error = %v, want ErrSessionInvalidated", err)
	}
	session, _, err := service.store.FindSessionByTokenHash(tokenHash(login.Token))
	if err != nil {
		t.Fatal(err)
	}
	if session.InvalidatedReason != SessionInvalidReasonUserDisabled {
		t.Fatalf("invalidated_reason = %q", session.InvalidatedReason)
	}

	if err := service.SetUserStatus(admin.ID, operator.ID, UserStatusEnabled); err != nil {
		t.Fatalf("SetUserStatus(enabled) error = %v", err)
	}
	if _, err := service.Authenticate(login.Token); !errors.Is(err, ErrSessionInvalidated) {
		t.Fatalf("re-enable revived old token: %v", err)
	}
}

func TestForceLogoutInvalidatesAllSessionsForAdminOnly(t *testing.T) {
	service := newLoginService()
	admin, err := service.BootstrapAdmin("admin", "a-long-initial-password")
	if err != nil {
		t.Fatal(err)
	}
	operator := mustCreateOperator(t, service, admin.ID, "operator-force")
	desktop, err := service.LoginWithOptions("operator-force", "a-long-operator-password", LoginOptions{ClientType: ClientTypeDesktop})
	if err != nil {
		t.Fatal(err)
	}
	web, err := service.Login("operator-force", "a-long-operator-password")
	if err != nil {
		t.Fatal(err)
	}
	operatorLogin, err := service.Login("operator-force", "a-long-operator-password")
	if err != nil {
		t.Fatal(err)
	}

	if err := service.ForceLogout(operator.ID, admin.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("non-admin ForceLogout() error = %v, want ErrForbidden", err)
	}
	if err := service.ForceLogout(admin.ID, operator.ID); err != nil {
		t.Fatalf("ForceLogout() error = %v", err)
	}
	for name, token := range map[string]string{"desktop": desktop.Token, "web": web.Token, "operator": operatorLogin.Token} {
		if _, err := service.Authenticate(token); !errors.Is(err, ErrSessionInvalidated) {
			t.Fatalf("%s token error = %v, want ErrSessionInvalidated", name, err)
		}
		session, _, _ := service.store.FindSessionByTokenHash(tokenHash(token))
		if session.InvalidatedReason != SessionInvalidReasonForceLogout {
			t.Fatalf("%s invalidated_reason = %q", name, session.InvalidatedReason)
		}
	}
	foundAudit := false
	for _, event := range service.store.(*memoryStore).AuditEvents() {
		if event.Action == "user.session.force_logout" {
			foundAudit = true
		}
	}
	if !foundAudit {
		t.Fatal("force logout audit event missing")
	}
}

func TestLogoutInvalidatesOnlyCurrentSession(t *testing.T) {
	service := newLoginService()
	if _, err := service.BootstrapAdmin("admin", "a-long-initial-password"); err != nil {
		t.Fatal(err)
	}
	desktop, err := service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{ClientType: ClientTypeDesktop})
	if err != nil {
		t.Fatal(err)
	}
	web, err := service.Login("admin", "a-long-initial-password")
	if err != nil {
		t.Fatal(err)
	}

	if err := service.Logout(web.Token); err != nil {
		t.Fatalf("Logout(web): %v", err)
	}
	if _, err := service.Authenticate(web.Token); !errors.Is(err, ErrSessionInvalidated) {
		t.Fatalf("web token still valid after its own logout: %v", err)
	}
	session, _, _ := service.store.FindSessionByTokenHash(tokenHash(web.Token))
	if session.InvalidatedReason != SessionInvalidReasonLogout {
		t.Fatalf("logout invalidated_reason = %q", session.InvalidatedReason)
	}
	if _, err := service.Authenticate(desktop.Token); err != nil {
		t.Fatalf("desktop session killed by web logout: %v", err)
	}
}
