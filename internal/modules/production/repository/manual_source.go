package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"gorm.io/gorm"
)

// ManualSourceSetting reads the original final task snapshot. It deliberately
// has no fallback to current strategy groups or a guessed historical default.
func ManualSourceSetting(output model.CompositeOutput, task model.ComposeTask) (model.ComposeOutputSetting, error) {
	invalid := func() (model.ComposeOutputSetting, error) {
		return model.ComposeOutputSetting{}, fmt.Errorf("%w: original output has incomplete or inconsistent task history; use normal creation", ErrManualBatchInvalid)
	}
	if output.ID <= 0 || output.TaskID != task.ID || task.ID <= 0 || task.Status != model.ComposeTaskSuccess ||
		output.OperatorID == nil || *output.OperatorID <= 0 || task.OperatorID == nil || *task.OperatorID != *output.OperatorID ||
		output.TeamID != task.TeamID || output.CategoryID != task.CategoryID || output.MaterialID != task.MaterialID ||
		output.StrategyID != task.StrategyID || task.StrategyID <= 0 || task.MaterialID <= 0 ||
		output.TemplateCode != task.TemplateCode || output.TemplateVersion != task.TemplateVersion || task.TemplateCode == "" || task.TemplateVersion == 0 ||
		strings.TrimSpace(task.SettingKey) == "" || strings.TrimSpace(task.SettingName) == "" {
		return invalid()
	}
	var platforms []string
	var params map[string]any
	if json.Unmarshal(task.PlatformsJSON, &platforms) != nil || len(platforms) == 0 || json.Unmarshal(task.ConfigSnapshot, &params) != nil || params == nil {
		return invalid()
	}
	groups, err := model.NormalizeComposeOutputSettings([]model.ComposeOutputSetting{{Key: task.SettingKey, Name: task.SettingName, Platforms: platforms, Copies: 1, Params: params}})
	if err != nil {
		return invalid()
	}
	return groups[0], nil
}

// ValidateManualSourceStrategy permits current parameter/group changes, while
// requiring the original authorization and ownership boundary to remain valid.
func ValidateManualSourceStrategy(output model.CompositeOutput, strategy model.ComposeStrategy) error {
	if strategy.Status != model.ComposeStrategyEnabled || strategy.ID != output.StrategyID ||
		strategy.TeamID != output.TeamID || strategy.CategoryID != output.CategoryID ||
		strategy.OperatorID == nil || output.OperatorID == nil || *strategy.OperatorID != *output.OperatorID {
		return fmt.Errorf("%w: original strategy is disabled or its ownership changed", ErrManualBatchInvalid)
	}
	return nil
}

func replayManualSourceBatch(tx *gorm.DB, command ManualBatchCommand) (ManualBatchResult, bool, error) {
	var batchID int64
	err := tx.Raw(`SELECT id FROM compose_task_batches WHERE request_key=?`, command.RequestKey).Row().Scan(&batchID)
	if errors.Is(err, sql.ErrNoRows) {
		return ManualBatchResult{}, false, nil
	}
	if err != nil {
		return ManualBatchResult{}, false, err
	}
	batch, found, err := findComposeTaskBatch(tx, batchID)
	if err != nil || !found {
		if err == nil {
			err = ErrNotFound
		}
		return ManualBatchResult{}, false, err
	}
	if batch.CreatedBy != command.ActorID || batch.RequestHash != command.IntentHash || (command.StrategyID > 0 && command.StrategyID != batch.StrategyID) {
		return ManualBatchResult{}, false, ErrManualBatchConflict
	}
	var snapshot manualBatchSnapshot
	if err = json.Unmarshal(batch.ConfigSnapshot, &snapshot); err != nil {
		return ManualBatchResult{}, false, err
	}
	if snapshot.SourceOutputID != command.SourceOutputID || snapshot.SourceTaskID <= 0 {
		return ManualBatchResult{}, false, ErrManualBatchConflict
	}
	if err = command.Authorize(model.ComposeStrategy{ID: batch.StrategyID, TeamID: batch.TeamID, CategoryID: batch.CategoryID, OperatorID: batch.OperatorID, Status: model.ComposeStrategyEnabled}); err != nil {
		return ManualBatchResult{}, false, err
	}
	tasks, err := manualBatchTasks(tx, batch.ID)
	if err != nil {
		return ManualBatchResult{}, false, err
	}
	return ManualBatchResult{Batch: &batch, Created: tasks, Failures: snapshot.Acceptance, Replayed: true}, true, nil
}
