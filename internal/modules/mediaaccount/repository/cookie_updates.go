package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/model"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// MutateCookie serializes the current account and the shared target window in a
// short transaction. No browser call is made while either row is locked.
func MutateCookie(id string, userID sharedidentity.UserID, mutate func(*model.AccountRecord, *string) error) (model.AccountRecord, error) {
	return mutateCookie(database.DB(), id, userID, mutate)
}
func mutateCookie(db *gorm.DB, id string, userID sharedidentity.UserID, mutate func(*model.AccountRecord, *string) error) (out model.AccountRecord, err error) {
	err = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Transaction(func(tx *gorm.DB) error {
		record, e := scanAccount(queryRow(tx, `SELECT `+accountReadColumns+accountReadFrom+` WHERE ma.id=? AND ma.user_id=? FOR UPDATE`, id, userID))
		if errors.Is(e, sql.ErrNoRows) {
			return model.ErrNotFound
		}
		if e != nil {
			return e
		}
		if record.BrowserProfileID == "" {
			return model.ErrProfileUnavailable
		}
		var lock sql.NullString
		var bitID string
		var enabled bool
		var reserved sql.NullString
		e = queryRow(tx, `SELECT cookie_operation_id, bit_profile_id, enabled, reserved_proxy_id FROM browser_profiles WHERE id=? AND user_id=? FOR UPDATE`, record.BrowserProfileID, userID).Scan(&lock, &bitID, &enabled, &reserved)
		if errors.Is(e, sql.ErrNoRows) {
			return model.ErrProfileUnavailable
		}
		if e != nil {
			return e
		}
		if reserved.Valid {
			return model.ErrCookieConflict
		}
		current := lock.String
		if e = mutate(&record, &current); e != nil {
			return e
		}
		if record.CookieUpdate == nil || record.CookieUpdate.BrowserProfileID != record.BrowserProfileID || record.CookieUpdate.BitProfileID != bitID {
			return model.ErrCookieConflict
		}
		if current != "" && !enabled {
			return model.ErrProfileUnavailable
		}
		encoded, e := json.Marshal(record.CookieUpdate)
		if e != nil {
			return e
		}
		_, e = execSQL(tx, `UPDATE media_accounts SET before_cookie=?,cookie_update=?,original_cookie=?,active_cookie=?,cookie_status=?,cookie_updated_at=?,platform_account_id=?,platform_name=?,platform_avatar_url=?,identification_status=?,platform_login_status=?,last_check_status=?,last_check_message=?,last_check_at=?,last_check_identity=?,check_operation_id=?,updated_at=? WHERE id=? AND user_id=?`, nullIfEmpty(record.BeforeCookie), string(encoded), nullIfEmpty(record.OriginalCookie), nullIfEmpty(record.ActiveCookie), nullIfEmpty(record.CookieStatus), record.CookieUpdatedAt, nullIfEmpty(record.PlatformAccountID), nullIfEmpty(record.PlatformName), nullIfEmpty(record.PlatformAvatarURL), record.IdentificationStatus, record.PlatformLoginStatus, record.LastCheckStatus, nullIfEmpty(record.LastCheckMessage), record.LastCheckAt, marshalIdentity(record.LastCheckIdentity), nullIfEmpty(record.CheckOperationID), record.UpdatedAt, id, userID)
		if duplicateKey(e) {
			return model.ErrProfilePlatformTaken
		}
		if e != nil {
			return e
		}
		_, e = execSQL(tx, `UPDATE browser_profiles SET cookie_operation_id=? WHERE id=?`, nullIfEmpty(current), record.BrowserProfileID)
		out = record
		return e
	})
	return
}
func marshalIdentity(identity *model.CheckIdentity) any {
	if identity == nil {
		return nil
	}
	b, _ := json.Marshal(identity)
	return string(b)
}
