package repository

import (
	"database/sql"
	"errors"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/profileguard/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/identity"
	"gorm.io/gorm"
)

func CreateAuthorizedTask(task model.SensitiveTask) error {
	return createAuthorizedTask(database.DB(), task)
}

func createAuthorizedTask(db *gorm.DB, task model.SensitiveTask) error {
	return db.Exec(`INSERT INTO sensitive_browser_tasks
		(id, user_id, profile_id, bit_profile_id, operation, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.UserID, task.ProfileID, task.BitProfileID, task.Operation, task.Status, task.CreatedAt, task.UpdatedAt,
	).Error
}

func FindAuthorizedTask(taskID string) (model.SensitiveTask, bool, error) {
	return findAuthorizedTask(database.DB(), taskID)
}

func findAuthorizedTask(db *gorm.DB, taskID string) (model.SensitiveTask, bool, error) {
	var task model.SensitiveTask
	err := db.Raw(`SELECT id, user_id, profile_id, bit_profile_id, operation, status, created_at, updated_at
		FROM sensitive_browser_tasks WHERE id = ?`, taskID).Row().
		Scan(&task.ID, &task.UserID, &task.ProfileID, &task.BitProfileID, &task.Operation, &task.Status, &task.CreatedAt, &task.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.SensitiveTask{}, false, nil
	}
	if err != nil {
		return model.SensitiveTask{}, false, err
	}
	return task, true, nil
}

func AcquirePermit(task model.SensitiveTask, permit model.Permit, at time.Time) (PreflightOutcome, error) {
	return acquirePermit(database.DB(), task, permit, at)
}

func acquirePermit(db *gorm.DB, task model.SensitiveTask, permit model.Permit, at time.Time) (PreflightOutcome, error) {
	outcome := PreflightOutcome{}
	err := db.Transaction(func(tx *gorm.DB) error {
		var profileID string
		if err := tx.Raw(`SELECT id FROM browser_profiles WHERE id = ? FOR UPDATE`, task.ProfileID).Row().Scan(&profileID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return model.ErrRuntimeUnavailable
			}
			return err
		}
		var validCount int
		if err := tx.Raw(`SELECT COUNT(*) FROM sensitive_browser_tasks t
			JOIN browser_profiles bp ON bp.id = t.profile_id
			JOIN users u ON u.id = t.user_id
			WHERE t.id = ? AND t.user_id = ? AND t.profile_id = ? AND t.operation = ?
			AND t.bit_profile_id = ? AND t.status = 'authorized' AND bp.local_status = 'active'
			AND bp.user_id = u.id
			AND bp.bit_profile_id = t.bit_profile_id AND bp.main_user_id = u.bit_main_user_id
			AND u.status = 'enabled'`,
			task.ID, task.UserID, task.ProfileID, task.Operation, task.BitProfileID,
		).Row().Scan(&validCount); err != nil {
			return err
		}
		if validCount != 1 {
			return model.ErrRuntimeUnavailable
		}

		var priorID string
		var priorStatus model.PermitStatus
		var priorExpires time.Time
		err := tx.Raw(`SELECT id, status, expires_at FROM sensitive_profile_permits
			WHERE profile_id = ? ORDER BY acquired_at DESC LIMIT 1`, task.ProfileID).Row().Scan(&priorID, &priorStatus, &priorExpires)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err == nil {
			if priorStatus == model.PermitReviewRequired {
				outcome = PreflightOutcome{Outcome: model.OutcomeReviewRequired, ProfileID: task.ProfileID}
				return nil
			}
			if priorStatus == model.PermitActive && priorExpires.After(at) {
				outcome = PreflightOutcome{Outcome: model.OutcomeWaiting, PermitID: priorID, ProfileID: task.ProfileID, ExpiresAt: &priorExpires}
				return nil
			}
			if priorStatus == model.PermitActive {
				if err := tx.Exec(`UPDATE sensitive_profile_permits SET status = ?, finished_at = ? WHERE id = ? AND status = ?`,
					model.PermitReviewRequired, at, priorID, model.PermitActive).Error; err != nil {
					return err
				}
				if err := tx.Exec(`UPDATE sensitive_browser_tasks SET status = ?, updated_at = ?
					WHERE id = (SELECT task_id FROM sensitive_profile_permits WHERE id = ?)`,
					model.TaskReviewRequired, at, priorID).Error; err != nil {
					return err
				}
				outcome = PreflightOutcome{Outcome: model.OutcomeReviewRequired, ProfileID: task.ProfileID}
				return nil
			}
		}
		if err := tx.Exec(`INSERT INTO sensitive_profile_permits
			(id, task_id, user_id, profile_id, operation, status, acquired_at, expires_at, finished_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			permit.ID, permit.TaskID, permit.UserID, permit.ProfileID, permit.Operation, permit.Status,
			permit.AcquiredAt, permit.ExpiresAt, permit.FinishedAt,
		).Error; err != nil {
			return err
		}
		result := tx.Exec(`UPDATE sensitive_browser_tasks SET status = ?, updated_at = ? WHERE id = ? AND status = ?`,
			model.TaskRunning, at, task.ID, model.TaskAuthorized)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return model.ErrTaskAssignmentMismatch
		}
		outcome = PreflightOutcome{Outcome: model.OutcomeGranted, PermitID: permit.ID, ProfileID: task.ProfileID, ExpiresAt: &permit.ExpiresAt}
		return nil
	})
	if err != nil {
		return PreflightOutcome{}, err
	}
	return outcome, nil
}

func RenewPermit(permitID string, userID identity.UserID, at, expiresAt time.Time) (time.Time, error) {
	return renewPermit(database.DB(), permitID, userID, at, expiresAt)
}

func renewPermit(db *gorm.DB, permitID string, userID identity.UserID, at, expiresAt time.Time) (time.Time, error) {
	result := db.Exec(`UPDATE sensitive_profile_permits SET expires_at = ?
		WHERE id = ? AND user_id = ? AND status = ? AND expires_at > ?`,
		expiresAt, permitID, userID, model.PermitActive, at)
	if result.Error != nil {
		return time.Time{}, result.Error
	}
	if result.RowsAffected != 1 {
		return time.Time{}, model.ErrPermitInvalid
	}
	return expiresAt, nil
}

func FinishPermit(permitID string, userID identity.UserID, outcome model.FinishOutcome, at time.Time) error {
	return finishPermit(database.DB(), permitID, userID, outcome, at)
}

func finishPermit(db *gorm.DB, permitID string, userID identity.UserID, outcome model.FinishOutcome, at time.Time) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var taskID string
		var status model.PermitStatus
		err := tx.Raw(`SELECT task_id, status FROM sensitive_profile_permits
			WHERE id = ? AND user_id = ? FOR UPDATE`, permitID, userID).
			Row().Scan(&taskID, &status)
		if errors.Is(err, sql.ErrNoRows) {
			return model.ErrPermitInvalid
		}
		if err != nil {
			return err
		}
		if status != model.PermitActive {
			return model.ErrPermitInvalid
		}
		permitStatus, taskStatus := model.PermitReleased, model.TaskCompleted
		if outcome == model.FinishResultUncertain {
			permitStatus, taskStatus = model.PermitReviewRequired, model.TaskReviewRequired
		}
		if err := tx.Exec(`UPDATE sensitive_profile_permits SET status = ?, finished_at = ? WHERE id = ? AND status = ?`,
			permitStatus, at, permitID, model.PermitActive).Error; err != nil {
			return err
		}
		return tx.Exec(`UPDATE sensitive_browser_tasks SET status = ?, updated_at = ? WHERE id = ?`, taskStatus, at, taskID).Error
	})
}
