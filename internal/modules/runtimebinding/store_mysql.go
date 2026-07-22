package runtimebinding

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/common"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
)

type MySQLStore struct{ db *sql.DB }

func NewMySQLStore(db *sql.DB) *MySQLStore { return &MySQLStore{db: db} }

func (s *MySQLStore) CreateTicket(ticket BindingTicket) error {
	_, err := s.db.Exec(
		`INSERT INTO local_agent_binding_tickets (id, user_id, session_id, token_hash, created_at, expires_at, used_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		ticket.ID, ticket.UserID, ticket.SessionID, ticket.TokenHash, ticket.CreatedAt, ticket.ExpiresAt, ticket.UsedAt,
	)
	return err
}

func (s *MySQLStore) ConsumeTicket(tokenHash string, at time.Time) (BindingTicket, bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return BindingTicket{}, false, err
	}
	defer tx.Rollback()
	var ticket BindingTicket
	var usedAt sql.NullTime
	err = tx.QueryRow(
		`SELECT id, user_id, session_id, token_hash, created_at, expires_at, used_at FROM local_agent_binding_tickets WHERE token_hash = ? FOR UPDATE`, tokenHash,
	).Scan(&ticket.ID, &ticket.UserID, &ticket.SessionID, &ticket.TokenHash, &ticket.CreatedAt, &ticket.ExpiresAt, &usedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return BindingTicket{}, false, nil
	}
	if err != nil {
		return BindingTicket{}, false, err
	}
	if usedAt.Valid || !ticket.ExpiresAt.After(at) {
		return BindingTicket{}, false, nil
	}
	result, err := tx.Exec(`UPDATE local_agent_binding_tickets SET used_at = ? WHERE id = ? AND used_at IS NULL`, at, ticket.ID)
	if err != nil {
		return BindingTicket{}, false, err
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		if err != nil {
			return BindingTicket{}, false, err
		}
		return BindingTicket{}, false, nil
	}
	if err := tx.Commit(); err != nil {
		return BindingTicket{}, false, err
	}
	ticket.UsedAt = &at
	return ticket, true, nil
}

func (s *MySQLStore) IsSessionActive(sessionID string, userID identity.UserID, at time.Time) (bool, error) {
	var active bool
	err := s.db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM user_sessions s JOIN users u ON u.id = s.user_id WHERE s.id = ? AND s.user_id = ? AND s.invalidated_at IS NULL AND u.status = 'enabled')`,
		sessionID, userID,
	).Scan(&active)
	return active, err
}

func (s *MySQLStore) SaveNode(node AgentNode) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(
		`UPDATE local_agent_nodes SET status = 'replaced', updated_at = ? WHERE user_id = ? AND device_id = ? AND status <> 'replaced'`,
		node.RegisteredAt, node.UserID, node.DeviceID,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`INSERT INTO local_agent_nodes (id, agent_id, device_id, user_id, session_id, mode, agent_version, contract_major_version, contract_revision, credential_hash, status, registered_at, last_heartbeat_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		node.ID, node.AgentID, node.DeviceID, node.UserID, node.SessionID, node.Mode, node.AgentVersion,
		node.ContractMajorVersion, node.ContractRevision, node.CredentialHash, node.Status,
		node.RegisteredAt, node.LastHeartbeatAt, node.RegisteredAt,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) FindNodeByCredentialHash(hash string) (AgentNode, bool, error) {
	var node AgentNode
	err := s.db.QueryRow(
		`SELECT id, agent_id, device_id, user_id, session_id, mode, agent_version, contract_major_version, contract_revision, credential_hash, status, registered_at, last_heartbeat_at FROM local_agent_nodes WHERE credential_hash = ?`, hash,
	).Scan(&node.ID, &node.AgentID, &node.DeviceID, &node.UserID, &node.SessionID, &node.Mode, &node.AgentVersion,
		&node.ContractMajorVersion, &node.ContractRevision, &node.CredentialHash, &node.Status, &node.RegisteredAt, &node.LastHeartbeatAt)
	if errors.Is(err, sql.ErrNoRows) {
		return AgentNode{}, false, nil
	}
	if err != nil {
		return AgentNode{}, false, err
	}
	return node, true, nil
}

func (s *MySQLStore) ValidateRuntimeProfiles(userID identity.UserID, mainUserID string, profileIDs []string) (bool, error) {
	if len(profileIDs) == 0 {
		return false, nil
	}
	var boundMain sql.NullString
	err := s.db.QueryRow(`SELECT bit_main_user_id FROM users WHERE id = ? AND status = 'enabled'`, userID).Scan(&boundMain)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !boundMain.Valid || boundMain.String != mainUserID {
		return false, nil
	}
	args := make([]any, 0, 2+len(profileIDs))
	args = append(args, userID, mainUserID)
	for _, id := range profileIDs {
		args = append(args, id)
	}
	var count int
	err = s.db.QueryRow(
		`SELECT COUNT(*) FROM browser_profiles WHERE user_id = ? AND main_user_id = ? AND local_status = 'active' AND bit_profile_id IN (`+placeholders(len(profileIDs))+`)`, args...,
	).Scan(&count)
	return count == len(profileIDs), err
}

func (s *MySQLStore) ApplyRuntimeReport(node AgentNode, report RuntimeReport, at time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(
		`UPDATE local_agent_nodes SET status = ?, last_heartbeat_at = ?, operating_system = ?, cpu_architecture = ?, agent_version = ?, python_version = ?, ffmpeg_status = ?, ffmpeg_version = ?, workdir_status = ?, disk_status = ?, disk_free_megabytes = ?, bitbrowser_status = ?, reported_main_user_id = ?, updated_at = ? WHERE id = ?`,
		node.Status, at, report.OperatingSystem, report.CPUArchitecture, report.AgentVersion, report.PythonVersion,
		report.FFmpeg.Status, nullIfEmpty(report.FFmpeg.Version), report.WorkdirStatus, report.Disk.Status,
		report.Disk.FreeMegabytes, report.BitBrowserStatus, nullIfEmpty(report.MainUserID), at, node.ID,
	)
	if err != nil {
		return err
	}
	if rows, err := result.RowsAffected(); err != nil || rows != 1 {
		if err != nil {
			return err
		}
		return ErrNodeCredentialInvalid
	}
	for _, bitProfileID := range report.BitProfileIDs {
		_, err := tx.Exec(
			`INSERT INTO browser_profile_runtime_presence (id, profile_id, bit_profile_id, node_id, user_id, main_user_id, status, last_seen_at, created_at, updated_at) SELECT ?, bp.id, bp.bit_profile_id, ?, ?, ?, ?, ?, ?, ? FROM browser_profiles bp WHERE bp.bit_profile_id = ? AND bp.user_id = ? AND bp.main_user_id = ? AND bp.local_status = 'active' ON DUPLICATE KEY UPDATE node_id = VALUES(node_id), user_id = VALUES(user_id), main_user_id = VALUES(main_user_id), status = VALUES(status), last_seen_at = VALUES(last_seen_at), updated_at = VALUES(updated_at)`,
			common.NewID("profile-runtime"), node.ID, node.UserID, report.MainUserID, "visible", at, at, at,
			bitProfileID, node.UserID, report.MainUserID,
		)
		if err != nil {
			return err
		}
	}
	query := `UPDATE browser_profile_runtime_presence SET status = 'not_visible', updated_at = ?, last_seen_at = ? WHERE user_id = ?`
	args := []any{at, at, node.UserID}
	if len(report.BitProfileIDs) > 0 {
		query += ` AND bit_profile_id NOT IN (` + placeholders(len(report.BitProfileIDs)) + `)`
		for _, id := range report.BitProfileIDs {
			args = append(args, id)
		}
	}
	if _, err := tx.Exec(query, args...); err != nil {
		return err
	}
	return tx.Commit()
}

func placeholders(count int) string { return strings.TrimRight(strings.Repeat("?, ", count), ", ") }
func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

var _ Store = (*MySQLStore)(nil)
