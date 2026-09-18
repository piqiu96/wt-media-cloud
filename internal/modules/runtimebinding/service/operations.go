package service

import (
	"github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding/model"
)

func IssueTicket(actor service.PublicUser, sessionID string) (dto.BindingTicketGrant, error) {
	return NewService(mysqlStore{}).IssueTicket(actor, sessionID)
}

func RegisterLocal(input dto.RegisterLocalInput) (dto.Registration, error) {
	return NewService(mysqlStore{}).RegisterLocal(input)
}

func CheckLocalTrust(userID service.UserID, nodeID string) error {
	return NewService(mysqlStore{}).CheckLocalTrust(userID, nodeID)
}

func ReportRuntime(nodeID, credential string, report dto.RuntimeReport) error {
	return NewService(mysqlStore{}).ReportRuntime(nodeID, credential, report)
}

func AuthenticateNode(nodeID, credential string) (model.AgentNode, error) {
	return NewService(mysqlStore{}).AuthenticateNode(nodeID, credential)
}
