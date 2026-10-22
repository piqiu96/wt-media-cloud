package service

import (
	"time"

	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/profileguard/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/profileguard/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/profileguard/repository"
)

type mysqlStore struct{}

func (mysqlStore) FindAuthorizedTask(taskID string) (model.SensitiveTask, bool, error) {
	return repository.FindAuthorizedTask(taskID)
}
func (mysqlStore) AcquirePermit(task model.SensitiveTask, permit model.Permit, at time.Time) (dto.PreflightOutcome, error) {
	outcome, err := repository.AcquirePermit(task, permit, at)
	if err != nil {
		return dto.PreflightOutcome{}, err
	}
	return dto.PreflightOutcome{
		Outcome:   outcome.Outcome,
		PermitID:  outcome.PermitID,
		ProfileID: outcome.ProfileID,
		ExpiresAt: outcome.ExpiresAt,
	}, nil
}
func (mysqlStore) RenewPermit(permitID string, userID identityservice.UserID, at, expiresAt time.Time) (time.Time, error) {
	return repository.RenewPermit(permitID, userID, at, expiresAt)
}
func (mysqlStore) FinishPermit(permitID string, userID identityservice.UserID, outcome model.FinishOutcome, at time.Time) error {
	return repository.FinishPermit(permitID, userID, outcome, at)
}
