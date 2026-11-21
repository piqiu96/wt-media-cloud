package model

import (
	"encoding/json"
	"time"
)

type PublicCheckClaim struct {
	Task             Task    `json:"task"`
	Attempt          Attempt `json:"attempt"`
	Generation       uint64  `json:"generation,string"`
	ExpectedRevision uint64  `json:"expected_revision,string"`
}
type PublicCheckReport struct {
	Generation  uint64           `json:"generation,string"`
	Observation ResultResolution `json:"observation"`
}

func (r PublicCheckReport) Valid() bool {
	return r.Generation > 0 && r.Observation.Valid() && (r.Observation.Conclusion == StatusUncertain || r.Observation.Conclusion == StatusReviewing || r.Observation.Conclusion == StatusPublished)
}

type PublicCheckAcknowledgement struct {
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
