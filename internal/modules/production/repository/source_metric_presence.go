package repository

import "encoding/json"

// SourceMetricKnown distinguishes an observed zero from the database's default
// zero. Douyin imports retain the original statistics object; discovery retains
// it under raw. Normalized top-level counters are not evidence because the
// crawler inserts zero even when the provider omitted the statistic.
// Historical zero counters without original evidence remain unknown.
func SourceMetricKnown(raw json.RawMessage, field string, value int64) bool {
	var statistic string
	switch field {
	case "like_count":
		statistic = "digg_count"
	case "favorite_count":
		statistic = "collect_count"
	default:
		return false
	}
	if value > 0 {
		return true
	}
	if value < 0 {
		return false
	}
	var envelope struct {
		Statistics map[string]json.RawMessage `json:"statistics"`
		Raw        struct {
			Statistics map[string]json.RawMessage `json:"statistics"`
		} `json:"raw"`
	}
	if json.Unmarshal(raw, &envelope) != nil {
		return false
	}
	for _, statistics := range []map[string]json.RawMessage{envelope.Statistics, envelope.Raw.Statistics} {
		observation, exists := statistics[statistic]
		if !exists {
			continue
		}
		var count *int64
		if json.Unmarshal(observation, &count) == nil && count != nil && *count >= 0 {
			return true
		}
	}
	return false
}
