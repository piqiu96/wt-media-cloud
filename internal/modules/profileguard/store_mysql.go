package profileguard

import (
	"database/sql"
	"errors"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding"
)

type MySQLStore struct{ db *sql.DB }

func NewMySQLStore(db *sql.DB) *MySQLStore { return &MySQLStore{db: db} }

// CreateAuthorizedTask is an internal integration point for future business
// modules. C5 deliberately exposes no generic public task-creation route.
func (s *MySQLStore) CreateAuthorizedTask(task SensitiveTask) error {
	_, err := s.db.Exec(
		`INSERT INTO sensitive_browser_tasks (id, user_id, profile_id, bit_profile_id, node_id, operation, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.UserID, task.ProfileID, task.BitProfileID, task.NodeID, task.Operation, task.Status, task.CreatedAt, task.UpdatedAt,
	)
	return err
}

func (s *MySQLStore) FindAuthorizedTask(taskID string) (SensitiveTask, bool, error) {
	var task SensitiveTask
	err := s.db.QueryRow(
		`SELECT id, user_id, profile_id, bit_profile_id, node_id, operation, status, created_at, updated_at FROM sensitive_browser_tasks WHERE id = ?`, taskID,
	).Scan(&task.ID, &task.UserID, &task.ProfileID, &task.BitProfileID, &task.NodeID, &task.Operation, &task.Status, &task.CreatedAt, &task.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return SensitiveTask{}, false, nil
	}
	if err != nil {
		return SensitiveTask{}, false, err
	}
	return task, true, nil
}

func (s *MySQLStore) AcquirePermit(task SensitiveTask, node runtimebinding.AgentNode, permit Permit, at time.Time, freshness time.Duration) (PreflightOutcome, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return PreflightOutcome{}, err
	}
	defer tx.Rollback()
	var profileID string
	if err := tx.QueryRow(`SELECT id FROM browser_profiles WHERE id = ? FOR UPDATE`, task.ProfileID).Scan(&profileID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PreflightOutcome{}, ErrRuntimeUnavailable
		}
		return PreflightOutcome{}, err
	}
	var validCount int
	err = tx.QueryRow(
		`SELECT COUNT(*) FROM sensitive_browser_tasks t JOIN browser_profiles bp ON bp.id = t.profile_id JOIN users u ON u.id = t.user_id JOIN local_agent_nodes n ON n.id = t.node_id JOIN user_sessions s ON s.id = n.session_id JOIN browser_profile_runtime_presence rp ON rp.profile_id = bp.id AND rp.node_id = n.id WHERE t.id = ? AND t.user_id = ? AND t.profile_id = ? AND t.node_id = ? AND t.operation = ? AND t.bit_profile_id = ? AND t.status = 'authorized' AND bp.local_status = 'active' AND bp.bit_profile_id = t.bit_profile_id AND bp.owner_user_id = u.bit_owner_user_id AND u.status = 'enabled' AND s.invalidated_at IS NULL AND n.status = 'online' AND n.bitbrowser_status = 'normal' AND n.reported_owner_user_id = u.bit_owner_user_id AND rp.status = 'visible' AND rp.owner_user_id = u.bit_owner_user_id AND rp.last_seen_at >= ?`,
		task.ID, task.UserID, task.ProfileID, node.ID, task.Operation, task.BitProfileID, at.Add(-freshness),
	).Scan(&validCount)
	if err != nil {
		return PreflightOutcome{}, err
	}
	if validCount != 1 {
		return PreflightOutcome{}, ErrRuntimeUnavailable
	}

	var priorID string
	var priorStatus PermitStatus
	var priorExpires time.Time
	err = tx.QueryRow(`SELECT id, status, expires_at FROM sensitive_profile_permits WHERE profile_id = ? ORDER BY acquired_at DESC LIMIT 1`, task.ProfileID).
		Scan(&priorID, &priorStatus, &priorExpires)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return PreflightOutcome{}, err
	}
	if err == nil {
		if priorStatus == PermitReviewRequired {
			if err := tx.Commit(); err != nil {
				return PreflightOutcome{}, err
			}
			return PreflightOutcome{Outcome: OutcomeReviewRequired, ProfileID: task.ProfileID}, nil
		}
		if priorStatus == PermitActive && priorExpires.After(at) {
			if err := tx.Commit(); err != nil {
				return PreflightOutcome{}, err
			}
			return PreflightOutcome{Outcome: OutcomeWaiting, PermitID: priorID, ProfileID: task.ProfileID, ExpiresAt: &priorExpires}, nil
		}
		if priorStatus == PermitActive {
			if _, err := tx.Exec(`UPDATE sensitive_profile_permits SET status = ?, finished_at = ? WHERE id = ? AND status = ?`, PermitReviewRequired, at, priorID, PermitActive); err != nil {
				return PreflightOutcome{}, err
			}
			if _, err := tx.Exec(`UPDATE sensitive_browser_tasks SET status = ?, updated_at = ? WHERE id = (SELECT task_id FROM sensitive_profile_permits WHERE id = ?)`, TaskReviewRequired, at, priorID); err != nil {
				return PreflightOutcome{}, err
			}
			if err := tx.Commit(); err != nil {
				return PreflightOutcome{}, err
			}
			return PreflightOutcome{Outcome: OutcomeReviewRequired, ProfileID: task.ProfileID}, nil
		}
	}
	if _, err := tx.Exec(
		`INSERT INTO sensitive_profile_permits (id, task_id, user_id, profile_id, node_id, operation, status, credential_hash, acquired_at, expires_at, finished_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		permit.ID, permit.TaskID, permit.UserID, permit.ProfileID, permit.NodeID, permit.Operation, permit.Status, permit.CredentialHash, permit.AcquiredAt, permit.ExpiresAt, permit.FinishedAt,
	); err != nil {
		return PreflightOutcome{}, err
	}
	result, err := tx.Exec(`UPDATE sensitive_browser_tasks SET status = ?, updated_at = ? WHERE id = ? AND status = ?`, TaskRunning, at, task.ID, TaskAuthorized)
	if err != nil {
		return PreflightOutcome{}, err
	}
	if rows, err := result.RowsAffected(); err != nil || rows != 1 {
		if err != nil {
			return PreflightOutcome{}, err
		}
		return PreflightOutcome{}, ErrTaskAssignmentMismatch
	}
	if err := tx.Commit(); err != nil {
		return PreflightOutcome{}, err
	}
	return PreflightOutcome{Outcome: OutcomeGranted, PermitID: permit.ID, ProfileID: task.ProfileID, ExpiresAt: &permit.ExpiresAt}, nil
}

func (s *MySQLStore) RenewPermit(permitID, nodeID, credentialHash string, at, expiresAt time.Time) (time.Time, error) {
	result, err := s.db.Exec(
		`UPDATE sensitive_profile_permits SET expires_at = ? WHERE id = ? AND node_id = ? AND credential_hash = ? AND status = ? AND expires_at > ?`,
		expiresAt, permitID, nodeID, credentialHash, PermitActive, at,
	)
	if err != nil {
		return time.Time{}, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return time.Time{}, err
	}
	if rows != 1 {
		return time.Time{}, ErrPermitCredentialInvalid
	}
	return expiresAt, nil
}

func (s *MySQLStore) FinishPermit(permitID, nodeID, credentialHash string, outcome FinishOutcome, at time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var taskID string
	var status PermitStatus
	err = tx.QueryRow(
		`SELECT task_id, status FROM sensitive_profile_permits WHERE id = ? AND node_id = ? AND credential_hash = ? FOR UPDATE`, permitID, nodeID, credentialHash,
	).Scan(&taskID, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrPermitCredentialInvalid
	}
	if err != nil {
		return err
	}
	if status != PermitActive {
		return ErrPermitCredentialInvalid
	}
	permitStatus, taskStatus := PermitReleased, TaskCompleted
	if outcome == FinishResultUncertain {
		permitStatus, taskStatus = PermitReviewRequired, TaskReviewRequired
	}
	if _, err := tx.Exec(`UPDATE sensitive_profile_permits SET status = ?, finished_at = ? WHERE id = ? AND status = ?`, permitStatus, at, permitID, PermitActive); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE sensitive_browser_tasks SET status = ?, updated_at = ? WHERE id = ?`, taskStatus, at, taskID); err != nil {
		return err
	}
	return tx.Commit()
}

var _ Store = (*MySQLStore)(nil)
