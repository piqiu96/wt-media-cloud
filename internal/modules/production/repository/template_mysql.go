package repository

import (
	"encoding/json"
	"fmt"

	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"gorm.io/gorm"
)

type TemplateStore struct{}

func (TemplateStore) ListActiveTemplates() ([]model.ComposeTemplate, error) {
	return listActiveTemplates(database.DB())
}

func listActiveTemplates(db *gorm.DB) ([]model.ComposeTemplate, error) {
	rows, err := db.Raw(`SELECT id, code, name, version, schema_json, status
		FROM compose_templates WHERE status = 'active' ORDER BY code, version DESC`).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]model.ComposeTemplate, 0)
	for rows.Next() {
		var item model.ComposeTemplate
		var schema []byte
		if err := rows.Scan(&item.ID, &item.Code, &item.Name, &item.Version, &schema, &item.Status); err != nil {
			return nil, err
		}
		if !json.Valid(schema) {
			return nil, fmt.Errorf("template %d has invalid schema_json", item.ID)
		}
		item.Schema = append(json.RawMessage(nil), schema...)
		items = append(items, item)
	}
	return items, rows.Err()
}
