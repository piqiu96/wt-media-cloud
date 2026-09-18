package service

import (
	"time"

	identitymodel "github.com/wt-media/wt-media-cloud/internal/modules/identity/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding/repository"
)

type mysqlStore struct{}

func (mysqlStore) CreateTicket(ticket model.BindingTicket) error {
	return repository.CreateTicket(ticket)
}
func (mysqlStore) ConsumeTicket(tokenHash string, at time.Time) (model.BindingTicket, bool, error) {
	return repository.ConsumeTicket(tokenHash, at)
}
func (mysqlStore) IsSessionActive(sessionID string, userID identitymodel.UserID, at time.Time) (bool, error) {
	return repository.IsSessionActive(sessionID, userID, at)
}
func (mysqlStore) SaveNode(node model.AgentNode) error { return repository.SaveNode(node) }
func (mysqlStore) FindNodeByCredentialHash(hash string) (model.AgentNode, bool, error) {
	return repository.FindNodeByCredentialHash(hash)
}
func (mysqlStore) CheckLocalTrust(userID identitymodel.UserID, nodeID string, at time.Time, freshness time.Duration) (bool, error) {
	return repository.CheckLocalTrust(userID, nodeID, at, freshness)
}
func (mysqlStore) ValidateRuntimeProfiles(userID identitymodel.UserID, mainUserID string, profileIDs []string) (bool, error) {
	return repository.ValidateRuntimeProfiles(userID, mainUserID, profileIDs)
}
func (mysqlStore) ApplyRuntimeReport(node model.AgentNode, report dto.RuntimeReport, at time.Time) error {
	return repository.ApplyRuntimeReport(node, report, at)
}
