package repository

import (
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"gorm.io/gorm"
	"strconv"
)

func OutputFileReferenced(db *gorm.DB, id int64) (bool, error) {
	var count int64
	err := db.Model(&model.CompositeOutput{}).Where("JSON_UNQUOTE(JSON_EXTRACT(media_json,'$.thumbnail_file_id'))=?", strconv.FormatInt(id, 10)).Count(&count).Error
	return count > 0, err
}

func OutputFileReferenceIDs(db *gorm.DB, filter ComposeOutputFilter, id int64) ([]int64, error) {
	out := []int64{}
	err := outputQuery(db, filter).Where("JSON_UNQUOTE(JSON_EXTRACT(o.media_json,'$.thumbnail_file_id'))=?", strconv.FormatInt(id, 10)).Pluck("o.id", &out).Error
	return out, err
}
