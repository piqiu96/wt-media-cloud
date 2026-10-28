package repository

import (
	"errors"
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
	mock.ExpectExec("INSERT INTO browser_profiles").
		WithArgs(sharedidentity.UserID(1), nil, "bit-1", "main-user-a", "bit-user-1", "窗口一", 0,
			nil, nil, nil, nil, nil, nil, 0, nil, model.ProfileActive, now, time.Time{}, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE browser_profiles SET local_state = ?").
		WithArgs(model.ProfileLocalMissing, now, now, sharedidentity.UserID(1), "main-user-a", "bit-1", model.ProfileArchived).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM browser_profile_syncs WHERE id = ? AND user_id = ?")).
		WithArgs("scan-1", sharedidentity.UserID(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO audit_logs").
		WithArgs(sqlmock.AnyArg(), sharedidentity.UserID(1), "bitbrowser.profile_sync.confirm", "browser_profile_sync", "scan-1", sqlmock.AnyArg(), now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := applyScan(db, scan, []string{"bit-1"}, now); err != nil {
		t.Fatalf("applyScan() error = %v", err)
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
	mock.ExpectExec(regexp.QuoteMeta("UPDATE browser_profiles SET bit_proxy_type = NULL, bit_proxy_host = NULL, bit_proxy_port = 0, updated_at = ? WHERE id = ? AND COALESCE(proxy_id, '') = ''")).
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
	mock.ExpectExec(regexp.QuoteMeta("UPDATE browser_profiles SET bit_proxy_type = NULL, bit_proxy_host = NULL, bit_proxy_port = 0, updated_at = ? WHERE id = ? AND COALESCE(proxy_id, '') = ''")).
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
