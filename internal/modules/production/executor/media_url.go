package executor

import (
	"fmt"
	"net/url"
	"strings"
)

func ValidatePublicMediaURL(value string) error {
	if len(value) == 0 || len(value) > 2048 || strings.TrimSpace(value) != value {
		return fmt.Errorf("media URL must contain 1-2048 characters without surrounding whitespace")
	}
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") {
		return fmt.Errorf("media URL must be a public HTTPS address without credentials or a custom port")
	}
	return nil
}
