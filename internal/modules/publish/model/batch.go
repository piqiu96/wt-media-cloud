package model

import "time"

type BatchTarget struct {
	CategoryID  int64 `json:"category_id,string"`
	TargetCount int64 `json:"target_count"`
}

type BatchPair struct {
	CategoryID        int64  `json:"category_id,string"`
	MediaAccountID    uint64 `json:"media_account_id,string"`
	CompositeOutputID int64  `json:"composite_output_id,string"`
}

type BatchCandidate struct {
	BatchPair
	RowKey         string            `json:"row_key"`
	InputRevision  string            `json:"input_revision"`
	FieldRevisions map[string]string `json:"field_revisions"`
	Order          int               `json:"order"`
	Content        Content           `json:"content"`
	CoverTitle     string            `json:"cover_title,omitempty"`
	NotBefore      *time.Time        `json:"not_before"`
}

type BatchAccountLimit struct {
	MediaAccountID uint64 `json:"media_account_id,string"`
	Limit          int    `json:"limit"`
}

type BatchAllocation struct {
	Accounts    []BatchAccountLimit `json:"accounts,omitempty"`
	OutputIDs   []string            `json:"output_ids,omitempty"`
	ManualPairs []BatchPair         `json:"manual_pairs,omitempty"`
}

type BatchConfig struct {
	BatchAllocation
	Candidates []BatchCandidate `json:"candidates"`
}

type BatchExecutionSetting struct {
	ConcurrencyLimit     int    `json:"concurrency_limit"`
	GroupIntervalSeconds int    `json:"group_interval_seconds"`
	BetweenGroupPolicy   string `json:"between_group_policy"`
	ExceptionTaskLimit   uint32 `json:"exception_task_limit"`
}

type BatchIssue struct {
	RowKey     string `json:"row_key,omitempty"`
	CategoryID int64  `json:"category_id,string,omitempty"`
	Field      string `json:"field,omitempty"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

type BatchShortfall struct {
	CategoryID     int64 `json:"category_id,string"`
	TargetCount    int64 `json:"target_count"`
	CandidateCount int   `json:"candidate_count"`
	Shortfall      int64 `json:"shortfall"`
}

type BatchCoverResource struct {
	RowKey                    string `json:"row_key"`
	CoverResourcePath         string `json:"cover_resource_path,omitempty"`
	PortraitCoverResourcePath string `json:"portrait_cover_resource_path,omitempty"`
}
type BatchReadiness struct {
	CoverResources []BatchCoverResource `json:"cover_resources"`
	Ready          bool                 `json:"ready"`
	Issues         []BatchIssue         `json:"issues"`
	Shortfalls     []BatchShortfall     `json:"shortfalls"`
}

const (
	BatchIncomplete = "incomplete"
	BatchReady      = "ready"
	BatchNotStarted = "not_started"
	BatchRunning    = "running"
	BatchPaused     = "paused"
	BatchStopping   = "stopping"
	BatchEnded      = "ended"
	BatchCancelled  = "cancelled"
)

type Batch struct {
	PlanID                  *uint64                `json:"plan_id,string"`
	PlanRowKey              *string                `json:"-"`
	PlanGroupOrder          *uint32                `json:"-"`
	PlanBatchOrder          *uint32                `json:"-"`
	PlanConfiguredAt        *time.Time             `json:"-"`
	PlanFrozenAt            *time.Time             `json:"-"`
	FrozenConfirmation      *PlanBatchConfirmation `json:"-" gorm:"column:frozen_confirmation_json;serializer:json"`
	HandedOffAt             *time.Time             `json:"handed_off_at"`
	HandoffBasisFingerprint *string                `json:"-"`

	ID                  uint64                `json:"id,string"`
	TeamID              int64                 `json:"team_id,string"`
	OperatorID          int64                 `json:"operator_id,string"`
	CreatedBy           int64                 `json:"created_by,string"`
	Name                string                `json:"name"`
	Platform            string                `json:"platform"`
	Targets             []BatchTarget         `json:"targets" gorm:"column:targets_json;serializer:json"`
	Config              BatchConfig           `json:"-" gorm:"column:config_json;serializer:json"`
	Readiness           BatchReadiness        `json:"-" gorm:"column:readiness_json;serializer:json"`
	CandidatesCount     int                   `json:"candidate_count"`
	ExecutionSetting    BatchExecutionSetting `json:"execution_setting" gorm:"embedded"`
	ConfigurationStatus string                `json:"configuration_status"`
	ExecutionStatus     string                `json:"execution_status"`
	StartedAt           *time.Time            `json:"-"`
	ExecutionGroupNo    *uint32               `json:"-"`
	GroupEndedAt        *time.Time            `json:"-"`
	NextGroupAt         *time.Time            `json:"-"`
	StoppedAt           *time.Time            `json:"-"`
	EndedAt             *time.Time            `json:"-"`
	StopReason          string                `json:"-"`
	FinishReason        string                `json:"-"`
	PauseID             uint64                `json:"-"`
	Pause               *BatchPause           `json:"-" gorm:"column:pause_json;serializer:json"`
	ExceptionAck        map[string]string     `json:"-" gorm:"column:exception_ack_json;serializer:json"`
	StartRequestKey     *string               `json:"-"`
	StartRequestHash    *string               `json:"-"`
	StartReceipt        *BatchStartResult     `json:"-" gorm:"column:start_receipt_json;serializer:json"`
	Execution           BatchExecution        `json:"execution" gorm:"-"`
	Revision            uint64                `json:"revision,string"`
	RequestKey          string                `json:"-"`
	RequestHash         string                `json:"-"`
	CreatedAt           time.Time             `json:"created_at"`
	UpdatedAt           time.Time             `json:"updated_at"`
	AllowedActions      []string              `json:"allowed_actions" gorm:"-"`
}

func (Batch) TableName() string { return "publication_batch" }

type BatchDetail struct {
	Batch
	Config    BatchConfig    `json:"config"`
	Readiness BatchReadiness `json:"readiness"`
}

type BatchRecord struct {
	ID        string    `json:"id"`
	ActorID   *int64    `json:"actor_id,string"`
	Event     string    `json:"event"`
	CreatedAt time.Time `json:"created_at"`
}
