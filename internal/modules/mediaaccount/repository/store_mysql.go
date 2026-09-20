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
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/id"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
	"gorm.io/gorm"
)

const accountColumns = `id, user_id, team_id, platform, platform_account_id, name, avatar_url, browser_profile_id, remark, identification_status, duplicate_of_account_id, business_status, login_status, original_cookie, active_cookie, cookie_status, active_cookie_updated_at, last_checked_at, check_items, created_at, updated_at`

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

func lastInsertID(db *gorm.DB) (int64, error) {
	var inserted int64
	err := queryRow(db, "SELECT LAST_INSERT_ID()").Scan(&inserted)
	return inserted, err
}

func rollbackTx(tx *gorm.DB) {
	_ = tx.Rollback().Error
}

func Create(record model.AccountRecord) (string, error) {
	return create(database.DB(), record)
}

func create(db *gorm.DB, record model.AccountRecord) (string, error) {
	tx := db.Begin()
	defer rollbackTx(tx)
	_, err := execSQL(tx,
		`INSERT INTO media_accounts (`+accountColumns+`) VALUES (NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.UserID, record.TeamID, record.Platform,
		nullIfEmpty(record.PlatformAccountID), nullIfEmpty(record.Name), nullIfEmpty(record.AvatarURL), nullIfEmpty(record.BrowserProfileID),
		nullIfEmpty(record.Remark), record.IdentificationStatus, nullIfEmpty(record.DuplicateOfAccountID), record.BusinessStatus, record.LoginStatus,
		nullIfEmpty(record.OriginalCookie), nullIfEmpty(record.ActiveCookie), nullIfEmpty(record.CookieStatus),
		record.ActiveCookieUpdatedAt, record.LastCheckedAt, marshalCheckItems(record.CheckItems), record.CreatedAt, record.UpdatedAt,
	)
	if duplicateKey(err) {
		return "", model.ErrDuplicateAccount
	}
	if err != nil {
		return "", err
	}
	id, err := lastInsertID(db)
	if err != nil {
		return "", err
	}
	accountID := strconv.FormatInt(id, 10)
	if err := replaceAccountGameIDs(tx, accountID, record.GameIDs); err != nil {
		return "", err
	}
	if err := tx.Commit().Error; err != nil {
		return "", err
	}
	return accountID, nil
}

func Find(id string) (model.AccountRecord, bool, error) {
	return findByID(database.DB(), id)
}

func FindByID(id string) (model.AccountRecord, bool, error) {
	return findByID(database.DB(), id)
}

func findByID(db *gorm.DB, id string) (model.AccountRecord, bool, error) {
	return findByQuery(db, `SELECT `+accountColumns+` FROM media_accounts WHERE id = ?`, id)
}

func FindByIdentity(userID sharedidentity.UserID, platform model.Platform, platformAccountID string) (model.AccountRecord, bool, error) {
	return findByIdentity(database.DB(), userID, platform, platformAccountID)
}

func findByIdentity(db *gorm.DB, userID sharedidentity.UserID, platform model.Platform, platformAccountID string) (model.AccountRecord, bool, error) {
	return findByQuery(db,
		`SELECT `+accountColumns+` FROM media_accounts WHERE user_id = ? AND platform = ? AND platform_account_id = ?`,
		userID, platform, platformAccountID,
	)
}

func FindByProfilePlatform(profileID string, platform model.Platform) (model.AccountRecord, bool, error) {
	return findByProfilePlatform(database.DB(), profileID, platform)
}

func findByProfilePlatform(db *gorm.DB, profileID string, platform model.Platform) (model.AccountRecord, bool, error) {
	return findByQuery(db,
		`SELECT `+accountColumns+` FROM media_accounts WHERE browser_profile_id = ? AND platform = ? LIMIT 1`,
		profileID, platform,
	)
}

func findByQuery(db *gorm.DB, query string, args ...any) (model.AccountRecord, bool, error) {
	record, err := scanAccount(queryRow(db, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return model.AccountRecord{}, false, nil
	}
	if err != nil {
		return model.AccountRecord{}, false, err
	}
	records := []model.AccountRecord{record}
	if err := loadGameIDs(db, records); err != nil {
		return model.AccountRecord{}, false, err
	}
	return records[0], true, nil
}

func Update(record model.AccountRecord, replaceGameIDs *[]string) error {
	return update(database.DB(), record, replaceGameIDs)
}

func update(db *gorm.DB, record model.AccountRecord, replaceGameIDs *[]string) error {
	tx := db.Begin()
	defer rollbackTx(tx)
	result, err := execSQL(tx,
		`UPDATE media_accounts SET user_id = ?, team_id = ?, platform = ?, platform_account_id = ?, name = ?, avatar_url = ?, browser_profile_id = ?, remark = ?, identification_status = ?, duplicate_of_account_id = ?, business_status = ?, login_status = ?, original_cookie = ?, active_cookie = ?, cookie_status = ?, active_cookie_updated_at = ?, last_checked_at = ?, check_items = ?, updated_at = ? WHERE id = ?`,
		record.UserID, record.TeamID, record.Platform, nullIfEmpty(record.PlatformAccountID), nullIfEmpty(record.Name),
		nullIfEmpty(record.AvatarURL), nullIfEmpty(record.BrowserProfileID), nullIfEmpty(record.Remark), record.IdentificationStatus,
		nullIfEmpty(record.DuplicateOfAccountID), record.BusinessStatus, record.LoginStatus,
		nullIfEmpty(record.OriginalCookie), nullIfEmpty(record.ActiveCookie), nullIfEmpty(record.CookieStatus),
		record.ActiveCookieUpdatedAt, record.LastCheckedAt, marshalCheckItems(record.CheckItems), record.UpdatedAt, record.ID,
	)
	if duplicateKey(err) {
		return model.ErrDuplicateAccount
	}
	if err != nil {
		return err
	}
	if result == 0 {
		return model.ErrNotFound
	}
	if replaceGameIDs != nil {
		if err := replaceAccountGameIDs(tx, record.ID, *replaceGameIDs); err != nil {
			return err
		}
	}
	return tx.Commit().Error
}

func List(query dto.AccountFilter) ([]model.AccountRecord, error) {
	return list(database.DB(), query)
}

func list(db *gorm.DB, query dto.AccountFilter) ([]model.AccountRecord, error) {
	statement := `SELECT ` + accountColumns + ` FROM media_accounts`
	conditions := make([]string, 0, 3)
	args := make([]any, 0, 3)
	if query.UserID > 0 {
		conditions = append(conditions, "user_id = ?")
		args = append(args, query.UserID)
	}
	if len(query.GameIDs) > 0 {
		conditions = append(conditions, `EXISTS (SELECT 1 FROM media_account_games mag WHERE mag.media_account_id = media_accounts.id AND mag.game_id IN (`+placeholders(len(query.GameIDs))+`))`)
		for _, gameID := range query.GameIDs {
			args = append(args, gameID)
		}
	}
	if query.Platform != "" {
		conditions = append(conditions, "platform = ?")
		args = append(args, query.Platform)
	}
	if query.BusinessStatus != "" {
		conditions = append(conditions, "business_status = ?")
		args = append(args, query.BusinessStatus)
	}
	if query.LoginStatus != "" {
		conditions = append(conditions, "login_status = ?")
		args = append(args, query.LoginStatus)
	}
	if query.Search != "" {
		like := "%" + query.Search + "%"
		conditions = append(conditions, "(id LIKE ? OR platform_account_id LIKE ? OR name LIKE ? OR remark LIKE ?)")
		args = append(args, like, like, like, like)
	}
	if query.ProfileSearch != "" {
		like := "%" + query.ProfileSearch + "%"
		conditions = append(conditions, `EXISTS (SELECT 1 FROM browser_profiles bp WHERE bp.id = media_accounts.browser_profile_id AND (bp.name LIKE ? OR bp.seq LIKE ? OR bp.bit_profile_id LIKE ? OR bp.id LIKE ?))`)
		args = append(args, like, like, like, like)
	}
	if len(conditions) > 0 {
		statement += " WHERE " + strings.Join(conditions, " AND ")
	}
	statement += " ORDER BY created_at, id"

	rows, err := queryRows(db, statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]model.AccountRecord, 0)
	for rows.Next() {
		record, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := loadGameIDs(db, records); err != nil {
		return nil, err
	}
	if len(query.AnyTags) == 0 && len(query.AllTags) == 0 && len(query.ExcludeTags) == 0 {
		return records, nil
	}
	ids := make([]string, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.ID)
	}
	tagsByAccount, err := listTags(db, ids)
	if err != nil {
		return nil, err
	}
	filtered := records[:0]
	for _, record := range records {
		if recordMatchesTags(tagsByAccount[record.ID], query.AnyTags, query.AllTags, query.ExcludeTags) {
			filtered = append(filtered, record)
		}
	}
	return filtered, nil
}

func replaceAccountGameIDs(db *gorm.DB, accountID string, gameIDs []string) error {
	if err := db.Exec(`DELETE FROM media_account_games WHERE media_account_id = ?`, accountID).Error; err != nil {
		return err
	}
	for _, gameID := range gameIDs {
		if err := db.Exec(`INSERT INTO media_account_games (media_account_id, game_id) VALUES (?, ?)`, accountID, gameID).Error; err != nil {
			return err
		}
	}
	return nil
}

func loadGameIDs(db *gorm.DB, records []model.AccountRecord) error {
	if len(records) == 0 {
		return nil
	}
	ids := make([]string, 0, len(records))
	byID := make(map[string]*model.AccountRecord, len(records))
	for index := range records {
		ids = append(ids, records[index].ID)
		byID[records[index].ID] = &records[index]
	}
	args := make([]any, len(ids))
	for index, id := range ids {
		args[index] = id
	}
	rows, err := queryRows(db,
		`SELECT media_account_id, game_id FROM media_account_games WHERE media_account_id IN (`+placeholders(len(ids))+`) ORDER BY media_account_id, game_id`,
		args...,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var accountID, gameID string
		if err := rows.Scan(&accountID, &gameID); err != nil {
			return err
		}
		if record := byID[accountID]; record != nil {
			record.GameIDs = append(record.GameIDs, gameID)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return nil
}

func Delete(id string) error {
	return delete(database.DB(), id)
}

func delete(db *gorm.DB, id string) error {
	return db.Exec(`DELETE FROM media_accounts WHERE id = ?`, id).Error
}

func AddTags(userID sharedidentity.UserID, accountIDs, tags []string, createdAt time.Time) error {
	return addTags(database.DB(), userID, accountIDs, tags, createdAt)
}

func addTags(db *gorm.DB, userID sharedidentity.UserID, accountIDs, tags []string, createdAt time.Time) error {
	tx := db.Begin()
	defer rollbackTx(tx)
	for _, accountID := range accountIDs {
		for _, tag := range tags {
			if _, err := execSQL(tx,
				`INSERT IGNORE INTO media_account_tags (id, user_id, media_account_id, tag_name, created_at) VALUES (?, ?, ?, ?, ?)`,
				id.NewID("media_account_tag"), userID, accountID, tag, createdAt,
			); err != nil {
				return err
			}
		}
	}
	return tx.Commit().Error
}

func RemoveTags(userID sharedidentity.UserID, accountIDs, tags []string) error {
	return removeTags(database.DB(), userID, accountIDs, tags)
}

func removeTags(db *gorm.DB, userID sharedidentity.UserID, accountIDs, tags []string) error {
	if len(accountIDs) == 0 || len(tags) == 0 {
		return nil
	}
	args := make([]any, 0, 1+len(accountIDs)+len(tags))
	args = append(args, userID)
	for _, accountID := range accountIDs {
		args = append(args, accountID)
	}
	for _, tag := range tags {
		args = append(args, tag)
	}
	_, err := execSQL(db,
		`DELETE FROM media_account_tags WHERE user_id = ? AND media_account_id IN (`+placeholders(len(accountIDs))+`) AND tag_name IN (`+placeholders(len(tags))+`)`,
		args...,
	)
	return err
}

func ListTags(accountIDs []string) (map[string][]string, error) {
	return listTags(database.DB(), accountIDs)
}

func listTags(db *gorm.DB, accountIDs []string) (map[string][]string, error) {
	result := make(map[string][]string, len(accountIDs))
	if len(accountIDs) == 0 {
		return result, nil
	}
	args := make([]any, len(accountIDs))
	for i, accountID := range accountIDs {
		args[i] = accountID
	}
	rows, err := queryRows(db,
		`SELECT media_account_id, tag_name FROM media_account_tags WHERE media_account_id IN (`+placeholders(len(accountIDs))+`) ORDER BY media_account_id, tag_name`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var accountID, tag string
		if err := rows.Scan(&accountID, &tag); err != nil {
			return nil, err
		}
		result[accountID] = append(result[accountID], tag)
	}
	return result, rows.Err()
}

func AppendAudit(event sharedidentity.AuditEvent) error {
	return appendAudit(database.DB(), event)
}

func appendAudit(db *gorm.DB, event sharedidentity.AuditEvent) error {
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

type scanner interface {
	Scan(dest ...any) error
}

func scanAccount(row scanner) (model.AccountRecord, error) {
	var record model.AccountRecord
	var platformAccountID, name, avatarURL, browserProfileID, remark sql.NullString
	var duplicateOfAccountID, originalCookie, activeCookie, cookieStatus sql.NullString
	var activeCookieUpdatedAt, lastCheckedAt sql.NullTime
	var checkItemsJSON sql.NullString
	err := row.Scan(
		&record.ID, &record.UserID, &record.TeamID, &record.Platform,
		&platformAccountID, &name, &avatarURL, &browserProfileID,
		&remark, &record.IdentificationStatus, &duplicateOfAccountID, &record.BusinessStatus, &record.LoginStatus,
		&originalCookie, &activeCookie, &cookieStatus, &activeCookieUpdatedAt, &lastCheckedAt,
		&checkItemsJSON, &record.CreatedAt, &record.UpdatedAt,
	)
	if err != nil {
		return model.AccountRecord{}, err
	}
	record.PlatformAccountID = platformAccountID.String
	record.Name = name.String
	record.AvatarURL = avatarURL.String
	record.BrowserProfileID = browserProfileID.String
	record.Remark = remark.String
	record.DuplicateOfAccountID = duplicateOfAccountID.String
	record.OriginalCookie = originalCookie.String
	record.ActiveCookie = activeCookie.String
	record.CookieStatus = cookieStatus.String
	if checkItemsJSON.Valid && checkItemsJSON.String != "" {
		_ = json.Unmarshal([]byte(checkItemsJSON.String), &record.CheckItems)
	}
	if activeCookieUpdatedAt.Valid {
		value := activeCookieUpdatedAt.Time
		record.ActiveCookieUpdatedAt = &value
	}
	if lastCheckedAt.Valid {
		value := lastCheckedAt.Time
		record.LastCheckedAt = &value
	}
	return record, nil
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func placeholders(count int) string {
	if count <= 0 {
		return ""
	}
	return strings.TrimRight(strings.Repeat("?, ", count), ", ")
}

func duplicateKey(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

func recordMatchesTags(tags, anyTags, allTags, excludeTags []string) bool {
	present := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		present[tag] = struct{}{}
	}
	if len(anyTags) > 0 {
		matched := false
		for _, tag := range anyTags {
			if _, ok := present[tag]; ok {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	for _, tag := range allTags {
		if _, ok := present[tag]; !ok {
			return false
		}
	}
	for _, tag := range excludeTags {
		if _, ok := present[tag]; ok {
			return false
		}
	}
	return true
}

const accountGroupColumns = `id, user_id, team_id, name, filters, sort_order, created_at, updated_at`

func CreateGroup(group model.AccountGroup) (string, error) {
	return createGroup(database.DB(), group)
}

func createGroup(db *gorm.DB, group model.AccountGroup) (string, error) {
	filtersJSON, err := json.Marshal(group.Filters)
	if err != nil {
		return "", err
	}
	_, err = execSQL(db,
		`INSERT INTO account_groups (user_id, team_id, name, filters, sort_order, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		group.UserID, group.TeamID, group.Name, string(filtersJSON), group.SortOrder, group.CreatedAt, group.UpdatedAt,
	)
	if err != nil {
		return "", err
	}
	id, err := lastInsertID(db)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(id, 10), nil
}

func FindGroup(id string) (model.AccountGroup, bool, error) {
	return findGroup(database.DB(), id)
}

func findGroup(db *gorm.DB, id string) (model.AccountGroup, bool, error) {
	group, err := scanAccountGroup(queryRow(db, `SELECT `+accountGroupColumns+` FROM account_groups WHERE id = ?`, id))
	if err != nil {
		return model.AccountGroup{}, false, err
	}
	return group, group.ID != "", nil
}

func ListGroups(userID sharedidentity.UserID) ([]model.AccountGroup, error) {
	return listGroups(database.DB(), userID)
}

func listGroups(db *gorm.DB, userID sharedidentity.UserID) ([]model.AccountGroup, error) {
	rows, err := queryRows(db, `SELECT `+accountGroupColumns+` FROM account_groups WHERE user_id = ? ORDER BY sort_order ASC, name ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groups := []model.AccountGroup{}
	for rows.Next() {
		group, err := scanAccountGroup(rows)
		if err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func UpdateGroup(group model.AccountGroup) error {
	return updateGroup(database.DB(), group)
}

func updateGroup(db *gorm.DB, group model.AccountGroup) error {
	filtersJSON, err := json.Marshal(group.Filters)
	if err != nil {
		return err
	}
	_, err = execSQL(db,
		`UPDATE account_groups SET name = ?, filters = ?, sort_order = ?, updated_at = ? WHERE id = ?`,
		group.Name, string(filtersJSON), group.SortOrder, group.UpdatedAt, group.ID,
	)
	return err
}

func DeleteGroup(id string) error {
	return deleteGroup(database.DB(), id)
}

func deleteGroup(db *gorm.DB, id string) error {
	_, err := execSQL(db, `DELETE FROM account_groups WHERE id = ?`, id)
	return err
}

func scanAccountGroup(row interface{ Scan(...any) error }) (model.AccountGroup, error) {
	var group model.AccountGroup
	var filtersJSON []byte
	if err := row.Scan(&group.ID, &group.UserID, &group.TeamID, &group.Name, &filtersJSON, &group.SortOrder, &group.CreatedAt, &group.UpdatedAt); err != nil {
		return model.AccountGroup{}, err
	}
	if len(filtersJSON) > 0 {
		if err := json.Unmarshal(filtersJSON, &group.Filters); err != nil {
			return model.AccountGroup{}, err
		}
	}
	return group, nil
}

func marshalCheckItems(items []model.AccountCheckItem) any {
	if len(items) == 0 {
		return nil
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return nil
	}
	return string(raw)
}
