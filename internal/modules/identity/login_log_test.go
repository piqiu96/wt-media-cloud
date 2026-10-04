package identity

import (
	"errors"
	"testing"

	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
)

func TestLoginFailureReasonIsGreppable(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"wrong password or unknown user", identityservice.ErrAuthenticationFailed, "invalid_credentials"},
		{"existing active session", identityservice.ErrSessionReplaceNeeded, "session_replace_needed"},
		{"forbidden", identityservice.ErrForbidden, "forbidden"},
		{"unexpected", errors.New("boom"), "internal_error"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := loginFailureReason(testCase.err); got != testCase.want {
				t.Fatalf("loginFailureReason() = %q, want %q", got, testCase.want)
			}
		})
	}
}
