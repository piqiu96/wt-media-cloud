package service

import (
	"errors"
	"fmt"
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

// Desktop and web sessions coexist: a web login must not conflict with an
// active desktop session (and vice versa), and each session records its own
// client_type.
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
	if desktop.Token == web.Token {
		t.Fatalf("desktop and web reused token %q", desktop.Token)
	}

	if _, err := service.Authenticate(desktop.Token); err != nil {
		t.Fatalf("desktop session died on web login: %v", err)
	}
	if _, err := service.Authenticate(web.Token); err != nil {
		t.Fatalf("web session invalid: %v", err)
	}

	desktopCtx, err := service.AuthenticateContext(desktop.Token)
	if err != nil {
		t.Fatal(err)
	}
	if desktopCtx.Session.ClientType != ClientTypeDesktop {
		t.Fatalf("desktop session client_type = %q, want %q", desktopCtx.Session.ClientType, ClientTypeDesktop)
	}
	webCtx, err := service.AuthenticateContext(web.Token)
	if err != nil {
		t.Fatal(err)
	}
	if webCtx.Session.ClientType != ClientTypeWeb {
		t.Fatalf("web session client_type = %q, want %q", webCtx.Session.ClientType, ClientTypeWeb)
	}
}

// A new login only conflicts with, and only replaces, sessions of the same
// client_type. An empty ClientType defaults to web.
func TestLoginReplacementIsPerClientType(t *testing.T) {
	service := newLoginService()
	if _, err := service.BootstrapAdmin("admin", "a-long-initial-password"); err != nil {
		t.Fatal(err)
	}

	desktop, err := service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{ClientType: ClientTypeDesktop})
	if err != nil {
		t.Fatal(err)
	}
	// Same type without replace -> ErrSessionReplaceNeeded (20010 path).
	if _, err := service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{ClientType: ClientTypeDesktop}); !errors.Is(err, ErrSessionReplaceNeeded) {
		t.Fatalf("second desktop login error = %v, want ErrSessionReplaceNeeded", err)
	}
	// Cross type never conflicts.
	web, err := service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{})
	if err != nil {
		t.Fatalf("web login while desktop active: %v", err)
	}

	// Confirmed replacement kills only the same type's old session.
	desktop2, err := service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{ClientType: ClientTypeDesktop, ReplaceExisting: true})
	if err != nil {
		t.Fatalf("confirmed desktop replacement: %v", err)
	}
	if _, err := service.Authenticate(desktop.Token); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("old desktop token still valid after replacement: %v", err)
	}
	if _, err := service.Authenticate(web.Token); err != nil {
		t.Fatalf("web session killed by desktop replacement: %v", err)
	}
	if _, err := service.Authenticate(desktop2.Token); err != nil {
		t.Fatalf("new desktop session invalid: %v", err)
	}
}

// Logout invalidates only the session that logged out; the coexisting session
// of the other client_type survives.
func TestLogoutInvalidatesOnlyCurrentSession(t *testing.T) {
	service := newLoginService()
	if _, err := service.BootstrapAdmin("admin", "a-long-initial-password"); err != nil {
		t.Fatal(err)
	}

	desktop, err := service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{ClientType: ClientTypeDesktop})
	if err != nil {
		t.Fatal(err)
	}
	web, err := service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if err := service.Logout(web.Token); err != nil {
		t.Fatalf("Logout(web): %v", err)
	}
	if _, err := service.Authenticate(web.Token); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("web token still valid after its own logout: %v", err)
	}
	if _, err := service.Authenticate(desktop.Token); err != nil {
		t.Fatalf("desktop session killed by web logout: %v", err)
	}
}
