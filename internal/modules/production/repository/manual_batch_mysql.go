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
	identity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
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
	ResolveTechnicalConfig          func(string, uint32) (json.RawMessage, error)
	SourceOutputID                  int64
	ActorID, StrategyID, OperatorID int64
	Name, RequestKey, IntentHash    string
	MaterialIDs                     []int64
	RemakeConfirmed                 bool
	Authorize                       func(model.ComposeStrategy) error
	AuthorizeSource                 func(model.CompositeOutput) error
	ValidateSourceInput             func(model.Material) error
	ResolveSettings                 func(model.ComposeStrategy, model.ComposeTemplate) ([]model.ComposeOutputSetting, error)
}

type manualBatchSnapshot struct {
	SourceOutputID int64                        `json:"source_output_id,omitempty"`
	SourceTaskID   int64                        `json:"source_task_id,omitempty"`
	Settings       []model.ComposeOutputSetting `json:"settings"`
	Acceptance     []ManualBatchFailure         `json:"acceptance"`
}

func (ComposeTaskStore) CreateManualBatch(command ManualBatchCommand) (ManualBatchResult, error) {
	return createManualBatch(database.DB(), command, time.Now().UTC())
}

func createManualBatch(db *gorm.DB, command ManualBatchCommand, now time.Time) (ManualBatchResult, error) {
	result := ManualBatchResult{Created: []model.ComposeTask{}, Failures: []ManualBatchFailure{}}
	isSource := command.SourceOutputID > 0
	if command.ActorID <= 0 || command.SourceOutputID < 0 || command.StrategyID < 0 || command.OperatorID < 0 || (!isSource && (command.StrategyID == 0 || len(command.MaterialIDs) == 0)) || command.Authorize == nil || command.ResolveSettings == nil || (isSource && (command.AuthorizeSource == nil || command.ValidateSourceInput == nil)) {
		return result, ErrManualBatchInvalid
	}
	var transactionOptions []*sql.TxOptions
	if isSource {
		// Source discovery must precede the strategy lock. Read committed keeps
		// later replay reads current when a competing source command commits.
		transactionOptions = append(transactionOptions, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		var sourceOutput model.CompositeOutput
		var sourceTask model.ComposeTask
		var sourceSetting model.ComposeOutputSetting
		if isSource {
			// A persisted source command can replay even after its source or current
			// strategy changes. The batch's immutable scope still gates access.
			if replay, found, err := replayManualSourceBatch(tx, command); err != nil || found {
				result = replay
				return err
			}
			var found bool
			var err error
			sourceOutput, found, err = findComposeOutput(tx, command.SourceOutputID)
			if err != nil {
				return err
			}
			if !found {
				return ErrNotFound
			}
			if err = command.AuthorizeSource(sourceOutput); err != nil {
				return err
			}
			sourceTask, found, err = findComposeTask(tx, sourceOutput.TaskID)
			if err != nil {
				return err
			}
			if !found {
				return fmt.Errorf("%w: original task history is missing; use normal creation", ErrManualBatchInvalid)
			}
			sourceSetting, err = ManualSourceSetting(sourceOutput, sourceTask)
			if err != nil {
				return err
			}
			if (command.StrategyID > 0 && command.StrategyID != sourceOutput.StrategyID) || (command.OperatorID > 0 && command.OperatorID != *sourceOutput.OperatorID) || len(command.MaterialIDs) > 0 {
				return fmt.Errorf("%w: supplied ownership does not match original output", ErrManualBatchInvalid)
			}
			command.StrategyID = sourceOutput.StrategyID
			command.OperatorID = *sourceOutput.OperatorID
			command.MaterialIDs = []int64{sourceOutput.MaterialID}
			command.RemakeConfirmed = true
		}
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
		batchQuery := `SELECT id FROM compose_task_batches WHERE request_key=?`
		if isSource {
			// The preflight above opened a read snapshot; use a current read after
			// waiting on the strategy lock so a concurrent identical command replays.
			batchQuery += ` FOR UPDATE`
		}
		err := tx.Raw(batchQuery, command.RequestKey).Row().Scan(&batchID)
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
			if snapshot.SourceOutputID != command.SourceOutputID {
				return ErrManualBatchConflict
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
		strategyQuery := `SELECT ` + strategyColumns + ` FROM compose_strategies WHERE id=?`
		if isSource {
			strategyQuery += ` FOR UPDATE`
		}
		strategy, err := scanComposeStrategy(tx.Raw(strategyQuery, strategyID).Row())
		if err != nil {
			return err
		}
		if err = command.Authorize(strategy); err != nil {
			return err
		}
		if strategy.Status != model.ComposeStrategyEnabled || strategy.OperatorID == nil || *strategy.OperatorID <= 0 || (command.OperatorID > 0 && command.OperatorID != *strategy.OperatorID) {
			return ErrManualBatchInvalid
		}
		if isSource {
			if err = ValidateManualSourceStrategy(sourceOutput, strategy); err != nil {
				return err
			}
		}
		var template model.ComposeTemplate
		var schema []byte
		templateQuery := `SELECT id, code, name, version, schema_json, executor, status FROM compose_templates WHERE id=? AND version=? FOR UPDATE`
		templateArgs := []any{strategy.TemplateID, strategy.TemplateVersion}
		if isSource {
			templateQuery = `SELECT id, code, name, version, schema_json, executor, status FROM compose_templates WHERE code=? AND version=? FOR UPDATE`
			templateArgs = []any{sourceTask.TemplateCode, sourceTask.TemplateVersion}
		}
		err = tx.Raw(templateQuery, templateArgs...).Row().Scan(&template.ID, &template.Code, &template.Name, &template.Version, &schema, &template.ExecutorID, &template.Status)
		if err != nil {
			if isSource && errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w: original template version is unavailable", ErrManualBatchInvalid)
			}
			return err
		}
		template.Schema = append(json.RawMessage(nil), schema...)
		settingStrategy := strategy
		if isSource {
			settingStrategy.OutputSettings = []model.ComposeOutputSetting{sourceSetting}
			settingStrategy.OutputSettingsJSON = nil
		}
		groups, err := command.ResolveSettings(settingStrategy, template)
		if err != nil {
			return err
		}
		groups, err = model.NormalizeComposeOutputSettings(groups)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrManualBatchInvalid, err)
		}
		if isSource && (len(groups) != 1 || groups[0].Key != sourceSetting.Key || groups[0].Name != sourceSetting.Name || groups[0].Copies != 1) {
			return fmt.Errorf("%w: source output must retain its original single setting", ErrManualBatchInvalid)
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
			if !isSource && !(videoStatus == "ready" && objectKey.String != "") && sourceURL.String == "" {
				reject(materialID, "input_unavailable", "素材没有可用视频或可准备的来源")
				continue
			}
			if isSource {
				if err = command.ValidateSourceInput(model.Material{ID: materialID, TeamID: identity.TeamID(teamID), Status: model.MaterialAvailable, VideoStatus: model.VideoStatus(videoStatus), SourceURL: sourceURL.String, SourceObjectKey: objectKey.String}); err != nil {
					return err
				}
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
		snapshot, err := json.Marshal(manualBatchSnapshot{SourceOutputID: command.SourceOutputID, SourceTaskID: sourceTask.ID, Settings: groups, Acceptance: result.Failures})
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
					var technical json.RawMessage
					if command.ResolveTechnicalConfig != nil {
						var e error
						technical, e = command.ResolveTechnicalConfig(template.Code, template.Version)
						if e != nil {
							return e
						}
					}
					task := model.ComposeTask{BatchID: &batch.ID, SettingKey: group.Key, SettingName: group.Name, PlatformsJSON: platforms, CopyIndex: copyIndex, DisplayName: manualTaskName(command.Name, material.Title, group.Name, copyIndex), TeamID: strategy.TeamID, CategoryID: strategy.CategoryID, OperatorID: strategy.OperatorID, TriggerType: "manual", MaterialID: material.ID, MaterialUsageID: &material.UsageID, StrategyID: strategy.ID, TemplateCode: template.Code, TemplateVersion: template.Version, TechnicalConfig: technical, ConfigSnapshot: params, Status: model.ComposeTaskPending, RequestKey: hex.EncodeToString(digest[:]), CreatedBy: command.ActorID}
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
	}, transactionOptions...)
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
