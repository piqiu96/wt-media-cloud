package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
	"strings"
	"time"
	"unicode/utf8"
)

type PlanRepository interface {
	PlanRowsByIDs([]uint64) (map[uint64][]model.Batch, error)
	CreatePlan(model.Plan, []model.PlanStructureBatch) (model.Plan, []model.Batch, error)
	EditPlanStructure(model.Plan, uint64, []model.PlanStructureBatch) (model.Plan, []model.Batch, error)
	GetPlan(uint64) (model.Plan, []model.Batch, error)
	PlanByRequest(int64, string) (model.Plan, []model.Batch, error)
	ListPlans(repository.PlanFilter) ([]model.Plan, int64, error)
	PlanBatchPage(uint64, int, int, string, *bool) ([]model.Batch, int64, error)
	ListPlanRecords(uint64, int, int) ([]model.PlanRecord, int64, error)
}
type PlanInput struct {
	Name      string                     `json:"name"`
	DateStart string                     `json:"date_start"`
	DateEnd   string                     `json:"date_end"`
	Note      string                     `json:"note"`
	Goals     []model.PlanGoal           `json:"goals"`
	Batches   []model.PlanStructureBatch `json:"batches"`
}
type CreatePlanInput struct {
	PlanInput
	RequestKey string `json:"request_key"`
}
type StructurePlanInput struct {
	PlanInput
	ExpectedRevision uint64 `json:"expected_revision,string"`
}
type ValidatePlanInput struct {
	PlanInput
	PlanID           uint64 `json:"plan_id,string"`
	ExpectedRevision uint64 `json:"expected_revision,string"`
}
type PlanIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type PlanValidation struct {
	Valid  bool        `json:"valid"`
	Issues []PlanIssue `json:"issues"`
}
type PlanBatchView struct {
	BatchID            uint64                       `json:"batch_id,string"`
	RowKey             string                       `json:"row_key"`
	GroupOrder         uint32                       `json:"group_order"`
	BatchOrder         uint32                       `json:"batch_order"`
	Batch              model.BatchDetail            `json:"batch"`
	Configured         bool                         `json:"configured"`
	HandedOffAt        *time.Time                   `json:"handed_off_at"`
	AllowedActions     []string                     `json:"allowed_actions"`
	ActionBlockers     []any                        `json:"action_blockers"`
	FrozenConfirmation *model.PlanBatchConfirmation `json:"frozen_confirmation"`
}
type PlanView struct {
	model.Plan
	Progress                  string           `json:"progress"`
	GoalResults               []PlanGoalResult `json:"goal_results"`
	BatchCount                int              `json:"batch_count"`
	ProcessedBatchCount       int              `json:"processed_batch_count"`
	PublishedCount            int64            `json:"published_count"`
	PendingCount              int64            `json:"pending_count"`
	VerificationRequiredCount int64            `json:"verification_required_count"`
	AllowedActions            []string         `json:"allowed_actions"`
	ActionBlockers            []any            `json:"action_blockers"`
	HandoffBasis              any              `json:"handoff_basis"`
}
type PlanGoalResult struct {
	model.PlanGoal
	PublishedCount int64 `json:"published_count"`
	Shortfall      int64 `json:"shortfall"`
}
type PlanDetail struct {
	Plan    PlanView        `json:"plan"`
	Batches []PlanBatchView `json:"batches"`
	Issues  []PlanIssue     `json:"issues"`
}

func planRead(a identity.PublicUser, p model.Plan) bool {
	if a.ID <= 0 || a.Status != identity.UserStatusEnabled || len(p.Goals) == 0 {
		return false
	}
	team := identity.TeamID(p.TeamID)
	for _, g := range p.Goals {
		if !a.CanAccess(identity.UserID(p.OwnerID), &team, identity.CategoryID(g.CategoryID)) {
			return false
		}
	}
	return true
}
func planOwn(a identity.PublicUser, p model.Plan) bool {
	return canPrepareBatch(a) && int64(a.ID) == p.OwnerID && int64(*a.TeamID) == p.TeamID
}
func planModel(a identity.PublicUser, in PlanInput) model.Plan {
	return model.Plan{TeamID: int64(*a.TeamID), OwnerID: int64(a.ID), CreatedBy: int64(a.ID), Name: in.Name, DateStart: in.DateStart, DateEnd: in.DateEnd, Note: in.Note, Goals: in.Goals}
}
func (s *Service) validatePlan(a identity.PublicUser, in PlanInput) error {
	if !canPrepareBatch(a) {
		return ErrNotFound
	}
	if !validBatchName(in.Name) || utf8.RuneCountInString(in.Note) > 2000 || len(in.Goals) == 0 || len(in.Goals) > 1000 {
		return ErrInvalid
	}
	for _, g := range in.Goals {
		if g.TargetCount > maxSafeTarget {
			return ErrInvalid
		}
		if e := s.checkBatchCategory(a, g.CategoryID); e != nil {
			return e
		}
	}
	if e := repository.ValidatePlanStructure(planModel(a, in), in.Batches); e != nil {
		return ErrInvalid
	}
	return nil
}
func (s *Service) readPlan(a identity.PublicUser, id uint64) (model.Plan, []model.Batch, error) {
	if s.PlanRepo == nil {
		return model.Plan{}, nil, errors.New("plan repository unavailable")
	}
	p, b, e := s.PlanRepo.GetPlan(id)
	if e == nil && !planRead(a, p) {
		return model.Plan{}, nil, ErrNotFound
	}
	return p, b, e
}
func (s *Service) CreatePlan(a identity.PublicUser, in CreatePlanInput) (PlanDetail, error) {
	if !canPrepareBatch(a) {
		return PlanDetail{}, ErrNotFound
	}
	if strings.TrimSpace(in.RequestKey) != in.RequestKey || len(in.RequestKey) == 0 || len(in.RequestKey) > 128 {
		return PlanDetail{}, ErrInvalid
	}
	raw, _ := json.Marshal(in)
	hash := sha256.Sum256(raw)
	digest := hex.EncodeToString(hash[:])
	p, b, e := s.PlanRepo.PlanByRequest(int64(a.ID), in.RequestKey)
	if e == nil {
		if !planRead(a, p) {
			return PlanDetail{}, ErrNotFound
		}
		if p.RequestHash != digest {
			return PlanDetail{}, ErrConflict
		}
		return s.planDetail(a, p, b)
	}
	if !errors.Is(e, ErrNotFound) {
		return PlanDetail{}, e
	}
	if e = s.validatePlan(a, in.PlanInput); e != nil {
		return PlanDetail{}, e
	}
	for _, r := range in.Batches {
		if r.BatchID != nil {
			return PlanDetail{}, ErrInvalid
		}
	}
	p = planModel(a, in.PlanInput)
	p.RequestKey = in.RequestKey
	p.RequestHash = digest
	p.CreatedAt = time.Now().UTC()
	p.UpdatedAt = p.CreatedAt
	p, b, e = s.PlanRepo.CreatePlan(p, in.Batches)
	if e != nil {
		return PlanDetail{}, e
	}
	return s.planDetail(a, p, b)
}
func (s *Service) EditPlan(a identity.PublicUser, id uint64, in StructurePlanInput) (PlanDetail, error) {
	p, _, e := s.readPlan(a, id)
	if e != nil {
		return PlanDetail{}, e
	}
	if !planOwn(a, p) {
		return PlanDetail{}, ErrNotFound
	}
	if in.ExpectedRevision == 0 {
		return PlanDetail{}, ErrInvalid
	}
	if e = s.validatePlan(a, in.PlanInput); e != nil {
		return PlanDetail{}, e
	}
	p.Name = in.Name
	p.Note = in.Note
	p.DateStart = in.DateStart
	p.DateEnd = in.DateEnd
	p.Goals = in.Goals
	p, b, e := s.PlanRepo.EditPlanStructure(p, in.ExpectedRevision, in.Batches)
	if e != nil {
		return PlanDetail{}, e
	}
	return s.planDetail(a, p, b)
}
func (s *Service) GetPlan(a identity.PublicUser, id uint64) (PlanDetail, error) {
	p, b, e := s.readPlan(a, id)
	if e != nil {
		return PlanDetail{}, e
	}
	return s.planDetail(a, p, b)
}
func (s *Service) planDetail(a identity.PublicUser, p model.Plan, b []model.Batch) (PlanDetail, error) {
	v := PlanView{Plan: p, Progress: "preparing", BatchCount: len(b), GoalResults: []PlanGoalResult{}, AllowedActions: []string{}, ActionBlockers: []any{}}
	// The same original task/work facts feed detail, list and control projections.
	if p.StartedAt != nil {
		v.Progress = "running"
	}
	if p.Status == "paused" {
		v.Progress = "manual_paused"
	}
	if p.StoppedAt != nil {
		v.Progress = "stopping"
	}
	if p.EndedAt != nil {
		v.Progress = "ended"
	}
	if planOwn(a, p) && p.StartedAt == nil && p.Status == "not_started" {
		v.AllowedActions = []string{"edit"}
	}
	if p.Status == "ended" && canPrepareBatch(a) {
		v.AllowedActions = append(v.AllowedActions, "copy")
	}
	rows := []PlanBatchView{}
	for _, batch := range b {
		rows = append(rows, planBatchView(a, model.ProjectPlanBatch(p, batch)))
	}
	for _, g := range p.Goals {
		v.GoalResults = append(v.GoalResults, PlanGoalResult{PlanGoal: g, Shortfall: g.TargetCount})
	}

	if reader, ok := s.PlanRepo.(PlanExecutionReadRepository); ok && p.StartedAt != nil {
		state, e := reader.PlanProgress(p.ID, time.Now().UTC())
		if e != nil {
			return PlanDetail{}, e
		}
		v.Progress = state.Progress
		v.HandoffBasis = state.Basis
		for i := range v.GoalResults {
			for _, t := range state.Tasks {
				if t.Platform == v.GoalResults[i].Platform && t.CategoryID == v.GoalResults[i].CategoryID && t.Status == model.StatusPublished && t.WorkID != nil && !t.VerificationRequired {
					v.GoalResults[i].PublishedCount++
				}
			}
			v.GoalResults[i].Shortfall = max(0, v.GoalResults[i].TargetCount-v.GoalResults[i].PublishedCount)
			v.PublishedCount += v.GoalResults[i].PublishedCount
		}
		for _, t := range state.Tasks {
			if t.VerificationRequiredAt(time.Now().UTC()) {
				v.VerificationRequiredCount++
			}
			switch t.Status {
			case model.StatusPending, model.StatusQueued, model.StatusRunning, model.StatusAwaitingHuman, model.StatusUncertain:
				v.PendingCount++
			}
		}
		for _, b := range state.Batches {
			if b.HandedOffAt != nil || b.ExecutionStatus == model.BatchEnded || b.ExecutionStatus == model.BatchCancelled {
				v.ProcessedBatchCount++
			}
		}
	}
	if planOwn(a, p) && p.StartedAt != nil && p.StoppedAt == nil && p.EndedAt == nil && v.PendingCount == 0 && v.VerificationRequiredCount == 0 {
		reader, ok := s.PlanRepo.(PlanExecutionReadRepository)
		if ok {
			state, e := reader.PlanProgress(p.ID, time.Now().UTC())
			if e != nil {
				return PlanDetail{}, e
			}
			reviewing := false
			for _, batch := range state.Batches {
				if batch.ExecutionStatus != model.BatchCancelled && (batch.StartedAt == nil || batch.HandedOffAt == nil && (state.Plan.CurrentBatchID == nil || batch.ID != *state.Plan.CurrentBatchID) && batch.ExecutionStatus != model.BatchEnded) {
					reviewing = true
				}
			}
			for _, t := range state.Tasks {
				reviewing = reviewing || t.Status == model.StatusReviewing
			}
			if !reviewing {
				v.AllowedActions = append(v.AllowedActions, "finish")
			}
		}
	}
	if planOwn(a, p) {
		for _, batch := range b {
			if model.CanTakeoverPlanBatch(p, batch) {
				v.AllowedActions = append(v.AllowedActions, "takeover")
				break
			}
		}
		if p.StoppedAt == nil && p.EndedAt == nil {
			v.AllowedActions = append(v.AllowedActions, "cancel")
			if p.StartedAt != nil {
				if p.Status == "paused" {
					v.AllowedActions = append(v.AllowedActions, "resume")
				} else if p.Status == "running" {
					v.AllowedActions = append(v.AllowedActions, "pause")
					if v.Progress == "waiting_skip" || v.Progress == "waiting_continue" {
						v.AllowedActions = append(v.AllowedActions, "confirm-next")
					}
				}
			} else {
				ready := len(b) > 0
				for _, batch := range b {
					if batch.ExecutionStatus != model.BatchCancelled && (batch.PlanConfiguredAt == nil || !batch.Readiness.Ready || batch.ConfigurationStatus != model.BatchReady) {
						ready = false
					}
				}
				if ready {
					v.AllowedActions = append(v.AllowedActions, "start")
				}
			}
		}
	}
	if v.Progress == "waiting_resources" {
		v.ActionBlockers = append(v.ActionBlockers, map[string]string{"action": "confirm-next", "code": "original_resources_unavailable", "message": "下一批原资源暂不可用，等待恢复后重检，不替换资源或下调目标"})
	}
	if v.Progress == "waiting_tasks" || v.Progress == "waiting_verification" {
		v.ActionBlockers = append(v.ActionBlockers, map[string]string{"action": "confirm-next", "code": "unresolved_original_tasks", "message": "请先通过原批次处理人工、未知或待核实结果"})
	}
	if v.PendingCount > 0 || v.VerificationRequiredCount > 0 {
		v.ActionBlockers = append(v.ActionBlockers, map[string]string{"action": "finish", "code": "unresolved_original_tasks", "message": "仍有未决执行或未核实结果，暂不能结束计划"})
	}
	return PlanDetail{Plan: v, Batches: rows, Issues: []PlanIssue{}}, nil
}
func planBatchView(a identity.PublicUser, b model.Batch) PlanBatchView {
	b = projectBatch(a, b)
	if b.PlanFrozenAt != nil {
		filtered := []string{}
		for _, act := range b.AllowedActions {
			if act != "edit" && act != "start" {
				filtered = append(filtered, act)
			}
		}
		b.AllowedActions = filtered
	}
	r := PlanBatchView{BatchID: b.ID, Batch: model.BatchDetail{Batch: b, Config: b.Config, Readiness: b.Readiness}, Configured: b.PlanConfiguredAt != nil, HandedOffAt: b.HandedOffAt, AllowedActions: b.AllowedActions, ActionBlockers: []any{}, FrozenConfirmation: b.FrozenConfirmation}
	for _, blocker := range b.ActionBlockers {
		r.ActionBlockers = append(r.ActionBlockers, blocker)
	}
	if b.PlanRowKey != nil {
		r.RowKey = *b.PlanRowKey
	}
	if b.PlanGroupOrder != nil {
		r.GroupOrder = *b.PlanGroupOrder
	}
	if b.PlanBatchOrder != nil {
		r.BatchOrder = *b.PlanBatchOrder
	}
	return r
}
func (s *Service) ListPlans(a identity.PublicUser, f repository.PlanFilter) (BatchPage[PlanView], error) {
	out := BatchPage[PlanView]{Items: []PlanView{}, Page: f.Page, PageSize: f.PageSize}
	if a.ID <= 0 || a.Status != identity.UserStatusEnabled {
		return out, ErrNotFound
	}
	if !validBatchPagination(f.Page, f.PageSize) || utf8.RuneCountInString(f.Search) > 100 || !validPlanStatus(f.Status) || f.Platform != "" && f.Platform != "baidu" {
		return out, ErrInvalid
	}
	f.Actor = a
	p, n, e := s.PlanRepo.ListPlans(f)
	if e != nil {
		return out, e
	}
	ids := make([]uint64, 0, len(p))
	for _, row := range p {
		ids = append(ids, row.ID)
	}
	pageBatches, e := s.PlanRepo.PlanRowsByIDs(ids)
	if e != nil {
		return out, e
	}
	for _, p := range p {
		if !planRead(a, p) {
			return out, ErrNotFound
		}
		d, e := s.planDetail(a, p, pageBatches[p.ID])
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, d.Plan)
	}
	out.Total = n
	return out, nil
}
func validPlanStatus(s string) bool {
	return s == "" || s == "not_started" || s == "running" || s == "paused" || s == "stopping" || s == "ended"
}
func (s *Service) PlanBatches(a identity.PublicUser, id uint64, page, size int, status string, pending *bool) (BatchPage[PlanBatchView], error) {
	out := BatchPage[PlanBatchView]{Items: []PlanBatchView{}, Page: page, PageSize: size}
	if _, _, e := s.readPlan(a, id); e != nil {
		return out, e
	}
	if !validBatchPagination(page, size) || !validBatchExecutionStatus(status) {
		return out, ErrInvalid
	}
	rows, n, e := s.PlanRepo.PlanBatchPage(id, page, size, status, pending)
	if e != nil {
		return out, e
	}
	for _, b := range rows {
		out.Items = append(out.Items, planBatchView(a, b))
	}
	out.Total = n
	return out, nil
}
func validBatchExecutionStatus(s string) bool {
	return s == "" || s == "not_started" || s == "running" || s == "paused" || s == "stopping" || s == "ended" || s == "cancelled"
}
func (s *Service) PlanRecords(a identity.PublicUser, id uint64, page, size int) (BatchPage[model.PlanRecord], error) {
	out := BatchPage[model.PlanRecord]{Items: []model.PlanRecord{}, Page: page, PageSize: size}
	if _, _, e := s.readPlan(a, id); e != nil {
		return out, e
	}
	if !validBatchPagination(page, size) {
		return out, ErrInvalid
	}
	rows, n, e := s.PlanRepo.ListPlanRecords(id, page, size)
	out.Items = rows
	out.Total = n
	return out, e
}
func (s *Service) CopyPlan(a identity.PublicUser, id, revision uint64) (any, error) {
	p, bs, e := s.readPlan(a, id)
	if e != nil {
		return nil, e
	}
	if !canPrepareBatch(a) {
		return nil, ErrNotFound
	}
	if p.Revision != revision || p.Status != "ended" {
		return nil, ErrConflict
	}
	rows := []model.PlanStructureBatch{}
	for _, b := range bs {
		r := planBatchView(a, b)
		rows = append(rows, model.PlanStructureBatch{RowKey: r.RowKey, Name: b.Name, Platform: b.Platform, GroupOrder: r.GroupOrder, BatchOrder: r.BatchOrder, Targets: b.Targets})
	}
	for _, g := range p.Goals {
		if e = s.checkBatchCategory(a, g.CategoryID); e != nil {
			return nil, e
		}
	}
	return map[string]any{"name": p.Name, "note": p.Note, "goals": p.Goals, "batches": rows}, nil
}
func (s *Service) ValidatePlan(a identity.PublicUser, in ValidatePlanInput) (PlanValidation, error) {
	if in.PlanID != 0 {
		p, _, e := s.readPlan(a, in.PlanID)
		if e != nil {
			return PlanValidation{}, e
		}
		if !planOwn(a, p) {
			return PlanValidation{}, ErrNotFound
		}
		if p.Revision != in.ExpectedRevision {
			return PlanValidation{}, ErrConflict
		}
	}
	e := s.validatePlan(a, in.PlanInput)
	if errors.Is(e, ErrInvalid) {
		return PlanValidation{Issues: []PlanIssue{{Code: "structure_invalid", Message: "平台类目逐格目标、顺序、日期或批次结构不一致"}}}, nil
	}
	if e != nil {
		return PlanValidation{}, e
	}
	return PlanValidation{Valid: true, Issues: []PlanIssue{}}, nil
}
