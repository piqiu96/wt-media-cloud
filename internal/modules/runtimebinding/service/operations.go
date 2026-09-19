package service

import (
	"github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding/model"
)

func IssueTicket(actor service.PublicUser, sessionID string) (dto.BindingTicketGrant, error) {
	return newService(mysqlStore{}).IssueTicket(actor, sessionID)
}

func RegisterLocal(input dto.RegisterLocalInput) (dto.Registration, error) {
	return newService(mysqlStore{}).RegisterLocal(input)
}

func CheckLocalTrust(userID service.UserID, nodeID string) error {
	return newService(mysqlStore{}).CheckLocalTrust(userID, nodeID)
}

func ReportRuntime(nodeID, credential string, report dto.RuntimeReport) error {
	return newService(mysqlStore{}).ReportRuntime(nodeID, credential, report)
}

func AuthenticateNode(nodeID, credential string) (model.AgentNode, error) {
	return newService(mysqlStore{}).AuthenticateNode(nodeID, credential)
}
