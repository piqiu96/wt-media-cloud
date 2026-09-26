package service

import (
	"context"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/storage"
	"github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/repository"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	runtimeservice "github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding/service"
)

type mysqlStore struct{}

func (mysqlStore) CreateTask(input repository.CreateTaskInput, now time.Time) (model.Task, error) {
	return repository.CreateTask(input, now)
}
func (mysqlStore) CreateUserDownloadTask(input repository.CreateUserDownloadInput, now time.Time) (model.Task, error) {
	return repository.CreateUserDownloadTask(input, now)
}
func (mysqlStore) CreateMaterialSourcePrepareTask(input repository.CreateMaterialSourcePrepareInput, now time.Time) (model.Task, error) {
	return repository.CreateMaterialSourcePrepareTask(input, now)
}
func (mysqlStore) GetTask(taskID string) (model.Task, error) { return repository.GetTask(taskID) }
func (mysqlStore) ListTasks(filter repository.TaskFilter) ([]model.Task, error) {
	return repository.ListTasks(filter)
}
func (mysqlStore) NextLocalTask(nodeID string, now time.Time) (model.Task, bool, error) {
	return repository.NextLocalTask(nodeID, now)
}
func (mysqlStore) ClaimLocalTask(taskID, nodeID string, now time.Time, lease time.Duration) (model.Task, bool, error) {
	return repository.ClaimLocalTask(taskID, nodeID, now, lease)
}
func (mysqlStore) HeartbeatTask(taskID, nodeID string, now time.Time, lease time.Duration) (bool, error) {
	return repository.HeartbeatTask(taskID, nodeID, now, lease)
}
func (mysqlStore) ReportProgress(input repository.ProgressInput, now time.Time) (bool, error) {
	return repository.ReportProgress(input, now)
}
func (mysqlStore) CompleteTask(input repository.CompletionInput, now time.Time) (bool, error) {
	return repository.CompleteTask(input, now)
}
func (mysqlStore) FailTask(input repository.FailureInput, now time.Time) (bool, error) {
	return repository.FailTask(input, now)
}
func (mysqlStore) CancelTask(taskID string, teamID identityservice.TeamID, requestedBy identityservice.UserID, now time.Time) (bool, error) {
	return repository.CancelTask(taskID, teamID, requestedBy, now)
}
func (mysqlStore) RetryTask(taskID string, teamID identityservice.TeamID, requestedBy identityservice.UserID, now time.Time) (model.Task, bool, error) {
	return repository.RetryTask(taskID, teamID, requestedBy, now)
}

// runtimeNodeAuth is the whole of this module's dependency on the
// runtime-binding domain. It is a type rather than a call inside the service so
// that a test can answer the credential question without a node table, and so
// that the one place a real node lookup happens is visible.
type runtimeNodeAuth struct{}

func (runtimeNodeAuth) AuthenticateNodeCredential(credential string) (runtimeservice.AgentNode, error) {
	return runtimeservice.AuthenticateNodeCredential(credential)
}

// objectStorageGrants is the whole of this module's dependency on the object
// store: it mints the grant a lease carries, and it is a type rather than a call
// inside the service so that the dependency is visible in one file and a test can
// answer without a bucket.
//
// It adapts rather than implements: `GrantIssuer.PresignGet` takes no lifetime,
// while `storage.Store.PresignGet` requires one. The store cannot simply be the
// issuer, and that is the point — a caller inside this module must not be able to
// choose how long an address stays valid, so the adapter supplies the configured
// lifetime and the module has no way to name another.
type objectStorageGrants struct{}

func (objectStorageGrants) PresignGet(ctx context.Context, objectKey string) (DownloadGrant, error) {
	grant, err := storage.PresignGet(ctx, objectKey)
	if err != nil {
		// `ErrNotConfigured` arrives here unchanged: a claim on a deployment with
		// no object-storage credential is an internal fault, and the frozen error
		// contract publishes no name for it. Nothing is leased, so the task keeps
		// its attempts and stays claimable once the credential is supplied.
		return DownloadGrant{}, err
	}
	return DownloadGrant{URL: grant.URL, ExpiresAt: grant.ExpiresAt}, nil
}

var (
	_ GrantIssuer = unavailableGrants{}
	_ GrantIssuer = objectStorageGrants{}
)
