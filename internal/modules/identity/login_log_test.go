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
		{"empty username or password", identityservice.ErrMissingCredentials, "missing_credentials"},
		{"input outside length bounds", identityservice.ErrLoginInputInvalid, "invalid_login_input"},
		{"wrong password or unknown user", identityservice.ErrAuthenticationFailed, "invalid_credentials"},
		{"active desktop session", identityservice.ErrDesktopLoginConflict, "desktop_login_conflict"},
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
