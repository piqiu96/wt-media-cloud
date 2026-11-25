package service

import (
	"context"
	"errors"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"strings"
	"testing"
)

func TestPlanOwnScopeAndStrictConservation(t *testing.T) {
	team := identity.TeamID(7)
	a := identity.PublicUser{ID: 9, TeamID: &team, Status: identity.UserStatusEnabled, Role: identity.RoleOperator, CategoryIDs: []identity.CategoryID{1}}
	in := PlanInput{Name: "plan", DateStart: "2026-11-25", DateEnd: "2026-11-26", Goals: []model.PlanGoal{{Platform: "baidu", CategoryID: 1, TargetCount: 1}}, Batches: []model.PlanStructureBatch{{RowKey: "r", Name: "first", Platform: "baidu", GroupOrder: 1, BatchOrder: 1, Targets: []model.BatchTarget{{CategoryID: 1, TargetCount: 1}}}}}
	s := Service{BatchCategory: func(id identity.CategoryID) (identity.Category, bool, error) {
		return identity.Category{ID: id, Enabled: true}, true, nil
	}}
	if e := s.validatePlan(a, in); e != nil {
		t.Fatal(e)
	}
	a.Role = identity.RoleAdmin
	if e := s.validatePlan(a, in); !errors.Is(e, ErrNotFound) {
		t.Fatalf("admin writes %v", e)
	}
	a.Role = identity.RoleOperator
	in.Goals[0].CategoryID = 2
	if e := s.validatePlan(a, in); !errors.Is(e, ErrNotFound) {
		t.Fatalf("foreign category %v", e)
	}
}
func TestPlanRecommendationRejectsInventedOrPartialModelStructure(t *testing.T) {
	team := identity.TeamID(7)
	a := identity.PublicUser{ID: 9, TeamID: &team, Status: identity.UserStatusEnabled, Role: identity.RoleOperator, CategoryIDs: []identity.CategoryID{1}}
	s := Service{BatchCategory: func(id identity.CategoryID) (identity.Category, bool, error) {
		return identity.Category{ID: id, Enabled: true}, true, nil
	}, PlanResourceCounts: func(context.Context, identity.PublicUser, string, int64) (BatchResource, error) {
		return BatchResource{AccountCount: 1, AvailableOutputCount: 2}, nil
	}, GenerateJSON: func(context.Context, string, string) (string, error) { return `{"batches":[]}`, nil }}
	run, e := s.PreparePlanRecommendation(context.Background(), a, PlanRecommendationInput{Goals: []model.PlanGoal{{Platform: "baidu", CategoryID: 1, TargetCount: 1}}, Batches: []model.PlanStructureBatch{}, Instruction: "拆批"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = run(context.Background()); e == nil {
		t.Fatal("accepted empty fake recommendation")
	}
}

func TestPlanRecommendationPromptIsBoundAndNeverAdoptsInvalidOutput(t *testing.T) {
	team := identity.TeamID(7)
	a := identity.PublicUser{ID: 9, TeamID: &team, Status: identity.UserStatusEnabled, Role: identity.RoleOperator, CategoryIDs: []identity.CategoryID{1}}
	in := PlanRecommendationInput{Goals: []model.PlanGoal{{Platform: "baidu", CategoryID: 1, TargetCount: 2}}, Batches: []model.PlanStructureBatch{}, Instruction: "两批"}
	current := `{"batches":[{"row_key":"one","name":"一","platform":"baidu","group_order":1,"batch_order":1,"targets":[{"category_id":"1","target_count":2}]}]}`
	var prompt string
	calls := 0
	s := Service{BatchCategory: func(id identity.CategoryID) (identity.Category, bool, error) {
		return identity.Category{ID: id, Enabled: true}, true, nil
	}, PlanResourceCounts: func(context.Context, identity.PublicUser, string, int64) (BatchResource, error) {
		return BatchResource{AccountCount: 1, AvailableOutputCount: 2}, nil
	}, GenerateJSON: func(_ context.Context, system, user string) (string, error) {
		calls++
		prompt = user
		return current, nil
	}}
	run, e := s.PreparePlanRecommendation(context.Background(), a, in)
	if e != nil {
		t.Fatal(e)
	}
	// Later edits cannot replace the basis already authorized and serialized.
	in.Goals[0].TargetCount = 900
	result, e := run(context.Background())
	if e != nil || len(result.Batches) != 1 {
		t.Fatalf("valid model %v", e)
	}
	if strings.Contains(prompt, "900") || strings.Contains(prompt, "owner") || strings.Contains(prompt, "profile") || strings.Contains(prompt, "account_id") {
		t.Fatalf("invalid prompt %s", prompt)
	}
	for _, raw := range []string{"not JSON", `{"batches":[]}`, strings.Replace(current, `"target_count":2`, `"target_count":1`, 1), strings.Replace(current, `"category_id":"1"`, `"category_id":"2"`, 1), current + `{}`, strings.Replace(current, `"row_key":"one"`, `"row_key":"one","batch_id":"8"`, 1)} {
		current = raw
		if _, e = run(context.Background()); e == nil {
			t.Fatalf("adopted invalid %s", raw)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	before := calls
	if _, e = run(ctx); e == nil || calls != before {
		t.Fatal("cancelled generation ran model")
	}
	s.PlanResourceCounts = func(context.Context, identity.PublicUser, string, int64) (BatchResource, error) {
		return BatchResource{}, errors.New("inventory offline")
	}
	in.Goals[0].TargetCount = 2
	if _, e = s.PreparePlanRecommendation(context.Background(), a, in); e == nil {
		t.Fatal("inventory failure treated as zero")
	}
}
