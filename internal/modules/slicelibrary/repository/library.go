package repository

import (
	"context"
	"errors"
	"fmt"
	"github.com/wt-media/wt-media-cloud/internal/modules/slicelibrary/model"
	"gorm.io/gorm"
	"strings"
)

type Repository struct{ DB *gorm.DB }

// A dedicated connection owns this advisory lock throughout naming, upload and
// index publication. No DB transaction is held over network/media work.
func (r Repository) WithLock(ctx context.Context, name string, fn func(Repository) error) error {
	return r.DB.WithContext(ctx).Connection(func(db *gorm.DB) error {
		var acquired int
		if err := db.Raw("SELECT GET_LOCK(?,0)", name).Scan(&acquired).Error; err != nil {
			return err
		}
		if acquired != 1 {
			return errors.New("slice library is being updated; retry later")
		}
		defer db.WithContext(context.WithoutCancel(ctx)).Exec("SELECT RELEASE_LOCK(?)", name)
		return fn(Repository{DB: db})
	})
}
func (r Repository) Reserve(ctx context.Context, source model.Source) (model.Source, error) {
	var existing model.Source
	err := r.DB.WithContext(ctx).Where("source_id=?", source.SourceID).Take(&existing).Error
	if err == nil {
		if existing.SHA256 != source.SHA256 {
			return existing, errors.New("source bytes changed after initial review")
		}
		return existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return source, err
	}
	var count int64
	if err = r.DB.WithContext(ctx).Model(&model.Source{}).Where("basename=?", source.Basename).Count(&count).Error; err != nil {
		return source, err
	}
	if count > 0 {
		source.Basename += "_" + source.SourceID
	}
	if strings.TrimSpace(source.Basename) == "" {
		return source, fmt.Errorf("source basename is empty")
	}
	err = r.DB.WithContext(ctx).Create(&source).Error
	return source, err
}
func (r Repository) Ready(ctx context.Context, id, clips string) error {
	return r.DB.WithContext(ctx).Model(&model.Source{}).Where("source_id=?", id).Update("clips_json", clips).Error
}
func (r Repository) Sources(ctx context.Context) ([]model.Source, error) {
	var rows []model.Source
	err := r.DB.WithContext(ctx).Where("clips_json IS NOT NULL").Order("source_id").Find(&rows).Error
	return rows, err
}
