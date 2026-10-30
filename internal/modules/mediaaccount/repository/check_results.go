package repository

import (
	"encoding/json"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/model"
	"gorm.io/gorm"
)

// StartCheck changes only current check facts, conditional on the selected window.
func StartCheck(record model.AccountRecord) error { return startCheck(database.DB(), record) }
func startCheck(db *gorm.DB, record model.AccountRecord) error {
	n, err := execSQL(db, `UPDATE media_accounts SET check_operation_id=?, check_profile_id=?, check_started_at=?, last_check_status='running', last_check_message=NULL WHERE id=? AND user_id=? AND browser_profile_id=? AND NOT EXISTS (SELECT 1 FROM browser_profiles bp WHERE bp.id=media_accounts.browser_profile_id AND bp.cookie_operation_id IS NOT NULL)`, record.CheckOperationID, record.CheckProfileID, record.CheckStartedAt, record.ID, record.UserID, record.CheckProfileID)
	if err != nil {
		return err
	}
	if n != 1 {
		return model.ErrStaleCheck
	}
	return nil
}

// CompleteCheck never rewrites credentials, categories, enabled or remarks.
func CompleteCheck(record model.AccountRecord) error { return completeCheck(database.DB(), record) }
func completeCheck(db *gorm.DB, record model.AccountRecord) error {
	var identity any
	if record.LastCheckIdentity != nil {
		b, err := json.Marshal(record.LastCheckIdentity)
		if err != nil {
			return err
		}
		identity = string(b)
	}
	n, err := execSQL(db, `UPDATE media_accounts SET platform_account_id=?, platform_name=?, platform_avatar_url=?, identification_status=?, platform_login_status=?, last_check_at=?, last_check_items=?, last_check_status=?, last_check_message=?, last_check_identity=?, updated_at=? WHERE id=? AND user_id=? AND check_operation_id=? AND check_profile_id=browser_profile_id AND last_check_status='running'`, nullIfEmpty(record.PlatformAccountID), nullIfEmpty(record.PlatformName), nullIfEmpty(record.PlatformAvatarURL), record.IdentificationStatus, record.PlatformLoginStatus, record.LastCheckAt, marshalLastCheckItems(record.LastCheckItems), record.LastCheckStatus, nullIfEmpty(record.LastCheckMessage), identity, record.UpdatedAt, record.ID, record.UserID, record.CheckOperationID)
	if duplicateKey(err) {
		return model.ErrProfilePlatformTaken
	}
	if err != nil {
		return err
	}
	if n != 1 {
		return model.ErrStaleCheck
	}
	return nil
}
