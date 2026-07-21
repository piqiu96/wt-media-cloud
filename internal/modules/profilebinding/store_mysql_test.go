package profilebinding

import (
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMySQLStoreCreatesStagedScanTransactionally(t *testing.T) {
	store, mock, closeDB := newMockStore(t)
	defer closeDB()
	scan := mysqlTestScan()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO profile_sync_scans`)).
		WithArgs(scan.ID, scan.UserID, scan.MainUserID, scan.Status, sqlmock.AnyArg(), scan.CreatedAt, scan.ExpiresAt, nil).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO profile_sync_candidates`)).
		WithArgs(scan.ID, scan.Profiles[0].ID, scan.Profiles[0].BitProfileID, scan.Profiles[0].MainUserID, scan.Profiles[0].ProfileUserID, scan.Profiles[0].Name, scan.Profiles[0].Seq, nil, nil, nil, nil, scan.Profiles[0].CreatedAt, scan.Profiles[0].UpdatedAt).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := store.CreateScan(scan); err != nil {
		t.Fatalf("CreateScan() error = %v", err)
	}
}

func TestMySQLStoreFindsBindingAndProfiles(t *testing.T) {
	store, mock, closeDB := newMockStore(t)
	defer closeDB()
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, bit_main_user_id, bit_account_status, bit_account_bound_at, bit_account_last_verified_at FROM users WHERE id = ?`)).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "bit_main_user_id", "bit_account_status", "bit_account_bound_at", "bit_account_last_verified_at"}).
			AddRow("user-1", "main-user-1", "bound", now, now))

	binding, found, err := store.FindBinding("user-1")
	if err != nil || !found || binding.MainUserID != "main-user-1" {
		t.Fatalf("FindBinding() binding=%#v found=%v error=%v", binding, found, err)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`FROM browser_profiles WHERE user_id = ? ORDER BY bit_profile_id`)).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows(profileColumns()).AddRow(profileValues(mysqlTestScan().Profiles[0])...))
	profiles, err := store.ListProfiles("user-1")
	if err != nil || len(profiles) != 1 || profiles[0].BitProfileID != "bit-profile-1" {
		t.Fatalf("ListProfiles() profiles=%#v error=%v", profiles, err)
	}
}

func TestMySQLStoreAppliesConfirmedScanTransactionally(t *testing.T) {
	store, mock, closeDB := newMockStore(t)
	defer closeDB()
	scan := mysqlTestScan()
	scan.Status = ScanConfirmed
	at := scan.CreatedAt.Add(time.Minute)
	scan.ConfirmedAt = &at
	binding := BitAccountBinding{UserID: scan.UserID, MainUserID: scan.MainUserID, Status: BitAccountBound, BoundAt: &scan.CreatedAt, LastVerifiedAt: &at}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE users SET bit_main_user_id = ?, bit_account_status = ?, bit_account_bound_at = COALESCE(bit_account_bound_at, ?), bit_account_last_verified_at = ?, updated_at = ? WHERE id = ? AND (bit_main_user_id IS NULL OR bit_main_user_id = ?)`)).
		WithArgs(binding.MainUserID, binding.Status, binding.BoundAt, binding.LastVerifiedAt, at, binding.UserID, binding.MainUserID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO browser_profiles`)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE browser_profiles SET local_status = ?, last_synced_at = ?, updated_at = ? WHERE user_id = ? AND bit_profile_id NOT IN (?) AND local_status <> ?`)).
		WithArgs(ProfileLocalMissing, at, at, scan.UserID, scan.Profiles[0].BitProfileID, ProfileArchived).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE profile_sync_scans SET status = ?, confirmed_at = ? WHERE id = ? AND user_id = ? AND status = ?`)).
		WithArgs(ScanConfirmed, at, scan.ID, scan.UserID, ScanReady).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO audit_logs`)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO audit_logs`)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := store.ApplyScan(scan, binding, at, auditMainAccountBind); err != nil {
		t.Fatalf("ApplyScan() error = %v", err)
	}
}

func newMockStore(t *testing.T) (*MySQLStore, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	return NewMySQLStore(db), mock, func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("SQL expectations: %v", err)
		}
		db.Close()
	}
}

func mysqlTestScan() ProfileScan {
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	profile := BrowserProfile{ID: "profile-1", UserID: "user-1", BitProfileID: "bit-profile-1", MainUserID: "main-user-1", ProfileUserID: "bit-user-1", Name: "窗口一", Seq: 1, LocalStatus: ProfileActive, LastSyncedAt: now, CreatedAt: now, UpdatedAt: now}
	return ProfileScan{ID: "scan-1", UserID: "user-1", MainUserID: "main-user-1", Status: ScanReady, Profiles: []BrowserProfile{profile}, Diff: []ProfileDiff{{Kind: DiffAdded, BitProfileID: profile.BitProfileID, ProfileID: profile.ID, Fields: []string{}}}, CreatedAt: now, ExpiresAt: now.Add(15 * time.Minute)}
}

func profileColumns() []string {
	return []string{"id", "user_id", "bit_profile_id", "main_user_id", "profile_user_id", "name", "seq", "group_id", "group_name", "bit_status", "bit_updated_at", "proxy_type", "proxy_host", "proxy_port", "remark", "local_status", "last_synced_at", "created_at", "updated_at"}
}

func profileValues(profile BrowserProfile) []driver.Value {
	return []driver.Value{profile.ID, profile.UserID, profile.BitProfileID, profile.MainUserID, profile.ProfileUserID, profile.Name, profile.Seq, nil, nil, nil, nil, profile.ProxyType, profile.ProxyHost, profile.ProxyPort, profile.Remark, profile.LocalStatus, profile.LastSyncedAt, profile.CreatedAt, profile.UpdatedAt}
}
