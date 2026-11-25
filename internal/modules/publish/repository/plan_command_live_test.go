package repository_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	repo "github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
	service "github.com/wt-media/wt-media-cloud/internal/modules/publish/service"
	"strings"
	"testing"
	"time"
)

func TestPlanOriginalCommandNonApplicationRequiresLockedOldRevisionAndNoReceipt(t *testing.T) {
	s, p, bs, in, tasks := preparedPlan(t)
	old := in
	if e := s.DB.Model(&model.Plan{}).Where("id=?", p.ID).Update("revision", p.Revision+1).Error; e != nil {
		t.Fatal(e)
	}
	_, e := s.ProbePlanStart(p.ID, 9, old, "original")
	var proof *repo.PlanCommandNotApplied
	if !errors.As(e, &proof) || proof.CommandApplied || proof.RequestKey != old.RequestKey || proof.ExpectedRevision != old.ExpectedRevision || proof.CurrentRevision != p.Revision+1 {
		t.Fatalf("missing exact proof: %#v %v", proof, e)
	}
	if _, e = s.StartPlan(p.ID, 9, old, "original", tasks[bs[0].ID], time.Now()); !errors.As(e, &proof) {
		t.Fatalf("commit path lost proof %v", e)
	}
	in.ExpectedRevision++
	first, e := s.StartPlan(p.ID, 9, in, "winner", tasks[bs[0].ID], time.Now())
	if e != nil {
		t.Fatal(e)
	}
	if replay, e := s.ProbePlanStart(p.ID, 9, in, "winner"); e != nil || replay == nil || replay.Revision != first.Revision {
		t.Fatalf("receipt not replayed %v %v", replay, e)
	}
	if _, e = s.ProbePlanStart(p.ID, 9, in, "different"); errors.As(e, &proof) || !errors.Is(e, repo.ErrConflict) {
		t.Fatalf("same key changed body must be unknown %v", e)
	}
	old.RequestKey = "other-tab"
	if _, e = s.ProbePlanStart(p.ID, 9, old, "other"); !errors.As(e, &proof) {
		t.Fatalf("different key older request %v", e)
	}
	advance := model.PlanAdvanceInput{RequestKey: "next", ExpectedRevision: first.Revision, CurrentBatchID: bs[0].ID, NextBatchID: bs[1].ID, ExpectedBasisFingerprint: strings.Repeat("a", 64), Decision: "continue", Reason: "继续"}
	if e = s.DB.Model(&model.Plan{}).Where("id=?", p.ID).Update("revision", first.Revision+1).Error; e != nil {
		t.Fatal(e)
	}
	if _, e = s.ProbePlanAdvance(p.ID, 9, advance, "next-digest", false); !errors.As(e, &proof) || proof.Command != "confirm-next" {
		t.Fatalf("old advance no proof %v", e)
	}
	if _, e = s.ProbePlanAdvance(p.ID, 9, advance, "next-digest", true); errors.As(e, &proof) {
		t.Fatalf("automatic must not sign proof %v", e)
	}
	for _, bad := range []string{"null", "{", `{"old":{"Digest":"x","Result":{}}}`} {
		if e = s.DB.Model(&model.Plan{}).Where("id=?", p.ID).Update("handoff_receipts_json", bad).Error; e != nil {
			if bad == "{" {
				continue
			}
			t.Fatal(e)
		}
		if _, e = s.ProbePlanAdvance(p.ID, 9, advance, "next-digest", false); errors.As(e, &proof) {
			t.Fatalf("corrupt map %q signed false", bad)
		}
	}
	if e = s.DB.Model(&model.Plan{}).Where("id=?", p.ID).Update("handoff_receipts_json", nil).Error; e != nil {
		t.Fatal(e)
	}
	if _, e = s.ProbePlanAdvance(p.ID, 9, advance, "next-digest", false); errors.As(e, &proof) {
		t.Fatal("missing map signed false")
	}
}

func TestPlanOriginalAdvanceReceiptAlwaysWinsAndCommitRechecksAfterProbe(t *testing.T) {
	s, p, bs, tasks, now := settledPlan(t, 8, 2)
	state, e := s.PlanProgress(p.ID, now)
	if e != nil {
		t.Fatal(e)
	}
	in := model.PlanAdvanceInput{RequestKey: "next-original", ExpectedRevision: p.Revision, CurrentBatchID: bs[0].ID, NextBatchID: bs[1].ID, ExpectedBasisFingerprint: state.Basis.Fingerprint, Decision: "continue", Reason: "明确继续"}
	if r, e := s.ProbePlanAdvance(p.ID, 9, in, "original-digest", false); e != nil || r != nil {
		t.Fatalf("fresh probe %v %v", r, e)
	}
	result, e := s.AdvancePlan(p.ID, 9, in, "original-digest", tasks[bs[1].ID], false, now)
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.ProbePlanAdvance(p.ID, 9, in, "original-digest", false)
	if e != nil || r == nil || r.Revision != result.Revision {
		t.Fatalf("lost replay %v %v", r, e)
	}
	var proof *repo.PlanCommandNotApplied
	if _, e = s.ProbePlanAdvance(p.ID, 9, in, "different", false); errors.As(e, &proof) || !errors.Is(e, repo.ErrConflict) {
		t.Fatalf("changed samekey not unknown %v", e)
	}
	in.RequestKey = "other-tab"
	if _, e = s.AdvancePlan(p.ID, 9, in, "other", nil, false, now); !errors.As(e, &proof) {
		t.Fatalf("advance commit did not check race %v", e)
	}
}

func TestPlanOriginalCommandProbeWaitsForParentThenProvesLosingRevision(t *testing.T) {
	s, p, _, in, _ := preparedPlan(t)
	tx := s.DB.Begin()
	defer tx.Rollback()
	if e := tx.Exec("SELECT id FROM publication_plan WHERE id=? FOR UPDATE", p.ID).Error; e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { _, e := s.ProbePlanStart(p.ID, 9, in, "digest"); done <- e }()
	select {
	case e := <-done:
		t.Fatalf("probe did not wait %v", e)
	case <-time.After(50 * time.Millisecond):
	}
	if e := tx.Model(&model.Plan{}).Where("id=?", p.ID).Update("revision", p.Revision+1).Error; e != nil {
		t.Fatal(e)
	}
	if e := tx.Commit().Error; e != nil {
		t.Fatal(e)
	}
	select {
	case e := <-done:
		var proof *repo.PlanCommandNotApplied
		if !errors.As(e, &proof) || proof.CurrentRevision != p.Revision+1 {
			t.Fatalf("stale lock snapshot %v", e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("probe hung")
	}
}

func TestPlanOriginalServiceChecksScopeAndReplaysBeforeUnavailableMutableResources(t *testing.T) {
	s, p, bs, in, tasks := preparedPlan(t)
	team := identity.TeamID(7)
	owner := identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, CategoryIDs: []identity.CategoryID{1}}
	svc := service.Service{PlanRepo: s.Store, BatchRepo: s.Store, PlanExecutionRepo: s}
	if e := s.DB.Model(&model.Plan{}).Where("id=?", p.ID).Update("revision", p.Revision+1).Error; e != nil {
		t.Fatal(e)
	}
	var proof *repo.PlanCommandNotApplied
	if _, e := svc.StartPlan(context.Background(), owner, p.ID, in); !errors.As(e, &proof) {
		t.Fatalf("old static-valid request touched unavailable resources: %v", e)
	}
	other := owner
	other.ID = 10
	for _, a := range []identity.PublicUser{other, func() identity.PublicUser { v := owner; v.Role = identity.RoleAdmin; return v }(), func() identity.PublicUser { v := owner; v.CategoryIDs = nil; return v }()} {
		if _, e := svc.StartPlan(context.Background(), a, p.ID, in); errors.As(e, &proof) || !errors.Is(e, service.ErrNotFound) {
			t.Fatalf("unauthorized proof %v", e)
		}
	}
	malformed := in
	malformed.FirstBatchID = 999
	if _, e := svc.StartPlan(context.Background(), owner, p.ID, malformed); errors.As(e, &proof) || !errors.Is(e, service.ErrInvalid) {
		t.Fatalf("invalid static signed proof %v", e)
	}
	in.ExpectedRevision++
	canonical := in
	canonical.BatchConfirmations = append([]model.PlanBatchConfirmation{}, in.BatchConfirmations...)
	for i, c := range canonical.BatchConfirmations {
		c.ExcludedRowKeys = append([]string(nil), c.ExcludedRowKeys...)
		canonical.BatchConfirmations[i] = c
	}
	raw, _ := json.Marshal(canonical)
	hash := sha256.Sum256(raw)
	digest := hex.EncodeToString(hash[:])
	result, e := s.StartPlan(p.ID, 9, in, digest, tasks[bs[0].ID], time.Now())
	if e != nil {
		t.Fatal(e)
	}
	replay, e := svc.StartPlan(context.Background(), owner, p.ID, in)
	if e != nil || replay.Revision != result.Revision {
		t.Fatalf("service receipt replay needs mutable resources %v", e)
	}
	changed := in
	changed.BatchConfirmations = append([]model.PlanBatchConfirmation{}, in.BatchConfirmations...)
	changed.BatchConfirmations[0].ExpectedRevision++
	if _, e = svc.StartPlan(context.Background(), owner, p.ID, changed); errors.As(e, &proof) || !errors.Is(e, repo.ErrConflict) {
		t.Fatalf("samekey different input signed false %v", e)
	}
}

func TestPlanOriginalCorruptStartReceiptCannotReplayOrProveAnotherKey(t *testing.T) {
	s, p, bs, in, tasks := preparedPlan(t)
	if _, e := s.StartPlan(p.ID, 9, in, "original", tasks[bs[0].ID], time.Now()); e != nil {
		t.Fatal(e)
	}
	p, _, e := s.GetPlan(p.ID)
	if e != nil {
		t.Fatal(e)
	}
	var fields map[string]json.RawMessage
	if e = json.Unmarshal(p.StartReceipt, &fields); e != nil {
		t.Fatal(e)
	}
	delete(fields, "batch_confirmations")
	partial, _ := json.Marshal(fields)
	for _, raw := range []string{`{}`, string(partial)} {
		if e = s.DB.Model(&model.Plan{}).Where("id=?", p.ID).Update("start_receipt_json", raw).Error; e != nil {
			t.Fatal(e)
		}
		for _, key := range []string{in.RequestKey, "other-tab"} {
			old := in
			old.RequestKey = key
			var proof *repo.PlanCommandNotApplied
			if r, e := s.ProbePlanStart(p.ID, 9, old, "original"); r != nil || errors.As(e, &proof) || !errors.Is(e, repo.ErrConflict) {
				t.Fatalf("corrupt receipt gave success/proof %v %v", r, e)
			}
		}
	}
}
