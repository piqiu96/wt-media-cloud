package repository

import (
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
)

func TestStrategyPageCountFailureCannotReturnAFalseTotal(t *testing.T) {
	db, mock := newMockGORM(t)
	failure := errors.New("count query unavailable")
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM compose_strategies s JOIN compose_templates t`).WithArgs(int64(7)).WillReturnError(failure)
	_, err := pageComposeStrategies(db, StrategyFilter{TeamID: 7}, 1, 20)
	if !errors.Is(err, failure) {
		t.Fatalf("err=%v", err)
	}
	assertExpectations(t, mock)
}
func TestStrategyPageOffsetAndTotalUseSameScopedConditions(t *testing.T) {
	db, mock := newMockGORM(t)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM compose_strategies s JOIN compose_templates t.*s\.team_id = \? AND s\.operator_id = \?`).WithArgs(int64(7), int64(9)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(205))
	mock.ExpectQuery(`SELECT .* FROM compose_strategies s JOIN compose_templates t.*s\.team_id = \? AND s\.operator_id = \?.*ORDER BY s\.id DESC LIMIT \? OFFSET \?`).WithArgs(int64(7), int64(9), 100, 200).WillReturnRows(sqlmock.NewRows([]string{"id", "team_id", "category_id", "operator_id", "compose_template_id", "compose_template_version", "name", "config_json", "execution_mode", "schedule_json", "priority_rules_json", "single_limit", "daily_limit", "next_run_at", "status", "created_by", "updated_by", "created_at", "updated_at", "output_settings_json"}))
	page, err := pageComposeStrategies(db, StrategyFilter{TeamID: 7, OperatorID: 9}, 3, 100)
	if err != nil || page.Total != 205 || page.Page != 3 || page.PageSize != 100 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	assertExpectations(t, mock)
}
