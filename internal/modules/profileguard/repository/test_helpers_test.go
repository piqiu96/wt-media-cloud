package repository

import (
	"github.com/wt-media/wt-media-cloud/internal/modules/profileguard/model"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

func authorizedTask() model.SensitiveTask {
	return model.SensitiveTask{
		ID: "task-1", UserID: sharedidentity.UserID(1), ProfileID: "profile-1",
		BitProfileID: "bit-profile-1", NodeID: "node-1",
		Operation: model.OperationInteraction, Status: model.TaskAuthorized,
	}
}
