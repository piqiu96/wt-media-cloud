package repository

// Persistence inputs decouple Cloud-Agent HTTP DTOs from SQL persistence.

type RegisterAgentInput struct {
	AgentID              string
	Mode                 string
	Version              string
	ContractMajorVersion string
	ContractRevision     string
	Capabilities         []string
}

type HeartbeatInput struct {
	Status string
}

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
