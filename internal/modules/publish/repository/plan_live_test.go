package repository_test

import (
	"errors"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	repo "github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
	"testing"
	"time"
)

func TestPlanSkeletonAtomicConservationIdentityAndFreeze(t *testing.T) {
	db := isolated(t)
	s := store(db)
	now := time.Now().UTC()
	p := model.Plan{TeamID: 7, OwnerID: 9, CreatedBy: 9, Name: "plan", DateStart: "2026-11-25", DateEnd: "2026-11-26", Goals: []model.PlanGoal{{Platform: "baidu", CategoryID: 1, TargetCount: 2}}, RequestKey: "original", RequestHash: "digest", CreatedAt: now, UpdatedAt: now}
	rows := []model.PlanStructureBatch{{RowKey: "one", Name: "first", Platform: "baidu", GroupOrder: 1, BatchOrder: 1, Targets: []model.BatchTarget{{CategoryID: 1, TargetCount: 1}}}, {RowKey: "two", Name: "second", Platform: "baidu", GroupOrder: 1, BatchOrder: 2, Targets: []model.BatchTarget{{CategoryID: 1, TargetCount: 1}}}}
	got, batches, e := s.CreatePlan(p, rows)
	if e != nil {
		t.Fatal(e)
	}
	replay, again, e := s.CreatePlan(p, rows)
	if e != nil || replay.ID != got.ID || len(again) != 2 {
		t.Fatalf("replay %v", e)
	}
	p.RequestHash = "other"
	if _, _, e = s.CreatePlan(p, rows); !errors.Is(e, repo.ErrConflict) {
		t.Fatalf("different input %v", e)
	}
	for _, b := range batches {
		if b.PlanID == nil || *b.PlanID != got.ID || b.PlanConfiguredAt != nil {
			t.Fatalf("invalid skeleton %+v", b)
		}
	}
	var n int64
	db.Model(&model.Task{}).Count(&n)
	if n != 0 {
		t.Fatal("skeleton created task")
	}
	actor := identity.PublicUser{ID: 9, Role: identity.RoleOperator, CategoryIDs: []identity.CategoryID{1}}
	_, n, e = s.ListBatches(repo.BatchFilter{Actor: actor, Page: 1, PageSize: 10})
	if e != nil || n != 0 {
		t.Fatalf("visible skeleton %d %v", n, e)
	}
	rows[0].BatchID = &batches[0].ID
	rows[1].BatchID = &batches[1].ID
	rows[1].Targets[0].TargetCount = 2
	if _, _, e = s.EditPlanStructure(got, got.Revision, rows); !errors.Is(e, repo.ErrConflict) {
		t.Fatalf("nonconservation %v", e)
	}
	saved, _, e := s.GetPlan(got.ID)
	if e != nil || saved.Revision != got.Revision {
		t.Fatal("partial structure")
	}
	rows[1].Targets[0].TargetCount = 1
	foreign := uint64(999999)
	rows[1].BatchID = &foreign
	if _, _, e = s.EditPlanStructure(got, got.Revision, rows); !errors.Is(e, repo.ErrNotFound) {
		t.Fatalf("foreign %v", e)
	}
	db.Model(&model.Plan{}).Where("id=?", got.ID).Update("started_at", now)
	if _, _, e = s.EditPlanStructure(got, got.Revision, rows); !errors.Is(e, repo.ErrParentBlocked) {
		t.Fatalf("freeze %v", e)
	}
	b := batches[0]
	if _, e = s.EditBatch(b, b.Revision); !errors.Is(e, repo.ErrParentBlocked) {
		t.Fatalf("configuration freeze %v", e)
	}
}

func TestPlanStructureTargetsReferencesAndTerminatedHistory(t *testing.T) {
	db := isolated(t)
	s := store(db)
	now := time.Now().UTC()
	p := model.Plan{TeamID: 7, OwnerID: 9, CreatedBy: 9, Name: "plan", DateStart: "2026-11-25", DateEnd: "2026-11-26", Goals: []model.PlanGoal{{Platform: "baidu", CategoryID: 1, TargetCount: 2}}, RequestKey: "p", RequestHash: "hash", CreatedAt: now, UpdatedAt: now}
	rows := []model.PlanStructureBatch{{RowKey: "one", Name: "first", Platform: "baidu", GroupOrder: 1, BatchOrder: 1, Targets: []model.BatchTarget{{CategoryID: 1, TargetCount: 1}}}, {RowKey: "two", Name: "second", Platform: "baidu", GroupOrder: 1, BatchOrder: 2, Targets: []model.BatchTarget{{CategoryID: 1, TargetCount: 1}}}}
	p, bs, e := s.CreatePlan(p, rows)
	if e != nil {
		t.Fatal(e)
	}
	if p.DateStart != "2026-11-25" || p.DateEnd != "2026-11-26" {
		t.Fatalf("date roundtrip %s/%s", p.DateStart, p.DateEnd)
	}
	for i := range rows {
		rows[i].BatchID = &bs[i].ID
	}
	b := bs[0]
	b.Config.Candidates = []model.BatchCandidate{{Content: model.Content{CoverFileID: 1, PortraitCoverFileID: 2, Confirmed: true}}}
	b.ConfigurationStatus = model.BatchReady
	b.Readiness.Ready = true
	b, e = s.EditBatch(b, b.Revision)
	if e != nil {
		t.Fatal(e)
	}
	if b.PlanID == nil || b.PlanConfiguredAt == nil {
		t.Fatalf("configured %+v", b)
	}
	var refs int64
	db.Table("publication_batch_file").Where("batch_id=?", b.ID).Count(&refs)
	if refs != 2 {
		t.Fatalf("cover refs %d", refs)
	}
	p, _, e = s.GetPlan(p.ID)
	if e != nil {
		t.Fatal(e)
	}
	rows[0].Targets[0].TargetCount = 2
	rows[1].Targets[0].TargetCount = 1
	p.Goals[0].TargetCount = 3
	p, bs, e = s.EditPlanStructure(p, p.Revision, rows)
	if e != nil {
		t.Fatal(e)
	}
	b, e = s.GetBatch(b.ID)
	if e != nil || b.ConfigurationStatus != model.BatchIncomplete || b.Readiness.Ready || b.Config.Candidates[0].Content.Confirmed || b.PlanConfiguredAt != nil {
		t.Fatalf("stale readiness %+v %v", b, e)
	}
	// A failed audit must roll back deleted configuration and its file references.
	if e = db.Exec("CREATE TRIGGER reject_plan_audit BEFORE INSERT ON audit_logs FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='audit unavailable'").Error; e != nil {
		t.Fatal(e)
	}
	shortened := []model.PlanStructureBatch{rows[1]}
	shortened[0].BatchOrder = 1
	shortened[0].Targets = []model.BatchTarget{{CategoryID: 1, TargetCount: 3}}
	if _, _, e = s.EditPlanStructure(p, p.Revision, shortened); e == nil {
		t.Fatal("audit failure accepted")
	}
	db.Table("publication_batch_file").Where("batch_id=?", b.ID).Count(&refs)
	if refs != 2 {
		t.Fatalf("rollback lost refs %d", refs)
	}
	if e = db.Exec("DROP TRIGGER reject_plan_audit").Error; e != nil {
		t.Fatal(e)
	}
	if e = db.Model(&model.Batch{}).Where("id=?", bs[0].ID).Update("execution_status", model.BatchCancelled).Error; e != nil {
		t.Fatal(e)
	}
	if _, _, e = s.EditPlanStructure(p, p.Revision, shortened); !errors.Is(e, repo.ErrParentBlocked) {
		t.Fatalf("removed terminated history %v", e)
	}
	b.ExecutionStatus = model.BatchNotStarted
	if _, e = s.EditBatch(b, b.Revision); e == nil {
		t.Fatal("revived terminated skeleton")
	}
}

func TestPlanMigrationKeepsLegacyRowsAndCanReverseInDisposableSchema(t *testing.T) {
	db := isolatedBeforeBatchExecution(t)
	applyFileMigration(t, db, "20261125_117_publication_batch_execution.sql")
	s := store(db)
	if e := db.Exec(`INSERT INTO publication_batch(team_id,operator_id,created_by,name,platform,targets_json,config_json,readiness_json,candidates_count,concurrency_limit,group_interval_seconds,between_group_policy,exception_task_limit,configuration_status,execution_status,revision,request_key,request_hash,created_at,updated_at) VALUES(7,9,9,'legacy','baidu','[{"category_id":"1","target_count":1}]','{"candidates":[]}','{}',0,1,0,'auto_continue',1,'incomplete','not_started',1,'old','old',NOW(6),NOW(6))`).Error; e != nil {
		t.Fatal(e)
	}
	var b model.Batch
	var e error
	db.Raw("SELECT id FROM publication_batch WHERE request_key='old'").Scan(&b.ID)
	applyFileMigration(t, db, "20261125_118_publication_plan.sql")
	b, e = s.GetBatch(b.ID)
	if e != nil || b.PlanID != nil || b.Revision != 1 {
		t.Fatalf("legacy changed %+v %v", b, e)
	}
	b.Name = "edited"
	if _, e = s.EditBatch(b, 1); e != nil {
		t.Fatal(e)
	}
	for _, sql := range []string{"ALTER TABLE publication_task DROP COLUMN latest_manual_verification_json", "ALTER TABLE publication_batch DROP FOREIGN KEY fk_publication_batch_plan, DROP INDEX uq_publication_plan_row, DROP INDEX uq_publication_plan_order, DROP COLUMN plan_id, DROP COLUMN plan_row_key, DROP COLUMN plan_group_order, DROP COLUMN plan_batch_order, DROP COLUMN plan_configured_at, DROP COLUMN plan_frozen_at, DROP COLUMN frozen_confirmation_json, DROP COLUMN handed_off_at, DROP COLUMN handoff_basis_fingerprint", "DROP TABLE publication_plan"} {
		if e = db.Exec(sql).Error; e != nil {
			t.Fatal(e)
		}
	}
	var name string
	if e = db.Table("publication_batch").Select("name").Where("id=?", b.ID).Scan(&name).Error; e != nil || name != "edited" {
		t.Fatalf("reverse changed legacy %s %v", name, e)
	}
}

func TestPlanStableIdentityAndConfigurationRevisionCompetition(t *testing.T) {
	db := isolated(t)
	s := store(db)
	p := model.Plan{TeamID: 7, OwnerID: 9, CreatedBy: 9, Name: "p", DateStart: "2026-11-25", DateEnd: "2026-11-26", Goals: []model.PlanGoal{{Platform: "baidu", CategoryID: 1, TargetCount: 2}}, RequestKey: "p", RequestHash: "h"}
	rows := []model.PlanStructureBatch{{RowKey: "a", Name: "a", Platform: "baidu", GroupOrder: 1, BatchOrder: 1, Targets: []model.BatchTarget{{CategoryID: 1, TargetCount: 1}}}, {RowKey: "b", Name: "b", Platform: "baidu", GroupOrder: 1, BatchOrder: 2, Targets: []model.BatchTarget{{CategoryID: 1, TargetCount: 1}}}}
	p, bs, e := s.CreatePlan(p, rows)
	if e != nil {
		t.Fatal(e)
	}
	for i := range rows {
		rows[i].BatchID = &bs[i].ID
	}
	bad := append([]model.PlanStructureBatch(nil), rows...)
	bad[1].BatchID = bad[0].BatchID
	if _, _, e = s.EditPlanStructure(p, p.Revision, bad); !errors.Is(e, repo.ErrConflict) {
		t.Fatalf("duplicate id %v", e)
	}
	bad = append([]model.PlanStructureBatch(nil), rows...)
	bad[1].RowKey = bad[0].RowKey
	if _, _, e = s.EditPlanStructure(p, p.Revision, bad); !errors.Is(e, repo.ErrConflict) {
		t.Fatalf("duplicate key %v", e)
	}
	// Both start from exactly the same old revisions. Plan->batch lock order
	// makes a structure save invalidate concurrent whole-batch replacement.
	outcomes := make(chan error, 2)
	go func() { _, _, e := s.EditPlanStructure(p, p.Revision, rows); outcomes <- e }()
	go func() { _, e := s.EditBatch(bs[0], bs[0].Revision); outcomes <- e }()
	first, second := <-outcomes, <-outcomes
	// EditBatch may precede structure; structure must detect that stale saved basis.
	if (first == nil) == (second == nil) {
		t.Fatalf("expected one winner: %v / %v", first, second)
	}
}
