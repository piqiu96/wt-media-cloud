package storage

import (
	"errors"
	"net/url"
	"strings"
)

// PublicObjectURL publishes an explicitly located object through its bucket's
// configured public HTTPS origin. It never reinterprets a legacy key or applies
// the current global prefix to a persisted location.
func PublicObjectURL(baseURL, bucket, reference string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil || base.Scheme != "https" || base.Hostname() == "" || base.User != nil || base.Opaque != "" || base.RawQuery != "" || base.ForceQuery || base.Fragment != "" || (base.Path != "" && base.Path != "/") {
		return "", errors.New("files public base URL must be an HTTPS origin")
	}
	ref, err := url.Parse(reference)
	if err != nil || bucket == "" || ref.Scheme != "s3" || ref.Host != bucket || ref.User != nil || ref.Opaque != "" || ref.RawQuery != "" || ref.ForceQuery || ref.Fragment != "" || !strings.HasPrefix(ref.Path, "/") {
		return "", errors.New("file object must have a full location in the configured public bucket")
	}
	if err := validateKey(strings.TrimPrefix(ref.Path, "/")); err != nil {
		return "", err
	}
	base.Path = ref.Path
	base.RawPath = ""
	return base.String(), nil
}
