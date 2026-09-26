package service

import (
	"context"

	"github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/dto"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
)

// newWiredService assembles the module's real dependencies, in one place.
//
// The grant issuer is deliberately not among them: the object store and its
// configuration arrive with the Cloud preparation worker, and until then a claim
// has no grant to hand out. It refuses through `ErrGrantUnavailable` rather than
// leasing a task it cannot serve, which is the honest answer — an executor that
// received a lease with a broken url could do nothing with it and no one could
// tell why. Wiring the real issuer is the one line this function is shaped to
// take.
func newWiredService() *Service {
	return NewService(mysqlStore{}, runtimeNodeAuth{})
}

func CreateTask(input CreateTaskInput) (dto.Task, error) {
	return newWiredService().CreateTask(input)
}

func CreateUserDownload(input CreateUserDownloadInput) (dto.Task, error) {
	return newWiredService().CreateUserDownload(input)
}

func ListTasks(actor identityservice.PublicUser) ([]dto.Task, error) {
	return newWiredService().ListTasks(actor)
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
