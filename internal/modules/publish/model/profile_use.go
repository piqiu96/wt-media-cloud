package model

import "time"

type ExecutionCapacity struct {
	AutomaticSlots uint16 `json:"automatic_slots"`
	WindowSlots    uint16 `json:"window_slots"`
}

func (c ExecutionCapacity) Valid() bool { return c.AutomaticSlots > 0 && c.WindowSlots > 0 }

func (c ExecutionCapacity) Limit(other ExecutionCapacity) ExecutionCapacity {
	windows := min(c.WindowSlots, other.WindowSlots)
	return ExecutionCapacity{AutomaticSlots: min(c.AutomaticSlots, other.AutomaticSlots, windows), WindowSlots: windows}
}

type ProfileUse struct {
	ID                          uint64
	BitMainUserID               string
	BitProfileID                string
	OwnerID                     int64
	PublicationID               uint64
	AttemptID                   *uint64
	Kind                        string
	CheckGeneration             *uint64
	CheckRevision               *uint64
	CheckBrowserFallbackAllowed bool
	WindowSlot                  uint16
	AcquiredAt                  time.Time
	ReleasedAt                  *time.Time
	Disposition                 *string
	ObservationHash             *string
}

func (ProfileUse) TableName() string { return "publication_profile_use" }

type PublicCheckRequest struct {
	OwnerID        int64
	RequestKey     string
	AgentSessionID string
	RequestHash    *string
	ReservationID  *uint64
	CancelledAt    *time.Time
	CreatedAt      time.Time
}

func (PublicCheckRequest) TableName() string { return "publication_check_request" }
