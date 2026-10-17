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
