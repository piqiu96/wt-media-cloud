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
	StrategyEnabled     = model.StrategyEnabled
	StrategyDisabled    = model.StrategyDisabled
	CrawlPending        = model.CrawlPending
	CrawlRunning        = model.CrawlRunning
	CrawlSuccess        = model.CrawlSuccess
	CrawlPartialSuccess = model.CrawlPartialSuccess
	CrawlFailed         = model.CrawlFailed
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
	DeleteStrategy(int64) error
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

func normalizeMaterialConfig(config map[string]any) bool {
	if config == nil {
		return false
	}
	autoMaterial := boolValue(config["auto_material"])
	rule := "AND"
	if value, exists := config["material_rule"]; exists {
		if text := strings.ToUpper(strings.TrimSpace(fmt.Sprint(value))); text != "" && text != "<NIL>" {
			rule = text
		}
	}
	if rule != "AND" && rule != "OR" {
		return false
	}
	likeThreshold := int64Value(config["like_threshold"])
	favoriteThreshold := int64Value(config["favorite_threshold"])
	if autoMaterial && likeThreshold <= 0 && favoriteThreshold <= 0 {
		return false
	}
	config["auto_material"] = autoMaterial
	config["material_rule"] = rule
	config["like_threshold"] = likeThreshold
	config["favorite_threshold"] = favoriteThreshold
	return true
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
	if !validStrategy(input) || !normalizeMaterialConfig(input.Config) {
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
	if !validStrategy(input) || !normalizeMaterialConfig(input.Config) {
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
	current.GameID = input.GameID
	return s.store.UpdateStrategy(current)
}

func (s *discoveryService) deleteStrategy(actor identityservice.PublicUser, id int64) error {
	item, ok, err := s.store.FindStrategy(id)
	if err != nil {
		return err
	}
	if !ok {
		return ErrStrategyNotFound
	}
	if _, err = s.scope(actor, &item.TeamID); err != nil {
		return err
	}
	return s.store.DeleteStrategy(id)
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
	if strategy.Status != model.StrategyEnabled || !validStrategy(strategy) || !normalizeMaterialConfig(strategy.Config) {
		return model.CrawlTask{}, ErrDiscoveryInvalid
	}
	now := s.now()
	snapshot := cloneMap(strategy.Config)
	snapshot["strategy_id"] = strategy.ID
	snapshot["strategy_name"] = strategy.Name
	snapshot["strategy_type"] = strategy.StrategyType
	snapshot["platform"] = strategy.Platform
	snapshot["schedule"] = strategy.Schedule
	snapshot["operation"] = strategy.StrategyType
	snapshot["game_id"] = strategy.GameID
	return s.store.CreateCrawlTask(model.CrawlTask{TeamID: strategy.TeamID, StrategyID: &strategy.ID, ScheduleKey: scheduleKey, TaskType: "discovery_task", Platform: strategy.Platform, Status: model.CrawlPending, Snapshot: snapshot, CreatedBy: actor.ID, CreatedAt: now, UpdatedAt: now})
}

func (s *discoveryService) createManualRun(actor identityservice.PublicUser, platform, operation string, config map[string]any) (model.CrawlTask, error) {
	return s.createManualRunWithTeam(actor, nil, platform, operation, config)
}

func (s *discoveryService) createManualRunWithTeam(actor identityservice.PublicUser, requested *identityservice.TeamID, platform, operation string, config map[string]any) (model.CrawlTask, error) {
	team, err := s.scope(actor, requested)
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
	if operation != "url" && operation != "keyword" && operation != "author" {
		return true, s.failClaimedTask(task, ErrDiscoveryInvalid)
	}
	if task.TaskType == "retry_failed_task" {
		return true, s.executeRetryClaimed(ctx, task, operation)
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
	task.Stats = model.CrawlStats{Scanned: scanned, Found: len(result.Items)}
	if manual && (operation == "keyword" || operation == "author") {
		task.Results = dedupeResultItems(result.Items)
		for _, item := range task.Results {
			item["processing_status"] = "unprocessed"
		}
		task.Stats.Found = len(task.Results)
	} else {
		sourceType := "strategy"
		if operation == "url" {
			sourceType = "link"
		}
		task.Results = make([]map[string]any, 0, len(result.Items)+len(result.Failures))
		for _, rawItem := range result.Items {
			item := cloneMap(rawItem)
			task.Results = append(task.Results, item)
			s.projectDiscoveredItem(&task, item, sourceType)
		}
		for _, failure := range result.Failures {
			item := cloneMap(failure)
			item["processing_status"] = "failed"
			task.Results = append(task.Results, item)
			task.Stats.Failed++
		}
		task.Stats.Found = len(result.Items)
	}
	now := s.now()
	task.FinishedAt, task.UpdatedAt = &now, now
	task.Status = crawlStatusFromStats(task.Stats, crawlErr)
	if crawlErr != nil {
		task.Error = crawlErr.Error()
	}
	_, err := s.store.UpdateCrawlTask(task)
	return err
}

func crawlStatusFromStats(stats model.CrawlStats, crawlErr error) model.CrawlStatus {
	processed := stats.Added + stats.Duplicate + stats.Pending + stats.AutoMaterialized
	if crawlErr != nil {
		if processed > 0 {
			return model.CrawlPartialSuccess
		}
		return model.CrawlFailed
	}
	if stats.Failed > 0 {
		if processed > 0 {
			return model.CrawlPartialSuccess
		}
		return model.CrawlFailed
	}
	return model.CrawlSuccess
}

func (s *discoveryService) projectDiscoveredItem(task *model.CrawlTask, item map[string]any, sourceType string) {
	team := task.TeamID
	taskID := task.ID
	source, err := s.content.createSource(actorForTask(*task), dto.SourceInput{
		TeamID: &team, Platform: task.Platform, PlatformContentID: fmt.Sprint(item["platform_content_id"]),
		Title: fmt.Sprint(item["title"]), Description: fmt.Sprint(item["description"]),
		CoverURL: fmt.Sprint(item["cover_url"]), SourceURL: fmt.Sprint(item["source_url"]),
		AuthorID: fmt.Sprint(item["author_id"]), AuthorSecUID: fmt.Sprint(item["author_sec_uid"]), AuthorUID: fmt.Sprint(item["author_uid"]), AuthorHomeURL: fmt.Sprint(item["author_home_url"]), AuthorName: fmt.Sprint(item["author_name"]),
		SourceType: sourceType, StrategyID: task.StrategyID, CrawlTaskID: &taskID,
		LikeCount: int64Value(item["like_count"]), FavoriteCount: int64Value(item["favorite_count"]), ViewCount: int64Value(item["view_count"]), CommentCount: int64Value(item["comment_count"]), ShareCount: int64Value(item["share_count"]),
		PublishedAt: parsePublishedAt(item["published_at"]), RawJSON: mustJSON(item), GameID: optionalString(task.Snapshot["game_id"]),
	})
	switch {
	case errors.Is(err, ErrDuplicate):
		task.Stats.Duplicate++
		item["processing_status"] = "duplicate"
	case err != nil:
		task.Stats.Failed++
		item["processing_status"] = "failed"
		item["failure_reason"] = err.Error()
	default:
		task.Stats.Added++
		item["source_content_id"] = source.ID
		if !shouldAutoMaterialize(task.Snapshot, item) {
			task.Stats.Pending++
			item["processing_status"] = "pending"
			return
		}
		material, materialErr := s.content.materialize(actorForTask(*task), source.ID)
		if materialErr != nil {
			task.Stats.Failed++
			task.Stats.Pending++
			item["processing_status"] = "material_failed"
			item["failure_reason"] = materialErr.Error()
			return
		}
		task.Stats.AutoMaterialized++
		item["processing_status"] = "auto_materialized"
		item["material_id"] = material.ID
	}
}

func (s *discoveryService) retryFailed(actor identityservice.PublicUser, id int64) (model.CrawlTask, error) {
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
	if task.Status != model.CrawlFailed && task.Status != model.CrawlPartialSuccess {
		return model.CrawlTask{}, ErrDiscoverySelection
	}
	retryItems := make([]map[string]any, 0)
	for _, item := range task.Results {
		status := fmt.Sprint(item["processing_status"])
		if status == "failed" || status == "material_failed" {
			retryItems = append(retryItems, cloneMap(item))
		}
	}
	if len(retryItems) == 0 {
		return model.CrawlTask{}, ErrDiscoverySelection
	}
	snapshot := cloneMap(task.Snapshot)
	snapshot["retry_items"] = retryItems
	now := s.now()
	parentID := task.ID
	return s.store.CreateCrawlTask(model.CrawlTask{
		TeamID: task.TeamID, StrategyID: task.StrategyID, ParentTaskID: &parentID,
		TaskType: "retry_failed_task", Platform: task.Platform, Status: model.CrawlPending,
		Snapshot: snapshot, CreatedBy: actor.ID, CreatedAt: now, UpdatedAt: now,
	})
}

func (s *discoveryService) executeRetryClaimed(ctx context.Context, task model.CrawlTask, operation string) error {
	retryItems := anyMapSlice(task.Snapshot["retry_items"])
	task.Stats = model.CrawlStats{Scanned: len(retryItems), Found: len(retryItems)}
	task.Results = make([]map[string]any, 0, len(retryItems))
	for _, input := range retryItems {
		item := cloneMap(input)
		sourceID := int64Value(item["source_content_id"])
		if sourceID > 0 && fmt.Sprint(item["processing_status"]) == "material_failed" {
			source, found, sourceErr := s.content.get(actorForTask(task), sourceID)
			if sourceErr != nil || !found {
				item["failure_reason"] = ErrNotFound.Error()
				task.Results = append(task.Results, item)
				task.Stats.Failed++
				continue
			}
			thresholdItem := map[string]any{"like_count": source.LikeCount, "favorite_count": source.FavoriteCount}
			if !shouldAutoMaterialize(task.Snapshot, thresholdItem) {
				item["processing_status"] = "pending"
				delete(item, "failure_reason")
				task.Results = append(task.Results, item)
				task.Stats.Pending++
				continue
			}
			material, materialErr := s.content.materialize(actorForTask(task), sourceID)
			if materialErr != nil {
				item["failure_reason"] = materialErr.Error()
				task.Results = append(task.Results, item)
				task.Stats.Failed++
				continue
			}
			item["processing_status"] = "auto_materialized"
			item["material_id"] = material.ID
			delete(item, "failure_reason")
			task.Results = append(task.Results, item)
			task.Stats.AutoMaterialized++
			continue
		}
		if sourceID > 0 {
			task.Results = append(task.Results, item)
			task.Stats.Failed++
			continue
		}
		key := strings.TrimSpace(fmt.Sprint(item["failure_key"]))
		if key == "" {
			item["failure_reason"] = "failure key is empty"
			task.Results = append(task.Results, item)
			task.Stats.Failed++
			continue
		}
		config := map[string]any{"url": key}
		if operation == "keyword" {
			config = map[string]any{"keyword": key, "limit": intValue(task.Snapshot["limit"], 20)}
		}
		retryResult, retryErr := s.crawler.Discover(ctx, dto.CrawlerRequest{Platform: task.Platform, Operation: operation, Config: config})
		if retryErr != nil || len(retryResult.Items) == 0 {
			if retryErr != nil {
				item["failure_reason"] = retryErr.Error()
			} else {
				item["failure_reason"] = "retry returned no items"
			}
			task.Results = append(task.Results, item)
			task.Stats.Failed++
			continue
		}
		sourceType := "strategy"
		if operation == "url" {
			sourceType = "link"
		}
		for _, rawItem := range retryResult.Items {
			retried := cloneMap(rawItem)
			task.Results = append(task.Results, retried)
			s.projectDiscoveredItem(&task, retried, sourceType)
		}
	}
	task.Stats.Found = len(task.Results)
	now := s.now()
	task.Status = crawlStatusFromStats(task.Stats, nil)
	task.FinishedAt, task.UpdatedAt = &now, now
	_, err := s.store.UpdateCrawlTask(task)
	return err
}

func optionalString(value any) *string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return nil
	}
	return &text
}

func shouldAutoMaterialize(snapshot map[string]any, item map[string]any) bool {
	if !boolValue(snapshot["auto_material"]) {
		return false
	}
	likeThreshold := int64Value(snapshot["like_threshold"])
	favoriteThreshold := int64Value(snapshot["favorite_threshold"])
	conditions := make([]bool, 0, 2)
	if likeThreshold > 0 {
		conditions = append(conditions, int64Value(item["like_count"]) >= likeThreshold)
	}
	if favoriteThreshold > 0 {
		conditions = append(conditions, int64Value(item["favorite_count"]) >= favoriteThreshold)
	}
	if len(conditions) == 0 {
		return false
	}
	if strings.EqualFold(fmt.Sprint(snapshot["material_rule"]), "OR") {
		for _, passed := range conditions {
			if passed {
				return true
			}
		}
		return false
	}
	for _, passed := range conditions {
		if !passed {
			return false
		}
	}
	return true
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
		_, createErr := s.content.createSource(actor, dto.SourceInput{TeamID: &team, Platform: task.Platform, PlatformContentID: contentID, Title: fmt.Sprint(item["title"]), Description: fmt.Sprint(item["description"]), CoverURL: fmt.Sprint(item["cover_url"]), SourceURL: fmt.Sprint(item["source_url"]), AuthorID: fmt.Sprint(item["author_id"]), AuthorSecUID: fmt.Sprint(item["author_sec_uid"]), AuthorUID: fmt.Sprint(item["author_uid"]), AuthorHomeURL: fmt.Sprint(item["author_home_url"]), AuthorName: fmt.Sprint(item["author_name"]), SourceType: sourceType, LikeCount: int64Value(item["like_count"]), FavoriteCount: int64Value(item["favorite_count"]), ViewCount: int64Value(item["view_count"]), CommentCount: int64Value(item["comment_count"]), ShareCount: int64Value(item["share_count"]), PublishedAt: parsePublishedAt(item["published_at"]), RawJSON: mustJSON(item), GameID: optionalString(task.Snapshot["game_id"])})
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
func anyMapSlice(value any) []map[string]any {
	switch values := value.(type) {
	case []map[string]any:
		return values
	case []any:
		out := make([]map[string]any, 0, len(values))
		for _, value := range values {
			if item, ok := value.(map[string]any); ok {
				out = append(out, item)
			}
		}
		return out
	}
	return nil
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
