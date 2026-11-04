package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"gorm.io/gorm"
)

type ComposeOutputPage struct {
	Items    []model.CompositeOutput `json:"items"`
	Total    int64                   `json:"total"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"page_size"`
}
type ComposeOutputStrategy struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
type ComposeOutputMaterial struct {
	MaterialID      int64                   `json:"material_id"`
	MaterialTitle   string                  `json:"material_title"`
	AuthorName      string                  `json:"author_name"`
	CategoryID      int64                   `json:"category_id"`
	OutputCount     int64                   `json:"output_count"`
	Strategies      []ComposeOutputStrategy `json:"strategies" gorm:"-"`
	LatestCreatedAt time.Time               `json:"latest_created_at"`
}
type ComposeOutputMaterialPage struct {
	Items    []ComposeOutputMaterial `json:"items"`
	Total    int64                   `json:"total"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"page_size"`
}

func outputQuery(db *gorm.DB, f ComposeOutputFilter) *gorm.DB {
	q := db.Table("composite_outputs o").Joins("LEFT JOIN materials m ON m.id=o.material_id").Joins("LEFT JOIN source_contents sc ON sc.id=m.source_content_id").Joins("LEFT JOIN compose_strategies s ON s.id=o.compose_strategy_id").Joins("LEFT JOIN compose_tasks t ON t.id=o.compose_task_id")
	if f.TeamID > 0 {
		q = q.Where("o.team_id=?", f.TeamID)
	}
	if f.OperatorID > 0 {
		q = q.Where("o.operator_id=?", f.OperatorID)
	}
	if len(f.CategoryIDs) > 0 {
		q = q.Where("o.category_id IN ?", f.CategoryIDs)
	}
	if f.MaterialID > 0 {
		q = q.Where("o.material_id=?", f.MaterialID)
	}
	if f.BatchID > 0 {
		q = q.Where("t.batch_id=?", f.BatchID)
	}
	if f.Platform != "" {
		q = q.Where("JSON_CONTAINS(t.platforms_json, JSON_QUOTE(?))", f.Platform)
	}
	if f.PublishStatus != "" {
		q = q.Where("o.publish_status=?", f.PublishStatus)
	}
	if f.TriggerType != "" {
		q = q.Where("t.trigger_type=?", f.TriggerType)
	}
	if f.Source != "" {
		q = q.Where("(CAST(o.material_id AS CHAR)=? OR m.title LIKE ? ESCAPE '!')", f.Source, likeContains(f.Source))
	}
	if f.Search != "" {
		v := likeContains(f.Search)
		q = q.Where("(CAST(o.id AS CHAR)=? OR CAST(o.material_id AS CHAR)=? OR CAST(t.batch_id AS CHAR)=? OR o.name LIKE ? ESCAPE '!' OR m.title LIKE ? ESCAPE '!' OR sc.author_name LIKE ? ESCAPE '!' OR s.name LIKE ? ESCAPE '!')", f.Search, f.Search, f.Search, v, v, v, v)
	}
	if !f.CreatedFrom.IsZero() {
		q = q.Where("o.created_at>=?", f.CreatedFrom)
	}
	if !f.CreatedBefore.IsZero() {
		q = q.Where("o.created_at<?", f.CreatedBefore)
	}
	if f.RestrictIDs {
		if len(f.OutputIDs) == 0 {
			q = q.Where("1=0")
		} else {
			q = q.Where("o.id IN ?", f.OutputIDs)
		}
	}
	return q
}
func (ComposeOutputStore) Page(ctx context.Context, f ComposeOutputFilter, page, size int) (ComposeOutputPage, error) {
	return pageComposeOutputs(database.DB().WithContext(ctx), f, page, size)
}
func pageComposeOutputs(db *gorm.DB, f ComposeOutputFilter, page, size int) (ComposeOutputPage, error) {
	result := ComposeOutputPage{Items: []model.CompositeOutput{}, Page: page, PageSize: size}
	if err := outputQuery(db, f).Count(&result.Total).Error; err != nil {
		return result, err
	}
	columns := strings.Split(composeOutputColumns, ", ")
	for i := range columns {
		columns[i] = "o." + columns[i]
	}
	rows, err := outputQuery(db, f).Select(strings.Join(columns, ", ") + ", COALESCE(m.title,''), COALESCE(sc.author_name,''), COALESCE(s.name,''), t.batch_id, COALESCE(t.setting_key,''), COALESCE(t.setting_name,''), t.platforms_json").Order("o.id DESC").Limit(size).Offset((page - 1) * size).Rows()
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var item model.CompositeOutput
		var platforms []byte
		wrapped := outputProjectionScanner{row: rows, extra: []any{&item.MaterialTitle, &item.AuthorName, &item.StrategyName, &item.BatchID, &item.SettingKey, &item.SettingName, &platforms}}
		base, err := scanComposeOutput(wrapped)
		if err != nil {
			return result, err
		}
		base.MaterialTitle = item.MaterialTitle
		base.AuthorName = item.AuthorName
		base.StrategyName = item.StrategyName
		base.BatchID = item.BatchID
		base.SettingKey = item.SettingKey
		base.SettingName = item.SettingName
		base.Platforms = []string{}
		if len(platforms) > 0 {
			if err = json.Unmarshal(platforms, &base.Platforms); err != nil {
				return result, err
			}
		}
		result.Items = append(result.Items, base)
	}
	return result, rows.Err()
}

type outputProjectionScanner struct {
	row   composeOutputScanner
	extra []any
}

func (s outputProjectionScanner) Scan(args ...any) error {
	return s.row.Scan(append(args, s.extra...)...)
}

type ComposeOutputObjectRef struct {
	ID        int64
	ObjectKey string
}

func (ComposeOutputStore) ObjectRefs(ctx context.Context, f ComposeOutputFilter) ([]ComposeOutputObjectRef, error) {
	return outputObjectRefs(database.DB().WithContext(ctx), f)
}
func outputObjectRefs(db *gorm.DB, f ComposeOutputFilter) ([]ComposeOutputObjectRef, error) {
	var refs []ComposeOutputObjectRef
	err := outputQuery(db, f).Select("o.id,o.object_key").Scan(&refs).Error
	return refs, err
}
func (ComposeOutputStore) MaterialPage(ctx context.Context, f ComposeOutputFilter, page, size int) (ComposeOutputMaterialPage, error) {
	return pageComposeOutputMaterials(database.DB().WithContext(ctx), f, page, size)
}
func pageComposeOutputMaterials(db *gorm.DB, f ComposeOutputFilter, page, size int) (ComposeOutputMaterialPage, error) {
	result := ComposeOutputMaterialPage{Items: []ComposeOutputMaterial{}, Page: page, PageSize: size}
	if err := outputQuery(db, f).Distinct("o.material_id").Count(&result.Total).Error; err != nil {
		return result, err
	}
	err := outputQuery(db, f).Select("o.material_id, COALESCE(m.title,'') material_title, COALESCE(sc.author_name,'') author_name, COALESCE(sc.category_id,0) category_id, COUNT(*) output_count, MAX(o.created_at) latest_created_at").Group("o.material_id,m.title,sc.author_name,sc.category_id").Order("latest_created_at DESC,o.material_id DESC").Limit(size).Offset((page - 1) * size).Scan(&result.Items).Error
	if err != nil {
		return result, err
	}
	ids := []int64{}
	index := map[int64]int{}
	for i := range result.Items {
		ids = append(ids, result.Items[i].MaterialID)
		index[result.Items[i].MaterialID] = i
		result.Items[i].Strategies = []ComposeOutputStrategy{}
	}
	if len(ids) == 0 {
		return result, nil
	}
	var strategies []struct {
		MaterialID int64
		ID         int64
		Name       string
	}
	err = outputQuery(db, f).Where("o.material_id IN ?", ids).Select("DISTINCT o.material_id,o.compose_strategy_id id,COALESCE(s.name,'') name").Order("o.compose_strategy_id").Scan(&strategies).Error
	if err != nil {
		return result, err
	}
	for _, s := range strategies {
		i := index[s.MaterialID]
		result.Items[i].Strategies = append(result.Items[i].Strategies, ComposeOutputStrategy{ID: s.ID, Name: s.Name})
	}
	return result, nil
}
func (ComposeOutputStore) OriginalTemplate(code string, version uint32) (model.ComposeTemplate, bool, error) {
	var t model.ComposeTemplate
	row := database.DB().Raw("SELECT id,code,name,version,schema_json,status,executor FROM compose_templates WHERE code=? AND version=?", code, version).Row()
	var schema []byte
	err := row.Scan(&t.ID, &t.Code, &t.Name, &t.Version, &schema, &t.Status, &t.ExecutorID)
	t.Schema = schema
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return t, false, nil
		}
		return t, false, err
	}
	return t, true, nil
}
