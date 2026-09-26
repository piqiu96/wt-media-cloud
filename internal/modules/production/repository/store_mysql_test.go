package repository

import (
	"regexp"
	"strings"
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
	mock.ExpectExec(regexp.QuoteMeta(upsertUsageSQL)).
		WithArgs(int64(7), int64(42), int64(9), now, now).
		WillReturnResult(sqlmock.NewResult(11, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, team_id, material_id, user_id, status, removed_at, created_at, updated_at FROM material_usages WHERE id = LAST_INSERT_ID()")).
		WillReturnRows(sqlmock.NewRows(usageColumns()).AddRow(int64(11), int64(7), int64(42), int64(9), "active", nil, now, now))
	mock.ExpectCommit()

	usage, created, err := createOrRestoreUsage(db, CreateUsageInput{TeamID: identity.TeamID(7), MaterialID: 42, UserID: identity.UserID(9)}, now)
	if err != nil {
		t.Fatalf("createOrRestoreUsage() error = %v", err)
	}
	if usage.ID != 11 || usage.Status != model.MaterialUsageActive || usage.MaterialID != 42 || usage.UserID != 9 {
		t.Fatalf("usage = %+v", usage)
	}
	if !created {
		t.Fatal("an insert touches a row and must report as created")
	}
	assertExpectations(t, mock)
}

// The statement, in one place so the three cases below cannot drift apart from
// each other. It is quoted into a sqlmock expectation, so a change here is a
// change to what the code under test is required to send.
const upsertUsageSQL = "INSERT INTO material_usages (team_id, material_id, user_id, status, created_at, updated_at) VALUES (?,?,?,'active',?,?) ON DUPLICATE KEY UPDATE id = LAST_INSERT_ID(id), removed_at = IF(status = 'removed', NULL, removed_at), updated_at = IF(status = 'removed', VALUES(updated_at), updated_at), status = IF(status = 'removed', 'active', status)"

// `created` is read out of `RowsAffected`, and these are the three values MySQL
// can report for this statement. It is the field the route turns into 200 or 201,
// so the arm that matters is the middle one: an already-active relation must
// report zero changed rows, which is only true while `updated_at` is conditional.
//
// The two non-zero arms are here to keep the zero arm from being satisfiable by
// a comparison that is simply wrong — a `!= 1` would pass the zero case for the
// wrong reason and fail the restore.
func TestCreateOrRestoreUsageReportsWhetherTheRelationWasCreated(t *testing.T) {
	now := time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)
	for _, testCase := range []struct {
		name       string
		affected   int64
		lastID     int64
		wantCreate bool
	}{
		{"insert", 1, 11, true},
		{"restore", 2, 11, true},
		{"already active", 0, 0, false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			db, mock := newMockGORM(t)
			mock.ExpectBegin()
			mock.ExpectExec(regexp.QuoteMeta(upsertUsageSQL)).
				WithArgs(int64(7), int64(42), int64(9), now, now).
				WillReturnResult(sqlmock.NewResult(testCase.lastID, testCase.affected))
			mock.ExpectQuery(regexp.QuoteMeta("SELECT id, team_id, material_id, user_id, status, removed_at, created_at, updated_at FROM material_usages WHERE id = LAST_INSERT_ID()")).
				WillReturnRows(sqlmock.NewRows(usageColumns()).AddRow(int64(11), int64(7), int64(42), int64(9), "active", nil, now, now))
			mock.ExpectCommit()

			if _, created, err := createOrRestoreUsage(db, CreateUsageInput{TeamID: identity.TeamID(7), MaterialID: 42, UserID: identity.UserID(9)}, now); err != nil {
				t.Fatalf("createOrRestoreUsage() error = %v", err)
			} else if created != testCase.wantCreate {
				t.Fatalf("created = %t for RowsAffected = %d, want %t", created, testCase.affected, testCase.wantCreate)
			}
			assertExpectations(t, mock)
		})
	}
}

// The three clauses the statement cannot express as a single condition are
// asserted on the text, because no mock can execute them: sqlmock matches the
// statement and returns whatever it is told, so a wrong `ON DUPLICATE KEY UPDATE`
// body is invisible to every test above.
//
// MySQL evaluates those assignments left to right and a later one sees the value
// an earlier one wrote, which is why the order is part of the requirement rather
// than a style choice.
func TestCreateOrRestoreUsageStatementReadsStatusBeforeItRewritesIt(t *testing.T) {
	restoreClause := "status = IF(status = 'removed', 'active', status)"
	restoreIndex := strings.Index(upsertUsageSQL, restoreClause)
	if restoreIndex < 0 {
		t.Fatalf("the statement no longer restores a removed relation: %s", upsertUsageSQL)
	}
	for _, reader := range []string{
		"removed_at = IF(status = 'removed', NULL, removed_at)",
		"updated_at = IF(status = 'removed', VALUES(updated_at), updated_at)",
	} {
		index := strings.Index(upsertUsageSQL, reader)
		if index < 0 {
			t.Fatalf("the statement no longer guards %q: %s", reader, upsertUsageSQL)
		}
		if index > restoreIndex {
			t.Errorf("%q is evaluated after `status` has already been set to 'active', so its IF can never see 'removed'", reader)
		}
	}
	if strings.Contains(upsertUsageSQL, "updated_at = VALUES(updated_at)") {
		t.Error("an unconditional updated_at makes every click look like a change, so a repeat click would answer 201 forever")
	}
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

// The list a user sees is scoped by `user_id` in the query, not by filtering in
// Go. Both halves matter: an unscoped SELECT would return other users' rows and
// rely on the service to drop them, and a missing `status = 'active'` would show
// materials the user has already removed.
func TestListActiveUsagesScopesTheQueryToTheUserAndTheActiveStatus(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, team_id, material_id, user_id, status, removed_at, created_at, updated_at FROM material_usages WHERE user_id = ? AND status = 'active' ORDER BY updated_at DESC, id DESC")).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows(usageColumns()).AddRow(int64(11), int64(7), int64(42), int64(9), "active", nil, now, now))

	usages, err := listActiveUsages(db, identity.UserID(9))
	if err != nil {
		t.Fatalf("listActiveUsages() error = %v", err)
	}
	if len(usages) != 1 || usages[0].ID != 11 || usages[0].MaterialID != 42 {
		t.Fatalf("usages = %+v", usages)
	}
	assertExpectations(t, mock)
}

// A row that exists but belongs to someone else must read as absent rather than
// as an error: the caller answers 404 for it, and a caller that cannot tell
// "not yours" from "the database is down" would answer 500.
func TestFindUsageForUserDoesNotDistinguishSomeoneElsesRowFromNoRow(t *testing.T) {
	db, mock := newMockGORM(t)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, team_id, material_id, user_id, status, removed_at, created_at, updated_at FROM material_usages WHERE id = ? AND user_id = ?")).
		WithArgs(int64(5), int64(9)).
		WillReturnRows(sqlmock.NewRows(usageColumns()))

	usage, found, err := findUsageForUser(db, 5, identity.UserID(9))
	if err != nil {
		t.Fatalf("findUsageForUser() error = %v", err)
	}
	if found {
		t.Fatalf("found = true for usage = %+v", usage)
	}
	assertExpectations(t, mock)
}

// Removal is addressed by usage id and scoped to the user, and the conditional
// `status = 'active'` is what makes a second removal report zero rows instead of
// silently rewriting `removed_at`. The zero-row result is the caller's signal.
func TestRemoveUsageByIDReportsWhetherTheRowWasStillActive(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)
	mock.ExpectExec(regexp.QuoteMeta("UPDATE material_usages SET status = 'removed', removed_at = ?, updated_at = ? WHERE id = ? AND user_id = ? AND status = 'active'")).
		WithArgs(now, now, int64(5), int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE material_usages SET status = 'removed', removed_at = ?, updated_at = ? WHERE id = ? AND user_id = ? AND status = 'active'")).
		WithArgs(now, now, int64(5), int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	removed, err := removeUsageByID(db, 5, identity.UserID(9), now)
	if err != nil {
		t.Fatalf("removeUsageByID() error = %v", err)
	}
	if !removed {
		t.Fatal("an active row must report as removed")
	}
	removed, err = removeUsageByID(db, 5, identity.UserID(9), now)
	if err != nil {
		t.Fatalf("removeUsageByID() second call error = %v", err)
	}
	if removed {
		t.Fatal("an already-removed row must report as not removed")
	}
	assertExpectations(t, mock)
}

// The projection may only move forward. `ready` is the promise that the object
// key, the size and the hash were written together, and a click that read the row
// before a preparation finished arrives with a stale view of it — so the guard is
// the only thing standing between "start preparing" and "forget that this video is
// already on the object store and fetch it again".
//
// The two `WHERE` clauses are asserted as text because sqlmock cannot execute
// them: a mock that matched the statement would match it just as well with the
// status list dropped, which is the mutation this test exists to catch.
func TestMarkVideoPreparingOnlyLeavesTheStatesAPreparationMayStartFrom(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)
	mock.ExpectExec(regexp.QuoteMeta(markVideoPreparingSQL)).
		WithArgs(now, int64(42), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(markVideoPreparingSQL)).
		WithArgs(now, int64(42), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	preparing, err := markVideoPreparing(db, 7, 42, now)
	if err != nil {
		t.Fatalf("markVideoPreparing() error = %v", err)
	}
	if !preparing {
		t.Fatal("a row in not_downloaded must be reported as taken")
	}
	preparing, err = markVideoPreparing(db, 7, 42, now)
	if err != nil {
		t.Fatalf("markVideoPreparing() second call error = %v", err)
	}
	if preparing {
		t.Fatal("a row the guard refused must not be reported as taken")
	}
	assertExpectations(t, mock)
}

// The guard is what a mock cannot execute, so it is asserted on the text — and
// the fragments below are written out here rather than referred to through the
// constant, because an assertion that shares its expectation with the code under
// test cannot fail when that code changes. Dropping the status list, adding
// `ready` to it or narrowing it to one state changes the statement and must break
// this test.
func TestMarkVideoPreparingStatementNamesBothStatesAPreparationMayStartFrom(t *testing.T) {
	for _, fragment := range []string{
		"video_status IN ('not_downloaded', 'failed')",
		"video_status = 'downloading'",
		"video_error = ''",
		"id = ? AND team_id = ?",
	} {
		if !strings.Contains(markVideoPreparingSQL, fragment) {
			t.Errorf("the statement no longer contains %q, so a `ready` material can be dragged back to `downloading`: %s", fragment, markVideoPreparingSQL)
		}
	}
}

func TestMarkVideoPreparingRefusesAnIncompleteScope(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)
	for _, testCase := range []struct {
		name       string
		teamID     identity.TeamID
		materialID int64
	}{
		{"no team", 0, 42},
		{"no material", 7, 0},
	} {
		if _, err := markVideoPreparing(db, testCase.teamID, testCase.materialID, now); err == nil {
			t.Fatalf("%s: error = nil, want a refusal before any statement runs", testCase.name)
		}
	}
	assertExpectations(t, mock)
}

func usageColumns() []string {
	return []string{"id", "team_id", "material_id", "user_id", "status", "removed_at", "created_at", "updated_at"}
}

func materialColumns() []string {
	return []string{"id", "team_id", "game_id", "source_content_id", "title", "source_url", "platform", "author_name", "published_at", "video_status", "source_object_key", "video_size_bytes", "video_sha256", "video_media_json", "video_error", "video_prepared_at", "created_at", "updated_at"}
}
