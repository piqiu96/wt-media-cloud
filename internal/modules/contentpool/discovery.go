package contentpool

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
)

type StrategyStatus string

const (
	StrategyEnabled  StrategyStatus = "enabled"
	StrategyDisabled StrategyStatus = "disabled"
)

type DiscoveryStrategy struct {
	ID           int64           `json:"id"`
	TeamID       identity.TeamID `json:"team_id"`
	Name         string          `json:"name"`
	StrategyType string          `json:"strategy_type"`
	Platform     string          `json:"platform"`
	Config       map[string]any  `json:"config"`
	Schedule     string          `json:"schedule"`
	Timezone     string          `json:"timezone"`
	Status       StrategyStatus  `json:"status"`
	CreatedBy    identity.UserID `json:"created_by"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type CrawlStatus string

const (
	CrawlPending CrawlStatus = "pending"
	CrawlRunning CrawlStatus = "running"
	CrawlSuccess CrawlStatus = "success"
	CrawlFailed  CrawlStatus = "failed"
)

type CrawlTask struct {
	ID         int64            `json:"id"`
	TeamID     identity.TeamID  `json:"team_id"`
	StrategyID *int64           `json:"strategy_id,omitempty"`
	TaskID     string           `json:"task_id,omitempty"`
	TaskType   string           `json:"task_type"`
	Platform   string           `json:"platform"`
	Status     CrawlStatus      `json:"status"`
	Snapshot   map[string]any   `json:"snapshot"`
	Stats      CrawlStats       `json:"stats"`
	Results    []map[string]any `json:"results,omitempty"`
	Error      string           `json:"error,omitempty"`
	StartedAt  *time.Time       `json:"started_at,omitempty"`
	FinishedAt *time.Time       `json:"finished_at,omitempty"`
	CreatedBy  identity.UserID  `json:"created_by"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`
}

type CrawlStats struct {
	Scanned   int `json:"scanned"`
	Found     int `json:"found"`
	Added     int `json:"added"`
	Duplicate int `json:"duplicate"`
	Failed    int `json:"failed"`
}

type DiscoveryStore interface {
	CreateStrategy(DiscoveryStrategy) (DiscoveryStrategy, error)
	ListStrategies(*identity.TeamID) ([]DiscoveryStrategy, error)
	FindStrategy(int64) (DiscoveryStrategy, bool, error)
	UpdateStrategy(DiscoveryStrategy) (DiscoveryStrategy, error)
	CreateCrawlTask(CrawlTask) (CrawlTask, error)
	ListCrawlTasks(*identity.TeamID, *int64) ([]CrawlTask, error)
	FindCrawlTask(int64) (CrawlTask, bool, error)
	UpdateCrawlTask(CrawlTask) (CrawlTask, error)
}

type DiscoveryService struct {
	store      DiscoveryStore
	content    *Service
	now        func() time.Time
	createTask func(cloudagent.CreateTaskRequest) cloudagent.Task
}

var (
	ErrStrategyNotFound   = errors.New("discovery strategy not found")
	ErrCrawlTaskNotFound  = errors.New("crawl task not found")
	ErrDiscoveryForbidden = errors.New("discovery operation is forbidden")
	ErrDiscoveryInvalid   = errors.New("discovery input is invalid")
	ErrStrategyDuplicate  = errors.New("discovery strategy already exists")
	ErrDiscoverySelection = errors.New("discovery result selection is invalid")
)

func NewDiscoveryService(store DiscoveryStore, content *Service) *DiscoveryService {
	return &DiscoveryService{store: store, content: content, now: time.Now}
}

func (s *DiscoveryService) SetTaskCreator(create func(cloudagent.CreateTaskRequest) cloudagent.Task) {
	s.createTask = create
}

func (s *DiscoveryService) scope(actor identity.PublicUser, team *identity.TeamID) (*identity.TeamID, error) {
	if actor.Role == identity.RoleAdmin {
		return team, nil
	}
	if actor.TeamID == nil || (team != nil && *team != *actor.TeamID) {
		return nil, ErrDiscoveryForbidden
	}
	v := *actor.TeamID
	return &v, nil
}

func (s *DiscoveryService) ListStrategies(actor identity.PublicUser) ([]DiscoveryStrategy, error) {
	team, err := s.scope(actor, nil)
	if err != nil {
		return nil, err
	}
	return s.store.ListStrategies(team)
}

func (s *DiscoveryService) CreateStrategy(actor identity.PublicUser, input DiscoveryStrategy) (DiscoveryStrategy, error) {
	team, err := s.scope(actor, &input.TeamID)
	if err != nil {
		return DiscoveryStrategy{}, err
	}
	if team == nil || strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Platform) == "" || strings.TrimSpace(input.Platform) != "douyin" {
		return DiscoveryStrategy{}, ErrDiscoveryInvalid
	}
	if input.StrategyType != "keyword" && input.StrategyType != "author" {
		return DiscoveryStrategy{}, ErrDiscoveryInvalid
	}
	if input.Config == nil {
		input.Config = map[string]any{}
	}
	if input.StrategyType == "keyword" && strings.TrimSpace(fmt.Sprint(input.Config["keyword"])) == "" && len(anyStringSlice(input.Config["keywords"])) == 0 {
		return DiscoveryStrategy{}, ErrDiscoveryInvalid
	}
	if input.StrategyType == "author" && strings.TrimSpace(fmt.Sprint(input.Config["author"])) == "" && strings.TrimSpace(fmt.Sprint(input.Config["author_id"])) == "" {
		return DiscoveryStrategy{}, ErrDiscoveryInvalid
	}
	if input.Timezone == "" {
		input.Timezone = "Asia/Shanghai"
	}
	if input.Schedule == "" {
		input.Schedule = "manual"
	}
	if input.Status == "" {
		input.Status = StrategyDisabled
	}
	if input.Status != StrategyEnabled && input.Status != StrategyDisabled {
		return DiscoveryStrategy{}, ErrDiscoveryInvalid
	}
	now := s.now()
	input.TeamID = *team
	input.Name = strings.TrimSpace(input.Name)
	input.Platform = strings.TrimSpace(input.Platform)
	input.CreatedBy = actor.ID
	input.CreatedAt = now
	input.UpdatedAt = now
	return s.store.CreateStrategy(input)
}

func (s *DiscoveryService) SetStrategyStatus(actor identity.PublicUser, id int64, status StrategyStatus) (DiscoveryStrategy, error) {
	item, ok, err := s.store.FindStrategy(id)
	if err != nil {
		return DiscoveryStrategy{}, err
	}
	if !ok {
		return DiscoveryStrategy{}, ErrStrategyNotFound
	}
	if _, err = s.scope(actor, &item.TeamID); err != nil {
		return DiscoveryStrategy{}, err
	}
	if status != StrategyEnabled && status != StrategyDisabled {
		return DiscoveryStrategy{}, ErrDiscoveryInvalid
	}
	item.Status = status
	item.UpdatedAt = s.now()
	return s.store.UpdateStrategy(item)
}

func (s *DiscoveryService) UpdateStrategy(actor identity.PublicUser, id int64, input DiscoveryStrategy) (DiscoveryStrategy, error) {
	current, ok, err := s.store.FindStrategy(id)
	if err != nil {
		return DiscoveryStrategy{}, err
	}
	if !ok {
		return DiscoveryStrategy{}, ErrStrategyNotFound
	}
	if _, err = s.scope(actor, &current.TeamID); err != nil {
		return DiscoveryStrategy{}, err
	}
	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Platform) != "douyin" || (input.StrategyType != "keyword" && input.StrategyType != "author") {
		return DiscoveryStrategy{}, ErrDiscoveryInvalid
	}
	if input.Config == nil {
		input.Config = map[string]any{}
	}
	if input.StrategyType == "keyword" && strings.TrimSpace(fmt.Sprint(input.Config["keyword"])) == "" && len(anyStringSlice(input.Config["keywords"])) == 0 {
		return DiscoveryStrategy{}, ErrDiscoveryInvalid
	}
	if input.StrategyType == "author" && strings.TrimSpace(fmt.Sprint(input.Config["author"])) == "" && strings.TrimSpace(fmt.Sprint(input.Config["author_id"])) == "" {
		return DiscoveryStrategy{}, ErrDiscoveryInvalid
	}
	if input.Schedule == "" {
		input.Schedule = "manual"
	}
	if input.Timezone == "" {
		input.Timezone = current.Timezone
	}
	if input.Status == "" {
		input.Status = current.Status
	}
	if input.Status != StrategyEnabled && input.Status != StrategyDisabled {
		return DiscoveryStrategy{}, ErrDiscoveryInvalid
	}
	current.Name = strings.TrimSpace(input.Name)
	current.StrategyType = input.StrategyType
	current.Platform = strings.TrimSpace(input.Platform)
	current.Config = input.Config
	current.Schedule = input.Schedule
	current.Timezone = input.Timezone
	current.Status = input.Status
	current.UpdatedAt = s.now()
	return s.store.UpdateStrategy(current)
}

func (s *DiscoveryService) CreateRun(actor identity.PublicUser, strategyID int64) (CrawlTask, error) {
	strategy, ok, err := s.store.FindStrategy(strategyID)
	if err != nil {
		return CrawlTask{}, err
	}
	if !ok {
		return CrawlTask{}, ErrStrategyNotFound
	}
	if _, err = s.scope(actor, &strategy.TeamID); err != nil {
		return CrawlTask{}, err
	}
	if strategy.Status != StrategyEnabled {
		return CrawlTask{}, ErrDiscoveryInvalid
	}
	if s.createTask == nil {
		return CrawlTask{}, ErrDiscoveryInvalid
	}
	if strategy.StrategyType == "keyword" && strings.TrimSpace(fmt.Sprint(strategy.Config["keyword"])) == "" && len(anyStringSlice(strategy.Config["keywords"])) == 0 {
		return CrawlTask{}, ErrDiscoveryInvalid
	}
	if strategy.StrategyType == "author" && strings.TrimSpace(fmt.Sprint(strategy.Config["author"])) == "" && strings.TrimSpace(fmt.Sprint(strategy.Config["author_id"])) == "" {
		return CrawlTask{}, ErrDiscoveryInvalid
	}
	now := s.now()
	task := CrawlTask{TeamID: strategy.TeamID, StrategyID: &strategy.ID, TaskType: "discovery_task", Platform: strategy.Platform, Status: CrawlPending, Snapshot: cloneMap(strategy.Config), CreatedBy: actor.ID, CreatedAt: now, UpdatedAt: now}
	created, err := s.store.CreateCrawlTask(task)
	if err != nil {
		return CrawlTask{}, err
	}
	payload := cloneMap(strategy.Config)
	payload["operation"] = strategy.StrategyType
	payload["platform"] = strategy.Platform
	payload["crawl_task_id"] = created.ID
	cloudTask := cloudagent.CreateTaskRequest{TaskType: cloudagent.TaskTypeDiscovery.String(), IdempotencyKey: fmt.Sprintf("crawl:%d", created.ID), Payload: payload}
	assigned := s.createTask(cloudTask)
	created.TaskID = assigned.TaskID
	_, err = s.store.UpdateCrawlTask(created)
	if err != nil {
		return CrawlTask{}, err
	}
	return created, nil
}

func (s *DiscoveryService) CreateManualRun(actor identity.PublicUser, platform, operation string, config map[string]any) (CrawlTask, error) {
	team, err := s.scope(actor, nil)
	if err != nil {
		return CrawlTask{}, err
	}
	if team == nil || strings.TrimSpace(platform) != "douyin" || (operation != "url" && operation != "keyword" && operation != "author") {
		return CrawlTask{}, ErrDiscoveryInvalid
	}
	if operation == "url" && strings.TrimSpace(fmt.Sprint(config["url"])) == "" && len(anyStringSlice(config["urls"])) == 0 {
		return CrawlTask{}, ErrDiscoveryInvalid
	}
	if (operation == "keyword" && strings.TrimSpace(fmt.Sprint(config["keyword"])) == "") || (operation == "author" && strings.TrimSpace(fmt.Sprint(config["author"])) == "") {
		return CrawlTask{}, ErrDiscoveryInvalid
	}
	if s.createTask == nil {
		return CrawlTask{}, ErrDiscoveryInvalid
	}
	now := s.now()
	snapshot := cloneMap(config)
	snapshot["operation"] = operation
	task := CrawlTask{TeamID: *team, TaskType: "manual_discovery_task", Platform: strings.TrimSpace(platform), Status: CrawlPending, Snapshot: snapshot, CreatedBy: actor.ID, CreatedAt: now, UpdatedAt: now}
	created, err := s.store.CreateCrawlTask(task)
	if err != nil {
		return CrawlTask{}, err
	}
	payload := cloneMap(config)
	payload["operation"] = operation
	payload["platform"] = platform
	payload["crawl_task_id"] = created.ID
	assigned := s.createTask(cloudagent.CreateTaskRequest{TaskType: cloudagent.TaskTypeDiscovery.String(), IdempotencyKey: fmt.Sprintf("crawl:%d", created.ID), Payload: payload})
	created.TaskID = assigned.TaskID
	_, err = s.store.UpdateCrawlTask(created)
	if err != nil {
		return CrawlTask{}, err
	}
	return created, nil
}

func (s *DiscoveryService) ListCrawlTasks(actor identity.PublicUser, strategyID *int64) ([]CrawlTask, error) {
	team, err := s.scope(actor, nil)
	if err != nil {
		return nil, err
	}
	return s.store.ListCrawlTasks(team, strategyID)
}

// RunDue triggers enabled strategies whose simple schedule is due. Scheduling
// remains a host concern; this method only applies business idempotency rules.
func (s *DiscoveryService) RunDue(now time.Time) int {
	strategies, err := s.store.ListStrategies(nil)
	if err != nil {
		return 0
	}
	triggered := 0
	for _, strategy := range strategies {
		location := time.Local
		if strategy.Timezone != "" {
			if loaded, loadErr := time.LoadLocation(strategy.Timezone); loadErr == nil {
				location = loaded
			}
		}
		localNow := now.In(location)
		if strategy.Status != StrategyEnabled || !scheduleDue(strategy.Schedule, localNow) {
			continue
		}
		previous, _ := s.store.ListCrawlTasks(&strategy.TeamID, &strategy.ID)
		if len(previous) > 0 && sameScheduleWindow(previous[0].CreatedAt.In(location), strategy.Schedule, localNow) {
			continue
		}
		if _, err := s.CreateRun(identity.PublicUser{ID: strategy.CreatedBy, Role: identity.RoleAdmin}, strategy.ID); err == nil {
			triggered++
		}
	}
	return triggered
}

func scheduleDue(schedule string, now time.Time) bool {
	value := strings.TrimSpace(schedule)
	if value == "" || value == "manual" {
		return false
	}
	if strings.HasPrefix(value, "interval:") {
		minutes, err := strconv.Atoi(strings.TrimPrefix(value, "interval:"))
		return err == nil && minutes > 0
	}
	if strings.HasPrefix(value, "daily ") {
		parts := strings.Split(strings.TrimSpace(strings.TrimPrefix(value, "daily ")), ":")
		if len(parts) != 2 {
			return false
		}
		h, e1 := strconv.Atoi(parts[0])
		m, e2 := strconv.Atoi(parts[1])
		return e1 == nil && e2 == nil && h == now.Hour() && m == now.Minute()
	}
	return false
}

func sameScheduleWindow(created time.Time, schedule string, now time.Time) bool {
	if strings.HasPrefix(schedule, "daily ") {
		y1, m1, d1 := created.Date()
		y2, m2, d2 := now.Date()
		return y1 == y2 && m1 == m2 && d1 == d2
	}
	if strings.HasPrefix(schedule, "interval:") {
		minutes, err := strconv.Atoi(strings.TrimPrefix(schedule, "interval:"))
		return err == nil && now.Sub(created) < time.Duration(minutes)*time.Minute
	}
	return false
}

func (s *DiscoveryService) GetCrawlTask(actor identity.PublicUser, id int64) (CrawlTask, bool, error) {
	item, ok, err := s.store.FindCrawlTask(id)
	if err != nil || !ok {
		return item, ok, err
	}
	if _, err = s.scope(actor, &item.TeamID); err != nil {
		return CrawlTask{}, false, err
	}
	return item, true, nil
}

// ConfirmResults promotes only selected manual-search results into the
// content pool. Link imports and scheduled strategies are ingested directly.
func (s *DiscoveryService) ConfirmResults(actor identity.PublicUser, id int64, ids []string) (CrawlTask, error) {
	if len(ids) == 0 || len(ids) > 500 {
		return CrawlTask{}, ErrDiscoverySelection
	}
	task, ok, err := s.store.FindCrawlTask(id)
	if err != nil {
		return CrawlTask{}, err
	}
	if !ok {
		return CrawlTask{}, ErrCrawlTaskNotFound
	}
	if _, err = s.scope(actor, &task.TeamID); err != nil {
		return CrawlTask{}, err
	}
	if task.Status != CrawlSuccess || len(task.Results) == 0 {
		return CrawlTask{}, ErrDiscoverySelection
	}
	selected := map[string]struct{}{}
	for _, value := range ids {
		if value = strings.TrimSpace(value); value != "" {
			selected[value] = struct{}{}
		}
	}
	if len(selected) == 0 {
		return CrawlTask{}, ErrDiscoverySelection
	}
	operation := fmt.Sprint(task.Snapshot["operation"])
	sourceType := map[string]string{"keyword": "search", "author": "author"}[operation]
	if sourceType == "" {
		return CrawlTask{}, ErrDiscoverySelection
	}
	team := task.TeamID
	matched := 0
	for _, item := range task.Results {
		contentID := strings.TrimSpace(fmt.Sprint(item["platform_content_id"]))
		if _, ok := selected[contentID]; !ok {
			continue
		}
		matched++
		if promoted, _ := item["promoted"].(bool); promoted {
			continue
		}
		_, createErr := s.content.CreateSource(actor, SourceInput{TeamID: &team, Platform: task.Platform, PlatformContentID: contentID, Title: fmt.Sprint(item["title"]), Description: fmt.Sprint(item["description"]), CoverURL: fmt.Sprint(item["cover_url"]), SourceURL: fmt.Sprint(item["source_url"]), AuthorID: fmt.Sprint(item["author_id"]), AuthorName: fmt.Sprint(item["author_name"]), SourceType: sourceType, PublishedAt: parsePublishedAt(item["published_at"]), RawJSON: mustJSON(item)})
		if errors.Is(createErr, ErrDuplicate) {
			task.Stats.Duplicate++
			item["promoted"] = true
		} else if createErr != nil {
			task.Stats.Failed++
		} else {
			task.Stats.Added++
			item["promoted"] = true
		}
	}
	if matched == 0 {
		return CrawlTask{}, ErrDiscoverySelection
	}
	task.UpdatedAt = s.now()
	_, err = s.store.UpdateCrawlTask(task)
	return task, err
}

// HandleTaskResult is called by Cloud-Agent task reporting after a discovery task succeeds.
func (s *DiscoveryService) HandleTaskResult(taskID string, result map[string]any) error {
	id, err := strconv.ParseInt(fmt.Sprint(result["crawl_task_id"]), 10, 64)
	if err != nil || id <= 0 {
		return ErrCrawlTaskNotFound
	}
	task, ok, err := s.store.FindCrawlTask(id)
	if err != nil {
		return err
	}
	if !ok {
		return ErrCrawlTaskNotFound
	}
	items := resultItems(result["items"])
	stats := CrawlStats{Scanned: len(items), Found: len(items)}
	sourceType := "strategy"
	if task.TaskType == "manual_discovery_task" {
		sourceType = fmt.Sprint(task.Snapshot["operation"])
		if sourceType == "url" {
			sourceType = "link"
		}
	}
	if sourceType == "keyword" || sourceType == "author" {
		items = dedupeResultItems(items)
		stats.Scanned, stats.Found = len(items), len(items)
	}
	task.Results = items
	if sourceType == "keyword" || sourceType == "author" {
		now := s.now()
		task.Status = CrawlSuccess
		task.Stats = stats
		task.FinishedAt = &now
		task.UpdatedAt = now
		_, err = s.store.UpdateCrawlTask(task)
		return err
	}
	for _, raw := range items {
		item := raw
		team := task.TeamID
		_, err = s.content.CreateSource(identity.PublicUser{ID: task.CreatedBy, Role: identity.RoleAdmin}, SourceInput{TeamID: &team, Platform: task.Platform, PlatformContentID: fmt.Sprint(item["platform_content_id"]), Title: fmt.Sprint(item["title"]), Description: fmt.Sprint(item["description"]), CoverURL: fmt.Sprint(item["cover_url"]), SourceURL: fmt.Sprint(item["source_url"]), AuthorID: fmt.Sprint(item["author_id"]), AuthorName: fmt.Sprint(item["author_name"]), SourceType: sourceType, PublishedAt: parsePublishedAt(item["published_at"]), RawJSON: mustJSON(item)})
		if errors.Is(err, ErrDuplicate) {
			stats.Duplicate++
			continue
		}
		if err != nil {
			stats.Failed++
			continue
		}
		stats.Added++
	}
	now := s.now()
	task.Status = CrawlSuccess
	task.Stats = stats
	task.FinishedAt = &now
	task.UpdatedAt = now
	_, err = s.store.UpdateCrawlTask(task)
	return err
}

func (s *DiscoveryService) HandleTaskProgress(taskID string, status CrawlStatus, message string) error {
	if taskID == "" {
		return ErrCrawlTaskNotFound
	}
	items, err := s.store.ListCrawlTasks(nil, nil)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.TaskID != taskID {
			continue
		}
		item.Status = status
		item.Error = message
		item.UpdatedAt = s.now()
		if status == CrawlFailed {
			finished := item.UpdatedAt
			item.FinishedAt = &finished
		}
		_, err = s.store.UpdateCrawlTask(item)
		return err
	}
	return ErrCrawlTaskNotFound
}

func cloneMap(input map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range input {
		out[k] = v
	}
	return out
}
func mustJSON(value any) json.RawMessage { raw, _ := json.Marshal(value); return raw }

func parsePublishedAt(value any) *time.Time {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, text)
	if err != nil {
		return nil
	}
	return &parsed
}

func anyStringSlice(value any) []string {
	switch raw := value.(type) {
	case []string:
		out := make([]string, 0, len(raw))
		for _, item := range raw {
			if item = strings.TrimSpace(item); item != "" {
				out = append(out, item)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(raw))
		for _, item := range raw {
			if text := strings.TrimSpace(fmt.Sprint(item)); text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func resultItems(value any) []map[string]any {
	switch items := value.(type) {
	case []map[string]any:
		return items
	case []any:
		out := make([]map[string]any, 0, len(items))
		for _, raw := range items {
			if item, ok := raw.(map[string]any); ok {
				out = append(out, item)
			}
		}
		return out
	default:
		return nil
	}
}

func dedupeResultItems(items []map[string]any) []map[string]any {
	seen := map[string]struct{}{}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		id := strings.TrimSpace(fmt.Sprint(item["platform_content_id"]))
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, item)
	}
	return out
}
