package repository

import (
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity/model"
)

func TestCategoryReferencesUsesCurrentAccountDisplayName(t *testing.T) {
	db, mock, closeDB := newIdentityMockGORM(t)
	defer closeDB()
	mock.ExpectQuery(`SELECT u.id, u.username, u.role`).WithArgs(model.CategoryID(4)).WillReturnRows(
		sqlmock.NewRows([]string{"id", "username", "role", "team_id", "team_name"}).AddRow(3, "operator01", "operator", 1, "团队"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT ma.id, COALESCE(NULLIF(ma.platform_name, ''), ma.platform_account_id, '')")).WithArgs(model.CategoryID(4)).WillReturnRows(
		sqlmock.NewRows([]string{"id", "name", "platform", "user_id", "username"}).AddRow("12", "平台昵称", "bilibili", 3, "operator01"))
	result, err := categoryReferences(db, 4)
	if err != nil || len(result.Users) != 1 || len(result.MediaAccounts) != 1 || result.MediaAccounts[0].Name != "平台昵称" {
		t.Fatalf("references=%+v, err=%v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTeamReferencesIncludeStandaloneCompositionBatch(t *testing.T) {
	db, mock, closeDB := newIdentityMockGORM(t)
	defer closeDB()
	// A batch need not have remaining tasks and has no team foreign key.
	mock.ExpectQuery(`SELECT[\s\S]*compose_task_batches WHERE team_id = \?`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	found, err := teamHasReferences(db, 9)
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCategoryDeletionPreservesHistoricalSourcesAndBatches(t *testing.T) {
	db, mock, closeDB := newIdentityMockGORM(t)
	defer closeDB()
	mock.ExpectBegin()
	// Source/discovery category FKs use SET NULL, batches have no category FK.
	mock.ExpectQuery(`SELECT[\s\S]*source_contents[\s\S]*discovery_strategies[\s\S]*compose_task_batches`).WillReturnRows(sqlmock.NewRows([]string{"used"}).AddRow(true))
	mock.ExpectRollback()
	err := deleteCategoryWithAudit(db, 4, model.AuditEvent{})
	if err != model.ErrCategoryInUse {
		t.Fatalf("got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
