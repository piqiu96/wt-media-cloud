package model

import "time"

// ComposeMaterialState is the history of one (strategy, material) pair, read
// by the create drawer to split candidates into 未使用过 / 已使用过. It is a
// read model: no table backs it, the counts come from compose_tasks.
//
// The decision it feeds is "does this strategy already have a task row for this
// material" — any of the four counts above zero — because a pair is produced
// once and a second run goes through the retry action on the existing task,
// whatever its status. A pair with all four counts at zero is the only shape a
// create accepts, which is also the shape a missing row carries.
type ComposeMaterialState struct {
	MaterialID     int64      `json:"material_id"`
	SuccessCount   int        `json:"success_count"`
	ActiveCount    int        `json:"active_count"`
	FailedCount    int        `json:"failed_count"`
	CancelledCount int        `json:"cancelled_count"`
	LastTaskAt     *time.Time `json:"last_task_at"`
}
