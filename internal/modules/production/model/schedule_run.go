package model

import "time"

type ComposeScheduleSkip struct {
	MaterialID int64  `json:"material_id"`
	Reason     string `json:"reason"`
}

type ComposeScheduleRun struct {
	RunKey         string                `json:"run_key"`
	StrategyID     int64                 `json:"compose_strategy_id"`
	TriggerType    string                `json:"trigger_type"`
	ScheduleSlot   time.Time             `json:"schedule_slot"`
	CandidateCount int                   `json:"candidate_count"`
	CreatedCount   int                   `json:"created_count"`
	Skipped        []ComposeScheduleSkip `json:"skipped"`
	CreatedAt      time.Time             `json:"created_at"`
}
