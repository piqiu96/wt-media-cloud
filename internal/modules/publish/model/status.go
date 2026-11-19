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
