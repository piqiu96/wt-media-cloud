package repository

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/id"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BatchFilter struct {
	Actor                                                              identity.PublicUser
	Page, PageSize                                                     int
	Search, Platform, ConfigurationStatus, ExecutionStatus, RequestKey string
}

type BatchConflictError struct {
	BatchID  uint64 `json:"batch_id,string"`
	Revision uint64 `json:"current_revision,string"`
}

func (e *BatchConflictError) Error() string { return ErrConflict.Error() }
func (e *BatchConflictError) Unwrap() error { return ErrConflict }

type batchFile struct {
	BatchID uint64
	FileID  int64
}

func (batchFile) TableName() string { return "publication_batch_file" }

func (s Store) BatchByRequest(actor int64, key string) (model.Batch, error) {
	var v model.Batch
	err := s.DB.Where("created_by=? AND request_key=?", actor, key).First(&v).Error
	if err != nil {
		return v, missing(err)
	}
	return s.enrichBatch(v)
}

func (s Store) GetBatch(id uint64) (model.Batch, error) {
	var v model.Batch
	err := s.DB.First(&v, id).Error
	if err != nil {
		return v, missing(err)
	}
	return s.enrichBatch(v)
}

func batchScope(q *gorm.DB, a identity.PublicUser) *gorm.DB {
	if a.Role == identity.RoleAdmin {
		return q
	}
	if a.Role == identity.RoleSeniorOperator && a.TeamID != nil {
		q = q.Where("team_id=?", *a.TeamID)
	} else {
		q = q.Where("operator_id=?", a.ID)
	}
	if len(a.CategoryIDs) == 0 {
		return q.Where("1=0")
	}
	return q.Where(`NOT EXISTS (
  SELECT 1 FROM JSON_TABLE(publication_batch.targets_json, '$[*]'
   COLUMNS(category_id BIGINT UNSIGNED PATH '$.category_id')) AS target
  WHERE target.category_id NOT IN ?)`, a.CategoryIDs)
}

func (s Store) ListBatches(f BatchFilter) ([]model.Batch, int64, error) {
	q := batchScope(s.DB.Model(&model.Batch{}), f.Actor)
	if f.RequestKey != "" {
		q = q.Where("created_by=? AND request_key=?", f.Actor.ID, f.RequestKey)
	} else {
		if f.Search != "" {
			search := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(f.Search)
			q = q.Where("CAST(id AS CHAR)=? OR name LIKE ? ESCAPE '!'", f.Search, "%"+search+"%")
		}
		if f.Platform != "" {
			q = q.Where("platform=?", f.Platform)
		}
		if f.ConfigurationStatus != "" {
			q = q.Where("configuration_status=?", f.ConfigurationStatus)
		}
		if f.ExecutionStatus != "" {
			q = q.Where("execution_status=?", f.ExecutionStatus)
		}
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := []model.Batch{}
	err := q.Omit("config_json", "readiness_json", "request_hash").Order("created_at DESC,id DESC").Offset((f.Page - 1) * f.PageSize).Limit(f.PageSize).Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	for i, v := range rows {
		rows[i], err = s.enrichBatch(v)
		if err != nil {
			return nil, 0, err
		}
	}
	return rows, total, nil
}

func (s Store) CreateBatch(v model.Batch) (model.Batch, error) {
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&v).Error; err != nil {
			return err
		}
		if err := s.replaceBatchFiles(tx, v); err != nil {
			return err
		}
		return auditBatch(tx, v, "publication_batch.create")
	})
	if err != nil {
		var duplicate *mysql.MySQLError
		if errors.As(err, &duplicate) && duplicate.Number == 1062 {
			prior, e := s.BatchByRequest(v.CreatedBy, v.RequestKey)
			if e != nil {
				return model.Batch{}, e
			}
			if prior.RequestHash != v.RequestHash {
				return model.Batch{}, &BatchConflictError{prior.ID, prior.Revision}
			}
			return prior, nil
		}
	}
	if err != nil {
		return v, err
	}
	return s.enrichBatch(v)
}

func (s Store) EditBatch(v model.Batch, revision uint64) (model.Batch, error) {
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		var prior model.Batch
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&prior, v.ID).Error; err != nil {
			return missing(err)
		}
		if prior.OperatorID != v.OperatorID || prior.TeamID != v.TeamID {
			return ErrNotFound
		}
		if prior.Revision != revision || prior.StartedAt != nil || prior.ExecutionStatus != model.BatchNotStarted {
			return &BatchConflictError{prior.ID, prior.Revision}
		}
		if err := s.replaceBatchFiles(tx, v); err != nil {
			return err
		}
		v.Revision = revision + 1
		v.UpdatedAt = time.Now().UTC()
		result := tx.Model(&model.Batch{}).Where("id=? AND revision=?", v.ID, revision).Updates(map[string]any{
			"name": v.Name, "targets_json": mustJSON(v.Targets), "config_json": mustJSON(v.Config),
			"readiness_json": mustJSON(v.Readiness), "candidates_count": v.CandidatesCount,
			"configuration_status": v.ConfigurationStatus, "revision": v.Revision,
			"concurrency_limit": v.ExecutionSetting.ConcurrencyLimit, "group_interval_seconds": v.ExecutionSetting.GroupIntervalSeconds,
			"between_group_policy": v.ExecutionSetting.BetweenGroupPolicy, "exception_task_limit": v.ExecutionSetting.ExceptionTaskLimit,
			"updated_at": v.UpdatedAt,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return &BatchConflictError{prior.ID, prior.Revision}
		}
		return auditBatch(tx, v, "publication_batch.configure")
	})
	if err != nil {
		return v, err
	}
	return s.enrichBatch(v)
}

func (s Store) replaceBatchFiles(tx *gorm.DB, v model.Batch) error {
	var ids []int64
	for _, row := range v.Config.Candidates {
		for _, id := range []int64{row.Content.CoverFileID, row.Content.PortraitCoverFileID} {
			if id > 0 {
				ids = append(ids, id)
			}
		}
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	if len(ids) > 0 {
		if s.LockFiles == nil {
			return errors.New("batch file reference locking unavailable")
		}
		if err := s.LockFiles(tx, ids); err != nil {
			return err
		}
	}
	if err := tx.Where("batch_id=?", v.ID).Delete(&batchFile{}).Error; err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	rows := make([]batchFile, len(ids))
	for i, id := range ids {
		rows[i] = batchFile{v.ID, id}
	}
	return tx.CreateInBatches(rows, 100).Error
}

func batchFileReferenced(tx *gorm.DB, id int64) (bool, error) {
	var row batchFile
	err := tx.Where("file_id=?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return err == nil, err
}

func listBatchFileReferences(db *gorm.DB, a identity.PublicUser, id int64) ([]FileReference, error) {
	var ids []uint64
	allowed := batchScope(db.Model(&model.Batch{}), a).Select("id")
	err := db.Model(&batchFile{}).Where("file_id=? AND batch_id IN (?)", id, allowed).Pluck("batch_id", &ids).Error
	out := make([]FileReference, 0, len(ids))
	for _, batchID := range ids {
		out = append(out, FileReference{Kind: "publication_batch", ID: strconv.FormatUint(batchID, 10)})
	}
	return out, err
}

func auditBatch(tx *gorm.DB, v model.Batch, event string) error {
	return auditBatchDetails(tx, v, v.OperatorID, event, map[string]string{"revision": fmt.Sprint(v.Revision)})
}

func auditBatchDetails(tx *gorm.DB, v model.Batch, actor int64, event string, details any) error {
	var actorValue any
	if actor > 0 {
		actorValue = actor
	}
	return tx.Exec(`INSERT INTO audit_logs(id,actor_user_id,action,target_type,target_id,summary_json,created_at) VALUES(?,?,?,'publication_batch',?,?,?)`, id.NewID("audit"), actorValue, event, fmt.Sprint(v.ID), mustJSON(details), time.Now().UTC()).Error
}

func (s Store) ListBatchRecords(id uint64, page, size int) ([]model.BatchRecord, int64, error) {
	q := s.DB.Table("audit_logs").Where("target_type='publication_batch' AND target_id=?", strconv.FormatUint(id, 10))
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	records := []model.BatchRecord{}
	err := q.Select("id,actor_user_id AS actor_id,action AS event,created_at").Order("created_at DESC,id DESC").Offset((page - 1) * size).Limit(size).Scan(&records).Error
	return records, total, err
}
