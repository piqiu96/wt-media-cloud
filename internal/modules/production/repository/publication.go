package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"gorm.io/gorm"
)

func LockSavedPublicationOutput(tx *gorm.DB, id int64) (model.CompositeOutput, bool, error) {
	item, e := scanComposeOutput(tx.Raw(`SELECT `+composeOutputColumns+` FROM composite_outputs WHERE id=? FOR SHARE`, id).Row())
	if errors.Is(e, sql.ErrNoRows) {
		return item, false, nil
	}
	if e != nil {
		return item, false, e
	}
	var platforms []byte
	e = tx.Raw("SELECT platforms_json FROM compose_tasks WHERE id=? FOR SHARE", item.TaskID).Row().Scan(&platforms)
	if e != nil {
		return item, false, e
	}
	if e = json.Unmarshal(platforms, &item.Platforms); e != nil {
		return item, false, e
	}
	return item, true, nil
}
