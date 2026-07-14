package cloudagent

import "testing"

func TestCurrentCompatibility(t *testing.T) {
	got := CurrentCompatibility()

	if got.API != "cloud-agent" {
		t.Fatalf("API = %q", got.API)
	}
	if got.MajorVersion != "v1" {
		t.Fatalf("MajorVersion = %q", got.MajorVersion)
	}
	if got.ContractRevision != "2026.07.14.6" {
		t.Fatalf("ContractRevision = %q", got.ContractRevision)
	}
	if got.MinimumAgentContractRevision != "2026.07.14.6" {
		t.Fatalf("MinimumAgentContractRevision = %q", got.MinimumAgentContractRevision)
	}
}

func TestIsAgentCompatible(t *testing.T) {
	tests := []struct {
		name     string
		major    string
		revision string
		want     bool
	}{
		{name: "current", major: "v1", revision: "2026.07.14.6", want: true},
		{name: "newer compatible revision", major: "v1", revision: "2026.07.14.7", want: true},
		{name: "prior revision", major: "v1", revision: "2026.07.14.5", want: false},
		{name: "older revision", major: "v1", revision: "2026.07.13.1", want: false},
		{name: "wrong major", major: "v2", revision: "2026.07.14.1", want: false},
		{name: "malformed revision", major: "v1", revision: "2026-07-14", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAgentCompatible(tt.major, tt.revision); got != tt.want {
				t.Fatalf("IsAgentCompatible(%q, %q) = %v, want %v", tt.major, tt.revision, got, tt.want)
			}
		})
	}
}
