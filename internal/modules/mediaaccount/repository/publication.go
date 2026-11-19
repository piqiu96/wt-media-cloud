package repository

import (
	"database/sql"
	"errors"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/model"
	"gorm.io/gorm"
)

type PublicationBinding struct {
	ID                     uint64
	Owner, Team, Category  int64
	Platform, UID, Profile string
}

func CheckPublicationBinding(tx *gorm.DB, want PublicationBinding) (bool, error) {
	var owner int64
	var team sql.NullInt64
	var platform, uid string
	var identification model.IdentificationStatus
	var login model.PlatformLoginStatus
	var profile sql.NullString
	var enabled bool
	e := queryRow(tx, `SELECT user_id,team_id,platform,COALESCE(platform_account_id,''),browser_profile_id,enabled,identification_status,platform_login_status FROM media_accounts WHERE id=? FOR SHARE`, want.ID).Scan(&owner, &team, &platform, &uid, &profile, &enabled, &identification, &login)
	if errors.Is(e, sql.ErrNoRows) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	if owner != want.Owner || !team.Valid || team.Int64 != want.Team || platform != want.Platform || uid != want.UID || !profile.Valid || profile.String != want.Profile || !enabled || identification != model.IdentificationIdentified || login != model.PlatformLoginNormal {
		return false, nil
	}
	var category int64
	e = queryRow(tx, `SELECT category_id FROM media_account_categories WHERE media_account_id=? AND category_id=? FOR SHARE`, want.ID, want.Category).Scan(&category)
	if errors.Is(e, sql.ErrNoRows) {
		return false, nil
	}
	return e == nil, e
}
