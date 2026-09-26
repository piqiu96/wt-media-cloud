package repository

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

func TestCreateOrRestoreUsageKeepsOneActiveRelation(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO material_usages (team_id, material_id, user_id, status, created_at, updated_at) VALUES (?,?,?,'active',?,?) ON DUPLICATE KEY UPDATE id = LAST_INSERT_ID(id), status = IF(status = 'removed', 'active', status), removed_at = IF(status = 'removed', NULL, removed_at), updated_at = VALUES(updated_at)")).
		WithArgs(int64(7), int64(42), int64(9), now, now).
		WillReturnResult(sqlmock.NewResult(11, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, team_id, material_id, user_id, status, removed_at, created_at, updated_at FROM material_usages WHERE id = LAST_INSERT_ID()")).
		WillReturnRows(sqlmock.NewRows(usageColumns()).AddRow(int64(11), int64(7), int64(42), int64(9), "active", nil, now, now))
	mock.ExpectCommit()

	usage, err := createOrRestoreUsage(db, CreateUsageInput{TeamID: identity.TeamID(7), MaterialID: 42, UserID: identity.UserID(9)}, now)
	if err != nil {
		t.Fatalf("createOrRestoreUsage() error = %v", err)
	}
	if usage.ID != 11 || usage.Status != model.MaterialUsageActive || usage.MaterialID != 42 || usage.UserID != 9 {
		t.Fatalf("usage = %+v", usage)
	}
	assertExpectations(t, mock)
}

func TestRemoveUsageDoesNotPhysicallyDeleteHistory(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)
	mock.ExpectExec(regexp.QuoteMeta("UPDATE material_usages SET status = 'removed', removed_at = ?, updated_at = ? WHERE team_id = ? AND material_id = ? AND user_id = ? AND status = 'active'")).
		WithArgs(now, now, int64(7), int64(42), int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	removed, err := removeUsage(db, identity.TeamID(7), 42, identity.UserID(9), now)
	if err != nil {
		t.Fatalf("removeUsage() error = %v", err)
	}
	if !removed {
		t.Fatal("expected active relation to be marked removed")
	}
	assertExpectations(t, mock)
}

func TestFindMaterialScopesTheSourceAndVideoProjectionByTeam(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("FROM materials m JOIN source_contents s ON s.id = m.source_content_id WHERE m.id = ? AND m.team_id = ?")).
		WithArgs(int64(42), int64(7)).
		WillReturnRows(sqlmock.NewRows(materialColumns()).AddRow(int64(42), int64(7), "game-1", int64(8), "Demo material", "https://source", "douyin", "Author", now, "ready", "materials/42.mp4", int64(100), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", []byte(`{"duration_seconds":10}`), nil, now, now, now))

	material, found, err := findMaterial(db, 42, identity.TeamID(7))
	if err != nil {
		t.Fatalf("findMaterial() error = %v", err)
	}
	if !found || material.VideoStatus != model.VideoReady || material.SourceObjectKey != "materials/42.mp4" || material.GameID == nil || *material.GameID != "game-1" {
		t.Fatalf("material = %+v found=%v", material, found)
	}
	assertExpectations(t, mock)
}

func usageColumns() []string {
	return []string{"id", "team_id", "material_id", "user_id", "status", "removed_at", "created_at", "updated_at"}
}

func materialColumns() []string {
	return []string{"id", "team_id", "game_id", "source_content_id", "title", "source_url", "platform", "author_name", "published_at", "video_status", "source_object_key", "video_size_bytes", "video_sha256", "video_media_json", "video_error", "video_prepared_at", "created_at", "updated_at"}
}
