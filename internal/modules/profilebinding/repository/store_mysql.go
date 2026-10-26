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

const profileColumnsSQL = `id, user_id, team_id, bit_profile_id, main_user_id, profile_user_id, name, seq, group_id, group_name, bit_status, bit_updated_at, proxy_type, proxy_host, proxy_port, proxy_id, remark, cloud_remark, business_status, local_status, last_synced_at, created_at, updated_at`

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
	binding.MainUserID = mainUserID.String
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
	UserID         sharedidentity.UserID
	ID             string
	Name           string
	Group          string
	BitProfileID   string
	Remark         string
	LocalStatus    string
	BusinessStatus string
}

func ListProfiles(filter ProfileListFilter) ([]model.BrowserProfile, error) {
	return listProfiles(database.DB(), filter)
}

func listProfiles(db *gorm.DB, filter ProfileListFilter) ([]model.BrowserProfile, error) {
	// join 是作用域的另一半：只有 main_user_id 仍等于所有者当前绑定时行才可见。
	// 旧主账号下的行继续保留，但在重新绑定该主账号之前不会出现在任何正常查询中。
	query := `SELECT ` + profileColumnsWithPrefix("bp") + ` FROM browser_profiles bp JOIN users u ON u.id = bp.user_id WHERE bp.main_user_id = u.bit_main_user_id`
	args := make([]any, 0, 10)
	if filter.UserID > 0 {
		query += ` AND bp.user_id = ?`
		args = append(args, filter.UserID)
	}
	if filter.ID != "" {
		query += ` AND bp.id LIKE ? ESCAPE '!'`
		args = append(args, profileLikeContains(filter.ID))
	}
	if filter.Name != "" {
		query += ` AND bp.name LIKE ? ESCAPE '!'`
		args = append(args, profileLikeContains(filter.Name))
	}
	if filter.Group != "" {
		like := profileLikeContains(filter.Group)
		query += ` AND (bp.group_id LIKE ? ESCAPE '!' OR bp.group_name LIKE ? ESCAPE '!')`
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
	if filter.LocalStatus != "" {
		query += ` AND bp.local_status = ?`
		args = append(args, filter.LocalStatus)
	}
	if filter.BusinessStatus != "" {
		query += ` AND bp.business_status = ?`
		args = append(args, filter.BusinessStatus)
	}
	// BitBrowser scans keep the scanned order; the page shows the newest first.
	query += ` ORDER BY bp.seq DESC, bp.id DESC`
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
	var status model.ProfileLocalStatus
	var inScope bool
	err := queryRow(db,
		`SELECT bp.user_id, bp.local_status, IFNULL(bp.main_user_id = u.bit_main_user_id, 0) FROM browser_profiles bp JOIN users u ON u.id = bp.user_id WHERE bp.id = ?`,
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
	var status model.ProfileLocalStatus
	var inScope bool
	err := queryRow(db,
		`SELECT bp.id, bp.user_id, bp.bit_profile_id, bp.local_status, IFNULL(bp.main_user_id = u.bit_main_user_id, 0) FROM browser_profiles bp JOIN users u ON u.id = bp.user_id WHERE bp.id = ?`,
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
func ResolveProxyForAccountCheck(profileID string) (string, string, string, *time.Time, bool, error) {
	return resolveProxyForAccountCheck(database.DB(), profileID)
}

func resolveProxyForAccountCheck(db *gorm.DB, profileID string) (string, string, string, *time.Time, bool, error) {
	var proxyID, businessStatus, lastCheckResult sql.NullString
	var expiresAt sql.NullTime
	err := queryRow(db, `SELECT bp.proxy_id, pc.business_status, pc.last_check_result, pc.expires_at
		FROM browser_profiles bp
		JOIN users u ON u.id = bp.user_id
		JOIN proxy_configs pc ON pc.id = bp.proxy_id
		WHERE bp.id = ? AND bp.proxy_id IS NOT NULL AND bp.proxy_id <> '' AND bp.main_user_id = u.bit_main_user_id`, profileID).
		Scan(&proxyID, &businessStatus, &lastCheckResult, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", "", nil, false, nil
	}
	if err != nil {
		return "", "", "", nil, false, err
	}
	var value *time.Time
	if expiresAt.Valid {
		at := expiresAt.Time
		value = &at
	}
	return proxyID.String, businessStatus.String, lastCheckResult.String, value, proxyID.String != "", nil
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
		`SELECT `+profileColumnsWithPrefix("bp")+` FROM browser_profiles bp JOIN users u ON u.id = bp.user_id WHERE bp.id = ? AND bp.main_user_id = u.bit_main_user_id`,
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

func CreateScan(scan model.ProfileScan) error {
	return createScan(database.DB(), scan)
}

func createScan(db *gorm.DB, scan model.ProfileScan) error {
	diffJSON, err := json.Marshal(scan.Diff)
	if err != nil {
		return err
	}
	tx := db.Begin()
	defer rollbackTx(tx)
	if _, err := execSQL(tx,
		`INSERT INTO profile_sync_scans (id, user_id, team_id, main_user_id, status, diff_json, created_at, expires_at, confirmed_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		scan.ID, scan.UserID, scan.TeamID, scan.MainUserID, scan.Status, diffJSON, scan.CreatedAt, scan.ExpiresAt, scan.ConfirmedAt,
	); err != nil {
		return err
	}
	for _, profile := range scan.Profiles {
		if _, err := execSQL(tx,
			`INSERT INTO profile_sync_candidates (scan_id, profile_id, bit_profile_id, main_user_id, profile_user_id, name, seq, group_id, group_name, bit_status, bit_updated_at, proxy_type, proxy_host, proxy_port, remark, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			scan.ID, profile.ID, profile.BitProfileID, profile.MainUserID, profile.ProfileUserID, profile.Name, profile.Seq,
			nullIfEmpty(profile.GroupID), nullIfEmpty(profile.GroupName), nullIfEmpty(profile.BitStatus), nullIfEmpty(profile.BitUpdatedAt),
			nullIfEmpty(profile.ProxyType), nullIfEmpty(profile.ProxyHost), profile.ProxyPort, nullIfEmpty(profile.Remark), profile.CreatedAt, profile.UpdatedAt,
		); err != nil {
			return err
		}
	}
	return tx.Commit().Error
}

func FindScan(scanID string) (model.ProfileScan, bool, error) {
	return findScan(database.DB(), scanID)
}

func findScan(db *gorm.DB, scanID string) (model.ProfileScan, bool, error) {
	var scan model.ProfileScan
	var diffJSON []byte
	var confirmedAt sql.NullTime
	err := queryRow(db,
		`SELECT id, user_id, team_id, main_user_id, status, diff_json, created_at, expires_at, confirmed_at FROM profile_sync_scans WHERE id = ?`, scanID,
	).Scan(&scan.ID, &scan.UserID, &scan.TeamID, &scan.MainUserID, &scan.Status, &diffJSON, &scan.CreatedAt, &scan.ExpiresAt, &confirmedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ProfileScan{}, false, nil
	}
	if err != nil {
		return model.ProfileScan{}, false, err
	}
	if err := json.Unmarshal(diffJSON, &scan.Diff); err != nil {
		return model.ProfileScan{}, false, err
	}
	if confirmedAt.Valid {
		value := confirmedAt.Time
		scan.ConfirmedAt = &value
	}
	rows, err := queryRows(db,
		`SELECT profile_id, bit_profile_id, main_user_id, profile_user_id, name, seq, group_id, group_name, bit_status, bit_updated_at, proxy_type, proxy_host, proxy_port, remark, created_at, updated_at FROM profile_sync_candidates WHERE scan_id = ? ORDER BY bit_profile_id`, scanID,
	)
	if err != nil {
		return model.ProfileScan{}, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var profile model.BrowserProfile
		var groupID, groupName, bitStatus, bitUpdatedAt, proxyType, proxyHost, remark sql.NullString
		var proxyPort sql.NullInt64
		if err := rows.Scan(&profile.ID, &profile.BitProfileID, &profile.MainUserID, &profile.ProfileUserID, &profile.Name, &profile.Seq, &groupID, &groupName, &bitStatus, &bitUpdatedAt, &proxyType, &proxyHost, &proxyPort, &remark, &profile.CreatedAt, &profile.UpdatedAt); err != nil {
			return model.ProfileScan{}, false, err
		}
		profile.UserID = scan.UserID
		profile.TeamID = scan.TeamID
		profile.GroupID, profile.GroupName, profile.BitStatus, profile.BitUpdatedAt = groupID.String, groupName.String, bitStatus.String, bitUpdatedAt.String
		profile.ProxyType, profile.ProxyHost, profile.Remark = proxyType.String, proxyHost.String, remark.String
		if proxyPort.Valid {
			profile.ProxyPort = int(proxyPort.Int64)
		}
		profile.LocalStatus = model.ProfileActive
		profile.LastSyncedAt = scan.CreatedAt
		scan.Profiles = append(scan.Profiles, profile)
	}
	if err := rows.Err(); err != nil {
		return model.ProfileScan{}, false, err
	}
	return scan, true, nil
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
		UserID: userID, MainUserID: mainUserID, Status: model.BitAccountBound,
		BoundAt: &firstBoundAt, LastVerifiedAt: &at,
	}, nil
}

func ApplyScan(scan model.ProfileScan, at time.Time) error {
	return applyScan(database.DB(), scan, at)
}

func applyScan(db *gorm.DB, scan model.ProfileScan, at time.Time) error {
	tx := db.Begin()
	defer rollbackTx(tx)
	// 提交扫描前再次核对绑定：扫描暂存期间用户可能已重绑，旧扫描不能写入新主账号。
	var boundMain sql.NullString
	err := queryRow(tx, `SELECT bit_main_user_id FROM users WHERE id = ? FOR UPDATE`, scan.UserID).Scan(&boundMain)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrForbidden
	}
	if err != nil {
		return err
	}
	if !boundMain.Valid || boundMain.String == "" {
		return model.ErrBitNotBound
	}
	if boundMain.String != scan.MainUserID {
		return model.ErrBitMainUserMismatch
	}
	for _, profile := range scan.Profiles {
		// 唯一键是 (user_id, main_user_id, bit_profile_id)：同一 bit_profile_id 在
		// 不同主账号下会插入新行。business_status 与 cloud_remark 属于 Cloud 侧，
		// 重复时不覆盖。
		if _, err := execSQL(tx,
			`INSERT INTO browser_profiles (`+profileColumnsSQL+`) VALUES (NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE profile_user_id = VALUES(profile_user_id), name = VALUES(name), seq = VALUES(seq), group_id = VALUES(group_id), group_name = VALUES(group_name), remark = VALUES(remark), bit_status = VALUES(bit_status), bit_updated_at = VALUES(bit_updated_at), proxy_type = VALUES(proxy_type), proxy_host = VALUES(proxy_host), proxy_port = VALUES(proxy_port), local_status = VALUES(local_status), last_synced_at = VALUES(last_synced_at), updated_at = VALUES(updated_at)`,
			profile.UserID, profile.TeamID, profile.BitProfileID, profile.MainUserID, profile.ProfileUserID, profile.Name, profile.Seq,
			nullIfEmpty(profile.GroupID), nullIfEmpty(profile.GroupName), nullIfEmpty(profile.BitStatus), nullIfEmpty(profile.BitUpdatedAt),
			nullIfEmpty(profile.ProxyType), nullIfEmpty(profile.ProxyHost), profile.ProxyPort, nil, nullIfEmpty(profile.Remark),
			"", model.ProfileBusinessEnabled, model.ProfileActive, at, profile.CreatedAt, at,
		); err != nil {
			return err
		}
	}
	bitIDs := make([]string, 0, len(scan.Profiles))
	for _, profile := range scan.Profiles {
		bitIDs = append(bitIDs, profile.BitProfileID)
	}
	notIn := placeholders(len(bitIDs))
	if len(bitIDs) > 0 {
		// 缺失窗口只标记 local_missing，且只作用于同一 user + main。不删除行：
		// 切回旧主账号时这些 Profile 需要恢复可见，媒体账号引用也不能被清空。
		markArgs := []any{model.ProfileLocalMissing, at, at, scan.UserID, scan.MainUserID}
		for _, bitID := range bitIDs {
			markArgs = append(markArgs, bitID)
		}
		markArgs = append(markArgs, model.ProfileArchived)
		if _, err := execSQL(tx,
			`UPDATE browser_profiles SET local_status = ?, last_synced_at = ?, updated_at = ? WHERE user_id = ? AND main_user_id = ? AND bit_profile_id NOT IN (`+notIn+`) AND local_status <> ?`,
			markArgs...,
		); err != nil {
			return err
		}
	}
	result, err := execSQL(tx,
		`UPDATE profile_sync_scans SET status = ?, confirmed_at = ? WHERE id = ? AND user_id = ? AND status = ?`,
		model.ScanConfirmed, at, scan.ID, scan.UserID, model.ScanReady,
	)
	if err != nil {
		return err
	}
	if result == 0 {
		return model.ErrScanNotReady
	}
	summary, _ := json.Marshal(map[string]any{"main_user_id": scan.MainUserID, "profile_count": len(scan.Profiles)})
	if _, err := execSQL(tx,
		`INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, summary_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id.NewID("audit"), scan.UserID, "bitbrowser.profile_scan.confirm", "profile_sync_scan", scan.ID, summary, at,
	); err != nil {
		return err
	}
	return tx.Commit().Error
}

type scanner interface{ Scan(...any) error }

func scanProfile(row scanner) (model.BrowserProfile, error) {
	var profile model.BrowserProfile
	var groupID, groupName, bitStatus, bitUpdatedAt, proxyType, proxyHost, proxyID, remark, cloudRemark sql.NullString
	var proxyPort sql.NullInt64
	err := row.Scan(&profile.ID, &profile.UserID, &profile.TeamID, &profile.BitProfileID, &profile.MainUserID, &profile.ProfileUserID, &profile.Name, &profile.Seq, &groupID, &groupName, &bitStatus, &bitUpdatedAt, &proxyType, &proxyHost, &proxyPort, &proxyID, &remark, &cloudRemark, &profile.BusinessStatus, &profile.LocalStatus, &profile.LastSyncedAt, &profile.CreatedAt, &profile.UpdatedAt)
	if err != nil {
		return model.BrowserProfile{}, err
	}
	profile.GroupID, profile.GroupName, profile.BitStatus, profile.BitUpdatedAt = groupID.String, groupName.String, bitStatus.String, bitUpdatedAt.String
	profile.ProxyType, profile.ProxyHost, profile.ProxyID, profile.Remark = proxyType.String, proxyHost.String, proxyID.String, remark.String
	profile.CloudRemark = cloudRemark.String
	if proxyPort.Valid {
		profile.ProxyPort = int(proxyPort.Int64)
	}
	return profile, nil
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func placeholders(count int) string { return strings.TrimRight(strings.Repeat("?, ", count), ", ") }

func DeleteProfile(id string) error {
	return deleteProfile(database.DB(), id)
}

func deleteProfile(db *gorm.DB, id string) error {
	// 只用于删除 BitBrowser 侧已消失的停用窗口；账号/任务/permit 引用由
	// service 层先行拦截。
	tx := db.Begin()
	defer rollbackTx(tx)
	if _, err := execSQL(tx, `DELETE FROM browser_profiles WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit().Error
}

func ProfileHasDependencies(profileID string) (bool, error) {
	return profileHasDependencies(database.DB(), profileID)
}

func profileHasDependencies(db *gorm.DB, profileID string) (bool, error) {
	var count int
	if err := queryRow(db,
		`SELECT (
			(SELECT COUNT(*) FROM media_accounts WHERE browser_profile_id = ?) +
			(SELECT COUNT(*) FROM sensitive_browser_tasks WHERE profile_id = ?) +
			(SELECT COUNT(*) FROM sensitive_profile_permits WHERE profile_id = ?)
		)`, profileID, profileID, profileID,
	).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func UpdateProfile(profileID string, cloudRemark *string, businessStatus *model.ProfileBusinessStatus, at time.Time) error {
	return updateProfile(database.DB(), profileID, cloudRemark, businessStatus, at)
}

func updateProfile(db *gorm.DB, profileID string, cloudRemark *string, businessStatus *model.ProfileBusinessStatus, at time.Time) error {
	var sets []string
	var args []any
	if cloudRemark != nil {
		sets = append(sets, "cloud_remark = ?")
		args = append(args, nullIfEmpty(*cloudRemark))
	}
	if businessStatus != nil {
		sets = append(sets, "business_status = ?")
		args = append(args, string(*businessStatus))
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, at)
	args = append(args, profileID)
	_, err := execSQL(db, `UPDATE browser_profiles SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...)
	return err
}

func CountProfilesByProxyID(proxyID string) (int, error) {
	return countProfilesByProxyID(database.DB(), proxyID)
}

func countProfilesByProxyID(db *gorm.DB, proxyID string) (int, error) {
	var count int
	if err := queryRow(db, `SELECT COUNT(*) FROM browser_profiles WHERE proxy_id = ? AND local_status = ?`, proxyID, model.ProfileActive).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func BindProxy(profileID, proxyID, proxyType, proxyHost string, proxyPort int) (model.BrowserProfile, error) {
	return bindProxy(database.DB(), profileID, proxyID, proxyType, proxyHost, proxyPort)
}

func bindProxy(db *gorm.DB, profileID, proxyID, proxyType, proxyHost string, proxyPort int) (model.BrowserProfile, error) {
	result, err := execSQL(db,
		`UPDATE browser_profiles SET proxy_id = ?, proxy_type = ?, proxy_host = ?, proxy_port = ?, updated_at = ? WHERE id = ?`,
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
		`UPDATE browser_profiles SET proxy_id = NULL, proxy_type = NULL, proxy_host = NULL, proxy_port = 0, updated_at = ? WHERE id = ? AND proxy_id = ?`,
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

func AssignProfileOwner(profileID string, userID sharedidentity.UserID, teamID *sharedidentity.TeamID, actorID sharedidentity.UserID, at time.Time) error {
	return assignProfileOwner(database.DB(), profileID, userID, teamID, actorID, at)
}

func assignProfileOwner(db *gorm.DB, profileID string, userID sharedidentity.UserID, teamID *sharedidentity.TeamID, actorID sharedidentity.UserID, at time.Time) error {
	tx := db.Begin()
	defer rollbackTx(tx)
	result, err := execSQL(tx,
		`UPDATE browser_profiles SET user_id = ?, team_id = ?, updated_at = ? WHERE id = ?`,
		userID, sqlTeamID(teamID), at, profileID,
	)
	if err != nil {
		return err
	}
	if result == 0 {
		return model.ErrProfileNotFound
	}
	summary, _ := json.Marshal(map[string]any{"user_id": userID, "team_id": teamID})
	if _, err := execSQL(tx,
		`INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, summary_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id.NewID("audit"), actorID, "bitbrowser.profile.assign_owner", "browser_profile", profileID, summary, at,
	); err != nil {
		return err
	}
	return tx.Commit().Error
}

func sqlTeamID(teamID *sharedidentity.TeamID) any {
	if teamID == nil {
		return nil
	}
	return *teamID
}
