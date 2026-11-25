package service

import (
	"context"
	"errors"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type PlanExecutionRepository interface {
	FinishPlan(uint64, int64, model.FinishPlanInput, time.Time) (model.Plan, error)
	ReconcileStoppedPlan(uint64, time.Time) error
	StartPlan(uint64, int64, model.StartPlanInput, string, []model.Task, time.Time) (model.StartPlanResult, error)
	AdvancePlan(uint64, int64, model.PlanAdvanceInput, string, []model.Task, bool, time.Time) (model.PlanAdvanceResult, error)
	ControlPlan(uint64, int64, uint64, string, string, uint64, uint64, time.Time) (model.PlanCancelResult, error)
}
type PlanExecutionReadRepository interface {
	RecordPlanResourceWait(uint64, model.PlanResourceWait) error
	PlanProgress(uint64, time.Time) (repository.PlanProgress, error)
	RunningPlanCandidates(uint64, int) ([]model.Plan, error)
}

func planStartHash(in model.StartPlanInput) string {
	in.BatchConfirmations = append([]model.PlanBatchConfirmation(nil), in.BatchConfirmations...)
	for i, c := range in.BatchConfirmations {
		c.IncludedRowKeys = append([]string(nil), c.IncludedRowKeys...)
		c.ExcludedRowKeys = append([]string(nil), c.ExcludedRowKeys...)
		c.ExpectedShortfalls = append([]model.ExpectedShortfall(nil), c.ExpectedShortfalls...)
		sort.Strings(c.IncludedRowKeys)
		sort.Strings(c.ExcludedRowKeys)
		sort.Slice(c.ExpectedShortfalls, func(i, j int) bool { return c.ExpectedShortfalls[i].CategoryID < c.ExpectedShortfalls[j].CategoryID })
		in.BatchConfirmations[i] = c
	}
	sort.Slice(in.BatchConfirmations, func(i, j int) bool { return in.BatchConfirmations[i].BatchID < in.BatchConfirmations[j].BatchID })
	return batchCommandHash(in)
}
func validPlanKey(key string) bool {
	return len(key) > 0 && len(key) <= 128 && strings.TrimSpace(key) == key
}
func (s *Service) planOwner(a identity.PublicUser, id uint64) (model.Plan, []model.Batch, error) {
	p, b, e := s.readPlan(a, id)
	if e == nil && !planOwn(a, p) {
		return model.Plan{}, nil, ErrNotFound
	}
	if e == nil && s.PlanExecutionRepo == nil {
		return model.Plan{}, nil, errors.New("plan execution unavailable")
	}
	return p, b, e
}
func (s *Service) StartPlan(ctx context.Context, a identity.PublicUser, id uint64, in model.StartPlanInput) (model.StartPlanResult, error) {
	empty := model.StartPlanResult{}
	if !validPlanKey(in.RequestKey) || in.ExpectedRevision == 0 || in.FirstBatchID == 0 || len(in.BatchConfirmations) == 0 {
		return empty, ErrInvalid
	}
	p, b, e := s.planOwner(a, id)
	if e != nil {
		return empty, e
	}
	digest := planStartHash(in)
	prior, e := repository.ReplayPlanStart(p, in.RequestKey, digest)
	if e != nil {
		return empty, e
	}
	if prior != nil {
		return *prior, nil
	}
	if p.Revision != in.ExpectedRevision {
		return empty, ErrConflict
	}
	var first *model.Batch
	var confirmation model.PlanBatchConfirmation
	byID := map[uint64]model.Batch{}
	for _, row := range b {
		byID[row.ID] = row
	}
	for _, c := range in.BatchConfirmations {
		row, ok := byID[c.BatchID]
		if !ok || row.Revision != c.ExpectedRevision || c.IncludedRowKeys == nil || c.ExcludedRowKeys == nil || c.ExpectedShortfalls == nil {
			return empty, ErrInvalid
		}
		if row.ConfigurationStatus != model.BatchReady || !row.Readiness.Ready {
			return empty, ErrConflict
		}
		for _, candidate := range row.Config.Candidates {
			if !contentValid(candidate.Content) {
				return empty, ErrConflict
			}
		}
		if c.BatchID == in.FirstBatchID {
			r := row
			first = &r
			confirmation = c
		}
	}
	if first == nil {
		return empty, ErrInvalid
	}
	bi := model.BatchStartInput{IncludedRowKeys: confirmation.IncludedRowKeys, ExcludedRowKeys: confirmation.ExcludedRowKeys, AllowShortfall: confirmation.AllowShortfall, ExpectedShortfalls: confirmation.ExpectedShortfalls}
	selected, gaps, e := model.BatchStartSelection(*first, bi)
	if e != nil {
		return empty, e
	}
	preview, tasks, e := s.prepareBatchExecution(ctx, a, *first, selected, gaps)
	if e != nil {
		return empty, e
	}
	if !preview.CanStart {
		return empty, ErrConflict
	}
	return s.PlanExecutionRepo.StartPlan(id, int64(a.ID), in, digest, tasks, time.Now().UTC())
}

type PlanReasonInput struct {
	ExpectedRevision uint64 `json:"expected_revision,string"`
	Reason           string `json:"reason"`
}
type PlanRevisionInput struct {
	ExpectedRevision uint64 `json:"expected_revision,string"`
}
type PlanTakeoverInput struct {
	ExpectedRevision      uint64 `json:"expected_revision,string"`
	CurrentBatchID        uint64 `json:"current_batch_id,string"`
	ExpectedBatchRevision uint64 `json:"expected_batch_revision,string"`
	Reason                string `json:"reason"`
}

func (s *Service) PlanControl(a identity.PublicUser, id uint64, action string, in PlanTakeoverInput) (any, error) {
	if in.ExpectedRevision == 0 || action != "resume" && !validReason(in.Reason) {
		return nil, ErrInvalid
	}
	if _, _, e := s.planOwner(a, id); e != nil {
		return nil, e
	}
	out, e := s.PlanExecutionRepo.ControlPlan(id, int64(a.ID), in.ExpectedRevision, action, in.Reason, in.CurrentBatchID, in.ExpectedBatchRevision, time.Now().UTC())
	if e != nil {
		return nil, e
	}
	detail, e := s.GetPlan(a, id)
	if e != nil {
		return nil, e
	}
	if action == "cancel" {
		return map[string]any{"plan": detail.Plan, "affected_batch_ids": out.AffectedBatchIDs, "settling_batch_ids": out.SettlingBatchIDs}, nil
	}
	return detail, nil
}
func (s *Service) AdvancePlan(ctx context.Context, a identity.PublicUser, id uint64, in model.PlanAdvanceInput, automatic bool) (model.PlanAdvanceResult, error) {
	empty := model.PlanAdvanceResult{}
	if !validPlanKey(in.RequestKey) || in.ExpectedRevision == 0 || in.CurrentBatchID == 0 || in.NextBatchID == 0 || len(in.ExpectedBasisFingerprint) != 64 || !automatic && (in.Decision != "continue" && in.Decision != "skip" || !validReason(in.Reason)) {
		return empty, ErrInvalid
	}
	p, _, e := s.planOwner(a, id)
	if e != nil {
		return empty, e
	}
	digest := batchCommandHash(in)
	prior, e := repository.ReplayPlanAdvance(p, in.RequestKey, digest)
	if e != nil {
		return empty, e
	}
	if prior != nil {
		return *prior, nil
	}
	reader, ok := s.PlanRepo.(PlanExecutionReadRepository)
	if !ok {
		return empty, errors.New("plan execution reads unavailable")
	}
	state, e := reader.PlanProgress(id, time.Now().UTC())
	if e != nil {
		return empty, e
	}
	if state.Next == nil || state.Next.ID != in.NextBatchID {
		return empty, ErrConflict
	}
	next := *state.Next
	tasks := []model.Task{}
	if next.ExecutionStatus != model.BatchCancelled {
		if next.FrozenConfirmation == nil {
			return empty, ErrConflict
		}
		c := next.FrozenConfirmation
		bi := model.BatchStartInput{IncludedRowKeys: c.IncludedRowKeys, ExcludedRowKeys: c.ExcludedRowKeys, AllowShortfall: c.AllowShortfall, ExpectedShortfalls: c.ExpectedShortfalls}
		selected, gaps, e := model.BatchStartSelection(next, bi)
		if e != nil {
			return empty, e
		}
		preview, prepared, e := s.prepareBatchExecution(ctx, a, next, selected, gaps)
		if e != nil {
			return empty, e
		}
		if !preview.CanStart {
			_ = reader.RecordPlanResourceWait(id, model.PlanResourceWait{NextBatchID: next.ID, CurrentBatchID: in.CurrentBatchID, ExpectedRevision: in.ExpectedRevision, Basis: in.ExpectedBasisFingerprint, Code: "original_resources_unavailable", CheckedAt: time.Now().UTC()})
			return empty, ErrConflict
		}
		tasks = prepared
	}
	return s.PlanExecutionRepo.AdvancePlan(id, int64(a.ID), in, digest, tasks, automatic, time.Now().UTC())
}
func (s *Service) AdvanceRunningPlans(ctx context.Context) error {
	reader, ok := s.PlanRepo.(PlanExecutionReadRepository)
	if !ok {
		return errors.New("plan execution reads unavailable")
	}
	var after uint64
	var lastErr error
	for {
		rows, e := reader.RunningPlanCandidates(after, 100)
		if e != nil {
			return e
		}
		if len(rows) == 0 {
			return lastErr
		}
		for _, p := range rows {
			if e = ctx.Err(); e != nil {
				return e
			}
			after = p.ID
			if p.Status == "stopping" {
				if e = s.PlanExecutionRepo.ReconcileStoppedPlan(p.ID, time.Now().UTC()); e != nil {
					lastErr = e
				}
				continue
			}
			if p.Status != "running" {
				continue
			}
			a, found, e := identity.ResolveUser(identity.UserID(p.OwnerID))
			if e != nil {
				return e
			}
			if !found || !planOwn(a, p) {
				continue
			}
			state, e := reader.PlanProgress(p.ID, time.Now().UTC())
			if e != nil {
				return e
			}
			if (state.Progress != "running" && state.Progress != "waiting_resources") || state.Next == nil || state.Current == nil {
				continue
			}
			in := model.PlanAdvanceInput{RequestKey: "auto:" + state.Basis.Fingerprint, ExpectedRevision: state.Plan.Revision, CurrentBatchID: state.Current.ID, NextBatchID: state.Next.ID, ExpectedBasisFingerprint: state.Basis.Fingerprint, Decision: "automatic"}
			if _, e = s.AdvancePlan(ctx, a, p.ID, in, true); e != nil && !errors.Is(e, ErrConflict) && !errors.Is(e, ErrNotFound) && !errors.Is(e, repository.ErrParentBlocked) && !errors.Is(e, repository.ErrOccupied) && !errors.Is(e, repository.ErrUnresolved) {
				lastErr = e
			}
		}
	}
}

func (s *Service) FinishPlan(a identity.PublicUser, id uint64, in model.FinishPlanInput) (PlanDetail, error) {
	if in.ExpectedRevision == 0 || in.ExpectedShortfalls == nil || in.EndReason != "achieved" && in.EndReason != "unmet" || in.EndReason == "unmet" && strings.TrimSpace(in.EndNote) == "" || utf8.RuneCountInString(in.EndNote) > 2000 {
		return PlanDetail{}, ErrInvalid
	}
	if _, _, e := s.planOwner(a, id); e != nil {
		return PlanDetail{}, e
	}
	if _, e := s.PlanExecutionRepo.FinishPlan(id, int64(a.ID), in, time.Now().UTC()); e != nil {
		return PlanDetail{}, e
	}
	return s.GetPlan(a, id)
}
