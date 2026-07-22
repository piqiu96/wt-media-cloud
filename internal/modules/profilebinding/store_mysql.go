package profilebinding

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/common"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
)

const profileColumnsSQL = `id, user_id, team_id, bit_profile_id, main_user_id, profile_user_id, name, seq, group_id, group_name, bit_status, bit_updated_at, proxy_type, proxy_host, proxy_port, remark, local_status, last_synced_at, created_at, updated_at`

type MySQLStore struct{ db *sql.DB }

func NewMySQLStore(db *sql.DB) *MySQLStore { return &MySQLStore{db: db} }

func (s *MySQLStore) FindBinding(userID identity.UserID) (BitAccountBinding, bool, error) {
	var binding BitAccountBinding
	var mainUserID, status sql.NullString
	var boundAt, verifiedAt sql.NullTime
	err := s.db.QueryRow(
		`SELECT id, bit_main_user_id, bit_account_status, bit_account_bound_at, bit_account_last_verified_at FROM users WHERE id = ?`, userID,
	).Scan(&binding.UserID, &mainUserID, &status, &boundAt, &verifiedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return BitAccountBinding{}, false, nil
	}
	if err != nil {
		return BitAccountBinding{}, false, err
	}
	if !mainUserID.Valid || mainUserID.String == "" {
		return BitAccountBinding{}, false, nil
	}
	binding.MainUserID = mainUserID.String
	binding.Status = BitAccountStatus(status.String)
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

func (s *MySQLStore) ListProfiles(userID identity.UserID) ([]BrowserProfile, error) {
	return s.listProfiles(`SELECT `+profileColumnsSQL+` FROM browser_profiles WHERE user_id = ? ORDER BY bit_profile_id`, userID)
}

func (s *MySQLStore) ListAllProfiles() ([]BrowserProfile, error) {
	return s.listProfiles(`SELECT ` + profileColumnsSQL + ` FROM browser_profiles ORDER BY bit_profile_id`)
}

func (s *MySQLStore) listProfiles(query string, args ...any) ([]BrowserProfile, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	profiles := make([]BrowserProfile, 0)
	for rows.Next() {
		profile, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	return profiles, rows.Err()
}

func (s *MySQLStore) ResolveProfile(profileID string) (identity.UserID, bool, bool, error) {
	var userID identity.UserID
	var status ProfileLocalStatus
	err := s.db.QueryRow(`SELECT user_id, local_status FROM browser_profiles WHERE id = ?`, profileID).Scan(&userID, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, false, nil
	}
	if err != nil {
		return 0, false, false, err
	}
	return userID, status == ProfileActive, true, nil
}

func (s *MySQLStore) GetProfile(profileID string) (BrowserProfile, bool, error) {
	row := s.db.QueryRow(`SELECT `+profileColumnsSQL+` FROM browser_profiles WHERE id = ?`, profileID)
	profile, err := scanProfile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return BrowserProfile{}, false, nil
	}
	if err != nil {
		return BrowserProfile{}, false, err
	}
	return profile, true, nil
}

func (s *MySQLStore) CreateScan(scan ProfileScan) error {
	diffJSON, err := json.Marshal(scan.Diff)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(
		`INSERT INTO profile_sync_scans (id, user_id, team_id, main_user_id, status, diff_json, created_at, expires_at, confirmed_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		scan.ID, scan.UserID, scan.TeamID, scan.MainUserID, scan.Status, diffJSON, scan.CreatedAt, scan.ExpiresAt, scan.ConfirmedAt,
	); err != nil {
		return err
	}
	for _, profile := range scan.Profiles {
		if _, err := tx.Exec(
			`INSERT INTO profile_sync_candidates (scan_id, profile_id, bit_profile_id, main_user_id, profile_user_id, name, seq, group_id, group_name, bit_status, bit_updated_at, proxy_type, proxy_host, proxy_port, remark, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			scan.ID, profile.ID, profile.BitProfileID, profile.MainUserID, profile.ProfileUserID, profile.Name, profile.Seq,
			nullIfEmpty(profile.GroupID), nullIfEmpty(profile.GroupName), nullIfEmpty(profile.BitStatus), nullIfEmpty(profile.BitUpdatedAt),
			nullIfEmpty(profile.ProxyType), nullIfEmpty(profile.ProxyHost), profile.ProxyPort, nullIfEmpty(profile.Remark), profile.CreatedAt, profile.UpdatedAt,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *MySQLStore) FindScan(scanID string) (ProfileScan, bool, error) {
	var scan ProfileScan
	var diffJSON []byte
	var confirmedAt sql.NullTime
	err := s.db.QueryRow(
		`SELECT id, user_id, team_id, main_user_id, status, diff_json, created_at, expires_at, confirmed_at FROM profile_sync_scans WHERE id = ?`, scanID,
	).Scan(&scan.ID, &scan.UserID, &scan.TeamID, &scan.MainUserID, &scan.Status, &diffJSON, &scan.CreatedAt, &scan.ExpiresAt, &confirmedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ProfileScan{}, false, nil
	}
	if err != nil {
		return ProfileScan{}, false, err
	}
	if err := json.Unmarshal(diffJSON, &scan.Diff); err != nil {
		return ProfileScan{}, false, err
	}
	if confirmedAt.Valid {
		value := confirmedAt.Time
		scan.ConfirmedAt = &value
	}
	rows, err := s.db.Query(
		`SELECT profile_id, bit_profile_id, main_user_id, profile_user_id, name, seq, group_id, group_name, bit_status, bit_updated_at, proxy_type, proxy_host, proxy_port, remark, created_at, updated_at FROM profile_sync_candidates WHERE scan_id = ? ORDER BY bit_profile_id`, scanID,
	)
	if err != nil {
		return ProfileScan{}, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var profile BrowserProfile
		var groupID, groupName, bitStatus, bitUpdatedAt, proxyType, proxyHost, remark sql.NullString
		var proxyPort sql.NullInt64
		if err := rows.Scan(&profile.ID, &profile.BitProfileID, &profile.MainUserID, &profile.ProfileUserID, &profile.Name, &profile.Seq, &groupID, &groupName, &bitStatus, &bitUpdatedAt, &proxyType, &proxyHost, &proxyPort, &remark, &profile.CreatedAt, &profile.UpdatedAt); err != nil {
			return ProfileScan{}, false, err
		}
		profile.UserID = scan.UserID
		profile.TeamID = scan.TeamID
		profile.GroupID, profile.GroupName, profile.BitStatus, profile.BitUpdatedAt = groupID.String, groupName.String, bitStatus.String, bitUpdatedAt.String
		profile.ProxyType, profile.ProxyHost, profile.Remark = proxyType.String, proxyHost.String, remark.String
		if proxyPort.Valid {
			profile.ProxyPort = int(proxyPort.Int64)
		}
		profile.LocalStatus = ProfileActive
		profile.LastSyncedAt = scan.CreatedAt
		scan.Profiles = append(scan.Profiles, profile)
	}
	if err := rows.Err(); err != nil {
		return ProfileScan{}, false, err
	}
	return scan, true, nil
}

func (s *MySQLStore) ApplyScan(scan ProfileScan, binding BitAccountBinding, at time.Time, bindingAuditAction string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(
		`UPDATE users SET bit_main_user_id = ?, bit_account_status = ?, bit_account_bound_at = COALESCE(bit_account_bound_at, ?), bit_account_last_verified_at = ?, updated_at = ? WHERE id = ? AND (bit_main_user_id IS NULL OR bit_main_user_id = ?)`,
		binding.MainUserID, binding.Status, binding.BoundAt, binding.LastVerifiedAt, at, binding.UserID, binding.MainUserID,
	)
	if err != nil {
		return err
	}
	if rows, err := result.RowsAffected(); err != nil || rows == 0 {
		if err != nil {
			return err
		}
		return ErrIdentityMismatch
	}
	for _, profile := range scan.Profiles {
		if _, err := tx.Exec(
			`INSERT INTO browser_profiles (`+profileColumnsSQL+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE main_user_id = VALUES(main_user_id), profile_user_id = VALUES(profile_user_id), name = VALUES(name), seq = VALUES(seq), group_id = VALUES(group_id), group_name = VALUES(group_name), bit_status = VALUES(bit_status), bit_updated_at = VALUES(bit_updated_at), proxy_type = VALUES(proxy_type), proxy_host = VALUES(proxy_host), proxy_port = VALUES(proxy_port), local_status = VALUES(local_status), last_synced_at = VALUES(last_synced_at), updated_at = VALUES(updated_at)`,
			profile.ID, profile.UserID, profile.TeamID, profile.BitProfileID, profile.MainUserID, profile.ProfileUserID, profile.Name, profile.Seq,
			nullIfEmpty(profile.GroupID), nullIfEmpty(profile.GroupName), nullIfEmpty(profile.BitStatus), nullIfEmpty(profile.BitUpdatedAt),
			nullIfEmpty(profile.ProxyType), nullIfEmpty(profile.ProxyHost), profile.ProxyPort, nullIfEmpty(profile.Remark),
			ProfileActive, at, profile.CreatedAt, at,
		); err != nil {
			return err
		}
	}
	bitIDs := make([]string, 0, len(scan.Profiles))
	args := make([]any, 0, 5+len(scan.Profiles))
	args = append(args, ProfileLocalMissing, at, at, scan.UserID)
	for _, profile := range scan.Profiles {
		bitIDs = append(bitIDs, profile.BitProfileID)
		args = append(args, profile.BitProfileID)
	}
	args = append(args, ProfileArchived)
	if _, err := tx.Exec(
		`UPDATE browser_profiles SET local_status = ?, last_synced_at = ?, updated_at = ? WHERE user_id = ? AND bit_profile_id NOT IN (`+placeholders(len(bitIDs))+`) AND local_status <> ?`,
		args...,
	); err != nil {
		return err
	}
	result, err = tx.Exec(
		`UPDATE profile_sync_scans SET status = ?, confirmed_at = ? WHERE id = ? AND user_id = ? AND status = ?`,
		ScanConfirmed, at, scan.ID, scan.UserID, ScanReady,
	)
	if err != nil {
		return err
	}
	if rows, err := result.RowsAffected(); err != nil || rows == 0 {
		if err != nil {
			return err
		}
		return ErrScanNotReady
	}
	summary, _ := json.Marshal(map[string]any{"main_user_id": scan.MainUserID, "profile_count": len(scan.Profiles)})
	if _, err := tx.Exec(
		`INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, summary_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		common.NewID("audit"), scan.UserID, "bitbrowser.profile_scan.confirm", "profile_sync_scan", scan.ID, summary, at,
	); err != nil {
		return err
	}
	bindingSummary, _ := json.Marshal(map[string]any{"main_user_id": scan.MainUserID, "profile_count": len(scan.Profiles), "result": "verified"})
	if _, err := tx.Exec(
		`INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, summary_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		common.NewID("audit"), scan.UserID, bindingAuditAction, "user", scan.UserID, bindingSummary, at,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) ConfirmMainIdentity(scan ProfileScan, binding BitAccountBinding, at time.Time, bindingAuditAction string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(
		`UPDATE users SET bit_main_user_id = ?, bit_account_status = ?, bit_account_bound_at = COALESCE(bit_account_bound_at, ?), bit_account_last_verified_at = ?, updated_at = ? WHERE id = ? AND (bit_main_user_id IS NULL OR bit_main_user_id = ?)`,
		binding.MainUserID, binding.Status, binding.BoundAt, binding.LastVerifiedAt, at, binding.UserID, binding.MainUserID,
	)
	if err != nil {
		return err
	}
	if rows, err := result.RowsAffected(); err != nil || rows == 0 {
		if err != nil {
			return err
		}
		return ErrIdentityMismatch
	}
	result, err = tx.Exec(
		`UPDATE profile_sync_scans SET status = ?, confirmed_at = ? WHERE id = ? AND user_id = ? AND status = ?`,
		ScanConfirmed, at, scan.ID, scan.UserID, ScanReady,
	)
	if err != nil {
		return err
	}
	if rows, err := result.RowsAffected(); err != nil || rows == 0 {
		if err != nil {
			return err
		}
		return ErrScanNotReady
	}
	bindingSummary, _ := json.Marshal(map[string]any{"main_user_id": scan.MainUserID, "profile_count": len(scan.Profiles), "result": "identity_verified", "identity_only": true})
	if _, err := tx.Exec(
		`INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, summary_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		common.NewID("audit"), scan.UserID, bindingAuditAction, "user", scan.UserID, bindingSummary, at,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) ConfirmMainIdentityDirect(binding BitAccountBinding, at time.Time, bindingAuditAction string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(
		`UPDATE users SET bit_main_user_id = ?, bit_account_status = ?, bit_account_bound_at = COALESCE(bit_account_bound_at, ?), bit_account_last_verified_at = ?, updated_at = ? WHERE id = ? AND (bit_main_user_id IS NULL OR bit_main_user_id = ?)`,
		binding.MainUserID, binding.Status, binding.BoundAt, binding.LastVerifiedAt, at, binding.UserID, binding.MainUserID,
	)
	if err != nil {
		return err
	}
	if rows, err := result.RowsAffected(); err != nil || rows == 0 {
		if err != nil {
			return err
		}
		return ErrIdentityMismatch
	}
	bindingSummary, _ := json.Marshal(map[string]any{"main_user_id": binding.MainUserID, "result": "identity_verified", "identity_only": true})
	if _, err := tx.Exec(
		`INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, summary_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		common.NewID("audit"), binding.UserID, bindingAuditAction, "user", binding.UserID, bindingSummary, at,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) ClearMainIdentity(userID identity.UserID, actorID identity.UserID, at time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(
		`UPDATE users SET bit_main_user_id = NULL, bit_account_status = NULL, bit_account_bound_at = NULL, bit_account_last_verified_at = NULL, updated_at = ? WHERE id = ?`,
		at, userID,
	)
	if err != nil {
		return err
	}
	if rows, err := result.RowsAffected(); err != nil || rows == 0 {
		if err != nil {
			return err
		}
		return ErrInvalidInput
	}
	if _, err := tx.Exec(
		`UPDATE local_agent_nodes SET status = ?, updated_at = ?, last_heartbeat_at = ? WHERE user_id = ? AND mode = 'local'`,
		"replaced", at, at, userID,
	); err != nil {
		return err
	}
	summary, _ := json.Marshal(map[string]any{"result": "cleared", "kept_profiles": true, "kept_accounts": true, "invalidated_local_nodes": true})
	if _, err := tx.Exec(
		`INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, summary_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		common.NewID("audit"), actorID, "bitbrowser.main_account.clear", "user", userID, summary, at,
	); err != nil {
		return err
	}
	return tx.Commit()
}

type scanner interface{ Scan(...any) error }

func scanProfile(row scanner) (BrowserProfile, error) {
	var profile BrowserProfile
	var groupID, groupName, bitStatus, bitUpdatedAt, proxyType, proxyHost, remark sql.NullString
	var proxyPort sql.NullInt64
	err := row.Scan(&profile.ID, &profile.UserID, &profile.TeamID, &profile.BitProfileID, &profile.MainUserID, &profile.ProfileUserID, &profile.Name, &profile.Seq, &groupID, &groupName, &bitStatus, &bitUpdatedAt, &proxyType, &proxyHost, &proxyPort, &remark, &profile.LocalStatus, &profile.LastSyncedAt, &profile.CreatedAt, &profile.UpdatedAt)
	if err != nil {
		return BrowserProfile{}, err
	}
	profile.GroupID, profile.GroupName, profile.BitStatus, profile.BitUpdatedAt = groupID.String, groupName.String, bitStatus.String, bitUpdatedAt.String
	profile.ProxyType, profile.ProxyHost, profile.Remark = proxyType.String, proxyHost.String, remark.String
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

func (s *MySQLStore) DeleteProfile(id string) error {
	_, err := s.db.Exec(`DELETE FROM browser_profiles WHERE id = ?`, id)
	return err
}

func (s *MySQLStore) ProfileHasAccountReferences(profileID string) (bool, error) {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM media_accounts WHERE browser_profile_id = ?`, profileID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *MySQLStore) AssignProfileOwner(profileID string, userID identity.UserID, teamID *identity.TeamID, actorID identity.UserID, at time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(
		`UPDATE browser_profiles SET user_id = ?, team_id = ?, updated_at = ? WHERE id = ?`,
		userID, sqlTeamID(teamID), at, profileID,
	)
	if err != nil {
		return err
	}
	if rows, err := result.RowsAffected(); err != nil || rows == 0 {
		if err != nil {
			return err
		}
		return ErrProfileNotFound
	}
	summary, _ := json.Marshal(map[string]any{"user_id": userID, "team_id": teamID})
	if _, err := tx.Exec(
		`INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, summary_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		common.NewID("audit"), actorID, "bitbrowser.profile.assign_owner", "browser_profile", profileID, summary, at,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func sqlTeamID(teamID *identity.TeamID) any {
	if teamID == nil {
		return nil
	}
	return *teamID
}

var _ Store = (*MySQLStore)(nil)
