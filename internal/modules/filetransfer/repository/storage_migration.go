package repository

import "github.com/wt-media/wt-media-cloud/internal/infra/database"

type StoredReference struct {
	ID        string
	AssetType string
	AssetID   int64
	Location  string
	Size      int64
	SHA256    string
}

func StoredReferences() ([]StoredReference, error) {
	var items []StoredReference
	err := database.DB().Raw(`SELECT id,asset_type,asset_id,source_object_key AS location,total_bytes AS size,COALESCE(expected_sha256,integrity_sha256,'') AS sha256 FROM file_transfer_tasks WHERE COALESCE(source_object_key,'')<>'' ORDER BY created_at,id`).Scan(&items).Error
	return items, err
}

func SwitchStoredReference(item StoredReference, destination string) (bool, error) {
	result := database.DB().Exec(`UPDATE file_transfer_tasks SET source_object_key=? WHERE id=? AND source_object_key=? AND COALESCE(expected_sha256,integrity_sha256,'')=?`, destination, item.ID, item.Location, item.SHA256)
	return result.RowsAffected == 1, result.Error
}
