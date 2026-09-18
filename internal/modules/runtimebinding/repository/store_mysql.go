package repository

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	identitymodel "github.com/wt-media/wt-media-cloud/internal/modules/identity/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/id"
	"gorm.io/gorm"
)

func CreateTicket(ticket model.BindingTicket) error {
	return createTicket(database.DB(), ticket)
}

func createTicket(db *gorm.DB, ticket model.BindingTicket) error {
	return db.Exec(`INSERT INTO local_agent_binding_tickets
		(id, user_id, session_id, token_hash, created_at, expires_at, used_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		ticket.ID, ticket.UserID, ticket.SessionID, ticket.TokenHash, ticket.CreatedAt, ticket.ExpiresAt, ticket.UsedAt,
	).Error
}

func ConsumeTicket(tokenHash string, at time.Time) (model.BindingTicket, bool, error) {
	return consumeTicket(database.DB(), tokenHash, at)
}

func consumeTicket(db *gorm.DB, tokenHash string, at time.Time) (model.BindingTicket, bool, error) {
	var ticket model.BindingTicket
	var usedAt sql.NullTime
	found := false
	err := db.Transaction(func(tx *gorm.DB) error {
		err := tx.Raw(`SELECT id, user_id, session_id, token_hash, created_at, expires_at, used_at
			FROM local_agent_binding_tickets WHERE token_hash = ? FOR UPDATE`, tokenHash).
			Row().Scan(&ticket.ID, &ticket.UserID, &ticket.SessionID, &ticket.TokenHash, &ticket.CreatedAt, &ticket.ExpiresAt, &usedAt)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if usedAt.Valid || !ticket.ExpiresAt.After(at) {
			return nil
		}
		result := tx.Exec(`UPDATE local_agent_binding_tickets SET used_at = ? WHERE id = ? AND used_at IS NULL`, at, ticket.ID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nil
		}
		ticket.UsedAt = &at
		found = true
		return nil
	})
	if err != nil {
		return model.BindingTicket{}, false, err
	}
	return ticket, found, nil
}

func IsSessionActive(sessionID string, userID identitymodel.UserID, at time.Time) (bool, error) {
	return isSessionActive(database.DB(), sessionID, userID, at)
}

func isSessionActive(db *gorm.DB, sessionID string, userID identitymodel.UserID, _ time.Time) (bool, error) {
	var active bool
	err := db.Raw(`SELECT EXISTS(SELECT 1 FROM user_sessions s JOIN users u ON u.id = s.user_id
		WHERE s.id = ? AND s.user_id = ? AND s.invalidated_at IS NULL AND u.status = 'enabled')`, sessionID, userID).
		Row().Scan(&active)
	return active, err
}

func SaveNode(node model.AgentNode) error { return saveNode(database.DB(), node) }

func saveNode(db *gorm.DB, node model.AgentNode) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`UPDATE local_agent_nodes SET status = 'replaced', updated_at = ?
			WHERE user_id = ? AND device_id = ? AND status <> 'replaced'`, node.RegisteredAt, node.UserID, node.DeviceID).Error; err != nil {
			return err
		}
		return tx.Exec(`INSERT INTO local_agent_nodes
			(id, agent_id, device_id, user_id, session_id, mode, agent_version, contract_major_version, contract_revision, credential_hash, status, registered_at, last_heartbeat_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			node.ID, node.AgentID, node.DeviceID, node.UserID, node.SessionID, node.Mode, node.AgentVersion,
			node.ContractMajorVersion, node.ContractRevision, node.CredentialHash, node.Status,
			node.RegisteredAt, node.LastHeartbeatAt, node.RegisteredAt,
		).Error
	})
}

func FindNodeByCredentialHash(hash string) (model.AgentNode, bool, error) {
	return findNodeByCredentialHash(database.DB(), hash)
}

func findNodeByCredentialHash(db *gorm.DB, hash string) (model.AgentNode, bool, error) {
	var node model.AgentNode
	err := db.Raw(`SELECT id, agent_id, device_id, user_id, session_id, mode, agent_version,
		contract_major_version, contract_revision, credential_hash, status, registered_at, last_heartbeat_at
		FROM local_agent_nodes WHERE credential_hash = ?`, hash).
		Row().Scan(&node.ID, &node.AgentID, &node.DeviceID, &node.UserID, &node.SessionID, &node.Mode, &node.AgentVersion,
		&node.ContractMajorVersion, &node.ContractRevision, &node.CredentialHash, &node.Status, &node.RegisteredAt, &node.LastHeartbeatAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.AgentNode{}, false, nil
	}
	if err != nil {
		return model.AgentNode{}, false, err
	}
	return node, true, nil
}

func CheckLocalTrust(userID identitymodel.UserID, nodeID string, at time.Time, freshness time.Duration) (bool, error) {
	return checkLocalTrust(database.DB(), userID, nodeID, at, freshness)
}

func checkLocalTrust(db *gorm.DB, userID identitymodel.UserID, nodeID string, at time.Time, freshness time.Duration) (bool, error) {
	var trusted bool
	err := db.Raw(`SELECT EXISTS(
		SELECT 1 FROM local_agent_nodes n
		JOIN users u ON u.id = n.user_id
		JOIN user_sessions s ON s.id = n.session_id
		WHERE n.id = ? AND n.user_id = ? AND n.mode = 'local' AND n.status = 'online'
		  AND n.last_heartbeat_at >= ? AND s.invalidated_at IS NULL AND u.status = 'enabled'
		  AND u.bit_main_user_id IS NOT NULL AND n.bitbrowser_status = 'normal'
		  AND n.reported_main_user_id = u.bit_main_user_id
	)`, nodeID, userID, at.Add(-freshness)).Row().Scan(&trusted)
	return trusted, err
}

func ValidateRuntimeProfiles(userID identitymodel.UserID, mainUserID string, profileIDs []string) (bool, error) {
	return validateRuntimeProfiles(database.DB(), userID, mainUserID, profileIDs)
}

func validateRuntimeProfiles(db *gorm.DB, userID identitymodel.UserID, mainUserID string, profileIDs []string) (bool, error) {
	var boundMain sql.NullString
	err := db.Raw(`SELECT bit_main_user_id FROM users WHERE id = ? AND status = 'enabled'`, userID).Row().Scan(&boundMain)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !boundMain.Valid || boundMain.String != mainUserID {
		return false, nil
	}
	if len(profileIDs) == 0 {
		return true, nil
	}
	args := make([]any, 0, 2+len(profileIDs))
	args = append(args, userID, mainUserID)
	for _, profileID := range profileIDs {
		args = append(args, profileID)
	}
	var count int
	err = db.Raw(`SELECT COUNT(*) FROM browser_profiles
		WHERE user_id = ? AND main_user_id = ? AND local_status = 'active'
		AND bit_profile_id IN (`+placeholders(len(profileIDs))+`)`, args...).Row().Scan(&count)
	return count == len(profileIDs), err
}

func ApplyRuntimeReport(node model.AgentNode, report dto.RuntimeReport, at time.Time) error {
	return applyRuntimeReport(database.DB(), node, report, at)
}

func applyRuntimeReport(db *gorm.DB, node model.AgentNode, report dto.RuntimeReport, at time.Time) error {
	return db.Transaction(func(tx *gorm.DB) error {
		result := tx.Exec(`UPDATE local_agent_nodes SET status = ?, last_heartbeat_at = ?, operating_system = ?,
			cpu_architecture = ?, agent_version = ?, python_version = ?, ffmpeg_status = ?, ffmpeg_version = ?,
			workdir_status = ?, disk_status = ?, disk_free_megabytes = ?, bitbrowser_status = ?,
			reported_main_user_id = ?, updated_at = ? WHERE id = ?`,
			node.Status, at, report.OperatingSystem, report.CPUArchitecture, report.AgentVersion, report.PythonVersion,
			report.FFmpeg.Status, nullIfEmpty(report.FFmpeg.Version), report.WorkdirStatus, report.Disk.Status,
			report.Disk.FreeMegabytes, report.BitBrowserStatus, nullIfEmpty(report.MainUserID), at, node.ID,
		)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("runtime node was not updated")
		}
		for _, bitProfileID := range report.BitProfileIDs {
			if err := tx.Exec(`INSERT INTO browser_profile_runtime_presence
				(id, profile_id, bit_profile_id, node_id, user_id, main_user_id, status, last_seen_at, created_at, updated_at)
				SELECT ?, bp.id, bp.bit_profile_id, ?, ?, ?, ?, ?, ?, ? FROM browser_profiles bp
				WHERE bp.bit_profile_id = ? AND bp.user_id = ? AND bp.main_user_id = ? AND bp.local_status = 'active'
				ON DUPLICATE KEY UPDATE node_id = VALUES(node_id), user_id = VALUES(user_id), main_user_id = VALUES(main_user_id),
				status = VALUES(status), last_seen_at = VALUES(last_seen_at), updated_at = VALUES(updated_at)`,
				id.NewID("profile-runtime"), node.ID, node.UserID, report.MainUserID, "visible", at, at, at,
				bitProfileID, node.UserID, report.MainUserID,
			).Error; err != nil {
				return err
			}
		}
		query := `UPDATE browser_profile_runtime_presence SET status = 'not_visible', updated_at = ?, last_seen_at = ? WHERE user_id = ?`
		args := []any{at, at, node.UserID}
		if len(report.BitProfileIDs) > 0 {
			query += ` AND bit_profile_id NOT IN (` + placeholders(len(report.BitProfileIDs)) + `)`
			for _, bitProfileID := range report.BitProfileIDs {
				args = append(args, bitProfileID)
			}
		}
		return tx.Exec(query, args...).Error
	})
}

func placeholders(count int) string { return strings.TrimRight(strings.Repeat("?, ", count), ", ") }

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
