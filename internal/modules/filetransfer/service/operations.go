package service

import (
	"context"

	"github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/dto"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
)

// newWiredService assembles the module's real dependencies, in one place.
//
// The grant issuer is wired here rather than defaulted, so that a service built
// by hand — a test's, or a future caller's — keeps the refusing zero value and
// cannot lease a task it has no way to serve. Only this path, the one the
// handlers use and Bootstrap has therefore prepared, mints real grants.
func newWiredService() *Service {
	return NewService(mysqlStore{}, WithGrantIssuer(objectStorageGrants{}))
}

func CreateTask(input CreateTaskInput) (dto.Task, error) {
	return newWiredService().CreateTask(input)
}

func CreateUserDownload(input CreateUserDownloadInput) (dto.Task, error) {
	return newWiredService().CreateUserDownload(input)
}

func EnsureMaterialSourcePrepare(input EnsureMaterialSourcePrepareInput) (dto.Task, error) {
	return newWiredService().EnsureMaterialSourcePrepare(input)
}

func ListTasks(actor identityservice.PublicUser, opts TaskListOptions) ([]dto.Task, error) {
	return newWiredService().ListTasks(actor, opts)
}

func HideDownloadHistory(actor identityservice.PublicUser, taskID string) error {
	return newWiredService().HideDownloadHistory(actor, taskID)
}

func LatestUserDownloadStatuses(userID identityservice.UserID, materialIDs []int64) (map[int64]string, error) {
	return newWiredService().LatestUserDownloadStatuses(userID, materialIDs)
}

func AttachCompletedDependency(taskID string) (dto.Task, bool, error) {
	return newWiredService().AttachCompletedDependency(taskID)
}

func CancelTask(actor identityservice.PublicUser, taskID string) (dto.Task, error) {
	return newWiredService().CancelTask(actor, taskID)
}

func ClaimTask(ctx context.Context, userID int64) (dto.ClaimResult, error) {
	return newWiredService().ClaimTask(ctx, userID)
}

func HeartbeatTask(userID int64, taskID, attemptID string, completedBytes int64) error {
	return newWiredService().HeartbeatTask(userID, taskID, attemptID, completedBytes)
}

func ReportProgress(userID int64, taskID, attemptID string, completedBytes, bytesPerSecond int64) error {
	return newWiredService().ReportProgress(userID, taskID, attemptID, completedBytes, bytesPerSecond)
}

func CompleteTask(userID int64, taskID, attemptID string, body dto.CompletionRequest) (dto.TransferTerminal, error) {
	return newWiredService().CompleteTask(userID, taskID, attemptID, body)
}
