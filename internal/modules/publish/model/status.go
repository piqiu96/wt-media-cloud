package model

// Status belongs to a publication task; other task modules own their statuses.
type Status string

const (
	StatusPending       Status = "pending"
	StatusQueued        Status = "queued"
	StatusRunning       Status = "running"
	StatusAwaitingHuman Status = "awaiting_human"
	StatusUncertain     Status = "uncertain"
	StatusReviewing     Status = "reviewing"
	StatusFailed        Status = "failed"
	StatusPublished     Status = "published"
	StatusCancelled     Status = "cancelled"
	StatusAbandoned     Status = "abandoned"
)

func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusQueued, StatusRunning, StatusAwaitingHuman, StatusUncertain, StatusReviewing, StatusFailed, StatusPublished, StatusCancelled, StatusAbandoned:
		return true
	}
	return false
}

// AttemptStatus describes one execution attempt, independently of task status.
type AttemptStatus string

const (
	AttemptStatusRunning       AttemptStatus = "running"
	AttemptStatusAwaitingHuman AttemptStatus = "awaiting_human"
	AttemptStatusCompleted     AttemptStatus = "completed"
	AttemptStatusInterrupted   AttemptStatus = "interrupted"
)

// AttemptStage records progress within an execution attempt.
type AttemptStage string

const (
	AttemptStageClaimed        AttemptStage = "claimed"
	AttemptStagePreparing      AttemptStage = "preparing"
	AttemptStageReady          AttemptStage = "ready"
	AttemptStageSubmitting     AttemptStage = "submitting"
	AttemptStageAwaitingHuman  AttemptStage = "awaiting_human"
	AttemptStageConnectionLost AttemptStage = "connection_lost"
	AttemptStageFinished       AttemptStage = "finished"
)
