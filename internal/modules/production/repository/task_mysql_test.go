package repository

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"database/sql/driver"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
)

var composeTaskRowColumns = []string{"id", "team_id", "game_id", "trigger_type", "material_id", "material_usage_id", "compose_strategy_id", "template_code", "template_version", "config_snapshot_json", "status", "request_key", "created_by", "claimed_node_id", "lease_expires_at", "heartbeat_at", "attempt_count", "max_attempts", "started_at", "finished_at", "stage", "progress", "error_code", "error_message", "output_object_key", "composite_output_id", "created_at", "updated_at"}

func composeTaskRow(id int64, status string, now time.Time) []driverValue {
	return []driverValue{int64(id), int64(7), "delta", "manual", int64(42), int64(9), int64(3), "basic_cut", uint32(1), []byte(`{"start_seconds":0,"end_seconds":1}`), status, "rk-123", int64(11), nil, nil, nil, 0, 3, nil, nil, "", 0, "", "", "", nil, now, now}
}

// driverValue keeps the row literals readable; sqlmock accepts any.
type driverValue = driver.Value

func TestCreateComposeTaskPersistsAndReadsBack(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	task := model.ComposeTask{TeamID: 7, GameID: "delta", TriggerType: "manual", MaterialID: 42, MaterialUsageID: 9, StrategyID: 3, TemplateCode: "basic_cut", TemplateVersion: 1, ConfigSnapshot: json.RawMessage(`{"start_seconds":0,"end_seconds":1}`), Status: model.ComposeTaskPending, RequestKey: "rk-123", CreatedBy: 11}
	mock.ExpectBegin()
	// GORM writes every column, including the nil claim/lease/heartbeat and
	// empty execution projection — the full 27-arg list pins that shape.
	mock.ExpectExec("INSERT INTO `compose_tasks`").WithArgs(int64(7), "delta", "manual", int64(42), int64(9), int64(3), "basic_cut", uint32(1), task.ConfigSnapshot, model.ComposeTaskPending, "rk-123", int64(11), nil, nil, nil, 0, 3, nil, nil, "", 0, "", "", "", nil, now, now).WillReturnResult(sqlmock.NewResult(77, 1))
	mock.ExpectExec("INSERT INTO audit_logs").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT .* FROM compose_tasks WHERE id = \\?").WithArgs(int64(77)).WillReturnRows(sqlmock.NewRows(composeTaskRowColumns).AddRow(composeTaskRow(77, "pending", now)...))
	created, err := createComposeTaskAudited(db, task, "compose_task.create", now)
	if err != nil || created.ID != 77 || created.Status != model.ComposeTaskPending || created.TemplateCode != "basic_cut" {
		t.Fatalf("created=%+v err=%v", created, err)
	}
	if string(created.ConfigSnapshot) != `{"start_seconds":0,"end_seconds":1}` {
		t.Fatalf("snapshot drifted: %s", created.ConfigSnapshot)
	}
	assertExpectations(t, mock)
}

func TestCreateComposeTaskRollsBackWhenAuditFails(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	task := model.ComposeTask{TeamID: 7, GameID: "delta", TriggerType: "manual", MaterialID: 42, MaterialUsageID: 9, StrategyID: 3, TemplateCode: "basic_cut", TemplateVersion: 1, ConfigSnapshot: json.RawMessage(`{}`), Status: model.ComposeTaskPending, RequestKey: "rk-1", CreatedBy: 11}
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `compose_tasks`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO audit_logs").WillReturnError(errors.New("audit unavailable"))
	mock.ExpectRollback()
	if _, err := createComposeTaskAudited(db, task, "compose_task.create", now); err == nil {
		t.Fatal("expected audit failure")
	}
	assertExpectations(t, mock)
}

// TestCancelComposeTaskAuditedWritesAuditOnlyWhenPending (below) supersedes
// the plain conditional-UPDATE test from T-02: the audited cancel is the only
// cancel path now, and its no-write-on-non-pending case is covered there.

func TestListComposeTasksAppliesScopeInSQL(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT .* FROM compose_tasks").WithArgs(int64(7), "delta", "pending", int64(3)).
		WillReturnRows(sqlmock.NewRows(composeTaskRowColumns).AddRow(composeTaskRow(77, "pending", now)...))
	items, err := listComposeTasks(db, ComposeTaskFilter{TeamID: 7, GameIDs: []string{"delta"}, Status: model.ComposeTaskPending, StrategyID: 3})
	if err != nil || len(items) != 1 || items[0].MaterialID != 42 {
		t.Fatalf("items=%v err=%v", items, err)
	}
	assertExpectations(t, mock)
}

func TestFindActiveUsageForTaskCreation(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT .* FROM material_usages WHERE user_id = \\? AND material_id = \\? AND status = 'active'").
		WithArgs(int64(11), int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "team_id", "material_id", "user_id", "status", "removed_at", "created_at", "updated_at"}).AddRow(int64(9), int64(7), int64(42), int64(11), "active", nil, now, now))
	usage, found, err := findActiveUsage(db, 11, 42)
	if err != nil || !found || usage.ID != 9 {
		t.Fatalf("usage=%+v found=%v err=%v", usage, found, err)
	}
	assertExpectations(t, mock)
}

func TestCreateForUsageChecksUsageInsideTransaction(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC)
	item := model.ComposeTask{TeamID: 7, GameID: "delta", TriggerType: "manual", MaterialID: 42, StrategyID: 3, TemplateCode: "basic_cut", TemplateVersion: 1, ConfigSnapshot: json.RawMessage(`{"start_seconds":0}`), Status: model.ComposeTaskPending, RequestKey: "rk-usage-1", CreatedBy: 11}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM material_usages WHERE user_id = \\? AND material_id = \\? AND status = 'active'").
		WithArgs(int64(11), int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "team_id", "material_id", "user_id", "status", "removed_at", "created_at", "updated_at"}).AddRow(int64(9), int64(7), int64(42), int64(11), "active", nil, now, now))
	mock.ExpectExec("INSERT INTO `compose_tasks`").WithArgs(int64(7), "delta", "manual", int64(42), int64(9), int64(3), "basic_cut", uint32(1), item.ConfigSnapshot, model.ComposeTaskPending, "rk-usage-1", int64(11), nil, nil, nil, 0, 3, nil, nil, "", 0, "", "", "", nil, now, now).WillReturnResult(sqlmock.NewResult(77, 1))
	mock.ExpectExec("INSERT INTO audit_logs").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT .* FROM compose_tasks WHERE id = \\?").WithArgs(int64(77)).WillReturnRows(sqlmock.NewRows(composeTaskRowColumns).AddRow(composeTaskRow(77, "pending", now)...))
	created, found, err := createForUsage(db, item, "compose_task.create", now)
	if err != nil || !found || created.ID != 77 || created.MaterialUsageID != 9 {
		t.Fatalf("created=%+v found=%v err=%v", created, found, err)
	}
	assertExpectations(t, mock)
}

func TestCreateForUsageWritesNothingWithoutActiveUsage(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC)
	item := model.ComposeTask{TeamID: 7, GameID: "delta", MaterialID: 42, Status: model.ComposeTaskPending, RequestKey: "rk-usage-2", CreatedBy: 11}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM material_usages WHERE user_id = \\? AND material_id = \\? AND status = 'active'").
		WithArgs(int64(11), int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "team_id", "material_id", "user_id", "status", "removed_at", "created_at", "updated_at"}))
	mock.ExpectCommit()
	created, found, err := createForUsage(db, item, "compose_task.create", now)
	if err != nil || found || created.ID != 0 {
		t.Fatalf("created=%+v found=%v err=%v", created, found, err)
	}
	assertExpectations(t, mock)
}

func TestCancelComposeTaskAuditedWritesAuditOnlyWhenPending(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 13, 10, 5, 0, 0, time.UTC)
	item := model.ComposeTask{ID: 77, TeamID: 7, GameID: "delta", Status: model.ComposeTaskPending, CreatedBy: 11}
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE compose_tasks SET status = 'cancelled'").WithArgs(now, now, int64(77)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO audit_logs").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	ok, err := cancelComposeTaskAudited(db, item, 9, "compose_task.cancel", now)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE compose_tasks SET status = 'cancelled'").WithArgs(now, now, int64(78)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	ok, err = cancelComposeTaskAudited(db, model.ComposeTask{ID: 78}, 9, "compose_task.cancel", now)
	if err != nil || ok {
		t.Fatalf("non-pending cancel must write nothing: ok=%v err=%v", ok, err)
	}
	assertExpectations(t, mock)
}

func TestFindComposeTaskByKeyReadsTheUniqueColumn(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT .* FROM compose_tasks WHERE request_key = \\?").WithArgs("rk-123").
		WillReturnRows(sqlmock.NewRows(composeTaskRowColumns).AddRow(composeTaskRow(77, "pending", now)...))
	item, found, err := findComposeTaskByKey(db, "rk-123")
	if err != nil || !found || item.ID != 77 {
		t.Fatalf("item=%+v found=%v err=%v", item, found, err)
	}
	assertExpectations(t, mock)
}
