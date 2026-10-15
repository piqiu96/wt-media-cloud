package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestComposeTaskDisplayNameUsesStableFallbackAndRuneLimit(t *testing.T) {
	if got := ComposeTaskDisplayName("", 157, " 高光 "); got != "素材 #157 · 高光" {
		t.Fatalf("fallback name = %q", got)
	}
	long := "这是一段很长的标题"
	for len([]rune(long)) < 110 {
		long += "标题"
	}
	got := ComposeTaskDisplayName(long, 157, "策略")
	if len([]rune(got)) > 110 || got == long+" · 策略" {
		t.Fatalf("name was not bounded by runes: %q", got)
	}
}

func TestComposeTaskSerializesStoredDisplayName(t *testing.T) {
	data, err := json.Marshal(ComposeTask{ID: 252, DisplayName: "素材一 · 切段"})
	if err != nil || !strings.Contains(string(data), `"display_name":"素材一 · 切段"`) {
		t.Fatalf("task API projection lost display name: %s, %v", data, err)
	}
}
