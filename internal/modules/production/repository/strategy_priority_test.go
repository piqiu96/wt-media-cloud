package repository

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
)

func priorityRule(priority int, action, field, operator string, value json.RawMessage) model.ComposePriorityRule {
	return model.ComposePriorityRule{Priority: priority, Action: action, Logic: "all", Conditions: []model.ComposeCondition{{Field: field, Operator: operator, Value: value}}}
}

func TestComposePrioritySelectionFiltersBeforeMatchingAndFillsFromLowerPriority(t *testing.T) {
	rules := []model.ComposePriorityRule{
		priorityRule(3, "match", "like_count", "gte", json.RawMessage(`0`)),
		priorityRule(1, "filter", "like_count", "gte", json.RawMessage(`100`)),
		priorityRule(2, "match", "favorite_count", "gte", json.RawMessage(`1`)),
	}
	pool := []scheduleCandidate{
		{MaterialID: 1, LikeCount: 100},
		{MaterialID: 2, FavoriteCount: 1},
		{MaterialID: 3},
		{MaterialID: 4},
	}
	selected := make([]int64, 0)
	created, skipped, err := executeComposePriorityRules(rules, pool, time.Now(), 2, 2, func(candidate scheduleCandidate) (bool, error) {
		selected = append(selected, candidate.MaterialID)
		return true, nil
	})
	if err != nil || created != 2 || !reflect.DeepEqual(selected, []int64{2, 3}) || len(skipped) != 2 || skipped[0].Reason != "filtered" || skipped[0].MaterialID != 1 || skipped[1].Reason != "daily_limit" {
		t.Fatalf("created=%d selected=%v skipped=%+v err=%v", created, selected, skipped, err)
	}
}

func TestComposePrioritySelectionQuotaAndItemFailure(t *testing.T) {
	rules := []model.ComposePriorityRule{priorityRule(1, "match", "like_count", "gte", json.RawMessage(`0`))}
	pool := []scheduleCandidate{{MaterialID: 1}, {MaterialID: 2}, {MaterialID: 3}}
	for _, tc := range []struct {
		name          string
		single, daily int
		failID        int64
		wantCreated   int
		wantReason    string
	}{
		{"single limit", 1, 3, 0, 1, "per_run_limit"},
		{"daily limit reached", 3, 0, 0, 0, "daily_limit"},
		{"daily remainder", 3, 1, 0, 1, "daily_limit"},
		{"one item fails", 2, 2, 1, 2, "task_create_failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			selected := make([]int64, 0)
			created, skips, err := executeComposePriorityRules(rules, pool, time.Now(), tc.single, tc.daily, func(candidate scheduleCandidate) (bool, error) {
				if candidate.MaterialID == tc.failID {
					return false, nil
				}
				selected = append(selected, candidate.MaterialID)
				return true, nil
			})
			if err != nil || created != tc.wantCreated || len(skips) == 0 || skips[0].Reason != tc.wantReason {
				t.Fatalf("created=%d selected=%v skips=%+v err=%v", created, selected, skips, err)
			}
			if tc.failID != 0 && !reflect.DeepEqual(selected, []int64{2, 3}) {
				t.Fatalf("successful tasks after first failure=%v", selected)
			}
		})
	}
}
