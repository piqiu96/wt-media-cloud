package contentpool

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
)

type MySQLDiscoveryStore struct{ db *sql.DB }

func NewMySQLDiscoveryStore(db *sql.DB) *MySQLDiscoveryStore { return &MySQLDiscoveryStore{db: db} }

func (s *MySQLDiscoveryStore) CreateStrategy(v DiscoveryStrategy) (DiscoveryStrategy, error) {
	config, _ := json.Marshal(v.Config)
	res, err := s.db.Exec(`INSERT INTO discovery_strategies (team_id,name,strategy_type,platform,config_json,schedule,timezone,status,created_by,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`, v.TeamID, v.Name, v.StrategyType, v.Platform, config, v.Schedule, v.Timezone, v.Status, v.CreatedBy, v.CreatedAt, v.UpdatedAt)
	if err != nil {
		var me *mysql.MySQLError
		if errors.As(err, &me) && me.Number == 1062 {
			return DiscoveryStrategy{}, ErrStrategyDuplicate
		}
		return DiscoveryStrategy{}, err
	}
	v.ID, _ = res.LastInsertId()
	return v, nil
}

func (s *MySQLDiscoveryStore) ListStrategies(team *identity.TeamID) ([]DiscoveryStrategy, error) {
	query := `SELECT id,team_id,name,strategy_type,platform,config_json,schedule,timezone,status,created_by,created_at,updated_at FROM discovery_strategies`
	var args []any
	if team != nil {
		query += ` WHERE team_id = ?`
		args = append(args, *team)
	}
	query += ` ORDER BY updated_at DESC,id DESC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DiscoveryStrategy{}
	for rows.Next() {
		v, err := scanStrategy(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *MySQLDiscoveryStore) FindStrategy(id int64) (DiscoveryStrategy, bool, error) {
	v, err := scanStrategy(s.db.QueryRow(`SELECT id,team_id,name,strategy_type,platform,config_json,schedule,timezone,status,created_by,created_at,updated_at FROM discovery_strategies WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return DiscoveryStrategy{}, false, nil
	}
	return v, err == nil, err
}

func (s *MySQLDiscoveryStore) UpdateStrategy(v DiscoveryStrategy) (DiscoveryStrategy, error) {
	config, _ := json.Marshal(v.Config)
	_, err := s.db.Exec(`UPDATE discovery_strategies SET name=?,strategy_type=?,platform=?,config_json=?,schedule=?,timezone=?,status=?,updated_at=? WHERE id=?`, v.Name, v.StrategyType, v.Platform, config, v.Schedule, v.Timezone, v.Status, v.UpdatedAt, v.ID)
	if err != nil {
		return DiscoveryStrategy{}, err
	}
	return v, nil
}

func scanStrategy(row interface{ Scan(...any) error }) (DiscoveryStrategy, error) {
	var v DiscoveryStrategy
	var team, createdBy int64
	var raw []byte
	var status string
	err := row.Scan(&v.ID, &team, &v.Name, &v.StrategyType, &v.Platform, &raw, &v.Schedule, &v.Timezone, &status, &createdBy, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return v, err
	}
	v.TeamID = identity.TeamID(team)
	v.CreatedBy = identity.UserID(createdBy)
	v.Status = StrategyStatus(status)
	_ = json.Unmarshal(raw, &v.Config)
	if v.Config == nil {
		v.Config = map[string]any{}
	}
	return v, nil
}

func (s *MySQLDiscoveryStore) CreateCrawlTask(v CrawlTask) (CrawlTask, error) {
	snapshot, _ := json.Marshal(v.Snapshot)
	stats, _ := json.Marshal(v.Stats)
	res, err := s.db.Exec(`INSERT INTO crawl_tasks (team_id,strategy_id,task_type,platform,status,snapshot_json,stats_json,created_by,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)`, v.TeamID, v.StrategyID, v.TaskType, v.Platform, v.Status, snapshot, stats, v.CreatedBy, v.CreatedAt, v.UpdatedAt)
	if err != nil {
		return CrawlTask{}, err
	}
	v.ID, _ = res.LastInsertId()
	return v, nil
}

func (s *MySQLDiscoveryStore) ListCrawlTasks(team *identity.TeamID, strategyID *int64) ([]CrawlTask, error) {
	query := `SELECT id,team_id,strategy_id,task_id,task_type,platform,status,snapshot_json,stats_json,result_json,error_message,started_at,finished_at,created_by,created_at,updated_at FROM crawl_tasks`
	var cond []string
	var args []any
	if team != nil {
		cond = append(cond, "team_id = ?")
		args = append(args, *team)
	}
	if strategyID != nil {
		cond = append(cond, "strategy_id = ?")
		args = append(args, *strategyID)
	}
	if len(cond) > 0 {
		query += " WHERE " + strings.Join(cond, " AND ")
	}
	query += " ORDER BY created_at DESC,id DESC LIMIT 500"
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CrawlTask{}
	for rows.Next() {
		v, err := scanCrawlTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *MySQLDiscoveryStore) FindCrawlTask(id int64) (CrawlTask, bool, error) {
	v, err := scanCrawlTask(s.db.QueryRow(`SELECT id,team_id,strategy_id,task_id,task_type,platform,status,snapshot_json,stats_json,result_json,error_message,started_at,finished_at,created_by,created_at,updated_at FROM crawl_tasks WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return CrawlTask{}, false, nil
	}
	return v, err == nil, err
}

func (s *MySQLDiscoveryStore) UpdateCrawlTask(v CrawlTask) (CrawlTask, error) {
	snapshot, _ := json.Marshal(v.Snapshot)
	stats, _ := json.Marshal(v.Stats)
	results, _ := json.Marshal(v.Results)
	_, err := s.db.Exec(`UPDATE crawl_tasks SET task_id=?,status=?,snapshot_json=?,stats_json=?,result_json=?,error_message=?,started_at=?,finished_at=?,updated_at=? WHERE id=?`, nullString(v.TaskID), v.Status, snapshot, stats, results, nullString(v.Error), v.StartedAt, v.FinishedAt, v.UpdatedAt, v.ID)
	if err != nil {
		return CrawlTask{}, err
	}
	return v, nil
}

func scanCrawlTask(row interface{ Scan(...any) error }) (CrawlTask, error) {
	var v CrawlTask
	var team, createdBy int64
	var strategyID sql.NullInt64
	var taskID, errorMsg sql.NullString
	var status string
	var snapshot, stats, results []byte
	var started, finished sql.NullTime
	err := row.Scan(&v.ID, &team, &strategyID, &taskID, &v.TaskType, &v.Platform, &status, &snapshot, &stats, &results, &errorMsg, &started, &finished, &createdBy, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return v, err
	}
	v.TeamID = identity.TeamID(team)
	v.CreatedBy = identity.UserID(createdBy)
	v.Status = CrawlStatus(status)
	if strategyID.Valid {
		n := strategyID.Int64
		v.StrategyID = &n
	}
	v.TaskID = taskID.String
	v.Error = errorMsg.String
	_ = json.Unmarshal(snapshot, &v.Snapshot)
	_ = json.Unmarshal(stats, &v.Stats)
	_ = json.Unmarshal(results, &v.Results)
	if started.Valid {
		v.StartedAt = &started.Time
	}
	if finished.Valid {
		v.FinishedAt = &finished.Time
	}
	return v, nil
}

func nullString(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}
