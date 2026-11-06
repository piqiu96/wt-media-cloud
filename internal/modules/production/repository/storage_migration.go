package repository

import (
	"errors"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"gorm.io/gorm"
)

type StoredAsset struct {
	Kind     string
	ID       int64
	Location string
	Size     int64
	SHA256   string
}

func StoredAssets() ([]StoredAsset, error) {
	var items []StoredAsset
	err := database.DB().Raw(`SELECT 'material' AS kind,id,source_object_key AS location,COALESCE(video_size_bytes,0) AS size,COALESCE(video_sha256,'') AS sha256 FROM materials WHERE COALESCE(source_object_key,'')<>'' UNION ALL SELECT 'composite_output' AS kind,id,object_key AS location,file_size_bytes AS size,sha256 FROM composite_outputs WHERE object_key<>'' ORDER BY kind,id`).Scan(&items).Error
	return items, err
}

// SwitchStoredAsset compares the identity, old location and recorded digest.
// Mutable configuration snapshots and source metadata are deliberately untouched.
func SwitchStoredAsset(item StoredAsset, destination string) (bool, error) {
	changed := false
	err := database.DB().Transaction(func(tx *gorm.DB) error {
		var result *gorm.DB
		switch item.Kind {
		case "material":
			result = tx.Exec(`UPDATE materials SET source_object_key=? WHERE id=? AND source_object_key=? AND COALESCE(video_sha256,'')=?`, destination, item.ID, item.Location, item.SHA256)
		case "composite_output":
			result = tx.Exec(`UPDATE composite_outputs SET object_key=? WHERE id=? AND object_key=? AND sha256=?`, destination, item.ID, item.Location, item.SHA256)
		default:
			return errors.New("unsupported stored asset kind")
		}
		if result.Error != nil {
			return result.Error
		}
		changed = result.RowsAffected == 1
		if changed && item.Kind == "composite_output" {
			return tx.Exec(`UPDATE compose_tasks SET output_object_key=? WHERE composite_output_id=? AND output_object_key=?`, destination, item.ID, item.Location).Error
		}
		return nil
	})
	return changed, err
}
