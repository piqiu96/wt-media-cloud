package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool/model"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
)

type (
	StrategyStatus    = model.StrategyStatus
	DiscoveryStrategy = model.DiscoveryStrategy
	CrawlStatus       = model.CrawlStatus
	CrawlTask         = model.CrawlTask
	CrawlStats        = model.CrawlStats
	CrawlerRequest    = dto.CrawlerRequest
	CrawlerResult     = dto.CrawlerResult
)

const (
	StrategyEnabled  = model.StrategyEnabled
	StrategyDisabled = model.StrategyDisabled
	CrawlPending     = model.CrawlPending
	CrawlRunning     = model.CrawlRunning
	CrawlSuccess     = model.CrawlSuccess
	CrawlFailed      = model.CrawlFailed
)

var (
	ErrStrategyNotFound   = errors.New("discovery strategy not found")
	ErrCrawlTaskNotFound  = errors.New("crawl task not found")
	ErrDiscoveryForbidden = errors.New("discovery operation is forbidden")
	ErrDiscoveryInvalid   = errors.New("discovery input is invalid")
	ErrStrategyDuplicate  = errors.New("discovery strategy already exists")
	ErrDiscoverySelection = errors.New("discovery result selection is invalid")
)

type crawler interface {
	Discover(context.Context, dto.CrawlerRequest) (dto.CrawlerResult, error)
}

type discoveryStore interface {
	CreateStrategy(model.DiscoveryStrategy) (model.DiscoveryStrategy, error)
	ListStrategies(*identityservice.TeamID) ([]model.DiscoveryStrategy, error)
	FindStrategy(int64) (model.DiscoveryStrategy, bool, error)
	UpdateStrategy(model.DiscoveryStrategy) (model.DiscoveryStrategy, error)
	CreateCrawlTask(model.CrawlTask) (model.CrawlTask, error)
	ListCrawlTasks(*identityservice.TeamID, *int64) ([]model.CrawlTask, error)
	FindCrawlTask(int64) (model.CrawlTask, bool, error)
	UpdateCrawlTask(model.CrawlTask) (model.CrawlTask, error)
	ClaimPendingCrawlTask(time.Time) (model.CrawlTask, bool, error)
}

type discoveryService struct {
	store   discoveryStore
	content *contentService
	now     func() time.Time
	crawler crawler
}

func newDiscoveryService(store discoveryStore, content *contentService, crawler crawler) *discoveryService {
	return &discoveryService{store: store, content: content, now: time.Now, crawler: crawler}
}

func (s *discoveryService) scope(actor identityservice.PublicUser, team *identityservice.TeamID) (*identityservice.TeamID, error) {
	if actor.Role == identityservice.RoleAdmin {
		return team, nil
	}
	if actor.TeamID == nil || (team != nil && *team != *actor.TeamID) {
		return nil, ErrDiscoveryForbidden
	}
	v := *actor.TeamID
	return &v, nil
}

func (s *discoveryService) listStrategies(actor identityservice.PublicUser) ([]model.DiscoveryStrategy, error) {
	team, err := s.scope(actor, nil)
	if err != nil {
		return nil, err
	}
	return s.store.ListStrategies(team)
}

func validStrategy(input model.DiscoveryStrategy) bool {
	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Platform) != "douyin" || (input.StrategyType != "keyword" && input.StrategyType != "author") {
		return false
	}
	if input.StrategyType == "keyword" {
		return strings.TrimSpace(fmt.Sprint(input.Config["keyword"])) != "" || len(anyStringSlice(input.Config["keywords"])) > 0
	}
	return strings.TrimSpace(fmt.Sprint(input.Config["author"])) != "" || strings.TrimSpace(fmt.Sprint(input.Config["author_id"])) != ""
}

func (s *discoveryService) createStrategy(actor identityservice.PublicUser, input model.DiscoveryStrategy) (model.DiscoveryStrategy, error) {
	team, err := s.scope(actor, &input.TeamID)
	if err != nil {
		return model.DiscoveryStrategy{}, err
	}
	if team == nil {
		return model.DiscoveryStrategy{}, ErrDiscoveryInvalid
	}
	if input.Config == nil {
		input.Config = map[string]any{}
	}
	if !validStrategy(input) {
		return model.DiscoveryStrategy{}, ErrDiscoveryInvalid
	}
	if input.Timezone == "" {
		input.Timezone = "Asia/Shanghai"
	}
	if input.Schedule == "" {
		input.Schedule = "manual"
	}
	if input.Status == "" {
		input.Status = model.StrategyDisabled
	}
	if input.Status != model.StrategyEnabled && input.Status != model.StrategyDisabled {
		return model.DiscoveryStrategy{}, ErrDiscoveryInvalid
	}
	now := s.now()
	input.TeamID = *team
	input.Name = strings.TrimSpace(input.Name)
	input.CreatedBy = actor.ID
	input.CreatedAt = now
	input.UpdatedAt = now
	return s.store.CreateStrategy(input)
}

func (s *discoveryService) setStrategyStatus(actor identityservice.PublicUser, id int64, status model.StrategyStatus) (model.DiscoveryStrategy, error) {
	item, ok, err := s.store.FindStrategy(id)
	if err != nil {
		return model.DiscoveryStrategy{}, err
	}
	if !ok {
		return model.DiscoveryStrategy{}, ErrStrategyNotFound
	}
	if _, err = s.scope(actor, &item.TeamID); err != nil {
		return model.DiscoveryStrategy{}, err
	}
	if status != model.StrategyEnabled && status != model.StrategyDisabled {
		return model.DiscoveryStrategy{}, ErrDiscoveryInvalid
	}
	item.Status = status
	item.UpdatedAt = s.now()
	return s.store.UpdateStrategy(item)
}

func (s *discoveryService) updateStrategy(actor identityservice.PublicUser, id int64, input model.DiscoveryStrategy) (model.DiscoveryStrategy, error) {
	current, ok, err := s.store.FindStrategy(id)
	if err != nil {
		return model.DiscoveryStrategy{}, err
	}
	if !ok {
		return model.DiscoveryStrategy{}, ErrStrategyNotFound
	}
	if _, err = s.scope(actor, &current.TeamID); err != nil {
		return model.DiscoveryStrategy{}, err
	}
	if input.Config == nil {
		input.Config = map[string]any{}
	}
	if !validStrategy(input) {
		return model.DiscoveryStrategy{}, ErrDiscoveryInvalid
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
	if input.Status != model.StrategyEnabled && input.Status != model.StrategyDisabled {
		return model.DiscoveryStrategy{}, ErrDiscoveryInvalid
	}
	current.Name, current.StrategyType, current.Platform, current.Config, current.Schedule, current.Timezone, current.Status, current.UpdatedAt = strings.TrimSpace(input.Name), input.StrategyType, input.Platform, input.Config, input.Schedule, input.Timezone, input.Status, s.now()
	return s.store.UpdateStrategy(current)
}

func (s *discoveryService) createRun(actor identityservice.PublicUser, strategyID int64, scheduleKey string) (model.CrawlTask, error) {
	strategy, ok, err := s.store.FindStrategy(strategyID)
	if err != nil {
		return model.CrawlTask{}, err
	}
	if !ok {
		return model.CrawlTask{}, ErrStrategyNotFound
	}
	if _, err = s.scope(actor, &strategy.TeamID); err != nil {
		return model.CrawlTask{}, err
	}
	if strategy.Status != model.StrategyEnabled || !validStrategy(strategy) {
		return model.CrawlTask{}, ErrDiscoveryInvalid
	}
	now := s.now()
	snapshot := cloneMap(strategy.Config)
	snapshot["operation"] = strategy.StrategyType
	return s.store.CreateCrawlTask(model.CrawlTask{TeamID: strategy.TeamID, StrategyID: &strategy.ID, ScheduleKey: scheduleKey, TaskType: "discovery_task", Platform: strategy.Platform, Status: model.CrawlPending, Snapshot: snapshot, CreatedBy: actor.ID, CreatedAt: now, UpdatedAt: now})
}

func (s *discoveryService) createManualRun(actor identityservice.PublicUser, platform, operation string, config map[string]any) (model.CrawlTask, error) {
	team, err := s.scope(actor, nil)
	if err != nil {
		return model.CrawlTask{}, err
	}
	if team == nil || strings.TrimSpace(platform) != "douyin" || (operation != "url" && operation != "keyword" && operation != "author") {
		return model.CrawlTask{}, ErrDiscoveryInvalid
	}
	if (operation == "url" && strings.TrimSpace(fmt.Sprint(config["url"])) == "" && len(anyStringSlice(config["urls"])) == 0) || (operation == "keyword" && strings.TrimSpace(fmt.Sprint(config["keyword"])) == "") || (operation == "author" && strings.TrimSpace(fmt.Sprint(config["author"])) == "") {
		return model.CrawlTask{}, ErrDiscoveryInvalid
	}
	now := s.now()
	snapshot := cloneMap(config)
	snapshot["operation"] = operation
	return s.store.CreateCrawlTask(model.CrawlTask{TeamID: *team, TaskType: "manual_discovery_task", Platform: "douyin", Status: model.CrawlPending, Snapshot: snapshot, CreatedBy: actor.ID, CreatedAt: now, UpdatedAt: now})
}

func (s *discoveryService) runNext(ctx context.Context) (bool, error) {
	task, found, err := s.store.ClaimPendingCrawlTask(s.now())
	if err != nil || !found {
		return found, err
	}
	if s.crawler == nil {
		return true, s.failClaimedTask(task, ErrDiscoveryInvalid)
	}
	operation := strings.TrimSpace(fmt.Sprint(task.Snapshot["operation"]))
	if (operation == "" || operation == "<nil>") && task.StrategyID != nil {
		if strategy, ok, e := s.store.FindStrategy(*task.StrategyID); e != nil {
			return true, s.failClaimedTask(task, e)
		} else if ok {
			operation = strategy.StrategyType
		}
	}
	if operation != "url" && operation != "keyword" && operation != "author" {
		return true, s.failClaimedTask(task, ErrDiscoveryInvalid)
	}
	return true, s.executeClaimed(ctx, task, operation, task.Snapshot, task.TaskType == "manual_discovery_task")
}

func (s *discoveryService) failClaimedTask(task model.CrawlTask, cause error) error {
	now := s.now()
	task.Status, task.Error, task.FinishedAt, task.UpdatedAt = model.CrawlFailed, cause.Error(), &now, now
	_, err := s.store.UpdateCrawlTask(task)
	return err
}

func (s *discoveryService) executeClaimed(ctx context.Context, task model.CrawlTask, operation string, config map[string]any, manual bool) error {
	result, crawlErr := s.crawler.Discover(ctx, dto.CrawlerRequest{Platform: task.Platform, Operation: operation, Config: cloneMap(config)})
	scanned := result.Scanned
	if scanned == 0 {
		scanned = len(result.Items)
	}
	task.Stats = model.CrawlStats{Scanned: scanned, Found: len(result.Items), Failed: result.Failed}
	if manual && (operation == "keyword" || operation == "author") {
		task.Results = dedupeResultItems(result.Items)
		task.Stats.Found = len(task.Results)
	} else {
		sourceType := "strategy"
		if operation == "url" {
			sourceType = "link"
		}
		for _, item := range result.Items {
			team := task.TeamID
			_, err := s.content.createSource(actorForTask(task), dto.SourceInput{TeamID: &team, Platform: task.Platform, PlatformContentID: fmt.Sprint(item["platform_content_id"]), Title: fmt.Sprint(item["title"]), Description: fmt.Sprint(item["description"]), CoverURL: fmt.Sprint(item["cover_url"]), SourceURL: fmt.Sprint(item["source_url"]), AuthorID: fmt.Sprint(item["author_id"]), AuthorName: fmt.Sprint(item["author_name"]), SourceType: sourceType, PublishedAt: parsePublishedAt(item["published_at"]), RawJSON: mustJSON(item)})
			if errors.Is(err, ErrDuplicate) {
				task.Stats.Duplicate++
			} else if err != nil {
				task.Stats.Failed++
			} else {
				task.Stats.Added++
			}
		}
	}
	now := s.now()
	task.FinishedAt, task.UpdatedAt = &now, now
	if crawlErr != nil {
		task.Status, task.Error = model.CrawlFailed, crawlErr.Error()
	} else {
		task.Status = model.CrawlSuccess
	}
	_, err := s.store.UpdateCrawlTask(task)
	return err
}

func (s *discoveryService) runDue(now time.Time) int {
	strategies, err := s.store.ListStrategies(nil)
	if err != nil {
		return 0
	}
	count := 0
	for _, strategy := range strategies {
		location := time.Local
		if strategy.Timezone != "" {
			if loaded, e := time.LoadLocation(strategy.Timezone); e == nil {
				location = loaded
			}
		}
		localNow := now.In(location)
		if strategy.Status != model.StrategyEnabled || !scheduleDue(strategy.Schedule, localNow) {
			continue
		}
		previous, _ := s.store.ListCrawlTasks(&strategy.TeamID, &strategy.ID)
		if len(previous) > 0 && sameScheduleWindow(previous[0].CreatedAt.In(location), strategy.Schedule, localNow) {
			continue
		}
		if _, err := s.createRun(identityservice.PublicUser{ID: strategy.CreatedBy, Role: identityservice.RoleAdmin}, strategy.ID, scheduleWindowKey(strategy.Schedule, localNow)); err == nil {
			count++
		}
	}
	return count
}

func (s *discoveryService) listCrawlTasks(actor identityservice.PublicUser, strategyID *int64) ([]model.CrawlTask, error) {
	team, err := s.scope(actor, nil)
	if err != nil {
		return nil, err
	}
	return s.store.ListCrawlTasks(team, strategyID)
}
func (s *discoveryService) getCrawlTask(actor identityservice.PublicUser, id int64) (model.CrawlTask, bool, error) {
	item, ok, err := s.store.FindCrawlTask(id)
	if err != nil || !ok {
		return item, ok, err
	}
	if _, err = s.scope(actor, &item.TeamID); err != nil {
		return model.CrawlTask{}, false, err
	}
	return item, true, nil
}

func (s *discoveryService) confirmResults(actor identityservice.PublicUser, id int64, ids []string) (model.CrawlTask, error) {
	if len(ids) == 0 || len(ids) > 500 {
		return model.CrawlTask{}, ErrDiscoverySelection
	}
	task, ok, err := s.store.FindCrawlTask(id)
	if err != nil {
		return model.CrawlTask{}, err
	}
	if !ok {
		return model.CrawlTask{}, ErrCrawlTaskNotFound
	}
	if _, err = s.scope(actor, &task.TeamID); err != nil {
		return model.CrawlTask{}, err
	}
	if task.Status != model.CrawlSuccess || len(task.Results) == 0 {
		return model.CrawlTask{}, ErrDiscoverySelection
	}
	selected := map[string]struct{}{}
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" {
			selected[id] = struct{}{}
		}
	}
	if len(selected) == 0 {
		return model.CrawlTask{}, ErrDiscoverySelection
	}
	sourceType := map[string]string{"keyword": "search", "author": "author"}[fmt.Sprint(task.Snapshot["operation"])]
	if sourceType == "" {
		return model.CrawlTask{}, ErrDiscoverySelection
	}
	matched := 0
	team := task.TeamID
	for _, item := range task.Results {
		contentID := strings.TrimSpace(fmt.Sprint(item["platform_content_id"]))
		if _, exists := selected[contentID]; !exists {
			continue
		}
		matched++
		if promoted, _ := item["promoted"].(bool); promoted {
			continue
		}
		_, createErr := s.content.createSource(actor, dto.SourceInput{TeamID: &team, Platform: task.Platform, PlatformContentID: contentID, Title: fmt.Sprint(item["title"]), Description: fmt.Sprint(item["description"]), CoverURL: fmt.Sprint(item["cover_url"]), SourceURL: fmt.Sprint(item["source_url"]), AuthorID: fmt.Sprint(item["author_id"]), AuthorName: fmt.Sprint(item["author_name"]), SourceType: sourceType, PublishedAt: parsePublishedAt(item["published_at"]), RawJSON: mustJSON(item)})
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
		return model.CrawlTask{}, ErrDiscoverySelection
	}
	task.UpdatedAt = s.now()
	_, err = s.store.UpdateCrawlTask(task)
	return task, err
}

func actorForTask(task model.CrawlTask) identityservice.PublicUser {
	team := task.TeamID
	return identityservice.PublicUser{ID: task.CreatedBy, Role: identityservice.RoleOperator, TeamID: &team}
}
func cloneMap(input map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range input {
		out[key] = value
	}
	return out
}
func mustJSON(value any) json.RawMessage { raw, _ := json.Marshal(value); return raw }
func parsePublishedAt(value any) *time.Time {
	text := strings.TrimSpace(fmt.Sprint(value))
	parsed, err := time.Parse(time.RFC3339, text)
	if err != nil {
		return nil
	}
	return &parsed
}
func anyStringSlice(value any) []string {
	switch values := value.(type) {
	case []string:
		return values
	case []any:
		out := make([]string, 0, len(values))
		for _, value := range values {
			if text := strings.TrimSpace(fmt.Sprint(value)); text != "" {
				out = append(out, text)
			}
		}
		return out
	}
	return nil
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
func scheduleWindowKey(schedule string, now time.Time) string {
	value := strings.TrimSpace(schedule)
	if strings.HasPrefix(value, "daily ") {
		return "daily:" + now.Format("2006-01-02") + ":" + strings.TrimSpace(strings.TrimPrefix(value, "daily "))
	}
	if strings.HasPrefix(value, "interval:") {
		if minutes, err := strconv.Atoi(strings.TrimPrefix(value, "interval:")); err == nil && minutes > 0 {
			return fmt.Sprintf("interval:%d:%d", minutes, now.Unix()/int64(minutes*60))
		}
	}
	return ""
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
		hour, e1 := strconv.Atoi(parts[0])
		minute, e2 := strconv.Atoi(parts[1])
		return e1 == nil && e2 == nil && hour == now.Hour() && minute == now.Minute()
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
