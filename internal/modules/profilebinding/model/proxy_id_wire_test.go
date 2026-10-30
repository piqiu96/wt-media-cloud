package model

import (
	"encoding/json"
	"testing"

	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

func TestBrowserProfileProxyReferencesUseNumericJSON(t *testing.T) {
	encoded, err := json.Marshal(BrowserProfile{ID: "window-1", ProxyID: sharedidentity.ProxyID("42"), ReservedProxyID: sharedidentity.ProxyID("43")})
	if err != nil {
		t.Fatal(err)
	}
	var view map[string]any
	if err := json.Unmarshal(encoded, &view); err != nil {
		t.Fatal(err)
	}
	if view["proxy_id"] != float64(42) || view["reserved_proxy_id"] != float64(43) {
		t.Fatalf("proxy references = %#v / %#v", view["proxy_id"], view["reserved_proxy_id"])
	}
}
