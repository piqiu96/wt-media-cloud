package service

import (
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/profileguard/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/profileguard/model"
)

func Preflight(nodeID, nodeCredential, taskID string) (dto.PreflightOutcome, error) {
	return NewService(mysqlStore{}, runtimeNodeAuth{}).Preflight(nodeID, nodeCredential, taskID)
}

func Renew(nodeID, nodeCredential, permitID, permitCredential string, extension time.Duration) (time.Time, error) {
	return NewService(mysqlStore{}, runtimeNodeAuth{}).Renew(nodeID, nodeCredential, permitID, permitCredential, extension)
}

func Finish(nodeID, nodeCredential, permitID, permitCredential string, outcome model.FinishOutcome) error {
	return NewService(mysqlStore{}, runtimeNodeAuth{}).Finish(nodeID, nodeCredential, permitID, permitCredential, outcome)
}
