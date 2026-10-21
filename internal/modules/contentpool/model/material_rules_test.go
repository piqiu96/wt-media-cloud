package model

import (
	"encoding/json"
	"testing"
	"time"

	productionmodel "github.com/wt-media/wt-media-cloud/internal/modules/production/model"
)

func condition(field, operator string, value any) MaterialCondition {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return MaterialCondition{Field: field, Operator: operator, Value: encoded}
}

func group(priority int, action, logic string, conditions ...MaterialCondition) MaterialRuleGroup {
	return MaterialRuleGroup{Priority: priority, Action: action, Logic: logic, Conditions: conditions}
}

func TestValidateMaterialRulesAllowsEmptyList(t *testing.T) {
	if err := ValidateMaterialRules(nil); err != nil {
		t.Fatalf("nil rules should be valid: %v", err)
	}
	if err := ValidateMaterialRules([]MaterialRuleGroup{}); err != nil {
		t.Fatalf("empty rules should be valid: %v", err)
	}
}

func TestValidateMaterialRulesRejectsMalformedGroups(t *testing.T) {
	like := condition("like_count", "gte", 1000)
	cases := map[string][]MaterialRuleGroup{
		"duplicate priority": {
			group(1, "match", "all", like),
			group(1, "filter", "all", like),
		},
		"priority gap": {
			group(1, "match", "all", like),
			group(3, "filter", "all", like),
		},
		"priority starts at zero": {
			group(0, "match", "all", like),
		},
		"unknown action": {
			group(1, "keep", "all", like),
		},
		"unknown logic": {
			group(1, "match", "xor", like),
		},
		"no match group": {
			group(1, "filter", "all", like),
		},
		"empty conditions": {
			group(1, "match", "all"),
		},
		"unknown field": {
			group(1, "match", "all", condition("comment_count", "gte", 1)),
		},
		"published_at with count operator": {
			group(1, "match", "all", condition("published_at", "gte", 10)),
		},
		"published_at not RFC3339": {
			group(1, "match", "all", condition("published_at", "gte", "2026-10-01")),
		},
		"recent_hours out of range": {
			group(1, "match", "all", condition("published_at", "recent_hours", 0)),
		},
		"negative count": {
			group(1, "match", "all", condition("like_count", "gte", -1)),
		},
	}
	for name, groups := range cases {
		if err := ValidateMaterialRules(groups); err == nil {
			t.Errorf("%s: expected validation error", name)
		}
	}
}

func TestValidateMaterialRulesAcceptsComposeShapes(t *testing.T) {
	groups := []MaterialRuleGroup{
		group(1, "filter", "any",
			condition("published_at", "recent_hours", 24),
			condition("like_count", "lte", 10),
		),
		group(2, "match", "all",
			condition("published_at", "gte", "2026-01-01T00:00:00Z"),
			condition("favorite_count", "lte", 5000),
			condition("like_count", "eq", 0),
		),
	}
	if err := ValidateMaterialRules(groups); err != nil {
		t.Fatalf("valid groups rejected: %v", err)
	}
}

// The two modules keep separate copies of the rules so the content pool does
// not depend on production/model. This test pins the shared semantics so the
// copies cannot drift: for every non-empty rule list both validators agree.
func TestMaterialRuleValidationMatchesCompose(t *testing.T) {
	like := condition("like_count", "gte", 1000)
	cases := [][]MaterialRuleGroup{
		{group(1, "match", "all", like)},
		{group(1, "filter", "all", like)},
		{group(1, "match", "any", like)},
		{group(1, "match", "xor", like)},
		{group(1, "keep", "all", like)},
		{group(2, "match", "all", like)},
		{group(1, "match", "all")},
		{group(1, "match", "all", condition("published_at", "recent_days", 7))},
		{group(1, "match", "all", condition("published_at", "recent_days", 366))},
		{group(1, "match", "all", condition("favorite_count", "lte", 0))},
		{
			group(1, "filter", "any", like),
			group(2, "match", "all", condition("favorite_count", "gte", 5)),
		},
	}
	for index, groups := range cases {
		compose := make([]productionmodel.ComposePriorityRule, 0, len(groups))
		for _, item := range groups {
			conditions := make([]productionmodel.ComposeCondition, 0, len(item.Conditions))
			for _, value := range item.Conditions {
				conditions = append(conditions, productionmodel.ComposeCondition{Field: value.Field, Operator: value.Operator, Value: value.Value})
			}
			compose = append(compose, productionmodel.ComposePriorityRule{Priority: item.Priority, Action: item.Action, Logic: item.Logic, Conditions: conditions})
		}
		materialErr := ValidateMaterialRules(groups)
		composeErr := productionmodel.ValidateComposePriorityRules(compose)
		if (materialErr == nil) != (composeErr == nil) {
			t.Errorf("case %d: material err=%v compose err=%v", index, materialErr, composeErr)
		}
	}
}

func TestResolveMaterialRulesPrefersCanonicalShape(t *testing.T) {
	config := map[string]any{
		"material_rules":     []any{},
		"auto_material":      true,
		"material_rule":      "OR",
		"like_threshold":     100,
		"favorite_threshold": 50,
	}
	rules, err := ResolveMaterialRules(config)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 0 {
		t.Fatalf("material_rules must win over legacy keys, got %+v", rules)
	}
}

func TestResolveMaterialRulesRejectsInvalidCanonicalShape(t *testing.T) {
	config := map[string]any{"material_rules": []any{
		map[string]any{"priority": 1, "action": "filter", "logic": "all", "conditions": []any{
			map[string]any{"field": "like_count", "operator": "gte", "value": 10},
		}},
	}}
	if _, err := ResolveMaterialRules(config); err == nil {
		t.Fatal("filter-only canonical rules must fail validation")
	}
}

func TestLegacyMaterialRulesTranslation(t *testing.T) {
	cases := []struct {
		name       string
		config     map[string]any
		wantGroups int
		wantLogic  string
		wantFields []string
	}{
		{name: "missing", config: map[string]any{}, wantGroups: 0},
		{name: "disabled", config: map[string]any{"auto_material": false, "like_threshold": 100}, wantGroups: 0},
		{name: "enabled without thresholds", config: map[string]any{"auto_material": true, "material_rule": "AND"}, wantGroups: 0},
		{name: "zero thresholds", config: map[string]any{"auto_material": true, "like_threshold": 0, "favorite_threshold": 0}, wantGroups: 0},
		{name: "and both", config: map[string]any{"auto_material": true, "material_rule": "AND", "like_threshold": 10000, "favorite_threshold": 500}, wantGroups: 1, wantLogic: "all", wantFields: []string{"like_count", "favorite_count"}},
		{name: "or both", config: map[string]any{"auto_material": true, "material_rule": "or", "like_threshold": 10000, "favorite_threshold": 500}, wantGroups: 1, wantLogic: "any", wantFields: []string{"like_count", "favorite_count"}},
		{name: "like only", config: map[string]any{"auto_material": true, "like_threshold": "1000"}, wantGroups: 1, wantLogic: "all", wantFields: []string{"like_count"}},
		{name: "favorite only", config: map[string]any{"auto_material": true, "favorite_threshold": 50}, wantGroups: 1, wantLogic: "all", wantFields: []string{"favorite_count"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			rules := LegacyMaterialRules(test.config)
			if len(rules) != test.wantGroups {
				t.Fatalf("groups=%+v", rules)
			}
			if test.wantGroups == 0 {
				return
			}
			if rules[0].Priority != 1 || rules[0].Action != "match" || rules[0].Logic != test.wantLogic {
				t.Fatalf("group=%+v", rules[0])
			}
			if len(rules[0].Conditions) != len(test.wantFields) {
				t.Fatalf("conditions=%+v", rules[0].Conditions)
			}
			for index, field := range test.wantFields {
				if rules[0].Conditions[index].Field != field || rules[0].Conditions[index].Operator != "gte" {
					t.Fatalf("condition %d=%+v", index, rules[0].Conditions[index])
				}
			}
		})
	}
}

func TestNormalizeMaterialConfig(t *testing.T) {
	config := map[string]any{
		"keyword":            "demo",
		"auto_material":      true,
		"material_rule":      "OR",
		"like_threshold":     100,
		"favorite_threshold": 0,
	}
	if !NormalizeMaterialConfig(config) {
		t.Fatal("legacy config should normalize")
	}
	if config["keyword"] != "demo" {
		t.Fatalf("unrelated keys must survive: %+v", config)
	}
	for _, legacy := range []string{"auto_material", "material_rule", "like_threshold", "favorite_threshold"} {
		if _, exists := config[legacy]; exists {
			t.Fatalf("legacy key %q survived: %+v", legacy, config)
		}
	}
	rules, err := ResolveMaterialRules(config)
	if err != nil || len(rules) != 1 || rules[0].Logic != "any" {
		t.Fatalf("rules=%+v err=%v", rules, err)
	}

	empty := map[string]any{"keyword": "demo"}
	if !NormalizeMaterialConfig(empty) {
		t.Fatal("config without material keys should normalize")
	}
	if raw, exists := empty["material_rules"]; !exists {
		t.Fatalf("canonical key missing: %+v", empty)
	} else if list, ok := raw.([]any); !ok || len(list) != 0 {
		t.Fatalf("empty rules should be written as an empty array, got %#v", raw)
	}
}

func TestShouldAutoMaterializeFirstMatchingGroupWins(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	facts := MaterialCandidateFacts{LikeCount: 5000, FavoriteCount: 500}
	truthy := group(1, "match", "all", condition("like_count", "gte", 1000))
	falsy := group(2, "filter", "all", condition("like_count", "gte", 1000))

	if ShouldAutoMaterialize(nil, facts, now) {
		t.Fatal("no rules must not materialize")
	}
	if !ShouldAutoMaterialize([]MaterialRuleGroup{truthy}, facts, now) {
		t.Fatal("matching match group should materialize")
	}
	// Priority order is authoritative even when the slice is not sorted.
	blocked := []MaterialRuleGroup{
		group(2, "match", "all", condition("like_count", "gte", 1000)),
		group(1, "filter", "all", condition("favorite_count", "gte", 100)),
	}
	if ShouldAutoMaterialize(blocked, facts, now) {
		t.Fatal("earlier filter group must block")
	}
	unmatchedFilter := []MaterialRuleGroup{
		group(1, "filter", "all", condition("like_count", "gte", 9000)),
		group(2, "match", "all", condition("like_count", "gte", 1000)),
	}
	if !ShouldAutoMaterialize(unmatchedFilter, facts, now) {
		t.Fatal("later match group should decide when the filter misses")
	}
	if ShouldAutoMaterialize([]MaterialRuleGroup{falsy}, facts, now) {
		t.Fatal("only a matching filter group must not materialize")
	}
}

func TestMaterialGroupPredicateLogic(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	facts := MaterialCandidateFacts{LikeCount: 5000, FavoriteCount: 10}
	highLike := condition("like_count", "gte", 1000)
	highFavorite := condition("favorite_count", "gte", 1000)

	if !MaterialGroupPredicate(group(1, "match", "any", highLike, highFavorite), facts, now) {
		t.Fatal("any should pass with one condition")
	}
	if MaterialGroupPredicate(group(1, "match", "all", highLike, highFavorite), facts, now) {
		t.Fatal("all should fail with one unmet condition")
	}
}

func TestMaterialConditionDateComparison(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	at := func(offset time.Duration) *time.Time {
		value := now.Add(offset)
		return &value
	}
	cases := []struct {
		name      string
		condition MaterialCondition
		facts     MaterialCandidateFacts
		want      bool
	}{
		{name: "recent hours inside", condition: condition("published_at", "recent_hours", 24), facts: MaterialCandidateFacts{PublishedAt: at(-23 * time.Hour)}, want: true},
		{name: "recent hours boundary", condition: condition("published_at", "recent_hours", 24), facts: MaterialCandidateFacts{PublishedAt: at(-24 * time.Hour)}, want: true},
		{name: "recent hours outside", condition: condition("published_at", "recent_hours", 24), facts: MaterialCandidateFacts{PublishedAt: at(-24*time.Hour - time.Second)}, want: false},
		{name: "recent hours future", condition: condition("published_at", "recent_hours", 24), facts: MaterialCandidateFacts{PublishedAt: at(time.Hour)}, want: false},
		{name: "recent days inside", condition: condition("published_at", "recent_days", 7), facts: MaterialCandidateFacts{PublishedAt: at(-6 * 24 * time.Hour)}, want: true},
		{name: "recent days outside", condition: condition("published_at", "recent_days", 7), facts: MaterialCandidateFacts{PublishedAt: at(-8 * 24 * time.Hour)}, want: false},
		{name: "gte boundary", condition: condition("published_at", "gte", "2026-10-01T00:00:00Z"), facts: MaterialCandidateFacts{PublishedAt: at(-19 * 24 * time.Hour)}, want: true},
		{name: "gte before", condition: condition("published_at", "gte", "2026-10-01T00:00:00Z"), facts: MaterialCandidateFacts{PublishedAt: at(-20 * 24 * time.Hour)}, want: false},
		{name: "lt boundary", condition: condition("published_at", "lt", "2026-10-01T00:00:00Z"), facts: MaterialCandidateFacts{PublishedAt: at(-19 * 24 * time.Hour)}, want: false},
		{name: "lt before", condition: condition("published_at", "lt", "2026-10-01T00:00:00Z"), facts: MaterialCandidateFacts{PublishedAt: at(-20*24*time.Hour - time.Second)}, want: true},
		{name: "missing published_at", condition: condition("published_at", "recent_days", 7), facts: MaterialCandidateFacts{}, want: false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := materialConditionMatches(test.condition, test.facts, now); got != test.want {
				t.Fatalf("got %v want %v", got, test.want)
			}
		})
	}
}
