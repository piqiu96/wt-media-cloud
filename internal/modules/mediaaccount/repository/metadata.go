package repository

import (
	"database/sql"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/model"
	"gorm.io/gorm"
	"strings"
)

func UpdateMetadata(record model.AccountRecord, input dto.UpdateAccountInput) error {
	return updateMetadata(database.DB(), record, input)
}
func updateMetadata(db *gorm.DB, record model.AccountRecord, input dto.UpdateAccountInput) error {
	return db.Transaction(func(tx *gorm.DB) error {
		// Serialize metadata changes and relation replacement, without touching platform facts.
		var id string
		if err := queryRow(tx, "SELECT id FROM media_accounts WHERE id=? AND user_id=? FOR UPDATE", record.ID, record.UserID).Scan(&id); err != nil {
			return err
		}
		fields := []string{"updated_at=?"}
		args := []any{record.UpdatedAt}
		if input.Enabled != nil {
			fields = append(fields, "enabled=?")
			args = append(args, *input.Enabled)
		}
		if input.Remark != nil {
			fields = append(fields, "remark=?")
			args = append(args, nullIfEmpty(record.Remark))
		}
		args = append(args, record.ID)
		if _, err := execSQL(tx, "UPDATE media_accounts SET "+strings.Join(fields, ",")+" WHERE id=?", args...); err != nil {
			return err
		}
		if input.CategoryIDs != nil {
			if err := replaceAccountCategoryIDs(tx, record.ID, record.CategoryIDs); err != nil {
				return err
			}
		}
		if input.Tags != nil {
			if err := replaceAccountTags(tx, record.ID, record.Tags, record.UpdatedAt); err != nil {
				return err
			}
		}
		return nil
	})
}
func replaceAccountTags(tx *gorm.DB, accountID string, tags []string, now any) error {
	if _, err := execSQL(tx, "DELETE FROM media_account_tags WHERE media_account_id=?", accountID); err != nil {
		return err
	}
	for _, tag := range tags {
		if _, err := execSQL(tx, "INSERT INTO media_account_tags (media_account_id,tag_name,created_at) VALUES (?,?,?)", accountID, tag, now); err != nil {
			return err
		}
	}
	return nil
}
func SetProfile(record model.AccountRecord) error { return setProfile(database.DB(), record) }
func setProfile(db *gorm.DB, record model.AccountRecord) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var old sql.NullString
		if err := queryRow(tx, `SELECT browser_profile_id FROM media_accounts WHERE id=? AND user_id=? FOR UPDATE`, record.ID, record.UserID).Scan(&old); err != nil {
			return err
		}
		rows, err := queryRows(tx, `SELECT id,cookie_operation_id FROM browser_profiles WHERE id IN (?,?) ORDER BY id FOR UPDATE`, old.String, record.BrowserProfileID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id string
			var op sql.NullString
			if err = rows.Scan(&id, &op); err != nil {
				rows.Close()
				return err
			}
			if op.Valid && op.String != "" {
				rows.Close()
				return model.ErrCookieConflict
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		n, err := execSQL(tx, `UPDATE media_accounts SET browser_profile_id=?, check_operation_id=NULL, check_profile_id=NULL, check_started_at=NULL, last_check_status='never', last_check_message=NULL, last_check_identity=NULL, updated_at=? WHERE id=? AND user_id=?`, nullIfEmpty(record.BrowserProfileID), record.UpdatedAt, record.ID, record.UserID)
		if duplicateKey(err) {
			return model.ErrProfilePlatformTaken
		}
		if err != nil {
			return err
		}
		if n != 1 {
			return model.ErrNotFound
		}
		return nil
	})
}
