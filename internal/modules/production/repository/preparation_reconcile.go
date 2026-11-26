package repository

import (
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"gorm.io/gorm"
)

// ReconcileCancelledMaterialPreparations repairs the projection when a pending
// preparation was cancelled or its executor never returned to settle it.
func ReconcileCancelledMaterialPreparations(now time.Time) (int64, error) {
	return reconcileCancelledMaterialPreparations(database.DB(), now)
}

func reconcileCancelledMaterialPreparations(db *gorm.DB, now time.Time) (int64, error) {
	result := db.Exec(`UPDATE materials AS m
SET video_status = 'not_downloaded', video_error = '', updated_at = ?
WHERE m.video_status = 'downloading' AND EXISTS (
 SELECT 1 FROM file_transfer_tasks AS t
 WHERE t.team_id = m.team_id AND t.asset_id = m.id
   AND t.asset_type = 'material' AND t.purpose = 'compose_input_prepare'
 GROUP BY t.team_id, t.asset_id
 HAVING SUM(t.status IN ('pending','running')) = 0
    AND SUM(t.status = 'cancelled') > 0
)`, now)
	return result.RowsAffected, result.Error
}
