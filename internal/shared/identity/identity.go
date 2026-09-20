// Package identity owns stable identity contracts shared by Cloud modules.
package identity

import "time"

// UserID is the stable numeric identity of a Cloud user.
type UserID int64

// TeamID is the stable numeric identity of an operation team.
type TeamID int64

// AuditEvent is the cross-module audit record contract.
type AuditEvent struct {
	ID          string
	ActorUserID UserID
	Action      string
	TargetType  string
	TargetID    string
	Summary     map[string]string
	CreatedAt   time.Time
}
