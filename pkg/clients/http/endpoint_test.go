package httpclient

import "testing"

func TestEndpointOriginAndRandomDialAddresses(t *testing.T) {
	endpoint := EndpointConfig{
		Scheme:    "https",
		Host:      "api.example.test",
		Port:      443,
		Addresses: []string{"127.0.0.1:443", "127.0.0.2:443"},
	}
	if got, want := endpoint.Origin(), "https://api.example.test:443"; got != want {
		t.Fatalf("Origin() = %q, want %q", got, want)
	}

	dialer := newEndpointDialer(endpoint).(endpointDialer)
	seen := make(map[string]struct{})
	for range 100 {
		seen[dialer.dialAddress(endpoint.LogicalAddress())] = struct{}{}
	}
	if len(seen) != 2 {
		t.Fatalf("dial addresses = %v, want both configured addresses", seen)
	}
	if got := dialer.dialAddress("other.example.test:443"); got != "other.example.test:443" {
		t.Fatalf("non-endpoint address = %q", got)
	}
}
