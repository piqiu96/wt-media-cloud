// Package agenttoken issues and verifies the stateless token a Local Agent uses
// to call Cloud Agent APIs. The token is bound to one Desktop user session and
// carries no server-side state: its lifetime is the session's lifetime.
package agenttoken

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const (
	Version    = 1
	ScopeAgent = "agent"
	header     = "v1"
)

var (
	ErrMalformed          = errors.New("agent token is malformed")
	ErrUnsupportedVersion = errors.New("agent token version is unsupported")
	ErrInvalidSignature   = errors.New("agent token signature is invalid")
	ErrExpired            = errors.New("agent token has expired")
	ErrScope              = errors.New("agent token scope is not agent")
	ErrEmptySecret        = errors.New("agent token secret is empty")
)

// Claims is the signed payload. Exp is the Desktop session's expires_at in Unix
// seconds; there is deliberately no independent agent-token TTL.
type Claims struct {
	Version   int    `json:"ver"`
	UserID    int64  `json:"sub"`
	SessionID string `json:"sid"`
	Scope     string `json:"scope"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	ID        string `json:"jti"`
}

type Signer struct {
	secret []byte
	now    func() time.Time
	newID  func() (string, error)
}

func NewSigner(secret []byte) (*Signer, error) {
	if len(secret) == 0 {
		return nil, ErrEmptySecret
	}
	return &Signer{
		secret: append([]byte(nil), secret...),
		now:    time.Now,
		newID:  randomID,
	}, nil
}

// WithClock is for tests and deterministic expiry checks.
func (s *Signer) WithClock(now func() time.Time) *Signer {
	s.now = now
	return s
}

// Issue signs a token whose exp is exactly the Desktop session's expiry. The
// caller must already have proven that session valid.
func (s *Signer) Issue(userID int64, sessionID string, expiresAt time.Time) (string, Claims, error) {
	if userID <= 0 || sessionID == "" {
		return "", Claims{}, ErrMalformed
	}
	id, err := s.newID()
	if err != nil {
		return "", Claims{}, err
	}
	now := s.now()
	claims := Claims{
		Version:   Version,
		UserID:    userID,
		SessionID: sessionID,
		Scope:     ScopeAgent,
		IssuedAt:  now.Unix(),
		ExpiresAt: expiresAt.Unix(),
		ID:        id,
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", Claims{}, err
	}
	payloadPart := base64.RawURLEncoding.EncodeToString(payload)
	signed := header + "." + payloadPart
	signature := s.sign(signed)
	return signed + "." + base64.RawURLEncoding.EncodeToString(signature), claims, nil
}

// Verify checks format, signature, version, scope and expiry in that order.
// Callers choose what else to require: strict mode additionally compares
// SessionID with the current valid Desktop session; report-safe mode only
// compares UserID with the task owner.
func (s *Signer) Verify(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != header || parts[1] == "" || parts[2] == "" {
		return Claims{}, ErrMalformed
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(signature) != sha256.Size {
		return Claims{}, ErrMalformed
	}
	if !hmac.Equal(signature, s.sign(parts[0]+"."+parts[1])) {
		return Claims{}, ErrInvalidSignature
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrMalformed
	}
	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Claims{}, ErrMalformed
	}
	if claims.Version != Version {
		return Claims{}, ErrUnsupportedVersion
	}
	if claims.Scope != ScopeAgent {
		return Claims{}, ErrScope
	}
	if claims.UserID <= 0 || claims.SessionID == "" || claims.ExpiresAt <= 0 {
		return Claims{}, ErrMalformed
	}
	if s.now().Unix() >= claims.ExpiresAt {
		return Claims{}, ErrExpired
	}
	return claims, nil
}

func (s *Signer) sign(signed string) []byte {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(signed))
	return mac.Sum(nil)
}

func randomID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}
