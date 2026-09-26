package service

import (
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/repository"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	runtimeservice "github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding/service"
)

type mysqlStore struct{}

func (mysqlStore) CreateTask(input repository.CreateTaskInput, now time.Time) (model.Task, error) {
	return repository.CreateTask(input, now)
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

// This module's dependency on the object store is not wired yet: the storage
// package and its configuration arrive with the Cloud preparation worker, and
// until then a claim has no grant to hand out and says so through
// `ErrGrantUnavailable` rather than leasing a task it cannot serve.
//
// The zero value is `unavailableGrants`, so there is nothing to wire here yet —
// naming the gap in one place is the point.
var _ GrantIssuer = unavailableGrants{}
