package repository

import (
	identitymodel "github.com/wt-media/wt-media-cloud/internal/modules/identity/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/profileguard/model"
)

func authorizedTask() model.SensitiveTask {
	return model.SensitiveTask{
		ID: "task-1", UserID: identitymodel.UserID(1), ProfileID: "profile-1",
		BitProfileID: "bit-profile-1", NodeID: "node-1",
		Operation: model.OperationInteraction, Status: model.TaskAuthorized,
	}
}
