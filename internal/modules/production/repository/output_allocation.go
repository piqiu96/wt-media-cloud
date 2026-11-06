package repository

import (
	"database/sql"
	"errors"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"gorm.io/gorm"
	"time"
)

// An allocation reserves only a filename identity. It is never returned by the
// output list and cannot make a task successful before verified bytes exist.
func (ComposeTaskStore) ReserveComposeOutputID(taskID, nodeID int64, attempt int) (int64, error) {
	return reserveComposeOutputID(database.DB(), taskID, nodeID, attempt, time.Now())
}

func reserveComposeOutputID(db *gorm.DB, taskID, nodeID int64, attempt int, now time.Time) (int64, error) {
	var outputID int64
	err := db.Transaction(func(tx *gorm.DB) error {
		var heldID int64
		err := tx.Raw(`SELECT id FROM compose_tasks WHERE id=? AND claimed_node_id=? AND attempt_count=? AND status='running' AND cancel_requested=0 AND lease_expires_at>? FOR UPDATE`, taskID, nodeID, attempt, now).Row().Scan(&heldID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrComposeCommitLost
		}
		if err != nil {
			return err
		}
		if err := tx.Exec(`INSERT INTO compose_output_allocations (task_id,attempt,created_at) VALUES (?,?,?) ON DUPLICATE KEY UPDATE id=id`, taskID, attempt, now).Error; err != nil {
			return err
		}
		return tx.Raw(`SELECT id FROM compose_output_allocations WHERE task_id=? AND attempt=?`, taskID, attempt).Row().Scan(&outputID)
	})
	return outputID, err
}
