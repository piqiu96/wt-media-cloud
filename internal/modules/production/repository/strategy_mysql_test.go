package repository

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

func TestStrategyUpdateRollsBackWhenVersionWriteFails(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	row := model.ComposeStrategy{ID: 5, TeamID: 2, GameID: "game-a", Name: "基础策略", ContentType: "highlight", TemplateKey: "basic_cut", TemplateVersion: "1.0", Parameters: model.StrategyParameters{VideoRatio: "9:16"}, AllowedOverrides: []string{"video_ratio"}, Status: "active", CreatedBy: 7, UpdatedBy: 7, CreatedAt: now, UpdatedAt: now}
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE compose_strategies SET").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO compose_strategy_versions").WillReturnError(errors.New("version insert failed"))
	mock.ExpectRollback()
	_, changed, err := updateStrategy(db, row, 1)
	if err == nil || changed {
		t.Fatalf("update = changed %t, error %v; expected rollback", changed, err)
	}
	assertExpectations(t, mock)
}

func TestStrategyListScopesTeamGamesAndActiveStatusInSQL(t *testing.T) {
	db, mock := newMockGORM(t)
	team := identity.TeamID(2)
	mock.ExpectQuery(regexp.QuoteMeta("FROM compose_strategies WHERE 1 = 1 AND team_id = ? AND game_id IN (?,?) AND status = 'active' ORDER BY updated_at DESC, id DESC")).
		WithArgs(team, "game-a", "game-b").
		WillReturnRows(sqlmock.NewRows([]string{"id", "team_id", "game_id", "name", "content_type", "template_key", "template_version", "parameters_json", "allowed_overrides_json", "status", "current_version", "created_by", "updated_by", "created_at", "updated_at"}))
	rows, err := listStrategies(db, StrategyFilter{TeamID: &team, GameIDs: []string{"game-a", "game-b"}})
	if err != nil || len(rows) != 0 {
		t.Fatalf("scoped list = %+v, %v", rows, err)
	}
	assertExpectations(t, mock)
}

func TestStrategyUpdateRejectsStaleVersionWithoutAppendingHistory(t *testing.T) {
	db, mock := newMockGORM(t)
	row := model.ComposeStrategy{ID: 5, TeamID: 2, GameID: "game-a", TemplateKey: "basic_cut", TemplateVersion: "1.0", Parameters: model.StrategyParameters{VideoRatio: "16:9"}, AllowedOverrides: []string{}, CurrentVersion: 1}
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE compose_strategies SET").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	_, changed, err := updateStrategy(db, row, 1)
	if err != nil || changed {
		t.Fatalf("stale update = changed %t, error %v", changed, err)
	}
	assertExpectations(t, mock)
}
