package repository

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
)

func TestCreateAndReadComposeStrategy(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC)
	item := model.ComposeStrategy{TeamID: 7, GameID: "delta", TemplateID: 3, TemplateVersion: 1, Name: "剪辑", Config: json.RawMessage(`{"start_seconds":5,"end_seconds":12}`), ExecutionMode: model.ComposeExecutionManual, RulesJSON: json.RawMessage(`[]`), SingleLimit: 1, DailyLimit: 2, Status: "enabled", CreatedBy: 9, UpdatedBy: 9}
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `compose_strategies`").WithArgs(int64(7), "delta", nil, int64(3), uint32(1), "剪辑", item.Config, item.ExecutionMode, item.RulesJSON, item.SingleLimit, item.DailyLimit, nil, item.Status, int64(9), int64(9), now, now).WillReturnResult(sqlmock.NewResult(21, 1))
	mock.ExpectExec("INSERT INTO audit_logs").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT .* FROM compose_strategies WHERE id = \\?").WithArgs(int64(21)).WillReturnRows(sqlmock.NewRows([]string{"id", "team_id", "game_id", "operator_id", "compose_template_id", "compose_template_version", "name", "config_json", "execution_mode", "schedule_json", "priority_rules_json", "single_limit", "daily_limit", "next_run_at", "status", "created_by", "updated_by", "created_at", "updated_at"}).AddRow(21, 7, "delta", nil, 3, 1, "剪辑", []byte(item.Config), "manual", nil, []byte(`[]`), 1, 2, nil, "enabled", 9, 9, now, now))
	created, err := createComposeStrategyAudited(db, item, "compose_strategy.create", now)
	if err != nil || created.ID != 21 || created.TeamID != 7 || created.TemplateID != 3 || string(created.Config) != string(item.Config) {
		t.Fatalf("created=%+v err=%v", created, err)
	}
	assertExpectations(t, mock)
}

func TestCreateComposeStrategyRollsBackWhenAuditFails(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC)
	item := model.ComposeStrategy{TeamID: 7, GameID: "delta", TemplateID: 3, TemplateVersion: 1, Name: "剪辑", Config: json.RawMessage(`{"start_seconds":5,"end_seconds":12}`), ExecutionMode: model.ComposeExecutionManual, RulesJSON: json.RawMessage(`[]`), SingleLimit: 1, DailyLimit: 1, Status: "enabled", CreatedBy: 9, UpdatedBy: 9}
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `compose_strategies`").WillReturnResult(sqlmock.NewResult(21, 1))
	mock.ExpectExec("INSERT INTO audit_logs").WillReturnError(errors.New("audit unavailable"))
	mock.ExpectRollback()
	if _, err := createComposeStrategyAudited(db, item, "compose_strategy.create", now); err == nil {
		t.Fatal("expected audit failure")
	}
	assertExpectations(t, mock)
}

func TestListComposeStrategiesAppliesScopeInSQL(t *testing.T) {
	db, mock := newMockGORM(t)
	mock.ExpectQuery("SELECT .* FROM compose_strategies s JOIN compose_templates t").WithArgs(int64(7), "delta", "enabled").WillReturnRows(sqlmock.NewRows([]string{"id", "team_id", "game_id", "operator_id", "compose_template_id", "compose_template_version", "name", "config_json", "execution_mode", "schedule_json", "priority_rules_json", "single_limit", "daily_limit", "next_run_at", "status", "created_by", "updated_by", "created_at", "updated_at"}))
	items, err := listComposeStrategies(db, StrategyFilter{TeamID: 7, GameIDs: []string{"delta"}, ActiveOnly: true})
	if err != nil || len(items) != 0 {
		t.Fatalf("items=%v err=%v", items, err)
	}
	assertExpectations(t, mock)
}

func TestListComposeStrategiesAppliesSearchAndFiltersInSQL(t *testing.T) {
	db, mock := newMockGORM(t)
	mock.ExpectQuery(`SELECT .* FROM compose_strategies s JOIN compose_templates t.*s\.name LIKE \? ESCAPE '!'.*s\.game_id = \?.*s\.compose_template_id = \?.*s\.status = \?.*LIMIT 200`).
		WithArgs("%剪辑!_100%", "剪辑_100", "delta", int64(3), "disabled").
		WillReturnRows(sqlmock.NewRows([]string{"id", "team_id", "game_id", "operator_id", "compose_template_id", "compose_template_version", "name", "config_json", "execution_mode", "schedule_json", "priority_rules_json", "single_limit", "daily_limit", "next_run_at", "status", "created_by", "updated_by", "created_at", "updated_at"}))
	items, err := listComposeStrategies(db, StrategyFilter{Search: "剪辑_100", GameID: "delta", TemplateID: 3, Status: "disabled"})
	if err != nil || len(items) != 0 {
		t.Fatalf("items=%v err=%v", items, err)
	}
	assertExpectations(t, mock)
}

func TestUpdateAndReadComposeStrategySchedule(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 14, 8, 0, 0, 0, time.UTC)
	next := now.Add(15 * time.Minute)
	schedule := model.ComposeSchedule{Mode: "interval", IntervalMinutes: 15, Timezone: "Asia/Shanghai"}
	scheduleJSON, _ := json.Marshal(schedule)
	item := model.ComposeStrategy{ID: 21, TeamID: 7, GameID: "delta", TemplateID: 3, TemplateVersion: 1, Name: "剪辑", Config: json.RawMessage(`{"start_seconds":5,"end_seconds":12}`), ExecutionMode: model.ComposeExecutionScheduled, Schedule: &schedule, RulesJSON: json.RawMessage(`[]`), SingleLimit: 1, DailyLimit: 2, NextRunAt: &next, Status: "enabled", CreatedBy: 9, UpdatedBy: 9}
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE compose_strategies SET name").WithArgs(item.Name, item.OperatorID, item.Config, item.ExecutionMode, scheduleJSON, item.RulesJSON, item.SingleLimit, item.DailyLimit, &next, item.Status, int64(9), now, int64(21), int64(7), "delta").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO audit_logs").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT .* FROM compose_strategies WHERE id = \\?").WithArgs(int64(21)).WillReturnRows(sqlmock.NewRows([]string{"id", "team_id", "game_id", "operator_id", "compose_template_id", "compose_template_version", "name", "config_json", "execution_mode", "schedule_json", "priority_rules_json", "single_limit", "daily_limit", "next_run_at", "status", "created_by", "updated_by", "created_at", "updated_at"}).AddRow(21, 7, "delta", nil, 3, 1, "剪辑", []byte(item.Config), "scheduled", scheduleJSON, []byte(`[]`), 1, 2, next, "enabled", 9, 9, now, now))
	stored, err := updateComposeStrategyAudited(db, item, "compose_strategy.schedule", now)
	if err != nil || stored.Schedule == nil || stored.ExecutionMode != model.ComposeExecutionScheduled || stored.NextRunAt == nil || !stored.NextRunAt.Equal(next) {
		t.Fatalf("stored=%+v err=%v", stored, err)
	}
	assertExpectations(t, mock)
}

func TestListComposeStrategiesNarrowsToTheAssignedOperator(t *testing.T) {
	db, mock := newMockGORM(t)
	mock.ExpectQuery(`s\.operator_id = \? AND s\.game_id IN`).
		WithArgs(int64(7), int64(9), "delta").
		WillReturnRows(sqlmock.NewRows([]string{"id", "team_id", "game_id", "operator_id", "compose_template_id", "compose_template_version", "name", "config_json", "execution_mode", "schedule_json", "priority_rules_json", "single_limit", "daily_limit", "next_run_at", "status", "created_by", "updated_by", "created_at", "updated_at"}))
	items, err := listComposeStrategies(db, StrategyFilter{TeamID: 7, GameIDs: []string{"delta"}, OperatorID: 9})
	if err != nil || len(items) != 0 {
		t.Fatalf("items=%v err=%v", items, err)
	}
	assertExpectations(t, mock)
}
