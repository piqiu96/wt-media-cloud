package executionpreflight

import (
	"testing"
	"time"
)

var testNow = time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC)

func greenRequest() Request {
	return Request{
		Mode:           ModeNewOperation,
		Action:         ActionInteraction,
		UserID:         7,
		ProfileID:      "11",
		MediaAccountID: "21",
		Cloud: CloudFacts{
			UserEnabled:         true,
			DesktopSessionValid: true,
			BitMainUserID:       "main-1",
			Profile:             ProfileFacts{Found: true, OwnedByUser: true, MainUserID: "main-1", LocalStatus: "active"},
			MediaAccount:        MediaAccountFacts{Found: true, LoginStatus: "normal"},
		},
		Agent:            AgentFacts{BitBrowserRunning: true, MainUserID: "main-1", ObservedAt: testNow, ProfileVisible: true, FilesReady: true},
		Now:              testNow,
		ObservedFreshFor: 120 * time.Second,
	}
}

// 基线同时满足所有检查：任何一个变异都只关掉一项，失败码才说明是哪一项。
func TestEvaluateAdmitsAHealthyInteraction(t *testing.T) {
	if failure := Evaluate(greenRequest()); failure != nil {
		t.Fatalf("Evaluate() = %+v, want nil", failure)
	}
}

func TestEvaluateFailsClosedOnEveryCheck(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Request)
		code   string
	}{
		{"user disabled", func(r *Request) { r.Cloud.UserEnabled = false }, "AUTH_SESSION_INVALIDATED"},
		{"desktop session replaced", func(r *Request) { r.Cloud.DesktopSessionValid = false }, "DESKTOP_LOGIN_REQUIRED"},
		{"bit account unbound", func(r *Request) { r.Cloud.BitMainUserID = "" }, "BIT_NOT_BOUND"},
		{"bitbrowser stopped", func(r *Request) { r.Agent.BitBrowserRunning = false }, "BIT_BROWSER_OFFLINE"},
		{"observation stale", func(r *Request) { r.Agent.ObservedAt = testNow.Add(-121 * time.Second) }, "BIT_BROWSER_OFFLINE"},
		{"observation missing", func(r *Request) { r.Agent.ObservedAt = time.Time{} }, "BIT_BROWSER_OFFLINE"},
		{"observed main differs", func(r *Request) { r.Agent.MainUserID = "main-2" }, "BIT_MAIN_USER_MISMATCH"},
		{"profile belongs to another user", func(r *Request) {
			r.Cloud.Profile = ProfileFacts{Found: true, AssignedToOtherUser: true, MainUserID: "main-1", LocalStatus: "active"}
		}, "PROFILE_ASSIGNED_TO_OTHER_USER"},
		{"profile missing", func(r *Request) { r.Cloud.Profile = ProfileFacts{} }, "PROFILE_NOT_FOUND"},
		{"profile under an old main", func(r *Request) { r.Cloud.Profile.MainUserID = "main-0" }, "PROFILE_NOT_FOUND"},
		{"profile locally missing", func(r *Request) { r.Cloud.Profile.LocalStatus = "local_missing" }, "PROFILE_NOT_FOUND"},
		{"profile not visible to bitbrowser", func(r *Request) { r.Agent.ProfileVisible = false }, "PROFILE_NOT_FOUND"},
		{"platform login expired", func(r *Request) { r.Cloud.MediaAccount.LoginStatus = "expired" }, "PROFILE_LOGIN_INVALID"},
		{"media account missing", func(r *Request) { r.Cloud.MediaAccount = MediaAccountFacts{} }, "PROFILE_LOGIN_INVALID"},
		{"files not ready", func(r *Request) { r.Agent.FilesReady = false }, "FILE_PREPARE_FAILED"},
		{"unknown action", func(r *Request) { r.Action = "publish_now" }, "UNSUPPORTED_TASK_TYPE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := greenRequest()
			tc.mutate(&request)
			failure := Evaluate(request)
			if failure == nil || failure.Code != tc.code {
				t.Fatalf("Evaluate() = %+v, want code %s", failure, tc.code)
			}
		})
	}
}

// 已领取任务的准入不看当前会话：抢占只挡住新工作，不打断已经开始的执行。
func TestEvaluateClaimedTaskSkipsTheCurrentSessionChecks(t *testing.T) {
	request := greenRequest()
	request.Mode = ModeClaimedTask
	request.Cloud.UserEnabled = false
	request.Cloud.DesktopSessionValid = false
	if failure := Evaluate(request); failure != nil {
		t.Fatalf("Evaluate() = %+v, want nil", failure)
	}
}

func TestEvaluateActionsOnlyRequireWhatTheyUse(t *testing.T) {
	accountCheck := greenRequest()
	accountCheck.Action = ActionAccountCheck
	accountCheck.Cloud.MediaAccount = MediaAccountFacts{}
	accountCheck.Agent.FilesReady = false
	if failure := Evaluate(accountCheck); failure != nil {
		t.Fatalf("account check = %+v, want nil", failure)
	}

	profileCreate := greenRequest()
	profileCreate.Action = ActionProfileCreate
	profileCreate.ProfileID = ""
	profileCreate.Cloud.Profile = ProfileFacts{}
	if failure := Evaluate(profileCreate); failure != nil {
		t.Fatalf("profile create = %+v, want nil", failure)
	}
}

func TestObservationFreshBoundary(t *testing.T) {
	request := greenRequest()
	request.Agent.ObservedAt = testNow.Add(-120 * time.Second)
	if failure := Evaluate(request); failure != nil {
		t.Fatalf("exactly at the window = %+v, want nil", failure)
	}
	request.Agent.ObservedAt = testNow.Add(-120*time.Second - time.Nanosecond)
	if failure := Evaluate(request); failure == nil || failure.Code != "BIT_BROWSER_OFFLINE" {
		t.Fatalf("past the window = %+v, want BIT_BROWSER_OFFLINE", failure)
	}
}

func TestActionForSensitiveOperationCoversEveryOperation(t *testing.T) {
	for operation, want := range map[string]Action{
		"assisted_publication":        ActionAssistedPublication,
		"interaction":                 ActionInteraction,
		"authenticated_account_check": ActionAccountCheck,
		"cookie_read":                 ActionCookieRead,
		"cookie_write":                ActionCookieWrite,
		"profile_mutation":            ActionProfileUpdate,
		"proxy_mutation":              ActionProxyMutation,
	} {
		action, ok := ActionForSensitiveOperation(operation)
		if !ok || action != want {
			t.Fatalf("%s = %q, %t; want %q", operation, action, ok, want)
		}
	}
	if _, ok := ActionForSensitiveOperation("publish_now"); ok {
		t.Fatal("unknown sensitive operation was accepted")
	}
}

func TestFailureStatusMapsEveryCodeToItsEnvelope(t *testing.T) {
	for code, want := range map[string]struct {
		status  int
		errcode int
	}{
		"AUTH_DESKTOP_REPLACED":          {401, 11001},
		"AUTH_SESSION_INVALIDATED":       {401, 11001},
		"DESKTOP_LOGIN_REQUIRED":         {401, 11001},
		"AUTH_AGENT_TOKEN_INVALID":       {401, 11001},
		"AGENT_SCOPE_FORBIDDEN":          {403, 11003},
		"PROFILE_ASSIGNED_TO_OTHER_USER": {403, 11003},
		"PROFILE_NOT_FOUND":              {404, 20004},
		"STALE_ATTEMPT":                  {409, 30009},
		"BIT_MAIN_USER_MISMATCH":         {409, 23002},
		"BIT_NOT_BOUND":                  {409, 23008},
		"BIT_BROWSER_OFFLINE":            {409, 23009},
		"PROFILE_LOGIN_INVALID":          {409, 23010},
		"FILE_PREPARE_FAILED":            {422, 23011},
		"UNSUPPORTED_TASK_TYPE":          {400, 10002},
	} {
		status, errcode := failureStatus(code)
		if status != want.status || errcode != want.errcode {
			t.Fatalf("%s = %d/%d, want %d/%d", code, status, errcode, want.status, want.errcode)
		}
	}
	if status, errcode := failureStatus("SOMETHING_ELSE"); status != 500 || errcode != 50000 {
		t.Fatalf("unknown code = %d/%d, want the generic internal error", status, errcode)
	}
}
