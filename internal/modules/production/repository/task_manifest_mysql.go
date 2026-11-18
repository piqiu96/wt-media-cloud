package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"gorm.io/gorm"
)

func (ComposeTaskStore) FreezeComposeManifest(taskID, nodeID int64, attempt int, raw json.RawMessage) (json.RawMessage, error) {
	return freezeComposeManifest(database.DB(), taskID, nodeID, attempt, raw)
}
func freezeComposeManifest(db *gorm.DB, taskID, nodeID int64, attempt int, raw json.RawMessage) (json.RawMessage, error) {
	if !json.Valid(raw) {
		return nil, errors.New("invalid manifest JSON")
	}
	var result []byte
	err := db.Transaction(func(tx *gorm.DB) error {
		var current []byte
		row := tx.Raw(`SELECT execution_manifest_json FROM compose_tasks WHERE id = ? AND claimed_node_id = ? AND attempt_count = ? AND status = 'running' AND cancel_requested = 0 AND lease_expires_at > NOW(3) FOR UPDATE`, taskID, nodeID, attempt).Row()
		if err := row.Scan(&current); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrComposeCommitLost
			}
			return err
		}
		if len(current) > 0 {
			result = current
			return nil
		}
		update := tx.Exec(`UPDATE compose_tasks SET execution_manifest_json = ?, updated_at = NOW(3) WHERE id = ? AND claimed_node_id = ? AND attempt_count = ? AND status = 'running' AND cancel_requested = 0 AND lease_expires_at > NOW(3) AND execution_manifest_json IS NULL`, string(raw), taskID, nodeID, attempt)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrComposeCommitLost
		}
		result = append([]byte(nil), raw...)
		return nil
	})
	return result, err
}
