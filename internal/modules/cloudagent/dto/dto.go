// Package dto owns Cloud-Agent request and response contracts.
package dto

type CreateTaskRequest struct {
	TaskType       string         `json:"task_type"`
	IdempotencyKey string         `json:"idempotency_key"`
	Payload        map[string]any `json:"payload,omitempty"`
}

type ClaimTaskRequest struct {
	LeaseSeconds int `json:"lease_seconds"`
}

type ReportTaskRequest struct {
	AttemptID      string         `json:"attempt_id"`
	Status         string         `json:"status"`
	Progress       int            `json:"progress"`
	Message        string         `json:"message"`
	ErrorCode      string         `json:"error_code,omitempty"`
	ErrorMessage   string         `json:"error_message,omitempty"`
	Retryable      bool           `json:"retryable,omitempty"`
	RecoveryAction string         `json:"recovery_action,omitempty"`
	Result         map[string]any `json:"result,omitempty"`
}

// ActivityTaskRequest 是保活请求，只刷新 last_report_at。
type ActivityTaskRequest struct {
	AttemptID string `json:"attempt_id"`
	Message   string `json:"message,omitempty"`
	Stage     string `json:"stage,omitempty"`
}

type CancelTaskRequest struct {
	Message string `json:"message"`
}

type RetryTaskRequest struct {
	ConfirmResultUnknown bool `json:"confirm_result_unknown"`
}
