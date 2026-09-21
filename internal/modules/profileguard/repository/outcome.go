package repository

import (
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/profileguard/model"
)

// PreflightOutcome is the persistence result of a permit acquisition.
type PreflightOutcome struct {
	Outcome          model.Outcome
	PermitID         string
	PermitCredential string
	ProfileID        string
	ExpiresAt        *time.Time
}
