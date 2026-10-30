package model

import (
	"encoding/json"
	"time"

	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

type ComposeStrategy struct {
	ID              int64                     `json:"id"`
	TeamID          int64                     `json:"team_id"`
	CategoryID      sharedidentity.CategoryID `json:"category_id"`
	OperatorID      *int64                    `json:"operator_id"`
	OperatorName    string                    `json:"operator_name" gorm:"-"`
	TemplateID      int64                     `json:"compose_template_id" gorm:"column:compose_template_id"`
	TemplateVersion uint32                    `json:"compose_template_version" gorm:"column:compose_template_version"`
	Name            string                    `json:"name"`
	Config          json.RawMessage           `json:"config" gorm:"column:config_json;type:json"`
	ExecutionMode   string                    `json:"execution_mode" gorm:"column:execution_mode"`
	PriorityRules   []ComposePriorityRule     `json:"priority_rules" gorm:"-"`
	RulesJSON       json.RawMessage           `json:"-" gorm:"column:priority_rules_json;type:json"`
	SingleLimit     int                       `json:"single_limit"`
	DailyLimit      int                       `json:"daily_limit"`
	Schedule        *ComposeSchedule          `json:"schedule" gorm:"-"`
	ScheduleJSON    json.RawMessage           `json:"-" gorm:"column:schedule_json;type:json"`
	NextRunAt       *time.Time                `json:"next_run_at" gorm:"column:next_run_at"`
	Status          string                    `json:"status"`
	CreatedBy       int64                     `json:"-"`
	UpdatedBy       int64                     `json:"-"`
	CreatedAt       time.Time                 `json:"created_at"`
	UpdatedAt       time.Time                 `json:"updated_at"`
}

const (
	ComposeStrategyEnabled    = "enabled"
	ComposeStrategyDisabled   = "disabled"
	ComposeExecutionManual    = "manual"
	ComposeExecutionScheduled = "scheduled"
)

type ComposePriorityRule struct {
	Priority   int                `json:"priority"`
	Action     string             `json:"action"`
	Logic      string             `json:"logic"`
	Conditions []ComposeCondition `json:"conditions"`
}

type ComposeCondition struct {
	Field    string          `json:"field"`
	Operator string          `json:"operator"`
	Value    json.RawMessage `json:"value"`
}

func (ComposeStrategy) TableName() string { return "compose_strategies" }
