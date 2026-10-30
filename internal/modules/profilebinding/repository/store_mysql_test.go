package repository

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/model"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
	mysqlgorm "gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestProfileBindingMigrationKeepsScansAndProfilesSeparate(t *testing.T) {
	content, err := readFileAsString("../../../../migrations/20260714_003_browser_profiles.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	for _, required := range []string{"CREATE TABLE profile_sync_scans", "CREATE TABLE browser_profiles", "bit_profile_id"} {
		if !containsString(content, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
}

func TestMissingWindowRetainsFormalProxyOccupancy(t *testing.T) {
	db, mock, closeDB := newProfileMockGORM(t)
	defer closeDB()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM browser_profiles WHERE proxy_id = ? AND local_state IN (?, ?)")).
		WithArgs("42", model.ProfileActive, model.ProfileLocalMissing).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	count, err := countProfilesByProxyID(db, "42")
	if err != nil || count != 2 {
		t.Fatalf("formal occupancy = %d, %v; missing windows must keep their slot", count, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteMissingProfileDetachesAccountsAndAuditsAtomically(t *testing.T) {
	db, mock, closeDB := newProfileMockGORM(t)
	defer closeDB()
	at := time.Date(2026, 10, 30, 10, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT local_state, reserved_proxy_id, proxy_id, bit_profile_id, user_id FROM browser_profiles WHERE id = ? FOR UPDATE")).
		WithArgs("42").WillReturnRows(sqlmock.NewRows([]string{"local_state", "reserved_proxy_id", "proxy_id", "bit_profile_id", "user_id"}).AddRow("local_missing", nil, 7, "bit-42", 2))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM media_accounts WHERE browser_profile_id = ?")).
		WithArgs("42").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectExec("UPDATE media_accounts SET browser_profile_id = NULL").WithArgs(at, "42").WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("DELETE FROM browser_profiles WHERE id =").WithArgs("42").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO audit_logs").WithArgs(sqlmock.AnyArg(), sharedidentity.UserID(2), "bitbrowser.profile_missing.delete", "browser_profile", "42", sqlmock.AnyArg(), at).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := deleteProfile(db, "42", 2, at); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteProfileRejectsWindowThatReappearedDuringCleanup(t *testing.T) {
	db, mock, closeDB := newProfileMockGORM(t)
	defer closeDB()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT local_state, reserved_proxy_id").WithArgs("42").
		WillReturnRows(sqlmock.NewRows([]string{"local_state", "reserved_proxy_id", "proxy_id", "bit_profile_id", "user_id"}).AddRow("active", nil, nil, "bit-42", 2))
	mock.ExpectRollback()
	if err := deleteProfile(db, "42", 2, time.Now()); !errors.Is(err, model.ErrProfileNotMissing) {
		t.Fatalf("deleteProfile() = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteMissingProfileRollsBackWhenAuditCannotBeWritten(t *testing.T) {
	db, mock, closeDB := newProfileMockGORM(t)
	defer closeDB()
	at := time.Date(2026, 10, 30, 10, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT local_state, reserved_proxy_id").WithArgs("42").
		WillReturnRows(sqlmock.NewRows([]string{"local_state", "reserved_proxy_id", "proxy_id", "bit_profile_id", "user_id"}).AddRow("local_missing", nil, 7, "bit-42", 2))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM media_accounts WHERE browser_profile_id = ?")).WithArgs("42").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec("UPDATE media_accounts SET browser_profile_id = NULL").WithArgs(at, "42").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM browser_profiles WHERE id =").WithArgs("42").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO audit_logs").WillReturnError(errors.New("audit unavailable"))
	mock.ExpectRollback()
	if err := deleteProfile(db, "42", 2, at); err == nil {
		t.Fatal("deleteProfile() succeeded despite audit failure")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListProfilesFiltersAndOrdersInSQL(t *testing.T) {
	db, mock, closeDB := newProfileMockGORM(t)
	defer closeDB()
	mock.ExpectQuery(`SELECT bp\.id, .* FROM browser_profiles bp JOIN users u ON u\.id = bp\.user_id WHERE bp\.bit_main_user_id = u\.bit_main_user_id AND bp\.user_id = \? AND bp\.id LIKE \? ESCAPE '!' AND bp\.bit_name LIKE \? ESCAPE '!' AND \(bp\.bit_group_id LIKE \? ESCAPE '!' OR bp\.bit_group_name LIKE \? ESCAPE '!'\) AND bp\.bit_profile_id LIKE \? ESCAPE '!' AND bp\.remark LIKE \? ESCAPE '!' AND bp\.local_state = \? AND bp\.enabled = \? ORDER BY bp\.bit_seq DESC, bp\.id DESC`).
		WithArgs(sharedidentity.UserID(5), "%bp!_1%", "%窗口!_1%", "%分组!_1%", "%分组!_1%", "%bit!_1%", "%备注!_1%", "active", true).
		WillReturnRows(sqlmock.NewRows(strings.Split(profileColumnsSQL, ", ")))
	items, err := listProfiles(db, ProfileListFilter{
		UserID: 5, ID: "bp_1", BitName: "窗口_1", BitGroup: "分组_1",
		BitProfileID: "bit_1", Remark: "备注_1", LocalState: "active", Enabled: boolPointer(true),
	})
	if err != nil || len(items) != 0 {
		t.Fatalf("items=%v err=%v", items, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFindBindingUsesGORMAndMapsIdentity(t *testing.T) {
	db, mock, closeDB := newProfileMockGORM(t)
	defer closeDB()
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, bit_main_user_id, bit_account_status")).
		WithArgs(sharedidentity.UserID(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "bit_main_user_id", "bit_account_status", "bit_account_bound_at", "bit_account_last_verified_at"}).
			AddRow(sharedidentity.UserID(1), "main-user-1", "bound", now, now))

	binding, found, err := findBinding(db, 1)
	if err != nil || !found {
		t.Fatalf("findBinding() = %v, %v, want found without error", found, err)
	}
	if binding.BitMainUserID != "main-user-1" || binding.Status != model.BitAccountBound {
		t.Fatalf("binding = %#v", binding)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestBindBitAccountFirstBindWritesBindingAndAudit(t *testing.T) {
	db, mock, closeDB := newProfileMockGORM(t)
	defer closeDB()
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT bit_main_user_id, bit_account_bound_at FROM users WHERE id = ? FOR UPDATE")).
		WithArgs(sharedidentity.UserID(1)).
		WillReturnRows(sqlmock.NewRows([]string{"bit_main_user_id", "bit_account_bound_at"}).AddRow(nil, nil))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE users SET bit_main_user_id = ?, bit_account_status = ?, bit_account_bound_at = ?, bit_account_last_verified_at = ?, updated_at = ? WHERE id = ?")).
		WithArgs("main-user-a", model.BitAccountBound, now, now, now, sharedidentity.UserID(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO audit_logs").
		WithArgs(sqlmock.AnyArg(), sharedidentity.UserID(1), model.AuditMainAccountBind, "user", sharedidentity.UserID(1), sqlmock.AnyArg(), now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	binding, err := bindBitAccount(db, 1, "main-user-a", false, now)
	if err != nil {
		t.Fatalf("bindBitAccount() error = %v", err)
	}
	if binding.BitMainUserID != "main-user-a" || binding.BoundAt == nil || !binding.BoundAt.Equal(now) {
		t.Fatalf("binding = %#v", binding)
	}
}

func TestBindBitAccountRefusesMismatchWithoutRebind(t *testing.T) {
	db, mock, closeDB := newProfileMockGORM(t)
	defer closeDB()
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT bit_main_user_id, bit_account_bound_at FROM users WHERE id = ? FOR UPDATE")).
		WithArgs(sharedidentity.UserID(1)).
		WillReturnRows(sqlmock.NewRows([]string{"bit_main_user_id", "bit_account_bound_at"}).AddRow("main-user-a", now))
	mock.ExpectRollback()

	if _, err := bindBitAccount(db, 1, "main-user-b", false, now); err != model.ErrBitMainUserMismatch {
		t.Fatalf("bindBitAccount() error = %v, want ErrBitMainUserMismatch", err)
	}
}

func TestBindBitAccountRebindKeepsThePreviousMainInTheAudit(t *testing.T) {
	db, mock, closeDB := newProfileMockGORM(t)
	defer closeDB()
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT bit_main_user_id, bit_account_bound_at FROM users WHERE id = ? FOR UPDATE")).
		WithArgs(sharedidentity.UserID(1)).
		WillReturnRows(sqlmock.NewRows([]string{"bit_main_user_id", "bit_account_bound_at"}).AddRow("main-user-a", now))
	mock.ExpectExec("UPDATE users SET bit_main_user_id = ?").
		WithArgs("main-user-b", model.BitAccountBound, now, now, now, sharedidentity.UserID(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO audit_logs").
		WithArgs(sqlmock.AnyArg(), sharedidentity.UserID(1), model.AuditMainAccountRebind, "user", sharedidentity.UserID(1), sqlmock.AnyArg(), now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	binding, err := bindBitAccount(db, 1, "main-user-b", true, now)
	if err != nil || binding.BitMainUserID != "main-user-b" {
		t.Fatalf("bindBitAccount() binding=%#v error = %v", binding, err)
	}
}

func TestApplyScanMarksMissingOnlyInsideTheCurrentMainScope(t *testing.T) {
	db, mock, closeDB := newProfileMockGORM(t)
	defer closeDB()
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	scan := model.ProfileScan{
		ID: "scan-1", UserID: 1, BitMainUserID: "main-user-a", Status: model.ScanConfirmed,
		Profiles: []model.BrowserProfile{{
			ID: "bp-1", UserID: 1, BitProfileID: "bit-1", BitMainUserID: "main-user-a",
			BitUserID: "bit-user-1", BitName: "窗口一", LocalState: model.ProfileActive,
		}},
	}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT bit_main_user_id FROM users WHERE id = ? FOR UPDATE")).
		WithArgs(sharedidentity.UserID(1)).
		WillReturnRows(sqlmock.NewRows([]string{"bit_main_user_id"}).AddRow("main-user-a"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT snapshot FROM browser_profile_syncs WHERE id = ? AND user_id = ? FOR UPDATE")).
		WithArgs("scan-1", sharedidentity.UserID(1)).
		WillReturnRows(sqlmock.NewRows([]string{"snapshot"}).AddRow(`{"profiles":[],"diff":[]}`))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, reserved_proxy_id FROM browser_profiles WHERE user_id = ? AND bit_main_user_id = ? FOR UPDATE")).WithArgs(sharedidentity.UserID(1), "main-user-a").WillReturnRows(sqlmock.NewRows([]string{"id", "reserved_proxy_id"}))
	mock.ExpectExec("INSERT INTO browser_profiles").
		WithArgs(sharedidentity.UserID(1), nil, "bit-1", "main-user-a", "bit-user-1", "窗口一", 0,
			nil, nil, nil, nil, nil, nil, 0, nil, model.ProfileActive, now, time.Time{}, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE browser_profiles SET local_state = ?").
		WithArgs(model.ProfileLocalMissing, now, now, sharedidentity.UserID(1), "main-user-a", "bit-1", model.ProfileArchived, model.ProfileLocalMissing).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE browser_profile_syncs SET snapshot = ? WHERE id = ? AND user_id = ?")).
		WithArgs(sqlmock.AnyArg(), "scan-1", sharedidentity.UserID(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO audit_logs").
		WithArgs(sqlmock.AnyArg(), sharedidentity.UserID(1), "bitbrowser.profile_sync.confirm", "browser_profile_sync", "scan-1", sqlmock.AnyArg(), now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := applyScan(db, scan, []string{"bit-1"}, now); err != nil {
		t.Fatalf("applyScan() error = %v", err)
	}
}

func TestApplyScanEmptyBitSnapshotMarksAllCurrentOwnerWindowsMissing(t *testing.T) {
	db, mock, closeDB := newProfileMockGORM(t)
	defer closeDB()
	now := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
	scan := model.ProfileScan{ID: "empty-scan", UserID: 1, BitMainUserID: "main-user-a"}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT bit_main_user_id FROM users WHERE id = ? FOR UPDATE")).
		WithArgs(sharedidentity.UserID(1)).
		WillReturnRows(sqlmock.NewRows([]string{"bit_main_user_id"}).AddRow("main-user-a"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT snapshot FROM browser_profile_syncs WHERE id = ? AND user_id = ? FOR UPDATE")).
		WithArgs("empty-scan", sharedidentity.UserID(1)).
		WillReturnRows(sqlmock.NewRows([]string{"snapshot"}).AddRow(`{"profiles":[],"diff":[]}`))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, reserved_proxy_id FROM browser_profiles WHERE user_id = ? AND bit_main_user_id = ? FOR UPDATE")).
		WithArgs(sharedidentity.UserID(1), "main-user-a").
		WillReturnRows(sqlmock.NewRows([]string{"id", "reserved_proxy_id"}))
	mock.ExpectExec("UPDATE browser_profiles SET local_state = ?").
		WithArgs(model.ProfileLocalMissing, now, now, sharedidentity.UserID(1), "main-user-a", model.ProfileArchived, model.ProfileLocalMissing).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE browser_profile_syncs SET snapshot = ? WHERE id = ? AND user_id = ?")).
		WithArgs(sqlmock.AnyArg(), "empty-scan", sharedidentity.UserID(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO audit_logs").
		WithArgs(sqlmock.AnyArg(), sharedidentity.UserID(1), "bitbrowser.profile_sync.confirm", "browser_profile_sync", "empty-scan", sqlmock.AnyArg(), now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := applyScan(db, scan, nil, now); err != nil {
		t.Fatalf("applyScan() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyScanRejectsScanFromPreviousMainAccount(t *testing.T) {
	db, mock, closeDB := newProfileMockGORM(t)
	defer closeDB()
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT bit_main_user_id FROM users WHERE id = ? FOR UPDATE")).
		WithArgs(sharedidentity.UserID(1)).
		WillReturnRows(sqlmock.NewRows([]string{"bit_main_user_id"}).AddRow("main-user-b"))
	mock.ExpectRollback()

	err := applyScan(db, model.ProfileScan{ID: "scan-1", UserID: 1, BitMainUserID: "main-user-a"}, nil, now)
	if err != model.ErrBitMainUserMismatch {
		t.Fatalf("applyScan() error = %v, want ErrBitMainUserMismatch", err)
	}
}

func TestGetProfileInScopeJoinsTheOwnerBinding(t *testing.T) {
	db, mock, closeDB := newProfileMockGORM(t)
	defer closeDB()
	mock.ExpectQuery(`SELECT bp\.id, .* FROM browser_profiles bp JOIN users u ON u\.id = bp\.user_id WHERE bp\.id = \? AND bp\.bit_main_user_id = u\.bit_main_user_id`).
		WithArgs("bp-1").
		WillReturnRows(sqlmock.NewRows(strings.Split(profileColumnsSQL, ", ")))
	if _, found, err := getProfileInScope(db, "bp-1"); err != nil || found {
		t.Fatalf("getProfileInScope() found=%v err=%v", found, err)
	}
}

func TestClearObservedProxyOnlyClearsUnmanagedWindows(t *testing.T) {
	db, mock, closeDB := newProfileMockGORM(t)
	defer closeDB()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE browser_profiles SET bit_proxy_type = NULL, bit_proxy_host = NULL, bit_proxy_port = 0, updated_at = ? WHERE id = ? AND proxy_id IS NULL")).
		WithArgs(sqlmock.AnyArg(), "bp-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT " + profileColumnsSQL + " FROM browser_profiles WHERE id = ?")).
		WithArgs("bp-1").
		WillReturnRows(sqlmock.NewRows(strings.Split(profileColumnsSQL, ", ")))

	_, err := clearObservedProxy(db, "bp-1")
	if !errors.Is(err, model.ErrProfileNotFound) {
		t.Fatalf("clearObservedProxy() error = %v, want ErrProfileNotFound for a vanished row", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestClearObservedProxyLeavesAssignmentsAlone(t *testing.T) {
	db, mock, closeDB := newProfileMockGORM(t)
	defer closeDB()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE browser_profiles SET bit_proxy_type = NULL, bit_proxy_host = NULL, bit_proxy_port = 0, updated_at = ? WHERE id = ? AND proxy_id IS NULL")).
		WithArgs(sqlmock.AnyArg(), "bp-1").
		WillReturnResult(sqlmock.NewResult(0, 0))

	_, err := clearObservedProxy(db, "bp-1")
	if !errors.Is(err, model.ErrProfileNotFound) {
		t.Fatalf("clearObservedProxy() error = %v, want ErrProfileNotFound when the managed relation already exists", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func newProfileMockGORM(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	db, err := gorm.Open(mysqlgorm.New(mysqlgorm.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		_ = sqlDB.Close()
		t.Fatalf("gorm.Open() error = %v", err)
	}
	return db, mock, func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("SQL expectations: %v", err)
		}
		_ = sqlDB.Close()
	}
}

func readFileAsString(path string) (string, error) {
	content, err := osReadFile(path)
	return string(content), err
}

func containsString(value, part string) bool {
	for i := 0; i+len(part) <= len(value); i++ {
		if value[i:i+len(part)] == part {
			return true
		}
	}
	return false
}

func osReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

func boolPointer(value bool) *bool { return &value }

func TestApplyScanRejectsReplacedOrConfirmedSnapshotBeforeMirrorWrite(t *testing.T) {
	for _, confirmed := range []bool{false, true} {
		t.Run(fmt.Sprintf("confirmed=%v", confirmed), func(t *testing.T) {
			db, mock, closeDB := newProfileMockGORM(t)
			defer closeDB()
			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta("SELECT bit_main_user_id FROM users WHERE id = ? FOR UPDATE")).WithArgs(sharedidentity.UserID(1)).
				WillReturnRows(sqlmock.NewRows([]string{"bit_main_user_id"}).AddRow("main-1"))
			expect := mock.ExpectQuery(regexp.QuoteMeta("SELECT snapshot FROM browser_profile_syncs WHERE id = ? AND user_id = ? FOR UPDATE")).WithArgs("s1", sharedidentity.UserID(1))
			rows := sqlmock.NewRows([]string{"snapshot"})
			if confirmed {
				rows.AddRow(`{"status":"confirmed"}`)
			}
			expect.WillReturnRows(rows)
			mock.ExpectRollback()
			err := applyScan(db, model.ProfileScan{ID: "s1", UserID: 1, BitMainUserID: "main-1", Profiles: []model.BrowserProfile{{BitProfileID: "p1"}}}, nil, time.Now())
			if !errors.Is(err, model.ErrScanNotReady) {
				t.Fatalf("expected stale scan, got %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFindScanReadsPersistedConfirmedReceipt(t *testing.T) {
	db, mock, closeDB := newProfileMockGORM(t)
	defer closeDB()
	now := time.Now().UTC().Truncate(time.Second)
	payload := fmt.Sprintf(`{"status":"confirmed","confirmed_at":%q,"profiles":[{"bit_profile_id":"p1"}],"diff":[]}`, now.Format(time.RFC3339))
	mock.ExpectQuery("SELECT id, user_id, bit_main_user_id, snapshot").WithArgs("s1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "bit_main_user_id", "snapshot", "observed_at", "expires_at", "created_at"}).AddRow("s1", 1, "main-1", []byte(payload), now, now.Add(time.Hour), now))
	receipt, found, err := findScan(db, "s1")
	if err != nil || !found || receipt.Status != model.ScanConfirmed || receipt.ConfirmedAt == nil || len(receipt.Profiles) != 1 {
		t.Fatalf("invalid receipt: %#v %v", receipt, err)
	}
}
