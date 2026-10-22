package service

import (
	"errors"
	"testing"
	"time"
)

// The Agent Token's `sid` is only meaningful while that session is the user's
// current Desktop login. This lookup is the strict-mode gate, so the branches
// it must keep apart are "replaced by another computer" (a distinct code the
// old Desktop shows) and every other way a session stops being usable (the
// caller is told to log in on this Desktop).
func TestAuthenticateAgentSessionSeparatesReplacementFromOtherInvalidations(t *testing.T) {
	service := newLoginService()
	admin, err := service.BootstrapAdmin("admin", "a-long-initial-password")
	if err != nil {
		t.Fatal(err)
	}

	first, err := service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{ClientType: ClientTypeDesktop})
	if err != nil {
		t.Fatalf("first desktop login: %v", err)
	}
	context, err := service.AuthenticateAgentSession(first.SessionID)
	if err != nil {
		t.Fatalf("current desktop session: %v", err)
	}
	if context.Session.ID != first.SessionID || context.User.ID != admin.ID {
		t.Fatalf("agent session = %+v", context)
	}
	if !context.Session.ExpiresAt.Equal(first.ExpiresAt) {
		t.Fatalf("session expiry = %v, want the login's %v: the token's exp is derived from it", context.Session.ExpiresAt, first.ExpiresAt)
	}

	second, err := service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{
		ClientType: ClientTypeDesktop, ReplaceExisting: true, ReplaceSessionID: first.SessionID,
	})
	if err != nil {
		t.Fatalf("confirmed desktop replacement: %v", err)
	}
	if _, err := service.AuthenticateAgentSession(first.SessionID); !errors.Is(err, ErrDesktopReplaced) {
		t.Fatalf("replaced session error = %v, want ErrDesktopReplaced", err)
	}
	if _, err := service.AuthenticateAgentSession(second.SessionID); err != nil {
		t.Fatalf("replacement session invalid: %v", err)
	}

	web, err := service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{})
	if err != nil {
		t.Fatalf("web login: %v", err)
	}
	if _, err := service.AuthenticateAgentSession(web.SessionID); !errors.Is(err, ErrDesktopLoginRequired) {
		t.Fatalf("web session error = %v, want ErrDesktopLoginRequired: an Agent Token is issued from a Desktop login", err)
	}
	if _, err := service.AuthenticateAgentSession("no-such-session"); !errors.Is(err, ErrDesktopLoginRequired) {
		t.Fatalf("unknown session error = %v, want ErrDesktopLoginRequired", err)
	}

	if err := service.ChangeOwnPassword(admin.ID, "a-long-initial-password", "another-long-password"); err != nil {
		t.Fatalf("change password: %v", err)
	}
	if _, err := service.AuthenticateAgentSession(second.SessionID); !errors.Is(err, ErrDesktopLoginRequired) {
		t.Fatalf("session after password change error = %v, want ErrDesktopLoginRequired", err)
	}
}

// Two valid Desktop rows cannot come out of a login, only out of a migration
// that kept both or a takeover that lost a race. Strict mode must still refuse
// new work as a takeover instead of asking the user to log in again, which
// would loop: the next login would find the same ambiguity.
func TestAuthenticateAgentSessionFailsClosedOnSeveralValidDesktopRows(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	store := NewMemoryStore()
	service := NewService(store, WithClock(func() time.Time { return now }))
	admin, err := service.BootstrapAdmin("admin", "a-long-initial-password")
	if err != nil {
		t.Fatal(err)
	}
	current, err := service.LoginWithOptions("admin", "a-long-initial-password", LoginOptions{ClientType: ClientTypeDesktop})
	if err != nil {
		t.Fatalf("desktop login: %v", err)
	}
	// Only reachable by writing behind the login path, which is the point.
	store.sessions["stale-token-hash"] = Session{
		ID: "stale-session", UserID: admin.ID, TokenHash: "stale-token-hash",
		ClientType: ClientTypeDesktop, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour),
	}

	if _, err := service.AuthenticateAgentSession(current.SessionID); !errors.Is(err, ErrDesktopReplaced) {
		t.Fatalf("ambiguous desktop rows error = %v, want ErrDesktopReplaced", err)
	}
}
