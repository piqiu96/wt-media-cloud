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
	return NewService(mysqlStore{}, runtimeNodeAuth{}, WithGrantIssuer(objectStorageGrants{}))
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

func LatestUserDownloadStatuses(userID identityservice.UserID, materialIDs []int64) (map[int64]string, error) {
	return newWiredService().LatestUserDownloadStatuses(userID, materialIDs)
}

func CancelTask(actor identityservice.PublicUser, taskID string) (dto.Task, error) {
	return newWiredService().CancelTask(actor, taskID)
}

func RetryTask(actor identityservice.PublicUser, taskID string) (dto.Task, error) {
	return newWiredService().RetryTask(actor, taskID)
}

func ClaimTask(ctx context.Context, credential string) (dto.ClaimResult, error) {
	return newWiredService().ClaimTask(ctx, credential)
}

func HeartbeatTask(credential, taskID string, completedBytes int64) error {
	return newWiredService().HeartbeatTask(credential, taskID, completedBytes)
}

func ReportProgress(credential, taskID string, completedBytes, bytesPerSecond int64) error {
	return newWiredService().ReportProgress(credential, taskID, completedBytes, bytesPerSecond)
}

func CompleteTask(credential, taskID string, body dto.CompletionRequest) (dto.TransferTerminal, error) {
	return newWiredService().CompleteTask(credential, taskID, body)
}
