package service

import (
	"time"

	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/profileguard/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/profileguard/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/profileguard/repository"
)

func Preflight(userID identityservice.UserID, taskID string) (dto.PreflightOutcome, error) {
	return newService(mysqlStore{}).Preflight(userID, taskID)
}

func Renew(userID identityservice.UserID, permitID string, extension time.Duration) (time.Time, error) {
	return newService(mysqlStore{}).Renew(userID, permitID, extension)
}

func Finish(userID identityservice.UserID, permitID string, outcome model.FinishOutcome) error {
	return newService(mysqlStore{}).Finish(userID, permitID, outcome)
}

func CreateAuthorizedTask(task model.SensitiveTask) error {
	return repository.CreateAuthorizedTask(task)
}
