package model

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// MaterialRuleGroup mirrors the compose strategy's priority group shape so the
// two editors read the same. Content pool keeps its own copy: importing
// production/model from this module would couple two bounded contexts for one
// small predicate.
type MaterialRuleGroup struct {
	Priority   int                 `json:"priority"`
	Action     string              `json:"action"`
	Logic      string              `json:"logic"`
	Conditions []MaterialCondition `json:"conditions"`
}

type MaterialCondition struct {
	Field    string          `json:"field"`
	Operator string          `json:"operator"`
	Value    json.RawMessage `json:"value"`
}

// MaterialCandidateFacts is the source projection a rule may inspect. A
// missing publication time never satisfies a date condition.
type MaterialCandidateFacts struct {
	PublishedAt   *time.Time
	LikeCount     int64
	FavoriteCount int64
}

// ValidateMaterialRules accepts an empty list: a strategy with no rule groups
// only discovers content and never materializes it. Non-empty lists follow the
// compose rule: contiguous priorities, a valid action/logic, at least one
// condition per group and at least one match group.
func ValidateMaterialRules(groups []MaterialRuleGroup) error {
	if len(groups) == 0 {
		return nil
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
			if err := ValidateMaterialCondition(condition); err != nil {
				return err
			}
		}
	}
	if !hasMatch {
		return fmt.Errorf("at least one match group is required")
	}
	return nil
}

func ValidateMaterialCondition(condition MaterialCondition) error {
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

// ResolveMaterialRules returns the rules a config actually means. New configs
// carry material_rules; historical configs and frozen snapshots still carry the
// old auto_material/threshold pair and are translated on read.
func ResolveMaterialRules(config map[string]any) ([]MaterialRuleGroup, error) {
	if config == nil {
		return nil, nil
	}
	if raw, exists := config["material_rules"]; exists {
		groups, err := decodeMaterialRules(raw)
		if err != nil {
			return nil, err
		}
		if err := ValidateMaterialRules(groups); err != nil {
			return nil, err
		}
		return groups, nil
	}
	return LegacyMaterialRules(config), nil
}

func decodeMaterialRules(raw any) ([]MaterialRuleGroup, error) {
	if raw == nil {
		return nil, nil
	}
	if groups, ok := raw.([]MaterialRuleGroup); ok {
		return groups, nil
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("material_rules are invalid: %w", err)
	}
	var groups []MaterialRuleGroup
	if err := json.Unmarshal(encoded, &groups); err != nil {
		return nil, fmt.Errorf("material_rules are invalid: %w", err)
	}
	return groups, nil
}

// LegacyMaterialRules translates the pre-rule-group config. A disabled switch
// or two zero thresholds means "never auto-materialize".
func LegacyMaterialRules(config map[string]any) []MaterialRuleGroup {
	if !materialBool(config["auto_material"]) {
		return nil
	}
	likeThreshold := materialInt64(config["like_threshold"])
	favoriteThreshold := materialInt64(config["favorite_threshold"])
	conditions := make([]MaterialCondition, 0, 2)
	if likeThreshold > 0 {
		conditions = append(conditions, MaterialCondition{Field: "like_count", Operator: "gte", Value: rawInt64(likeThreshold)})
	}
	if favoriteThreshold > 0 {
		conditions = append(conditions, MaterialCondition{Field: "favorite_count", Operator: "gte", Value: rawInt64(favoriteThreshold)})
	}
	if len(conditions) == 0 {
		return nil
	}
	logic := "all"
	if strings.EqualFold(strings.TrimSpace(fmt.Sprint(config["material_rule"])), "OR") {
		logic = "any"
	}
	return []MaterialRuleGroup{{Priority: 1, Action: "match", Logic: logic, Conditions: conditions}}
}

// NormalizeMaterialConfig validates the config and rewrites it to the single
// current shape. The old keys are removed so a later read cannot disagree with
// the rules that were actually saved.
func NormalizeMaterialConfig(config map[string]any) bool {
	if config == nil {
		return false
	}
	groups, err := ResolveMaterialRules(config)
	if err != nil {
		return false
	}
	encoded, err := json.Marshal(groups)
	if err != nil {
		return false
	}
	var normalized []any
	if err := json.Unmarshal(encoded, &normalized); err != nil {
		return false
	}
	if normalized == nil {
		normalized = []any{}
	}
	config["material_rules"] = normalized
	delete(config, "auto_material")
	delete(config, "material_rule")
	delete(config, "like_threshold")
	delete(config, "favorite_threshold")
	return true
}

// MaterialGroupPredicate answers one group. all is the identity element for
// AND and any for OR, the same way the compose predicate works.
func MaterialGroupPredicate(group MaterialRuleGroup, facts MaterialCandidateFacts, now time.Time) bool {
	result := group.Logic == "all"
	for _, condition := range group.Conditions {
		matches := materialConditionMatches(condition, facts, now)
		if group.Logic == "all" {
			result = result && matches
		} else {
			result = result || matches
		}
	}
	return result
}

// ShouldAutoMaterialize evaluates groups in priority order: the first matching
// group decides. A filter match excludes the item, a match match includes it,
// and no match excludes it.
func ShouldAutoMaterialize(rules []MaterialRuleGroup, facts MaterialCandidateFacts, now time.Time) bool {
	ordered := append([]MaterialRuleGroup(nil), rules...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Priority < ordered[j].Priority })
	for _, group := range ordered {
		if MaterialGroupPredicate(group, facts, now) {
			return group.Action == "match"
		}
	}
	return false
}

func materialConditionMatches(condition MaterialCondition, facts MaterialCandidateFacts, now time.Time) bool {
	if condition.Field == "published_at" {
		if facts.PublishedAt == nil {
			return false
		}
		if condition.Operator == "recent_days" || condition.Operator == "recent_hours" {
			var amount int
			_ = json.Unmarshal(condition.Value, &amount)
			duration := time.Duration(amount) * time.Hour
			if condition.Operator == "recent_days" {
				duration *= 24
			}
			start := now.Add(-duration)
			return !facts.PublishedAt.Before(start) && !facts.PublishedAt.After(now)
		}
		var text string
		_ = json.Unmarshal(condition.Value, &text)
		boundary, _ := time.Parse(time.RFC3339, text)
		if condition.Operator == "gte" {
			return !facts.PublishedAt.Before(boundary)
		}
		return facts.PublishedAt.Before(boundary)
	}
	var threshold int64
	_ = json.Unmarshal(condition.Value, &threshold)
	value := facts.LikeCount
	if condition.Field == "favorite_count" {
		value = facts.FavoriteCount
	}
	switch condition.Operator {
	case "eq":
		return value == threshold
	case "gte":
		return value >= threshold
	default:
		return value <= threshold
	}
}

func rawInt64(value int64) json.RawMessage {
	return json.RawMessage(strconv.FormatInt(value, 10))
}

func materialBool(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	case float64:
		return typed != 0
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case json.Number:
		parsed, err := typed.Int64()
		return err == nil && parsed != 0
	default:
		return false
	}
}

func materialInt64(value any) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0
		}
		return parsed
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}
