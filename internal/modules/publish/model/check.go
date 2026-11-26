package model

import (
	"encoding/json"
	"time"
)

type PublicCheckClaim struct {
	ReservationID          uint64  `json:"reservation_id,string,omitempty"`
	ReservationState       string  `json:"reservation_state,omitempty"`
	Replayed               bool    `json:"replayed"`
	BrowserFallbackAllowed bool    `json:"browser_fallback_allowed"`
	Task                   Task    `json:"task"`
	Attempt                Attempt `json:"attempt"`
	Generation             uint64  `json:"generation,string"`
	ExpectedRevision       uint64  `json:"expected_revision,string"`
}
type PublicCheckReport struct {
	ReservationID      uint64           `json:"reservation_id,string"`
	ProfileDisposition string           `json:"profile_disposition"`
	BrowserUsed        bool             `json:"browser_used,omitempty"`
	LookupResult       string           `json:"lookup_result,omitempty"`
	Generation         uint64           `json:"generation,string"`
	Observation        ResultResolution `json:"observation"`
}

func (r PublicCheckReport) Valid() bool {
	return r.ValidFor("baidu")
}

func (r PublicCheckReport) ValidFor(platform string) bool {
	lookupValid := r.LookupResult == "" || ((r.LookupResult == "found") && (r.Observation.Conclusion == StatusReviewing || r.Observation.Conclusion == StatusPublished)) || (r.LookupResult == "not_found" && r.Observation.Conclusion == StatusUncertain && r.Observation.Evidence.SceneClosed && !r.Observation.Evidence.WorkMatched) || (r.LookupResult == "blocked" && r.Observation.Conclusion == StatusUncertain)
	if r.ReservationID != 0 && (!ValidProfileDisposition(r.ProfileDisposition) || !r.Observation.Evidence.SceneClosed ||
		r.ProfileDisposition == "not_opened" && r.BrowserUsed) {
		return false
	}
	return lookupValid && r.Generation > 0 && r.Observation.ValidFor(platform) && (r.Observation.Conclusion == StatusUncertain || r.Observation.Conclusion == StatusReviewing || r.Observation.Conclusion == StatusPublished)
}

type PublicCheckAcknowledgement struct {
	ReservationID   uint64 `json:"reservation_id,string,omitempty"`
	Released        bool   `json:"released,omitempty"`
	PublicationID   uint64 `json:"publication_id,string"`
	AttemptID       uint64 `json:"attempt_id,string"`
	Generation      uint64 `json:"generation,string"`
	ObservationHash string `json:"observation_hash"`
}
type PublicationObservation struct {
	ID               uint64          `json:"id,string"`
	PublicationID    uint64          `json:"publication_id,string"`
	AttemptID        uint64          `json:"attempt_id,string"`
	ObserverID       int64           `json:"observer_id,string"`
	Generation       uint64          `json:"generation,string"`
	ExpectedRevision uint64          `json:"expected_revision,string"`
	ObservationHash  string          `json:"observation_hash"`
	Observation      json.RawMessage `json:"observation" gorm:"column:observation_json"`
	ObservedAt       time.Time       `json:"observed_at"`
}

func (PublicationObservation) TableName() string { return "publication_observation" }

type CheckActor struct {
	UserID    int64
	SessionID string
	Current   bool
}

type CheckClaimInput struct {
	SupportedPlatforms []string          `json:"supported_platforms,omitempty"`
	RequestKey         string            `json:"request_key"`
	Capacity           ExecutionCapacity `json:"capacity"`
	SettleUnreceived   bool              `json:"settle_unreceived"`
}

type CancelledCheckClaim struct {
	RequestKey       string `json:"request_key"`
	ReservationState string `json:"reservation_state"`
}

type PublicCheckRelease struct {
	ReleaseOnly        bool   `json:"release_only"`
	ReservationID      uint64 `json:"reservation_id,string"`
	AttemptID          uint64 `json:"attempt_id,string"`
	Generation         uint64 `json:"generation,string"`
	ProfileDisposition string `json:"profile_disposition"`
}

type PublicCheckReleaseAck struct {
	PublicationID    uint64 `json:"publication_id,string"`
	AttemptID        uint64 `json:"attempt_id,string"`
	Generation       uint64 `json:"generation,string"`
	ReservationID    uint64 `json:"reservation_id,string"`
	Released         bool   `json:"released"`
	ObservationHash  string `json:"observation_hash,omitempty"`
	ReservationState string `json:"reservation_state,omitempty"`
	Replayed         bool   `json:"replayed,omitempty"`
}

func ValidProfileDisposition(v string) bool { return v == "not_opened" || v == "closed" }

func (r PublicCheckRelease) Valid() bool {
	return r.ReleaseOnly && r.ReservationID > 0 && r.AttemptID > 0 && r.Generation > 0 && ValidProfileDisposition(r.ProfileDisposition)
}
