package repository_test

import (
	"context"
	"errors"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	repo "github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
	service "github.com/wt-media/wt-media-cloud/internal/modules/publish/service"
	"gorm.io/gorm"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPlanStartRejectsUnreadyOrUnconfirmedFutureBatchWithoutAnyPartialEffects(t *testing.T) {
	for _, missingConfirmation := range []bool{false, true} {
		name := "future_incomplete"
		if missingConfirmation {
			name = "missing_future_confirmation"
		}
		t.Run(name, func(t *testing.T) {
			s, p, bs, in, prepared := preparedPlan(t)
			if !missingConfirmation {
				future := bs[1]
				future.Config.Candidates[0].Content.Confirmed = false
				future.ConfigurationStatus = model.BatchIncomplete
				future.Readiness.Ready = false
				saved, e := s.EditBatch(future, future.Revision)
				if e != nil {
					t.Fatal(e)
				}
				in.BatchConfirmations[1].ExpectedRevision = saved.Revision
				current, rows, e := s.GetPlan(p.ID)
				if e != nil {
					t.Fatal(e)
				}
				if !rows[0].Readiness.Ready || rows[0].ConfigurationStatus != model.BatchReady || rows[1].Readiness.Ready || rows[1].ConfigurationStatus != model.BatchIncomplete {
					t.Fatal("fixture must have a genuinely ready first batch and incomplete future batch")
				}
				in.ExpectedRevision = current.Revision
			} else {
				in.BatchConfirmations = in.BatchConfirmations[:1]
			}
			if !model.ValidPlanStartCommand(in) {
				t.Fatal("negative case must remain a statically valid original command")
			}
			var beforePlan model.Plan
			var beforeBatches []model.Batch
			if e := s.DB.First(&beforePlan, p.ID).Error; e != nil {
				t.Fatal(e)
			}
			if e := s.DB.Where("plan_id=?", p.ID).Order("id").Find(&beforeBatches).Error; e != nil {
				t.Fatal(e)
			}
			var beforeAudits int64
			if e := s.DB.Table("audit_logs").Count(&beforeAudits).Error; e != nil {
				t.Fatal(e)
			}
			assertUnchanged := func(stage string) {
				t.Helper()
				var afterPlan model.Plan
				var afterBatches []model.Batch
				if e := s.DB.First(&afterPlan, p.ID).Error; e != nil {
					t.Fatal(e)
				}
				if e := s.DB.Where("plan_id=?", p.ID).Order("id").Find(&afterBatches).Error; e != nil {
					t.Fatal(e)
				}
				if !reflect.DeepEqual(beforePlan, afterPlan) || !reflect.DeepEqual(beforeBatches, afterBatches) {
					t.Fatalf("%s changed plan/batches or revision/configuration", stage)
				}
				if afterPlan.Status != "not_started" || afterPlan.StartedAt != nil || afterPlan.CurrentBatchID != nil || afterPlan.StartRequestKey != nil || afterPlan.StartRequestHash != nil || len(afterPlan.StartReceipt) != 0 || len(afterPlan.HandoffReceipts) != 0 {
					t.Fatalf("%s retained plan cursor/start/command journal", stage)
				}
				for _, b := range afterBatches {
					if b.PlanFrozenAt != nil || b.FrozenConfirmation != nil || b.StartedAt != nil || b.StartRequestKey != nil || b.StartRequestHash != nil || b.StartReceipt != nil || b.ExecutionStatus != model.BatchNotStarted {
						t.Fatalf("%s retained freeze/start/receipt in batch %d", stage, b.ID)
					}
				}
				for _, q := range []struct{ table, where string }{{"publication_task", "1=1"}, {"publication_task", "occupied_output_id IS NOT NULL"}, {"publication_task", "authorized_at IS NOT NULL"}, {"publication_attempt", "1=1"}, {"publication_profile_use", "1=1"}} {
					var n int64
					if e := s.DB.Table(q.table).Where(q.where).Count(&n).Error; e != nil || n != 0 {
						t.Fatalf("%s left %s/%s count=%d err=%v", stage, q.table, q.where, n, e)
					}
				}
				var audits int64
				if e := s.DB.Table("audit_logs").Count(&audits).Error; e != nil || audits != beforeAudits {
					t.Fatalf("%s persisted success audit/journal count=%d previous=%d err=%v", stage, audits, beforeAudits, e)
				}
			}
			if !missingConfirmation {
				team := identity.TeamID(7)
				owner := identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, CategoryIDs: []identity.CategoryID{1}}
				svc := service.Service{PlanRepo: s.Store, BatchRepo: s.Store, PlanExecutionRepo: s}
				if _, e := svc.StartPlan(context.Background(), owner, p.ID, in); !errors.Is(e, service.ErrConflict) {
					t.Fatalf("service admitted incomplete future batch: %v", e)
				}
				assertUnchanged("service rejection")
			}
			freezeWrites := 0
			if e := s.DB.Callback().Update().After("gorm:update").Register("verify_first_freeze_then_rollback", func(tx *gorm.DB) {
				if strings.Contains(tx.Statement.SQL.String(), "plan_frozen_at") {
					freezeWrites++
				}
			}); e != nil {
				t.Fatal(e)
			}
			if _, e := s.StartPlan(p.ID, 9, in, "readiness-negative", prepared[bs[0].ID], time.Now().UTC()); !errors.Is(e, repo.ErrConflict) {
				t.Fatalf("transaction admitted future batch: %v", e)
			}
			if freezeWrites != 1 {
				t.Fatalf("expected first batch freeze write before future rejection, got %d", freezeWrites)
			}
			assertUnchanged("transaction rollback")
		})
	}
}
