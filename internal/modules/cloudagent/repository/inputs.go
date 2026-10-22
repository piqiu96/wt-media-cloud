package repository

// Persistence inputs decouple Cloud-Agent HTTP DTOs from SQL persistence.

type CreateTaskInput struct {
	TaskType       string
	IdempotencyKey string
	Payload        map[string]any
}

type ClaimTaskInput struct {
	AgentID      string
	LeaseSeconds int
}

type ReportTaskInput struct {
	AgentID   string
	Status    string
	Progress  int
	Message   string
	ErrorCode string
	Result    map[string]any
}

type CancelTaskInput struct {
	Message string
}
