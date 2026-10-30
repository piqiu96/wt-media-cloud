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
	for _, categoryID := range user.CategoryIDs {
		if _, err := execSQL(tx,
			`INSERT INTO user_category_scopes (user_id, category_id, created_at) VALUES (?, ?, ?)`,
			user.ID, categoryID, user.CreatedAt,
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
	return findUserByQuery(db, `SELECT u.id, u.username, COALESCE(u.nickname, u.username), COALESCE(u.avatar_id, 'sky'), u.password_hash, u.role, u.status, u.team_id, COALESCE(t.name, ''), u.created_at, u.updated_at FROM users u LEFT JOIN operation_teams t ON t.id = u.team_id WHERE u.id = ?`, id)
}

func FindUserByUsername(username string) (model.User, bool, error) {
	return findUserByUsername(database.DB(), username)
}

func findUserByUsername(db *gorm.DB, username string) (model.User, bool, error) {
	return findUserByQuery(db, `SELECT u.id, u.username, COALESCE(u.nickname, u.username), COALESCE(u.avatar_id, 'sky'), u.password_hash, u.role, u.status, u.team_id, COALESCE(t.name, ''), u.created_at, u.updated_at FROM users u LEFT JOIN operation_teams t ON t.id = u.team_id WHERE u.username = ?`, username)
}

func findUserByQuery(db *gorm.DB, query string, arg any) (model.User, bool, error) {
	var user model.User
	var teamID sql.NullInt64
	err := queryRow(db, query, arg).Scan(
		&user.ID,
		&user.Username,
		&user.Nickname,
		&user.AvatarID,
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
	categoryIDs, err := findCategoryIDs(db, user.ID)
	if err != nil {
		return model.User{}, false, err
	}
	user.CategoryIDs = categoryIDs
	return user, true, nil
}

func findCategoryIDs(db *gorm.DB, userID model.UserID) ([]model.CategoryID, error) {
	rows, err := queryRows(db, `SELECT category_id FROM user_category_scopes WHERE user_id = ? ORDER BY category_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categoryIDs []model.CategoryID
	for rows.Next() {
		var categoryID model.CategoryID
		if err := rows.Scan(&categoryID); err != nil {
			return nil, err
		}
		categoryIDs = append(categoryIDs, categoryID)
	}
	return categoryIDs, rows.Err()
}

func UpdateUser(user model.User) error {
	return updateUser(database.DB(), user)
}

func UpdateOwnProfile(userID model.UserID, nickname, avatarID string, at time.Time, event model.AuditEvent) error {
	tx := database.DB().Begin()
	defer rollbackTx(tx)
	changed, err := execSQL(tx, `UPDATE users SET nickname = ?, avatar_id = ?, updated_at = ? WHERE id = ? AND status = 'enabled'`, nickname, avatarID, at, userID)
	if err != nil {
		return err
	}
	if changed == 0 {
		return model.ErrInvalidInput
	}
	if err := appendAuditTx(tx, event); err != nil {
		return err
	}
	return tx.Commit().Error
}

func updateUser(db *gorm.DB, user model.User) error {
	tx := db.Begin()
	defer rollbackTx(tx)

	if err := updateUserTx(tx, user); err != nil {
		return err
	}
	return tx.Commit().Error
}

func UpdateUserAndInvalidateSessions(user model.User, event model.AuditEvent, reason string) error {
	return updateUserAndInvalidateSessions(database.DB(), user, event, reason)
}

func updateUserAndInvalidateSessions(db *gorm.DB, user model.User, event model.AuditEvent, reason string) error {
	tx := db.Begin()
	defer rollbackTx(tx)
	if err := updateUserTx(tx, user); err != nil {
		return err
	}
	if _, err := execSQL(tx, `UPDATE user_sessions SET invalidated_at = ?, invalidated_reason = ? WHERE user_id = ? AND invalidated_at IS NULL`, user.UpdatedAt, reason, user.ID); err != nil {
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
	if _, err := execSQL(tx, `DELETE FROM user_category_scopes WHERE user_id = ?`, user.ID); err != nil {
		return err
	}
	for _, categoryID := range user.CategoryIDs {
		if _, err := execSQL(tx,
			`INSERT INTO user_category_scopes (user_id, category_id, created_at) VALUES (?, ?, ?)`,
			user.ID, categoryID, user.UpdatedAt,
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
        (SELECT COUNT(*) FROM browser_profiles WHERE user_id = ?)`,
		userID, userID, userID, userID,
	).Scan(&references)
	if err != nil {
		return err
	}
	if references > 0 {
		return model.ErrTeamInUse
	}
	tx := db.Begin()
	defer rollbackTx(tx)
	if _, err := execSQL(tx, `DELETE FROM user_category_scopes WHERE user_id = ?`, userID); err != nil {
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
        (SELECT COUNT(*) FROM browser_profiles WHERE user_id = ?)`,
		userID, userID, userID, userID,
	).Scan(&references); err != nil {
		return err
	}
	if references > 0 {
		return model.ErrTeamInUse
	}
	if _, err := execSQL(tx, `DELETE FROM user_category_scopes WHERE user_id = ?`, userID); err != nil {
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

// teamColumns reads the membership count in the same statement as the row, so a
// team's number of users is never a second query that can answer a different
// question than the row it is shown beside.
const teamColumns = `SELECT t.id, t.name, t.created_at, t.updated_at,
	(SELECT COUNT(*) FROM users u WHERE u.team_id = t.id) AS user_count
	FROM operation_teams t`

func findTeam(db *gorm.DB, teamID model.TeamID) (model.OperationTeam, bool, error) {
	var team model.OperationTeam
	err := queryRow(db, teamColumns+` WHERE t.id = ?`, teamID).Scan(&team.ID, &team.Name, &team.CreatedAt, &team.UpdatedAt, &team.UserCount)
	if errors.Is(err, sql.ErrNoRows) {
		return model.OperationTeam{}, false, nil
	}
	return team, err == nil, err
}

func ListTeams(keyword string) ([]model.OperationTeam, error) {
	return listTeams(database.DB(), keyword)
}

func listTeams(db *gorm.DB, keyword string) ([]model.OperationTeam, error) {
	query := teamColumns + ` WHERE 1=1`
	args := make([]any, 0, 2)
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		query += ` AND (t.name LIKE ? ESCAPE '!' OR CAST(t.id AS CHAR) = ?)`
		args = append(args, teamLikeContains(keyword), keyword)
	}
	query += ` ORDER BY t.name, t.id`
	rows, err := queryRows(db, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var teams []model.OperationTeam
	for rows.Next() {
		var team model.OperationTeam
		if err := rows.Scan(&team.ID, &team.Name, &team.CreatedAt, &team.UpdatedAt, &team.UserCount); err != nil {
			return nil, err
		}
		teams = append(teams, team)
	}
	return teams, rows.Err()
}

func teamLikeContains(value string) string {
	return "%" + strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(value) + "%"
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
        (SELECT COUNT(*) FROM browser_profiles WHERE team_id = ?)`,
		teamID, teamID, teamID,
	).Scan(&count)
	return count > 0, err
}

func CreateCategoryWithAudit(category model.Category, event model.AuditEvent) (model.CategoryID, error) {
	return createCategoryWithAudit(database.DB(), category, event)
}

func createCategoryWithAudit(db *gorm.DB, category model.Category, event model.AuditEvent) (model.CategoryID, error) {
	tx := db.Begin()
	defer rollbackTx(tx)
	if _, err := execSQL(tx, `INSERT INTO categories (type, name, enabled, remark, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		category.Type, category.Name, category.Enabled, category.Remark, category.CreatedAt, category.UpdatedAt); err != nil {
		if duplicateKey(err) {
			return 0, model.ErrCategoryNameTaken
		}
		return 0, err
	}
	id, err := lastInsertID(tx)
	if err != nil {
		return 0, err
	}
	if err := appendAuditTx(tx, event); err != nil {
		return 0, err
	}
	if err := tx.Commit().Error; err != nil {
		return 0, err
	}
	return model.CategoryID(id), nil
}

func FindCategory(id model.CategoryID) (model.Category, bool, error) {
	return findCategory(database.DB(), id)
}

func findCategory(db *gorm.DB, id model.CategoryID) (model.Category, bool, error) {
	var category model.Category
	err := queryRow(db, `SELECT id, type, name, enabled, remark, created_at, updated_at FROM categories WHERE id = ?`, id).
		Scan(&category.ID, &category.Type, &category.Name, &category.Enabled, &category.Remark, &category.CreatedAt, &category.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Category{}, false, nil
	}
	return category, err == nil, err
}

func ListCategories() ([]model.Category, error) {
	return listCategories(database.DB())
}

func listCategories(db *gorm.DB) ([]model.Category, error) {
	rows, err := queryRows(db, `SELECT id, type, name, enabled, remark, created_at, updated_at FROM categories ORDER BY enabled DESC, type, name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var categories []model.Category
	for rows.Next() {
		var category model.Category
		if err := rows.Scan(&category.ID, &category.Type, &category.Name, &category.Enabled, &category.Remark, &category.CreatedAt, &category.UpdatedAt); err != nil {
			return nil, err
		}
		categories = append(categories, category)
	}
	return categories, rows.Err()
}

func UpdateCategoryWithAudit(category model.Category, event model.AuditEvent) error {
	return updateCategoryWithAudit(database.DB(), category, event)
}

func updateCategoryWithAudit(db *gorm.DB, category model.Category, event model.AuditEvent) error {
	tx := db.Begin()
	defer rollbackTx(tx)
	result, err := execSQL(tx, `UPDATE categories SET name = ?, enabled = ?, remark = ?, updated_at = ? WHERE id = ?`,
		category.Name, category.Enabled, category.Remark, category.UpdatedAt, category.ID)
	if err != nil {
		if duplicateKey(err) {
			return model.ErrCategoryNameTaken
		}
		return err
	}
	if result == 0 {
		return model.ErrInvalidInput
	}
	if err := appendAuditTx(tx, event); err != nil {
		return err
	}
	return tx.Commit().Error
}

func DeleteCategoryWithAudit(id model.CategoryID, event model.AuditEvent) error {
	return deleteCategoryWithAudit(database.DB(), id, event)
}

func deleteCategoryWithAudit(db *gorm.DB, id model.CategoryID, event model.AuditEvent) error {
	tx := db.Begin()
	defer rollbackTx(tx)
	result, err := execSQL(tx, `DELETE FROM categories WHERE id = ?`, id)
	if err != nil {
		if foreignKeyReferenced(err) {
			return model.ErrCategoryInUse
		}
		return err
	}
	if result == 0 {
		return model.ErrInvalidInput
	}
	if err := appendAuditTx(tx, event); err != nil {
		return err
	}
	return tx.Commit().Error
}

func CategoryHasReferences(id model.CategoryID) (bool, error) {
	return categoryHasReferences(database.DB(), id)
}

func categoryHasReferences(db *gorm.DB, id model.CategoryID) (bool, error) {
	var count int
	err := queryRow(db, `SELECT
        (SELECT COUNT(*) FROM user_category_scopes WHERE category_id = ?) +
		(SELECT COUNT(*) FROM media_account_categories WHERE category_id = ?)`,
		id, id,
	).Scan(&count)
	return count > 0, err
}

func ListCategoryReferenceSummaries() (map[model.CategoryID]model.CategoryReferenceSummary, error) {
	return listCategoryReferenceSummaries(database.DB())
}

func listCategoryReferenceSummaries(db *gorm.DB) (map[model.CategoryID]model.CategoryReferenceSummary, error) {
	rows, err := queryRows(db, `SELECT g.id, COALESCE(scopes.count, 0), COALESCE(accounts.count, 0)
		FROM categories g
		LEFT JOIN (SELECT category_id, COUNT(*) AS count FROM user_category_scopes GROUP BY category_id) scopes ON scopes.category_id = g.id
		LEFT JOIN (SELECT category_id, COUNT(*) AS count FROM media_account_categories GROUP BY category_id) accounts ON accounts.category_id = g.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	summaries := make(map[model.CategoryID]model.CategoryReferenceSummary)
	for rows.Next() {
		var categoryID model.CategoryID
		var summary model.CategoryReferenceSummary
		if err := rows.Scan(&categoryID, &summary.UserScopeCount, &summary.MediaAccountCount); err != nil {
			return nil, err
		}
		summaries[categoryID] = summary
	}
	return summaries, rows.Err()
}

func CategoryReferences(categoryID model.CategoryID) (model.CategoryReferences, error) {
	return categoryReferences(database.DB(), categoryID)
}

func categoryReferences(db *gorm.DB, categoryID model.CategoryID) (model.CategoryReferences, error) {
	references := model.CategoryReferences{CategoryID: categoryID, Users: []model.CategoryReferenceUser{}, MediaAccounts: []model.CategoryReferenceAccount{}}
	userRows, err := queryRows(db, `SELECT u.id, u.username, u.role, u.team_id, COALESCE(t.name, '')
		FROM user_category_scopes scopes
		JOIN users u ON u.id = scopes.user_id
		LEFT JOIN operation_teams t ON t.id = u.team_id
		WHERE scopes.category_id = ? ORDER BY u.id`, categoryID)
	if err != nil {
		return model.CategoryReferences{}, err
	}
	defer userRows.Close()
	for userRows.Next() {
		var reference model.CategoryReferenceUser
		var teamID sql.NullInt64
		if err := userRows.Scan(&reference.UserID, &reference.Username, &reference.Role, &teamID, &reference.TeamName); err != nil {
			return model.CategoryReferences{}, err
		}
		if teamID.Valid {
			value := model.TeamID(teamID.Int64)
			reference.TeamID = &value
		}
		references.Users = append(references.Users, reference)
	}
	if err := userRows.Err(); err != nil {
		return model.CategoryReferences{}, err
	}
	accountRows, err := queryRows(db, `SELECT ma.id, COALESCE(ma.name, ''), ma.platform, ma.user_id, u.username
		FROM media_account_categories relations
		JOIN media_accounts ma ON ma.id = relations.media_account_id
		JOIN users u ON u.id = ma.user_id
		WHERE relations.category_id = ? ORDER BY ma.id`, categoryID)
	if err != nil {
		return model.CategoryReferences{}, err
	}
	defer accountRows.Close()
	for accountRows.Next() {
		var reference model.CategoryReferenceAccount
		if err := accountRows.Scan(&reference.AccountID, &reference.Name, &reference.Platform, &reference.UserID, &reference.Username); err != nil {
			return model.CategoryReferences{}, err
		}
		references.MediaAccounts = append(references.MediaAccounts, reference)
	}
	return references, accountRows.Err()
}

func UserHasCategoryReferencesOutsideScope(userID model.UserID, categoryIDs []model.CategoryID) (bool, error) {
	return userHasCategoryReferencesOutsideScope(database.DB(), userID, categoryIDs)
}

func userHasCategoryReferencesOutsideScope(db *gorm.DB, userID model.UserID, categoryIDs []model.CategoryID) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM media_accounts accounts JOIN media_account_categories relations ON relations.media_account_id = accounts.id WHERE accounts.user_id = ?`
	args := []any{userID}
	if len(categoryIDs) > 0 {
		query += ` AND relations.category_id NOT IN (` + strings.TrimRight(strings.Repeat("?,", len(categoryIDs)), ",") + `)`
		for _, categoryID := range categoryIDs {
			args = append(args, categoryID)
		}
	}
	query += `)`
	var exists bool
	err := queryRow(db, query, args...).Scan(&exists)
	return exists, err
}

func CreateSession(session model.Session, replaceSessionID string) (model.SessionCreateResult, error) {
	return createSession(database.DB(), session, replaceSessionID)
}

func createSession(db *gorm.DB, session model.Session, replaceSessionID string) (model.SessionCreateResult, error) {
	tx := db.Begin()
	defer rollbackTx(tx)
	result := model.SessionCreateResult{}

	if session.ClientType == model.ClientTypeDesktop {
		// The user row is the per-user mutex for Desktop conflict resolution.
		// It is locked, not modified: no authentication state is stored on users.
		var lockedUserID model.UserID
		if err := queryRow(tx, `SELECT id FROM users WHERE id = ? FOR UPDATE`, session.UserID).Scan(&lockedUserID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return model.SessionCreateResult{}, model.ErrAuthenticationFailed
			}
			return model.SessionCreateResult{}, err
		}
		active, err := activeDesktopSessions(tx, session.UserID, session.CreatedAt, true)
		if err != nil {
			return model.SessionCreateResult{}, err
		}
		if len(active) > 1 {
			return model.SessionCreateResult{}, model.ErrSessionStateInvalid
		}
		if len(active) == 1 {
			if replaceSessionID != active[0].ID {
				return model.SessionCreateResult{}, &model.DesktopLoginConflictError{ActiveSessionID: active[0].ID}
			}
			if _, err := execSQL(tx,
				`UPDATE user_sessions SET invalidated_at = ?, invalidated_reason = ? WHERE id = ? AND invalidated_at IS NULL`,
				session.CreatedAt, model.SessionInvalidReasonDesktopReplaced, active[0].ID,
			); err != nil {
				return model.SessionCreateResult{}, err
			}
			result.ReplacedSessionID = active[0].ID
		}
	}

	if err := insertSession(tx, session); err != nil {
		return model.SessionCreateResult{}, err
	}
	if err := tx.Commit().Error; err != nil {
		return model.SessionCreateResult{}, err
	}
	return result, nil
}

func insertSession(db *gorm.DB, session model.Session) error {
	_, err := execSQL(db,
		`INSERT INTO user_sessions (id, user_id, token_hash, client_type, created_at, expires_at, invalidated_at, invalidated_reason) VALUES (?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''))`,
		session.ID, session.UserID, session.TokenHash, session.ClientType, session.CreatedAt, session.ExpiresAt, session.InvalidAt, session.InvalidatedReason,
	)
	return err
}

func activeDesktopSessions(db *gorm.DB, userID model.UserID, at time.Time, forUpdate bool) ([]model.Session, error) {
	query := `SELECT id, user_id, token_hash, client_type, created_at, expires_at, invalidated_at, invalidated_reason
		FROM user_sessions
		WHERE user_id = ? AND client_type = ? AND invalidated_at IS NULL AND expires_at > ?
		ORDER BY created_at DESC, id DESC`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	rows, err := queryRows(db, query, userID, model.ClientTypeDesktop, at)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sessions []model.Session
	for rows.Next() {
		session, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

func FindSessionByTokenHash(tokenHash string) (model.Session, bool, error) {
	return findSessionByTokenHash(database.DB(), tokenHash)
}

func findSessionByTokenHash(db *gorm.DB, tokenHash string) (model.Session, bool, error) {
	row := queryRow(db,
		`SELECT id, user_id, token_hash, client_type, created_at, expires_at, invalidated_at, invalidated_reason FROM user_sessions WHERE token_hash = ?`,
		tokenHash,
	)
	session, err := scanSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Session{}, false, nil
	}
	if err != nil {
		return model.Session{}, false, err
	}
	return session, true, nil
}

func FindSessionByID(sessionID string) (model.Session, bool, error) {
	return findSessionByID(database.DB(), sessionID)
}

func findSessionByID(db *gorm.DB, sessionID string) (model.Session, bool, error) {
	row := queryRow(db,
		`SELECT id, user_id, token_hash, client_type, created_at, expires_at, invalidated_at, invalidated_reason FROM user_sessions WHERE id = ?`,
		sessionID,
	)
	session, err := scanSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Session{}, false, nil
	}
	if err != nil {
		return model.Session{}, false, err
	}
	return session, true, nil
}

func FindActiveDesktopSession(userID model.UserID, at time.Time) (model.Session, bool, error) {
	return findActiveDesktopSession(database.DB(), userID, at)
}

func findActiveDesktopSession(db *gorm.DB, userID model.UserID, at time.Time) (model.Session, bool, error) {
	sessions, err := activeDesktopSessions(db, userID, at, false)
	if err != nil {
		return model.Session{}, false, err
	}
	if len(sessions) > 1 {
		return model.Session{}, false, model.ErrSessionStateInvalid
	}
	if len(sessions) == 0 {
		return model.Session{}, false, nil
	}
	return sessions[0], true, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSession(row rowScanner) (model.Session, error) {
	var session model.Session
	var clientType string
	var invalidatedAt sql.NullTime
	var invalidatedReason sql.NullString
	if err := row.Scan(
		&session.ID, &session.UserID, &session.TokenHash, &clientType, &session.CreatedAt,
		&session.ExpiresAt, &invalidatedAt, &invalidatedReason,
	); err != nil {
		return model.Session{}, err
	}
	session.ClientType = model.ClientType(clientType)
	if invalidatedAt.Valid {
		at := invalidatedAt.Time
		session.InvalidAt = &at
	}
	if invalidatedReason.Valid {
		session.InvalidatedReason = invalidatedReason.String
	}
	return session, nil
}

func InvalidateUserSessions(userID model.UserID, at time.Time, reason string) error {
	return invalidateUserSessions(database.DB(), userID, at, reason)
}

func invalidateUserSessions(db *gorm.DB, userID model.UserID, at time.Time, reason string) error {
	_, err := execSQL(db,
		`UPDATE user_sessions SET invalidated_at = ?, invalidated_reason = ? WHERE user_id = ? AND invalidated_at IS NULL`,
		at, reason, userID,
	)
	return err
}

func InvalidateUserSessionsWithAudit(userID model.UserID, at time.Time, reason string, event model.AuditEvent) error {
	return invalidateUserSessionsWithAudit(database.DB(), userID, at, reason, event)
}

func invalidateUserSessionsWithAudit(db *gorm.DB, userID model.UserID, at time.Time, reason string, event model.AuditEvent) error {
	tx := db.Begin()
	defer rollbackTx(tx)
	if _, err := execSQL(tx,
		`UPDATE user_sessions SET invalidated_at = ?, invalidated_reason = ? WHERE user_id = ? AND invalidated_at IS NULL`,
		at, reason, userID,
	); err != nil {
		return err
	}
	if err := appendAuditTx(tx, event); err != nil {
		return err
	}
	return tx.Commit().Error
}

func InvalidateSessionByID(sessionID string, at time.Time, reason string) error {
	return invalidateSessionByID(database.DB(), sessionID, at, reason)
}

func invalidateSessionByID(db *gorm.DB, sessionID string, at time.Time, reason string) error {
	_, err := execSQL(db,
		`UPDATE user_sessions SET invalidated_at = ?, invalidated_reason = ? WHERE id = ? AND invalidated_at IS NULL`,
		at, reason, sessionID,
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
	rows, err := queryRows(db, "SELECT u.id, u.username, COALESCE(u.nickname, u.username), COALESCE(u.avatar_id, 'sky'), u.password_hash, u.role, u.status, u.team_id, COALESCE(t.name, ''), u.created_at, u.updated_at FROM users u LEFT JOIN operation_teams t ON t.id = u.team_id ORDER BY u.created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []model.User
	for rows.Next() {
		var u model.User
		var teamID sql.NullInt64
		if err := rows.Scan(&u.ID, &u.Username, &u.Nickname, &u.AvatarID, &u.PasswordHash, &u.Role, &u.Status, &teamID, &u.TeamName, &u.CreatedAt, &u.UpdatedAt); err != nil {
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
		categoryIDs, err := findCategoryIDs(db, users[i].ID)
		if err != nil {
			return nil, err
		}
		users[i].CategoryIDs = categoryIDs
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
