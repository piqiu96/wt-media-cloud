package agenttoken

import (
	"errors"
	"sync/atomic"
)

var ErrNotConfigured = errors.New("agent token signer is not configured")

var current atomic.Pointer[Signer]

// Configure installs the process signer once at startup. Tests may replace it.
func Configure(signer *Signer) {
	current.Store(signer)
}

func Current() (*Signer, error) {
	signer := current.Load()
	if signer == nil {
		return nil, ErrNotConfigured
	}
	return signer, nil
}

// IsAgentToken reports whether an Authorization header carries a token signed
// by the configured signer. It lets the user-session middleware reject the
// wrong credential kind with an explicit code instead of reporting "no session".
func IsAgentToken(header string) bool {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || header[:len(prefix)] != prefix {
		return false
	}
	signer, err := Current()
	if err != nil {
		return false
	}
	_, err = signer.Verify(header[len(prefix):])
	return err == nil
}
