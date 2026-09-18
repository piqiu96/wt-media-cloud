package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	identitymodel "github.com/wt-media/wt-media-cloud/internal/modules/identity/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/id"
	"gorm.io/gorm"
)

const profileColumnsSQL = `id, user_id, team_id, bit_profile_id, main_user_id, profile_user_id, name, seq, group_id, group_name, bit_status, bit_updated_at, proxy_type, proxy_host, proxy_port, proxy_id, remark, cloud_remark, business_status, local_status, last_synced_at, created_at, updated_at`

func FindBinding(userID identitymodel.UserID) (model.BitAccountBinding, bool, error) {
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

func findBinding(db *gorm.DB, userID identitymodel.UserID) (model.BitAccountBinding, bool, error) {
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

func ListProfiles(userID identitymodel.UserID) ([]model.BrowserProfile, error) {
	return listProfilesByUser(database.DB(), userID)
}

func listProfilesByUser(db *gorm.DB, userID identitymodel.UserID) ([]model.BrowserProfile, error) {
	return listProfilesByQuery(db, `SELECT `+profileColumnsSQL+` FROM browser_profiles WHERE user_id = ? ORDER BY id DESC`, userID)
}

func ListAllProfiles() ([]model.BrowserProfile, error) {
	return listAllProfiles(database.DB())
}

func listAllProfiles(db *gorm.DB) ([]model.BrowserProfile, error) {
	return listProfilesByQuery(db, `SELECT `+profileColumnsSQL+` FROM browser_profiles ORDER BY id DESC`)
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

func ResolveProfile(profileID string) (identitymodel.UserID, bool, bool, error) {
	return resolveProfile(database.DB(), profileID)
}

func resolveProfile(db *gorm.DB, profileID string) (identitymodel.UserID, bool, bool, error) {
	var userID identitymodel.UserID
	var status model.ProfileLocalStatus
	err := queryRow(db, `SELECT user_id, local_status FROM browser_profiles WHERE id = ?`, profileID).Scan(&userID, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, false, nil
	}
	if err != nil {
		return 0, false, false, err
	}
	return userID, status == model.ProfileActive, true, nil
}

func ResolveProfileForAccountCheck(profileID string) (string, identitymodel.UserID, string, bool, bool, error) {
	return resolveProfileForAccountCheck(database.DB(), profileID)
}

func resolveProfileForAccountCheck(db *gorm.DB, profileID string) (string, identitymodel.UserID, string, bool, bool, error) {
	var id string
	var userID identitymodel.UserID
	var bitProfileID string
	var status model.ProfileLocalStatus
	err := queryRow(db, `SELECT id, user_id, bit_profile_id, local_status FROM browser_profiles WHERE id = ?`, profileID).
		Scan(&id, &userID, &bitProfileID, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, "", false, false, nil
	}
	if err != nil {
		return "", 0, "", false, false, err
	}
	return id, userID, bitProfileID, status == model.ProfileActive, true, nil
}

// ResolveProxyForAccountCheck returns only the proxy health facts needed to
// render media-account check items. Proxy credentials and source URLs are not
// selected or returned from this cross-module read model.
func ResolveProxyForAccountCheck(profileID string) (string, string, string, *time.Time, bool, error) {
	return resolveProxyForAccountCheck(database.DB(), profileID)
}

func resolveProxyForAccountCheck(db *gorm.DB, profileID string) (string, string, string, *time.Time, bool, error) {
	var proxyID, businessStatus, lastCheckResult sql.NullString
	var expiresAt sql.NullTime
	err := queryRow(db, `SELECT bp.proxy_id, pc.business_status, pc.last_check_result, pc.expires_at
		FROM browser_profiles bp
		JOIN proxy_configs pc ON pc.id = bp.proxy_id
		WHERE bp.id = ? AND bp.proxy_id IS NOT NULL AND bp.proxy_id <> ''`, profileID).
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

func ApplyScan(scan model.ProfileScan, binding model.BitAccountBinding, at time.Time, bindingAuditAction string) error {
	return applyScan(database.DB(), scan, binding, at, bindingAuditAction)
}

func applyScan(db *gorm.DB, scan model.ProfileScan, binding model.BitAccountBinding, at time.Time, bindingAuditAction string) error {
	tx := db.Begin()
	defer rollbackTx(tx)
	result, err := execSQL(tx,
		`UPDATE users SET bit_main_user_id = ?, bit_account_status = ?, bit_account_bound_at = COALESCE(bit_account_bound_at, ?), bit_account_last_verified_at = ?, updated_at = ? WHERE id = ? AND (bit_main_user_id IS NULL OR bit_main_user_id = ?)`,
		binding.MainUserID, binding.Status, binding.BoundAt, binding.LastVerifiedAt, at, binding.UserID, binding.MainUserID,
	)
	if err != nil {
		return err
	}
	if result == 0 {
		return model.ErrIdentityMismatch
	}
	for _, profile := range scan.Profiles {
		// id is omitted (NULL) so MySQL auto-increments; upsert matches on the
		// existing UNIQUE(user_id, bit_profile_id) key. business_status and
		// cloud_remark are Cloud-side and are preserved on duplicate.
		if _, err := execSQL(tx,
			`INSERT INTO browser_profiles (`+profileColumnsSQL+`) VALUES (NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE main_user_id = VALUES(main_user_id), profile_user_id = VALUES(profile_user_id), name = VALUES(name), seq = VALUES(seq), group_id = VALUES(group_id), group_name = VALUES(group_name), remark = VALUES(remark), bit_status = VALUES(bit_status), bit_updated_at = VALUES(bit_updated_at), proxy_type = VALUES(proxy_type), proxy_host = VALUES(proxy_host), proxy_port = VALUES(proxy_port), local_status = VALUES(local_status), last_synced_at = VALUES(last_synced_at), updated_at = VALUES(updated_at)`,
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
		args := []any{scan.UserID}
		for _, bitID := range bitIDs {
			args = append(args, bitID)
		}
		// Accepting a scan cleans up Cloud records for windows that are gone from
		// BitBrowser and not referenced by any media account. Referenced windows
		// are kept as local_missing so account bindings are not orphaned.
		//
		// Delete runtime presence first for ALL missing (non-archived) profiles so
		// the FK from browser_profile_runtime_presence never blocks the profile
		// delete. A correlated subquery inside a multi-table DELETE is unreliable,
		// so the media-account guard lives only on the single-table profile delete.
		cleanupArgs := append(append([]any{}, args...), model.ProfileArchived)
		if _, err := execSQL(tx,
			`DELETE rp FROM browser_profile_runtime_presence rp JOIN browser_profiles bp ON bp.id = rp.profile_id WHERE bp.user_id = ? AND bp.bit_profile_id NOT IN (`+notIn+`) AND bp.local_status <> ?`,
			cleanupArgs...,
		); err != nil {
			return err
		}
		if _, err := execSQL(tx,
			`DELETE bp FROM browser_profiles bp WHERE bp.user_id = ? AND bp.bit_profile_id NOT IN (`+notIn+`) AND NOT EXISTS (SELECT 1 FROM media_accounts ma WHERE ma.browser_profile_id = bp.id) AND bp.local_status <> ?`,
			cleanupArgs...,
		); err != nil {
			return err
		}
		// Remaining missing windows (with account references) are marked local_missing.
		markArgs := []any{model.ProfileLocalMissing, at, at, scan.UserID}
		for _, bitID := range bitIDs {
			markArgs = append(markArgs, bitID)
		}
		markArgs = append(markArgs, model.ProfileArchived)
		if _, err := execSQL(tx,
			`UPDATE browser_profiles SET local_status = ?, last_synced_at = ?, updated_at = ? WHERE user_id = ? AND bit_profile_id NOT IN (`+notIn+`) AND local_status <> ?`,
			markArgs...,
		); err != nil {
			return err
		}
	}
	result, err = execSQL(tx,
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
	bindingSummary, _ := json.Marshal(map[string]any{"main_user_id": scan.MainUserID, "profile_count": len(scan.Profiles), "result": "verified"})
	if _, err := execSQL(tx,
		`INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, summary_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id.NewID("audit"), scan.UserID, bindingAuditAction, "user", scan.UserID, bindingSummary, at,
	); err != nil {
		return err
	}
	return tx.Commit().Error
}

func ConfirmMainIdentity(scan model.ProfileScan, binding model.BitAccountBinding, at time.Time, bindingAuditAction string) error {
	return confirmMainIdentity(database.DB(), scan, binding, at, bindingAuditAction)
}

func confirmMainIdentity(db *gorm.DB, scan model.ProfileScan, binding model.BitAccountBinding, at time.Time, bindingAuditAction string) error {
	tx := db.Begin()
	defer rollbackTx(tx)
	result, err := execSQL(tx,
		`UPDATE users SET bit_main_user_id = ?, bit_account_status = ?, bit_account_bound_at = COALESCE(bit_account_bound_at, ?), bit_account_last_verified_at = ?, updated_at = ? WHERE id = ? AND (bit_main_user_id IS NULL OR bit_main_user_id = ?)`,
		binding.MainUserID, binding.Status, binding.BoundAt, binding.LastVerifiedAt, at, binding.UserID, binding.MainUserID,
	)
	if err != nil {
		return err
	}
	if result == 0 {
		return model.ErrIdentityMismatch
	}
	result, err = execSQL(tx,
		`UPDATE profile_sync_scans SET status = ?, confirmed_at = ? WHERE id = ? AND user_id = ? AND status = ?`,
		model.ScanConfirmed, at, scan.ID, scan.UserID, model.ScanReady,
	)
	if err != nil {
		return err
	}
	if result == 0 {
		return model.ErrScanNotReady
	}
	bindingSummary, _ := json.Marshal(map[string]any{"main_user_id": scan.MainUserID, "profile_count": len(scan.Profiles), "result": "identity_verified", "identity_only": true})
	if _, err := execSQL(tx,
		`INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, summary_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id.NewID("audit"), scan.UserID, bindingAuditAction, "user", scan.UserID, bindingSummary, at,
	); err != nil {
		return err
	}
	return tx.Commit().Error
}

func ConfirmMainIdentityDirect(binding model.BitAccountBinding, at time.Time, bindingAuditAction string) error {
	return confirmMainIdentityDirect(database.DB(), binding, at, bindingAuditAction)
}

func confirmMainIdentityDirect(db *gorm.DB, binding model.BitAccountBinding, at time.Time, bindingAuditAction string) error {
	tx := db.Begin()
	defer rollbackTx(tx)
	result, err := execSQL(tx,
		`UPDATE users SET bit_main_user_id = ?, bit_account_status = ?, bit_account_bound_at = COALESCE(bit_account_bound_at, ?), bit_account_last_verified_at = ?, updated_at = ? WHERE id = ? AND (bit_main_user_id IS NULL OR bit_main_user_id = ?)`,
		binding.MainUserID, binding.Status, binding.BoundAt, binding.LastVerifiedAt, at, binding.UserID, binding.MainUserID,
	)
	if err != nil {
		return err
	}
	if result == 0 {
		return model.ErrIdentityMismatch
	}
	bindingSummary, _ := json.Marshal(map[string]any{"main_user_id": binding.MainUserID, "result": "identity_verified", "identity_only": true})
	if _, err := execSQL(tx,
		`INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, summary_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id.NewID("audit"), binding.UserID, bindingAuditAction, "user", binding.UserID, bindingSummary, at,
	); err != nil {
		return err
	}
	return tx.Commit().Error
}

func ClearMainIdentity(userID identitymodel.UserID, actorID identitymodel.UserID, at time.Time) error {
	return clearMainIdentity(database.DB(), userID, actorID, at)
}

func clearMainIdentity(db *gorm.DB, userID identitymodel.UserID, actorID identitymodel.UserID, at time.Time) error {
	tx := db.Begin()
	defer rollbackTx(tx)
	result, err := execSQL(tx,
		`UPDATE users SET bit_main_user_id = NULL, bit_account_status = NULL, bit_account_bound_at = NULL, bit_account_last_verified_at = NULL, updated_at = ? WHERE id = ?`,
		at, userID,
	)
	if err != nil {
		return err
	}
	if result == 0 {
		return model.ErrInvalidInput
	}
	if _, err := execSQL(tx,
		`UPDATE local_agent_nodes SET status = ?, updated_at = ?, last_heartbeat_at = ? WHERE user_id = ? AND mode = 'local'`,
		"replaced", at, at, userID,
	); err != nil {
		return err
	}
	summary, _ := json.Marshal(map[string]any{"result": "cleared", "kept_profiles": true, "kept_accounts": true, "invalidated_local_nodes": true})
	if _, err := execSQL(tx,
		`INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, summary_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id.NewID("audit"), actorID, "bitbrowser.main_account.clear", "user", userID, summary, at,
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
	// Sync-delete: only called for disabled windows whose BitBrowser profile is
	// gone. Removes stale runtime presence rows (the window no longer exists in
	// BitBrowser) then the Cloud mirror record. Account/task/permit references
	// are guarded in the service layer.
	tx := db.Begin()
	defer rollbackTx(tx)
	if _, err := execSQL(tx, `DELETE FROM browser_profile_runtime_presence WHERE profile_id = ?`, id); err != nil {
		return err
	}
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

func AssignProfileOwner(profileID string, userID identitymodel.UserID, teamID *identitymodel.TeamID, actorID identitymodel.UserID, at time.Time) error {
	return assignProfileOwner(database.DB(), profileID, userID, teamID, actorID, at)
}

func assignProfileOwner(db *gorm.DB, profileID string, userID identitymodel.UserID, teamID *identitymodel.TeamID, actorID identitymodel.UserID, at time.Time) error {
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

func sqlTeamID(teamID *identitymodel.TeamID) any {
	if teamID == nil {
		return nil
	}
	return *teamID
}
