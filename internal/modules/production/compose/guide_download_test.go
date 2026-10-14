package compose

import (
	"context"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGuideDownloadRejectsNonPublicTargets(t *testing.T) {
	for _, address := range []string{"127.0.0.1", "10.1.2.3", "169.254.169.254", "100.64.1.1", "198.18.0.1", "::1", "fc00::1", "2001:db8::1"} {
		if publicGuideIP(netip.MustParseAddr(address)) {
			t.Fatalf("%s accepted", address)
		}
	}
	for _, address := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"} {
		if !publicGuideIP(netip.MustParseAddr(address)) {
			t.Fatalf("%s rejected", address)
		}
	}
	path := filepath.Join(t.TempDir(), "guide.mp4")
	if err := DownloadGuide(context.Background(), "https://127.0.0.1/guide.mp4", path); err == nil || !strings.Contains(err.Error(), "guide download failed") {
		t.Fatalf("private target error = %v", err)
	}
}

func TestGuideDownloadRejectsDeclaredAndStreamedOversize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "guide.mp4")
	if err := saveGuideBody(strings.NewReader("video"), 100, path, 4); err == nil {
		t.Fatal("declared oversize accepted")
	}
	if err := saveGuideBody(strings.NewReader("video"), -1, path, 4); err == nil {
		t.Fatal("streamed oversize accepted")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("oversize file remains: %v", err)
	}
	if err := saveGuideBody(strings.NewReader("video"), 5, path, 5); err != nil {
		t.Fatal(err)
	}
}
