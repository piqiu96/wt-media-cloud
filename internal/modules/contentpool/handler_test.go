package contentpool

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseCategoryIDsAcceptsCommaSeparatedValues(t *testing.T) {
	got, ok := parseCategoryIDs(" 1, 3 ,,1 ")
	if !ok || len(got) != 2 || got[0] != 1 || got[1] != 3 {
		t.Fatalf("parseCategoryIDs() = %#v, %v", got, ok)
	}
	values := make([]string, 0, 51)
	for index := 0; index < 51; index++ {
		values = append(values, fmt.Sprintf("%d", index+1))
	}
	if _, ok := parseCategoryIDs(strings.Join(values, ",")); ok {
		t.Fatal("51 category ids should be rejected")
	}
}
