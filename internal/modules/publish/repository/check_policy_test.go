package repository

import (
	"testing"
	"time"
)

func TestPublicCheckDelayDegradesAndStaysWithinApprovedRanges(t *testing.T) {
	for _, v := range []struct{ age, lo, hi time.Duration }{{0, 4 * time.Minute, 6 * time.Minute}, {30 * time.Minute, 10 * time.Minute, 15 * time.Minute}, {3 * time.Hour, 25 * time.Minute, 35 * time.Minute}} {
		seen := map[time.Duration]bool{}
		for i := 0; i < 100; i++ {
			d := publicCheckDelay(v.age)
			if d < v.lo || d > v.hi {
				t.Fatalf("delay %v", d)
			}
			seen[d] = true
		}
		if len(seen) < 2 {
			t.Fatal("fixed cadence")
		}
	}
}
