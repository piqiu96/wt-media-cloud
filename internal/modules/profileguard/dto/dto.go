// Package dto owns Profile Guard response contracts.
package dto

import (
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/profileguard/model"
)

type PreflightOutcome struct {
	Outcome          model.Outcome `json:"outcome"`
	PermitID         string        `json:"permit_id,omitempty"`
	PermitCredential string        `json:"permit_credential,omitempty"`
	ProfileID        string        `json:"profile_id"`
	ExpiresAt        *time.Time    `json:"expires_at,omitempty"`
}
