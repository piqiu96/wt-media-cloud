package mediaaccount

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
)

func TestMySQLStoreCreatesAccountWithSecretFieldsInternal(t *testing.T) {
	store, mock, closeDB := newMockMySQLStore(t)
	defer closeDB()
	record := mysqlTestRecord()

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO media_accounts`)).
		WithArgs(
			record.UserID, record.TeamID, record.GameID, record.Platform, nil, nil, nil, nil,
			nil, record.IdentificationStatus, nil, record.BusinessStatus, record.LoginStatus,
			record.OriginalCookie, nil, nil, nil, nil, nil, record.CreatedAt, record.UpdatedAt,
		).
		WillReturnResult(sqlmock.NewResult(42, 1))

	id, err := store.Create(record)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if id != "42" {
		t.Fatalf("Create() id = %q, want 42", id)
	}
}

func TestMySQLStoreFindsAccountIdentity(t *testing.T) {
	store, mock, closeDB := newMockMySQLStore(t)
	defer closeDB()
	record := mysqlTestRecord()
	record.PlatformAccountID = "platform-42"
	record.IdentificationStatus = IdentificationIdentified

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT `+accountColumns+` FROM media_accounts WHERE user_id = ? AND platform = ? AND platform_account_id = ?`)).
		WithArgs(record.UserID, record.Platform, record.PlatformAccountID).
		WillReturnRows(accountRows(record))

	got, found, err := store.FindByIdentity(record.UserID, record.Platform, record.PlatformAccountID)
	if err != nil || !found {
		t.Fatalf("FindByIdentity() found = %v, error = %v", found, err)
	}
	if got.PlatformAccountID != record.PlatformAccountID || got.OriginalCookie != record.OriginalCookie {
		t.Fatalf("record = %#v", got)
	}
}

func TestMySQLStoreReturnsNotFound(t *testing.T) {
	store, mock, closeDB := newMockMySQLStore(t)
	defer closeDB()
	mock.ExpectQuery("FROM media_accounts WHERE id =").WithArgs("missing").WillReturnError(sql.ErrNoRows)

	_, found, err := store.Find("missing")
	if err != nil || found {
		t.Fatalf("Find() found = %v, error = %v", found, err)
	}
}

func TestMySQLStoreMapsIdentityDuplicateOnUpdate(t *testing.T) {
	store, mock, closeDB := newMockMySQLStore(t)
	defer closeDB()
	record := mysqlTestRecord()
	record.PlatformAccountID = "platform-42"

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE media_accounts SET`)).
		WillReturnError(&mysqlDriver.MySQLError{Number: 1062, Message: "duplicate"})

	err := store.Update(record)
	if !errors.Is(err, ErrDuplicateAccount) {
		t.Fatalf("Update() error = %v, want ErrDuplicateAccount", err)
	}
}

func TestMySQLStoreListsScopedAccountsAndAppliesTagFilters(t *testing.T) {
	store, mock, closeDB := newMockMySQLStore(t)
	defer closeDB()
	first := mysqlTestRecord()
	second := mysqlTestRecord()
	second.ID = "media_account-2"
	second.Platform = PlatformBilibili

	mock.ExpectQuery(regexp.QuoteMeta(`FROM media_accounts WHERE user_id = ? AND game_id = ? ORDER BY created_at, id`)).
		WithArgs(identity.UserID(1), "game-a").
		WillReturnRows(accountRows(first).AddRow(accountRowValues(second)...))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT media_account_id, tag_name FROM media_account_tags WHERE media_account_id IN (?, ?) ORDER BY media_account_id, tag_name`)).
		WithArgs(first.ID, second.ID).
		WillReturnRows(sqlmock.NewRows([]string{"media_account_id", "tag_name"}).
			AddRow(first.ID, "launch").
			AddRow(first.ID, "vip").
			AddRow(second.ID, "launch"))

	got, err := store.List(AccountQuery{UserID: identity.UserID(1), GameID: "game-a", AllTags: []string{"launch", "vip"}})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != first.ID {
		t.Fatalf("List() = %#v", got)
	}
}

func TestMySQLStoreAddsAndRemovesTagsTransactionally(t *testing.T) {
	store, mock, closeDB := newMockMySQLStore(t)
	defer closeDB()
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`INSERT IGNORE INTO media_account_tags`)).
		WithArgs(sqlmock.AnyArg(), identity.UserID(1), "account-1", "launch", now).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT IGNORE INTO media_account_tags`)).
		WithArgs(sqlmock.AnyArg(), identity.UserID(1), "account-1", "vip", now).
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()
	if err := store.AddTags(identity.UserID(1), []string{"account-1"}, []string{"launch", "vip"}, now); err != nil {
		t.Fatalf("AddTags() error = %v", err)
	}

	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM media_account_tags WHERE user_id = ? AND media_account_id IN (?) AND tag_name IN (?)`)).
		WithArgs(identity.UserID(1), "account-1", "vip").
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.RemoveTags(identity.UserID(1), []string{"account-1"}, []string{"vip"}); err != nil {
		t.Fatalf("RemoveTags() error = %v", err)
	}
}

func TestMySQLStoreAppendsSecretFreeAudit(t *testing.T) {
	store, mock, closeDB := newMockMySQLStore(t)
	defer closeDB()
	event := identity.AuditEvent{
		ID: "audit-1", ActorUserID: identity.UserID(1), Action: "media_account.create", TargetType: "media_account",
		TargetID: "account-1", Summary: map[string]string{"platform": "douyin"}, CreatedAt: time.Now().UTC(),
	}
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO audit_logs`)).
		WithArgs(event.ID, event.ActorUserID, event.Action, event.TargetType, event.TargetID, []byte(`{"platform":"douyin"}`), event.CreatedAt).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := store.AppendAudit(event); err != nil {
		t.Fatalf("AppendAudit() error = %v", err)
	}
}

func TestMySQLStoreListsAccountsFilteredByProfileSearch(t *testing.T) {
	store, mock, closeDB := newMockMySQLStore(t)
	defer closeDB()
	first := mysqlTestRecord()
	first.BrowserProfileID = "profile-9"

	mock.ExpectQuery(regexp.QuoteMeta(`FROM media_accounts WHERE user_id = ? AND game_id = ? AND EXISTS (SELECT 1 FROM browser_profiles bp WHERE bp.id = media_accounts.browser_profile_id AND (bp.name LIKE ? OR bp.seq LIKE ? OR bp.bit_profile_id LIKE ? OR bp.id LIKE ?)) ORDER BY created_at, id`)).
		WithArgs(identity.UserID(1), "game-a", "%运营%", "%运营%", "%运营%", "%运营%").
		WillReturnRows(accountRows(first))

	got, err := store.List(AccountQuery{UserID: identity.UserID(1), GameID: "game-a", ProfileSearch: "运营"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != first.ID {
		t.Fatalf("List() = %#v", got)
	}
}

func TestMediaAccountMigrationDefinesOwnershipAndUniqueness(t *testing.T) {
	content, err := os.ReadFile("../../../migrations/20260714_002_media_accounts.sql")
	if err != nil {
		t.Fatal(err)
	}
	sqlText := string(content)
	for _, required := range []string{
		"FOREIGN KEY (user_id) REFERENCES users (id)",
		"UNIQUE KEY uq_media_accounts_user_platform_identity (user_id, platform, platform_account_id)",
		"UNIQUE KEY uq_media_accounts_profile_platform (browser_profile_id, platform)",
		"UNIQUE KEY uq_media_account_tags_owner_account_name (user_id, media_account_id, tag_name)",
		"original_cookie LONGTEXT",
		"active_cookie LONGTEXT",
	} {
		if !strings.Contains(sqlText, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
}

func newMockMySQLStore(t *testing.T) (*MySQLStore, sqlmock.Sqlmock, func()) {
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

func mysqlTestRecord() AccountRecord {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	return AccountRecord{
		Account: Account{
			ID: "media_account-1", UserID: identity.UserID(1), TeamID: teamIDPointer(10), GameID: "game-a", Platform: PlatformDouyin,
			IdentificationStatus: IdentificationPending, BusinessStatus: BusinessEnabled, LoginStatus: LoginUnknown,
			CreatedAt: now, UpdatedAt: now,
		},
		OriginalCookie: "secret-cookie",
	}
}

func accountRows(records ...AccountRecord) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{
		"id", "user_id", "team_id", "game_id", "platform", "platform_account_id", "name", "avatar_url", "browser_profile_id",
		"remark", "identification_status", "duplicate_of_account_id", "business_status", "login_status", "original_cookie", "active_cookie",
		"cookie_status", "active_cookie_updated_at", "last_checked_at", "check_items", "created_at", "updated_at",
	})
	for _, record := range records {
		rows.AddRow(accountRowValues(record)...)
	}
	return rows
}

func accountRowValues(record AccountRecord) []driver.Value {
	return []driver.Value{
		record.ID, record.UserID, record.TeamID, record.GameID, record.Platform, nullableString(record.PlatformAccountID), nullableString(record.Name), nullableString(record.AvatarURL), nullableString(record.BrowserProfileID),
		nullableString(record.Remark), record.IdentificationStatus, nullableString(record.DuplicateOfAccountID), record.BusinessStatus, record.LoginStatus, nullableString(record.OriginalCookie), nullableString(record.ActiveCookie),
		nullableString(record.CookieStatus), record.ActiveCookieUpdatedAt, record.LastCheckedAt, checkItemsValue(record.CheckItems), record.CreatedAt, record.UpdatedAt,
	}
}

func checkItemsValue(items []AccountCheckItem) driver.Value {
	if len(items) == 0 {
		return nil
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return nil
	}
	return string(raw)
}

func teamIDPointer(value identity.TeamID) *identity.TeamID {
	return &value
}

func nullableString(value string) driver.Value {
	if value == "" {
		return nil
	}
	return value
}
