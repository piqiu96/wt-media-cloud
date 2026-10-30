package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/id"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
	"gorm.io/gorm"
)

const profileColumnsSQL = `id, user_id, team_id, bit_profile_id, bit_main_user_id, bit_user_id, bit_name, bit_seq, bit_group_id, bit_group_name, bit_profile_status, bit_updated_at, bit_proxy_type, bit_proxy_host, bit_proxy_port, proxy_id, reserved_proxy_id, reserved_since, bit_remark, remark, enabled, local_state, last_synced_at, created_at, updated_at`

// profileColumnsWithPrefix 给每一列加上表别名，供需要 join users 的查询使用。
func profileColumnsWithPrefix(alias string) string {
	columns := strings.Split(profileColumnsSQL, ", ")
	for index := range columns {
		columns[index] = alias + "." + columns[index]
	}
	return strings.Join(columns, ", ")
}

func FindBinding(userID sharedidentity.UserID) (model.BitAccountBinding, bool, error) {
	return findBinding(database.DB(), userID)
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

func findBinding(db *gorm.DB, userID sharedidentity.UserID) (model.BitAccountBinding, bool, error) {
	var binding model.BitAccountBinding
	var mainUserID, status sql.NullString
	var boundAt, verifiedAt sql.NullTime
	err := queryRow(db,
		`SELECT id, bit_main_user_id, bit_account_status, bit_account_bound_at, bit_account_last_verified_at FROM users WHERE id = ?`, userID,
	).Scan(&binding.UserID, &mainUserID, &status, &boundAt, &verifiedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.BitAccountBinding{}, false, nil
	}
	if err != nil {
		return model.BitAccountBinding{}, false, err
	}
	if !mainUserID.Valid || mainUserID.String == "" {
		return model.BitAccountBinding{}, false, nil
	}
	binding.BitMainUserID = mainUserID.String
	binding.Status = model.BitAccountStatus(status.String)
	if boundAt.Valid {
		value := boundAt.Time
		binding.BoundAt = &value
	}
	if verifiedAt.Valid {
		value := verifiedAt.Time
		binding.LastVerifiedAt = &value
	}
	return binding, true, nil
}

// ProfileListFilter 是列表页的筛选条件。运行态不在这里：它来自 Desktop 运行时，
// 不来自 Cloud 数据行。
type ProfileListFilter struct {
	UserID       sharedidentity.UserID
	ID           string
	BitName      string
	BitGroup     string
	BitProfileID string
	Remark       string
	LocalState   string
	Enabled      *bool
}

func ListProfiles(filter ProfileListFilter) ([]model.BrowserProfile, error) {
	return listProfiles(database.DB(), filter)
}

func listProfiles(db *gorm.DB, filter ProfileListFilter) ([]model.BrowserProfile, error) {
	// join 是作用域的另一半：只有 main_user_id 仍等于所有者当前绑定时行才可见。
	// 旧主账号下的行继续保留，但在重新绑定该主账号之前不会出现在任何正常查询中。
	query := `SELECT ` + profileColumnsWithPrefix("bp") + ` FROM browser_profiles bp JOIN users u ON u.id = bp.user_id WHERE bp.bit_main_user_id = u.bit_main_user_id`
	args := make([]any, 0, 10)
	if filter.UserID > 0 {
		query += ` AND bp.user_id = ?`
		args = append(args, filter.UserID)
	}
	if filter.ID != "" {
		query += ` AND bp.id LIKE ? ESCAPE '!'`
		args = append(args, profileLikeContains(filter.ID))
	}
	if filter.BitName != "" {
		query += ` AND bp.bit_name LIKE ? ESCAPE '!'`
		args = append(args, profileLikeContains(filter.BitName))
	}
	if filter.BitGroup != "" {
		like := profileLikeContains(filter.BitGroup)
		query += ` AND (bp.bit_group_id LIKE ? ESCAPE '!' OR bp.bit_group_name LIKE ? ESCAPE '!')`
		args = append(args, like, like)
	}
	if filter.BitProfileID != "" {
		query += ` AND bp.bit_profile_id LIKE ? ESCAPE '!'`
		args = append(args, profileLikeContains(filter.BitProfileID))
	}
	if filter.Remark != "" {
		query += ` AND bp.remark LIKE ? ESCAPE '!'`
		args = append(args, profileLikeContains(filter.Remark))
	}
	if filter.LocalState != "" {
		query += ` AND bp.local_state = ?`
		args = append(args, filter.LocalState)
	}
	if filter.Enabled != nil {
		query += ` AND bp.enabled = ?`
		args = append(args, *filter.Enabled)
	}
	// BitBrowser scans keep the scanned order; the page shows the newest first.
	query += ` ORDER BY bp.bit_seq DESC, bp.id DESC`
	return listProfilesByQuery(db, query, args...)
}

func profileLikeContains(value string) string {
	return "%" + strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(value) + "%"
}

func listProfilesByQuery(db *gorm.DB, query string, args ...any) ([]model.BrowserProfile, error) {
	rows, err := queryRows(db, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	profiles := make([]model.BrowserProfile, 0)
	for rows.Next() {
		profile, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	return profiles, rows.Err()
}

// ResolveProfile 同时返回 Profile 是否存在、以及是否仍在其所有者当前主账号作用域内。
// 两个答案都要给调用方：旧主账号下的 Profile 不能执行，但也不属于其他用户。
func ResolveProfile(profileID string) (sharedidentity.UserID, bool, bool, bool, error) {
	return resolveProfile(database.DB(), profileID)
}

func resolveProfile(db *gorm.DB, profileID string) (sharedidentity.UserID, bool, bool, bool, error) {
	var userID sharedidentity.UserID
	var status model.ProfileLocalState
	var inScope bool
	err := queryRow(db,
		`SELECT bp.user_id, bp.local_state, IFNULL(bp.bit_main_user_id = u.bit_main_user_id, 0) FROM browser_profiles bp JOIN users u ON u.id = bp.user_id WHERE bp.id = ?`,
		profileID,
	).Scan(&userID, &status, &inScope)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, false, false, nil
	}
	if err != nil {
		return 0, false, false, false, err
	}
	return userID, status == model.ProfileActive, inScope, true, nil
}

func ResolveProfileForAccountCheck(profileID string) (string, sharedidentity.UserID, string, bool, bool, bool, error) {
	return resolveProfileForAccountCheck(database.DB(), profileID)
}

func resolveProfileForAccountCheck(db *gorm.DB, profileID string) (string, sharedidentity.UserID, string, bool, bool, bool, error) {
	var id string
	var userID sharedidentity.UserID
	var bitProfileID string
	var status model.ProfileLocalState
	var inScope bool
	err := queryRow(db,
		`SELECT bp.id, bp.user_id, bp.bit_profile_id, bp.local_state, IFNULL(bp.bit_main_user_id = u.bit_main_user_id, 0) FROM browser_profiles bp JOIN users u ON u.id = bp.user_id WHERE bp.id = ?`,
		profileID,
	).Scan(&id, &userID, &bitProfileID, &status, &inScope)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, "", false, false, false, nil
	}
	if err != nil {
		return "", 0, "", false, false, false, err
	}
	return id, userID, bitProfileID, inScope, status == model.ProfileActive, true, nil
}

// ResolveProxyForAccountCheck 只返回渲染媒体账号检测项所需的代理健康事实。
// 这个跨模块读模型不查询、也不返回代理凭据与来源 URL。
func ResolveProxyForAccountCheck(profileID string) (string, bool, string, *time.Time, bool, error) {
	return resolveProxyForAccountCheck(database.DB(), profileID)
}

func resolveProxyForAccountCheck(db *gorm.DB, profileID string) (string, bool, string, *time.Time, bool, error) {
	var proxyID, lastCheckResult sql.NullString
	var enabled sql.NullBool
	var expiresAt sql.NullTime
	err := queryRow(db, `SELECT bp.proxy_id, pc.enabled, pc.last_check_result, pc.expires_at
		FROM browser_profiles bp
		JOIN users u ON u.id = bp.user_id
		JOIN proxies pc ON pc.id = bp.proxy_id
		WHERE bp.id = ? AND bp.proxy_id IS NOT NULL AND bp.bit_main_user_id = u.bit_main_user_id`, profileID).
		Scan(&proxyID, &enabled, &lastCheckResult, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, "", nil, false, nil
	}
	if err != nil {
		return "", false, "", nil, false, err
	}
	var value *time.Time
	if expiresAt.Valid {
		at := expiresAt.Time
		value = &at
	}
	return proxyID.String, enabled.Bool, lastCheckResult.String, value, proxyID.String != "", nil
}

func GetProfile(profileID string) (model.BrowserProfile, bool, error) {
	return getProfile(database.DB(), profileID)
}

func getProfile(db *gorm.DB, profileID string) (model.BrowserProfile, bool, error) {
	row := queryRow(db, `SELECT `+profileColumnsSQL+` FROM browser_profiles WHERE id = ?`, profileID)
	profile, err := scanProfile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.BrowserProfile{}, false, nil
	}
	if err != nil {
		return model.BrowserProfile{}, false, err
	}
	return profile, true, nil
}

// GetProfileInScope 只在 Profile 仍属于其所有者当前绑定的 BitBrowser 主账号时返回。
// 旧主账号下的行保留在表中，但正常读取不可见。
func GetProfileInScope(profileID string) (model.BrowserProfile, bool, error) {
	return getProfileInScope(database.DB(), profileID)
}

func getProfileInScope(db *gorm.DB, profileID string) (model.BrowserProfile, bool, error) {
	row := queryRow(db,
		`SELECT `+profileColumnsWithPrefix("bp")+` FROM browser_profiles bp JOIN users u ON u.id = bp.user_id WHERE bp.id = ? AND bp.bit_main_user_id = u.bit_main_user_id`,
		profileID,
	)
	profile, err := scanProfile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.BrowserProfile{}, false, nil
	}
	if err != nil {
		return model.BrowserProfile{}, false, err
	}
	return profile, true, nil
}

type profileSyncSnapshot struct {
	Profiles    []model.BrowserProfile `json:"profiles"`
	Diff        []model.ProfileDiff    `json:"diff"`
	Status      model.ScanStatus       `json:"status,omitempty"`
	ConfirmedAt *time.Time             `json:"confirmed_at,omitempty"`
}

func CreateScan(scan model.ProfileScan) error {
	return createScan(database.DB(), scan)
}

func createScan(db *gorm.DB, scan model.ProfileScan) error {
	snapshot, err := json.Marshal(profileSyncSnapshot{Profiles: scan.Profiles, Diff: scan.Diff})
	if err != nil {
		return err
	}
	_, err = execSQL(db,
		`INSERT INTO browser_profile_syncs (id, user_id, bit_main_user_id, snapshot, observed_at, expires_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE id = VALUES(id), bit_main_user_id = VALUES(bit_main_user_id), snapshot = VALUES(snapshot), observed_at = VALUES(observed_at), expires_at = VALUES(expires_at), created_at = VALUES(created_at)`,
		scan.ID, scan.UserID, scan.BitMainUserID, string(snapshot), scan.CreatedAt, scan.ExpiresAt, scan.CreatedAt,
	)
	return err
}

func FindScan(scanID string) (model.ProfileScan, bool, error) {
	return findScan(database.DB(), scanID)
}

func findScan(db *gorm.DB, scanID string) (model.ProfileScan, bool, error) {
	var scan model.ProfileScan
	var snapshot []byte
	err := queryRow(db,
		`SELECT id, user_id, bit_main_user_id, snapshot, observed_at, expires_at, created_at FROM browser_profile_syncs WHERE id = ?`, scanID,
	).Scan(&scan.ID, &scan.UserID, &scan.BitMainUserID, &snapshot, &scan.CreatedAt, &scan.ExpiresAt, &scan.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ProfileScan{}, false, nil
	}
	if err != nil {
		return model.ProfileScan{}, false, err
	}
	var payload profileSyncSnapshot
	if err := json.Unmarshal(snapshot, &payload); err != nil {
		return model.ProfileScan{}, false, err
	}
	scan.Status = payload.Status
	if scan.Status == "" {
		scan.Status = model.ScanReady
	}
	scan.ConfirmedAt = payload.ConfirmedAt
	scan.Profiles = payload.Profiles
	scan.Diff = payload.Diff
	return scan, true, nil
}

func FindScanByUser(userID sharedidentity.UserID) (model.ProfileScan, bool, error) {
	return findScanByUser(database.DB(), userID)
}

func findScanByUser(db *gorm.DB, userID sharedidentity.UserID) (model.ProfileScan, bool, error) {
	var scanID string
	err := queryRow(db, `SELECT id FROM browser_profile_syncs WHERE user_id = ?`, userID).Scan(&scanID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ProfileScan{}, false, nil
	}
	if err != nil {
		return model.ProfileScan{}, false, err
	}
	return findScan(db, scanID)
}

func DeleteScanByUser(userID sharedidentity.UserID) error {
	return deleteScanByUser(database.DB(), userID)
}

func deleteScanByUser(db *gorm.DB, userID sharedidentity.UserID) error {
	_, err := execSQL(db, `DELETE FROM browser_profile_syncs WHERE user_id = ?`, userID)
	return err
}

// BindBitAccount 以行锁串行化同一用户的绑定写入，保证并发观察不会相互覆盖。
func BindBitAccount(userID sharedidentity.UserID, mainUserID string, rebind bool, at time.Time) (model.BitAccountBinding, error) {
	return bindBitAccount(database.DB(), userID, mainUserID, rebind, at)
}

func bindBitAccount(db *gorm.DB, userID sharedidentity.UserID, mainUserID string, rebind bool, at time.Time) (model.BitAccountBinding, error) {
	tx := db.Begin()
	defer rollbackTx(tx)
	var current sql.NullString
	var boundAt sql.NullTime
	err := queryRow(tx, `SELECT bit_main_user_id, bit_account_bound_at FROM users WHERE id = ? FOR UPDATE`, userID).
		Scan(&current, &boundAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.BitAccountBinding{}, model.ErrForbidden
	}
	if err != nil {
		return model.BitAccountBinding{}, err
	}
	action := model.AuditMainAccountBind
	result := "bound"
	previous := ""
	firstBoundAt := at
	switch {
	case !current.Valid || current.String == "":
		if _, err := execSQL(tx,
			`UPDATE users SET bit_main_user_id = ?, bit_account_status = ?, bit_account_bound_at = ?, bit_account_last_verified_at = ?, updated_at = ? WHERE id = ?`,
			mainUserID, model.BitAccountBound, at, at, at, userID,
		); err != nil {
			return model.BitAccountBinding{}, err
		}
	case current.String == mainUserID:
		// 主账号未变时只刷新验证时间，首次绑定时间保持原值。历史行可能没有
		// bound_at，这里顺手补齐，返回值和数据库才不会各说一套。
		action, result = model.AuditMainAccountVerify, "verified"
		if boundAt.Valid {
			firstBoundAt = boundAt.Time
		}
		if _, err := execSQL(tx,
			`UPDATE users SET bit_account_status = ?, bit_account_bound_at = COALESCE(bit_account_bound_at, ?), bit_account_last_verified_at = ?, updated_at = ? WHERE id = ?`,
			model.BitAccountBound, at, at, at, userID,
		); err != nil {
			return model.BitAccountBinding{}, err
		}
	case !rebind:
		return model.BitAccountBinding{}, model.ErrBitMainUserMismatch
	default:
		action, result, previous = model.AuditMainAccountRebind, "rebound", current.String
		if _, err := execSQL(tx,
			`UPDATE users SET bit_main_user_id = ?, bit_account_status = ?, bit_account_bound_at = ?, bit_account_last_verified_at = ?, updated_at = ? WHERE id = ?`,
			mainUserID, model.BitAccountBound, at, at, at, userID,
		); err != nil {
			return model.BitAccountBinding{}, err
		}
	}
	summaryFields := map[string]any{"main_user_id": mainUserID, "result": result}
	if previous != "" {
		summaryFields["previous_main_user_id"] = previous
	}
	summary, _ := json.Marshal(summaryFields)
	if _, err := execSQL(tx,
		`INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, summary_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id.NewID("audit"), userID, action, "user", userID, summary, at,
	); err != nil {
		return model.BitAccountBinding{}, err
	}
	if err := tx.Commit().Error; err != nil {
		return model.BitAccountBinding{}, err
	}
	return model.BitAccountBinding{
		UserID: userID, BitMainUserID: mainUserID, Status: model.BitAccountBound,
		BoundAt: &firstBoundAt, LastVerifiedAt: &at,
	}, nil
}

func ApplyScan(scan model.ProfileScan, observedBitProfileIDs []string, at time.Time) error {
	return applyScan(database.DB(), scan, observedBitProfileIDs, at)
}

func applyScan(db *gorm.DB, scan model.ProfileScan, observedBitProfileIDs []string, at time.Time) error {
	tx := db.Begin()
	defer rollbackTx(tx)
	var boundMain sql.NullString
	err := queryRow(tx, `SELECT bit_main_user_id FROM users WHERE id = ? FOR UPDATE`, scan.UserID).Scan(&boundMain)
	if err != nil {
		return err
	}
	if !boundMain.Valid || boundMain.String == "" {
		return model.ErrBitNotBound
	}
	if boundMain.String != scan.BitMainUserID {
		return model.ErrBitMainUserMismatch
	}
	// Serialize confirmations and reject an obsolete preview before any mirror write.
	var storedSnapshot []byte
	err = queryRow(tx, `SELECT snapshot FROM browser_profile_syncs WHERE id = ? AND user_id = ? FOR UPDATE`, scan.ID, scan.UserID).Scan(&storedSnapshot)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrScanNotReady
	}
	if err != nil {
		return err
	}
	var stored profileSyncSnapshot
	if err := json.Unmarshal(storedSnapshot, &stored); err != nil {
		return err
	}
	if stored.Status == model.ScanConfirmed {
		return model.ErrScanNotReady
	}
	// Lock the owner's mirrors before applying snapshots, so a concurrent local
	// proxy write cannot be overwritten or have its quota released by sync.
	rows, err := tx.Raw(`SELECT id, reserved_proxy_id FROM browser_profiles WHERE user_id = ? AND bit_main_user_id = ? FOR UPDATE`, scan.UserID, scan.BitMainUserID).Rows()
	if err != nil {
		return err
	}
	pending := false
	for rows.Next() {
		var profileID string
		var reserved sql.NullString
		if err := rows.Scan(&profileID, &reserved); err != nil {
			rows.Close()
			return err
		}
		pending = pending || reserved.String != ""
	}
	rowErr := rows.Err()
	rows.Close()
	if rowErr != nil {
		return rowErr
	}
	if pending {
		return model.ErrProfileProxyPending
	}
	for _, profile := range scan.Profiles {
		if _, err := execSQL(tx,
			`INSERT INTO browser_profiles (
				user_id, team_id, bit_profile_id, bit_main_user_id, bit_user_id,
				bit_name, bit_seq, bit_group_id, bit_group_name, bit_profile_status, bit_updated_at,
				bit_proxy_type, bit_proxy_host, bit_proxy_port, bit_remark, enabled, local_state,
				last_synced_at, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
				bit_user_id = VALUES(bit_user_id), bit_name = VALUES(bit_name), bit_seq = VALUES(bit_seq),
				bit_group_id = VALUES(bit_group_id), bit_group_name = VALUES(bit_group_name),
				bit_profile_status = VALUES(bit_profile_status), bit_updated_at = VALUES(bit_updated_at),
				bit_proxy_type = VALUES(bit_proxy_type), bit_proxy_host = VALUES(bit_proxy_host),
				bit_proxy_port = VALUES(bit_proxy_port), bit_remark = VALUES(bit_remark),
				local_state = VALUES(local_state), last_synced_at = VALUES(last_synced_at), updated_at = VALUES(updated_at)`,
			profile.UserID, profile.TeamID, profile.BitProfileID, profile.BitMainUserID, profile.BitUserID,
			profile.BitName, profile.BitSeq, nullIfEmpty(profile.BitGroupID), nullIfEmpty(profile.BitGroupName),
			nullIfEmpty(profile.BitProfileStatus), nullIfEmpty(profile.BitUpdatedAt), nullIfEmpty(profile.BitProxyType),
			nullIfEmpty(profile.BitProxyHost), profile.BitProxyPort, nullIfEmpty(profile.BitRemark), model.ProfileActive,
			at, profile.CreatedAt, at,
		); err != nil {
			return err
		}
	}
	bitIDs := make([]string, 0, len(observedBitProfileIDs))
	for _, bitID := range observedBitProfileIDs {
		bitIDs = append(bitIDs, bitID)
	}
	{
		markArgs := []any{model.ProfileLocalMissing, at, at, scan.UserID, scan.BitMainUserID}
		missingCondition := ""
		if len(bitIDs) > 0 {
			missingCondition = " AND bit_profile_id NOT IN (" + placeholders(len(bitIDs)) + ")"
		}
		for _, bitID := range bitIDs {
			markArgs = append(markArgs, bitID)
		}
		markArgs = append(markArgs, model.ProfileArchived, model.ProfileLocalMissing)
		if _, err := execSQL(tx,
			`UPDATE browser_profiles SET local_state = ?, last_synced_at = ?, updated_at = ?
			 WHERE user_id = ? AND bit_main_user_id = ?`+missingCondition+` AND local_state NOT IN (?, ?)`,
			markArgs...,
		); err != nil {
			return err
		}
	}
	receipt, err := json.Marshal(profileSyncSnapshot{Profiles: scan.Profiles, Diff: scan.Diff, Status: model.ScanConfirmed, ConfirmedAt: &at})
	if err != nil {
		return err
	}
	result, err := execSQL(tx, `UPDATE browser_profile_syncs SET snapshot = ? WHERE id = ? AND user_id = ?`, string(receipt), scan.ID, scan.UserID)
	if err != nil {
		return err
	}
	if result == 0 {
		return model.ErrScanNotReady
	}

	summary, _ := json.Marshal(map[string]any{"bit_main_user_id": scan.BitMainUserID, "profile_count": len(scan.Profiles)})
	if _, err := execSQL(tx,
		`INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, summary_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id.NewID("audit"), scan.UserID, "bitbrowser.profile_sync.confirm", "browser_profile_sync", scan.ID, summary, at,
	); err != nil {
		return err
	}
	return tx.Commit().Error
}

type scanner interface{ Scan(...any) error }

func scanProfile(row scanner) (model.BrowserProfile, error) {
	var profile model.BrowserProfile
	var groupID, groupName, bitStatus, bitUpdatedAt, proxyType, proxyHost, proxyID, reservedProxyID, bitRemark, remark sql.NullString
	var proxyPort sql.NullInt64
	var reservedSince sql.NullTime
	err := row.Scan(
		&profile.ID, &profile.UserID, &profile.TeamID, &profile.BitProfileID, &profile.BitMainUserID, &profile.BitUserID,
		&profile.BitName, &profile.BitSeq, &groupID, &groupName, &bitStatus, &bitUpdatedAt,
		&proxyType, &proxyHost, &proxyPort, &proxyID, &reservedProxyID, &reservedSince,
		&bitRemark, &remark, &profile.Enabled, &profile.LocalState, &profile.LastSyncedAt, &profile.CreatedAt, &profile.UpdatedAt,
	)
	if err != nil {
		return model.BrowserProfile{}, err
	}
	profile.BitGroupID = groupID.String
	profile.BitGroupName = groupName.String
	profile.BitProfileStatus = bitStatus.String
	profile.BitUpdatedAt = bitUpdatedAt.String
	profile.BitProxyType = proxyType.String
	profile.BitProxyHost = proxyHost.String
	profile.ProxyID = sharedidentity.ProxyID(proxyID.String)
	profile.ReservedProxyID = sharedidentity.ProxyID(reservedProxyID.String)
	profile.BitRemark = bitRemark.String
	profile.Remark = remark.String
	if proxyPort.Valid {
		profile.BitProxyPort = int(proxyPort.Int64)
	}
	if reservedSince.Valid {
		value := reservedSince.Time
		profile.ReservedSince = &value
	}
	return profile, nil
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func placeholders(count int) string {
	return strings.TrimRight(strings.Repeat("?, ", count), ", ")
}

func DeleteProfile(profileID string, actorID sharedidentity.UserID, at time.Time) error {
	return deleteProfile(database.DB(), profileID, actorID, at)
}

// deleteProfile keeps account detachment, proxy quota release, physical deletion,
// and the credential-free audit in one transaction. The locked state is checked
// again because a scan or proxy operation may have changed it after the service
// read the row.
func deleteProfile(db *gorm.DB, profileID string, actorID sharedidentity.UserID, at time.Time) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var localState string
		var reservedProxyID, proxyID sql.NullInt64
		var bitProfileID string
		var ownerID sharedidentity.UserID
		err := queryRow(tx, `SELECT local_state, reserved_proxy_id, proxy_id, bit_profile_id, user_id FROM browser_profiles WHERE id = ? FOR UPDATE`, profileID).
			Scan(&localState, &reservedProxyID, &proxyID, &bitProfileID, &ownerID)
		if errors.Is(err, sql.ErrNoRows) {
			return model.ErrProfileNotFound
		}
		if err != nil {
			return err
		}
		if localState != string(model.ProfileLocalMissing) {
			return model.ErrProfileNotMissing
		}
		if reservedProxyID.Valid {
			return model.ErrProfileProxyPending
		}
		var accountCount int
		if err := queryRow(tx, `SELECT COUNT(*) FROM media_accounts WHERE browser_profile_id = ?`, profileID).Scan(&accountCount); err != nil {
			return err
		}
		if _, err := execSQL(tx, `UPDATE media_accounts SET browser_profile_id = NULL, platform_login_status = 'unknown', last_check_at = NULL, last_check_items = NULL, updated_at = ? WHERE browser_profile_id = ?`, at, profileID); err != nil {
			return err
		}
		rows, err := execSQL(tx, `DELETE FROM browser_profiles WHERE id = ? AND local_state = 'local_missing' AND reserved_proxy_id IS NULL`, profileID)
		if err != nil {
			return err
		}
		if rows != 1 {
			return model.ErrProfileNotMissing
		}
		summary, err := json.Marshal(map[string]any{
			"bit_profile_id": bitProfileID, "owner_user_id": ownerID,
			"unlinked_account_count": accountCount, "released_proxy_id": proxyID.Int64,
		})
		if err != nil {
			return err
		}
		_, err = execSQL(tx,
			`INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, summary_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			id.NewID("audit"), actorID, "bitbrowser.profile_missing.delete", "browser_profile", profileID, summary, at,
		)
		return err
	})
}

func ProfileHasDependencies(profileID string) (bool, error) {
	var count int
	if err := queryRow(database.DB(), `SELECT COUNT(*) FROM media_accounts WHERE browser_profile_id = ?`, profileID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func UpdateProfile(profileID string, remark *string, enabled *bool, at time.Time) error {
	return updateProfile(database.DB(), profileID, remark, enabled, at)
}

func updateProfile(db *gorm.DB, profileID string, remark *string, enabled *bool, at time.Time) error {
	var sets []string
	var args []any
	if remark != nil {
		sets = append(sets, "remark = ?")
		args = append(args, nullIfEmpty(*remark))
	}
	if enabled != nil {
		sets = append(sets, "enabled = ?")
		args = append(args, *enabled)
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, at)
	args = append(args, profileID)
	where := " WHERE id = ?"
	if enabled != nil {
		where += " AND reserved_proxy_id IS NULL"
	}
	rows, err := execSQL(db, `UPDATE browser_profiles SET `+strings.Join(sets, ", ")+where, args...)
	if err == nil && rows == 0 && enabled != nil {
		return model.ErrProfileProxyPending
	}
	return err
}

func CountProfilesByProxyID(proxyID string) (int, error) {
	return countProfilesByProxyID(database.DB(), proxyID)
}

func countProfilesByProxyID(db *gorm.DB, proxyID string) (int, error) {
	var count int
	if err := queryRow(db, `SELECT COUNT(*) FROM browser_profiles WHERE proxy_id = ? AND local_state IN (?, ?)`, proxyID, model.ProfileActive, model.ProfileLocalMissing).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func BindProxy(profileID, proxyID, proxyType, proxyHost string, proxyPort int) (model.BrowserProfile, error) {
	return bindProxy(database.DB(), profileID, proxyID, proxyType, proxyHost, proxyPort)
}

func bindProxy(db *gorm.DB, profileID, proxyID, proxyType, proxyHost string, proxyPort int) (model.BrowserProfile, error) {
	result, err := execSQL(db,
		`UPDATE browser_profiles SET proxy_id = ?, bit_proxy_type = ?, bit_proxy_host = ?, bit_proxy_port = ?, updated_at = ? WHERE id = ?`,
		proxyID, nullIfEmpty(proxyType), nullIfEmpty(proxyHost), proxyPort, time.Now(), profileID,
	)
	if err != nil {
		return model.BrowserProfile{}, err
	}
	updated := result
	if updated == 0 {
		return model.BrowserProfile{}, model.ErrProfileNotFound
	}
	profile, found, err := getProfile(db, profileID)
	if err != nil {
		return model.BrowserProfile{}, err
	}
	if !found {
		return model.BrowserProfile{}, model.ErrProfileNotFound
	}
	return profile, nil
}

func UnbindProxy(profileID, expectedProxyID string) (model.BrowserProfile, error) {
	return unbindProxy(database.DB(), profileID, expectedProxyID)
}

func unbindProxy(db *gorm.DB, profileID, expectedProxyID string) (model.BrowserProfile, error) {
	result, err := execSQL(db,
		`UPDATE browser_profiles SET proxy_id = NULL, bit_proxy_type = NULL, bit_proxy_host = NULL, bit_proxy_port = 0, updated_at = ? WHERE id = ? AND proxy_id = ?`,
		time.Now(), profileID, expectedProxyID,
	)
	if err != nil {
		return model.BrowserProfile{}, err
	}
	updated := result
	if updated == 0 {
		return model.BrowserProfile{}, model.ErrProfileNotFound
	}
	profile, found, err := getProfile(db, profileID)
	if err != nil {
		return model.BrowserProfile{}, err
	}
	if !found {
		return model.BrowserProfile{}, model.ErrProfileNotFound
	}
	return profile, nil
}

// ReleaseProxy 只解除正式托管关系，保留窗口的观测代理配置，也就是「改为比特自管」：
// 比特继续使用该地址，代理资源不再占用窗口配额。
func ReleaseProxy(profileID, expectedProxyID string) (model.BrowserProfile, error) {
	result, err := execSQL(database.DB(),
		`UPDATE browser_profiles SET proxy_id = NULL, updated_at = ? WHERE id = ? AND proxy_id = ?`,
		time.Now(), profileID, expectedProxyID,
	)
	if err != nil {
		return model.BrowserProfile{}, err
	}
	if result == 0 {
		return model.BrowserProfile{}, model.ErrProfileNotFound
	}
	profile, found, err := getProfile(database.DB(), profileID)
	if err != nil || !found {
		return model.BrowserProfile{}, model.ErrProfileNotFound
	}
	return profile, nil
}

// ClearObservedProxy 清掉比特自管窗口的观测地址。proxy_id 条件用于挡住并发分配：
// 对方已建立托管关系时，这次过期的清除不会把关系覆盖掉。
func ClearObservedProxy(profileID string) (model.BrowserProfile, error) {
	return clearObservedProxy(database.DB(), profileID)
}

func clearObservedProxy(db *gorm.DB, profileID string) (model.BrowserProfile, error) {
	result, err := execSQL(db,
		`UPDATE browser_profiles SET bit_proxy_type = NULL, bit_proxy_host = NULL, bit_proxy_port = 0, updated_at = ? WHERE id = ? AND proxy_id IS NULL`,
		time.Now(), profileID,
	)
	if err != nil {
		return model.BrowserProfile{}, err
	}
	if result == 0 {
		return model.BrowserProfile{}, model.ErrProfileNotFound
	}
	profile, found, err := getProfile(db, profileID)
	if err != nil || !found {
		return model.BrowserProfile{}, model.ErrProfileNotFound
	}
	return profile, nil
}

func ProfileHasAccountReferences(profileID string) (bool, error) {
	return profileHasAccountReferences(database.DB(), profileID)
}

func profileHasAccountReferences(db *gorm.DB, profileID string) (bool, error) {
	var count int
	if err := queryRow(db, `SELECT COUNT(*) FROM media_accounts WHERE browser_profile_id = ?`, profileID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// ListProfileOwners 返回同一比特主账号下每条 Cloud 记录的当前归属。扫描用它区分
// 「我的」「已登记」「别人的」，再决定是否提供登记入口。
func ListProfileOwners(mainUserID string) (map[string]sharedidentity.UserID, error) {
	rows, err := queryRows(database.DB(),
		`SELECT bit_profile_id, user_id FROM browser_profiles WHERE bit_main_user_id = ?`, mainUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	owners := make(map[string]sharedidentity.UserID)
	for rows.Next() {
		var bitProfileID string
		var userID sharedidentity.UserID
		if err := rows.Scan(&bitProfileID, &userID); err != nil {
			return nil, err
		}
		owners[bitProfileID] = userID
	}
	return owners, rows.Err()
}
