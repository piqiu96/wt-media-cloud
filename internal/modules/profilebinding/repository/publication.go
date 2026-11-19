package repository

import (
	"database/sql"
	"errors"
	identitymodel "github.com/wt-media/wt-media-cloud/internal/modules/identity/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/model"
	"gorm.io/gorm"
)

// The joined owner lock also serializes per-user publication admission. It does
// not lock another user's work or introduce a durable scheduling lease.
func LockPublicationProfile(tx *gorm.DB, id string, owner int64) (model.BrowserProfile, error) {
	p, e := scanProfile(queryRow(tx, `SELECT `+profileColumnsWithPrefix("bp")+` FROM browser_profiles bp JOIN users u ON u.id=bp.user_id WHERE bp.id=? AND bp.user_id=? AND u.status=? AND bp.bit_main_user_id=u.bit_main_user_id FOR UPDATE`, id, owner, identitymodel.UserStatusEnabled))
	if errors.Is(e, sql.ErrNoRows) {
		return p, model.ErrProfileNotFound
	}
	if e != nil {
		return p, e
	}
	var cookie sql.NullString
	if e = queryRow(tx, "SELECT cookie_operation_id FROM browser_profiles WHERE id=?", id).Scan(&cookie); e != nil {
		return p, e
	}
	if !p.Enabled || p.LocalState != model.ProfileActive || p.ReservedProxyID != "" || cookie.Valid && cookie.String != "" {
		return p, model.ErrProfileInactive
	}
	return p, nil
}
