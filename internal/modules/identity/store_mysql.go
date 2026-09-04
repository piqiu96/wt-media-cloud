package identity

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

func (s *MySQLStore) CountUsers() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

func (s *MySQLStore) CreateUser(user User) (UserID, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	userID, err := createUserTx(tx, user)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return userID, nil
}

func (s *MySQLStore) CreateUserWithAudit(user User, event AuditEvent) (UserID, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	userID, err := createUserTx(tx, user)
	if err != nil {
		return 0, err
	}
	if event.ActorUserID == 0 {
		event.ActorUserID = userID
	}
	if event.TargetID == "" || event.TargetID == "0" {
		event.TargetID = strconv.FormatInt(int64(userID), 10)
	}
	if err := appendAuditTx(tx, event); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return userID, nil
}

func createUserTx(tx *sql.Tx, user User) (UserID, error) {
	result, err := tx.Exec(
		`INSERT INTO users (legacy_id, username, password_hash, role, status, team_id, created_at, updated_at) VALUES (NULL, ?, ?, ?, ?, ?, ?, ?)`,
		user.Username, user.PasswordHash, user.Role, user.Status, user.TeamID, user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		if duplicateKey(err) {
			return 0, ErrUsernameTaken
		}
		return 0, err
	}
	insertedID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	user.ID = UserID(insertedID)
	for _, gameID := range user.GameIDs {
		if _, err := tx.Exec(
			`INSERT INTO user_game_scopes (user_id, game_id, created_at) VALUES (?, ?, ?)`,
			user.ID, gameID, user.CreatedAt,
		); err != nil {
			return 0, err
		}
	}
	return user.ID, nil
}

func (s *MySQLStore) FindUser(id UserID) (User, bool, error) {
	return s.findUser(`SELECT u.id, u.username, u.password_hash, u.role, u.status, u.team_id, COALESCE(t.name, ''), u.created_at, u.updated_at FROM users u LEFT JOIN operation_teams t ON t.id = u.team_id WHERE u.id = ?`, id)
}

func (s *MySQLStore) FindUserByUsername(username string) (User, bool, error) {
	return s.findUser(`SELECT u.id, u.username, u.password_hash, u.role, u.status, u.team_id, COALESCE(t.name, ''), u.created_at, u.updated_at FROM users u LEFT JOIN operation_teams t ON t.id = u.team_id WHERE u.username = ?`, username)
}

func (s *MySQLStore) findUser(query string, arg any) (User, bool, error) {
	var user User
	var teamID sql.NullInt64
	err := s.db.QueryRow(query, arg).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.Role,
		&user.Status,
		&teamID,
		&user.TeamName,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, err
	}
	if teamID.Valid {
		value := TeamID(teamID.Int64)
		user.TeamID = &value
	}
	gameIDs, err := s.findGameIDs(user.ID)
	if err != nil {
		return User{}, false, err
	}
	user.GameIDs = gameIDs
	return user, true, nil
}

func (s *MySQLStore) findGameIDs(userID UserID) ([]string, error) {
	rows, err := s.db.Query(`SELECT game_id FROM user_game_scopes WHERE user_id = ? ORDER BY game_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var gameIDs []string
	for rows.Next() {
		var gameID string
		if err := rows.Scan(&gameID); err != nil {
			return nil, err
		}
		gameIDs = append(gameIDs, gameID)
	}
	return gameIDs, rows.Err()
}

func (s *MySQLStore) UpdateUser(user User) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := updateUserTx(tx, user); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) UpdateUserAndInvalidateSessions(user User, event AuditEvent) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := updateUserTx(tx, user); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE user_sessions SET invalidated_at = ? WHERE user_id = ? AND invalidated_at IS NULL`, user.UpdatedAt, user.ID); err != nil {
		return err
	}
	if err := appendAuditTx(tx, event); err != nil {
		return err
	}
	return tx.Commit()
}

func updateUserTx(tx *sql.Tx, user User) error {
	result, err := tx.Exec(
		`UPDATE users SET username = ?, password_hash = ?, role = ?, status = ?, team_id = ?, updated_at = ? WHERE id = ?`,
		user.Username, user.PasswordHash, user.Role, user.Status, user.TeamID, user.UpdatedAt, user.ID,
	)
	if err != nil {
		if duplicateKey(err) {
			return ErrUsernameTaken
		}
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrInvalidInput
	}
	if _, err := tx.Exec(`DELETE FROM user_game_scopes WHERE user_id = ?`, user.ID); err != nil {
		return err
	}
	for _, gameID := range user.GameIDs {
		if _, err := tx.Exec(
			`INSERT INTO user_game_scopes (user_id, game_id, created_at) VALUES (?, ?, ?)`,
			user.ID, gameID, user.UpdatedAt,
		); err != nil {
			return err
		}
	}
	return nil
}

func (s *MySQLStore) DeleteUser(userID UserID) error {
	var references int
	err := s.db.QueryRow(`SELECT
        (SELECT COUNT(*) FROM user_sessions WHERE user_id = ?) +
        (SELECT COUNT(*) FROM audit_logs WHERE actor_user_id = ?) +
        (SELECT COUNT(*) FROM media_accounts WHERE user_id = ?) +
        (SELECT COUNT(*) FROM media_account_tags WHERE user_id = ?) +
        (SELECT COUNT(*) FROM browser_profiles WHERE user_id = ?) +
        (SELECT COUNT(*) FROM profile_sync_scans WHERE user_id = ?) +
        (SELECT COUNT(*) FROM local_agent_binding_tickets WHERE user_id = ?) +
        (SELECT COUNT(*) FROM local_agent_nodes WHERE user_id = ?) +
        (SELECT COUNT(*) FROM browser_profile_runtime_presence WHERE user_id = ?) +
        (SELECT COUNT(*) FROM sensitive_browser_tasks WHERE user_id = ?) +
        (SELECT COUNT(*) FROM sensitive_profile_permits WHERE user_id = ?)`,
		userID, userID, userID, userID, userID, userID, userID, userID, userID, userID, userID,
	).Scan(&references)
	if err != nil {
		return err
	}
	if references > 0 {
		return ErrTeamInUse
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM user_game_scopes WHERE user_id = ?`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM users WHERE id = ?`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) DeleteUserWithAudit(userID UserID, event AuditEvent) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var references int
	if err := tx.QueryRow(`SELECT
        (SELECT COUNT(*) FROM user_sessions WHERE user_id = ?) +
        (SELECT COUNT(*) FROM audit_logs WHERE actor_user_id = ?) +
        (SELECT COUNT(*) FROM media_accounts WHERE user_id = ?) +
        (SELECT COUNT(*) FROM media_account_tags WHERE user_id = ?) +
        (SELECT COUNT(*) FROM browser_profiles WHERE user_id = ?) +
        (SELECT COUNT(*) FROM profile_sync_scans WHERE user_id = ?) +
        (SELECT COUNT(*) FROM local_agent_binding_tickets WHERE user_id = ?) +
        (SELECT COUNT(*) FROM local_agent_nodes WHERE user_id = ?) +
        (SELECT COUNT(*) FROM browser_profile_runtime_presence WHERE user_id = ?) +
        (SELECT COUNT(*) FROM sensitive_browser_tasks WHERE user_id = ?) +
        (SELECT COUNT(*) FROM sensitive_profile_permits WHERE user_id = ?)`,
		userID, userID, userID, userID, userID, userID, userID, userID, userID, userID, userID,
	).Scan(&references); err != nil {
		return err
	}
	if references > 0 {
		return ErrTeamInUse
	}
	if _, err := tx.Exec(`DELETE FROM user_game_scopes WHERE user_id = ?`, userID); err != nil {
		return err
	}
	result, err := tx.Exec(`DELETE FROM users WHERE id = ?`, userID)
	if err != nil {
		if foreignKeyReferenced(err) {
			return ErrTeamInUse
		}
		return err
	}
	if rows, err := result.RowsAffected(); err != nil || rows == 0 {
		if err != nil {
			return err
		}
		return ErrInvalidInput
	}
	if err := appendAuditTx(tx, event); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) CreateTeam(team OperationTeam) (TeamID, error) {
	result, err := s.db.Exec(
		`INSERT INTO operation_teams (name, created_at, updated_at) VALUES (?, ?, ?)`,
		team.Name, team.CreatedAt, team.UpdatedAt,
	)
	if err != nil {
		if duplicateKey(err) {
			return 0, ErrTeamNameTaken
		}
		return 0, err
	}
	id, err := result.LastInsertId()
	return TeamID(id), err
}

func (s *MySQLStore) CreateTeamWithAudit(team OperationTeam, event AuditEvent) (TeamID, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`INSERT INTO operation_teams (name, created_at, updated_at) VALUES (?, ?, ?)`, team.Name, team.CreatedAt, team.UpdatedAt)
	if err != nil {
		if duplicateKey(err) {
			return 0, ErrTeamNameTaken
		}
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if event.TargetID == "" || event.TargetID == "0" {
		event.TargetID = strconv.FormatInt(id, 10)
	}
	if err := appendAuditTx(tx, event); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return TeamID(id), nil
}

func (s *MySQLStore) FindTeam(teamID TeamID) (OperationTeam, bool, error) {
	var team OperationTeam
	err := s.db.QueryRow(
		`SELECT id, name, created_at, updated_at FROM operation_teams WHERE id = ?`, teamID,
	).Scan(&team.ID, &team.Name, &team.CreatedAt, &team.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return OperationTeam{}, false, nil
	}
	return team, err == nil, err
}

func (s *MySQLStore) ListTeams() ([]OperationTeam, error) {
	rows, err := s.db.Query(`SELECT id, name, created_at, updated_at FROM operation_teams ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var teams []OperationTeam
	for rows.Next() {
		var team OperationTeam
		if err := rows.Scan(&team.ID, &team.Name, &team.CreatedAt, &team.UpdatedAt); err != nil {
			return nil, err
		}
		teams = append(teams, team)
	}
	return teams, rows.Err()
}

func (s *MySQLStore) UpdateTeam(team OperationTeam) error {
	result, err := s.db.Exec(`UPDATE operation_teams SET name = ?, updated_at = ? WHERE id = ?`, team.Name, team.UpdatedAt, team.ID)
	if err != nil {
		if duplicateKey(err) {
			return ErrTeamNameTaken
		}
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrInvalidInput
	}
	return nil
}

func (s *MySQLStore) UpdateTeamWithAudit(team OperationTeam, event AuditEvent) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE operation_teams SET name = ?, updated_at = ? WHERE id = ?`, team.Name, team.UpdatedAt, team.ID)
	if err != nil {
		if duplicateKey(err) {
			return ErrTeamNameTaken
		}
		return err
	}
	if rows, err := result.RowsAffected(); err != nil || rows == 0 {
		if err != nil {
			return err
		}
		return ErrInvalidInput
	}
	if err := appendAuditTx(tx, event); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) DeleteTeam(teamID TeamID) error {
	result, err := s.db.Exec(`DELETE FROM operation_teams WHERE id = ?`, teamID)
	if err != nil {
		if foreignKeyReferenced(err) {
			return ErrTeamInUse
		}
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrInvalidInput
	}
	return nil
}

func (s *MySQLStore) DeleteTeamWithAudit(teamID TeamID, event AuditEvent) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`DELETE FROM operation_teams WHERE id = ?`, teamID)
	if err != nil {
		if foreignKeyReferenced(err) {
			return ErrTeamInUse
		}
		return err
	}
	if rows, err := result.RowsAffected(); err != nil || rows == 0 {
		if err != nil {
			return err
		}
		return ErrInvalidInput
	}
	if err := appendAuditTx(tx, event); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) TeamHasReferences(teamID TeamID) (bool, error) {
	var count int
	err := s.db.QueryRow(`SELECT
        (SELECT COUNT(*) FROM users WHERE team_id = ?) +
        (SELECT COUNT(*) FROM media_accounts WHERE team_id = ?) +
        (SELECT COUNT(*) FROM browser_profiles WHERE team_id = ?) +
        (SELECT COUNT(*) FROM profile_sync_scans WHERE team_id = ?)`,
		teamID, teamID, teamID, teamID,
	).Scan(&count)
	return count > 0, err
}

func (s *MySQLStore) CreateGameWithAudit(game OperationGame, event AuditEvent) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO operation_games (id, name, status, remark, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`, game.ID, game.Name, game.Status, game.Remark, game.CreatedAt, game.UpdatedAt); err != nil {
		if duplicateKey(err) {
			return ErrGameNameTaken
		}
		return err
	}
	if err := appendAuditTx(tx, event); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) FindGame(id string) (OperationGame, bool, error) {
	var game OperationGame
	err := s.db.QueryRow(`SELECT id, name, status, remark, created_at, updated_at FROM operation_games WHERE id = ?`, id).Scan(&game.ID, &game.Name, &game.Status, &game.Remark, &game.CreatedAt, &game.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return OperationGame{}, false, nil
	}
	return game, err == nil, err
}

func (s *MySQLStore) ListGames() ([]OperationGame, error) {
	rows, err := s.db.Query(`SELECT id, name, status, remark, created_at, updated_at FROM operation_games ORDER BY status, name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var games []OperationGame
	for rows.Next() {
		var game OperationGame
		if err := rows.Scan(&game.ID, &game.Name, &game.Status, &game.Remark, &game.CreatedAt, &game.UpdatedAt); err != nil {
			return nil, err
		}
		games = append(games, game)
	}
	return games, rows.Err()
}

func (s *MySQLStore) UpdateGameWithAudit(game OperationGame, event AuditEvent) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE operation_games SET name = ?, status = ?, remark = ?, updated_at = ? WHERE id = ?`, game.Name, game.Status, game.Remark, game.UpdatedAt, game.ID)
	if err != nil {
		if duplicateKey(err) {
			return ErrGameNameTaken
		}
		return err
	}
	if rows, err := result.RowsAffected(); err != nil || rows == 0 {
		if err != nil {
			return err
		}
		return ErrInvalidInput
	}
	if err := appendAuditTx(tx, event); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) DeleteGameWithAudit(id string, event AuditEvent) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`DELETE FROM operation_games WHERE id = ?`, id)
	if err != nil {
		if foreignKeyReferenced(err) {
			return ErrGameInUse
		}
		return err
	}
	if rows, err := result.RowsAffected(); err != nil || rows == 0 {
		if err != nil {
			return err
		}
		return ErrInvalidInput
	}
	if err := appendAuditTx(tx, event); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) GameHasReferences(id string) (bool, error) {
	var count int
	err := s.db.QueryRow(`SELECT
        (SELECT COUNT(*) FROM user_game_scopes WHERE game_id = ?) +
		(SELECT COUNT(*) FROM media_account_games WHERE game_id = ?)`,
		id, id,
	).Scan(&count)
	return count > 0, err
}

func (s *MySQLStore) ListGameReferenceSummaries() (map[string]GameReferenceSummary, error) {
	rows, err := s.db.Query(`SELECT g.id, COALESCE(scopes.count, 0), COALESCE(accounts.count, 0)
		FROM operation_games g
		LEFT JOIN (SELECT game_id, COUNT(*) AS count FROM user_game_scopes GROUP BY game_id) scopes ON scopes.game_id = g.id
		LEFT JOIN (SELECT game_id, COUNT(*) AS count FROM media_account_games GROUP BY game_id) accounts ON accounts.game_id = g.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	summaries := make(map[string]GameReferenceSummary)
	for rows.Next() {
		var gameID string
		var summary GameReferenceSummary
		if err := rows.Scan(&gameID, &summary.UserScopeCount, &summary.MediaAccountCount); err != nil {
			return nil, err
		}
		summaries[gameID] = summary
	}
	return summaries, rows.Err()
}

func (s *MySQLStore) GameReferences(gameID string) (GameReferences, error) {
	references := GameReferences{GameID: gameID, Users: []GameReferenceUser{}, MediaAccounts: []GameReferenceAccount{}}
	userRows, err := s.db.Query(`SELECT u.id, u.username, u.role, u.team_id, COALESCE(t.name, '')
		FROM user_game_scopes scopes
		JOIN users u ON u.id = scopes.user_id
		LEFT JOIN operation_teams t ON t.id = u.team_id
		WHERE scopes.game_id = ? ORDER BY u.id`, gameID)
	if err != nil {
		return GameReferences{}, err
	}
	defer userRows.Close()
	for userRows.Next() {
		var reference GameReferenceUser
		var teamID sql.NullInt64
		if err := userRows.Scan(&reference.UserID, &reference.Username, &reference.Role, &teamID, &reference.TeamName); err != nil {
			return GameReferences{}, err
		}
		if teamID.Valid {
			value := TeamID(teamID.Int64)
			reference.TeamID = &value
		}
		references.Users = append(references.Users, reference)
	}
	if err := userRows.Err(); err != nil {
		return GameReferences{}, err
	}
	accountRows, err := s.db.Query(`SELECT ma.id, COALESCE(ma.name, ''), ma.platform, ma.user_id, u.username
		FROM media_account_games relations
		JOIN media_accounts ma ON ma.id = relations.media_account_id
		JOIN users u ON u.id = ma.user_id
		WHERE relations.game_id = ? ORDER BY ma.id`, gameID)
	if err != nil {
		return GameReferences{}, err
	}
	defer accountRows.Close()
	for accountRows.Next() {
		var reference GameReferenceAccount
		if err := accountRows.Scan(&reference.AccountID, &reference.Name, &reference.Platform, &reference.UserID, &reference.Username); err != nil {
			return GameReferences{}, err
		}
		references.MediaAccounts = append(references.MediaAccounts, reference)
	}
	return references, accountRows.Err()
}

func (s *MySQLStore) UserHasGameReferencesOutsideScope(userID UserID, gameIDs []string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM media_accounts accounts JOIN media_account_games relations ON relations.media_account_id = accounts.id WHERE accounts.user_id = ?`
	args := []any{userID}
	if len(gameIDs) > 0 {
		query += ` AND relations.game_id NOT IN (` + strings.TrimRight(strings.Repeat("?,", len(gameIDs)), ",") + `)`
		for _, gameID := range gameIDs {
			args = append(args, gameID)
		}
	}
	query += `)`
	var exists bool
	err := s.db.QueryRow(query, args...).Scan(&exists)
	return exists, err
}

func (s *MySQLStore) CreateSession(session Session) error {
	_, err := s.db.Exec(
		`INSERT INTO user_sessions (id, user_id, token_hash, created_at, invalidated_at) VALUES (?, ?, ?, ?, ?)`,
		session.ID, session.UserID, session.TokenHash, session.CreatedAt, session.InvalidAt,
	)
	return err
}

func (s *MySQLStore) FindSessionByTokenHash(tokenHash string) (Session, bool, error) {
	var session Session
	var invalidatedAt sql.NullTime
	err := s.db.QueryRow(
		`SELECT id, user_id, token_hash, created_at, invalidated_at FROM user_sessions WHERE token_hash = ?`,
		tokenHash,
	).Scan(&session.ID, &session.UserID, &session.TokenHash, &session.CreatedAt, &invalidatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, false, nil
	}
	if err != nil {
		return Session{}, false, err
	}
	if invalidatedAt.Valid {
		at := invalidatedAt.Time
		session.InvalidAt = &at
	}
	return session, true, nil
}

func (s *MySQLStore) HasActiveSession(userID UserID) (bool, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM user_sessions WHERE user_id = ? AND invalidated_at IS NULL`,
		userID,
	).Scan(&count)
	return count > 0, err
}

func (s *MySQLStore) InvalidateUserSessions(userID UserID, at time.Time) error {
	_, err := s.db.Exec(
		`UPDATE user_sessions SET invalidated_at = ? WHERE user_id = ? AND invalidated_at IS NULL`,
		at, userID,
	)
	return err
}

func (s *MySQLStore) AppendAudit(event AuditEvent) error {
	return appendAuditTx(s.db, event)
}

type sqlExecer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func appendAuditTx(exec sqlExecer, event AuditEvent) error {
	summary, err := json.Marshal(event.Summary)
	if err != nil {
		return err
	}
	_, err = exec.Exec(
		`INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, summary_json, created_at) VALUES (?, NULLIF(?, 0), ?, ?, NULLIF(?, ''), ?, ?)`,
		event.ID, event.ActorUserID, event.Action, event.TargetType, event.TargetID, summary, event.CreatedAt,
	)
	return err
}

func (s *MySQLStore) ListUsers() ([]User, error) {
	rows, err := s.db.Query("SELECT u.id, u.username, u.password_hash, u.role, u.status, u.team_id, COALESCE(t.name, ''), u.created_at, u.updated_at FROM users u LEFT JOIN operation_teams t ON t.id = u.team_id ORDER BY u.created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		var teamID sql.NullInt64
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.Status, &teamID, &u.TeamName, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		if teamID.Valid {
			value := TeamID(teamID.Int64)
			u.TeamID = &value
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range users {
		gameIDs, err := s.findGameIDs(users[i].ID)
		if err != nil {
			return nil, err
		}
		users[i].GameIDs = gameIDs
	}
	return users, nil
}

func (s *MySQLStore) ListAuditLogs(limit int) ([]AuditEvent, error) {
	rows, err := s.db.Query("SELECT id, actor_user_id, action, target_type, target_id, summary_json, created_at FROM audit_logs ORDER BY created_at DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []AuditEvent
	for rows.Next() {
		var e AuditEvent
		var actorID sql.NullInt64
		var summaryJSON string
		if err := rows.Scan(&e.ID, &actorID, &e.Action, &e.TargetType, &e.TargetID, &summaryJSON, &e.CreatedAt); err != nil {
			return nil, err
		}
		if actorID.Valid {
			e.ActorUserID = UserID(actorID.Int64)
		}
		json.Unmarshal([]byte(summaryJSON), &e.Summary)
		events = append(events, e)
	}
	return events, rows.Err()
}

func duplicateKey(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

func foreignKeyReferenced(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1451
}
