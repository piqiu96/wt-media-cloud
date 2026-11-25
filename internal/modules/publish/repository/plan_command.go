package repository

import (
	"bytes"
	"encoding/json"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"gorm.io/gorm"
	"strconv"
)

// This error is a proof for one immutable input, not a generic failure receipt.
// It is only constructed while holding the monotonically revised parent fence.
type PlanCommandNotApplied struct {
	CommandApplied   bool   `json:"command_applied"`
	Command          string `json:"command"`
	RequestKey       string `json:"request_key"`
	ExpectedRevision uint64 `json:"expected_revision,string"`
	CurrentRevision  uint64 `json:"current_revision,string"`
}

func (e *PlanCommandNotApplied) Error() string { return ErrConflict.Error() }
func (e *PlanCommandNotApplied) Unwrap() error { return ErrConflict }
func stalePlanCommand(p model.Plan, command, key string, revision uint64, valid bool) error {
	if valid && revision < p.Revision {
		return &PlanCommandNotApplied{Command: command, RequestKey: key, ExpectedRevision: revision, CurrentRevision: p.Revision}
	}
	if revision != p.Revision {
		return ErrConflict
	}
	return nil
}
func checkPlanStartCommand(p model.Plan, in model.StartPlanInput, digest string) (*model.StartPlanResult, error) {
	if p.StartRequestKey != nil {
		if p.StartRequestHash == nil || *p.StartRequestHash == "" || len(p.StartReceipt) == 0 {
			return nil, ErrConflict
		}
		if *p.StartRequestKey == in.RequestKey {
			return ReplayPlanStart(p, in.RequestKey, digest)
		}
		if _, e := ReplayPlanStart(p, *p.StartRequestKey, *p.StartRequestHash); e != nil {
			return nil, e
		}
		if e := stalePlanCommand(p, "start", in.RequestKey, in.ExpectedRevision, model.ValidPlanStartCommand(in) && digest != ""); e != nil {
			return nil, e
		}
		return nil, ErrConflict
	}
	if p.StartRequestHash != nil || len(p.StartReceipt) > 0 {
		return nil, ErrConflict
	}
	valid := model.ValidPlanStartCommand(in) && digest != "" && p.Status == "not_started" && p.StartedAt == nil && p.CurrentBatchID == nil && p.StoppedAt == nil && p.EndedAt == nil
	return nil, stalePlanCommand(p, "start", in.RequestKey, in.ExpectedRevision, valid)
}
func completePlanReceipt(raw json.RawMessage, nullable string, keys ...string) bool {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return false
	}
	for _, key := range keys {
		value, ok := fields[key]
		if !ok || key != nullable && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return false
		}
	}
	return true
}
func validPlanBatchReceipt(r model.BatchStartResult) bool {
	if !r.HasCompleteJSON() || r.BatchID == 0 || r.Revision < 2 || r.ExecutionStatus != model.BatchRunning || r.FirstStartedAt.IsZero() || r.CreatedCount <= 0 || r.CreatedCount != int64(len(r.TaskIDs)) || r.TargetCount < r.CreatedCount || r.CreationShortfall != r.TargetCount-r.CreatedCount {
		return false
	}
	seen := map[string]bool{}
	for _, value := range r.TaskIDs {
		id, e := strconv.ParseUint(value, 10, 64)
		if e != nil || id == 0 || strconv.FormatUint(id, 10) != value || seen[value] {
			return false
		}
		seen[value] = true
	}
	return true
}
func readAdvanceReceipts(p model.Plan) (map[string]planAdvanceReceipt, error) {
	rs := map[string]planAdvanceReceipt{}
	if len(p.HandoffReceipts) == 0 {
		return rs, nil
	}
	if e := json.Unmarshal(p.HandoffReceipts, &rs); e != nil || rs == nil {
		return nil, ErrConflict
	}
	var raw map[string]struct {
		Result json.RawMessage `json:"result"`
	}
	if json.Unmarshal(p.HandoffReceipts, &raw) != nil {
		return nil, ErrConflict
	}
	for key, r := range rs {
		if !completePlanReceipt(raw[key].Result, "batch_receipt", "plan_id", "revision", "previous_batch_id", "current_batch_id", "decision", "advanced_at", "batch_receipt", "current_batch_status") {
			return nil, ErrConflict
		}
		v := r.Result
		if key == "" || r.Digest == "" || v.PlanID != p.ID || v.Revision == 0 || v.Revision > p.Revision || v.CurrentBatchID == 0 || v.PreviousBatchID == 0 || v.CurrentBatchID == v.PreviousBatchID || v.AdvancedAt.IsZero() || (v.Decision != "automatic" && v.Decision != "continue" && v.Decision != "skip") {
			return nil, ErrConflict
		}
		if v.BatchReceipt == nil {
			if v.CurrentBatchStatus != model.BatchCancelled {
				return nil, ErrConflict
			}
		} else if v.CurrentBatchStatus != model.BatchRunning || v.BatchReceipt.BatchID != v.CurrentBatchID || !validPlanBatchReceipt(*v.BatchReceipt) || !v.BatchReceipt.FirstStartedAt.Equal(v.AdvancedAt) {
			return nil, ErrConflict
		}
	}
	return rs, nil
}
func checkPlanAdvanceCommand(p model.Plan, in model.PlanAdvanceInput, digest string, automatic bool) (*model.PlanAdvanceResult, map[string]planAdvanceReceipt, error) {
	rs, e := readAdvanceReceipts(p)
	if e != nil {
		return nil, nil, e
	}
	if r, ok := rs[in.RequestKey]; ok {
		if r.Digest != digest {
			return nil, nil, ErrConflict
		}
		return &r.Result, rs, nil
	}
	valid := !automatic && len(p.HandoffReceipts) > 0 && model.ValidPlanAdvanceCommand(in, false) && digest != "" && p.StartRequestKey != nil && p.StartRequestHash != nil
	if valid {
		_, err := ReplayPlanStart(p, *p.StartRequestKey, *p.StartRequestHash)
		valid = err == nil
	}
	return nil, rs, stalePlanCommand(p, "confirm-next", in.RequestKey, in.ExpectedRevision, valid)
}
func (s ExecutionStore) ProbePlanStart(id uint64, actor int64, in model.StartPlanInput, digest string) (out *model.StartPlanResult, e error) {
	e = currentTransaction(s.DB, func(tx *gorm.DB) error {
		p, err := lockPlan(tx, id)
		if err != nil {
			return err
		}
		if p.OwnerID != actor {
			return ErrNotFound
		}
		out, err = checkPlanStartCommand(p, in, digest)
		return err
	})
	return
}
func (s ExecutionStore) ProbePlanAdvance(id uint64, actor int64, in model.PlanAdvanceInput, digest string, automatic bool) (out *model.PlanAdvanceResult, e error) {
	e = currentTransaction(s.DB, func(tx *gorm.DB) error {
		p, err := lockPlan(tx, id)
		if err != nil {
			return err
		}
		if p.OwnerID != actor {
			return ErrNotFound
		}
		out, _, err = checkPlanAdvanceCommand(p, in, digest, automatic)
		return err
	})
	return
}
