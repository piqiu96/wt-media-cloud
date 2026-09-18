// Package dto owns Cloud-Agent request and response contracts.
package dto

type RegisterAgentRequest struct {
	AgentID              string   `json:"agent_id"`
	Mode                 string   `json:"mode"`
	Version              string   `json:"version"`
	ContractMajorVersion string   `json:"contract_major_version"`
	ContractRevision     string   `json:"contract_revision"`
	Capabilities         []string `json:"capabilities"`
}

type HeartbeatRequest struct {
	Status string `json:"status"`
}

type CreateTaskRequest struct {
	TaskType       string         `json:"task_type"`
	IdempotencyKey string         `json:"idempotency_key"`
	Payload        map[string]any `json:"payload,omitempty"`
}

type ClaimTaskRequest struct {
	AgentID      string `json:"agent_id"`
	LeaseSeconds int    `json:"lease_seconds"`
}

type ReportTaskRequest struct {
	AgentID   string         `json:"agent_id"`
	Status    string         `json:"status"`
	Progress  int            `json:"progress"`
	Message   string         `json:"message"`
	ErrorCode string         `json:"error_code,omitempty"`
	Result    map[string]any `json:"result,omitempty"`
}

type CancelTaskRequest struct {
	Message string `json:"message"`
}
