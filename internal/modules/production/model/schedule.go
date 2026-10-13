package model

const ComposeMaterialScopeCreatorUsages = "creator_usages"

type ComposeSchedule struct {
	Enabled         bool   `json:"enabled"`
	Mode            string `json:"mode"`
	IntervalMinutes int    `json:"interval_minutes,omitempty"`
	DailyTime       string `json:"daily_time,omitempty"`
	Timezone        string `json:"timezone"`
	PerRunLimit     int    `json:"per_run_limit"`
	DailyLimit      int    `json:"daily_limit"`
	MaterialScope   string `json:"material_scope"`
}

