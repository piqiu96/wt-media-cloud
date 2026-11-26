package model

import "testing"

func TestOmittedCapabilitiesCannotClaimNewPlatforms(t *testing.T) {
	p, ok := SupportedPublicationPlatforms(nil)
	if !ok || len(p) != 1 || p[0] != "baidu" {
		t.Fatal("legacy Agent gained new-platform execution")
	}
	for _, in := range [][]string{{}, {"bilibili", "bilibili"}, {"xiaohongshu"}, {"baidu", "bilibili", "baidu"}} {
		if _, ok := SupportedPublicationPlatforms(in); ok {
			t.Fatalf("invalid capabilities accepted: %+v", in)
		}
	}
	in := []string{"baidu", "bilibili"}
	p, ok = SupportedPublicationPlatforms(in)
	if !ok || len(p) != 2 {
		t.Fatal("explicit Bilibili consumer rejected")
	}
	p[0] = "changed"
	if in[0] != "baidu" {
		t.Fatal("capabilities were not copied")
	}
}
