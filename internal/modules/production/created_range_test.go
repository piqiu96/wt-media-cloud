package production

import (
	"testing"
	"time"
)

func TestParseComposeCreatedRange(t *testing.T) {
	from, before, err := parseComposeCreatedRange("2026-10-12T16:00:00Z", "2026-10-13T16:00:00Z")
	if err != nil || !from.Equal(time.Date(2026, 10, 12, 16, 0, 0, 0, time.UTC)) || before.Sub(from) != 24*time.Hour {
		t.Fatalf("from=%v before=%v err=%v", from, before, err)
	}
	for _, pair := range [][2]string{
		{"2026-10-13", ""},
		{"", "invalid"},
		{"2026-10-13T16:00:00Z", "2026-10-13T16:00:00Z"},
		{"2026-10-14T16:00:00Z", "2026-10-13T16:00:00Z"},
	} {
		if _, _, err := parseComposeCreatedRange(pair[0], pair[1]); err == nil {
			t.Fatalf("accepted invalid range %v", pair)
		}
	}
}
