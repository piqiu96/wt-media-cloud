package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestComposeTaskDisplayNameUsesStableFallbackAndRuneLimit(t *testing.T) {
	if got := ComposeTaskDisplayName("", 157, " 高光 "); got != "素材 157 · 高光" {
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

// The batch label is the operator's own words, so it is used as given — only
// the numbering and the blank fallback belong to the server.
func TestComposeTaskBatchDisplayNameNumbersTheCommand(t *testing.T) {
	for _, tc := range []struct {
		name         string
		label        string
		index, total int
		want         string
	}{
		{"several materials are numbered from one", "双十一预热", 0, 3, "双十一预热-1"},
		{"the numbering follows submission order", "双十一预热", 2, 3, "双十一预热-3"},
		{"a single material keeps the plain label", "双十一预热", 0, 1, "双十一预热"},
		{"surrounding blanks are not part of the name", "  双十一预热  ", 0, 2, "双十一预热-1"},
		{"a blank label keeps the derived name", "", 0, 2, "素材标题 · 高光"},
		{"a whitespace-only label is blank too", "   ", 1, 2, "素材标题 · 高光"},
	} {
		got := ComposeTaskBatchDisplayName(tc.label, tc.index, tc.total, "素材标题", 157, "高光")
		if got != tc.want {
			t.Errorf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

// The derived-name path must stay byte-identical to the single-task rule the
// product has had all along, so a client that sends no name sees no change.
func TestComposeTaskBatchDisplayNameFallsBackToTheExistingRule(t *testing.T) {
	long := strings.Repeat("标", 120)
	for _, tc := range []struct {
		label      string
		title      string
		materialID int64
		strategy   string
	}{
		{"", "素材一", 157, "高光"},
		{"  ", "", 157, " 高光 "},
		{"", long, 9, strings.Repeat("策", 130)},
	} {
		if got, want := ComposeTaskBatchDisplayName(tc.label, 0, 1, tc.title, tc.materialID, tc.strategy), ComposeTaskDisplayName(tc.title, tc.materialID, tc.strategy); got != want {
			t.Errorf("fallback = %q, want the derived name %q", got, want)
		}
	}
}
