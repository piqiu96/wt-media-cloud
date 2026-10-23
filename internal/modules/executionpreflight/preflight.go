// Package executionpreflight 是所有“本机能不能执行这个动作”判断的唯一入口：
// 普通任务、敏感任务和 Desktop/Web 触发的新操作复用同一组检查与错误码。
package executionpreflight

import (
	"strings"
	"time"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

type Mode int

const (
	// ModeNewOperation 是新操作准入：要求 Agent Token 绑定的 Desktop 会话仍有效。
	ModeNewOperation Mode = iota
	// ModeClaimedTask 是已领取任务的执行准入：claim 时的严格档已是准入证据，
	// 不再要求当前仍有 Desktop 会话，否则抢占会中断已在运行的任务。
	ModeClaimedTask
)

type Action string

const (
	ActionAccountCheck        Action = "account_check"
	ActionCookieRead          Action = "cookie_read"
	ActionCookieWrite         Action = "cookie_write"
	ActionProfileCreate       Action = "profile_create"
	ActionProfileOpen         Action = "profile_open"
	ActionProfileClose        Action = "profile_close"
	ActionProfileUpdate       Action = "profile_update"
	ActionProxyCheck          Action = "proxy_check"
	ActionProxyMutation       Action = "proxy_mutation"
	ActionAssistedPublication Action = "assisted_publication"
	ActionInteraction         Action = "interaction"
)

type actionSpec struct {
	needsProfile       bool
	needsPlatformLogin bool
	needsLocalFiles    bool
}

var actionSpecs = map[Action]actionSpec{
	ActionAccountCheck:        {needsProfile: true},
	ActionCookieRead:          {needsProfile: true},
	ActionCookieWrite:         {needsProfile: true},
	ActionProfileCreate:       {},
	ActionProfileOpen:         {needsProfile: true},
	ActionProfileClose:        {needsProfile: true},
	ActionProfileUpdate:       {needsProfile: true},
	ActionProxyCheck:          {},
	ActionProxyMutation:       {needsProfile: true},
	ActionAssistedPublication: {needsProfile: true, needsPlatformLogin: true, needsLocalFiles: true},
	ActionInteraction:         {needsProfile: true, needsPlatformLogin: true, needsLocalFiles: true},
}

var sensitiveOperationActions = map[string]Action{
	"assisted_publication":        ActionAssistedPublication,
	"interaction":                 ActionInteraction,
	"authenticated_account_check": ActionAccountCheck,
	"cookie_read":                 ActionCookieRead,
	"cookie_write":                ActionCookieWrite,
	"profile_mutation":            ActionProfileUpdate,
	"proxy_mutation":              ActionProxyMutation,
}

// ActionForSensitiveOperation 把敏感任务的操作名映射成动作；未知操作返回 false。
func ActionForSensitiveOperation(operation string) (Action, bool) {
	action, ok := sensitiveOperationActions[operation]
	return action, ok
}

type ProfileFacts struct {
	Found               bool
	OwnedByUser         bool
	AssignedToOtherUser bool
	MainUserID          string
	LocalStatus         string
}

type MediaAccountFacts struct {
	Found       bool
	LoginStatus string
}

type CloudFacts struct {
	UserEnabled         bool
	DesktopSessionValid bool
	BitMainUserID       string
	Profile             ProfileFacts
	MediaAccount        MediaAccountFacts
}

type AgentFacts struct {
	BitBrowserRunning bool
	MainUserID        string
	ObservedAt        time.Time
	ProfileVisible    bool
	FilesReady        bool
}

type Request struct {
	Mode             Mode
	Action           Action
	UserID           int64
	ProfileID        string
	MediaAccountID   string
	Cloud            CloudFacts
	Agent            AgentFacts
	Now              time.Time
	ObservedFreshFor time.Duration
}

// Failure 是准入失败结果，也是唯一会产生这些错误码的地方。
type Failure struct {
	Code           string `json:"code"`
	Message        string `json:"message"`
	Retryable      bool   `json:"retryable"`
	RecoveryAction string `json:"recovery_action"`
}

func (f *Failure) Error() string { return f.Code + ": " + f.Message }

// Evaluate 按固定顺序检查；未知动作失败关闭。
func Evaluate(req Request) *Failure {
	spec, known := actionSpecs[req.Action]
	if !known {
		return &Failure{Code: "UNSUPPORTED_TASK_TYPE", Message: "不支持的任务动作", RecoveryAction: "none"}
	}
	if req.Mode == ModeNewOperation {
		if !req.Cloud.UserEnabled {
			return &Failure{Code: "AUTH_SESSION_INVALIDATED", Message: "登录状态已失效，请重新登录", RecoveryAction: "login_again"}
		}
		if !req.Cloud.DesktopSessionValid {
			return &Failure{Code: "DESKTOP_LOGIN_REQUIRED", Message: "请先在 Desktop 登录", RecoveryAction: "login_desktop"}
		}
	}
	boundMain := strings.TrimSpace(req.Cloud.BitMainUserID)
	if boundMain == "" {
		return &Failure{Code: "BIT_NOT_BOUND", Message: "尚未绑定 BitBrowser 主账号", RecoveryAction: "bind_bitbrowser"}
	}
	if !req.Agent.BitBrowserRunning || !observationFresh(req.Agent.ObservedAt, req.Now, req.ObservedFreshFor) {
		return &Failure{Code: "BIT_BROWSER_OFFLINE", Message: "BitBrowser 未运行或观察已过期", Retryable: true, RecoveryAction: "start_bitbrowser"}
	}
	if strings.TrimSpace(req.Agent.MainUserID) != boundMain {
		return &Failure{Code: "BIT_MAIN_USER_MISMATCH", Message: "当前比特浏览器登录账号与系统绑定账号不一致", RecoveryAction: "rebind_bitbrowser"}
	}
	if spec.needsProfile {
		switch profile := req.Cloud.Profile; {
		case profile.AssignedToOtherUser:
			return &Failure{Code: "PROFILE_ASSIGNED_TO_OTHER_USER", Message: "浏览器环境属于其他用户", RecoveryAction: "choose_own_profile"}
		case !profile.Found || !profile.OwnedByUser:
			return &Failure{Code: "PROFILE_NOT_FOUND", Message: "未找到对应的浏览器环境", RecoveryAction: "choose_profile"}
		case profile.MainUserID != boundMain:
			return &Failure{Code: "PROFILE_NOT_FOUND", Message: "未找到对应的浏览器环境", RecoveryAction: "choose_profile"}
		case profile.LocalStatus != "active" || !req.Agent.ProfileVisible:
			return &Failure{Code: "PROFILE_NOT_FOUND", Message: "未找到对应的浏览器环境", RecoveryAction: "choose_profile"}
		}
	}
	if spec.needsPlatformLogin && (!req.Cloud.MediaAccount.Found || req.Cloud.MediaAccount.LoginStatus != "normal") {
		return &Failure{Code: "PROFILE_LOGIN_INVALID", Message: "平台登录状态不满足该操作", RecoveryAction: "login_platform"}
	}
	if spec.needsLocalFiles && !req.Agent.FilesReady {
		return &Failure{Code: "FILE_PREPARE_FAILED", Message: "本地文件准备失败", Retryable: true, RecoveryAction: "retry_prepare"}
	}
	return nil
}

const defaultObservedFreshFor = 120 * time.Second

var processObservedFreshFor = defaultObservedFreshFor

// ConfigureObservedFreshFor 安装 Agent 观察的新鲜窗口；进程启动时由 bootstrap 调用。
func ConfigureObservedFreshFor(freshFor time.Duration) {
	if freshFor > 0 {
		processObservedFreshFor = freshFor
	}
}

func ObservedFreshFor() time.Duration { return processObservedFreshFor }

func observationFresh(at, now time.Time, freshFor time.Duration) bool {
	if at.IsZero() || freshFor <= 0 {
		return false
	}
	if at.After(now) {
		return true
	}
	return now.Sub(at) <= freshFor
}

// WriteFailure 把准入失败写成统一的 error envelope。
func WriteFailure(c *hertzapp.RequestContext, failure *Failure) {
	status, errcode := failureStatus(failure.Code)
	api.FailureCode(c, status, errcode, failure.Message, failure.Code, nil, failure.Retryable, failure.RecoveryAction)
}

func failureStatus(code string) (int, int) {
	switch code {
	case "AUTH_DESKTOP_REPLACED", "AUTH_SESSION_INVALIDATED", "AUTH_SESSION_INVALID", "AUTH_SESSION_EXPIRED",
		"DESKTOP_LOGIN_REQUIRED", "AUTH_AGENT_TOKEN_INVALID", "AUTH_AGENT_TOKEN_EXPIRED":
		return 401, 11001
	case "AGENT_SCOPE_FORBIDDEN", "PROFILE_ASSIGNED_TO_OTHER_USER":
		return 403, 11003
	case "PROFILE_NOT_FOUND":
		return 404, 20004
	case "STALE_ATTEMPT":
		return 409, 30009
	case "BIT_MAIN_USER_MISMATCH":
		return 409, 23002
	case "BIT_NOT_BOUND":
		return 409, 23008
	case "BIT_BROWSER_OFFLINE":
		return 409, 23009
	case "PROFILE_LOGIN_INVALID":
		return 409, 23010
	case "FILE_PREPARE_FAILED":
		return 422, 23011
	case "UNSUPPORTED_TASK_TYPE":
		return 400, 10002
	default:
		return 500, 50000
	}
}
