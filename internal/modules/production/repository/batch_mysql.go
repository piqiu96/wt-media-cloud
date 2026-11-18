package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"gorm.io/gorm"
	"time"
)

func findComposeTaskBatch(db *gorm.DB, id int64) (model.ComposeTaskBatch, bool, error) {
	var b model.ComposeTaskBatch
	var operator sql.NullInt64
	var snapshot []byte
	err := db.Raw(`SELECT id,name,team_id,category_id,operator_id,compose_strategy_id,trigger_type,created_by,request_key,request_hash,config_snapshot_json,created_at FROM compose_task_batches WHERE id=?`, id).Row().Scan(&b.ID, &b.Name, &b.TeamID, &b.CategoryID, &operator, &b.StrategyID, &b.TriggerType, &b.CreatedBy, &b.RequestKey, &b.RequestHash, &snapshot, &b.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return b, false, nil
	}
	if err != nil {
		return b, false, err
	}
	if operator.Valid {
		b.OperatorID = &operator.Int64
	}
	b.ConfigSnapshot = append(json.RawMessage(nil), snapshot...)
	rows, err := db.Raw(`SELECT status,COUNT(*) FROM compose_tasks WHERE batch_id=? GROUP BY status`, id).Rows()
	if err != nil {
		return b, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return b, false, err
		}
		switch status {
		case "pending":
			b.Counts.Pending = n
		case "running":
			b.Counts.Running = n
		case "success":
			b.Counts.Success = n
		case "failed":
			b.Counts.Failed = n
		case "cancelled":
			b.Counts.Cancelled = n
		}
	}
	return b, true, rows.Err()
}
func (ComposeTaskStore) FindBatch(id int64) (model.ComposeTaskBatch, bool, error) {
	return findComposeTaskBatch(database.DB(), id)
}
func insertComposeTaskBatch(tx *gorm.DB, b *model.ComposeTaskBatch, now time.Time) error {
	b.CreatedAt = now
	return tx.Create(b).Error
}
