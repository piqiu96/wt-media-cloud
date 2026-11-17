package repository

import (
	"database/sql"
	"errors"
	"gorm.io/gorm"
)

func CheckPublicationScope(tx *gorm.DB, owner, team, category int64) (bool, error) {
	var role, status string
	var currentTeam sql.NullInt64
	e := queryRow(tx, "SELECT role,status,team_id FROM users WHERE id=? FOR UPDATE", owner).Scan(&role, &status, &currentTeam)
	if errors.Is(e, sql.ErrNoRows) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	if status != "enabled" {
		return false, nil
	}
	if role != "admin" && (role != "operator" && role != "senior_operator" || !currentTeam.Valid || currentTeam.Int64 != team) {
		return false, nil
	}
	var id int64
	e = queryRow(tx, "SELECT id FROM categories WHERE id=? AND enabled=1 FOR UPDATE", category).Scan(&id)
	if errors.Is(e, sql.ErrNoRows) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	if role == "admin" {
		return true, nil
	}
	e = queryRow(tx, "SELECT category_id FROM user_category_scopes WHERE user_id=? AND category_id=? FOR UPDATE", owner, category).Scan(&id)
	if errors.Is(e, sql.ErrNoRows) {
		return false, nil
	}
	return e == nil, e
}
