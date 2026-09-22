package contentpool

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool/model"
	contentservice "github.com/wt-media/wt-media-cloud/internal/modules/contentpool/service"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

type sourceRequest struct {
	TeamID            *identityservice.TeamID `json:"team_id"`
	Platform          string                  `json:"platform"`
	PlatformContentID string                  `json:"platform_content_id"`
	Title             string                  `json:"title"`
	Description       string                  `json:"description"`
	CoverURL          string                  `json:"cover_url"`
	SourceURL         string                  `json:"source_url"`
	AuthorID          string                  `json:"author_id"`
	AuthorName        string                  `json:"author_name"`
	SourceType        string                  `json:"source_type"`
	PublishedAt       string                  `json:"published_at"`
	RawJSON           json.RawMessage         `json:"raw_json"`
}
type statusRequest struct {
	Status    model.Status `json:"status"`
	Reason    string       `json:"reason"`
	AuditNote string       `json:"audit_note"`
}
type batchStatusRequest struct {
	IDs       []int64      `json:"ids"`
	Status    model.Status `json:"status"`
	Reason    string       `json:"reason"`
	AuditNote string       `json:"audit_note"`
}
type strategyRequest struct {
	TeamID       *identityservice.TeamID `json:"team_id"`
	Name         string                  `json:"name"`
	StrategyType string                  `json:"strategy_type"`
	Platform     string                  `json:"platform"`
	Config       map[string]any          `json:"config"`
	Schedule     string                  `json:"schedule"`
	Timezone     string                  `json:"timezone"`
	Status       model.StrategyStatus    `json:"status"`
}
type manualSearchRequest struct {
	TeamID    *identityservice.TeamID `json:"team_id"`
	Platform  string                  `json:"platform"`
	Keyword   string                  `json:"keyword"`
	Author    string                  `json:"author"`
	URL       string                  `json:"url"`
	URLs      []string                `json:"urls"`
	Limit     int                     `json:"limit"`
	Offset    int                     `json:"offset"`
	MaxCursor int64                   `json:"max_cursor"`
}

func actor(c *hertzapp.RequestContext) (identityservice.PublicUser, bool) {
	return middleware.AuthenticateRequest(c)
}
func contentID(c *hertzapp.RequestContext, name, message string) (int64, bool) {
	value, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || value <= 0 {
		api.BadRequest(c, 10001, message)
		return 0, false
	}
	return value, true
}

func ListContent(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	var team *identityservice.TeamID
	if raw := strings.TrimSpace(c.Query("team_id")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value <= 0 {
			api.BadRequest(c, 10001, "团队参数无效")
			return
		}
		teamValue := identityservice.TeamID(value)
		team = &teamValue
	}
	filter := dto.Filter{TeamID: team, Platform: c.Query("platform"), Status: model.Status(c.Query("status")), SourceType: c.Query("source_type"), Search: c.Query("search")}
	for name, target := range map[string]**int64{"strategy_id": &filter.StrategyID, "crawl_task_id": &filter.CrawlTaskID, "material_id": &filter.MaterialID} {
		if raw := strings.TrimSpace(c.Query(name)); raw != "" {
			value, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || value <= 0 {
				api.BadRequest(c, 10001, "来源参数无效")
				return
			}
			*target = &value
		}
	}
	items, err := contentservice.List(actor, filter)
	if err != nil {
		writeContentError(c, err)
		return
	}
	api.Success(c, items)
}
func GetContent(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	id, ok := contentID(c, "id", "内容 ID 无效")
	if !ok {
		return
	}
	item, found, err := contentservice.Get(actor, id)
	if err != nil {
		writeContentError(c, err)
		return
	}
	if !found {
		api.NotFound(c, 14001, "内容不存在")
		return
	}
	api.Success(c, item)
}
func CreateContent(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	var request sourceRequest
	if !api.DecodeJSON(c, &request) {
		return
	}
	var published *time.Time
	if request.PublishedAt != "" {
		value, err := time.Parse(time.RFC3339, request.PublishedAt)
		if err != nil {
			api.BadRequest(c, 10001, "发布时间格式无效")
			return
		}
		published = &value
	}
	item, err := contentservice.CreateSource(actor, dto.SourceInput{TeamID: request.TeamID, Platform: request.Platform, PlatformContentID: request.PlatformContentID, Title: request.Title, Description: request.Description, CoverURL: request.CoverURL, SourceURL: request.SourceURL, AuthorID: request.AuthorID, AuthorName: request.AuthorName, SourceType: request.SourceType, PublishedAt: published, RawJSON: request.RawJSON})
	if err != nil {
		writeContentError(c, err)
		return
	}
	api.Created(c, item)
}
func BatchUpdateContentStatus(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	var request batchStatusRequest
	if !api.DecodeJSON(c, &request) {
		return
	}
	items, err := contentservice.BatchSetStatus(actor, request.IDs, request.Status, request.Reason, request.AuditNote)
	if err != nil {
		writeContentError(c, err)
		return
	}
	api.Success(c, items)
}
func UpdateContentStatus(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	id, ok := contentID(c, "id", "内容 ID 无效")
	if !ok {
		return
	}
	var request statusRequest
	if !api.DecodeJSON(c, &request) {
		return
	}
	item, err := contentservice.SetStatus(actor, id, request.Status, request.Reason, request.AuditNote)
	if err != nil {
		writeContentError(c, err)
		return
	}
	api.Success(c, item)
}
func BatchMaterializeContent(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	var request struct {
		IDs []int64 `json:"ids"`
	}
	if !api.DecodeJSON(c, &request) {
		return
	}
	api.Success(c, contentservice.BatchMaterialize(actor, request.IDs))
}
func MaterializeContent(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	id, ok := contentID(c, "id", "内容 ID 无效")
	if !ok {
		return
	}
	item, err := contentservice.Materialize(actor, id)
	if err != nil {
		writeContentError(c, err)
		return
	}
	api.Created(c, item)
}
func RunDueDiscovery(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	if actor.Role != identityservice.RoleAdmin {
		api.Forbidden(c, 11003, "没有权限执行调度")
		return
	}
	now := time.Now().UTC()
	api.Success(c, map[string]any{"at": now, "triggered": contentservice.RunDue(now)})
}
func SearchContent(ctx context.Context, c *hertzapp.RequestContext) {
	if _, ok := actor(c); !ok {
		return
	}
	var request manualSearchRequest
	if !api.DecodeJSON(c, &request) {
		return
	}
	result, err := contentservice.Search(ctx, dto.SearchInput{Platform: request.Platform, Keyword: request.Keyword, Limit: request.Limit, Offset: request.Offset})
	if err != nil {
		writeDiscoveryError(c, err)
		return
	}
	api.Success(c, result)
}
func AuthorSearchContent(ctx context.Context, c *hertzapp.RequestContext) {
	if _, ok := actor(c); !ok {
		return
	}
	var request manualSearchRequest
	if !api.DecodeJSON(c, &request) {
		return
	}
	result, err := contentservice.FindAuthor(ctx, dto.AuthorSearchInput{Platform: request.Platform, Author: request.Author, Limit: request.Limit, MaxCursor: request.MaxCursor})
	if err != nil {
		writeDiscoveryError(c, err)
		return
	}
	api.Success(c, result)
}
func ImportSearchResults(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	var request dto.ImportResultsRequest
	if !api.DecodeJSON(c, &request) {
		return
	}
	result, err := contentservice.ImportResults(actor, request)
	if err != nil {
		writeImportResultsError(c, err)
		return
	}
	api.Success(c, result)
}
func ImportContentURL(_ context.Context, c *hertzapp.RequestContext) { createURLImportTask(c) }
func createURLImportTask(c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	var request manualSearchRequest
	if !api.DecodeJSON(c, &request) {
		return
	}
	if request.Limit <= 0 {
		request.Limit = 20
	}
	config := map[string]any{"limit": request.Limit, "offset": request.Offset}
	urls := append([]string{}, request.URLs...)
	if strings.TrimSpace(request.URL) != "" {
		urls = append([]string{strings.TrimSpace(request.URL)}, urls...)
	}
	if len(urls) == 0 {
		writeDiscoveryError(c, contentservice.ErrDiscoveryInvalid)
		return
	}
	config["url"] = urls[0]
	if len(urls) > 1 {
		config["urls"] = urls
	}
	item, err := contentservice.CreateManualRunWithTeam(actor, request.TeamID, request.Platform, "url", config)
	if err != nil {
		writeDiscoveryError(c, err)
		return
	}
	api.Created(c, item)
}
func ListDiscoveryStrategies(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	items, err := contentservice.ListStrategies(actor)
	if err != nil {
		writeDiscoveryError(c, err)
		return
	}
	api.Success(c, items)
}
func CreateDiscoveryStrategy(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	var request strategyRequest
	if !api.DecodeJSON(c, &request) {
		return
	}
	team := identityservice.TeamID(0)
	if request.TeamID != nil {
		team = *request.TeamID
	} else if actor.TeamID != nil {
		team = *actor.TeamID
	}
	item, err := contentservice.CreateStrategy(actor, model.DiscoveryStrategy{TeamID: team, Name: request.Name, StrategyType: request.StrategyType, Platform: request.Platform, Config: request.Config, Schedule: request.Schedule, Timezone: request.Timezone, Status: request.Status})
	if err != nil {
		writeDiscoveryError(c, err)
		return
	}
	api.Created(c, item)
}
func UpdateDiscoveryStrategyStatus(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	id, ok := contentID(c, "id", "策略 ID 无效")
	if !ok {
		return
	}
	var request struct {
		Status model.StrategyStatus `json:"status"`
	}
	if !api.DecodeJSON(c, &request) {
		return
	}
	item, err := contentservice.SetStrategyStatus(actor, id, request.Status)
	if err != nil {
		writeDiscoveryError(c, err)
		return
	}
	api.Success(c, item)
}
func UpdateDiscoveryStrategy(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	id, ok := contentID(c, "id", "策略 ID 无效")
	if !ok {
		return
	}
	var request strategyRequest
	if !api.DecodeJSON(c, &request) {
		return
	}
	item, err := contentservice.UpdateStrategy(actor, id, model.DiscoveryStrategy{Name: request.Name, StrategyType: request.StrategyType, Platform: request.Platform, Config: request.Config, Schedule: request.Schedule, Timezone: request.Timezone, Status: request.Status})
	if err != nil {
		writeDiscoveryError(c, err)
		return
	}
	api.Success(c, item)
}
func RunDiscoveryStrategy(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	id, ok := contentID(c, "id", "策略 ID 无效")
	if !ok {
		return
	}
	item, err := contentservice.CreateRun(actor, id)
	if err != nil {
		writeDiscoveryError(c, err)
		return
	}
	api.Created(c, item)
}
func ListCrawlTasks(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	var strategyID *int64
	if raw := strings.TrimSpace(c.Query("strategy_id")); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			api.BadRequest(c, 10001, "策略 ID 无效")
			return
		}
		strategyID = &id
	}
	items, err := contentservice.ListCrawlTasks(actor, strategyID)
	if err != nil {
		writeDiscoveryError(c, err)
		return
	}
	api.Success(c, items)
}
func GetCrawlTask(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	id, ok := contentID(c, "id", "任务 ID 无效")
	if !ok {
		return
	}
	item, found, err := contentservice.GetCrawlTask(actor, id)
	if err != nil {
		writeDiscoveryError(c, err)
		return
	}
	if !found {
		api.NotFound(c, 14004, "挖掘任务不存在")
		return
	}
	api.Success(c, item)
}
func RetryFailedCrawlTask(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	id, ok := contentID(c, "id", "任务 ID 无效")
	if !ok {
		return
	}
	item, err := contentservice.RetryFailed(actor, id)
	if err != nil {
		writeDiscoveryError(c, err)
		return
	}
	api.Success(c, item)
}
func ConfirmCrawlTask(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := actor(c)
	if !ok {
		return
	}
	id, ok := contentID(c, "id", "任务 ID 无效")
	if !ok {
		return
	}
	var request struct {
		IDs []string `json:"ids"`
	}
	if !api.DecodeJSON(c, &request) {
		return
	}
	item, err := contentservice.ConfirmResults(actor, id, request.IDs)
	if err != nil {
		writeDiscoveryError(c, err)
		return
	}
	api.Success(c, item)
}
func writeContentError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, contentservice.ErrForbidden):
		api.Forbidden(c, 11003, "没有权限访问该团队内容")
	case errors.Is(err, contentservice.ErrNotFound):
		api.NotFound(c, 14001, "内容不存在")
	case errors.Is(err, contentservice.ErrInvalidInput):
		api.BadRequest(c, 14002, "内容参数无效")
	case errors.Is(err, contentservice.ErrInvalidTransition):
		api.Conflict(c, 14003, "已转素材内容不能恢复为待处理")
	case errors.Is(err, contentservice.ErrDuplicate):
		api.Conflict(c, 14004, "内容已存在")
	default:
		api.InternalError(c, "内容池服务内部错误")
	}
}
func writeDiscoveryError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, contentservice.ErrDouyinAuthorUnavailable):
		api.BadRequest(c, 14006, "博主搜索接口维护中")
	case errors.Is(err, contentservice.ErrDiscoveryForbidden):
		api.Forbidden(c, 11003, "没有权限访问该团队资源")
	case errors.Is(err, contentservice.ErrStrategyNotFound):
		api.NotFound(c, 14005, "挖掘策略不存在")
	case errors.Is(err, contentservice.ErrCrawlTaskNotFound):
		api.NotFound(c, 14004, "挖掘任务不存在")
	case errors.Is(err, contentservice.ErrDiscoveryInvalid):
		api.BadRequest(c, 14006, "挖掘参数无效")
	case errors.Is(err, contentservice.ErrStrategyDuplicate):
		api.Conflict(c, 14007, "策略名称已存在")
	case errors.Is(err, contentservice.ErrDiscoverySelection):
		api.BadRequest(c, 14008, "请选择可入池的搜索结果")
	default:
		api.InternalError(c, "挖掘服务内部错误")
	}
}
func writeImportResultsError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, contentservice.ErrForbidden), errors.Is(err, contentservice.ErrInvalidInput):
		writeContentError(c, err)
	default:
		writeDiscoveryError(c, err)
	}
}
