package repository

import (
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"gorm.io/gorm"
)

func taskSnapshot(tx *gorm.DB, id uint64) (model.Task, error) {
	var v model.Task
	e := tx.First(&v, id).Error
	return v, missing(e)
}

// Updates use the revision read by this transaction. A competing writer wins
// once; the loser rolls back its task, attempt and audit changes together.
func updateTask(tx *gorm.DB, v model.Task, updates map[string]any) error {
	updates["revision"] = v.Revision + 1
	result := tx.Model(&model.Task{}).Where("id=? AND revision=?", v.ID, v.Revision).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrConflict
	}
	return nil
}
func updateAttempt(tx *gorm.DB, a model.Attempt, updates map[string]any) error {
	updates["revision"] = a.Revision + 1
	result := tx.Model(&model.Attempt{}).Where("id=? AND revision=?", a.ID, a.Revision).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrConflict
	}
	return nil
}
