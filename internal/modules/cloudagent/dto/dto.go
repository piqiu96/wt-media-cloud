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
	Status    string         `json:"status"`
	Progress  int            `json:"progress"`
	Message   string         `json:"message"`
	ErrorCode string         `json:"error_code,omitempty"`
	Result    map[string]any `json:"result,omitempty"`
}

type CancelTaskRequest struct {
	Message string `json:"message"`
}
