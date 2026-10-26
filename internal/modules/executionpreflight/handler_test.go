package executionpreflight

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	cloudmodel "github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/localagent"
	"github.com/wt-media/wt-media-cloud/internal/shared/agenttoken"
)

const testTokenSecret = "test-agent-token-secret-with-at-least-32-characters"

func issueTestAgentToken(t *testing.T) string {
	t.Helper()
	signer, err := agenttoken.NewSigner([]byte(testTokenSecret))
	if err != nil {
		t.Fatalf("NewSigner() error = %v", err)
	}
	agenttoken.Configure(signer)
	t.Cleanup(func() { agenttoken.Configure(nil) })
	token, _, err := signer.Issue(7, "session-1", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	return token
}

func stubDesktopCheck(t *testing.T, failure *localagent.SessionFailure) *int {
	t.Helper()
	calls := 0
	original := checkDesktopSession
	checkDesktopSession = func(agenttoken.Claims) (localagent.AgentAuth, *localagent.SessionFailure, error) {
		calls++
		return localagent.AgentAuth{}, failure, nil
	}
	t.Cleanup(func() { checkDesktopSession = original })
	return &calls
}

func stubGetTask(t *testing.T, task cloudmodel.Task, err error) {
	t.Helper()
	original := getTask
	getTask = func(string, int64) (cloudmodel.Task, error) { return task, err }
	t.Cleanup(func() { getTask = original })
}

func stubCloudFacts(t *testing.T, facts CloudFacts, err error) *int {
	t.Helper()
	calls := 0
	original := loadCloudFacts
	loadCloudFacts = func(int64, string, string) (CloudFacts, error) {
		calls++
		return facts, err
	}
	t.Cleanup(func() { loadCloudFacts = original })
	return &calls
}

func admittedCloudFacts() CloudFacts {
	return CloudFacts{
		UserEnabled:   true,
		BitMainUserID: "main-1",
		Profile:       ProfileFacts{Found: true, OwnedByUser: true, MainUserID: "main-1", LocalStatus: "active"},
		MediaAccount:  MediaAccountFacts{Found: true, LoginStatus: "normal"},
	}
}

func liveFacts(observedAt time.Time) FactsInput {
	return FactsInput{BitBrowserStatus: "running", MainUserID: "main-1", ObservedAt: observedAt, ProfileVisible: true, FilesReady: true}
}

type preflightRequest struct {
	Action         string     `json:"action"`
	TaskID         string     `json:"task_id,omitempty"`
	AttemptID      string     `json:"attempt_id,omitempty"`
	ProfileID      string     `json:"profile_id,omitempty"`
	MediaAccountID string     `json:"media_account_id,omitempty"`
	Facts          FactsInput `json:"facts"`
}

type probeResult struct {
	status         int
	code           string
	retryable      bool
	recoveryAction string
	dataStatus     string
}

func runPreflight(t *testing.T, token string, payload any) probeResult {
	t.Helper()
	engine := server.New()
	RegisterRoutes(engine)
	var requestBody *ut.Body
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		requestBody = &ut.Body{Body: strings.NewReader(string(encoded)), Len: len(encoded)}
	}
	var headers []ut.Header
	if token != "" {
		headers = append(headers, ut.Header{Key: "Authorization", Value: "Bearer " + token})
	}
	result := ut.PerformRequest(engine.Engine, consts.MethodPost, "/api/v1/local-agent/execution-preflight", requestBody, headers...)
	var envelope struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
		Error struct {
			Code           string `json:"code"`
			Retryable      bool   `json:"retryable"`
			RecoveryAction string `json:"recovery_action"`
		} `json:"error"`
	}
	if err := json.Unmarshal(result.Result().Body(), &envelope); err != nil {
		t.Fatalf("decode response %s: %v", result.Result().Body(), err)
	}
	return probeResult{
		status: result.Result().StatusCode(), code: envelope.Error.Code,
		retryable: envelope.Error.Retryable, recoveryAction: envelope.Error.RecoveryAction,
		dataStatus: envelope.Data.Status,
	}
}

func TestExecutionPreflightRequiresAnAgentToken(t *testing.T) {
	result := runPreflight(t, "", preflightRequest{Action: "interaction"})
	if result.status != consts.StatusUnauthorized || result.code != "AUTH_AGENT_TOKEN_INVALID" {
		t.Fatalf("status=%d code=%q", result.status, result.code)
	}
}

// 会话检查在读取任何业务事实之前：被替换的 Desktop 不会再触发一次数据库查询。
func TestNewOperationStopsBeforeFactsWhenTheDesktopSessionWasReplaced(t *testing.T) {
	token := issueTestAgentToken(t)
	stubDesktopCheck(t, &localagent.SessionFailure{Code: "AUTH_DESKTOP_REPLACED", Message: "你的账号已在另一台电脑登录", RecoveryAction: "login_again"})
	factsCalls := stubCloudFacts(t, admittedCloudFacts(), nil)

	result := runPreflight(t, token, preflightRequest{Action: "interaction", ProfileID: "11", MediaAccountID: "21", Facts: liveFacts(time.Now())})
	if result.status != consts.StatusUnauthorized || result.code != "AUTH_DESKTOP_REPLACED" {
		t.Fatalf("status=%d code=%q", result.status, result.code)
	}
	if *factsCalls != 0 {
		t.Fatalf("facts were loaded %d time(s) for a replaced Desktop", *factsCalls)
	}
}

func TestNewOperationAdmitsAHealthyInteraction(t *testing.T) {
	token := issueTestAgentToken(t)
	stubDesktopCheck(t, nil)
	stubCloudFacts(t, admittedCloudFacts(), nil)

	result := runPreflight(t, token, preflightRequest{Action: "interaction", ProfileID: "11", MediaAccountID: "21", Facts: liveFacts(time.Now())})
	if result.status != consts.StatusOK || result.dataStatus != "ok" {
		t.Fatalf("status=%d data=%q code=%q", result.status, result.dataStatus, result.code)
	}
}

// 已领取任务走上报档：Desktop 被替换后，它的会话检查根本不该发生。
func TestClaimedTaskSkipsTheDesktopSessionGate(t *testing.T) {
	token := issueTestAgentToken(t)
	desktopCalls := stubDesktopCheck(t, nil)
	stubGetTask(t, cloudmodel.Task{AttemptID: "attempt-1", Status: "running"}, nil)
	stubCloudFacts(t, admittedCloudFacts(), nil)

	result := runPreflight(t, token, preflightRequest{
		Action: "interaction", TaskID: "task-1", AttemptID: "attempt-1",
		ProfileID: "11", MediaAccountID: "21", Facts: liveFacts(time.Now()),
	})
	if result.status != consts.StatusOK || result.dataStatus != "ok" {
		t.Fatalf("status=%d data=%q code=%q", result.status, result.dataStatus, result.code)
	}
	if *desktopCalls != 0 {
		t.Fatalf("claimed-task preflight consulted the session %d time(s)", *desktopCalls)
	}
}

func TestClaimedTaskIsStaleWhenTheAttemptOrTaskMovedOn(t *testing.T) {
	cases := []struct {
		name string
		task cloudmodel.Task
		err  error
	}{
		{"newer attempt", cloudmodel.Task{AttemptID: "attempt-2", Status: "running"}, nil},
		{"task already finished", cloudmodel.Task{AttemptID: "attempt-1", Status: "succeeded"}, nil},
		{"task of another user", cloudmodel.Task{}, cloudmodel.ErrTaskNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			token := issueTestAgentToken(t)
			stubDesktopCheck(t, nil)
			stubGetTask(t, tc.task, tc.err)
			factsCalls := stubCloudFacts(t, admittedCloudFacts(), nil)

			result := runPreflight(t, token, preflightRequest{
				Action: "interaction", TaskID: "task-1", AttemptID: "attempt-1",
				ProfileID: "11", MediaAccountID: "21", Facts: liveFacts(time.Now()),
			})
			if result.status != consts.StatusConflict || result.code != "STALE_ATTEMPT" || result.recoveryAction != "none" {
				t.Fatalf("status=%d code=%q recovery=%q", result.status, result.code, result.recoveryAction)
			}
			if *factsCalls != 0 {
				t.Fatalf("a stale attempt still loaded facts %d time(s)", *factsCalls)
			}
		})
	}
}

func TestEvaluatorFailureKeepsItsRecoveryInTheEnvelope(t *testing.T) {
	token := issueTestAgentToken(t)
	stubDesktopCheck(t, nil)
	stubCloudFacts(t, admittedCloudFacts(), nil)
	facts := liveFacts(time.Now())
	facts.FilesReady = false

	result := runPreflight(t, token, preflightRequest{Action: "interaction", ProfileID: "11", MediaAccountID: "21", Facts: facts})
	if result.status != consts.StatusUnprocessableEntity || result.code != "FILE_PREPARE_FAILED" {
		t.Fatalf("status=%d code=%q", result.status, result.code)
	}
	if !result.retryable || result.recoveryAction != "retry_prepare" {
		t.Fatalf("retryable=%t recovery=%q", result.retryable, result.recoveryAction)
	}
}

func TestFactsLoadFailureIsAnInternalError(t *testing.T) {
	token := issueTestAgentToken(t)
	stubDesktopCheck(t, nil)
	stubCloudFacts(t, CloudFacts{}, errors.New("database down"))

	result := runPreflight(t, token, preflightRequest{Action: "interaction", ProfileID: "11", MediaAccountID: "21", Facts: liveFacts(time.Now())})
	if result.status != consts.StatusInternalServerError {
		t.Fatalf("status=%d code=%q", result.status, result.code)
	}
}

func TestExecutionPreflightRejectsAnEmptyBody(t *testing.T) {
	token := issueTestAgentToken(t)
	result := runPreflight(t, token, nil)
	if result.status != consts.StatusBadRequest {
		t.Fatalf("status=%d code=%q", result.status, result.code)
	}
}
