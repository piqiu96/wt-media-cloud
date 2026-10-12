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
	item := model.ComposeStrategy{TeamID: 7, GameID: "delta", TemplateID: 3, TemplateVersion: 1, Name: "剪辑", ContentType: "video", Config: json.RawMessage(`{"start_seconds":5,"end_seconds":12}`), Status: "active", CreatedBy: 9, UpdatedBy: 9}
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `compose_strategies`").WithArgs(int64(7), "delta", int64(3), uint32(1), "剪辑", "video", item.Config, "active", int64(9), int64(9), now, now).WillReturnResult(sqlmock.NewResult(21, 1))
	mock.ExpectExec("INSERT INTO audit_logs").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT .* FROM compose_strategies WHERE id = \\?").WithArgs(int64(21)).WillReturnRows(sqlmock.NewRows([]string{"id", "team_id", "game_id", "compose_template_id", "compose_template_version", "name", "content_type", "config_json", "status", "created_by", "updated_by", "created_at", "updated_at"}).AddRow(21, 7, "delta", 3, 1, "剪辑", "video", []byte(item.Config), "active", 9, 9, now, now))
	created, err := createComposeStrategyAudited(db, item, "compose_strategy.create", now)
	if err != nil || created.ID != 21 || created.TeamID != 7 || created.TemplateID != 3 || string(created.Config) != string(item.Config) {
		t.Fatalf("created=%+v err=%v", created, err)
	}
	assertExpectations(t, mock)
}

func TestCreateComposeStrategyRollsBackWhenAuditFails(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC)
	item := model.ComposeStrategy{TeamID: 7, GameID: "delta", TemplateID: 3, TemplateVersion: 1, Name: "剪辑", ContentType: "video", Config: json.RawMessage(`{"start_seconds":5,"end_seconds":12}`), Status: "active", CreatedBy: 9, UpdatedBy: 9}
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
	mock.ExpectQuery("SELECT .* FROM compose_strategies s JOIN compose_templates t").WithArgs(int64(7), "delta", "active").WillReturnRows(sqlmock.NewRows([]string{"id", "team_id", "game_id", "compose_template_id", "compose_template_version", "name", "content_type", "config_json", "status", "created_by", "updated_by", "created_at", "updated_at"}))
	items, err := listComposeStrategies(db, StrategyFilter{TeamID: 7, GameIDs: []string{"delta"}, ActiveOnly: true})
	if err != nil || len(items) != 0 {
		t.Fatalf("items=%v err=%v", items, err)
	}
	assertExpectations(t, mock)
}
