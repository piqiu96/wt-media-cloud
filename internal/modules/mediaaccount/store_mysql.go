package mediaaccount

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/wt-media/wt-media-cloud/internal/common"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
)

const accountColumns = `id, user_id, team_id, game_id, platform, platform_account_id, name, avatar_url, browser_profile_id, remark, identification_status, duplicate_of_account_id, business_status, login_status, original_cookie, active_cookie, cookie_status, active_cookie_updated_at, last_checked_at, check_items, created_at, updated_at`

type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

func (s *MySQLStore) Create(record AccountRecord) (string, error) {
	result, err := s.db.Exec(
		`INSERT INTO media_accounts (`+accountColumns+`) VALUES (NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.UserID, record.TeamID, record.GameID, record.Platform,
		nullIfEmpty(record.PlatformAccountID), nullIfEmpty(record.Name), nullIfEmpty(record.AvatarURL), nullIfEmpty(record.BrowserProfileID),
		nullIfEmpty(record.Remark), record.IdentificationStatus, nullIfEmpty(record.DuplicateOfAccountID), record.BusinessStatus, record.LoginStatus,
		nullIfEmpty(record.OriginalCookie), nullIfEmpty(record.ActiveCookie), nullIfEmpty(record.CookieStatus),
		record.ActiveCookieUpdatedAt, record.LastCheckedAt, marshalCheckItems(record.CheckItems), record.CreatedAt, record.UpdatedAt,
	)
	if duplicateKey(err) {
		return "", ErrDuplicateAccount
	}
	if err != nil {
		return "", err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(id, 10), nil
}

func (s *MySQLStore) Find(id string) (AccountRecord, bool, error) {
	return s.find(`SELECT `+accountColumns+` FROM media_accounts WHERE id = ?`, id)
}

func (s *MySQLStore) FindByIdentity(userID identity.UserID, platform Platform, platformAccountID string) (AccountRecord, bool, error) {
	return s.find(
		`SELECT `+accountColumns+` FROM media_accounts WHERE user_id = ? AND platform = ? AND platform_account_id = ?`,
		userID, platform, platformAccountID,
	)
}

func (s *MySQLStore) FindByProfilePlatform(profileID string, platform Platform) (AccountRecord, bool, error) {
	return s.find(
		`SELECT `+accountColumns+` FROM media_accounts WHERE browser_profile_id = ? AND platform = ? LIMIT 1`,
		profileID, platform,
	)
}

func (s *MySQLStore) find(query string, args ...any) (AccountRecord, bool, error) {
	record, err := scanAccount(s.db.QueryRow(query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return AccountRecord{}, false, nil
	}
	if err != nil {
		return AccountRecord{}, false, err
	}
	return record, true, nil
}

func (s *MySQLStore) Update(record AccountRecord, _ *[]string) error {
	result, err := s.db.Exec(
		`UPDATE media_accounts SET user_id = ?, team_id = ?, game_id = ?, platform = ?, platform_account_id = ?, name = ?, avatar_url = ?, browser_profile_id = ?, remark = ?, identification_status = ?, duplicate_of_account_id = ?, business_status = ?, login_status = ?, original_cookie = ?, active_cookie = ?, cookie_status = ?, active_cookie_updated_at = ?, last_checked_at = ?, check_items = ?, updated_at = ? WHERE id = ?`,
		record.UserID, record.TeamID, record.GameID, record.Platform, nullIfEmpty(record.PlatformAccountID), nullIfEmpty(record.Name),
		nullIfEmpty(record.AvatarURL), nullIfEmpty(record.BrowserProfileID), nullIfEmpty(record.Remark), record.IdentificationStatus,
		nullIfEmpty(record.DuplicateOfAccountID), record.BusinessStatus, record.LoginStatus,
		nullIfEmpty(record.OriginalCookie), nullIfEmpty(record.ActiveCookie), nullIfEmpty(record.CookieStatus),
		record.ActiveCookieUpdatedAt, record.LastCheckedAt, marshalCheckItems(record.CheckItems), record.UpdatedAt, record.ID,
	)
	if duplicateKey(err) {
		return ErrDuplicateAccount
	}
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *MySQLStore) List(query AccountQuery) ([]AccountRecord, error) {
	statement := `SELECT ` + accountColumns + ` FROM media_accounts`
	conditions := make([]string, 0, 3)
	args := make([]any, 0, 3)
	if query.UserID > 0 {
		conditions = append(conditions, "user_id = ?")
		args = append(args, query.UserID)
	}
	if query.GameID != "" {
		conditions = append(conditions, "game_id = ?")
		args = append(args, query.GameID)
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

	rows, err := s.db.Query(statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]AccountRecord, 0)
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
	if len(query.AnyTags) == 0 && len(query.AllTags) == 0 && len(query.ExcludeTags) == 0 {
		return records, nil
	}
	ids := make([]string, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.ID)
	}
	tagsByAccount, err := s.ListTags(ids)
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

func (s *MySQLStore) AddTags(userID identity.UserID, accountIDs, tags []string, createdAt time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, accountID := range accountIDs {
		for _, tag := range tags {
			if _, err := tx.Exec(
				`INSERT IGNORE INTO media_account_tags (id, user_id, media_account_id, tag_name, created_at) VALUES (?, ?, ?, ?, ?)`,
				common.NewID("media_account_tag"), userID, accountID, tag, createdAt,
			); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (s *MySQLStore) RemoveTags(userID identity.UserID, accountIDs, tags []string) error {
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
	_, err := s.db.Exec(
		`DELETE FROM media_account_tags WHERE user_id = ? AND media_account_id IN (`+placeholders(len(accountIDs))+`) AND tag_name IN (`+placeholders(len(tags))+`)`,
		args...,
	)
	return err
}

func (s *MySQLStore) ListTags(accountIDs []string) (map[string][]string, error) {
	result := make(map[string][]string, len(accountIDs))
	if len(accountIDs) == 0 {
		return result, nil
	}
	args := make([]any, len(accountIDs))
	for i, accountID := range accountIDs {
		args[i] = accountID
	}
	rows, err := s.db.Query(
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

func (s *MySQLStore) AppendAudit(event identity.AuditEvent) error {
	summary, err := json.Marshal(event.Summary)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, summary_json, created_at) VALUES (?, NULLIF(?, 0), ?, ?, NULLIF(?, ''), ?, ?)`,
		event.ID, event.ActorUserID, event.Action, event.TargetType, event.TargetID, summary, event.CreatedAt,
	)
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanAccount(row scanner) (AccountRecord, error) {
	var record AccountRecord
	var platformAccountID, name, avatarURL, browserProfileID, remark sql.NullString
	var duplicateOfAccountID, originalCookie, activeCookie, cookieStatus sql.NullString
	var activeCookieUpdatedAt, lastCheckedAt sql.NullTime
	var checkItemsJSON sql.NullString
	err := row.Scan(
		&record.ID, &record.UserID, &record.TeamID, &record.GameID, &record.Platform,
		&platformAccountID, &name, &avatarURL, &browserProfileID,
		&remark, &record.IdentificationStatus, &duplicateOfAccountID, &record.BusinessStatus, &record.LoginStatus,
		&originalCookie, &activeCookie, &cookieStatus, &activeCookieUpdatedAt, &lastCheckedAt,
		&checkItemsJSON, &record.CreatedAt, &record.UpdatedAt,
	)
	if err != nil {
		return AccountRecord{}, err
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

func (s *MySQLStore) CreateGroup(group AccountGroup) (string, error) {
	filtersJSON, err := json.Marshal(group.Filters)
	if err != nil {
		return "", err
	}
	result, err := s.db.Exec(
		`INSERT INTO account_groups (user_id, team_id, name, filters, sort_order, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		group.UserID, group.TeamID, group.Name, string(filtersJSON), group.SortOrder, group.CreatedAt, group.UpdatedAt,
	)
	if err != nil {
		return "", err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(id, 10), nil
}

func (s *MySQLStore) FindGroup(id string) (AccountGroup, bool, error) {
	group, err := scanAccountGroup(s.db.QueryRow(`SELECT `+accountGroupColumns+` FROM account_groups WHERE id = ?`, id))
	if err != nil {
		return AccountGroup{}, false, err
	}
	return group, group.ID != "", nil
}

func (s *MySQLStore) ListGroups(userID identity.UserID) ([]AccountGroup, error) {
	rows, err := s.db.Query(`SELECT `+accountGroupColumns+` FROM account_groups WHERE user_id = ? ORDER BY sort_order ASC, name ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groups := []AccountGroup{}
	for rows.Next() {
		group, err := scanAccountGroup(rows)
		if err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func (s *MySQLStore) UpdateGroup(group AccountGroup) error {
	filtersJSON, err := json.Marshal(group.Filters)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`UPDATE account_groups SET name = ?, filters = ?, sort_order = ?, updated_at = ? WHERE id = ?`,
		group.Name, string(filtersJSON), group.SortOrder, group.UpdatedAt, group.ID,
	)
	return err
}

func (s *MySQLStore) DeleteGroup(id string) error {
	_, err := s.db.Exec(`DELETE FROM account_groups WHERE id = ?`, id)
	return err
}

func scanAccountGroup(row interface{ Scan(...any) error }) (AccountGroup, error) {
	var group AccountGroup
	var filtersJSON []byte
	if err := row.Scan(&group.ID, &group.UserID, &group.TeamID, &group.Name, &filtersJSON, &group.SortOrder, &group.CreatedAt, &group.UpdatedAt); err != nil {
		return AccountGroup{}, err
	}
	if len(filtersJSON) > 0 {
		if err := json.Unmarshal(filtersJSON, &group.Filters); err != nil {
			return AccountGroup{}, err
		}
	}
	return group, nil
}

func marshalCheckItems(items []AccountCheckItem) any {
	if len(items) == 0 {
		return nil
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return nil
	}
	return string(raw)
}

var _ Store = (*MySQLStore)(nil)
