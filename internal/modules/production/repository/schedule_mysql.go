package repository

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"gorm.io/gorm"
)

type ComposeScheduleStore struct{}

func (ComposeScheduleStore) ListRuns(strategyID int64, limit int) ([]model.ComposeScheduleRun, error) {
	return listComposeScheduleRuns(database.DB(), strategyID, limit)
}

// FindRun returns the strategy's latest run for a slot — the run whose batch
// detail a page is asking about. Manual run-now and the scheduler can both
// produce a run; the newest carries the skips the operator cares about.
func (ComposeScheduleStore) FindRun(strategyID int64, slot time.Time) (model.ComposeScheduleRun, bool, error) {
	rows, err := database.DB().Raw(`SELECT run_key, compose_strategy_id, trigger_type, schedule_slot, candidate_count, created_count, skipped_json, created_at FROM compose_schedule_runs WHERE compose_strategy_id = ? AND schedule_slot = ? ORDER BY created_at DESC, run_key DESC LIMIT 1`, strategyID, slot).Rows()
	if err != nil {
		return model.ComposeScheduleRun{}, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var run model.ComposeScheduleRun
		var skipped []byte
		if err := rows.Scan(&run.RunKey, &run.StrategyID, &run.TriggerType, &run.ScheduleSlot, &run.CandidateCount, &run.CreatedCount, &skipped, &run.CreatedAt); err != nil {
			return model.ComposeScheduleRun{}, false, err
		}
		if err := json.Unmarshal(skipped, &run.Skipped); err != nil {
			return model.ComposeScheduleRun{}, false, fmt.Errorf("compose schedule run %s has invalid skips: %w", run.RunKey, err)
		}
		return run, true, rows.Err()
	}
	return model.ComposeScheduleRun{}, false, rows.Err()
}

// BatchTasks reads one schedule run's own rows: every task the run created
// for its (strategy, slot), with per-status counts. The counts come from a
// GROUP BY, so partial failures read as partial failures.
func (ComposeScheduleStore) BatchTasks(strategyID int64, slot time.Time) ([]model.ComposeTask, model.ComposeBatchCounts, error) {
	return composeBatchTasks(database.DB(), strategyID, slot)
}

func composeBatchTasks(db *gorm.DB, strategyID int64, slot time.Time) ([]model.ComposeTask, model.ComposeBatchCounts, error) {
	rows, err := db.Raw(`SELECT `+composeTaskColumns+` FROM compose_tasks WHERE compose_strategy_id = ? AND schedule_slot = ? ORDER BY material_id`, strategyID, slot).Rows()
	if err != nil {
		return nil, model.ComposeBatchCounts{}, err
	}
	defer rows.Close()
	tasks := make([]model.ComposeTask, 0)
	for rows.Next() {
		item, err := scanComposeTask(rows)
		if err != nil {
			return nil, model.ComposeBatchCounts{}, err
		}
		tasks = append(tasks, item)
	}
	if err := rows.Err(); err != nil {
		return nil, model.ComposeBatchCounts{}, err
	}
	var counts model.ComposeBatchCounts
	countRows, err := db.Raw(`SELECT status, COUNT(*) FROM compose_tasks WHERE compose_strategy_id = ? AND schedule_slot = ? GROUP BY status`, strategyID, slot).Rows()
	if err != nil {
		return nil, model.ComposeBatchCounts{}, err
	}
	defer countRows.Close()
	for countRows.Next() {
		var status string
		var count int
		if err := countRows.Scan(&status, &count); err != nil {
			return nil, model.ComposeBatchCounts{}, err
		}
		switch status {
		case "pending":
			counts.Pending = count
		case "running":
			counts.Running = count
		case "success":
			counts.Success = count
		case "failed":
			counts.Failed = count
		case "cancelled":
			counts.Cancelled = count
		}
	}
	return tasks, counts, countRows.Err()
}

func listComposeScheduleRuns(db *gorm.DB, strategyID int64, limit int) ([]model.ComposeScheduleRun, error) {
	rows, err := db.Raw(`SELECT run_key, compose_strategy_id, trigger_type, schedule_slot, candidate_count, created_count, skipped_json, created_at FROM compose_schedule_runs WHERE compose_strategy_id = ? ORDER BY created_at DESC, run_key DESC LIMIT ?`, strategyID, limit).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs := make([]model.ComposeScheduleRun, 0)
	for rows.Next() {
		var run model.ComposeScheduleRun
		var skipped []byte
		if err := rows.Scan(&run.RunKey, &run.StrategyID, &run.TriggerType, &run.ScheduleSlot, &run.CandidateCount, &run.CreatedCount, &skipped, &run.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(skipped, &run.Skipped); err != nil {
			return nil, fmt.Errorf("compose schedule run %s has invalid skips: %w", run.RunKey, err)
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

type ScheduleRunOptions struct {
	StrategyID int64
	Now        time.Time
	ManualKey  string
	NextRun    func(model.ComposeStrategy, time.Time) (*time.Time, error)
}

func (ComposeScheduleStore) DueStrategyIDs(now time.Time) ([]int64, error) {
	var ids []int64
	err := database.DB().Raw(`SELECT id FROM compose_strategies WHERE status = 'enabled' AND execution_mode = 'scheduled' AND next_run_at IS NOT NULL AND next_run_at <= ? ORDER BY next_run_at, id LIMIT 100`, now).Scan(&ids).Error
	return ids, err
}

func (ComposeScheduleStore) Run(options ScheduleRunOptions) (model.ComposeScheduleRun, bool, error) {
	return runComposeSchedule(database.DB(), options)
}

func scheduleRunKey(strategyID int64, slot time.Time, manualKey string) string {
	value := fmt.Sprintf("%d|%s|%s", strategyID, slot.UTC().Format(time.RFC3339Nano), manualKey)
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func scheduledTaskKey(runKey string, materialID int64) string {
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s|%d", runKey, materialID)))
	return hex.EncodeToString(digest[:])
}

type scheduleCandidate struct {
	UsageID       int64
	MaterialID    int64
	GameID        sql.NullString
	Status        string
	Title         string
	PublishedAt   sql.NullTime
	LikeCount     int64
	FavoriteCount int64
}

func (candidate scheduleCandidate) facts() model.ComposeCandidateFacts {
	facts := model.ComposeCandidateFacts{LikeCount: candidate.LikeCount, FavoriteCount: candidate.FavoriteCount}
	if candidate.PublishedAt.Valid {
		value := candidate.PublishedAt.Time.UTC()
		facts.PublishedAt = &value
	}
	return facts
}

func runComposeSchedule(db *gorm.DB, options ScheduleRunOptions) (model.ComposeScheduleRun, bool, error) {
	if options.StrategyID <= 0 || options.NextRun == nil {
		return model.ComposeScheduleRun{}, false, errors.New("invalid compose schedule run options")
	}
	now := options.Now.UTC()
	var result model.ComposeScheduleRun
	processed := false
	err := db.Transaction(func(tx *gorm.DB) error {
		strategy, err := scanComposeStrategy(tx.Raw(`SELECT `+strategyColumns+` FROM compose_strategies WHERE id = ? FOR UPDATE`, options.StrategyID).Row())
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		manual := options.ManualKey != ""
		if manual {
			key := scheduleRunKey(strategy.ID, time.Time{}, options.ManualKey)
			if existing, found, err := findComposeScheduleRun(tx, key); err != nil {
				return err
			} else if found {
				result, processed = existing, true
				return nil
			}
		}
		if strategy.Status != model.ComposeStrategyEnabled {
			return nil
		}
		if err := model.ValidateComposePriorityRules(strategy.PriorityRules); err != nil {
			return fmt.Errorf("strategy %d has invalid rules: %w", strategy.ID, err)
		}
		if !manual && (strategy.ExecutionMode != model.ComposeExecutionScheduled || strategy.Schedule == nil || strategy.NextRunAt == nil || strategy.NextRunAt.After(now)) {
			return nil
		}
		slot := now
		trigger := "manual"
		if !manual {
			slot = strategy.NextRunAt.UTC()
			trigger = "scheduled"
		}
		runKey := scheduleRunKey(strategy.ID, slot, options.ManualKey)
		if manual {
			// The client key is the stable identity of one immediate command.
			runKey = scheduleRunKey(strategy.ID, time.Time{}, options.ManualKey)
		}
		result = model.ComposeScheduleRun{RunKey: runKey, StrategyID: strategy.ID, TriggerType: trigger, ScheduleSlot: slot, Skipped: []model.ComposeScheduleSkip{}, CreatedAt: now}
		processed = true
		var creatorStatus string
		if err := tx.Raw(`SELECT status FROM users WHERE id = ?`, strategy.CreatedBy).Row().Scan(&creatorStatus); err != nil {
			return err
		}
		if creatorStatus != "enabled" {
			result.Skipped = append(result.Skipped, model.ComposeScheduleSkip{Reason: "creator_disabled"})
			return finishComposeScheduleRun(tx, strategy, result, now, manual, options.NextRun)
		}
		var templateCode, templateStatus string
		if err := tx.Raw(`SELECT code, status FROM compose_templates WHERE id = ? AND version = ?`, strategy.TemplateID, strategy.TemplateVersion).Row().Scan(&templateCode, &templateStatus); err != nil {
			return err
		}
		if templateStatus != "active" {
			result.Skipped = append(result.Skipped, model.ComposeScheduleSkip{Reason: "template_unavailable"})
			return finishComposeScheduleRun(tx, strategy, result, now, manual, options.NextRun)
		}
		candidates, err := listComposeScheduleCandidates(tx, strategy)
		if err != nil {
			return err
		}
		result.CandidateCount = len(candidates)
		zone := "Asia/Shanghai"
		if strategy.Schedule != nil {
			zone = strategy.Schedule.Timezone
		}
		location, err := time.LoadLocation(zone)
		if err != nil {
			return err
		}
		local := now.In(location)
		dayStart := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
		var dailyCount int
		if err := tx.Raw(`SELECT COUNT(*) FROM compose_tasks WHERE compose_strategy_id = ? AND schedule_slot IS NOT NULL AND created_at >= ? AND created_at < ?`, strategy.ID, dayStart.UTC(), dayStart.AddDate(0, 0, 1).UTC()).Row().Scan(&dailyCount); err != nil {
			return err
		}
		pool := make([]scheduleCandidate, 0, len(candidates))
		for _, candidate := range candidates {
			reason, err := composeCandidateSkipReason(tx, strategy, candidate, 0, 0)
			if err != nil {
				return err
			}
			if reason != "" {
				result.Skipped = append(result.Skipped, model.ComposeScheduleSkip{MaterialID: candidate.MaterialID, Reason: reason})
				continue
			}
			pool = append(pool, candidate)
		}
		created, skipped, err := executeComposePriorityRules(strategy.PriorityRules, pool, now, strategy.SingleLimit, strategy.DailyLimit-dailyCount, func(candidate scheduleCandidate) (bool, error) {
			task := model.ComposeTask{TeamID: strategy.TeamID, GameID: strategy.GameID, TriggerType: trigger,
				DisplayName: model.ComposeTaskDisplayName(candidate.Title, candidate.MaterialID, strategy.Name),
				MaterialID:  candidate.MaterialID, MaterialUsageID: candidate.UsageID, StrategyID: strategy.ID,
				TemplateCode: templateCode, TemplateVersion: strategy.TemplateVersion, ConfigSnapshot: append(json.RawMessage(nil), strategy.Config...),
				Status: model.ComposeTaskPending, RequestKey: scheduledTaskKey(runKey, candidate.MaterialID), CreatedBy: strategy.CreatedBy}
			if err := tx.SavePoint("compose_candidate").Error; err != nil {
				return false, err
			}
			if err := composeTaskInsertWithAudit(tx, &task, "compose_task.schedule", strategy.CreatedBy, now); err != nil {
				if rollbackErr := tx.RollbackTo("compose_candidate").Error; rollbackErr != nil {
					return false, errors.Join(err, rollbackErr)
				}
				return false, nil
			}
			if err := tx.Exec(`UPDATE compose_tasks SET schedule_slot = ? WHERE id = ?`, slot, task.ID).Error; err != nil {
				if rollbackErr := tx.RollbackTo("compose_candidate").Error; rollbackErr != nil {
					return false, errors.Join(err, rollbackErr)
				}
				return false, nil
			}
			return true, nil
		})
		if err != nil {
			return err
		}
		result.CreatedCount = created
		result.Skipped = append(result.Skipped, skipped...)
		return finishComposeScheduleRun(tx, strategy, result, now, manual, options.NextRun)
	})
	if err != nil {
		return model.ComposeScheduleRun{}, false, err
	}
	return result, processed, nil
}

// executeComposePriorityRules owns ordering, group semantics and quotas for both
// immediate and scheduled runs. The callback creates one task with its audit.
func executeComposePriorityRules(rules []model.ComposePriorityRule, pool []scheduleCandidate, now time.Time, singleLimit, remainingDaily int, create func(scheduleCandidate) (bool, error)) (int, []model.ComposeScheduleSkip, error) {
	ordered := append([]model.ComposePriorityRule(nil), rules...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Priority < ordered[j].Priority })
	created := 0
	skipped := make([]model.ComposeScheduleSkip, 0)
	for _, rule := range ordered {
		remaining := make([]scheduleCandidate, 0, len(pool))
		for _, candidate := range pool {
			if !model.ComposeGroupPredicate(rule, candidate.facts(), now) {
				remaining = append(remaining, candidate)
				continue
			}
			if rule.Action == "filter" {
				skipped = append(skipped, model.ComposeScheduleSkip{MaterialID: candidate.MaterialID, Reason: "filtered"})
				continue
			}
			if created >= singleLimit || created >= remainingDaily {
				reason := "per_run_limit"
				if created >= remainingDaily {
					reason = "daily_limit"
				}
				skipped = append(skipped, model.ComposeScheduleSkip{MaterialID: candidate.MaterialID, Reason: reason})
				remaining = append(remaining, candidate)
				continue
			}
			ok, err := create(candidate)
			if err != nil {
				return created, skipped, err
			}
			if !ok {
				skipped = append(skipped, model.ComposeScheduleSkip{MaterialID: candidate.MaterialID, Reason: "task_create_failed"})
				remaining = append(remaining, candidate)
				continue
			}
			created++
		}
		pool = remaining
		if created >= singleLimit || created >= remainingDaily {
			break
		}
	}
	return created, skipped, nil
}

func listComposeScheduleCandidates(tx *gorm.DB, strategy model.ComposeStrategy) ([]scheduleCandidate, error) {
	rows, err := tx.Raw(`SELECT mu.id, mu.material_id, sc.game_id, m.status, m.title, sc.published_at, sc.like_count, sc.favorite_count FROM material_usages mu JOIN materials m ON m.id = mu.material_id JOIN source_contents sc ON sc.id = m.source_content_id WHERE mu.user_id = ? AND mu.team_id = ? AND m.team_id = ? AND mu.status = 'active' ORDER BY mu.id`, strategy.CreatedBy, strategy.TeamID, strategy.TeamID).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]scheduleCandidate, 0)
	for rows.Next() {
		var item scheduleCandidate
		if err := rows.Scan(&item.UsageID, &item.MaterialID, &item.GameID, &item.Status, &item.Title, &item.PublishedAt, &item.LikeCount, &item.FavoriteCount); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func composeCandidateSkipReason(tx *gorm.DB, strategy model.ComposeStrategy, candidate scheduleCandidate, createdCount, dailyCount int) (string, error) {
	if !candidate.GameID.Valid || candidate.GameID.String != strategy.GameID {
		return "other_game", nil
	}
	if candidate.Status != "available" {
		return "material_unavailable", nil
	}
	// Any task row at all — queued, produced, failed or cancelled — is the one
	// reason to leave the pair alone: it was run once, and running it again is
	// the retry action on that task. This is the same rule the create guard
	// applies, so the automatic and the manual path offer the same pairs.
	var taskCount int
	if err := tx.Raw(`SELECT COUNT(*) FROM compose_tasks WHERE compose_strategy_id = ? AND material_id = ?`, strategy.ID, candidate.MaterialID).Row().Scan(&taskCount); err != nil {
		return "", err
	}
	if taskCount > 0 {
		return "previously_run", nil
	}
	if dailyCount >= strategy.DailyLimit {
		return "daily_limit", nil
	}
	if createdCount >= strategy.SingleLimit {
		return "per_run_limit", nil
	}
	return "", nil
}

func finishComposeScheduleRun(tx *gorm.DB, strategy model.ComposeStrategy, result model.ComposeScheduleRun, now time.Time, manual bool, calculate func(model.ComposeStrategy, time.Time) (*time.Time, error)) error {
	skips, err := json.Marshal(result.Skipped)
	if err != nil {
		return err
	}
	if err := tx.Exec(`INSERT INTO compose_schedule_runs (run_key, compose_strategy_id, trigger_type, schedule_slot, candidate_count, created_count, skipped_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, result.RunKey, strategy.ID, result.TriggerType, result.ScheduleSlot, result.CandidateCount, result.CreatedCount, skips, now).Error; err != nil {
		return err
	}
	if !manual {
		next, err := calculate(strategy, now)
		if err != nil {
			return err
		}
		if err := tx.Exec(`UPDATE compose_strategies SET next_run_at = ? WHERE id = ?`, next, strategy.ID).Error; err != nil {
			return err
		}
	}
	return nil
}

func findComposeScheduleRun(tx *gorm.DB, key string) (model.ComposeScheduleRun, bool, error) {
	var result model.ComposeScheduleRun
	var skipped []byte
	err := tx.Raw(`SELECT run_key, compose_strategy_id, trigger_type, schedule_slot, candidate_count, created_count, skipped_json, created_at FROM compose_schedule_runs WHERE run_key = ?`, key).Row().Scan(&result.RunKey, &result.StrategyID, &result.TriggerType, &result.ScheduleSlot, &result.CandidateCount, &result.CreatedCount, &skipped, &result.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ComposeScheduleRun{}, false, nil
	}
	if err != nil {
		return model.ComposeScheduleRun{}, false, err
	}
	if err := json.Unmarshal(skipped, &result.Skipped); err != nil {
		return model.ComposeScheduleRun{}, false, err
	}
	return result, true, nil
}
