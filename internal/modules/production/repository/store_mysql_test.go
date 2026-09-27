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
		WillReturnRows(sqlmock.NewRows(materialColumns()).AddRow(int64(42), int64(7), "game-1", int64(8), "7123456789012345678", "Demo material", "https://source", "douyin", "Author", now, "ready", "materials/42.mp4", int64(100), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", []byte(`{"duration_seconds":10}`), nil, now, now, now))

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

func usageColumns() []string {
	return []string{"id", "team_id", "material_id", "user_id", "status", "removed_at", "created_at", "updated_at"}
}

func materialColumns() []string {
	return []string{"id", "team_id", "game_id", "source_content_id", "platform_content_id", "title", "source_url", "platform", "author_name", "published_at", "video_status", "source_object_key", "video_size_bytes", "video_sha256", "video_media_json", "video_error", "video_prepared_at", "created_at", "updated_at"}
}
