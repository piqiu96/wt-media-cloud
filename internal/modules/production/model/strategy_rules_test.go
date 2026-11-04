package model

import (
	"encoding/json"
	"testing"
	"time"
)

func condition(field, operator, value string) ComposeCondition {
	return ComposeCondition{Field: field, Operator: operator, Value: json.RawMessage(value)}
}

func TestComposePriorityRuleValidation(t *testing.T) {
	valid := []ComposePriorityRule{
		{Priority: 2, Action: "match", Logic: "all", Conditions: []ComposeCondition{condition("like_count", "gte", "10")}},
		{Priority: 1, Action: "filter", Logic: "any", Conditions: []ComposeCondition{condition("favorite_count", "eq", "0")}},
	}
	if err := ValidateComposePriorityRules(valid); err != nil {
		t.Fatalf("valid rules: %v", err)
	}
	invalid := []struct {
		name  string
		rules []ComposePriorityRule
	}{
		{"no match", valid[1:]},
		{"duplicate priority", []ComposePriorityRule{valid[0], valid[0]}},
		{"unknown field", []ComposePriorityRule{{Priority: 1, Action: "match", Logic: "all", Conditions: []ComposeCondition{condition("tags", "eq", `"new"`)}}}},
		{"wrong operator", []ComposePriorityRule{{Priority: 1, Action: "match", Logic: "all", Conditions: []ComposeCondition{condition("like_count", "lt", "10")}}}},
		{"fractional count", []ComposePriorityRule{{Priority: 1, Action: "match", Logic: "all", Conditions: []ComposeCondition{condition("like_count", "gte", "1.5")}}}},
	}
	for _, item := range invalid {
		t.Run(item.name, func(t *testing.T) {
			if err := ValidateComposePriorityRules(item.rules); err == nil {
				t.Fatal("invalid rule accepted")
			}
		})
	}
}

func TestComposeGroupPredicateAndWholeGroupFilter(t *testing.T) {
	now := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	facts := ComposeCandidateFacts{LikeCount: 80, FavoriteCount: 2}
	groups := []struct {
		logic string
		want  bool
	}{
		{"all", false},
		{"any", true},
	}
	for _, item := range groups {
		rule := ComposePriorityRule{Logic: item.logic, Conditions: []ComposeCondition{condition("like_count", "gte", "60"), condition("favorite_count", "gte", "5")}}
		got := ComposeGroupPredicate(rule, facts, now)
		if got != item.want {
			t.Fatalf("%s predicate=%v want=%v", item.logic, got, item.want)
		}
		if keep := !got; keep != !item.want {
			t.Fatalf("%s filter keep=%v", item.logic, keep)
		}
	}
}

func TestComposeRecentHoursAndDaysUseRollingBoundary(t *testing.T) {
	now := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	start := now.Add(-24 * time.Hour)
	for _, item := range []struct {
		operator string
		value    string
	}{{"recent_hours", "24"}, {"recent_days", "1"}} {
		rule := ComposePriorityRule{Logic: "all", Conditions: []ComposeCondition{condition("published_at", item.operator, item.value)}}
		if !ComposeGroupPredicate(rule, ComposeCandidateFacts{PublishedAt: &start}, now) {
			t.Fatalf("%s excluded inclusive start", item.operator)
		}
		before := start.Add(-time.Nanosecond)
		if ComposeGroupPredicate(rule, ComposeCandidateFacts{PublishedAt: &before}, now) {
			t.Fatalf("%s included older timestamp", item.operator)
		}
		if ComposeGroupPredicate(rule, ComposeCandidateFacts{}, now) {
			t.Fatalf("%s included missing publication time", item.operator)
		}
	}
}

func TestComposeMissingMetricsAreNotZero(t *testing.T) {
	rule := ComposePriorityRule{Logic: "all", Conditions: []ComposeCondition{condition("favorite_count", "eq", "0")}}
	if ComposeGroupPredicate(rule, ComposeCandidateFacts{}, time.Now()) {
		t.Fatal("absent favorite count matched recorded zero")
	}
}

func TestComposeRuleDecisionExcludesBeforePriority(t *testing.T) {
	rules := []ComposePriorityRule{
		{Priority: 1, Action: "match", Logic: "all", Conditions: []ComposeCondition{condition("like_count", "gte", "10")}},
		{Priority: 2, Action: "filter", Logic: "all", Conditions: []ComposeCondition{condition("favorite_count", "eq", "0")}},
	}
	facts := ComposeCandidateFacts{LikeCount: 20, FavoriteCountKnown: true}
	got := EvaluateComposeRules(rules, facts, time.Now())
	if got.Eligible || got.Reason != "filtered" || got.Priority != 2 {
		t.Fatalf("decision=%+v", got)
	}
	if rules[0].Priority != 1 {
		t.Fatal("evaluation mutated supplied priority order")
	}
}

func TestComposeUnknownGroupOnlyBlocksWhenNeeded(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct{ name, logic, operator, threshold, want string }{
		{"any true decides", "any", "gte", "10", "matched"},
		{"all false decides", "all", "gte", "100", "not_matched"},
		{"any false needs missing", "any", "gte", "100", "unknown"},
		{"all true needs missing", "all", "gte", "10", "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			group := ComposePriorityRule{Logic: tc.logic, Conditions: []ComposeCondition{condition("favorite_count", "eq", "0"), condition("like_count", tc.operator, tc.threshold)}}
			got := EvaluateComposeGroup(group, ComposeCandidateFacts{LikeCount: 20}, now)
			if got.State != tc.want {
				t.Fatalf("got=%+v want=%s", got, tc.want)
			}
			if tc.want == "unknown" && (len(got.MissingFields) != 1 || got.MissingFields[0] != "favorite_count") {
				t.Fatalf("missing=%v", got.MissingFields)
			}
			if tc.want != "unknown" && len(got.MissingFields) != 0 {
				t.Fatalf("decidable group reported blocking missing fields: %v", got.MissingFields)
			}
		})
	}
}

func TestComposeUnknownFilterBlocksSelectionButKnownFilterWins(t *testing.T) {
	rules := []ComposePriorityRule{
		{Priority: 1, Action: "match", Logic: "all", Conditions: []ComposeCondition{condition("like_count", "gte", "10")}},
		{Priority: 2, Action: "filter", Logic: "all", Conditions: []ComposeCondition{condition("favorite_count", "eq", "0")}},
	}
	got := EvaluateComposeRules(rules, ComposeCandidateFacts{LikeCount: 20}, time.Now())
	if got.Eligible || got.Reason != "metrics_missing" || got.Priority != 2 || len(got.MissingFields) != 1 {
		t.Fatalf("decision=%+v", got)
	}
	rules = append(rules, ComposePriorityRule{Priority: 3, Action: "filter", Logic: "all", Conditions: []ComposeCondition{condition("like_count", "gte", "10")}})
	got = EvaluateComposeRules(rules, ComposeCandidateFacts{LikeCount: 20}, time.Now())
	if got.Reason != "filtered" || got.Priority != 3 {
		t.Fatalf("decision=%+v", got)
	}
}

func TestComposeRuleDecisionMatchOrderAndEvidence(t *testing.T) {
	rules := []ComposePriorityRule{
		{Priority: 2, Action: "match", Logic: "all", Conditions: []ComposeCondition{condition("like_count", "gte", "10")}},
		{Priority: 1, Action: "match", Logic: "all", Conditions: []ComposeCondition{condition("favorite_count", "eq", "0")}},
	}
	got := EvaluateComposeRules(rules, ComposeCandidateFacts{LikeCount: 20}, time.Now())
	if got.Eligible || got.Reason != "metrics_missing" || got.Priority != 1 {
		t.Fatalf("uncertain first match=%+v", got)
	}
	got = EvaluateComposeRules(rules, ComposeCandidateFacts{LikeCount: 20, FavoriteCount: 5}, time.Now())
	if !got.Eligible || got.Priority != 2 || got.Reason != "matched" {
		t.Fatalf("matched=%+v", got)
	}
	evidence := got.Conditions[0]
	if evidence.Field != "like_count" || evidence.Operator != "gte" || string(evidence.Expected) != "10" || string(evidence.Actual) != "20" || evidence.State != "matched" {
		t.Fatalf("evidence=%+v", evidence)
	}
	got = EvaluateComposeRules(rules, ComposeCandidateFacts{LikeCountKnown: true, FavoriteCount: 5}, time.Now())
	if got.Eligible || got.Reason != "no_match" {
		t.Fatalf("zero recorded value=%+v", got)
	}
}
