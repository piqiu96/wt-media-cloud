package repository

import (
	"database/sql"
	"errors"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity/model"
	"gorm.io/gorm"
)

func CheckPublicationScope(tx *gorm.DB, owner, team, category int64) (bool, error) {
	var role model.Role
	var status model.UserStatus
	var currentTeam sql.NullInt64
	e := queryRow(tx, "SELECT role,status,team_id FROM users WHERE id=? FOR SHARE", owner).Scan(&role, &status, &currentTeam)
	if errors.Is(e, sql.ErrNoRows) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	if status != model.UserStatusEnabled {
		return false, nil
	}
	if role != model.RoleAdmin && (role != model.RoleOperator && role != model.RoleSeniorOperator || !currentTeam.Valid || currentTeam.Int64 != team) {
		return false, nil
	}
	var id int64
	e = queryRow(tx, "SELECT id FROM categories WHERE id=? AND enabled=1 FOR SHARE", category).Scan(&id)
	if errors.Is(e, sql.ErrNoRows) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	if role == model.RoleAdmin {
		return true, nil
	}
	e = queryRow(tx, "SELECT category_id FROM user_category_scopes WHERE user_id=? AND category_id=? FOR SHARE", owner, category).Scan(&id)
	if errors.Is(e, sql.ErrNoRows) {
		return false, nil
	}
	return e == nil, e
}
