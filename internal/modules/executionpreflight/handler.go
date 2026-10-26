package executionpreflight

import (
	"context"
	"errors"
	"strings"
	"time"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	cloudmodel "github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/model"
	cloudagentservice "github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/localagent"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

// 这三个查询在 handler 测试里替换掉，以覆盖准入分支而不依赖数据库。
var (
	checkDesktopSession = localagent.CheckDesktopSession
	getTask             = cloudagentservice.GetTask
	loadCloudFacts      = LoadCloudFacts
)

// ExecutionPreflight 是普通任务与 Desktop/Web 新操作的统一准入入口。
// 带 attempt_id 的请求按已领取任务处理，否则要求当前 Desktop 会话仍有效。
func ExecutionPreflight(_ context.Context, c *hertzapp.RequestContext) {
	auth, ok := localagent.AgentAuthFrom(c)
	if !ok {
		api.FailureCode(c, 401, 11001, "Agent 凭证无效", "AUTH_AGENT_TOKEN_INVALID", nil, false, "reissue_agent_token")
		return
	}
	var body PreflightRequest
	if !api.DecodeJSON(c, &body) {
		return
	}
	request := Request{
		Action:           Action(strings.TrimSpace(body.Action)),
		UserID:           auth.Claims.UserID,
		ProfileID:        strings.TrimSpace(body.ProfileID),
		MediaAccountID:   strings.TrimSpace(body.MediaAccountID),
		Agent:            body.Facts.AgentFacts(),
		Now:              time.Now().UTC(),
		ObservedFreshFor: ObservedFreshFor(),
	}
	if attemptID := strings.TrimSpace(body.AttemptID); attemptID != "" {
		request.Mode = ModeClaimedTask
		failure, err := checkClaimedAttempt(auth.Claims.UserID, strings.TrimSpace(body.TaskID), attemptID)
		if err != nil {
			api.InternalError(c, "任务执行准入校验失败")
			return
		}
		if failure != nil {
			WriteFailure(c, failure)
			return
		}
	} else {
		request.Mode = ModeNewOperation
		_, failure, err := checkDesktopSession(auth.Claims)
		if err != nil {
			api.InternalError(c, "Agent 凭证校验失败")
			return
		}
		if failure != nil {
			api.FailureCode(c, 401, 11001, failure.Message, failure.Code, nil, false, failure.RecoveryAction)
			return
		}
		request.Cloud.DesktopSessionValid = true
	}

	cloudFacts, err := loadCloudFacts(request.UserID, request.ProfileID, request.MediaAccountID)
	if err != nil {
		api.InternalError(c, "执行准入查询失败")
		return
	}
	sessionValid := request.Cloud.DesktopSessionValid
	request.Cloud = cloudFacts
	request.Cloud.DesktopSessionValid = sessionValid
	if failure := Evaluate(request); failure != nil {
		WriteFailure(c, failure)
		return
	}
	api.Success(c, map[string]string{"status": "ok"})
}

func checkClaimedAttempt(userID int64, taskID, attemptID string) (*Failure, error) {
	if taskID == "" || attemptID == "" {
		return staleAttempt(), nil
	}
	task, err := getTask(taskID, userID)
	if errors.Is(err, cloudmodel.ErrTaskNotFound) {
		return staleAttempt(), nil
	}
	if err != nil {
		return nil, err
	}
	// 终态任务的 attempt 也不再执行；旧 attempt 不能借此前置检查重新开跑。
	if task.AttemptID != attemptID || (task.Status != "leased" && task.Status != "running") {
		return staleAttempt(), nil
	}
	return nil, nil
}

func staleAttempt() *Failure {
	return &Failure{Code: "STALE_ATTEMPT", Message: "任务 attempt 已失效", RecoveryAction: "none"}
}
