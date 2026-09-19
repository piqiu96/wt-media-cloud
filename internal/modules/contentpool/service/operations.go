package service

import (
	"context"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool/model"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
)

func CreateSource(actor identityservice.PublicUser, input dto.SourceInput) (model.SourceContent, error) {
	return defaultContentService().createSource(actor, input)
}
func List(actor identityservice.PublicUser, filter dto.Filter) ([]model.SourceContent, error) {
	return defaultContentService().list(actor, filter)
}
func Get(actor identityservice.PublicUser, id int64) (model.SourceContent, bool, error) {
	return defaultContentService().get(actor, id)
}
func SetStatus(actor identityservice.PublicUser, id int64, status model.Status, reason string) (model.SourceContent, error) {
	return defaultContentService().setStatus(actor, id, status, reason)
}
func BatchSetStatus(actor identityservice.PublicUser, ids []int64, status model.Status, reason string) ([]model.SourceContent, error) {
	return defaultContentService().batchSetStatus(actor, ids, status, reason)
}
func Materialize(actor identityservice.PublicUser, id int64) (model.Material, error) {
	return defaultContentService().materialize(actor, id)
}
func ListStrategies(actor identityservice.PublicUser) ([]model.DiscoveryStrategy, error) {
	return defaultDiscoveryService().listStrategies(actor)
}
func CreateStrategy(actor identityservice.PublicUser, input model.DiscoveryStrategy) (model.DiscoveryStrategy, error) {
	return defaultDiscoveryService().createStrategy(actor, input)
}
func SetStrategyStatus(actor identityservice.PublicUser, id int64, status model.StrategyStatus) (model.DiscoveryStrategy, error) {
	return defaultDiscoveryService().setStrategyStatus(actor, id, status)
}
func UpdateStrategy(actor identityservice.PublicUser, id int64, input model.DiscoveryStrategy) (model.DiscoveryStrategy, error) {
	return defaultDiscoveryService().updateStrategy(actor, id, input)
}
func CreateRun(actor identityservice.PublicUser, strategyID int64) (model.CrawlTask, error) {
	return defaultDiscoveryService().createRun(actor, strategyID, "")
}
func CreateManualRun(actor identityservice.PublicUser, platform, operation string, config map[string]any) (model.CrawlTask, error) {
	return defaultDiscoveryService().createManualRun(actor, platform, operation, config)
}
func RunDue(now time.Time) int                  { return defaultDiscoveryService().runDue(now) }
func RunNext(ctx context.Context) (bool, error) { return defaultDiscoveryService().runNext(ctx) }
func ListCrawlTasks(actor identityservice.PublicUser, strategyID *int64) ([]model.CrawlTask, error) {
	return defaultDiscoveryService().listCrawlTasks(actor, strategyID)
}
func GetCrawlTask(actor identityservice.PublicUser, id int64) (model.CrawlTask, bool, error) {
	return defaultDiscoveryService().getCrawlTask(actor, id)
}
func ConfirmResults(actor identityservice.PublicUser, id int64, ids []string) (model.CrawlTask, error) {
	return defaultDiscoveryService().confirmResults(actor, id, ids)
}

func Search(ctx context.Context, input dto.SearchInput) (dto.SearchResponse, error) {
	return searchWithClient(ctx, input, douyinClient())
}

func FindAuthor(ctx context.Context, input dto.AuthorSearchInput) (dto.SearchResponse, error) {
	return findAuthorWithClient(ctx, input, douyinClient())
}

func ImportResults(actor identityservice.PublicUser, input dto.ImportResultsRequest) (dto.ImportResultsResponse, error) {
	return defaultContentService().importResults(actor, input)
}
