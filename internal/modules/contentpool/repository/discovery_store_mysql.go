package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool/model"
	identitymodel "github.com/wt-media/wt-media-cloud/internal/modules/identity/model"
	"gorm.io/gorm"
)

func CreateStrategy(v model.DiscoveryStrategy) (model.DiscoveryStrategy, error) {
	return createStrategy(database.DB(), v)
}
func createStrategy(db *gorm.DB, v model.DiscoveryStrategy) (model.DiscoveryStrategy, error) {
	config, _ := json.Marshal(v.Config)
	result := db.Exec(`INSERT INTO discovery_strategies (team_id,name,strategy_type,platform,config_json,schedule,timezone,status,created_by,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`, v.TeamID, v.Name, v.StrategyType, v.Platform, config, v.Schedule, v.Timezone, v.Status, v.CreatedBy, v.CreatedAt, v.UpdatedAt)
	if result.Error != nil {
		var mysqlError *mysql.MySQLError
		if errors.As(result.Error, &mysqlError) && mysqlError.Number == 1062 {
			return model.DiscoveryStrategy{}, errStrategyDuplicate
		}
		return model.DiscoveryStrategy{}, result.Error
	}
	if err := db.Raw("SELECT LAST_INSERT_ID()").Row().Scan(&v.ID); err != nil {
		return model.DiscoveryStrategy{}, err
	}
	return v, nil
}
func ListStrategies(team *identitymodel.TeamID) ([]model.DiscoveryStrategy, error) {
	return listStrategies(database.DB(), team)
}
func listStrategies(db *gorm.DB, team *identitymodel.TeamID) ([]model.DiscoveryStrategy, error) {
	query := `SELECT id,team_id,name,strategy_type,platform,config_json,schedule,timezone,status,created_by,created_at,updated_at FROM discovery_strategies`
	args := []any{}
	if team != nil {
		query += " WHERE team_id = ?"
		args = append(args, *team)
	}
	query += " ORDER BY updated_at DESC,id DESC"
	rows, err := db.Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.DiscoveryStrategy{}
	for rows.Next() {
		item, err := scanStrategy(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func FindStrategy(id int64) (model.DiscoveryStrategy, bool, error) {
	return findStrategy(database.DB(), id)
}
func findStrategy(db *gorm.DB, id int64) (model.DiscoveryStrategy, bool, error) {
	item, err := scanStrategy(db.Raw(`SELECT id,team_id,name,strategy_type,platform,config_json,schedule,timezone,status,created_by,created_at,updated_at FROM discovery_strategies WHERE id = ?`, id).Row())
	if errors.Is(err, sql.ErrNoRows) {
		return model.DiscoveryStrategy{}, false, nil
	}
	return item, err == nil, err
}
func UpdateStrategy(v model.DiscoveryStrategy) (model.DiscoveryStrategy, error) {
	return updateStrategy(database.DB(), v)
}
func updateStrategy(db *gorm.DB, v model.DiscoveryStrategy) (model.DiscoveryStrategy, error) {
	config, _ := json.Marshal(v.Config)
	result := db.Exec(`UPDATE discovery_strategies SET name=?,strategy_type=?,platform=?,config_json=?,schedule=?,timezone=?,status=?,updated_at=? WHERE id=?`, v.Name, v.StrategyType, v.Platform, config, v.Schedule, v.Timezone, v.Status, v.UpdatedAt, v.ID)
	if result.Error != nil {
		return model.DiscoveryStrategy{}, result.Error
	}
	if result.RowsAffected == 0 {
		return model.DiscoveryStrategy{}, errStrategyNotFound
	}
	return v, nil
}
func CreateCrawlTask(v model.CrawlTask) (model.CrawlTask, error) {
	return createCrawlTask(database.DB(), v)
}
func createCrawlTask(db *gorm.DB, v model.CrawlTask) (model.CrawlTask, error) {
	snapshot, _ := json.Marshal(v.Snapshot)
	stats, _ := json.Marshal(v.Stats)
	result := db.Exec(`INSERT INTO crawl_tasks (team_id,strategy_id,schedule_key,task_type,platform,status,snapshot_json,stats_json,created_by,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`, v.TeamID, v.StrategyID, nullString(v.ScheduleKey), v.TaskType, v.Platform, v.Status, snapshot, stats, v.CreatedBy, v.CreatedAt, v.UpdatedAt)
	if result.Error != nil {
		return model.CrawlTask{}, result.Error
	}
	if err := db.Raw("SELECT LAST_INSERT_ID()").Row().Scan(&v.ID); err != nil {
		return model.CrawlTask{}, err
	}
	return v, nil
}
func ListCrawlTasks(team *identitymodel.TeamID, strategyID *int64) ([]model.CrawlTask, error) {
	return listCrawlTasks(database.DB(), team, strategyID)
}
func listCrawlTasks(db *gorm.DB, team *identitymodel.TeamID, strategyID *int64) ([]model.CrawlTask, error) {
	query := `SELECT id,team_id,strategy_id,schedule_key,task_id,task_type,platform,status,snapshot_json,stats_json,result_json,error_message,started_at,finished_at,created_by,created_at,updated_at FROM crawl_tasks`
	conditions := []string{}
	args := []any{}
	if team != nil {
		conditions = append(conditions, "team_id = ?")
		args = append(args, *team)
	}
	if strategyID != nil {
		conditions = append(conditions, "strategy_id = ?")
		args = append(args, *strategyID)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at DESC,id DESC LIMIT 500"
	rows, err := db.Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.CrawlTask{}
	for rows.Next() {
		item, err := scanCrawlTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func FindCrawlTask(id int64) (model.CrawlTask, bool, error) { return findCrawlTask(database.DB(), id) }
func findCrawlTask(db *gorm.DB, id int64) (model.CrawlTask, bool, error) {
	item, err := scanCrawlTask(db.Raw(`SELECT id,team_id,strategy_id,schedule_key,task_id,task_type,platform,status,snapshot_json,stats_json,result_json,error_message,started_at,finished_at,created_by,created_at,updated_at FROM crawl_tasks WHERE id = ?`, id).Row())
	if errors.Is(err, sql.ErrNoRows) {
		return model.CrawlTask{}, false, nil
	}
	return item, err == nil, err
}
func UpdateCrawlTask(v model.CrawlTask) (model.CrawlTask, error) {
	return updateCrawlTask(database.DB(), v)
}
func updateCrawlTask(db *gorm.DB, v model.CrawlTask) (model.CrawlTask, error) {
	snapshot, _ := json.Marshal(v.Snapshot)
	stats, _ := json.Marshal(v.Stats)
	results, _ := json.Marshal(v.Results)
	result := db.Exec(`UPDATE crawl_tasks SET task_id=?,status=?,snapshot_json=?,stats_json=?,result_json=?,error_message=?,started_at=?,finished_at=?,updated_at=? WHERE id=?`, nullString(v.TaskID), v.Status, snapshot, stats, results, nullString(v.Error), v.StartedAt, v.FinishedAt, v.UpdatedAt, v.ID)
	if result.Error != nil {
		return model.CrawlTask{}, result.Error
	}
	if result.RowsAffected == 0 {
		return model.CrawlTask{}, errCrawlTaskNotFound
	}
	return v, nil
}
func ClaimPendingCrawlTask(now time.Time) (model.CrawlTask, bool, error) {
	return claimPendingCrawlTask(database.DB(), now)
}
func claimPendingCrawlTask(db *gorm.DB, now time.Time) (model.CrawlTask, bool, error) {
	var task model.CrawlTask
	found := false
	err := db.Transaction(func(tx *gorm.DB) error {
		item, err := scanCrawlTask(tx.Raw(`SELECT id,team_id,strategy_id,schedule_key,task_id,task_type,platform,status,snapshot_json,stats_json,result_json,error_message,started_at,finished_at,created_by,created_at,updated_at FROM crawl_tasks WHERE status = 'pending' ORDER BY created_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`).Row())
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		item.Status = model.CrawlRunning
		item.TaskID = fmt.Sprintf("crawl-%d", item.ID)
		item.StartedAt = &now
		item.UpdatedAt = now
		result := tx.Exec(`UPDATE crawl_tasks SET task_id=?,status='running',started_at=?,updated_at=? WHERE id=? AND status='pending'`, item.TaskID, now, now, item.ID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("crawl task claim lost")
		}
		task, found = item, true
		return nil
	})
	return task, found, err
}

func scanStrategy(row scannable) (model.DiscoveryStrategy, error) {
	var item model.DiscoveryStrategy
	var team, creator int64
	var status string
	var raw []byte
	err := row.Scan(&item.ID, &team, &item.Name, &item.StrategyType, &item.Platform, &raw, &item.Schedule, &item.Timezone, &status, &creator, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return item, err
	}
	item.TeamID = identitymodel.TeamID(team)
	item.CreatedBy = identitymodel.UserID(creator)
	item.Status = model.StrategyStatus(status)
	_ = json.Unmarshal(raw, &item.Config)
	if item.Config == nil {
		item.Config = map[string]any{}
	}
	return item, nil
}
func scanCrawlTask(row scannable) (model.CrawlTask, error) {
	var item model.CrawlTask
	var team, creator int64
	var strategyID sql.NullInt64
	var scheduleKey, taskID, errorMessage sql.NullString
	var status string
	var snapshot, stats, results []byte
	var started, finished sql.NullTime
	err := row.Scan(&item.ID, &team, &strategyID, &scheduleKey, &taskID, &item.TaskType, &item.Platform, &status, &snapshot, &stats, &results, &errorMessage, &started, &finished, &creator, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return item, err
	}
	item.TeamID = identitymodel.TeamID(team)
	item.CreatedBy = identitymodel.UserID(creator)
	item.Status = model.CrawlStatus(status)
	if strategyID.Valid {
		value := strategyID.Int64
		item.StrategyID = &value
	}
	item.ScheduleKey, item.TaskID, item.Error = scheduleKey.String, taskID.String, errorMessage.String
	_ = json.Unmarshal(snapshot, &item.Snapshot)
	_ = json.Unmarshal(stats, &item.Stats)
	_ = json.Unmarshal(results, &item.Results)
	if started.Valid {
		item.StartedAt = &started.Time
	}
	if finished.Valid {
		item.FinishedAt = &finished.Time
	}
	return item, nil
}
func nullString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

var errStrategyDuplicate = errors.New("discovery strategy already exists")
var errStrategyNotFound = errors.New("discovery strategy not found")
var errCrawlTaskNotFound = errors.New("crawl task not found")
