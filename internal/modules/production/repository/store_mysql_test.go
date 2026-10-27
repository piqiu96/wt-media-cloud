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
		WillReturnRows(sqlmock.NewRows(materialColumns()).AddRow(int64(42), int64(7), "game-1", int64(8), "7123456789012345678", "Demo material", "https://source", "douyin", "Author", "https://cover/42.jpg", "https://author/42", int64(12000), int64(72313), int64(882), int64(4028), int64(8323), now, "ready", "available", "materials/42.mp4", int64(100), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", []byte(`{"duration_seconds":10}`), nil, now, now, now, int64(9)))

	material, found, err := findMaterial(db, 42, identity.TeamID(7))
	if err != nil {
		t.Fatalf("findMaterial() error = %v", err)
	}
	if !found || material.VideoStatus != model.VideoReady || material.SourceObjectKey != "materials/42.mp4" || material.GameID == nil || *material.GameID != "game-1" {
		t.Fatalf("material = %+v found=%v", material, found)
	}
	// The provider's id and the local row id are separate columns and both have
	// to land: reading one into the other is the whole defect this projection
	// was widened for.
	if material.SourceContentID != 8 || material.PlatformContentID != "7123456789012345678" {
		t.Fatalf("material ids = source_content_id %d platform_content_id %q, want 8 and 7123456789012345678",
			material.SourceContentID, material.PlatformContentID)
	}
	// The cover and the author's home page are the source row's own links, and
	// the two lists identify a material by them (CHG-20260930-069). A column the
	// SELECT forgets reads as an empty string here, which the UI would render as
	// a permanently missing cover rather than as an error.
	if material.CoverURL != "https://cover/42.jpg" || material.AuthorHomeURL != "https://author/42" {
		t.Fatalf("material links = cover %q author_home %q, want the source row's own values",
			material.CoverURL, material.AuthorHomeURL)
	}
	// A projection that forgot the column would read as the empty string — not one
	// of the three enum values — and the list would paint an unlabelled badge.
	if material.Status != model.MaterialAvailable {
		t.Fatalf("material status = %q, want %q", material.Status, model.MaterialAvailable)
	}
	// The content-pool statistics are crawl-time decision data for the detail
	// drawer; a missing column reads as zero, which is indistinguishable from
	// “the crawl saw no plays”.
	if material.ViewCount != 12000 || material.LikeCount != 72313 || material.FavoriteCount != 882 || material.CommentCount != 4028 || material.ShareCount != 8323 {
		t.Fatalf("material stats = view %d like %d favorite %d comment %d share %d",
			material.ViewCount, material.LikeCount, material.FavoriteCount, material.CommentCount, material.ShareCount)
	}
	// The creator is written once at materialization and never updated; a scan
	// that slipped one position would attribute the material to the wrong person.
	if material.CreatedBy != identity.UserID(9) {
		t.Fatalf("material created_by = %d, want 9", material.CreatedBy)
	}
	assertExpectations(t, mock)
}

// The two source links have to be in the projection's column list itself, not
// only in the scan: a SELECT that forgot them would still satisfy sqlmock (the
// expectation pins only the FROM clause), and the scan above would fail only at
// runtime with a column-count mismatch. Pinning the statement, in the same shape
// as `TestMarkVideoPreparingStatementNamesBothStatesAPreparationMayStartFrom`,
// is what makes the column list a reviewed fact.
func TestMaterialProjectionJoinsTheSourceRowLinksIntoEveryRead(t *testing.T) {
	for _, column := range []string{"s.cover_url", "s.author_home_url", "s.view_count", "s.like_count", "s.favorite_count", "s.comment_count", "s.share_count"} {
		if !strings.Contains(materialProjectionColumns, column) {
			t.Errorf("materialProjectionColumns no longer selects %s: %s", column, materialProjectionColumns)
		}
	}
}

// The lifecycle belongs in the column list itself: the sqlmock expectations pin only
// the FROM clause, so a SELECT that forgot `m.status` would still satisfy them and the
// scan would fail only at runtime, as a column-count mismatch.
func TestMaterialProjectionCarriesTheLibraryLifecycleStatus(t *testing.T) {
	if !strings.Contains(materialProjectionColumns, "m.status") {
		t.Errorf("materialProjectionColumns no longer selects m.status: %s", materialProjectionColumns)
	}
}

// The library names who created the material. Like the columns above, the
// projection has to select it itself; sqlmock pins only the FROM clause.
func TestMaterialProjectionCarriesTheCreator(t *testing.T) {
	if !strings.Contains(materialProjectionColumns, "m.created_by") {
		t.Errorf("materialProjectionColumns no longer selects m.created_by: %s", materialProjectionColumns)
	}
}

// The list a user sees is scoped by `user_id` in the query, not by filtering in
// Go — an unscoped SELECT would return other users' rows and rely on the service
// to drop them.
//
// It carries **both** relation states (CHG-20260930-069 task 15). Until then the
// statement filtered `status = 'active'`, which is exactly what the page's
// 「已放弃」rows and their 「恢复使用」button need to read back: a list that dropped
// them would leave the button unreachable and the removal irreversible in the UI.
// The removed row's `removed_at` has to survive the scan too — the row is what
// the page branches on, and a projection that dropped the column would render
// 已放弃 with an empty 加入时间.
//
// The two rows are ordered by `created_at DESC` because that is the timestamp the
// page shows: ordering by `updated_at` would make a row jump to the top the moment
// it is given up or restored, so the row the user just clicked would move out from
// under the cursor they are about to click again.
func TestListUsagesScopesTheQueryToTheUserAndKeepsBothRelationStates(t *testing.T) {
	db, mock := newMockGORM(t)
	added := time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)
	removedAt := time.Date(2026, 9, 27, 11, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, team_id, material_id, user_id, status, removed_at, created_at, updated_at FROM material_usages WHERE user_id = ? ORDER BY created_at DESC, id DESC")).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows(usageColumns()).
			AddRow(int64(11), int64(7), int64(42), int64(9), "removed", removedAt, added, removedAt).
			AddRow(int64(5), int64(7), int64(43), int64(9), "active", nil, added, added))

	usages, err := listUsages(db, identity.UserID(9))
	if err != nil {
		t.Fatalf("listUsages() error = %v", err)
	}
	if len(usages) != 2 {
		t.Fatalf("usages = %+v, want the given-up row alongside the one in use", usages)
	}
	if usages[0].Status != model.MaterialUsageRemoved || usages[0].RemovedAt == nil || !usages[0].RemovedAt.Equal(removedAt) {
		t.Fatalf("a given-up relation must read back as removed with its timestamp: %+v", usages[0])
	}
	if usages[1].Status != model.MaterialUsageActive || usages[1].RemovedAt != nil {
		t.Fatalf("an in-use relation must read back as active with no removal time: %+v", usages[1])
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

// Restoration is the inverse statement of the removal above, and it must clear
// `removed_at` in the same write. Setting only `status` would leave a row that
// reads 使用中 while still carrying the moment it was given up — the page renders
// that timestamp in the 使用情况 block, and 已放弃's history would then show up on a
// relation that is in use again.
//
// The statement is asserted as text for the same reason the WHERE clauses of the
// preparation guards are: sqlmock cannot execute it, and dropping the
// `status = 'removed'` guard would still match a mock that only looked at the
// table name. The guard is what keeps a concurrent second restore from rewriting
// the timestamp of a row this call did not touch.
//
// Zero affected rows is not an error. It means the row was no longer `removed` by
// the time the statement ran — another request restored it, or the actor's own
// first click did — which is the state this call was asked to reach. Reporting it
// as a failure would turn a successful second click into an error message.
func TestRestoreUsageByIDOnlyTakesBackARowTheUserGaveUp(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	statement := "UPDATE material_usages SET status = 'active', removed_at = NULL, updated_at = ? WHERE id = ? AND user_id = ? AND status = 'removed'"
	mock.ExpectExec(regexp.QuoteMeta(statement)).
		WithArgs(now, int64(5), int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(statement)).
		WithArgs(now, int64(5), int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := restoreUsageByID(db, 5, identity.UserID(9), now); err != nil {
		t.Fatalf("restoreUsageByID() error = %v", err)
	}
	if err := restoreUsageByID(db, 5, identity.UserID(9), now); err != nil {
		t.Fatalf("restoreUsageByID() on an already-active row error = %v, want the same success", err)
	}
	assertExpectations(t, mock)
}

// An incomplete scope must be refused before anything reaches MySQL: an `UPDATE`
// with no `WHERE user_id` would restore every user's row, and one with no id would
// read as a full-table write.
//
// The catch-all expectation is what turns that into a measurement. Asserting only
// `err != nil` would pass for the wrong reason — an Exec the mock was never told
// to expect also returns an error — so the assertion here is that the catch-all
// went **unused**, which is the positive control for "no statement was sent".
func TestRestoreUsageByIDRefusesAnIncompleteScope(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	for _, scope := range []struct {
		name    string
		usageID int64
		userID  identity.UserID
	}{
		{"no usage", 0, identity.UserID(9)},
		{"no user", 5, 0},
	} {
		db, mock := newMockGORM(t)
		mock.ExpectExec(".*").WillReturnResult(sqlmock.NewResult(0, 1))
		if err := restoreUsageByID(db, scope.usageID, scope.userID, now); err == nil {
			t.Fatalf("%s: restoreUsageByID() error = nil, want a refusal before any statement", scope.name)
		}
		if err := mock.ExpectationsWereMet(); err == nil {
			t.Fatalf("%s: restoreUsageByID() sent a statement, want a refusal before the database", scope.name)
		}
	}
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
		"video_status = 'ready' AND COALESCE(source_object_key, '') = ''",
		"video_status = 'downloading'",
		"video_error = ''",
		"id = ? AND team_id = ?",
	} {
		if !strings.Contains(markVideoPreparingSQL, fragment) {
			t.Errorf("the statement no longer contains %q, which is a state or a fact a preparation may start from: %s", fragment, markVideoPreparingSQL)
		}
	}
	// A ready row that names a key is not this statement's to take back: that is
	// the observed-missing case and needs the key-and-timestamp compare.
	if strings.Contains(markVideoPreparingSQL, "IN ('not_downloaded', 'failed', 'ready')") {
		t.Errorf("a plain ready row was added to the startable states, so a stale click can take back a prepared video: %s", markVideoPreparingSQL)
	}
}

// The take-back of a ready row is the one write that undoes a projection whose
// promise was observed false, so its predicate is two facts read by the caller:
// the exact key, and the moment it was written. A statement that matched on the
// material id alone would take back a preparation that finished in between.
func TestMarkVideoRepreparingScopesTheTakeBackToTheObservedKeyAndMoment(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	preparedAt := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	mock.ExpectExec(regexp.QuoteMeta(markVideoRepreparingSQL)).
		WithArgs(now, int64(42), int64(7), "materials/42/x.mp4", preparedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(markVideoRepreparingSQL)).
		WithArgs(now, int64(42), int64(7), "materials/42/x.mp4", preparedAt).
		WillReturnResult(sqlmock.NewResult(0, 0))

	repreparing, err := markVideoRepreparing(db, 7, 42, "materials/42/x.mp4", &preparedAt, now)
	if err != nil {
		t.Fatalf("markVideoRepreparing() error = %v", err)
	}
	if !repreparing {
		t.Fatal("a row still carrying the observed key and moment must be taken back")
	}
	repreparing, err = markVideoRepreparing(db, 7, 42, "materials/42/x.mp4", &preparedAt, now)
	if err != nil {
		t.Fatalf("markVideoRepreparing() second call error = %v", err)
	}
	if repreparing {
		t.Fatal("a row the compare refused must not be reported as taken")
	}
	assertExpectations(t, mock)

	for _, fragment := range []string{
		"video_status = 'ready'",
		"COALESCE(source_object_key, '') = ?",
		"video_prepared_at <=> ?",
		"video_error = ''",
	} {
		if !strings.Contains(markVideoRepreparingSQL, fragment) {
			t.Errorf("the take-back lost %q, so a newer preparation's result can be overwritten: %s", fragment, markVideoRepreparingSQL)
		}
	}
}

// The compare takes a NULL timestamp from a legacy row through MySQL's null-safe
// equality. A plain `=` would match nothing there, and the repair would never run
// for exactly the rows most likely to have lost their object.
func TestMarkVideoRepreparingMatchesALegacyNullPreparedAt(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	mock.ExpectExec(regexp.QuoteMeta(markVideoRepreparingSQL)).
		WithArgs(now, int64(42), int64(7), "materials/42/x.mp4", nil).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repreparing, err := markVideoRepreparing(db, 7, 42, "materials/42/x.mp4", nil, now)
	if err != nil || !repreparing {
		t.Fatalf("repreparing=%v error=%v, want a NULL prepared_at matched", repreparing, err)
	}
	assertExpectations(t, mock)
}

// An empty key is a keyless ready row, which belongs to `MarkVideoPreparing`;
// letting this statement also cover it would give two entry points with
// different guards the same meaning.
func TestMarkVideoRepreparingRefusesAnEmptyKeyOrIncompleteScope(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	for _, testCase := range []struct {
		name       string
		teamID     identity.TeamID
		materialID int64
		key        string
	}{
		{"no team", 0, 42, "materials/42/x.mp4"},
		{"no material", 7, 0, "materials/42/x.mp4"},
		{"no key", 7, 42, "   "},
	} {
		_, err := markVideoRepreparing(db, testCase.teamID, testCase.materialID, testCase.key, nil, now)
		if err == nil || !strings.Contains(err.Error(), "invalid material repreparation marker") {
			t.Fatalf("%s: error = %v, want the scope refused before any statement runs", testCase.name, err)
		}
	}
	assertExpectations(t, mock)
}

// The refusal is asserted by its reason rather than by "an error came back",
// because a statement that reaches an unarmed mock also produces an error: that
// form of the test passes whether the guard is there or not.
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
		_, err := markVideoPreparing(db, testCase.teamID, testCase.materialID, now)
		if err == nil || !strings.Contains(err.Error(), "invalid material preparation marker") {
			t.Fatalf("%s: error = %v, want the scope refused before any statement runs", testCase.name, err)
		}
	}
	assertExpectations(t, mock)
}

// The cancellation's write is the mirror image of the preparation marker above: it
// may only leave `downloading`, so a material that was prepared keeps the video it
// has, and a material that was never preparing is not touched at all.
//
// As above, the guard is asserted as text because sqlmock cannot execute a `WHERE`:
// a mock matches this statement just as well with the predicate dropped, and the
// dropped predicate is the mutation that would let a late cancellation take `ready`
// back to `not_downloaded` while the object the row named is still in the bucket.
func TestMarkVideoNotPreparedOnlyLeavesTheStateACancelledPreparationLeft(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)
	mock.ExpectExec(regexp.QuoteMeta(markVideoNotPreparedSQL)).
		WithArgs(now, int64(42), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(markVideoNotPreparedSQL)).
		WithArgs(now, int64(42), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	taken, err := markVideoNotPrepared(db, 7, 42, now)
	if err != nil {
		t.Fatalf("markVideoNotPrepared() error = %v", err)
	}
	if !taken {
		t.Fatal("a row in downloading must be reported as taken back")
	}
	// The second call stands for a material that is `ready`, `failed` or
	// `not_downloaded`: the predicate refuses all three, and — unlike the worker's
	// other writes — that is not an error, because there was nothing to take back.
	taken, err = markVideoNotPrepared(db, 7, 42, now)
	if err != nil {
		t.Fatalf("markVideoNotPrepared() second call error = %v", err)
	}
	if taken {
		t.Fatal("a row the guard refused must not be reported as taken back")
	}
	assertExpectations(t, mock)
}

// Written out rather than referred to through the constant, for the reason the
// preparation marker's version of this test gives: an assertion that shares its
// expectation with the code under test cannot fail when that code changes.
func TestMarkVideoNotPreparedStatementNamesTheOneStateItMayLeave(t *testing.T) {
	for _, fragment := range []string{
		"video_status = 'not_downloaded'",
		"video_status = 'downloading'",
		"video_error = ''",
		"id = ? AND team_id = ?",
	} {
		if !strings.Contains(markVideoNotPreparedSQL, fragment) {
			t.Errorf("the statement no longer contains %q, so a cancellation can take back a projection it does not own: %s", fragment, markVideoNotPreparedSQL)
		}
	}
}

// testDigest is 64 characters of hexadecimal, which is what the column and the
// video's own bytes both require. It is spelled out rather than produced by
// hashing something, so that no test here depends on what the digest is a digest
// of: this layer writes down a hash that was computed elsewhere.
var testDigest = strings.Repeat("a1", 32)

func verifiedFacts() VideoFacts {
	return VideoFacts{
		ObjectKey: "materials/42/" + testDigest + ".mp4",
		SizeBytes: 1048576,
		SHA256:    testDigest,
		Media:     []byte(`{"container":"mp4","duration_ms":5000}`),
	}
}

// The argument order is the thing this test actually pins: the statement sets six
// columns before it names the row, and a hash written into the size column would
// be accepted by the database and by a mock that was not watching.
func TestMarkVideoReadyWritesEveryFactOfAVerifiedSource(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	facts := verifiedFacts()
	mock.ExpectExec(regexp.QuoteMeta(markVideoReadySQL)).
		WithArgs(facts.ObjectKey, facts.SizeBytes, facts.SHA256, string(facts.Media), now, now, int64(42), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	written, err := markVideoReady(db, 7, 42, facts, now)
	if err != nil {
		t.Fatalf("markVideoReady() error = %v", err)
	}
	if !written {
		t.Fatal("a row that changed must be reported as written")
	}
	assertExpectations(t, mock)
}

// An unprobed file writes SQL NULL, not the four characters `null`. The two are
// different answers to "was this file probed", and only one of them leaves the
// column describing a document that was never written.
func TestMarkVideoReadyWritesNoMediaSummaryWhenTheFileWasNotProbed(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		media []byte
		want  any
	}{
		{"probed", []byte(`{"container":"mp4"}`), `{"container":"mp4"}`},
		{"not probed", nil, nil},
		{"probed to nothing", []byte{}, nil},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			db, mock := newMockGORM(t)
			now := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
			facts := verifiedFacts()
			facts.Media = testCase.media
			mock.ExpectExec(regexp.QuoteMeta(markVideoReadySQL)).
				WithArgs(facts.ObjectKey, facts.SizeBytes, facts.SHA256, testCase.want, now, now, int64(42), int64(7)).
				WillReturnResult(sqlmock.NewResult(0, 1))

			if _, err := markVideoReady(db, 7, 42, facts, now); err != nil {
				t.Fatalf("markVideoReady() error = %v", err)
			}
			assertExpectations(t, mock)
		})
	}
}

// `reason` is asserted and not merely the presence of an error: the statement is
// never armed on the mock here, so a fact set that got past validation would reach
// the mock and come back as an "unexpected call" error — which is an error too, and
// would make a test of the weaker form pass with a check deleted.
func TestMarkVideoReadyRefusesAFactSetThatIsNotComplete(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		amend  func(*VideoFacts)
		reason string
	}{
		{"no object key", func(facts *VideoFacts) { facts.ObjectKey = "" }, "must name its object key"},
		{"a blank object key", func(facts *VideoFacts) { facts.ObjectKey = "   " }, "must name its object key"},
		{"no size", func(facts *VideoFacts) { facts.SizeBytes = 0 }, "must have a positive size"},
		{"a negative size", func(facts *VideoFacts) { facts.SizeBytes = -1 }, "must have a positive size"},
		{"no hash", func(facts *VideoFacts) { facts.SHA256 = "" }, "must carry a 64-character sha256"},
		{"a short hash", func(facts *VideoFacts) { facts.SHA256 = testDigest[:63] }, "must carry a 64-character sha256"},
		{"a long hash", func(facts *VideoFacts) { facts.SHA256 = testDigest + "a1" }, "must carry a 64-character sha256"},
		{"a hash that is not hexadecimal", func(facts *VideoFacts) { facts.SHA256 = strings.Repeat("zz", 32) }, "is not hexadecimal"},
		{"a media summary that is not JSON", func(facts *VideoFacts) { facts.Media = []byte("<html>") }, "media summary is not JSON"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			db, mock := newMockGORM(t)
			now := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
			facts := verifiedFacts()
			testCase.amend(&facts)

			written, err := markVideoReady(db, 7, 42, facts, now)
			if err == nil || !strings.Contains(err.Error(), testCase.reason) {
				t.Fatalf("error = %v, want a refusal containing %q rather than a row that says ready about nothing", err, testCase.reason)
			}
			if written {
				t.Fatal("a refused fact set must not be reported as written")
			}
			// No expectation was armed, so this failing would mean a statement ran.
			assertExpectations(t, mock)
		})
	}
}

// What reaches the columns is the value, not the string it arrived in. A hash in
// capitals names the same bytes, so it is lower-cased rather than refused: the
// column is compared against digests other layers computed.
//
// Surrounding whitespace is stripped because validation already accepted the value
// without it — the write has to agree with the check that let it through. It is not
// cosmetic for the hash: `video_sha256` is `CHAR(64)`, so a padded value that
// validation trimmed to 64 characters would be 66 characters at the statement and
// would fail the whole update in strict mode.
func TestMarkVideoReadyWritesTheValueNotTheStringItArrivedIn(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		amend      func(*VideoFacts)
		wantKey    string
		wantDigest string
	}{
		{"a hash in capitals", func(facts *VideoFacts) { facts.SHA256 = strings.ToUpper(testDigest) }, "materials/42/" + testDigest + ".mp4", testDigest},
		{"a padded object key", func(facts *VideoFacts) { facts.ObjectKey = "\t materials/42/" + testDigest + ".mp4 \n" }, "materials/42/" + testDigest + ".mp4", testDigest},
		{"a padded hash", func(facts *VideoFacts) { facts.SHA256 = "  " + testDigest + "  " }, "materials/42/" + testDigest + ".mp4", testDigest},
		{"a padded hash in capitals", func(facts *VideoFacts) { facts.SHA256 = "\n" + strings.ToUpper(testDigest) + " " }, "materials/42/" + testDigest + ".mp4", testDigest},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			db, mock := newMockGORM(t)
			now := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
			facts := verifiedFacts()
			testCase.amend(&facts)
			mock.ExpectExec(regexp.QuoteMeta(markVideoReadySQL)).
				WithArgs(testCase.wantKey, facts.SizeBytes, testCase.wantDigest, string(facts.Media), now, now, int64(42), int64(7)).
				WillReturnResult(sqlmock.NewResult(0, 1))

			if _, err := markVideoReady(db, 7, 42, facts, now); err != nil {
				t.Fatalf("markVideoReady() error = %v", err)
			}
			assertExpectations(t, mock)
		})
	}
}

// The statement's own shape, asserted as text because a mock cannot execute a
// `WHERE` clause: a matched statement would match just as well with a predicate
// added, and an added predicate is the failure this test is here to notice — the
// absence of one is what lets a retry that finishes after an earlier attempt write
// its facts from whatever state the projection happens to be in.
//
// The fragments are literals here rather than references to the constant, so that
// changing the statement changes this test.
func TestMarkVideoReadyStatementWritesAllFourFactsAndGuardsOnNothing(t *testing.T) {
	for _, fragment := range []string{
		"video_status = 'ready'",
		"source_object_key = ?",
		"video_size_bytes = ?",
		"video_sha256 = ?",
		"video_media_json = ?",
		"video_error = ''",
		"video_prepared_at = ?",
		"id = ? AND team_id = ?",
		// Also what makes the row count readable. The connection does not set
		// `CLIENT_FOUND_ROWS`, so MySQL reports rows changed rather than rows
		// matched, and the caller reads a zero as "the scope named no material". A
		// statement that could match a row without changing it would make that
		// reading wrong, and this assignment to a fresh timestamp is what rules it
		// out: every matched row is a changed row.
		"updated_at = ?",
	} {
		if !strings.Contains(markVideoReadySQL, fragment) {
			t.Errorf("the statement no longer contains %q, so a part of the verified source is not written: %s", fragment, markVideoReadySQL)
		}
	}
	if strings.Contains(markVideoReadySQL, "WHERE id = ? AND team_id = ? AND") {
		t.Errorf("the statement grew a predicate: a preparation may finish from any state, and a predicate would refuse the states it forgot: %s", markVideoReadySQL)
	}
}

// A failure must not take back a material that has a verified video. The retry
// that fails after a slower earlier attempt succeeded is the case: without the
// predicate the row would say the video is unavailable while the object it names
// sits in the bucket.
func TestMarkVideoFailedWillNotTakeBackAReadyMaterial(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	mock.ExpectExec(regexp.QuoteMeta(markVideoFailedSQL)).
		WithArgs("source http status 404: Not Found", now, int64(42), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(markVideoFailedSQL)).
		WithArgs("source http status 404: Not Found", now, int64(42), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	failed, err := markVideoFailed(db, 7, 42, "  source http status 404: Not Found  ", now)
	if err != nil {
		t.Fatalf("markVideoFailed() error = %v", err)
	}
	if !failed {
		t.Fatal("a row that changed must be reported as failed")
	}
	failed, err = markVideoFailed(db, 7, 42, "source http status 404: Not Found", now)
	if err != nil {
		t.Fatalf("markVideoFailed() second call error = %v", err)
	}
	if failed {
		t.Fatal("a ready material must not be reported as taken back to failed")
	}
	assertExpectations(t, mock)

	if !strings.Contains(markVideoFailedSQL, "video_status <> 'ready'") {
		t.Errorf("the statement no longer refuses a ready material: %s", markVideoFailedSQL)
	}
	// The exception: a ready row that names no key never had a video to keep, so a
	// preparation that failed can and must say so.
	if !strings.Contains(markVideoFailedSQL, "COALESCE(source_object_key, '') = ''") {
		t.Errorf("a keyless ready row can never be marked failed: %s", markVideoFailedSQL)
	}
}

// The column is 500 characters wide and MySQL runs in strict mode, where an
// over-long value is an error rather than a silent truncation. Losing the whole
// statement would leave the material saying `downloading` with a task that has
// already given up, which is the one outcome a failure record exists to prevent.
//
// The rune count is what matters and not the byte count: an error message carrying
// Chinese text is three bytes per character, so a cut measured in bytes would
// refuse a message that fits.
func TestMarkVideoFailedCutsAMessageToTheColumnWidth(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		message string
		want    string
	}{
		{"a short message", "source http status 404", "source http status 404"},
		{"surrounding whitespace", "  boom  ", "boom"},
		{"exactly the column width", strings.Repeat("x", videoErrorLimit), strings.Repeat("x", videoErrorLimit)},
		{"one character too many", strings.Repeat("x", videoErrorLimit+1), strings.Repeat("x", videoErrorLimit-1) + "…"},
		{"far too many", strings.Repeat("x", 4096), strings.Repeat("x", videoErrorLimit-1) + "…"},
		{"too many characters that are not one byte each", strings.Repeat("失", videoErrorLimit+1), strings.Repeat("失", videoErrorLimit-1) + "…"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			got := boundedVideoError(testCase.message)
			if got != testCase.want {
				t.Fatalf("boundedVideoError() = %q (%d runes), want %q", got, len([]rune(got)), testCase.want)
			}
			if len([]rune(got)) > videoErrorLimit {
				t.Fatalf("the message is %d characters, longer than the column", len([]rune(got)))
			}
		})
	}
}

// The number is written out here rather than read from the constant, because a
// limit that drifts from the column is exactly the failure this guards: the tests
// above measure against `videoErrorLimit`, so they would all still pass if it
// changed. The column is `materials.video_error VARCHAR(500)`
// (`migrations/20260926_039_content_production_m4_a.sql`).
func TestVideoErrorLimitIsTheWidthOfItsColumn(t *testing.T) {
	if videoErrorLimit != 500 {
		t.Fatalf("videoErrorLimit = %d, but `materials.video_error` is VARCHAR(500): a longer cut is error 1406 in strict mode, a shorter one loses a reason it had room for", videoErrorLimit)
	}
}

// The bounded value is what the statement carries, asserted through the statement
// rather than only through the helper: a caller that bounded the message and then
// passed the raw one would satisfy every test above.
func TestMarkVideoFailedWritesTheBoundedMessage(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	mock.ExpectExec(regexp.QuoteMeta(markVideoFailedSQL)).
		WithArgs(strings.Repeat("x", videoErrorLimit-1)+"…", now, int64(42), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if _, err := markVideoFailed(db, 7, 42, strings.Repeat("x", 4096), now); err != nil {
		t.Fatalf("markVideoFailed() error = %v", err)
	}
	assertExpectations(t, mock)
}

func TestVideoProjectionWritesRefuseAnIncompleteScope(t *testing.T) {
	db, mock := newMockGORM(t)
	now := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	for _, testCase := range []struct {
		name       string
		teamID     identity.TeamID
		materialID int64
	}{
		{"no team", 0, 42},
		{"no material", 7, 0},
	} {
		// Asserted by its reason, not by "an error came back": a statement that
		// reaches an unarmed mock also produces an error, so the weaker form would
		// pass with the guard deleted.
		_, err := markVideoReady(db, testCase.teamID, testCase.materialID, verifiedFacts(), now)
		if err == nil || !strings.Contains(err.Error(), "invalid material readiness marker") {
			t.Fatalf("markVideoReady %s: error = %v, want the scope refused before any statement runs", testCase.name, err)
		}
		_, err = markVideoFailed(db, testCase.teamID, testCase.materialID, "boom", now)
		if err == nil || !strings.Contains(err.Error(), "invalid material failure marker") {
			t.Fatalf("markVideoFailed %s: error = %v, want the scope refused before any statement runs", testCase.name, err)
		}
	}
	assertExpectations(t, mock)
}

// The library's three usage dimensions are independent predicates, and the
// team is repeated on the output join: a successful output from another team's
// copy of the same external work must never make this team's material look
// used. The joined relation is the viewer's and only an active row counts.
func TestListMaterialsAppliesTheUsageDimensionsAsIndependentPredicates(t *testing.T) {
	team := identity.TeamID(7)
	db, mock := newMockGORM(t)
	fragment := func(parts ...string) string {
		quoted := make([]string, 0, len(parts))
		for _, part := range parts {
			quoted = append(quoted, regexp.QuoteMeta(part))
		}
		return strings.Join(quoted, ".*")
	}
	mock.ExpectQuery(fragment(
		"FROM materials m JOIN source_contents s ON s.id = m.source_content_id",
		"m.team_id = ?",
		"s.game_id IN (?,?)",
		"EXISTS (SELECT 1 FROM material_usages u WHERE u.material_id = m.id AND u.user_id = ? AND u.status = 'active')",
		"EXISTS (SELECT 1 FROM composite_outputs o WHERE o.material_id = m.id AND o.team_id = m.team_id AND o.operator_id = ?)",
		"NOT EXISTS (SELECT 1 FROM composite_outputs o WHERE o.material_id = m.id AND o.team_id = m.team_id AND o.operator_id IS NOT NULL)",
	)).WithArgs(int64(7), "game-a", "game-b", int64(9), int64(9)).
		WillReturnRows(sqlmock.NewRows(materialColumns()))

	if _, err := listMaterials(db, MaterialFilter{
		TeamID:    &team,
		GameIDs:   []string{"game-a", "game-b"},
		Joined:    "joined",
		MyUsage:   "used",
		TeamUsage: "unused",
		UserID:    identity.UserID(9),
	}); err != nil {
		t.Fatalf("listMaterials() error = %v", err)
	}
	assertExpectations(t, mock)

	// The opposite arms are separate because each NOT EXISTS is a different
	// question; a filter that only ever built the positive predicates would pass
	// the arm above.
	db, mock = newMockGORM(t)
	mock.ExpectQuery(fragment(
		"FROM materials m JOIN source_contents s ON s.id = m.source_content_id",
		"NOT EXISTS (SELECT 1 FROM material_usages u WHERE u.material_id = m.id AND u.user_id = ? AND u.status = 'active')",
		"NOT EXISTS (SELECT 1 FROM composite_outputs o WHERE o.material_id = m.id AND o.team_id = m.team_id AND o.operator_id = ?)",
		"EXISTS (SELECT 1 FROM composite_outputs o WHERE o.material_id = m.id AND o.team_id = m.team_id AND o.operator_id IS NOT NULL)",
	)).WithArgs(int64(9), int64(9)).
		WillReturnRows(sqlmock.NewRows(materialColumns()))

	if _, err := listMaterials(db, MaterialFilter{Joined: "not_joined", MyUsage: "unused", TeamUsage: "used", UserID: identity.UserID(9)}); err != nil {
		t.Fatalf("listMaterials() error = %v", err)
	}
	assertExpectations(t, mock)
}

// One material's history can hold both named and historical unnamed outputs.
// The count is per material and operator, so the caller can add the unnamed
// rows to the total without inventing a person for them.
func TestListMaterialOutputFactsKeepsUnknownOperatorsOutOfTheGrouping(t *testing.T) {
	db, mock := newMockGORM(t)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT o.material_id, o.operator_id, COUNT(*) FROM composite_outputs o JOIN materials m ON m.id = o.material_id AND m.team_id = o.team_id WHERE o.material_id IN (?,?) GROUP BY o.material_id, o.operator_id ORDER BY o.material_id, o.operator_id")).
		WithArgs(int64(42), int64(30)).
		WillReturnRows(sqlmock.NewRows([]string{"material_id", "operator_id", "count"}).
			AddRow(int64(42), nil, int64(3)).
			AddRow(int64(42), int64(9), int64(2)).
			AddRow(int64(30), int64(9), int64(1)))

	facts, err := listMaterialOutputFacts(db, []int64{42, 30, 42})
	if err != nil {
		t.Fatalf("listMaterialOutputFacts() error = %v", err)
	}
	if len(facts) != 3 {
		t.Fatalf("facts = %+v, want three (material, operator) groups", facts)
	}
	if facts[0].MaterialID != 42 || facts[0].OperatorID != nil || facts[0].OutputCount != 3 {
		t.Fatalf("the historical group = %+v, want a nil operator with three outputs", facts[0])
	}
	if facts[1].OperatorID == nil || *facts[1].OperatorID != 9 || facts[1].OutputCount != 2 {
		t.Fatalf("the named group = %+v, want operator 9 with two outputs", facts[1])
	}
	assertExpectations(t, mock)
}

// A removed relation is still a relation: the library asks for both states so
// the page can show 已移出 with the original added time.
func TestListUsageRelationsReadsBothStatesForTheViewer(t *testing.T) {
	db, mock := newMockGORM(t)
	added := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT material_id, status, created_at FROM material_usages WHERE user_id = ? AND material_id IN (?,?) ORDER BY material_id")).
		WithArgs(int64(9), int64(42), int64(30)).
		WillReturnRows(sqlmock.NewRows([]string{"material_id", "status", "created_at"}).
			AddRow(int64(30), "removed", added).
			AddRow(int64(42), "active", added))

	relations, err := listUsageRelations(db, identity.UserID(9), []int64{42, 30, 42})
	if err != nil {
		t.Fatalf("listUsageRelations() error = %v", err)
	}
	if len(relations) != 2 || relations[0].MaterialID != 30 || relations[0].Status != model.MaterialUsageRemoved || !relations[0].CreatedAt.Equal(added) {
		t.Fatalf("relations = %+v, want both rows with their original state", relations)
	}
	assertExpectations(t, mock)
}

// The gate answers only for an active relation. A removed row and no row are
// the same refusal; the download route turns both into the same 403.
func TestHasActiveUsageAnswersOnlyForAnActiveRelation(t *testing.T) {
	const statement = "SELECT 1 FROM material_usages WHERE material_id = ? AND user_id = ? AND status = 'active' LIMIT 1"
	db, mock := newMockGORM(t)
	mock.ExpectQuery(regexp.QuoteMeta(statement)).
		WithArgs(int64(42), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	active, err := hasActiveUsage(db, 42, identity.UserID(9))
	if err != nil || !active {
		t.Fatalf("hasActiveUsage() = %v, %v; want true", active, err)
	}
	assertExpectations(t, mock)

	db, mock = newMockGORM(t)
	mock.ExpectQuery(regexp.QuoteMeta(statement)).
		WithArgs(int64(42), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}))
	active, err = hasActiveUsage(db, 42, identity.UserID(9))
	if err != nil || active {
		t.Fatalf("hasActiveUsage() = %v, %v; want false and no error", active, err)
	}
	assertExpectations(t, mock)
}

func usageColumns() []string {
	return []string{"id", "team_id", "material_id", "user_id", "status", "removed_at", "created_at", "updated_at"}
}

func materialColumns() []string {
	return []string{"id", "team_id", "game_id", "source_content_id", "platform_content_id", "title", "source_url", "platform", "author_name", "cover_url", "author_home_url", "view_count", "like_count", "favorite_count", "comment_count", "share_count", "published_at", "video_status", "status", "source_object_key", "video_size_bytes", "video_sha256", "video_media_json", "video_error", "video_prepared_at", "created_at", "updated_at", "created_by"}
}
