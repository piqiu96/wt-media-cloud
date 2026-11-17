package repository

import (
	"fmt"
	"gorm.io/gorm"
)

// OperationsFileReferenced checks the two existing production snapshots that
// may contain operations-file capabilities. It never rewrites historical JSON.
func OperationsFileReferenced(tx *gorm.DB, token string) (bool, error) {
	pattern := "%/api/v1/file-resources/" + token + "%"
	for _, source := range []struct{ table, column string }{{"compose_strategies", "config_json"}, {"compose_strategies", "output_settings_json"}, {"compose_tasks", "config_snapshot_json"}, {"compose_tasks", "execution_manifest_json"}, {"compose_tasks", "technical_config_json"}, {"compose_task_batches", "config_snapshot_json"}} {
		var count int64
		if err := tx.Table(source.table).Where(fmt.Sprintf("JSON_SEARCH(%s,'one',?) IS NOT NULL", source.column), pattern).Count(&count).Error; err != nil {
			return false, err
		}
		if count > 0 {
			return true, nil
		}
	}
	return false, nil
}
