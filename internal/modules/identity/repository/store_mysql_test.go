package repository

import (
	"errors"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	mysqlgorm "gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/wt-media/wt-media-cloud/internal/modules/identity/model"
)

func TestIdentityRepositoryExposesPackageFunctionsUsingGORM(t *testing.T) {
	var _ func() (int, error) = CountUsers
	var _ func(model.User) (model.UserID, error) = CreateUser
	var _ func(model.UserID) (model.User, bool, error) = FindUser
	var _ func(string) (model.User, bool, error) = FindUserByUsername
	var _ func() ([]model.User, error) = ListUsers

	var _ func(*gorm.DB) (int, error) = countUsers
	var _ func(*gorm.DB, model.User) (model.UserID, error) = createUser
	var _ func(*gorm.DB, string) (model.User, bool, error) = findUserByUsername
	var _ func(*gorm.DB) ([]model.User, error) = listUsers
}

func TestIdentityRepositoryUsesExplicitSQLThroughGORM(t *testing.T) {
	// These compile-time signatures keep the repository boundary tied to the
	// controlled *gorm.DB resource while private tests can inject sqlmock.
	var _ func(*gorm.DB, model.User, model.AuditEvent) (model.UserID, error) = createUserWithAudit
	var _ func(*gorm.DB, model.User, model.AuditEvent, string) error = updateUserAndInvalidateSessions
	var _ func(*gorm.DB, model.Session, string) (model.SessionCreateResult, error) = createSession
}

func TestFindUserByUsernameUsesGORMAndMapsStoredUser(t *testing.T) {
	db, mock, closeDB := newIdentityMockGORM(t)
	defer closeDB()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT u.id, u.username, COALESCE(u.nickname, u.username), COALESCE(u.avatar_id, 'sky'), u.password_hash")).
		WithArgs("admin").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "username", "nickname", "avatar_id", "password_hash", "role", "status", "team_id", "team_name", "created_at", "updated_at",
		}).AddRow(int64(1), "admin", "Admin", "sky", "hash", "admin", "enabled", nil, "", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT category_id FROM user_category_scopes")).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"category_id"}).AddRow(1))

	user, found, err := findUserByUsername(db, "admin")
	if err != nil || !found {
		t.Fatalf("findUserByUsername() = %v, %v, want found without error", found, err)
	}
	if user.ID != 1 || user.Username != "admin" || user.Nickname != "Admin" || user.AvatarID != "sky" || len(user.CategoryIDs) != 1 || user.CategoryIDs[0] != model.CategoryID(1) {
		t.Fatalf("user = %#v", user)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestCreateWebSessionWritesExpiryWithoutReplacement(t *testing.T) {
	db, mock, closeDB := newIdentityMockGORM(t)
	defer closeDB()
	created := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	session := model.Session{
		ID: "session-web", UserID: 1, TokenHash: "hash", ClientType: model.ClientTypeWeb,
		CreatedAt: created, ExpiresAt: created.Add(30 * 24 * time.Hour),
	}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO user_sessions")).
		WithArgs(session.ID, session.UserID, session.TokenHash, session.ClientType, session.CreatedAt, session.ExpiresAt, nil, "").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	result, err := createSession(db, session, "")
	if err != nil {
		t.Fatalf("createSession() error = %v", err)
	}
	if result.ReplacedSessionID != "" {
		t.Fatalf("web session replaced %q", result.ReplacedSessionID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestCreateDesktopSessionReplacementLocksUserAndMarksOldRow(t *testing.T) {
	db, mock, closeDB := newIdentityMockGORM(t)
	defer closeDB()
	created := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	expires := created.Add(30 * 24 * time.Hour)
	session := model.Session{
		ID: "session-new", UserID: 1, TokenHash: "hash-new", ClientType: model.ClientTypeDesktop,
		CreatedAt: created, ExpiresAt: expires,
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM users WHERE id = ? FOR UPDATE")).
		WithArgs(model.UserID(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, user_id, token_hash, client_type, created_at, expires_at, invalidated_at, invalidated_reason")).
		WithArgs(model.UserID(1), model.ClientTypeDesktop, created).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "token_hash", "client_type", "created_at", "expires_at", "invalidated_at", "invalidated_reason",
		}).AddRow("session-old", int64(1), "hash-old", "desktop", created.Add(-time.Hour), expires, nil, nil))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE user_sessions SET invalidated_at = ?, invalidated_reason = ? WHERE id = ? AND invalidated_at IS NULL")).
		WithArgs(created, model.SessionInvalidReasonDesktopReplaced, "session-old").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO user_sessions")).
		WithArgs(session.ID, session.UserID, session.TokenHash, session.ClientType, session.CreatedAt, session.ExpiresAt, nil, "").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	result, err := createSession(db, session, "session-old")
	if err != nil {
		t.Fatalf("createSession() error = %v", err)
	}
	if result.ReplacedSessionID != "session-old" {
		t.Fatalf("replaced session = %q", result.ReplacedSessionID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestCreateDesktopSessionConflictWritesNothing(t *testing.T) {
	db, mock, closeDB := newIdentityMockGORM(t)
	defer closeDB()
	created := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	session := model.Session{
		ID: "session-new", UserID: 1, TokenHash: "hash-new", ClientType: model.ClientTypeDesktop,
		CreatedAt: created, ExpiresAt: created.Add(30 * 24 * time.Hour),
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM users WHERE id = ? FOR UPDATE")).
		WithArgs(model.UserID(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, user_id, token_hash, client_type, created_at, expires_at, invalidated_at, invalidated_reason")).
		WithArgs(model.UserID(1), model.ClientTypeDesktop, created).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "token_hash", "client_type", "created_at", "expires_at", "invalidated_at", "invalidated_reason",
		}).AddRow("session-active", int64(1), "hash-old", "desktop", created.Add(-time.Hour), created.Add(time.Hour), nil, nil))
	mock.ExpectRollback()

	_, err := createSession(db, session, "stale-session")
	var conflict *model.DesktopLoginConflictError
	if !errors.As(err, &conflict) || conflict.ActiveSessionID != "session-active" {
		t.Fatalf("createSession() error = %v, want conflict with session-active", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestFindSessionByTokenHashMapsExpiryAndInvalidationReason(t *testing.T) {
	db, mock, closeDB := newIdentityMockGORM(t)
	defer closeDB()
	created := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	invalidated := created.Add(time.Hour)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, user_id, token_hash, client_type, created_at, expires_at, invalidated_at, invalidated_reason")).
		WithArgs("hash").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "token_hash", "client_type", "created_at", "expires_at", "invalidated_at", "invalidated_reason",
		}).AddRow("session", int64(1), "hash", "desktop", created, created.Add(30*24*time.Hour), invalidated, "desktop_replaced"))

	session, found, err := findSessionByTokenHash(db, "hash")
	if err != nil || !found {
		t.Fatalf("findSessionByTokenHash() = %v, %v", found, err)
	}
	if session.ExpiresAt != created.Add(30*24*time.Hour) || session.InvalidatedReason != model.SessionInvalidReasonDesktopReplaced {
		t.Fatalf("session = %#v", session)
	}
	if session.InvalidAt == nil || !session.InvalidAt.Equal(invalidated) {
		t.Fatalf("invalidated_at = %v", session.InvalidAt)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestInvalidateUserSessionsWritesReason(t *testing.T) {
	db, mock, closeDB := newIdentityMockGORM(t)
	defer closeDB()
	at := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

	mock.ExpectExec(regexp.QuoteMeta("UPDATE user_sessions SET invalidated_at = ?, invalidated_reason = ? WHERE user_id = ? AND invalidated_at IS NULL")).
		WithArgs(at, model.SessionInvalidReasonForceLogout, model.UserID(7)).
		WillReturnResult(sqlmock.NewResult(0, 2))

	if err := invalidateUserSessions(db, 7, at, model.SessionInvalidReasonForceLogout); err != nil {
		t.Fatalf("invalidateUserSessions() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func newIdentityMockGORM(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	db, err := gorm.Open(mysqlgorm.New(mysqlgorm.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		_ = sqlDB.Close()
		t.Fatalf("gorm.Open() error = %v", err)
	}
	return db, mock, func() { _ = sqlDB.Close() }
}
