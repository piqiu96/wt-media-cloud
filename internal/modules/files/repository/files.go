package repository

import (
	"errors"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/files/model"
	"gorm.io/gorm"
)

var ErrNotFound = errors.New("file not found")

type Store struct{ DB *gorm.DB }

func (s Store) Insert(f *model.File) error { return s.DB.Create(f).Error }
func (s Store) SetLocation(id int64, key string) error {
	return s.DB.Exec(`UPDATE operations_files SET object_key=? WHERE id=? AND uploaded_at IS NULL`, key, id).Error
}
func (s Store) Ready(f model.File) error {
	r := s.DB.Exec(`UPDATE operations_files SET object_key=?,size_bytes=?,sha256=?,uploaded_at=?,updated_at=? WHERE id=? AND uploaded_at IS NULL`, f.ObjectKey, f.SizeBytes, f.SHA256, f.UploadedAt, f.UpdatedAt, f.ID)
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected != 1 {
		return ErrNotFound
	}
	return nil
}
func (s Store) Find(id int64) (model.File, error) {
	var f model.File
	err := s.DB.Where("id=? AND uploaded_at IS NOT NULL", id).First(&f).Error
	return f, missing(err)
}
func (s Store) FindToken(token string) (model.File, error) {
	var f model.File
	err := s.DB.Where("resource_token=? AND uploaded_at IS NOT NULL", token).First(&f).Error
	return f, missing(err)
}
func (s Store) Rename(id int64, name string) error {
	return s.DB.Exec(`UPDATE operations_files SET name=?,updated_at=? WHERE id=? AND uploaded_at IS NOT NULL`, name, time.Now(), id).Error
}
func (s Store) Delete(id int64) error {
	return s.DB.Exec(`DELETE FROM operations_files WHERE id=?`, id).Error
}
func (s Store) List(scope model.Scope, kind, search string, page, size int) ([]model.File, int64, error) {
	q := s.DB.Table("operations_files f").Where("f.uploaded_at IS NOT NULL")
	if !scope.All {
		if scope.OwnerID > 0 {
			q = q.Where("f.owner_id=?", scope.OwnerID)
		} else if scope.TeamID > 0 {
			q = q.Where("f.team_id=?", scope.TeamID)
		} else {
			q = q.Where("1=0")
		}
	}
	if kind != "" {
		q = q.Where("f.type=?", kind)
	}
	if search != "" {
		q = q.Where("f.name LIKE ? OR CAST(f.id AS CHAR)=?", "%"+search+"%", search)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.File
	err := q.Select("f.*,COALESCE(NULLIF(u.nickname,''),u.username,'') AS owner_name,COALESCE(t.name,'') AS team_name").Joins("LEFT JOIN users u ON u.id=f.owner_id").Joins("LEFT JOIN operation_teams t ON t.id=f.team_id").Order("f.created_at DESC,f.id DESC").Offset((page - 1) * size).Limit(size).Scan(&items).Error
	return items, total, err
}
func missing(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}
