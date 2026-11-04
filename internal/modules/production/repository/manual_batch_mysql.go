package repository

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"gorm.io/gorm"
)

var ErrManualBatchConflict = errors.New("manual batch request_key has different content")
var ErrManualBatchInvalid = errors.New("manual batch input is invalid")

type ManualBatchFailure struct {
	MaterialID int64  `json:"material_id"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}
type ManualBatchResult struct {
	Batch    *model.ComposeTaskBatch
	Created  []model.ComposeTask
	Failures []ManualBatchFailure
	Replayed bool
}
type ManualBatchCommand struct {
	ActorID, StrategyID, OperatorID int64
	Name, RequestKey, IntentHash    string
	MaterialIDs                     []int64
	RemakeConfirmed                 bool
	Authorize                       func(model.ComposeStrategy) error
	ResolveSettings                 func(model.ComposeStrategy, model.ComposeTemplate) ([]model.ComposeOutputSetting, error)
}

type manualBatchSnapshot struct {
	Settings   []model.ComposeOutputSetting `json:"settings"`
	Acceptance []ManualBatchFailure         `json:"acceptance"`
}

func (ComposeTaskStore) CreateManualBatch(command ManualBatchCommand) (ManualBatchResult, error) {
	return createManualBatch(database.DB(), command, time.Now().UTC())
}

func createManualBatch(db *gorm.DB, command ManualBatchCommand, now time.Time) (ManualBatchResult, error) {
	result := ManualBatchResult{Created: []model.ComposeTask{}, Failures: []ManualBatchFailure{}}
	if command.ActorID <= 0 || command.StrategyID <= 0 || len(command.MaterialIDs) == 0 || command.Authorize == nil || command.ResolveSettings == nil {
		return result, ErrManualBatchInvalid
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		// All creation sources lock strategy first. Resolve replays before mutable
		// template/assignment/usage state; the immutable batch supplies its own scope.
		var strategyID int64
		if err := tx.Raw(`SELECT id FROM compose_strategies WHERE id=? FOR UPDATE`, command.StrategyID).Row().Scan(&strategyID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		var batchID int64
		err := tx.Raw(`SELECT id FROM compose_task_batches WHERE request_key=?`, command.RequestKey).Row().Scan(&batchID)
		if err == nil {
			batch, found, err := findComposeTaskBatch(tx, batchID)
			if err != nil {
				return err
			}
			if !found {
				return ErrNotFound
			}
			if batch.CreatedBy != command.ActorID || batch.StrategyID != command.StrategyID || batch.RequestHash != command.IntentHash {
				return ErrManualBatchConflict
			}
			if err = command.Authorize(model.ComposeStrategy{ID: batch.StrategyID, TeamID: batch.TeamID, CategoryID: batch.CategoryID, OperatorID: batch.OperatorID, Status: model.ComposeStrategyEnabled}); err != nil {
				return err
			}
			var snapshot manualBatchSnapshot
			if err = json.Unmarshal(batch.ConfigSnapshot, &snapshot); err != nil {
				return err
			}
			tasks, err := manualBatchTasks(tx, batch.ID)
			if err != nil {
				return err
			}
			result = ManualBatchResult{Batch: &batch, Created: tasks, Failures: snapshot.Acceptance, Replayed: true}
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		strategy, err := scanComposeStrategy(tx.Raw(`SELECT `+strategyColumns+` FROM compose_strategies WHERE id=?`, strategyID).Row())
		if err != nil {
			return err
		}
		if err = command.Authorize(strategy); err != nil {
			return err
		}
		if strategy.Status != model.ComposeStrategyEnabled || strategy.OperatorID == nil || *strategy.OperatorID <= 0 || (command.OperatorID > 0 && command.OperatorID != *strategy.OperatorID) {
			return ErrManualBatchInvalid
		}
		var template model.ComposeTemplate
		var schema []byte
		err = tx.Raw(`SELECT id, code, name, version, schema_json, executor, status FROM compose_templates WHERE id=? AND version=? FOR UPDATE`, strategy.TemplateID, strategy.TemplateVersion).Row().Scan(&template.ID, &template.Code, &template.Name, &template.Version, &schema, &template.ExecutorID, &template.Status)
		if err != nil {
			return err
		}
		template.Schema = append(json.RawMessage(nil), schema...)
		groups, err := command.ResolveSettings(strategy, template)
		if err != nil {
			return err
		}
		groups, err = model.NormalizeComposeOutputSettings(groups)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrManualBatchInvalid, err)
		}
		var owner int64
		err = tx.Raw(`SELECT u.id FROM users u JOIN user_category_scopes s ON s.user_id=u.id WHERE u.id=? AND u.team_id=? AND s.category_id=? AND u.status='enabled' AND u.role IN ('operator','senior_operator') LIMIT 1 FOR UPDATE`, *strategy.OperatorID, strategy.TeamID, strategy.CategoryID).Row().Scan(&owner)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrManualBatchInvalid
		}
		if err != nil {
			return err
		}
		type acceptedMaterial struct {
			ID, UsageID int64
			Title       string
		}
		accepted := []acceptedMaterial{}
		reject := func(id int64, code, message string) {
			result.Failures = append(result.Failures, ManualBatchFailure{MaterialID: id, Code: code, Message: message})
		}
		for _, materialID := range command.MaterialIDs {
			var teamID int64
			var category sql.NullInt64
			var status, title, videoStatus string
			var objectKey, sourceURL sql.NullString
			err = tx.Raw(`SELECT m.team_id,sc.category_id,m.status,m.title,m.video_status,m.source_object_key,sc.source_url FROM materials m JOIN source_contents sc ON sc.id=m.source_content_id WHERE m.id=? FOR UPDATE`, materialID).Row().Scan(&teamID, &category, &status, &title, &videoStatus, &objectKey, &sourceURL)
			if errors.Is(err, sql.ErrNoRows) {
				reject(materialID, "invalid", "素材不存在")
				continue
			}
			if err != nil {
				return err
			}
			if teamID != strategy.TeamID || !category.Valid || category.Int64 != int64(strategy.CategoryID) {
				reject(materialID, "forbidden", "素材不在策略的团队或类目范围内")
				continue
			}
			if status != "available" {
				reject(materialID, "invalid", "素材当前不可选用")
				continue
			}
			if !(videoStatus == "ready" && objectKey.String != "") && sourceURL.String == "" {
				reject(materialID, "input_unavailable", "素材没有可用视频或可准备的来源")
				continue
			}
			var usage model.MaterialUsage
			err = scanUsage(tx.Raw(`SELECT id, team_id, material_id, user_id, status, removed_at, created_at, updated_at FROM material_usages WHERE user_id=? AND material_id=? AND status='active' FOR UPDATE`, owner, materialID).Row(), &usage)
			if errors.Is(err, sql.ErrNoRows) {
				reject(materialID, "usage_unavailable", "素材不在任务负责人的个人素材中或已移出")
				continue
			}
			if err != nil {
				return err
			}
			if int64(usage.TeamID) != strategy.TeamID {
				reject(materialID, "forbidden", "个人素材关系与策略团队不匹配")
				continue
			}
			if !command.RemakeConfirmed {
				var previous int64
				err = tx.Raw(`SELECT id FROM compose_tasks WHERE compose_strategy_id=? AND material_id=? LIMIT 1 FOR UPDATE`, strategy.ID, materialID).Row().Scan(&previous)
				if err == nil {
					reject(materialID, "remake_required", "该素材已有任务记录，请明确确认再次制作")
					continue
				}
				if !errors.Is(err, sql.ErrNoRows) {
					return err
				}
			}
			accepted = append(accepted, acceptedMaterial{ID: materialID, UsageID: usage.ID, Title: title})
		}
		if len(accepted) == 0 {
			return nil
		}
		snapshot, err := json.Marshal(manualBatchSnapshot{Settings: groups, Acceptance: result.Failures})
		if err != nil {
			return err
		}
		batch := model.ComposeTaskBatch{Name: command.Name, TeamID: strategy.TeamID, CategoryID: strategy.CategoryID, OperatorID: strategy.OperatorID, StrategyID: strategy.ID, TriggerType: "manual", CreatedBy: command.ActorID, RequestKey: command.RequestKey, RequestHash: command.IntentHash, ConfigSnapshot: snapshot}
		if err = insertComposeTaskBatch(tx, &batch, now); err != nil {
			// Another strategy command with the same actor/key can win the global
			// unique key while holding a different strategy lock. It is a content
			// conflict, never an internal failure or a second batch.
			var duplicate *mysql.MySQLError
			if errors.As(err, &duplicate) && duplicate.Number == 1062 {
				return ErrManualBatchConflict
			}
			return err
		}
		for _, material := range accepted {
			for _, group := range groups {
				for copyIndex := 1; copyIndex <= group.Copies; copyIndex++ {
					params, _ := json.Marshal(group.Params)
					platforms, _ := json.Marshal(group.Platforms)
					digest := sha256.Sum256([]byte(fmt.Sprintf("manual:%d:%d:%s:%d", batch.ID, material.ID, group.Key, copyIndex)))
					task := model.ComposeTask{BatchID: &batch.ID, SettingKey: group.Key, SettingName: group.Name, PlatformsJSON: platforms, CopyIndex: copyIndex, DisplayName: manualTaskName(command.Name, material.Title, group.Name, copyIndex), TeamID: strategy.TeamID, CategoryID: strategy.CategoryID, OperatorID: strategy.OperatorID, TriggerType: "manual", MaterialID: material.ID, MaterialUsageID: &material.UsageID, StrategyID: strategy.ID, TemplateCode: template.Code, TemplateVersion: template.Version, ConfigSnapshot: params, Status: model.ComposeTaskPending, RequestKey: hex.EncodeToString(digest[:]), CreatedBy: command.ActorID}
					if err = composeTaskInsertWithAudit(tx, &task, "compose_task.create", command.ActorID, now); err != nil {
						return err
					}
					result.Created = append(result.Created, task)
				}
			}
		}
		batch.Counts.Pending = len(result.Created)
		result.Batch = &batch
		return nil
	})
	if err != nil {
		return ManualBatchResult{}, err
	}
	return result, nil
}

func manualBatchTasks(db *gorm.DB, batchID int64) ([]model.ComposeTask, error) {
	rows, err := db.Raw(`SELECT `+composeTaskColumns+` FROM compose_tasks WHERE batch_id=? ORDER BY id`, batchID).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := []model.ComposeTask{}
	for rows.Next() {
		task, err := scanComposeTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func manualTaskName(name, title, setting string, copyIndex int) string {
	short := func(value string) string {
		runes := []rune(value)
		if len(runes) > 80 {
			return string(runes[:80]) + "…"
		}
		return value
	}
	return fmt.Sprintf("%s · %s · %s · %d", name, short(title), short(setting), copyIndex)
}
