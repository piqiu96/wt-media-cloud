package contentpool

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseGameIDsAcceptsCommaSeparatedValues(t *testing.T) {
	got, ok := parseGameIDs(" hy, sjz ,,hy ")
	if !ok || len(got) != 2 || got[0] != "hy" || got[1] != "sjz" {
		t.Fatalf("parseGameIDs() = %#v, %v", got, ok)
	}
	values := make([]string, 0, 51)
	for index := 0; index < 51; index++ {
		values = append(values, fmt.Sprintf("game-%d", index))
	}
	if _, ok := parseGameIDs(strings.Join(values, ",")); ok {
		t.Fatal("51 game ids should be rejected")
	}
}
