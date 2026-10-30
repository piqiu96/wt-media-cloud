package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/model"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
	"gorm.io/gorm"
)

// accountColumns is the INSERT column list; reads use accountReadColumns, which
// qualifies every column and adds the owner name.
const accountColumns = `id, user_id, team_id, platform, platform_account_id, platform_name, platform_avatar_url, browser_profile_id, remark, identification_status, duplicate_of_account_id, enabled, platform_login_status, original_cookie, active_cookie, cookie_status, cookie_updated_at, last_check_at, last_check_items, created_at, updated_at`

// accountReadColumns must stay in the same order as scanAccount's destinations.
// The owner name is resolved by the join rather than stored, so a renamed user
// shows the new name and an account whose user row is gone shows an empty one.
const accountReadColumns = `ma.id, ma.user_id, ma.team_id, ma.platform, ma.platform_account_id, ma.platform_name, ma.platform_avatar_url, ma.browser_profile_id, ma.remark, ma.identification_status, ma.duplicate_of_account_id, ma.enabled, ma.platform_login_status, ma.original_cookie, ma.active_cookie, ma.cookie_status, ma.cookie_updated_at, ma.last_check_at, ma.last_check_items, ma.created_at, ma.updated_at, COALESCE(NULLIF(u.nickname, ''), u.username, ''), ma.last_check_status, ma.last_check_message, ma.last_check_identity, ma.check_operation_id, ma.check_profile_id, ma.check_started_at, ma.before_cookie, ma.cookie_update`

const accountReadFrom = ` FROM media_accounts ma LEFT JOIN users u ON u.id = ma.user_id`

// accountSummaryColumns deliberately excludes original_cookie and active_cookie.
// List and summary endpoints have no use for those large secrets.
const accountSummaryColumns = `ma.id, ma.user_id, ma.team_id, ma.platform, ma.platform_account_id, ma.platform_name, ma.platform_avatar_url, ma.browser_profile_id, ma.remark, ma.identification_status, ma.duplicate_of_account_id, ma.enabled, ma.platform_login_status, ma.cookie_status, ma.cookie_updated_at, ma.last_check_at, ma.last_check_items, ma.created_at, ma.updated_at, COALESCE(NULLIF(u.nickname, ''), u.username, ''), ma.last_check_status, ma.last_check_message, ma.last_check_identity, ma.check_operation_id, ma.check_profile_id, ma.check_started_at`

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
	id, err := createInTx(tx, record)
	if err != nil {
		return "", err
	}
	if err := tx.Commit().Error; err != nil {
		return "", err
	}
	return id, nil
}

func createInTx(tx *gorm.DB, record model.AccountRecord) (string, error) {
	_, err := execSQL(tx,
		`INSERT INTO media_accounts (`+accountColumns+`) VALUES (NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.UserID, record.TeamID, record.Platform,
		nullIfEmpty(record.PlatformAccountID), nullIfEmpty(record.PlatformName), nullIfEmpty(record.PlatformAvatarURL), nullIfEmpty(record.BrowserProfileID),
		nullIfEmpty(record.Remark), record.IdentificationStatus, nullIfEmpty(record.DuplicateOfAccountID), record.Enabled, record.PlatformLoginStatus,
		nullIfEmpty(record.OriginalCookie), nullIfEmpty(record.ActiveCookie), nullIfEmpty(record.CookieStatus),
		record.CookieUpdatedAt, record.LastCheckAt, marshalLastCheckItems(record.LastCheckItems), record.CreatedAt, record.UpdatedAt,
	)
	if duplicateKey(err) {
		return "", model.ErrDuplicateAccount
	}
	if err != nil {
		return "", err
	}
	id, err := lastInsertID(tx)
	if err != nil {
		return "", err
	}
	accountID := strconv.FormatInt(id, 10)
	if err := replaceAccountCategoryIDs(tx, accountID, record.CategoryIDs); err != nil {
		return "", err
	}
	if len(record.Tags) > 0 {
		if err := replaceAccountTags(tx, accountID, record.Tags, record.CreatedAt); err != nil {
			return "", err
		}
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
	return findByQuery(db, `SELECT `+accountReadColumns+accountReadFrom+` WHERE ma.id = ?`, id)
}

func FindByIdentity(userID sharedidentity.UserID, platform model.Platform, platformAccountID string) (model.AccountRecord, bool, error) {
	return findByIdentity(database.DB(), userID, platform, platformAccountID)
}

func findByIdentity(db *gorm.DB, userID sharedidentity.UserID, platform model.Platform, platformAccountID string) (model.AccountRecord, bool, error) {
	return findByQuery(db,
		`SELECT `+accountReadColumns+accountReadFrom+` WHERE ma.user_id = ? AND ma.platform = ? AND ma.platform_account_id = ?`,
		userID, platform, platformAccountID,
	)
}

func FindByProfilePlatform(profileID string, platform model.Platform) (model.AccountRecord, bool, error) {
	return findByProfilePlatform(database.DB(), profileID, platform)
}

func findByProfilePlatform(db *gorm.DB, profileID string, platform model.Platform) (model.AccountRecord, bool, error) {
	return findByQuery(db,
		`SELECT `+accountReadColumns+accountReadFrom+` WHERE ma.browser_profile_id = ? AND ma.platform = ? LIMIT 1`,
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
	if err := loadCategoryIDs(db, records); err != nil {
		return model.AccountRecord{}, false, err
	}
	return records[0], true, nil
}

func List(query AccountFilter) ([]model.AccountRecord, error) {
	return list(database.DB(), query)
}

func list(db *gorm.DB, query AccountFilter) ([]model.AccountRecord, error) {
	where, args := accountConditions(query)
	statement := `SELECT ` + accountSummaryColumns + accountReadFrom + where + " ORDER BY ma.created_at DESC, ma.id DESC"
	if query.Limit > 0 {
		statement += " LIMIT ? OFFSET ?"
		args = append(args, query.Limit, query.Offset)
	}
	rows, err := queryRows(db, statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]model.AccountRecord, 0)
	for rows.Next() {
		record, err := scanAccountSummary(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := loadCategoryIDs(db, records); err != nil {
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

func replaceAccountCategoryIDs(db *gorm.DB, accountID string, categoryIDs []sharedidentity.CategoryID) error {
	if err := db.Exec(`DELETE FROM media_account_categories WHERE media_account_id = ?`, accountID).Error; err != nil {
		return err
	}
	for _, categoryID := range categoryIDs {
		if err := db.Exec(`INSERT INTO media_account_categories (media_account_id, category_id) VALUES (?, ?)`, accountID, categoryID).Error; err != nil {
			return err
		}
	}
	return nil
}

func loadCategoryIDs(db *gorm.DB, records []model.AccountRecord) error {
	if len(records) == 0 {
		return nil
	}
	for index := range records {
		records[index].CategoryIDs = []sharedidentity.CategoryID{}
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
		`SELECT media_account_id, category_id FROM media_account_categories WHERE media_account_id IN (`+placeholders(len(ids))+`) ORDER BY media_account_id, category_id`,
		args...,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var accountID string
		var categoryID sharedidentity.CategoryID
		if err := rows.Scan(&accountID, &categoryID); err != nil {
			return err
		}
		if record := byID[accountID]; record != nil {
			record.CategoryIDs = append(record.CategoryIDs, categoryID)
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

func scanAccountSummary(row scanner) (model.AccountRecord, error) {
	var record model.AccountRecord
	var platformAccountID, platformName, platformAvatarURL, browserProfileID, remark sql.NullString
	var duplicateOfAccountID, cookieStatus, userName sql.NullString
	var cookieUpdatedAt, lastCheckAt sql.NullTime
	var lastCheckItemsJSON sql.NullString
	var checkMessage, checkIdentity, checkOperation, checkProfile sql.NullString
	var checkStarted sql.NullTime
	err := row.Scan(
		&record.ID, &record.UserID, &record.TeamID, &record.Platform,
		&platformAccountID, &platformName, &platformAvatarURL, &browserProfileID,
		&remark, &record.IdentificationStatus, &duplicateOfAccountID, &record.Enabled, &record.PlatformLoginStatus,
		&cookieStatus, &cookieUpdatedAt, &lastCheckAt, &lastCheckItemsJSON,
		&record.CreatedAt, &record.UpdatedAt, &userName,
		&record.LastCheckStatus, &checkMessage, &checkIdentity, &checkOperation, &checkProfile, &checkStarted,
	)
	if err != nil {
		return model.AccountRecord{}, err
	}
	record.PlatformAccountID = platformAccountID.String
	record.PlatformName = platformName.String
	record.PlatformAvatarURL = platformAvatarURL.String
	record.BrowserProfileID = browserProfileID.String
	record.Remark = remark.String
	record.DuplicateOfAccountID = duplicateOfAccountID.String
	record.CookieStatus = cookieStatus.String
	record.UserName = userName.String
	record.LastCheckMessage = checkMessage.String
	record.CheckOperationID = checkOperation.String
	record.CheckProfileID = checkProfile.String
	if checkIdentity.Valid {
		if err := json.Unmarshal([]byte(checkIdentity.String), &record.LastCheckIdentity); err != nil {
			return model.AccountRecord{}, err
		}
	}
	if checkStarted.Valid {
		value := checkStarted.Time
		record.CheckStartedAt = &value
	}
	if lastCheckItemsJSON.Valid && lastCheckItemsJSON.String != "" {
		_ = json.Unmarshal([]byte(lastCheckItemsJSON.String), &record.LastCheckItems)
	}
	if cookieUpdatedAt.Valid {
		value := cookieUpdatedAt.Time
		record.CookieUpdatedAt = &value
	}
	if lastCheckAt.Valid {
		value := lastCheckAt.Time
		record.LastCheckAt = &value
	}
	return record, nil
}

func scanAccount(row scanner) (model.AccountRecord, error) {
	var record model.AccountRecord
	var beforeCookie, cookieUpdate sql.NullString
	var platformAccountID, name, avatarURL, browserProfileID, remark sql.NullString
	var duplicateOfAccountID, originalCookie, activeCookie, cookieStatus sql.NullString
	var activeCookieUpdatedAt, lastCheckedAt sql.NullTime
	var checkItemsJSON, userName sql.NullString
	var checkMessage, checkIdentity, checkOperation, checkProfile sql.NullString
	var checkStarted sql.NullTime
	err := row.Scan(
		&record.ID, &record.UserID, &record.TeamID, &record.Platform,
		&platformAccountID, &name, &avatarURL, &browserProfileID,
		&remark, &record.IdentificationStatus, &duplicateOfAccountID, &record.Enabled, &record.PlatformLoginStatus,
		&originalCookie, &activeCookie, &cookieStatus, &activeCookieUpdatedAt, &lastCheckedAt,
		&checkItemsJSON, &record.CreatedAt, &record.UpdatedAt, &userName,
		&record.LastCheckStatus, &checkMessage, &checkIdentity, &checkOperation, &checkProfile, &checkStarted, &beforeCookie, &cookieUpdate,
	)
	if err != nil {
		return model.AccountRecord{}, err
	}
	record.PlatformAccountID = platformAccountID.String
	record.PlatformName = name.String
	record.PlatformAvatarURL = avatarURL.String
	record.BrowserProfileID = browserProfileID.String
	record.Remark = remark.String
	record.DuplicateOfAccountID = duplicateOfAccountID.String
	record.OriginalCookie = originalCookie.String
	record.ActiveCookie = activeCookie.String
	record.BeforeCookie = beforeCookie.String
	if cookieUpdate.Valid {
		if err := json.Unmarshal([]byte(cookieUpdate.String), &record.CookieUpdate); err != nil {
			return model.AccountRecord{}, err
		}
	}
	record.CookieStatus = cookieStatus.String
	record.UserName = userName.String
	record.LastCheckMessage = checkMessage.String
	record.CheckOperationID = checkOperation.String
	record.CheckProfileID = checkProfile.String
	if checkIdentity.Valid {
		if err := json.Unmarshal([]byte(checkIdentity.String), &record.LastCheckIdentity); err != nil {
			return model.AccountRecord{}, err
		}
	}
	if checkStarted.Valid {
		value := checkStarted.Time
		record.CheckStartedAt = &value
	}
	if checkItemsJSON.Valid && checkItemsJSON.String != "" {
		_ = json.Unmarshal([]byte(checkItemsJSON.String), &record.LastCheckItems)
	}
	if activeCookieUpdatedAt.Valid {
		value := activeCookieUpdatedAt.Time
		record.CookieUpdatedAt = &value
	}
	if lastCheckedAt.Valid {
		value := lastCheckedAt.Time
		record.LastCheckAt = &value
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
func marshalLastCheckItems(items []model.AccountCheckItem) any {
	if len(items) == 0 {
		return nil
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return nil
	}
	return string(raw)
}

func accountConditions(query AccountFilter) (string, []any) {
	conditions := make([]string, 0, 3)
	args := make([]any, 0, 3)
	if query.UserID > 0 {
		conditions = append(conditions, "ma.user_id = ?")
		args = append(args, query.UserID)
	}
	if len(query.CategoryIDs) > 0 {
		conditions = append(conditions, `EXISTS (SELECT 1 FROM media_account_categories mag WHERE mag.media_account_id = ma.id AND mag.category_id IN (`+placeholders(len(query.CategoryIDs))+`))`)
		for _, categoryID := range query.CategoryIDs {
			args = append(args, categoryID)
		}
	}
	if query.Platform != "" {
		conditions = append(conditions, "ma.platform = ?")
		args = append(args, query.Platform)
	}
	if query.Enabled != nil {
		conditions = append(conditions, "ma.enabled = ?")
		args = append(args, *query.Enabled)
	}
	if query.PlatformLoginStatus != "" {
		conditions = append(conditions, "ma.platform_login_status = ?")
		args = append(args, query.PlatformLoginStatus)
	}
	if query.Search != "" {
		like := "%" + query.Search + "%"
		conditions = append(conditions, `(ma.id LIKE ? OR ma.platform_account_id LIKE ? OR ma.platform_name LIKE ? OR ma.remark LIKE ? OR EXISTS (SELECT 1 FROM browser_profiles bp WHERE bp.id = ma.browser_profile_id AND bp.bit_name LIKE ?))`)
		args = append(args, like, like, like, like, like)
	}
	if query.ProfileSearch != "" {
		like := "%" + query.ProfileSearch + "%"
		conditions = append(conditions, `EXISTS (SELECT 1 FROM browser_profiles bp WHERE bp.id = ma.browser_profile_id AND (bp.bit_name LIKE ? OR bp.bit_seq LIKE ? OR bp.bit_profile_id LIKE ? OR bp.id LIKE ?))`)
		args = append(args, like, like, like, like)
	}
	if query.TeamID > 0 {
		conditions = append(conditions, "ma.team_id = ?")
		args = append(args, query.TeamID)
	}
	if query.CategoryType != "" {
		conditions = append(conditions, `EXISTS (SELECT 1 FROM media_account_categories rel JOIN categories c ON c.id=rel.category_id WHERE rel.media_account_id=ma.id AND c.type=?)`)
		args = append(args, query.CategoryType)
	}
	for _, set := range []struct {
		tags    []string
		exclude bool
	}{{query.AnyTags, false}, {query.ExcludeTags, true}} {
		if len(set.tags) > 0 {
			clause := `EXISTS (SELECT 1 FROM media_account_tags tag WHERE tag.media_account_id=ma.id AND tag.tag_name IN (` + placeholders(len(set.tags)) + `))`
			if set.exclude {
				clause = "NOT " + clause
			}
			conditions = append(conditions, clause)
			for _, tag := range set.tags {
				args = append(args, tag)
			}
		}
	}
	for _, tag := range query.AllTags {
		conditions = append(conditions, `EXISTS (SELECT 1 FROM media_account_tags tag WHERE tag.media_account_id=ma.id AND tag.tag_name=?)`)
		args = append(args, tag)
	}
	switch query.View {
	case "normal":
		conditions = append(conditions, "ma.platform_login_status='normal' AND ma.last_check_status<>'failed'")
	case "attention":
		conditions = append(conditions, "(ma.last_check_status='failed' OR ma.platform_login_status IN ('not_logged_in','verification_needed','expired','restricted','account_mismatch','environment_error'))")
	case "unbound":
		conditions = append(conditions, "ma.browser_profile_id IS NULL")
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}
	return where, args
}

func ListPage(query AccountFilter) ([]model.AccountRecord, model.AccountCounts, error) {
	return listPage(database.DB(), query)
}
func listPage(db *gorm.DB, query AccountFilter) ([]model.AccountRecord, model.AccountCounts, error) {
	where, args := accountConditions(query)
	var counts model.AccountCounts
	aggregate := `SELECT COUNT(*), COALESCE(SUM(ma.platform_login_status='normal' AND ma.last_check_status<>'failed'),0), COALESCE(SUM(ma.last_check_status='failed' OR ma.platform_login_status IN ('not_logged_in','verification_needed','expired','restricted','account_mismatch','environment_error')),0), COALESCE(SUM(ma.browser_profile_id IS NULL),0)`
	if err := queryRow(db, aggregate+accountReadFrom+where, args...).Scan(&counts.Total, &counts.Normal, &counts.Abnormal, &counts.Unbound); err != nil {
		return nil, counts, err
	}
	records, err := list(db, query)
	return records, counts, err
}
