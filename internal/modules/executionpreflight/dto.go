package executionpreflight

import (
	"strings"
	"time"
)

// FactsInput 是 Local Agent 在准入请求里提交的本机事实。
type FactsInput struct {
	BitBrowserStatus string    `json:"bitbrowser_status"`
	MainUserID       string    `json:"main_user_id"`
	ObservedAt       time.Time `json:"observed_at"`
	ProfileVisible   bool      `json:"profile_visible"`
	FilesReady       bool      `json:"files_ready"`
}

func (in FactsInput) AgentFacts() AgentFacts {
	return AgentFacts{
		BitBrowserRunning: in.BitBrowserStatus == "running",
		MainUserID:        strings.TrimSpace(in.MainUserID),
		ObservedAt:        in.ObservedAt,
		ProfileVisible:    in.ProfileVisible,
		FilesReady:        in.FilesReady,
	}
}

type PreflightRequest struct {
	Action         string     `json:"action"`
	TaskID         string     `json:"task_id"`
	AttemptID      string     `json:"attempt_id"`
	ProfileID      string     `json:"profile_id"`
	MediaAccountID string     `json:"media_account_id"`
	Facts          FactsInput `json:"facts"`
}
