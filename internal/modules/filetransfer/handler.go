// Package filetransfer exposes the module's HTTP surface.
//
// Two surfaces, deliberately in one module: the session routes a browser calls,
// and the executor routes a Local Agent calls. They are separate routers and
// separate authentication — a session cookie against a node bearer credential —
// but they are the same lifecycle, and splitting them across modules would let
// the two drift into disagreeing about what a task's state means.
package filetransfer

import (
	"context"
	"errors"
	"strings"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
	transferservice "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/service"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

func actor(c *hertzapp.RequestContext) (identityservice.PublicUser, bool) {
	return middleware.AuthenticateRequest(c)
}

// ListTasks answers with the acting user's tasks, newest first.
func ListTasks(_ context.Context, c *hertzapp.RequestContext) {
	current, ok := actor(c)
	if !ok {
		return
	}
	items, err := transferservice.ListTasks(current)
	if err != nil {
		writeTransferError(c, err)
		return
	}
	api.Success(c, items)
}

// CancelTask asks for a transfer to stop. The body is the refreshed task, which
// may still be `running`: a running executor owns the transition to terminal.
func CancelTask(_ context.Context, c *hertzapp.RequestContext) {
	current, ok := actor(c)
	if !ok {
		return
	}
	task, err := transferservice.CancelTask(current, c.Param("task_id"))
	if err != nil {
		writeTransferError(c, err)
		return
	}
	api.Success(c, task)
}

// RetryTask requeues a failed transfer within its attempt bound.
func RetryTask(_ context.Context, c *hertzapp.RequestContext) {
	current, ok := actor(c)
	if !ok {
		return
	}
	task, err := transferservice.RetryTask(current, c.Param("task_id"))
	if err != nil {
		writeTransferError(c, err)
		return
	}
	api.Success(c, task)
}

// ClaimTask leases one task to the node the bearer credential identifies, or
// answers with a null task.
//
// The frozen route has no path parameter and no body: the credential is the only
// thing naming the caller, which is why the service resolves the node from it.
func ClaimTask(ctx context.Context, c *hertzapp.RequestContext) {
	credential, ok := bearerCredential(string(c.Request.Header.Peek("Authorization")))
	if !ok {
		writeTransferError(c, transferservice.ErrNodeUnauthenticated)
		return
	}
	result, err := transferservice.ClaimTask(ctx, credential)
	if err != nil {
		writeTransferError(c, err)
		return
	}
	api.Success(c, result)
}

// HeartbeatTask renews the lease. The frozen contract gives the response no
// body, and a renewed lease is implicit in the call succeeding.
func HeartbeatTask(_ context.Context, c *hertzapp.RequestContext) {
	credential, ok := bearerCredential(string(c.Request.Header.Peek("Authorization")))
	if !ok {
		writeTransferError(c, transferservice.ErrNodeUnauthenticated)
		return
	}
	var request transferservice.HeartbeatRequest
	if !api.DecodeJSON(c, &request) {
		return
	}
	if err := transferservice.HeartbeatTask(credential, c.Param("task_id"), request.CompletedBytes); err != nil {
		writeTransferError(c, err)
		return
	}
	api.NoContent(c)
}

// ReportProgress records how far the transfer has got.
func ReportProgress(_ context.Context, c *hertzapp.RequestContext) {
	credential, ok := bearerCredential(string(c.Request.Header.Peek("Authorization")))
	if !ok {
		writeTransferError(c, transferservice.ErrNodeUnauthenticated)
		return
	}
	var request transferservice.ProgressRequest
	if !api.DecodeJSON(c, &request) {
		return
	}
	if err := transferservice.ReportProgress(credential, c.Param("task_id"), request.CompletedBytes, request.BytesPerSecond); err != nil {
		writeTransferError(c, err)
		return
	}
	api.NoContent(c)
}

// CompleteTask records the terminal outcome and echoes what Cloud recorded.
func CompleteTask(_ context.Context, c *hertzapp.RequestContext) {
	credential, ok := bearerCredential(string(c.Request.Header.Peek("Authorization")))
	if !ok {
		writeTransferError(c, transferservice.ErrNodeUnauthenticated)
		return
	}
	var request transferservice.CompletionRequest
	if !api.DecodeJSON(c, &request) {
		return
	}
	terminal, err := transferservice.CompleteTask(credential, c.Param("task_id"), request)
	if err != nil {
		writeTransferError(c, err)
		return
	}
	api.Success(c, terminal)
}

// bearerCredential reads the node credential, and every executor handler asks for
// it before it parses the body: a caller that cannot be identified has no business
// learning whether its json was well formed, and the order is what makes the
// unauthenticated answer for all four executor routes the same 401.
//
// It is a per-module helper rather
// than a shared one because the modules that need it are exactly the modules
// whose routes are authenticated by a node rather than by a session, and each
// states that fact in its own handler file.
func bearerCredential(header string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	value := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	return value, value != ""
}

// writeTransferError maps the service's sentinels onto the frozen error names.
//
// The names travel in `error.type` rather than in the numeric errcode, because
// `contracts/cloud-error-codes/` publishes names and HTTP statuses and no
// numbers at all. A client that has to tell `transfer_task_conflict` from
// `transfer_task_cancelled` — both 409 — has no other field to read.
func writeTransferError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, transferservice.ErrInvalidInput):
		api.BadRequest(c, 10001, "文件传输请求格式错误")
	case errors.Is(err, transferservice.ErrNodeUnauthenticated):
		api.Unauthorized(c, 11001, "节点凭证无效")
	case errors.Is(err, transferservice.ErrTaskNotFound):
		api.FailureNamed(c, consts.StatusNotFound, 15101, "文件传输任务不存在", "transfer_task_not_found")
	case errors.Is(err, transferservice.ErrTaskForbidden):
		api.FailureNamed(c, consts.StatusForbidden, 15102, "没有操作该文件传输任务的权限", "transfer_task_forbidden")
	case errors.Is(err, transferservice.ErrTaskConflict):
		api.ConflictNamed(c, 15103, "文件传输任务当前状态不允许该操作", "transfer_task_conflict")
	case errors.Is(err, transferservice.ErrTaskCancelled):
		api.ConflictNamed(c, 15104, "文件传输任务已取消或已结束", "transfer_task_cancelled")
	case errors.Is(err, transferservice.ErrIntegrityFailed):
		api.UnprocessableEntity(c, 15106, "传输结果与任务声明不一致", "transfer_integrity_failed")
	default:
		api.InternalError(c, "文件传输服务内部错误")
	}
}
