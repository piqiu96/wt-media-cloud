package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/id"
	"gorm.io/gorm"
	"strconv"
	"time"
)

var (
	ErrNotFound = errors.New("publication not found")
	ErrConflict = errors.New("publication changed")
	ErrOccupied = errors.New("output already occupied")
)

type ConflictError struct {
	TaskID   uint64 `json:"task_id,string"`
	Revision uint64 `json:"current_revision,string"`
}

func (e *ConflictError) Error() string { return ErrConflict.Error() }
func (e *ConflictError) Unwrap() error { return ErrConflict }

type Filter struct {
	Actor                      identity.PublicUser
	Page, PageSize             int
	Search, Status             string
	Platform                   string
	CategoryID                 int64
	CreatedFrom, CreatedBefore *time.Time
}
type Store struct {
	DB        *gorm.DB
	LockFiles func(*gorm.DB, []int64) error
}

func missing(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}
func (s Store) ByRequest(actor int64, key string) (model.Task, error) {
	var v model.Task
	err := s.DB.Where("created_by=? AND request_key=?", actor, key).First(&v).Error
	return v, missing(err)
}
func (s Store) Get(id uint64) (model.Task, error) {
	var v model.Task
	err := s.DB.First(&v, id).Error
	return v, missing(err)
}
func (s Store) Create(v model.Task) (model.Task, error) {
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if s.LockFiles == nil {
			return errors.New("file reference locking unavailable")
		}
		if err := s.LockFiles(tx, []int64{v.CoverFileID}); err != nil {
			return err
		}
		if err := tx.Create(&v).Error; err != nil {
			return err
		}
		return audit(tx, v.CreatedBy, v.ID, "publication.create", v.Revision)
	})
	if err != nil {
		var duplicate *mysql.MySQLError
		if errors.As(err, &duplicate) && duplicate.Number == 1452 {
			return model.Task{}, ErrConflict
		}
		if errors.As(err, &duplicate) && duplicate.Number == 1062 {
			if prior, e := s.ByRequest(v.CreatedBy, v.RequestKey); e == nil {
				if prior.RequestHash == v.RequestHash {
					return prior, nil
				}
				return model.Task{}, ErrConflict
			}
			return model.Task{}, ErrOccupied
		}
	}
	return v, err
}
func (s Store) Edit(v model.Task, revision uint64) (model.Task, error) {
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if s.LockFiles == nil {
			return errors.New("file reference locking unavailable")
		}
		if err := s.LockFiles(tx, []int64{v.CoverFileID}); err != nil {
			return err
		}
		var old model.Task
		if err := tx.First(&old, v.ID).Error; err != nil {
			return missing(err)
		}
		if old.Revision != revision || old.Status != model.StatusPending || old.CurrentAttemptID != nil || old.AuthorizedAt != nil {
			return &ConflictError{TaskID: old.ID, Revision: old.Revision}
		}
		if err := updateTask(tx, old, map[string]any{"content_json": mustJSON(v.Content), "resource_snapshot_json": string(v.ResourceSnapshot), "cover_file_id": v.CoverFileID, "not_before": v.NotBefore, "updated_at": time.Now().UTC()}); err != nil {
			return err
		}
		return audit(tx, v.AuditActor, v.ID, "publication.edit", revision+1)
	})
	if err != nil {
		return model.Task{}, err
	}
	return s.Get(v.ID)
}
func (s Store) query(f Filter) *gorm.DB {
	q := s.DB.Model(&model.Task{})
	a := f.Actor
	if a.Role != identity.RoleAdmin {
		if a.Role == identity.RoleSeniorOperator && a.TeamID != nil {
			q = q.Where("team_id=?", *a.TeamID)
		} else {
			q = q.Where("operator_id=?", a.ID)
		}
		q = q.Where("category_id IN ?", a.CategoryIDs)
	}
	if f.Platform != "" {
		q = q.Where("platform=?", f.Platform)
	}
	if f.CategoryID > 0 {
		q = q.Where("category_id=?", f.CategoryID)
	}
	if f.CreatedFrom != nil {
		q = q.Where("created_at>=?", *f.CreatedFrom)
	}
	if f.CreatedBefore != nil {
		q = q.Where("created_at<?", *f.CreatedBefore)
	}
	if f.Status != "" {
		q = q.Where("status=?", f.Status)
	}
	if f.Search != "" {
		q = q.Where("CAST(id AS CHAR)=? OR JSON_UNQUOTE(JSON_EXTRACT(content_json,'$.description')) LIKE ?", f.Search, "%"+f.Search+"%")
	}
	return q
}
func (s Store) Summary(f Filter) (map[string]int64, error) {
	f.Status = ""
	var rows []struct {
		Status model.Status
		Count  int64
	}
	if err := s.query(f).Select("status,COUNT(*) AS count").Group("status").Scan(&rows).Error; err != nil {
		return nil, err
	}
	counts := map[string]int64{"all": 0, string(model.StatusAwaitingHuman): 0, string(model.StatusRunning): 0, string(model.StatusUncertain): 0}
	for _, row := range rows {
		counts[string(row.Status)] = row.Count
		counts["all"] += row.Count
	}
	return counts, nil
}
func (s Store) List(f Filter) ([]model.Task, int64, error) {
	q := s.query(f)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	out := []model.Task{}
	err := q.Order("id DESC").Offset((f.Page - 1) * f.PageSize).Limit(f.PageSize).Find(&out).Error
	return out, total, err
}
func audit(tx *gorm.DB, actor int64, task uint64, action string, revision uint64) error {
	return auditDetails(tx, actor, task, action, map[string]any{"revision": revision})
}
func auditDetails(tx *gorm.DB, actor int64, task uint64, action string, details any) error {
	var actorValue any
	if actor > 0 {
		actorValue = actor
	}
	return tx.Exec(`INSERT INTO audit_logs(id,actor_user_id,action,target_type,target_id,summary_json,created_at) VALUES(?,?,?,'publication',?,?,?)`, id.NewID("audit"), actorValue, action, fmt.Sprint(task), mustJSON(details), time.Now().UTC()).Error
}

func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

// FileReferenced is read while the caller holds the operations file row lock.
func FileReferenced(tx *gorm.DB, fileID int64) (bool, error) {
	var count int64
	err := tx.Model(&model.Task{}).Where("cover_file_id=?", fileID).Count(&count).Error
	if err != nil || count > 0 {
		return count > 0, err
	}
	err = tx.Model(&model.Attempt{}).Where("JSON_UNQUOTE(JSON_EXTRACT(input_snapshot_json,'$.content.cover_file_id'))=?", strconv.FormatInt(fileID, 10)).Count(&count).Error
	return count > 0, err
}

// FileReference identifies references without exposing task or attempt content.
type FileReference struct {
	Kind string
	ID   string
}

func ListFileReference(db *gorm.DB, a identity.PublicUser, id int64) ([]FileReference, error) {
	out := []FileReference{}
	var taskIDs, attemptIDs []uint64
	allowed := (Store{DB: db}).query(Filter{Actor: a}).Select("id")
	if e := db.Model(&model.Task{}).Where("id IN (?)", allowed).Where("cover_file_id=?", id).Pluck("id", &taskIDs).Error; e != nil {
		return nil, e
	}
	if e := db.Model(&model.Attempt{}).Where("publication_id IN (?)", allowed).Where("JSON_UNQUOTE(JSON_EXTRACT(input_snapshot_json,'$.content.cover_file_id'))=?", strconv.FormatInt(id, 10)).Pluck("id", &attemptIDs).Error; e != nil {
		return nil, e
	}
	for _, id := range taskIDs {
		out = append(out, FileReference{Kind: "publication", ID: strconv.FormatUint(id, 10)})
	}
	for _, id := range attemptIDs {
		out = append(out, FileReference{Kind: "publication_attempt", ID: strconv.FormatUint(id, 10)})
	}
	return out, nil
}
