package model

import (
	"encoding/json"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
	"time"
)

type ComposeTaskBatch struct {
	OperatorName   string                    `json:"operator_name" gorm:"-"`
	StrategyName   string                    `json:"strategy_name" gorm:"-"`
	ID             int64                     `json:"id"`
	Name           string                    `json:"name"`
	TeamID         int64                     `json:"team_id"`
	CategoryID     sharedidentity.CategoryID `json:"category_id"`
	OperatorID     *int64                    `json:"operator_id"`
	StrategyID     int64                     `json:"compose_strategy_id" gorm:"column:compose_strategy_id"`
	TriggerType    string                    `json:"trigger_type"`
	CreatedBy      int64                     `json:"created_by"`
	RequestKey     string                    `json:"-"`
	RequestHash    string                    `json:"-"`
	ConfigSnapshot json.RawMessage           `json:"config_snapshot" gorm:"column:config_snapshot_json;type:json"`
	CreatedAt      time.Time                 `json:"created_at"`
	Counts         ComposeBatchCounts        `json:"counts" gorm:"-"`
}

func (ComposeTaskBatch) TableName() string { return "compose_task_batches" }
func (c ComposeBatchCounts) Total() int {
	return c.Pending + c.Running + c.Success + c.Failed + c.Cancelled
}
func (c ComposeBatchCounts) ProgressPercent() int {
	if c.Total() == 0 {
		return 0
	}
	return (c.Success + c.Failed + c.Cancelled) * 100 / c.Total()
}

// A batch is a task snapshot projection and follows the task address redaction.
func (b ComposeTaskBatch) MarshalJSON() ([]byte, error) {
	type projection ComposeTaskBatch
	redact := func(groups []ComposeOutputSetting) []ComposeOutputSetting {
		for i := range groups {
			raw, _ := json.Marshal(groups[i].Params)
			safe := safeComposeSnapshot(raw)
			_ = json.Unmarshal(safe, &groups[i].Params)
		}
		return groups
	}
	var groups []ComposeOutputSetting
	if json.Unmarshal(b.ConfigSnapshot, &groups) == nil && groups != nil {
		b.ConfigSnapshot, _ = json.Marshal(redact(groups))
	} else {
		var snapshot struct {
			SourceOutputID int64                  `json:"source_output_id,omitempty"`
			SourceTaskID   int64                  `json:"source_task_id,omitempty"`
			Settings       []ComposeOutputSetting `json:"settings"`
			Acceptance     json.RawMessage        `json:"acceptance"`
		}
		if json.Unmarshal(b.ConfigSnapshot, &snapshot) == nil && snapshot.Settings != nil {
			snapshot.Settings = redact(snapshot.Settings)
			b.ConfigSnapshot, _ = json.Marshal(snapshot)
		} else {
			b.ConfigSnapshot = json.RawMessage(`[]`)
		}
	}
	return json.Marshal(projection(b))
}
