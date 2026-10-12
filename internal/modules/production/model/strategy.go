package model

import (
	"encoding/json"
	"time"
)

type ComposeStrategy struct {
	ID              int64           `json:"id"`
	TeamID          int64           `json:"team_id"`
	GameID          string          `json:"game_id"`
	TemplateID      int64           `json:"compose_template_id" gorm:"column:compose_template_id"`
	TemplateVersion uint32          `json:"compose_template_version" gorm:"column:compose_template_version"`
	Name            string          `json:"name"`
	Config          json.RawMessage `json:"config" gorm:"column:config_json;type:json"`
	Status          string          `json:"status"`
	CreatedBy       int64           `json:"-"`
	UpdatedBy       int64           `json:"-"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

func (ComposeStrategy) TableName() string { return "compose_strategies" }
