package repository

import (
	"database/sql/driver"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

var outputRowColumns = []string{"id", "team_id", "game_id", "operator_id", "material_id", "material_usage_id", "compose_task_id", "compose_strategy_id", "template_code", "template_version", "object_key", "file_size_bytes", "sha256", "media_json", "publish_status", "created_by", "created_at", "updated_at"}

func TestListComposeOutputsSearchesFactsAndTriggerInSQL(t *testing.T) {
	from := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	before := from.Add(24 * time.Hour)
	db, mock := newMockGORM(t)
	mock.ExpectQuery(`FROM composite_outputs.*CAST\(composite_outputs\.id AS CHAR\) LIKE \? ESCAPE '!'.*m\.title LIKE \? ESCAPE '!'.*s\.name LIKE \? ESCAPE '!'.*trigger_type = \?.*created_at >= \? AND created_at < \?`).
		WithArgs(int64(7), "delta", "%58!_1%", "%58!_1%", "%58!_1%", "%58!_1%", "manual", from, before).
		WillReturnRows(sqlmock.NewRows(outputRowColumns))
	items, err := listComposeOutputs(db, ComposeOutputFilter{TeamID: 7, GameIDs: []string{"delta"}, Search: "58_1", TriggerType: "manual", CreatedFrom: from, CreatedBefore: before})
	if err != nil || len(items) != 0 {
		t.Fatalf("items=%v err=%v", items, err)
	}
	assertExpectations(t, mock)
}

func TestListComposeOutputsScopesSourceAndDateInSQL(t *testing.T) {
	from := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	before := from.Add(24 * time.Hour)
	for _, item := range []struct {
		name, source, pattern string
		args                  []driver.Value
	}{
		{"material id", "42", "material_id = .*m.title LIKE", []driver.Value{int64(7), "delta", int64(42), "%42%", from, before}},
		{"material title", "海光_片", "m.title LIKE", []driver.Value{int64(7), "delta", "%海光!_片%", from, before}},
	} {
		t.Run(item.name, func(t *testing.T) {
			db, mock := newMockGORM(t)
			mock.ExpectQuery("FROM composite_outputs.*" + item.pattern + ".*created_at >= \\? AND created_at < \\?").
				WithArgs(item.args...).WillReturnRows(sqlmock.NewRows(outputRowColumns))
			items, err := listComposeOutputs(db, ComposeOutputFilter{TeamID: 7, GameIDs: []string{"delta"}, Source: item.source, CreatedFrom: from, CreatedBefore: before})
			if err != nil || len(items) != 0 {
				t.Fatalf("items=%v err=%v", items, err)
			}
			assertExpectations(t, mock)
		})
	}
}

func TestListComposeOutputsMineAlsoMatchesUnownedCreatorRows(t *testing.T) {
	db, mock := newMockGORM(t)
	mock.ExpectQuery(`AND \(operator_id = \? OR \(operator_id IS NULL AND created_by = \?\)\)`).
		WithArgs(int64(7), "delta", int64(9), int64(9)).
		WillReturnRows(sqlmock.NewRows(outputRowColumns))
	items, err := listComposeOutputs(db, ComposeOutputFilter{TeamID: 7, GameIDs: []string{"delta"}, OperatorID: 9, IncludeUnownedCreatedBy: 9})
	if err != nil || len(items) != 0 {
		t.Fatalf("items=%v err=%v", items, err)
	}
	assertExpectations(t, mock)
}

func TestListComposeOutputsExplicitOperatorDoesNotFallBackToCreator(t *testing.T) {
	db, mock := newMockGORM(t)
	mock.ExpectQuery(`AND operator_id = \? ORDER BY id DESC`).
		WithArgs(int64(7), "delta", int64(9)).
		WillReturnRows(sqlmock.NewRows(outputRowColumns))
	items, err := listComposeOutputs(db, ComposeOutputFilter{TeamID: 7, GameIDs: []string{"delta"}, OperatorID: 9})
	if err != nil || len(items) != 0 {
		t.Fatalf("items=%v err=%v", items, err)
	}
	assertExpectations(t, mock)
}
