package identity

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	mysqlDriver "github.com/go-sql-driver/mysql"
)

func TestMySQLStoreCreateUserPersistsGameScopesAtomically(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 7, 14, 13, 0, 0, 0, time.UTC)
	user := User{
		Username:     "operator",
		PasswordHash: "bcrypt-hash",
		Role:         RoleOperator,
		Status:       UserStatusEnabled,
		GameIDs:      []string{"game-a", "game-b"},
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO users (legacy_id, username, password_hash, role, status, team_id, created_at, updated_at) VALUES (NULL, ?, ?, ?, ?, ?, ?, ?)`)).
		WithArgs(user.Username, user.PasswordHash, user.Role, user.Status, user.TeamID, now, now).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO user_game_scopes (user_id, game_id, created_at) VALUES (?, ?, ?)`)).
		WithArgs(UserID(1), "game-a", now).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO user_game_scopes (user_id, game_id, created_at) VALUES (?, ?, ?)`)).
		WithArgs(UserID(1), "game-b", now).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if _, err := NewMySQLStore(db).CreateUser(user); err != nil {
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
		WithArgs(now, UserID(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := NewMySQLStore(db).InvalidateUserSessions(UserID(1), now); err != nil {
		t.Fatalf("InvalidateUserSessions() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestMySQLStoreDetectsActiveSession(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM user_sessions WHERE user_id = ? AND invalidated_at IS NULL`)).
		WithArgs(UserID(1)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	active, err := NewMySQLStore(db).HasActiveSession(UserID(1))
	if err != nil {
		t.Fatalf("HasActiveSession() error = %v", err)
	}
	if !active {
		t.Fatal("HasActiveSession() = false, want true")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestMySQLStoreTeamHasReferencesIncludesHistoricalProfileScans(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	teamID := TeamID(17)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT
        (SELECT COUNT(*) FROM users WHERE team_id = ?) +
        (SELECT COUNT(*) FROM media_accounts WHERE team_id = ?) +
        (SELECT COUNT(*) FROM browser_profiles WHERE team_id = ?) +
        (SELECT COUNT(*) FROM profile_sync_scans WHERE team_id = ?)`)).
		WithArgs(teamID, teamID, teamID, teamID).
		WillReturnRows(sqlmock.NewRows([]string{"references"}).AddRow(1))

	referenced, err := NewMySQLStore(db).TeamHasReferences(teamID)
	if err != nil {
		t.Fatalf("TeamHasReferences() error = %v", err)
	}
	if !referenced {
		t.Fatal("TeamHasReferences() = false, want true for historical profile scan")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestMySQLStoreRollsBackUserAndSessionsWhenAuditFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 7, 22, 15, 0, 0, 0, time.UTC)
	teamID := TeamID(8)
	user := User{ID: 2, Username: "operator", PasswordHash: "hash", Role: RoleSeniorOperator, Status: UserStatusEnabled, TeamID: &teamID, GameIDs: []string{"game-a"}, UpdatedAt: now}
	event := AuditEvent{ID: "audit-1", ActorUserID: 1, Action: "user.access.update", TargetType: "user", TargetID: "2", Summary: map[string]string{"team_id": "8", "game_ids": "game-a"}, CreatedAt: now}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE users SET username = ?, password_hash = ?, role = ?, status = ?, team_id = ?, updated_at = ? WHERE id = ?`)).
		WithArgs(user.Username, user.PasswordHash, user.Role, user.Status, user.TeamID, user.UpdatedAt, user.ID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM user_game_scopes WHERE user_id = ?`)).
		WithArgs(user.ID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO user_game_scopes (user_id, game_id, created_at) VALUES (?, ?, ?)`)).
		WithArgs(user.ID, "game-a", user.UpdatedAt).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE user_sessions SET invalidated_at = ? WHERE user_id = ? AND invalidated_at IS NULL`)).
		WithArgs(user.UpdatedAt, user.ID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO audit_logs`)).
		WillReturnError(errors.New("audit unavailable"))
	mock.ExpectRollback()

	if err := NewMySQLStore(db).UpdateUserAndInvalidateSessions(user, event); err == nil {
		t.Fatal("UpdateUserAndInvalidateSessions() error = nil")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestMySQLStoreRollsBackCreatedUserWhenAuditFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()
	now := time.Date(2026, 7, 22, 16, 0, 0, 0, time.UTC)
	user := User{Username: "operator", PasswordHash: "hash", Role: RoleOperator, Status: UserStatusEnabled, GameIDs: []string{"game-a"}, CreatedAt: now, UpdatedAt: now}
	event := AuditEvent{ID: "audit-1", ActorUserID: 1, Action: "user.create", TargetType: "user", Summary: map[string]string{"role": "operator"}, CreatedAt: now}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO users (legacy_id, username, password_hash, role, status, team_id, created_at, updated_at) VALUES (NULL, ?, ?, ?, ?, ?, ?, ?)`)).
		WithArgs(user.Username, user.PasswordHash, user.Role, user.Status, user.TeamID, now, now).
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO user_game_scopes (user_id, game_id, created_at) VALUES (?, ?, ?)`)).
		WithArgs(UserID(2), "game-a", now).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO audit_logs`)).WillReturnError(errors.New("audit unavailable"))
	mock.ExpectRollback()

	if _, err := NewMySQLStore(db).CreateUserWithAudit(user, event); err == nil {
		t.Fatal("CreateUserWithAudit() error = nil")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestMySQLStoreMapsTeamForeignKeyReferenceToTeamInUse(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM operation_teams WHERE id = ?`)).
		WithArgs(TeamID(8)).
		WillReturnError(&mysqlDriver.MySQLError{Number: 1451, Message: "referenced"})

	if err := NewMySQLStore(db).DeleteTeam(TeamID(8)); !errors.Is(err, ErrTeamInUse) {
		t.Fatalf("DeleteTeam() error=%v, want ErrTeamInUse", err)
	}
}
