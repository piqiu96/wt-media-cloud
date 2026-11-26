package repository

import (
	"errors"
	"reflect"
	"slices"
	"strconv"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrParentBlocked    = errors.New("publication parent control blocks the operation")
	ErrShortfallChanged = errors.New("publication batch shortfall changed")
	ErrPauseChanged     = errors.New("publication batch pause changed")
	ErrUnresolved       = errors.New("publication batch has unresolved work")
	ErrCapacityWait     = errors.New("publication capacity busy")
	ErrAgentUpgrade     = errors.New("publication Agent upgrade required")
	ErrProfileUse       = errors.New("publication Profile use conflict")
	ErrAdmissionClosed  = errors.New("publication admission cutover closed")
)

type BatchExecutionConflict struct {
	BatchConflictError
	ExecutionStatus     string                    `json:"execution_status"`
	StartRequestApplied *bool                     `json:"start_request_applied,omitempty"`
	Pause               *model.BatchPause         `json:"pause,omitempty"`
	Issues              []model.BatchIssue        `json:"issues,omitempty"`
	Shortfalls          []model.CreationShortfall `json:"shortfalls,omitempty"`
	Cause               error                     `json:"-"`
}

func (e *BatchExecutionConflict) Error() string { return e.Cause.Error() }
func (e *BatchExecutionConflict) Unwrap() error { return e.Cause }

func BatchConflict(v model.Batch, cause error) *BatchExecutionConflict {
	return &BatchExecutionConflict{BatchConflictError: BatchConflictError{BatchID: v.ID, Revision: v.Revision},
		ExecutionStatus: v.ExecutionStatus, Pause: v.Pause, Cause: cause}
}

func lockBatch(tx *gorm.DB, id uint64) (model.Batch, error) {
	parent, e := lockBatchPlan(tx, id)
	if e != nil {
		return model.Batch{}, e
	}
	var v model.Batch
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&v, id).Error
	if parent != nil {
		v.PlanPermanentStopped = parent.StoppedAt != nil || parent.EndedAt != nil
		v.PlanPositionBlocked = parent.StartedAt != nil && (parent.CurrentBatchID == nil || *parent.CurrentBatchID != v.ID) || v.HandedOffAt != nil
	}
	return v, missing(err)
}

// A task's parent is immutable. Every write which can admit or change an
// attached task takes the same batch control fence before its task and attempt.
func lockTaskBatch(tx *gorm.DB, task model.Task) (*model.Batch, error) {
	if task.BatchID == nil {
		return nil, nil
	}
	result, err := lockBatch(tx, *task.BatchID)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func canDispatch(b *model.Batch, t model.Task) bool {
	return b == nil || !b.PlanPermanentStopped && !b.PlanPositionBlocked && b.ExecutionStatus == model.BatchRunning && b.StoppedAt == nil && b.StartedAt != nil &&
		b.ExecutionGroupNo != nil && t.ExecutionGroupNo != nil && *t.ExecutionGroupNo == *b.ExecutionGroupNo && b.GroupEndedAt == nil
}

func stoppedParent(b *model.Batch) bool {
	return b != nil && (b.PlanPermanentStopped || b.PlanPositionBlocked || b.HandedOffAt != nil || b.StoppedAt != nil || b.ExecutionStatus == model.BatchStopping || b.ExecutionStatus == model.BatchCancelled || b.ExecutionStatus == model.BatchEnded)
}

// This validates only the immutable first-start receipt already in storage.
// It cannot authorize work or expose the original request key to a caller.
func validBatchStartReceipt(v model.Batch) bool {
	r := v.StartReceipt
	if v.StartRequestKey == nil || *v.StartRequestKey == "" || v.StartRequestHash == nil || *v.StartRequestHash == "" ||
		r == nil || !r.HasCompleteJSON() || r.BatchID != v.ID || r.Revision < 2 || r.Revision > v.Revision || r.ExecutionStatus != model.BatchRunning ||
		v.StartedAt == nil || r.FirstStartedAt.IsZero() || !r.FirstStartedAt.Equal(*v.StartedAt) ||
		r.CreatedCount <= 0 || r.CreatedCount != int64(len(r.TaskIDs)) {
		return false
	}
	var target int64
	for _, row := range v.Targets {
		target += row.TargetCount
	}
	if r.TargetCount != target || r.CreatedCount > target || r.CreationShortfall != target-r.CreatedCount {
		return false
	}
	seen := make(map[string]bool, len(r.TaskIDs))
	for _, value := range r.TaskIDs {
		id, err := strconv.ParseUint(value, 10, 64)
		if err != nil || id == 0 || strconv.FormatUint(id, 10) != value || seen[value] {
			return false
		}
		seen[value] = true
	}
	return true
}

func ReplayBatchStart(v model.Batch, key, digest string) (*model.BatchStartResult, error) {
	if v.StartRequestKey == nil {
		return nil, nil
	}
	if *v.StartRequestKey != key {
		conflict := BatchConflict(v, ErrConflict)
		if validBatchStartReceipt(v) {
			notApplied := false
			conflict.StartRequestApplied = &notApplied
		}
		return nil, conflict
	}
	if v.StartRequestHash == nil || *v.StartRequestHash != digest || !validBatchStartReceipt(v) {
		return nil, BatchConflict(v, ErrConflict)
	}
	return v.StartReceipt, nil
}

func (s ExecutionStore) StartBatch(id uint64, actor int64, in model.BatchStartInput, digest string, prepared []model.Task, now time.Time) (model.BatchStartResult, error) {
	var out model.BatchStartResult
	now = now.UTC().Truncate(time.Microsecond)
	err := currentTransaction(s.DB, func(tx *gorm.DB) error {
		v, err := lockBatch(tx, id)
		if err != nil {
			return err
		}
		if prior, e := ReplayBatchStart(v, in.RequestKey, digest); e == nil && prior != nil {
			out = *prior
			return nil
		}
		if v.PlanID != nil {
			return ErrParentBlocked
		}
		out, err = s.startBatchTx(tx, v, actor, in, digest, prepared, now)
		return err
	})
	if err != nil {
		return model.BatchStartResult{}, err
	}
	return out, nil
}

// startBatchTx is the single atomic task/occupancy/authorization writer shared
// by standalone starts and the parent plan transaction. It never opens a transaction.
func (s ExecutionStore) startBatchTx(tx *gorm.DB, v model.Batch, actor int64, in model.BatchStartInput, digest string, prepared []model.Task, now time.Time) (model.BatchStartResult, error) {
	id := v.ID
	var out model.BatchStartResult
	err := func() error {
		if v.OperatorID != actor {
			return ErrNotFound
		}
		prior, err := ReplayBatchStart(v, in.RequestKey, digest)
		if err != nil {
			return err
		}
		if prior != nil {
			out = *prior
			return nil
		}
		if v.Revision != in.ExpectedRevision {
			return BatchConflict(v, ErrConflict)
		}
		if v.ExecutionStatus != model.BatchNotStarted || v.StartedAt != nil || v.StoppedAt != nil {
			return BatchConflict(v, ErrParentBlocked)
		}
		if err = admissionOpen(tx); err != nil {
			return err
		}
		included, shortfalls, err := model.BatchStartSelection(v, in)
		if err != nil {
			e := BatchConflict(v, ErrShortfallChanged)
			e.Shortfalls = shortfalls
			return e
		}
		if len(prepared) != len(included) {
			return BatchConflict(v, ErrConflict)
		}
		hold := make([]int64, 0, 2*len(prepared))
		for i, row := range included {
			task := prepared[i]
			if task.RowKey == nil || *task.RowKey != row.RowKey || task.TeamID != v.TeamID || task.OperatorID != actor || task.Platform != v.Platform ||
				task.CategoryID != row.CategoryID || task.MediaAccountID != row.MediaAccountID || task.CompositeOutputID != row.CompositeOutputID ||
				!reflect.DeepEqual(task.Content, row.Content) || !equalSchedule(task.NotBefore, row.NotBefore) {
				return BatchConflict(v, ErrConflict)
			}
			hold = append(hold, taskCoverIDs(task)...)
		}
		if s.LockFiles == nil {
			return errors.New("publication file locking unavailable")
		}
		slices.Sort(hold)
		if err = s.LockFiles(tx, slices.Compact(hold)); err != nil {
			return err
		}
		out = model.BatchStartResult{BatchID: id, Revision: v.Revision + 1, ExecutionStatus: model.BatchRunning, FirstStartedAt: now, TaskIDs: make([]string, 0, len(prepared)), CreatedCount: int64(len(prepared))}
		for _, gap := range shortfalls {
			out.TargetCount += gap.TargetCount
			out.CreationShortfall += gap.Shortfall
		}
		for i, task := range prepared {
			task.BatchID, task.TaskOrder = &id, uint32(i)
			task.Status, task.AuthorizedAt, task.Revision = model.StatusPending, &now, 1
			task.CreatedBy, task.CreatedAt, task.UpdatedAt = actor, now, now
			if _, err = s.readInput(tx, task); err != nil {
				if errors.Is(err, ErrConflict) || errors.Is(err, ErrNotFound) {
					conflict := BatchConflict(v, ErrConflict)
					conflict.Issues = []model.BatchIssue{{RowKey: *task.RowKey, Field: "resources", Code: "execution_input_unavailable", Message: "请检查本人账号、原浏览器环境、成片和封面"}}
					return conflict
				}
				return err
			}
			if err = tx.Omit("RequestKey", "RequestHash").Create(&task).Error; err != nil {
				if duplicateIndex(err, "uq_publication_output") {
					conflict := BatchConflict(v, ErrOccupied)
					conflict.Issues = []model.BatchIssue{{RowKey: *task.RowKey, Field: "composite_output_id", Code: "output_occupied", Message: "成片已被当前平台的任务占用"}}
					return conflict
				}
				return err
			}
			out.TaskIDs = append(out.TaskIDs, strconv.FormatUint(task.ID, 10))
			if err = audit(tx, actor, task.ID, "publication.batch_start", task.Revision); err != nil {
				return err
			}
		}
		v.Revision = out.Revision
		if err = tx.Model(&model.Batch{}).Where("id=?", id).Updates(map[string]any{
			"started_at": now, "execution_status": model.BatchRunning, "revision": v.Revision, "updated_at": now,
			"start_request_key": in.RequestKey, "start_request_hash": digest, "start_receipt_json": mustJSON(out),
		}).Error; err != nil {
			return err
		}
		return auditBatchDetails(tx, v, actor, "publication_batch.start", out)
	}()
	if err != nil {
		return model.BatchStartResult{}, err
	}
	return out, nil
}

func equalSchedule(a, b *time.Time) bool {
	return a == nil && b == nil || a != nil && b != nil && a.Equal(*b)
}
