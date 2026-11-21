package repository

import (
	"github.com/wt-media/wt-media-cloud/internal/modules/image/model"
	"gorm.io/gorm"
	"strconv"
)

func artworkReferenceQuery(db *gorm.DB, fileID int64) *gorm.DB {
	return db.Table("operations_files").Where("uploaded_at IS NOT NULL AND deleted_at IS NULL AND JSON_UNQUOTE(JSON_EXTRACT(metadata_json,'$.render_input.background_file_id'))=?", strconv.FormatInt(fileID, 10))
}
func ArtworkFileReferenced(db *gorm.DB, id int64) (bool, error) {
	var n int64
	err := artworkReferenceQuery(db, id).Count(&n).Error
	return n > 0, err
}
func ListArtworkFileReference(db *gorm.DB, sc model.Scope, id int64) ([]model.Reference, error) {
	q := artworkReferenceQuery(db, id)
	if !sc.All {
		if sc.OwnerID > 0 {
			q = q.Where("owner_id=?", sc.OwnerID)
		} else if sc.TeamID > 0 {
			q = q.Where("team_id=?", sc.TeamID)
		} else {
			q = q.Where("1=0")
		}
	}
	var ids []int64
	if err := q.Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	out := make([]model.Reference, 0, len(ids))
	for _, id := range ids {
		out = append(out, model.Reference{Kind: "cover", ID: strconv.FormatInt(id, 10)})
	}
	return out, nil
}
