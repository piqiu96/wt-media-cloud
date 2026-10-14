package repository

import (
	"database/sql"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"gorm.io/gorm"
)

func (ComposeTaskStore) ResolveUncertain(taskID, actorID int64, evidence string) (model.ComposeUncertainReview, bool, error) {
	return resolveComposeUncertain(database.DB(), taskID, actorID, evidence, time.Now())
}

func resolveComposeUncertain(db *gorm.DB, taskID, actorID int64, evidence string, now time.Time) (model.ComposeUncertainReview, bool, error) {
	var review model.ComposeUncertainReview
	changed := false
	err := db.Transaction(func(tx *gorm.DB) error {
		// The status guard serializes competing administrators. The output guards
		// prevent requeueing an uncertain task whose success was already saved.
		result := tx.Exec(`UPDATE compose_tasks SET status = 'pending', claimed_node_id = NULL, lease_expires_at = NULL, heartbeat_at = NULL, next_attempt_at = NULL, cancel_requested = 0, stage = '', progress = 0, started_at = NULL, finished_at = NULL, max_attempts = max_attempts + 1, updated_at = ? WHERE id = ? AND status = 'failed' AND error_code = 'execution_uncertain' AND attempt_count > 0 AND composite_output_id IS NULL AND output_object_key = '' AND NOT EXISTS (SELECT 1 FROM composite_outputs WHERE compose_task_id = ?)`, now, taskID, taskID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nil
		}
		item, found, err := findComposeTask(tx, taskID)
		if err != nil || !found {
			return err
		}
		if err := tx.Exec(`INSERT INTO compose_uncertain_reviews (task_id, attempt, actor_user_id, evidence, created_at) VALUES (?, ?, ?, ?, ?)`, taskID, item.AttemptCount, actorID, evidence, now).Error; err != nil {
			return err
		}
		if err := tx.Raw(`SELECT id, task_id, attempt, actor_user_id, evidence, created_at FROM compose_uncertain_reviews WHERE task_id = ? AND attempt = ?`, taskID, item.AttemptCount).Row().Scan(&review.ID, &review.TaskID, &review.Attempt, &review.ActorUserID, &review.Evidence, &review.CreatedAt); err != nil {
			return err
		}
		if err := tx.Exec(`INSERT INTO compose_task_attempts (task_id, attempt, source, outcome, created_at, updated_at) VALUES (?, ?, 'manual', 'pending', ?, ?)`, taskID, item.AttemptCount+1, now, now).Error; err != nil {
			return err
		}
		if err := appendComposeTaskAudit(tx, item, "compose_task.resolve_uncertain", actorID, now); err != nil {
			return err
		}
		changed = true
		return nil
	})
	if err != nil {
		return model.ComposeUncertainReview{}, false, err
	}
	return review, changed, nil
}

func (ComposeTaskStore) ListUncertainReviews(taskID int64) ([]model.ComposeUncertainReview, error) {
	return listComposeUncertainReviews(database.DB(), taskID)
}

func listComposeUncertainReviews(db *gorm.DB, taskID int64) ([]model.ComposeUncertainReview, error) {
	rows, err := db.Raw(`SELECT id, task_id, attempt, actor_user_id, evidence, created_at FROM compose_uncertain_reviews WHERE task_id = ? ORDER BY attempt`, taskID).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]model.ComposeUncertainReview, 0)
	for rows.Next() {
		var item model.ComposeUncertainReview
		if err := rows.Scan(&item.ID, &item.TaskID, &item.Attempt, &item.ActorUserID, &item.Evidence, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	return items, nil
}
