package model

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// ComposeCandidateFacts contains only the source facts exposed by the strategy
// condition contract. Missing publication time never satisfies a date condition.
type ComposeCandidateFacts struct {
	PublishedAt   *time.Time
	LikeCount     int64
	FavoriteCount int64
	// Nonzero counters are known for compatibility. Recorded zero requires its Known flag.
	LikeCountKnown     bool
	FavoriteCountKnown bool
}

func ValidateComposePriorityRules(groups []ComposePriorityRule) error {
	if len(groups) == 0 {
		return fmt.Errorf("at least one match group is required")
	}
	priorities := make(map[int]bool, len(groups))
	hasMatch := false
	for _, group := range groups {
		if group.Priority < 1 || group.Priority > len(groups) || priorities[group.Priority] {
			return fmt.Errorf("priority must be unique and contiguous from 1")
		}
		priorities[group.Priority] = true
		if group.Action != "match" && group.Action != "filter" {
			return fmt.Errorf("action must be match or filter")
		}
		if group.Action == "match" {
			hasMatch = true
		}
		if group.Logic != "all" && group.Logic != "any" {
			return fmt.Errorf("logic must be all or any")
		}
		if len(group.Conditions) == 0 {
			return fmt.Errorf("each group needs a condition")
		}
		for _, condition := range group.Conditions {
			if err := ValidateComposeCondition(condition); err != nil {
				return err
			}
		}
	}
	if !hasMatch {
		return fmt.Errorf("at least one match group is required")
	}
	return nil
}

func ValidateComposeCondition(condition ComposeCondition) error {
	switch condition.Field {
	case "published_at":
		switch condition.Operator {
		case "gte", "lt":
			var value string
			if err := json.Unmarshal(condition.Value, &value); err != nil {
				return fmt.Errorf("published_at requires an RFC3339 date-time")
			}
			if _, err := time.Parse(time.RFC3339, value); err != nil {
				return fmt.Errorf("published_at requires an RFC3339 date-time")
			}
		case "recent_days", "recent_hours":
			var amount int
			maximum := 365
			if condition.Operator == "recent_hours" {
				maximum = 8760
			}
			if err := json.Unmarshal(condition.Value, &amount); err != nil || amount < 1 || amount > maximum {
				return fmt.Errorf("%s must be an integer from 1 to %d", condition.Operator, maximum)
			}
		default:
			return fmt.Errorf("operator is not allowed for published_at")
		}
	case "like_count", "favorite_count":
		if condition.Operator != "eq" && condition.Operator != "gte" && condition.Operator != "lte" {
			return fmt.Errorf("operator is not allowed for %s", condition.Field)
		}
		var value int64
		if err := json.Unmarshal(condition.Value, &value); err != nil || value < 0 {
			return fmt.Errorf("%s requires a non-negative integer", condition.Field)
		}
	default:
		return fmt.Errorf("condition field is not allowed")
	}
	return nil
}

// ComposeConditionDecision preserves the exact facts used for a preview reason.
// Actual is omitted for an unknown fact; it is never fabricated as zero.
type ComposeConditionDecision struct {
	Field    string          `json:"field"`
	Operator string          `json:"operator"`
	Expected json.RawMessage `json:"expected"`
	Actual   json.RawMessage `json:"actual,omitempty"`
	State    string          `json:"state"`
}

type ComposeGroupDecision struct {
	State         string                     `json:"state"` // matched, not_matched, unknown
	MissingFields []string                   `json:"missing_fields,omitempty"`
	Conditions    []ComposeConditionDecision `json:"conditions"`
}

type ComposeRuleDecision struct {
	Eligible      bool                       `json:"eligible"`
	Reason        string                     `json:"reason"` // matched, filtered, no_match, metrics_missing
	Priority      int                        `json:"priority,omitempty"`
	MissingFields []string                   `json:"missing_fields,omitempty"`
	Conditions    []ComposeConditionDecision `json:"conditions"`
}

// EvaluateComposeRules is shared by read-only preview and execution. All filters
// apply before match priority, regardless of their position in the supplied slice.
// An unresolved filter or higher-priority match cannot safely admit a candidate.
func EvaluateComposeRules(groups []ComposePriorityRule, facts ComposeCandidateFacts, now time.Time) ComposeRuleDecision {
	ordered := append([]ComposePriorityRule(nil), groups...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Priority < ordered[j].Priority })
	var unknown *ComposeRuleDecision
	for _, group := range ordered {
		if group.Action != "filter" {
			continue
		}
		result := EvaluateComposeGroup(group, facts, now)
		decision := ComposeRuleDecision{Priority: group.Priority, Conditions: result.Conditions, MissingFields: result.MissingFields}
		if result.State == "matched" {
			decision.Reason = "filtered"
			return decision
		}
		if result.State == "unknown" && unknown == nil {
			decision.Reason = "metrics_missing"
			unknown = &decision
		}
	}
	if unknown != nil {
		return *unknown
	}
	evidence := []ComposeConditionDecision{}
	for _, group := range ordered {
		if group.Action != "match" {
			continue
		}
		result := EvaluateComposeGroup(group, facts, now)
		decision := ComposeRuleDecision{Priority: group.Priority, Conditions: result.Conditions, MissingFields: result.MissingFields}
		if result.State == "matched" {
			decision.Eligible = true
			decision.Reason = "matched"
			return decision
		}
		if result.State == "unknown" {
			decision.Reason = "metrics_missing"
			return decision
		}
		evidence = append(evidence, result.Conditions...)
	}
	return ComposeRuleDecision{Reason: "no_match", Conditions: evidence}
}

func ComposeGroupPredicate(group ComposePriorityRule, facts ComposeCandidateFacts, now time.Time) bool {
	return EvaluateComposeGroup(group, facts, now).State == "matched"
}

// EvaluateComposeGroup uses three-valued logic: true settles any, false settles
// all. Missing fields block only when the known predicates cannot settle a group.
func EvaluateComposeGroup(group ComposePriorityRule, facts ComposeCandidateFacts, now time.Time) ComposeGroupDecision {
	result := ComposeGroupDecision{State: "not_matched", Conditions: []ComposeConditionDecision{}}
	if len(group.Conditions) == 0 || (group.Logic != "all" && group.Logic != "any") {
		return result
	}
	missing := []string{}
	decisive := false
	for _, condition := range group.Conditions {
		detail := composeConditionDecision(condition, facts, now)
		result.Conditions = append(result.Conditions, detail)
		if detail.State == "unknown" {
			found := false
			for _, field := range missing {
				if field == condition.Field {
					found = true
					break
				}
			}
			if !found {
				missing = append(missing, condition.Field)
			}
		}
		if (group.Logic == "any" && detail.State == "matched") || (group.Logic == "all" && detail.State == "not_matched") {
			decisive = true
		}
	}
	if decisive {
		if group.Logic == "any" {
			result.State = "matched"
		}
		return result
	}
	if len(missing) > 0 {
		result.State = "unknown"
		result.MissingFields = missing
		return result
	}
	if group.Logic == "all" {
		result.State = "matched"
	}
	return result
}

func composeConditionDecision(condition ComposeCondition, facts ComposeCandidateFacts, now time.Time) ComposeConditionDecision {
	detail := ComposeConditionDecision{Field: condition.Field, Operator: condition.Operator, Expected: append(json.RawMessage(nil), condition.Value...), State: "unknown"}
	// Persisted rules are validated on save and run. Still fail closed for malformed
	// input to the shared pure helper, rather than interpreting bad JSON as zero.
	if ValidateComposeCondition(condition) != nil {
		return detail
	}
	matches := false
	if condition.Field == "published_at" {
		if facts.PublishedAt == nil {
			return detail
		}
		detail.Actual, _ = json.Marshal(facts.PublishedAt.UTC().Format(time.RFC3339Nano))
		if condition.Operator == "recent_days" || condition.Operator == "recent_hours" {
			var amount int
			_ = json.Unmarshal(condition.Value, &amount)
			duration := time.Duration(amount) * time.Hour
			if condition.Operator == "recent_days" {
				duration *= 24
			}
			matches = !facts.PublishedAt.Before(now.Add(-duration)) && !facts.PublishedAt.After(now)
		} else {
			var text string
			_ = json.Unmarshal(condition.Value, &text)
			boundary, _ := time.Parse(time.RFC3339, text)
			matches = facts.PublishedAt.Before(boundary)
			if condition.Operator == "gte" {
				matches = !matches
			}
		}
	} else {
		value, known := facts.LikeCount, facts.LikeCountKnown || facts.LikeCount != 0
		if condition.Field == "favorite_count" {
			value, known = facts.FavoriteCount, facts.FavoriteCountKnown || facts.FavoriteCount != 0
		}
		if !known {
			return detail
		}
		detail.Actual, _ = json.Marshal(value)
		var threshold int64
		_ = json.Unmarshal(condition.Value, &threshold)
		switch condition.Operator {
		case "eq":
			matches = value == threshold
		case "gte":
			matches = value >= threshold
		case "lte":
			matches = value <= threshold
		}
	}
	detail.State = "not_matched"
	if matches {
		detail.State = "matched"
	}
	return detail
}
