package model

import (
	"encoding/json"
	"fmt"
	"time"
)

// ComposeCandidateFacts contains only the source facts exposed by the strategy
// condition contract. Missing publication time never satisfies a date condition.
type ComposeCandidateFacts struct {
	PublishedAt   *time.Time
	LikeCount     int64
	FavoriteCount int64
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

func ComposeGroupPredicate(group ComposePriorityRule, facts ComposeCandidateFacts, now time.Time) bool {
	result := group.Logic == "all"
	for _, condition := range group.Conditions {
		matches := composeConditionMatches(condition, facts, now)
		if group.Logic == "all" {
			result = result && matches
		} else {
			result = result || matches
		}
	}
	return result
}

func composeConditionMatches(condition ComposeCondition, facts ComposeCandidateFacts, now time.Time) bool {
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
