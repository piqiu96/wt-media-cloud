package repository

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/executor"
)

func TestPublishTemplateReadsBackTheDeployedDefinition(t *testing.T) {
	db, mock := newMockGORM(t)
	definition := executor.BasicCutV1()
	now := time.Date(2026, 10, 12, 15, 0, 0, 0, time.UTC)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO compose_templates")).
		WithArgs(definition.Code, "基础切段", definition.Version, definition.Schema, definition.ExecutorID, now, now).
		WillReturnResult(sqlmock.NewResult(7, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, code, name, version, schema_json, executor, status FROM compose_templates")).
		WithArgs(definition.Code, definition.Version).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name", "version", "schema_json", "executor", "status"}).
			AddRow(7, "basic_cut", "基础切段", 1, []byte(definition.Schema), "basic_cut_v1", "active"))
	item, err := publishTemplate(db, definition, "基础切段", now)
	if err != nil {
		t.Fatal(err)
	}
	if item.ID != 7 || item.Code != "basic_cut" || item.Version != 1 {
		t.Fatalf("published template = %+v", item)
	}
	assertExpectations(t, mock)
}

func TestPublishTemplateRejectsAnExistingVersionWithDifferentSchema(t *testing.T) {
	db, mock := newMockGORM(t)
	definition := executor.BasicCutV1()
	now := time.Date(2026, 10, 12, 15, 0, 0, 0, time.UTC)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO compose_templates")).
		WithArgs(definition.Code, "基础切段", definition.Version, definition.Schema, definition.ExecutorID, now, now).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, code, name, version, schema_json, executor, status FROM compose_templates")).
		WithArgs(definition.Code, definition.Version).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name", "version", "schema_json", "executor", "status"}).
			AddRow(7, "basic_cut", "基础切段", 1, []byte(`{"type":"object"}`), "basic_cut_v1", "active"))
	_, err := publishTemplate(db, definition, "基础切段", now)
	if !errors.Is(err, ErrTemplateDefinitionConflict) {
		t.Fatalf("error = %v, want ErrTemplateDefinitionConflict", err)
	}
	assertExpectations(t, mock)
}
