package service

import (
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/profileguard/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/profileguard/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/profileguard/repository"
)

func Preflight(nodeID, nodeCredential, taskID string) (dto.PreflightOutcome, error) {
	return newService(mysqlStore{}, runtimeNodeAuth{}).Preflight(nodeID, nodeCredential, taskID)
}

func Renew(nodeID, nodeCredential, permitID, permitCredential string, extension time.Duration) (time.Time, error) {
	return newService(mysqlStore{}, runtimeNodeAuth{}).Renew(nodeID, nodeCredential, permitID, permitCredential, extension)
}

func Finish(nodeID, nodeCredential, permitID, permitCredential string, outcome model.FinishOutcome) error {
	return newService(mysqlStore{}, runtimeNodeAuth{}).Finish(nodeID, nodeCredential, permitID, permitCredential, outcome)
}

func CreateAuthorizedTask(task model.SensitiveTask) error {
	return repository.CreateAuthorizedTask(task)
}
