package service

import "testing"

func TestProxyDialAddressSupportsIPv6(t *testing.T) {
	if got := proxyDialAddress("2001:db8::1", 1080); got != "[2001:db8::1]:1080" {
		t.Fatalf("proxyDialAddress() = %q", got)
	}
}
