package service

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

// Run only with an explicitly supplied safe test proxy. The address and
// credentials are never saved or printed; this exercises the same Cloud-side
// probe used by the API against its real target network.
func TestLiveProvidedProxyReachesDefaultTarget(t *testing.T) {
	if os.Getenv("WT_MEDIA_TEST_OPERATING_RESOURCES") != "1" {
		t.Skip("live proxy testing is not enabled")
	}
	raw := os.Getenv("WT_MEDIA_TEST_PROXY_ADDRESS")
	if raw == "" {
		t.Skip("no safe test proxy supplied")
	}
	parts := strings.Split(raw, "|")
	if len(parts) < 4 {
		t.Fatal("test proxy input must contain host, port, username and password")
	}
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		t.Fatal("test proxy port is invalid")
	}
	for _, protocol := range []Protocol{ProtocolHTTP, ProtocolSOCKS5} {
		t.Run(string(protocol), func(t *testing.T) {
			result := Probe(protocol, parts[0], port, parts[2], parts[3], ProbeOptions{})
			if result.Connectivity != CheckOK {
				t.Fatalf("Cloud probe result = %s; target connection did not pass", result.Connectivity)
			}
		})
	}
}
