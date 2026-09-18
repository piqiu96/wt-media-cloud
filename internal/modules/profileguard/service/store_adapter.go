package service

import (
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/profileguard/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/profileguard/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/profileguard/repository"
	runtimeservice "github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding/service"
)

type mysqlStore struct{}

func (mysqlStore) FindAuthorizedTask(taskID string) (model.SensitiveTask, bool, error) {
	return repository.FindAuthorizedTask(taskID)
}
func (mysqlStore) AcquirePermit(task model.SensitiveTask, node runtimeservice.AgentNode, permit model.Permit, at time.Time, freshness time.Duration) (dto.PreflightOutcome, error) {
	return repository.AcquirePermit(task, node, permit, at, freshness)
}
func (mysqlStore) RenewPermit(permitID, nodeID, credentialHash string, at, expiresAt time.Time) (time.Time, error) {
	return repository.RenewPermit(permitID, nodeID, credentialHash, at, expiresAt)
}
func (mysqlStore) FinishPermit(permitID, nodeID, credentialHash string, outcome model.FinishOutcome, at time.Time) error {
	return repository.FinishPermit(permitID, nodeID, credentialHash, outcome, at)
}

type runtimeNodeAuth struct{}

func (runtimeNodeAuth) AuthenticateNode(nodeID, credential string) (runtimeservice.AgentNode, error) {
	return runtimeservice.AuthenticateNode(nodeID, credential)
}
