package repository

// Persistence inputs decouple Cloud-Agent HTTP DTOs from SQL persistence.

type CreateTaskInput struct {
	UserID         int64
	TaskType       string
	IdempotencyKey string
	Payload        map[string]any
}

type ClaimTaskInput struct {
	UserID       int64
	AgentID      string
	LeaseSeconds int
}

type ReportTaskInput struct {
	UserID         int64
	AttemptID      string
	Status         string
	Progress       int
	Message        string
	ErrorCode      string
	ErrorMessage   string
	Retryable      bool
	RecoveryAction string
	Result         map[string]any
}

type ActivityTaskInput struct {
	UserID    int64
	AttemptID string
	Message   string
	Stage     string
}

type CancelTaskInput struct {
	UserID  int64
	Message string
}

type RetryTaskInput struct {
	UserID               int64
	ConfirmResultUnknown bool
}
