package model

import "time"

type StartPlanInput struct {
	RequestKey         string                  `json:"request_key"`
	ExpectedRevision   uint64                  `json:"expected_revision,string"`
	FirstBatchID       uint64                  `json:"first_batch_id,string"`
	BatchConfirmations []PlanBatchConfirmation `json:"batch_confirmations"`
}
type StartPlanResult struct {
	PlanID             uint64                  `json:"plan_id,string"`
	Revision           uint64                  `json:"revision,string"`
	FirstStartedAt     time.Time               `json:"first_started_at"`
	CurrentBatchID     uint64                  `json:"current_batch_id,string"`
	BatchConfirmations []PlanBatchConfirmation `json:"batch_confirmations"`
	FirstBatchReceipt  BatchStartResult        `json:"first_batch_receipt"`
}
type PlanHandoffBasis struct {
	CurrentBatchID       *uint64 `json:"current_batch_id,string"`
	NextBatchID          *uint64 `json:"next_batch_id,string"`
	CurrentBatchRevision *uint64 `json:"current_batch_revision,string"`
	NextBatchRevision    *uint64 `json:"next_batch_revision,string"`
	ExplicitCount        int64   `json:"explicit_count"`
	FailedCount          int64   `json:"failed_count"`
	Fingerprint          string  `json:"fingerprint"`
}
type PlanAdvanceInput struct {
	RequestKey               string `json:"request_key"`
	ExpectedRevision         uint64 `json:"expected_revision,string"`
	CurrentBatchID           uint64 `json:"current_batch_id,string"`
	NextBatchID              uint64 `json:"next_batch_id,string"`
	ExpectedBasisFingerprint string `json:"expected_basis_fingerprint"`
	Decision                 string `json:"decision"`
	Reason                   string `json:"reason"`
}
type PlanAdvanceResult struct {
	CurrentBatchStatus string            `json:"current_batch_status"`
	PlanID             uint64            `json:"plan_id,string"`
	Revision           uint64            `json:"revision,string"`
	PreviousBatchID    uint64            `json:"previous_batch_id,string"`
	CurrentBatchID     uint64            `json:"current_batch_id,string"`
	Decision           string            `json:"decision"`
	AdvancedAt         time.Time         `json:"advanced_at"`
	BatchReceipt       *BatchStartResult `json:"batch_receipt"`
}
type PlanCancelResult struct {
	Plan             Plan     `json:"plan"`
	AffectedBatchIDs []string `json:"affected_batch_ids"`
	SettlingBatchIDs []string `json:"settling_batch_ids"`
}

// VerificationRequired is a projection of original facts, not a new task state.
func (t Task) VerificationRequiredAt(now time.Time) bool {
	if t.Status == StatusPublished {
		return t.WorkID == nil || t.VerificationRequired
	}
	if t.Status != StatusReviewing {
		return false
	}
	if t.AcceptedAt == nil || t.CurrentAttemptID == nil {
		return true
	}
	cutoff := t.AcceptedAt.Add(24 * time.Hour)
	stopped := t.CheckStoppedAt != nil || !now.Before(cutoff)
	if !stopped {
		return false
	}
	v := t.LatestManualVerification
	if v == nil || v.AttemptID != *t.CurrentAttemptID || v.TaskRevision > t.Revision {
		return true
	}
	if t.CheckStoppedAt != nil && v.VerifiedAt.Before(*t.CheckStoppedAt) {
		return true
	}
	if !now.Before(cutoff) && v.VerifiedAt.Before(cutoff) {
		return true
	}
	return v.Conclusion != StatusReviewing && v.Conclusion != StatusPublished && v.Conclusion != StatusFailed
}

type PlanResourceWait struct {
	NextBatchID      uint64    `json:"next_batch_id,string"`
	CurrentBatchID   uint64    `json:"current_batch_id,string"`
	ExpectedRevision uint64    `json:"expected_revision,string"`
	Basis            string    `json:"basis"`
	Code             string    `json:"code"`
	CheckedAt        time.Time `json:"checked_at"`
}

type PlanGoalShortfall struct {
	Platform   string `json:"platform"`
	CategoryID int64  `json:"category_id,string"`
	Shortfall  int64  `json:"shortfall"`
}
type FinishPlanInput struct {
	ExpectedRevision   uint64              `json:"expected_revision,string"`
	ExpectedShortfalls []PlanGoalShortfall `json:"expected_shortfalls"`
	EndReason          string              `json:"end_reason"`
	EndNote            string              `json:"end_note"`
}

// Takeover only pauses later plan advancement around the current, already
// authorized execution. It cannot start a future batch or grant a new attempt.
func CanTakeoverPlanBatch(p Plan, b Batch) bool {
	if p.Status != "running" || p.StartedAt == nil || p.StoppedAt != nil || p.EndedAt != nil || p.CurrentBatchID == nil || *p.CurrentBatchID != b.ID || b.PlanID == nil || *b.PlanID != p.ID || b.StartedAt == nil || b.StoppedAt != nil || b.EndedAt != nil || b.HandedOffAt != nil || b.Revision == 0 || b.PlanFrozenAt == nil || b.FrozenConfirmation == nil {
		return false
	}
	c := b.FrozenConfirmation
	return (b.ExecutionStatus == BatchRunning || b.ExecutionStatus == BatchPaused) && c.BatchID == b.ID && c.ExpectedRevision > 0 && len(c.IncludedRowKeys) > 0 && c.ExcludedRowKeys != nil && len(c.ExpectedShortfalls) > 0
}
