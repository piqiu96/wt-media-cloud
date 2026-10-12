package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/executor"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"gorm.io/gorm"
)

var ErrTemplateDefinitionConflict = errors.New("published template differs from deployed executor")

// PublishTemplate inserts the deployed definition once and never rewrites an existing version.
func PublishTemplate(definition executor.Definition, name string) (model.ComposeTemplate, error) {
	return publishTemplate(database.DB(), definition, name, time.Now())
}

func publishTemplate(db *gorm.DB, definition executor.Definition, name string, now time.Time) (model.ComposeTemplate, error) {
	if definition.Code == "" || definition.Version == 0 || definition.ExecutorID == "" || definition.Executor == nil || !json.Valid(definition.Schema) {
		return model.ComposeTemplate{}, errors.New("template definition is incomplete")
	}
	if name == "" {
		return model.ComposeTemplate{}, errors.New("template name is empty")
	}
	result := db.Exec(`INSERT INTO compose_templates
		(code, name, version, schema_json, executor, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 'active', ?, ?)
		ON DUPLICATE KEY UPDATE id=id`, definition.Code, name, definition.Version, definition.Schema, definition.ExecutorID, now, now)
	if result.Error != nil {
		return model.ComposeTemplate{}, fmt.Errorf("publish compose template: %w", result.Error)
	}
	var item model.ComposeTemplate
	var storedSchema []byte
	var storedExecutor string
	err := db.Raw(`SELECT id, code, name, version, schema_json, executor, status FROM compose_templates
		WHERE code = ? AND version = ?`, definition.Code, definition.Version).
		Row().Scan(&item.ID, &item.Code, &item.Name, &item.Version, &storedSchema, &storedExecutor, &item.Status)
	if err != nil {
		return model.ComposeTemplate{}, fmt.Errorf("read published template: %w", err)
	}
	var current, deployed any
	if json.Unmarshal(storedSchema, &current) != nil || json.Unmarshal(definition.Schema, &deployed) != nil || !reflect.DeepEqual(current, deployed) || storedExecutor != definition.ExecutorID {
		return model.ComposeTemplate{}, fmt.Errorf("%w: %s/%d", ErrTemplateDefinitionConflict, definition.Code, definition.Version)
	}
	item.Schema = append(json.RawMessage(nil), storedSchema...)
	return item, nil
}
