package repository_test

import (
	"errors"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	repo "github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
	service "github.com/wt-media/wt-media-cloud/internal/modules/publish/service"
	"testing"
)

func TestPlanReadScopeAcrossCountDetailsRecordsAndExactRequest(t *testing.T) {
	db := isolated(t)
	r := store(db)
	team := identity.TeamID(7)
	owner := identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, CategoryIDs: []identity.CategoryID{1, 2}}
	s := service.Service{PlanRepo: r, BatchCategory: func(id identity.CategoryID) (identity.Category, bool, error) {
		return identity.Category{ID: id, Enabled: true}, true, nil
	}}
	in := service.CreatePlanInput{RequestKey: "scope", PlanInput: service.PlanInput{Name: "scope", DateStart: "2026-11-25", DateEnd: "2026-11-26", Goals: []model.PlanGoal{{Platform: "baidu", CategoryID: 1, TargetCount: 1}, {Platform: "baidu", CategoryID: 2, TargetCount: 1}}, Batches: []model.PlanStructureBatch{{RowKey: "a", Name: "a", Platform: "baidu", GroupOrder: 1, BatchOrder: 1, Targets: []model.BatchTarget{{CategoryID: 1, TargetCount: 1}, {CategoryID: 2, TargetCount: 1}}}}}}
	created, e := s.CreatePlan(owner, in)
	if e != nil {
		t.Fatal(e)
	}
	id := created.Plan.ID
	restricted := owner
	restricted.CategoryIDs = []identity.CategoryID{1}
	for _, a := range []identity.PublicUser{restricted, {ID: 10, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, CategoryIDs: owner.CategoryIDs}} {
		page, e := s.ListPlans(a, repo.PlanFilter{Page: 1, PageSize: 10})
		if e != nil || page.Total != 0 {
			t.Fatalf("scope leaked count %v %v", page, e)
		}
		page, e = s.ListPlans(a, repo.PlanFilter{Page: 1, PageSize: 10, RequestKey: "scope"})
		if e != nil || page.Total != 0 {
			t.Fatalf("request leaked %v %v", page, e)
		}
		if _, e = s.GetPlan(a, id); !errors.Is(e, service.ErrNotFound) {
			t.Fatalf("detail leak %v", e)
		}
		if _, e = s.PlanBatches(a, id, 1, 10, "", nil); !errors.Is(e, service.ErrNotFound) {
			t.Fatalf("batches leak %v", e)
		}
		if _, e = s.PlanRecords(a, id, 1, 10); !errors.Is(e, service.ErrNotFound) {
			t.Fatalf("record leak %v", e)
		}
	}
	senior := owner
	senior.ID = 11
	senior.Role = identity.RoleSeniorOperator
	if _, e = s.GetPlan(senior, id); e != nil {
		t.Fatal(e)
	}
	if _, e = s.EditPlan(senior, id, service.StructurePlanInput{PlanInput: in.PlanInput, ExpectedRevision: 1}); !errors.Is(e, service.ErrNotFound) {
		t.Fatalf("senior executed other owner %v", e)
	}
	admin := owner
	admin.Role = identity.RoleAdmin
	if _, e = s.CreatePlan(admin, in); !errors.Is(e, service.ErrNotFound) {
		t.Fatalf("admin writes %v", e)
	}
	if _, e = s.CopyPlan(owner, id, 1); !errors.Is(e, service.ErrConflict) {
		t.Fatalf("copied live plan %v", e)
	}
	if e = db.Model(&model.Plan{}).Where("id=?", id).Update("status", "ended").Error; e != nil {
		t.Fatal(e)
	}
	copied, e := s.CopyPlan(owner, id, 1)
	if e != nil {
		t.Fatal(e)
	}
	white := copied.(map[string]any)
	if _, ok := white["date_start"]; ok {
		t.Fatal("copied old period")
	}
	if _, ok := white["request_key"]; ok {
		t.Fatal("copied request key")
	}
	for _, b := range white["batches"].([]model.PlanStructureBatch) {
		if b.BatchID != nil {
			t.Fatal("copied original batch identity")
		}
	}
	// Same-key recovery remains valid even if current category preparation became unavailable.
	s.BatchCategory = func(identity.CategoryID) (identity.Category, bool, error) { return identity.Category{}, false, nil }
	recovered, e := s.CreatePlan(owner, in)
	if e != nil || recovered.Plan.ID != id {
		t.Fatalf("original recovery %v", e)
	}
	in.Note = "changed"
	if _, e = s.CreatePlan(owner, in); !errors.Is(e, service.ErrConflict) {
		t.Fatalf("key reused %v", e)
	}
}
