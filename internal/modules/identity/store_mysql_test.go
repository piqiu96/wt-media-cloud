package identity

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMySQLStoreCreateUserPersistsGameScopesAtomically(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 7, 14, 13, 0, 0, 0, time.UTC)
	user := User{
		ID:           "user-1",
		Username:     "operator",
		PasswordHash: "bcrypt-hash",
		Role:         RoleOperator,
		Status:       UserStatusEnabled,
		GameIDs:      []string{"game-a", "game-b"},
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO users (id, username, password_hash, role, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`)).
		WithArgs(user.ID, user.Username, user.PasswordHash, user.Role, user.Status, now, now).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO user_game_scopes (user_id, game_id, created_at) VALUES (?, ?, ?)`)).
		WithArgs(user.ID, "game-a", now).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO user_game_scopes (user_id, game_id, created_at) VALUES (?, ?, ?)`)).
		WithArgs(user.ID, "game-b", now).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := NewMySQLStore(db).CreateUser(user); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestMySQLStoreInvalidatesPriorSessions(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 7, 14, 13, 5, 0, 0, time.UTC)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE user_sessions SET invalidated_at = ? WHERE user_id = ? AND invalidated_at IS NULL`)).
		WithArgs(now, "user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := NewMySQLStore(db).InvalidateUserSessions("user-1", now); err != nil {
		t.Fatalf("InvalidateUserSessions() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}
