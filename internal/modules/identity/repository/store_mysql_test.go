package repository

import (
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
	var _ func(*gorm.DB, model.User, model.AuditEvent) error = updateUserAndInvalidateSessions
	var _ func(*gorm.DB, model.Session) error = createSession
}

func TestFindUserByUsernameUsesGORMAndMapsStoredUser(t *testing.T) {
	db, mock, closeDB := newIdentityMockGORM(t)
	defer closeDB()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT u.id, u.username, COALESCE(u.nickname, u.username), COALESCE(u.avatar_id, 'sky'), u.password_hash")).
		WithArgs("admin").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "username", "nickname", "avatar_id", "password_hash", "role", "status", "team_id", "team_name", "created_at", "updated_at",
		}).AddRow(int64(1), "admin", "Admin", "sky", "hash", "admin", "enabled", nil, "", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT game_id FROM user_game_scopes")).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"game_id"}).AddRow("game-a"))

	user, found, err := findUserByUsername(db, "admin")
	if err != nil || !found {
		t.Fatalf("findUserByUsername() = %v, %v, want found without error", found, err)
	}
	if user.ID != 1 || user.Username != "admin" || user.Nickname != "Admin" || user.AvatarID != "sky" || len(user.GameIDs) != 1 || user.GameIDs[0] != "game-a" {
		t.Fatalf("user = %#v", user)
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
