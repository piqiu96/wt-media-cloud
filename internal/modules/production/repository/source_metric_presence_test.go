package repository

import (
	"encoding/json"
	"testing"
)

func TestSourceMetricKnownRequiresOriginalZeroEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, raw, field string
		value            int64
		want             bool
	}{
		{"manual explicit zero", `{"statistics":{"digg_count":0}}`, "like_count", 0, true},
		{"discovery explicit zero", `{"like_count":0,"raw":{"statistics":{"digg_count":0}}}`, "like_count", 0, true},
		{"views explicit zero", `{"statistics":{"play_count":0}}`, "view_count", 0, true},
		{"favorite explicit zero", `{"statistics":{"collect_count":0}}`, "favorite_count", 0, true},
		{"discovery missing metric", `{"like_count":0,"favorite_count":0,"raw":{"statistics":{}}}`, "like_count", 0, false},
		{"normalized zero is not evidence", `{"like_count":0}`, "like_count", 0, false},
		{"missing raw", `{}`, "favorite_count", 0, false},
		{"null statistic", `{"statistics":{"digg_count":null}}`, "like_count", 0, false},
		{"invalid statistic", `{"statistics":{"digg_count":"unknown"}}`, "like_count", 0, false},
		{"negative statistic", `{"statistics":{"digg_count":-1}}`, "like_count", 0, false},
		{"fractional statistic", `{"statistics":{"digg_count":0.5}}`, "like_count", 0, false},
		{"numeric string is not provider count", `{"statistics":{"digg_count":"0"}}`, "like_count", 0, false},
		{"malformed raw", `{`, "like_count", 0, false},
		{"legacy positive intrinsically known", `{}`, "like_count", 23, true},
		{"unsupported field", `{"statistics":{"digg_count":0}}`, "title", 23, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := SourceMetricKnown(json.RawMessage(tc.raw), tc.field, tc.value); got != tc.want {
				t.Fatalf("SourceMetricKnown=%v want=%v", got, tc.want)
			}
		})
	}
}
