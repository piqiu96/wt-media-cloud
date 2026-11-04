package repository

import (
	"database/sql"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"strings"
	"testing"
	"time"
)

func TestManualBatchWithNoEligibleMaterialsDoesNotCreateEmptyBatch(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Now().UTC()
	c := ManualBatchCommand{ActorID: 9, StrategyID: 3, Name: "人工批次", RequestKey: "request-key", IntentHash: "hash", MaterialIDs: []int64{42}, Authorize: func(model.ComposeStrategy) error { return nil }, ResolveSettings: func(model.ComposeStrategy, model.ComposeTemplate) ([]model.ComposeOutputSetting, error) {
		return []model.ComposeOutputSetting{{Key: "main", Name: "主片", Copies: 2, Platforms: []string{"bilibili"}, Params: map[string]any{"start_seconds": 0, "end_seconds": 2}}}, nil
	}}
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM compose_strategies.*FOR UPDATE`).WithArgs(int64(3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
	mock.ExpectQuery(`SELECT id FROM compose_task_batches WHERE request_key`).WithArgs(c.RequestKey).WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT .* FROM compose_strategies WHERE id`).WithArgs(int64(3)).WillReturnRows(sqlmock.NewRows([]string{"id", "team_id", "category_id", "operator_id", "compose_template_id", "compose_template_version", "name", "config_json", "execution_mode", "schedule_json", "priority_rules_json", "single_limit", "daily_limit", "next_run_at", "status", "created_by", "updated_by", "created_at", "updated_at", "output_settings_json"}).AddRow(3, 7, 1, 9, 1, 1, "策略", []byte(`{"start_seconds":0,"end_seconds":2}`), "manual", nil, []byte(`[]`), 1, 0, nil, "enabled", 9, 9, now, now, nil))
	mock.ExpectQuery(`SELECT id, code, name, version, schema_json, executor, status FROM compose_templates`).WithArgs(int64(1), uint32(1)).WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name", "version", "schema_json", "executor", "status"}).AddRow(1, "basic_cut", "基础", 1, []byte(`{}`), "basic_cut_v1", "active"))
	mock.ExpectQuery(`SELECT u.id FROM users`).WithArgs(int64(9), int64(7), 1).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectQuery(`SELECT m.team_id`).WithArgs(int64(42)).WillReturnError(sql.ErrNoRows)
	mock.ExpectCommit()
	got, err := createManualBatch(db, c, now)
	if err != nil || got.Batch != nil || len(got.Created) != 0 || len(got.Failures) != 1 {
		t.Fatalf("result=%+v err=%v", got, err)
	}
	assertExpectations(t, mock)
}

func TestManualBatchIntentConflictDoesNotReadMutableEligibility(t *testing.T) {
	db, mock := newMockGORM(t)
	c := ManualBatchCommand{ActorID: 9, StrategyID: 3, Name: "批次", RequestKey: "request-key", IntentHash: "new-hash", MaterialIDs: []int64{42}, Authorize: func(model.ComposeStrategy) error { return nil }, ResolveSettings: func(model.ComposeStrategy, model.ComposeTemplate) ([]model.ComposeOutputSetting, error) {
		t.Fatal("replay must not resolve current config")
		return nil, nil
	}}
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM compose_strategies.*FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
	mock.ExpectQuery(`SELECT id FROM compose_task_batches WHERE request_key`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(5))
	mock.ExpectQuery(`SELECT id,name,team_id`).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "team_id", "category_id", "operator_id", "compose_strategy_id", "trigger_type", "created_by", "request_key", "request_hash", "config_snapshot_json", "created_at"}).AddRow(5, "批次", 7, 1, 9, 3, "manual", 9, c.RequestKey, "old-hash", []byte(`{"settings":[],"acceptance":[]}`), time.Now()))
	mock.ExpectQuery(`SELECT status,COUNT`).WillReturnRows(sqlmock.NewRows([]string{"status", "count"}))
	mock.ExpectRollback()
	_, err := createManualBatch(db, c, time.Now())
	if err != ErrManualBatchConflict {
		t.Fatalf("err=%v", err)
	}
	assertExpectations(t, mock)
}

func TestManualTaskNamePreservesSettingAndCopyWithinColumn(t *testing.T) {
	name := manualTaskName(strings.Repeat("批", 60), strings.Repeat("材", 255), strings.Repeat("组", 120), 20)
	if len([]rune(name)) > 255 || !strings.HasSuffix(name, " · 20") {
		t.Fatalf("name length=%d suffix=%s", len([]rune(name)), name)
	}
}

func TestManualBatchConcurrentCommandKeyCollisionIsConflict(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Now().UTC()
	c := ManualBatchCommand{ActorID: 9, StrategyID: 3, Name: "人工批次", RequestKey: "request-key", IntentHash: "hash", MaterialIDs: []int64{42}, Authorize: func(model.ComposeStrategy) error { return nil }, ResolveSettings: func(model.ComposeStrategy, model.ComposeTemplate) ([]model.ComposeOutputSetting, error) {
		return []model.ComposeOutputSetting{{Key: "main", Name: "主片", Copies: 2, Platforms: []string{"bilibili"}, Params: map[string]any{"start_seconds": 0, "end_seconds": 2}}}, nil
	}}
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM compose_strategies.*FOR UPDATE`).WithArgs(int64(3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
	mock.ExpectQuery(`SELECT id FROM compose_task_batches WHERE request_key`).WithArgs(c.RequestKey).WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT .* FROM compose_strategies WHERE id`).WithArgs(int64(3)).WillReturnRows(sqlmock.NewRows([]string{"id", "team_id", "category_id", "operator_id", "compose_template_id", "compose_template_version", "name", "config_json", "execution_mode", "schedule_json", "priority_rules_json", "single_limit", "daily_limit", "next_run_at", "status", "created_by", "updated_by", "created_at", "updated_at", "output_settings_json"}).AddRow(3, 7, 1, 9, 1, 1, "策略", []byte(`{"start_seconds":0,"end_seconds":2}`), "manual", nil, []byte(`[]`), 1, 0, nil, "enabled", 9, 9, now, now, nil))
	mock.ExpectQuery(`SELECT id, code, name, version, schema_json, executor, status FROM compose_templates`).WithArgs(int64(1), uint32(1)).WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name", "version", "schema_json", "executor", "status"}).AddRow(1, "basic_cut", "基础", 1, []byte(`{}`), "basic_cut_v1", "active"))
	mock.ExpectQuery(`SELECT u.id FROM users`).WithArgs(int64(9), int64(7), 1).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectQuery(`SELECT m.team_id`).WithArgs(int64(42)).WillReturnRows(sqlmock.NewRows([]string{"team", "category", "status", "title", "video", "object", "source"}).AddRow(7, 1, "available", "素材", "ready", "video-key", ""))
	mock.ExpectQuery(`SELECT id, team_id, material_id, user_id, status, removed_at`).WithArgs(int64(9), int64(42)).WillReturnRows(sqlmock.NewRows([]string{"id", "team_id", "material_id", "user_id", "status", "removed_at", "created_at", "updated_at"}).AddRow(11, 7, 42, 9, "active", nil, now, now))
	mock.ExpectQuery(`SELECT id FROM compose_tasks`).WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO `compose_task_batches`").WillReturnError(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry for uq_compose_batch_request"})
	mock.ExpectRollback()
	_, err := createManualBatch(db, c, now)
	if err != ErrManualBatchConflict {
		t.Fatalf("err=%v", err)
	}
	assertExpectations(t, mock)
}
