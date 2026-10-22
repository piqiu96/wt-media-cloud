package agenttoken

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func testSigner(t *testing.T, now time.Time) *Signer {
	t.Helper()
	signer, err := NewSigner([]byte("a-test-agent-token-secret-with-enough-length"))
	if err != nil {
		t.Fatal(err)
	}
	next := 0
	signer.WithClock(func() time.Time { return now })
	signer.newID = func() (string, error) {
		next++
		return "jti-" + string(rune('0'+next)), nil
	}
	return signer
}

func TestIssueBindsExpiryToSessionExpiry(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(30 * 24 * time.Hour)
	signer := testSigner(t, now)

	token, issued, err := signer.Issue(7, "session-desktop", expiresAt)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if issued.ExpiresAt != expiresAt.Unix() || issued.IssuedAt != now.Unix() {
		t.Fatalf("claims times = iat %d exp %d", issued.IssuedAt, issued.ExpiresAt)
	}
	claims, err := signer.Verify(token)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if claims.UserID != 7 || claims.SessionID != "session-desktop" || claims.Scope != ScopeAgent || claims.Version != Version {
		t.Fatalf("claims = %+v", claims)
	}
}

func TestTokenExpiresExactlyAtSessionExpiry(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(time.Hour)
	signer := testSigner(t, now)
	token, _, err := signer.Issue(7, "session-desktop", expiresAt)
	if err != nil {
		t.Fatal(err)
	}

	signer.WithClock(func() time.Time { return expiresAt.Add(-time.Second) })
	if _, err := signer.Verify(token); err != nil {
		t.Fatalf("token expired before session: %v", err)
	}
	signer.WithClock(func() time.Time { return expiresAt })
	if _, err := signer.Verify(token); !errors.Is(err, ErrExpired) {
		t.Fatalf("token at session expiry error = %v, want ErrExpired", err)
	}
}

func TestVerifyRejectsTamperingAndOtherSecrets(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	signer := testSigner(t, now)
	token, _, err := signer.Issue(7, "session-desktop", now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	tampered := token[:len(token)-2] + "aa"
	if _, err := signer.Verify(tampered); !errors.Is(err, ErrInvalidSignature) && !errors.Is(err, ErrMalformed) {
		t.Fatalf("tampered token error = %v", err)
	}

	other, err := NewSigner([]byte("a-different-agent-token-secret-long-enough"))
	if err != nil {
		t.Fatal(err)
	}
	other.WithClock(func() time.Time { return now })
	if _, err := other.Verify(token); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("other-secret token error = %v", err)
	}
}

func TestVerifyRejectsMalformedTokens(t *testing.T) {
	signer := testSigner(t, time.Now())
	for _, token := range []string{
		"",
		"v1",
		"v1.",
		"v1..",
		"v1.payload.",
		"v1.payload.signature.extra",
		"v2.cGF5bG9hZA.c2ln",
		"v1.cGF5bG9hZA=.c2ln",
		"v1.cGF5bG9hZA.c2ln",
	} {
		if _, err := signer.Verify(token); err == nil {
			t.Fatalf("Verify(%q) accepted malformed token", token)
		}
	}
}

func TestVerifyRejectsWrongVersionAndScope(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	signer := testSigner(t, now)

	for name, claims := range map[string]Claims{
		"version": {Version: 2, UserID: 7, SessionID: "s", Scope: ScopeAgent, ExpiresAt: now.Add(time.Hour).Unix()},
		"scope":   {Version: Version, UserID: 7, SessionID: "s", Scope: "user", ExpiresAt: now.Add(time.Hour).Unix()},
	} {
		token := signClaims(t, signer, claims)
		_, err := signer.Verify(token)
		switch name {
		case "version":
			if !errors.Is(err, ErrUnsupportedVersion) {
				t.Fatalf("version error = %v", err)
			}
		case "scope":
			if !errors.Is(err, ErrScope) {
				t.Fatalf("scope error = %v", err)
			}
		}
	}
}

func TestIssueRejectsMissingBinding(t *testing.T) {
	signer := testSigner(t, time.Now())
	if _, _, err := signer.Issue(0, "session", time.Now().Add(time.Hour)); !errors.Is(err, ErrMalformed) {
		t.Fatalf("zero user error = %v", err)
	}
	if _, _, err := signer.Issue(1, "", time.Now().Add(time.Hour)); !errors.Is(err, ErrMalformed) {
		t.Fatalf("empty session error = %v", err)
	}
	if _, err := NewSigner(nil); !errors.Is(err, ErrEmptySecret) {
		t.Fatalf("empty secret error = %v", err)
	}
}

func signClaims(t *testing.T, signer *Signer, claims Claims) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	payloadPart := base64.RawURLEncoding.EncodeToString(payload)
	signed := header + "." + payloadPart
	return signed + "." + base64.RawURLEncoding.EncodeToString(signer.sign(signed))
}
