package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"time"
)

type TaskRevision struct {
	TaskID   uint64 `json:"task_id,string"`
	Revision uint64 `json:"revision,string"`
}

type BatchException struct {
	TaskRevision
	Status Status `json:"status"`
	Code   string `json:"code,omitempty"`
}

type BatchPause struct {
	ID         uint64           `json:"id,string"`
	Kind       string           `json:"kind"`
	Reason     string           `json:"reason"`
	PausedAt   time.Time        `json:"paused_at"`
	Exceptions []BatchException `json:"exceptions"`
}

type BatchExecution struct {
	VerificationRequiredCount int64            `json:"verification_required_count"`
	Phase                     string           `json:"phase"`
	FirstStartedAt            *time.Time       `json:"first_started_at"`
	GroupNo                   *uint32          `json:"execution_group_no"`
	TaskIDs                   []string         `json:"group_task_ids"`
	GroupEndedAt              *time.Time       `json:"group_ended_at"`
	NextGroupAt               *time.Time       `json:"next_group_at"`
	Pause                     *BatchPause      `json:"pause"`
	StoppedAt                 *time.Time       `json:"stopped_at"`
	EndedAt                   *time.Time       `json:"ended_at"`
	StopReason                string           `json:"stop_reason,omitempty"`
	FinishReason              string           `json:"finish_reason,omitempty"`
	TargetCount               int64            `json:"target_count"`
	CreatedCount              int64            `json:"created_count"`
	CreationShortfall         *int64           `json:"creation_shortfall"`
	PublishedCount            int64            `json:"published_count"`
	TargetShortfall           int64            `json:"target_shortfall"`
	TaskCounts                map[Status]int64 `json:"task_counts"`
}

type CreationShortfall struct {
	CategoryID   int64 `json:"category_id,string"`
	TargetCount  int64 `json:"target_count"`
	CreatedCount int64 `json:"created_count"`
	Shortfall    int64 `json:"shortfall"`
}

type ExpectedShortfall struct {
	CategoryID int64 `json:"category_id,string"`
	Shortfall  int64 `json:"shortfall"`
}

type BatchStartResult struct {
	BatchID           uint64    `json:"batch_id,string"`
	Revision          uint64    `json:"revision,string"`
	ExecutionStatus   string    `json:"execution_status"`
	FirstStartedAt    time.Time `json:"first_started_at"`
	TaskIDs           []string  `json:"task_ids"`
	TargetCount       int64     `json:"target_count"`
	CreatedCount      int64     `json:"created_count"`
	CreationShortfall int64     `json:"creation_shortfall"`

	invalidJSON bool
}

// UnmarshalJSON keeps storage corruption distinct from an explicit zero value.
// A damaged historical receipt must not prevent reading its parent batch, but
// it cannot prove whether a start request was applied or be replayed as success.
func (r *BatchStartResult) UnmarshalJSON(data []byte) error {
	type wire BatchStartResult
	var decoded wire
	var fields map[string]json.RawMessage
	valid := json.Unmarshal(data, &decoded) == nil && json.Unmarshal(data, &fields) == nil
	for _, key := range []string{"batch_id", "revision", "execution_status", "first_started_at", "task_ids", "target_count", "created_count", "creation_shortfall"} {
		value, present := fields[key]
		if !present || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			valid = false
		}
	}
	*r = BatchStartResult(decoded)
	r.invalidJSON = !valid
	return nil
}

// HasCompleteJSON also accepts newly constructed results before persistence.
func (r BatchStartResult) HasCompleteJSON() bool { return !r.invalidJSON }

type BatchControlResult struct {
	BatchID         uint64         `json:"batch_id,string"`
	Revision        uint64         `json:"revision,string"`
	ExecutionStatus string         `json:"execution_status"`
	Execution       BatchExecution `json:"execution"`
}

type BatchStopResult struct {
	BatchControlResult
	CancelledUnclaimedCount int64 `json:"cancelled_unclaimed_count"`
	InflightCount           int64 `json:"inflight_count"`
}

type BatchStartInput struct {
	RequestKey         string              `json:"request_key"`
	ExpectedRevision   uint64              `json:"expected_revision,string"`
	IncludedRowKeys    []string            `json:"included_row_keys"`
	ExcludedRowKeys    []string            `json:"excluded_row_keys"`
	AllowShortfall     bool                `json:"allow_shortfall"`
	ExpectedShortfalls []ExpectedShortfall `json:"expected_shortfalls"`
}

var ErrInvalidBatchSelection = errors.New("invalid batch row set or target confirmation")

func BatchPreviewSelection(v Batch, keys []string) ([]BatchCandidate, []CreationShortfall, error) {
	selected := make(map[string]bool, len(keys))
	for _, key := range keys {
		if key == "" || selected[key] {
			return nil, nil, ErrInvalidBatchSelection
		}
		selected[key] = true
	}
	if len(selected) == 0 {
		return nil, nil, ErrInvalidBatchSelection
	}
	counts := make(map[int64]int64)
	rows := make([]BatchCandidate, 0, len(keys))
	for _, row := range v.Config.Candidates {
		if selected[row.RowKey] {
			rows = append(rows, row)
			counts[row.CategoryID]++
			delete(selected, row.RowKey)
		}
	}
	if len(selected) > 0 {
		return nil, nil, ErrInvalidBatchSelection
	}
	gaps := make([]CreationShortfall, 0, len(v.Targets))
	for _, target := range v.Targets {
		count := counts[target.CategoryID]
		gaps = append(gaps, CreationShortfall{CategoryID: target.CategoryID, TargetCount: target.TargetCount, CreatedCount: count, Shortfall: max(0, target.TargetCount-count)})
	}
	return rows, gaps, nil
}

func BatchStartSelection(v Batch, in BatchStartInput) ([]BatchCandidate, []CreationShortfall, error) {
	rows, gaps, err := BatchPreviewSelection(v, in.IncludedRowKeys)
	if err != nil {
		return nil, gaps, err
	}
	remaining := make(map[string]bool, len(v.Config.Candidates))
	for _, row := range v.Config.Candidates {
		remaining[row.RowKey] = true
	}
	for _, row := range rows {
		delete(remaining, row.RowKey)
	}
	for _, key := range in.ExcludedRowKeys {
		if !remaining[key] {
			return nil, gaps, ErrInvalidBatchSelection
		}
		delete(remaining, key)
	}
	if len(remaining) != 0 || len(in.ExpectedShortfalls) != len(gaps) {
		return nil, gaps, ErrInvalidBatchSelection
	}
	expected := make(map[int64]int64, len(gaps))
	for _, item := range in.ExpectedShortfalls {
		if _, found := expected[item.CategoryID]; found {
			return nil, gaps, ErrInvalidBatchSelection
		}
		expected[item.CategoryID] = item.Shortfall
	}
	for _, gap := range gaps {
		n, found := expected[gap.CategoryID]
		if !found || n != gap.Shortfall || gap.CreatedCount > gap.TargetCount || n > 0 && !in.AllowShortfall {
			return nil, gaps, ErrInvalidBatchSelection
		}
	}
	return rows, gaps, nil
}
