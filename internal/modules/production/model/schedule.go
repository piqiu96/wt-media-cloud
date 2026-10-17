package model

type ComposeSchedule struct {
	Mode            string `json:"mode"`
	IntervalMinutes int    `json:"interval_minutes,omitempty"`
	DailyTime       string `json:"daily_time,omitempty"`
	Timezone        string `json:"timezone"`
}
