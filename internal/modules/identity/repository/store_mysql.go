package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity/model"
	"gorm.io/gorm"
)

func CountUsers() (int, error) {
	return countUsers(database.DB())
}

func countUsers(db *gorm.DB) (int, error) {
	var count int
	err := queryRow(db, `SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

func CreateUser(user model.User) (model.UserID, error) {
	return createUser(database.DB(), user)
}

func createUser(db *gorm.DB, user model.User) (model.UserID, error) {
	tx := db.Begin()
	defer rollbackTx(tx)

	userID, err := createUserTx(tx, user)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit().Error; err != nil {
		return 0, err
	}
	return userID, nil
}

func CreateUserWithAudit(user model.User, event model.AuditEvent) (model.UserID, error) {
	return createUserWithAudit(database.DB(), user, event)
}

func createUserWithAudit(db *gorm.DB, user model.User, event model.AuditEvent) (model.UserID, error) {
	tx := db.Begin()
	defer rollbackTx(tx)
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
	if err := tx.Commit().Error; err != nil {
		return 0, err
	}
	return userID, nil
}

func createUserTx(tx *gorm.DB, user model.User) (model.UserID, error) {
	_, err := execSQL(tx,
		`INSERT INTO users (username, password_hash, role, status, team_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		user.Username, user.PasswordHash, user.Role, user.Status, user.TeamID, user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		if duplicateKey(err) {
			return 0, model.ErrUsernameTaken
		}
		return 0, err
	}
	insertedID, err := lastInsertID(tx)
	if err != nil {
		return 0, err
	}
	user.ID = model.UserID(insertedID)
	for _, gameID := range user.GameIDs {
		if _, err := execSQL(tx,
			`INSERT INTO user_game_scopes (user_id, game_id, created_at) VALUES (?, ?, ?)`,
			user.ID, gameID, user.CreatedAt,
		); err != nil {
			return 0, err
		}
	}
	return user.ID, nil
}

func FindUser(id model.UserID) (model.User, bool, error) {
	return findUserByID(database.DB(), id)
}

func findUserByID(db *gorm.DB, id model.UserID) (model.User, bool, error) {
	return findUserByQuery(db, `SELECT u.id, u.username, u.password_hash, u.role, u.status, u.team_id, COALESCE(t.name, ''), u.created_at, u.updated_at FROM users u LEFT JOIN operation_teams t ON t.id = u.team_id WHERE u.id = ?`, id)
}

func FindUserByUsername(username string) (model.User, bool, error) {
	return findUserByUsername(database.DB(), username)
}

func findUserByUsername(db *gorm.DB, username string) (model.User, bool, error) {
	return findUserByQuery(db, `SELECT u.id, u.username, u.password_hash, u.role, u.status, u.team_id, COALESCE(t.name, ''), u.created_at, u.updated_at FROM users u LEFT JOIN operation_teams t ON t.id = u.team_id WHERE u.username = ?`, username)
}

func findUserByQuery(db *gorm.DB, query string, arg any) (model.User, bool, error) {
	var user model.User
	var teamID sql.NullInt64
	err := queryRow(db, query, arg).Scan(
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
		return model.User{}, false, nil
	}
	if err != nil {
		return model.User{}, false, err
	}
	if teamID.Valid {
		value := model.TeamID(teamID.Int64)
		user.TeamID = &value
	}
	gameIDs, err := findGameIDs(db, user.ID)
	if err != nil {
		return model.User{}, false, err
	}
	user.GameIDs = gameIDs
	return user, true, nil
}

func findGameIDs(db *gorm.DB, userID model.UserID) ([]string, error) {
	rows, err := queryRows(db, `SELECT game_id FROM user_game_scopes WHERE user_id = ? ORDER BY game_id`, userID)
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

func UpdateUser(user model.User) error {
	return updateUser(database.DB(), user)
}

func updateUser(db *gorm.DB, user model.User) error {
	tx := db.Begin()
	defer rollbackTx(tx)

	if err := updateUserTx(tx, user); err != nil {
		return err
	}
	return tx.Commit().Error
}

func UpdateUserAndInvalidateSessions(user model.User, event model.AuditEvent) error {
	return updateUserAndInvalidateSessions(database.DB(), user, event)
}

func updateUserAndInvalidateSessions(db *gorm.DB, user model.User, event model.AuditEvent) error {
	tx := db.Begin()
	defer rollbackTx(tx)
	if err := updateUserTx(tx, user); err != nil {
		return err
	}
	if _, err := execSQL(tx, `UPDATE user_sessions SET invalidated_at = ? WHERE user_id = ? AND invalidated_at IS NULL`, user.UpdatedAt, user.ID); err != nil {
		return err
	}
	if err := appendAuditTx(tx, event); err != nil {
		return err
	}
	return tx.Commit().Error
}

func updateUserTx(tx *gorm.DB, user model.User) error {
	result, err := execSQL(tx,
		`UPDATE users SET username = ?, password_hash = ?, role = ?, status = ?, team_id = ?, updated_at = ? WHERE id = ?`,
		user.Username, user.PasswordHash, user.Role, user.Status, user.TeamID, user.UpdatedAt, user.ID,
	)
	if err != nil {
		if duplicateKey(err) {
			return model.ErrUsernameTaken
		}
		return err
	}
	rows := result
	if rows == 0 {
		return model.ErrInvalidInput
	}
	if _, err := execSQL(tx, `DELETE FROM user_game_scopes WHERE user_id = ?`, user.ID); err != nil {
		return err
	}
	for _, gameID := range user.GameIDs {
		if _, err := execSQL(tx,
			`INSERT INTO user_game_scopes (user_id, game_id, created_at) VALUES (?, ?, ?)`,
			user.ID, gameID, user.UpdatedAt,
		); err != nil {
			return err
		}
	}
	return nil
}

func DeleteUser(userID model.UserID) error {
	return deleteUser(database.DB(), userID)
}

func deleteUser(db *gorm.DB, userID model.UserID) error {
	var references int
	err := queryRow(db, `SELECT
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
		return model.ErrTeamInUse
	}
	tx := db.Begin()
	defer rollbackTx(tx)
	if _, err := execSQL(tx, `DELETE FROM user_game_scopes WHERE user_id = ?`, userID); err != nil {
		return err
	}
	if _, err := execSQL(tx, `DELETE FROM users WHERE id = ?`, userID); err != nil {
		return err
	}
	return tx.Commit().Error
}

func DeleteUserWithAudit(userID model.UserID, event model.AuditEvent) error {
	return deleteUserWithAudit(database.DB(), userID, event)
}

func deleteUserWithAudit(db *gorm.DB, userID model.UserID, event model.AuditEvent) error {
	tx := db.Begin()
	defer rollbackTx(tx)
	var references int
	if err := queryRow(tx, `SELECT
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
		return model.ErrTeamInUse
	}
	if _, err := execSQL(tx, `DELETE FROM user_game_scopes WHERE user_id = ?`, userID); err != nil {
		return err
	}
	result, err := execSQL(tx, `DELETE FROM users WHERE id = ?`, userID)
	if err != nil {
		if foreignKeyReferenced(err) {
			return model.ErrTeamInUse
		}
		return err
	}
	if rows := result; rows == 0 {
		return model.ErrInvalidInput
	}
	if err := appendAuditTx(tx, event); err != nil {
		return err
	}
	return tx.Commit().Error
}

func CreateTeam(team model.OperationTeam) (model.TeamID, error) {
	return createTeam(database.DB(), team)
}

func createTeam(db *gorm.DB, team model.OperationTeam) (model.TeamID, error) {
	_, err := execSQL(db,
		`INSERT INTO operation_teams (name, created_at, updated_at) VALUES (?, ?, ?)`,
		team.Name, team.CreatedAt, team.UpdatedAt,
	)
	if err != nil {
		if duplicateKey(err) {
			return 0, model.ErrTeamNameTaken
		}
		return 0, err
	}
	id, err := lastInsertID(db)
	return model.TeamID(id), err
}

func CreateTeamWithAudit(team model.OperationTeam, event model.AuditEvent) (model.TeamID, error) {
	return createTeamWithAudit(database.DB(), team, event)
}

func createTeamWithAudit(db *gorm.DB, team model.OperationTeam, event model.AuditEvent) (model.TeamID, error) {
	tx := db.Begin()
	defer rollbackTx(tx)
	_, err := execSQL(tx, `INSERT INTO operation_teams (name, created_at, updated_at) VALUES (?, ?, ?)`, team.Name, team.CreatedAt, team.UpdatedAt)
	if err != nil {
		if duplicateKey(err) {
			return 0, model.ErrTeamNameTaken
		}
		return 0, err
	}
	id, err := lastInsertID(tx)
	if err != nil {
		return 0, err
	}
	if event.TargetID == "" || event.TargetID == "0" {
		event.TargetID = strconv.FormatInt(id, 10)
	}
	if err := appendAuditTx(tx, event); err != nil {
		return 0, err
	}
	if err := tx.Commit().Error; err != nil {
		return 0, err
	}
	return model.TeamID(id), nil
}

func FindTeam(teamID model.TeamID) (model.OperationTeam, bool, error) {
	return findTeam(database.DB(), teamID)
}

func findTeam(db *gorm.DB, teamID model.TeamID) (model.OperationTeam, bool, error) {
	var team model.OperationTeam
	err := queryRow(db,
		`SELECT id, name, created_at, updated_at FROM operation_teams WHERE id = ?`, teamID,
	).Scan(&team.ID, &team.Name, &team.CreatedAt, &team.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.OperationTeam{}, false, nil
	}
	return team, err == nil, err
}

func ListTeams() ([]model.OperationTeam, error) {
	return listTeams(database.DB())
}

func listTeams(db *gorm.DB) ([]model.OperationTeam, error) {
	rows, err := queryRows(db, `SELECT id, name, created_at, updated_at FROM operation_teams ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var teams []model.OperationTeam
	for rows.Next() {
		var team model.OperationTeam
		if err := rows.Scan(&team.ID, &team.Name, &team.CreatedAt, &team.UpdatedAt); err != nil {
			return nil, err
		}
		teams = append(teams, team)
	}
	return teams, rows.Err()
}

func UpdateTeam(team model.OperationTeam) error {
	return updateTeam(database.DB(), team)
}

func updateTeam(db *gorm.DB, team model.OperationTeam) error {
	result, err := execSQL(db, `UPDATE operation_teams SET name = ?, updated_at = ? WHERE id = ?`, team.Name, team.UpdatedAt, team.ID)
	if err != nil {
		if duplicateKey(err) {
			return model.ErrTeamNameTaken
		}
		return err
	}
	rows := result
	if rows == 0 {
		return model.ErrInvalidInput
	}
	return nil
}

func UpdateTeamWithAudit(team model.OperationTeam, event model.AuditEvent) error {
	return updateTeamWithAudit(database.DB(), team, event)
}

func updateTeamWithAudit(db *gorm.DB, team model.OperationTeam, event model.AuditEvent) error {
	tx := db.Begin()
	defer rollbackTx(tx)
	result, err := execSQL(tx, `UPDATE operation_teams SET name = ?, updated_at = ? WHERE id = ?`, team.Name, team.UpdatedAt, team.ID)
	if err != nil {
		if duplicateKey(err) {
			return model.ErrTeamNameTaken
		}
		return err
	}
	if rows := result; rows == 0 {
		return model.ErrInvalidInput
	}
	if err := appendAuditTx(tx, event); err != nil {
		return err
	}
	return tx.Commit().Error
}

func DeleteTeam(teamID model.TeamID) error {
	return deleteTeam(database.DB(), teamID)
}

func deleteTeam(db *gorm.DB, teamID model.TeamID) error {
	result, err := execSQL(db, `DELETE FROM operation_teams WHERE id = ?`, teamID)
	if err != nil {
		if foreignKeyReferenced(err) {
			return model.ErrTeamInUse
		}
		return err
	}
	rows := result
	if rows == 0 {
		return model.ErrInvalidInput
	}
	return nil
}

func DeleteTeamWithAudit(teamID model.TeamID, event model.AuditEvent) error {
	return deleteTeamWithAudit(database.DB(), teamID, event)
}

func deleteTeamWithAudit(db *gorm.DB, teamID model.TeamID, event model.AuditEvent) error {
	tx := db.Begin()
	defer rollbackTx(tx)
	result, err := execSQL(tx, `DELETE FROM operation_teams WHERE id = ?`, teamID)
	if err != nil {
		if foreignKeyReferenced(err) {
			return model.ErrTeamInUse
		}
		return err
	}
	if rows := result; rows == 0 {
		return model.ErrInvalidInput
	}
	if err := appendAuditTx(tx, event); err != nil {
		return err
	}
	return tx.Commit().Error
}

func TeamHasReferences(teamID model.TeamID) (bool, error) {
	return teamHasReferences(database.DB(), teamID)
}

func teamHasReferences(db *gorm.DB, teamID model.TeamID) (bool, error) {
	var count int
	err := queryRow(db, `SELECT
        (SELECT COUNT(*) FROM users WHERE team_id = ?) +
        (SELECT COUNT(*) FROM media_accounts WHERE team_id = ?) +
        (SELECT COUNT(*) FROM browser_profiles WHERE team_id = ?) +
        (SELECT COUNT(*) FROM profile_sync_scans WHERE team_id = ?)`,
		teamID, teamID, teamID, teamID,
	).Scan(&count)
	return count > 0, err
}

func CreateGameWithAudit(game model.OperationGame, event model.AuditEvent) error {
	return createGameWithAudit(database.DB(), game, event)
}

func createGameWithAudit(db *gorm.DB, game model.OperationGame, event model.AuditEvent) error {
	tx := db.Begin()
	defer rollbackTx(tx)
	if _, err := execSQL(tx, `INSERT INTO operation_games (id, name, status, remark, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`, game.ID, game.Name, game.Status, game.Remark, game.CreatedAt, game.UpdatedAt); err != nil {
		if duplicateKey(err) {
			return model.ErrGameNameTaken
		}
		return err
	}
	if err := appendAuditTx(tx, event); err != nil {
		return err
	}
	return tx.Commit().Error
}

func FindGame(id string) (model.OperationGame, bool, error) {
	return findGame(database.DB(), id)
}

func findGame(db *gorm.DB, id string) (model.OperationGame, bool, error) {
	var game model.OperationGame
	err := queryRow(db, `SELECT id, name, status, remark, created_at, updated_at FROM operation_games WHERE id = ?`, id).Scan(&game.ID, &game.Name, &game.Status, &game.Remark, &game.CreatedAt, &game.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.OperationGame{}, false, nil
	}
	return game, err == nil, err
}

func ListGames() ([]model.OperationGame, error) {
	return listGames(database.DB())
}

func listGames(db *gorm.DB) ([]model.OperationGame, error) {
	rows, err := queryRows(db, `SELECT id, name, status, remark, created_at, updated_at FROM operation_games ORDER BY status, name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var games []model.OperationGame
	for rows.Next() {
		var game model.OperationGame
		if err := rows.Scan(&game.ID, &game.Name, &game.Status, &game.Remark, &game.CreatedAt, &game.UpdatedAt); err != nil {
			return nil, err
		}
		games = append(games, game)
	}
	return games, rows.Err()
}

func UpdateGameWithAudit(game model.OperationGame, event model.AuditEvent) error {
	return updateGameWithAudit(database.DB(), game, event)
}

func updateGameWithAudit(db *gorm.DB, game model.OperationGame, event model.AuditEvent) error {
	tx := db.Begin()
	defer rollbackTx(tx)
	result, err := execSQL(tx, `UPDATE operation_games SET name = ?, status = ?, remark = ?, updated_at = ? WHERE id = ?`, game.Name, game.Status, game.Remark, game.UpdatedAt, game.ID)
	if err != nil {
		if duplicateKey(err) {
			return model.ErrGameNameTaken
		}
		return err
	}
	if rows := result; rows == 0 {
		return model.ErrInvalidInput
	}
	if err := appendAuditTx(tx, event); err != nil {
		return err
	}
	return tx.Commit().Error
}

func DeleteGameWithAudit(id string, event model.AuditEvent) error {
	return deleteGameWithAudit(database.DB(), id, event)
}

func deleteGameWithAudit(db *gorm.DB, id string, event model.AuditEvent) error {
	tx := db.Begin()
	defer rollbackTx(tx)
	result, err := execSQL(tx, `DELETE FROM operation_games WHERE id = ?`, id)
	if err != nil {
		if foreignKeyReferenced(err) {
			return model.ErrGameInUse
		}
		return err
	}
	if rows := result; rows == 0 {
		return model.ErrInvalidInput
	}
	if err := appendAuditTx(tx, event); err != nil {
		return err
	}
	return tx.Commit().Error
}

func GameHasReferences(id string) (bool, error) {
	return gameHasReferences(database.DB(), id)
}

func gameHasReferences(db *gorm.DB, id string) (bool, error) {
	var count int
	err := queryRow(db, `SELECT
        (SELECT COUNT(*) FROM user_game_scopes WHERE game_id = ?) +
		(SELECT COUNT(*) FROM media_account_games WHERE game_id = ?)`,
		id, id,
	).Scan(&count)
	return count > 0, err
}

func ListGameReferenceSummaries() (map[string]model.GameReferenceSummary, error) {
	return listGameReferenceSummaries(database.DB())
}

func listGameReferenceSummaries(db *gorm.DB) (map[string]model.GameReferenceSummary, error) {
	rows, err := queryRows(db, `SELECT g.id, COALESCE(scopes.count, 0), COALESCE(accounts.count, 0)
		FROM operation_games g
		LEFT JOIN (SELECT game_id, COUNT(*) AS count FROM user_game_scopes GROUP BY game_id) scopes ON scopes.game_id = g.id
		LEFT JOIN (SELECT game_id, COUNT(*) AS count FROM media_account_games GROUP BY game_id) accounts ON accounts.game_id = g.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	summaries := make(map[string]model.GameReferenceSummary)
	for rows.Next() {
		var gameID string
		var summary model.GameReferenceSummary
		if err := rows.Scan(&gameID, &summary.UserScopeCount, &summary.MediaAccountCount); err != nil {
			return nil, err
		}
		summaries[gameID] = summary
	}
	return summaries, rows.Err()
}

func GameReferences(gameID string) (model.GameReferences, error) {
	return gameReferences(database.DB(), gameID)
}

func gameReferences(db *gorm.DB, gameID string) (model.GameReferences, error) {
	references := model.GameReferences{GameID: gameID, Users: []model.GameReferenceUser{}, MediaAccounts: []model.GameReferenceAccount{}}
	userRows, err := queryRows(db, `SELECT u.id, u.username, u.role, u.team_id, COALESCE(t.name, '')
		FROM user_game_scopes scopes
		JOIN users u ON u.id = scopes.user_id
		LEFT JOIN operation_teams t ON t.id = u.team_id
		WHERE scopes.game_id = ? ORDER BY u.id`, gameID)
	if err != nil {
		return model.GameReferences{}, err
	}
	defer userRows.Close()
	for userRows.Next() {
		var reference model.GameReferenceUser
		var teamID sql.NullInt64
		if err := userRows.Scan(&reference.UserID, &reference.Username, &reference.Role, &teamID, &reference.TeamName); err != nil {
			return model.GameReferences{}, err
		}
		if teamID.Valid {
			value := model.TeamID(teamID.Int64)
			reference.TeamID = &value
		}
		references.Users = append(references.Users, reference)
	}
	if err := userRows.Err(); err != nil {
		return model.GameReferences{}, err
	}
	accountRows, err := queryRows(db, `SELECT ma.id, COALESCE(ma.name, ''), ma.platform, ma.user_id, u.username
		FROM media_account_games relations
		JOIN media_accounts ma ON ma.id = relations.media_account_id
		JOIN users u ON u.id = ma.user_id
		WHERE relations.game_id = ? ORDER BY ma.id`, gameID)
	if err != nil {
		return model.GameReferences{}, err
	}
	defer accountRows.Close()
	for accountRows.Next() {
		var reference model.GameReferenceAccount
		if err := accountRows.Scan(&reference.AccountID, &reference.Name, &reference.Platform, &reference.UserID, &reference.Username); err != nil {
			return model.GameReferences{}, err
		}
		references.MediaAccounts = append(references.MediaAccounts, reference)
	}
	return references, accountRows.Err()
}

func UserHasGameReferencesOutsideScope(userID model.UserID, gameIDs []string) (bool, error) {
	return userHasGameReferencesOutsideScope(database.DB(), userID, gameIDs)
}

func userHasGameReferencesOutsideScope(db *gorm.DB, userID model.UserID, gameIDs []string) (bool, error) {
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
	err := queryRow(db, query, args...).Scan(&exists)
	return exists, err
}

func CreateSession(session model.Session) error {
	return createSession(database.DB(), session)
}

func createSession(db *gorm.DB, session model.Session) error {
	_, err := execSQL(db,
		`INSERT INTO user_sessions (id, user_id, token_hash, created_at, invalidated_at) VALUES (?, ?, ?, ?, ?)`,
		session.ID, session.UserID, session.TokenHash, session.CreatedAt, session.InvalidAt,
	)
	return err
}

func FindSessionByTokenHash(tokenHash string) (model.Session, bool, error) {
	return findSessionByTokenHash(database.DB(), tokenHash)
}

func findSessionByTokenHash(db *gorm.DB, tokenHash string) (model.Session, bool, error) {
	var session model.Session
	var invalidatedAt sql.NullTime
	err := queryRow(db,
		`SELECT id, user_id, token_hash, created_at, invalidated_at FROM user_sessions WHERE token_hash = ?`,
		tokenHash,
	).Scan(&session.ID, &session.UserID, &session.TokenHash, &session.CreatedAt, &invalidatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Session{}, false, nil
	}
	if err != nil {
		return model.Session{}, false, err
	}
	if invalidatedAt.Valid {
		at := invalidatedAt.Time
		session.InvalidAt = &at
	}
	return session, true, nil
}

func HasActiveSession(userID model.UserID) (bool, error) {
	return hasActiveSession(database.DB(), userID)
}

func hasActiveSession(db *gorm.DB, userID model.UserID) (bool, error) {
	var count int
	err := queryRow(db,
		`SELECT COUNT(*) FROM user_sessions WHERE user_id = ? AND invalidated_at IS NULL`,
		userID,
	).Scan(&count)
	return count > 0, err
}

func InvalidateUserSessions(userID model.UserID, at time.Time) error {
	return invalidateUserSessions(database.DB(), userID, at)
}

func invalidateUserSessions(db *gorm.DB, userID model.UserID, at time.Time) error {
	_, err := execSQL(db,
		`UPDATE user_sessions SET invalidated_at = ? WHERE user_id = ? AND invalidated_at IS NULL`,
		at, userID,
	)
	return err
}

func AppendAudit(event model.AuditEvent) error {
	return appendAudit(database.DB(), event)
}

func appendAudit(db *gorm.DB, event model.AuditEvent) error {
	return appendAuditTx(db, event)
}

func appendAuditTx(db *gorm.DB, event model.AuditEvent) error {
	summary, err := json.Marshal(event.Summary)
	if err != nil {
		return err
	}
	_, err = execSQL(db,
		`INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, summary_json, created_at) VALUES (?, NULLIF(?, 0), ?, ?, NULLIF(?, ''), ?, ?)`,
		event.ID, event.ActorUserID, event.Action, event.TargetType, event.TargetID, summary, event.CreatedAt,
	)
	return err
}

func ListUsers() ([]model.User, error) {
	return listUsers(database.DB())
}

func listUsers(db *gorm.DB) ([]model.User, error) {
	rows, err := queryRows(db, "SELECT u.id, u.username, u.password_hash, u.role, u.status, u.team_id, COALESCE(t.name, ''), u.created_at, u.updated_at FROM users u LEFT JOIN operation_teams t ON t.id = u.team_id ORDER BY u.created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []model.User
	for rows.Next() {
		var u model.User
		var teamID sql.NullInt64
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.Status, &teamID, &u.TeamName, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		if teamID.Valid {
			value := model.TeamID(teamID.Int64)
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
		gameIDs, err := findGameIDs(db, users[i].ID)
		if err != nil {
			return nil, err
		}
		users[i].GameIDs = gameIDs
	}
	return users, nil
}

func ListAuditLogs(limit int) ([]model.AuditEvent, error) {
	return listAuditLogs(database.DB(), limit)
}

func listAuditLogs(db *gorm.DB, limit int) ([]model.AuditEvent, error) {
	rows, err := queryRows(db, "SELECT id, actor_user_id, action, target_type, target_id, summary_json, created_at FROM audit_logs ORDER BY created_at DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []model.AuditEvent
	for rows.Next() {
		var e model.AuditEvent
		var actorID sql.NullInt64
		var summaryJSON string
		if err := rows.Scan(&e.ID, &actorID, &e.Action, &e.TargetType, &e.TargetID, &summaryJSON, &e.CreatedAt); err != nil {
			return nil, err
		}
		if actorID.Valid {
			e.ActorUserID = model.UserID(actorID.Int64)
		}
		json.Unmarshal([]byte(summaryJSON), &e.Summary)
		events = append(events, e)
	}
	return events, rows.Err()
}

func lastInsertID(db *gorm.DB) (int64, error) {
	var id int64
	err := queryRow(db, "SELECT LAST_INSERT_ID()").Scan(&id)
	return id, err
}

func queryRow(db *gorm.DB, query string, args ...any) *sql.Row {
	return db.Raw(query, args...).Row()
}

func queryRows(db *gorm.DB, query string, args ...any) (*sql.Rows, error) {
	return db.Raw(query, args...).Rows()
}

func execSQL(db *gorm.DB, query string, args ...any) (int64, error) {
	result := db.Exec(query, args...)
	return result.RowsAffected, result.Error
}

func rollbackTx(tx *gorm.DB) {
	_ = tx.Rollback().Error
}

func duplicateKey(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

func foreignKeyReferenced(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1451
}
