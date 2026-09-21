package service

// These adapters keep focused unit tests on private, injected seams while
// production callers use package-level operations and repository getters.

import (
	"context"
	"time"

	douyinclient "github.com/wt-media/wt-media-cloud/internal/infra/client/platforms/douyin"
	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool/model"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
)

type Service = contentService
type DiscoveryService = discoveryService
type Store = contentStore
type DiscoveryStore = discoveryStore
type Crawler = crawler
type DouyinCrawler = douyinCrawler

func NewService(store Store) *Service { return newContentService(store) }
func NewDiscoveryService(store DiscoveryStore, content *Service, crawlers ...Crawler) *DiscoveryService {
	var selected Crawler
	if len(crawlers) > 0 {
		selected = crawlers[0]
	} else {
		selected = unavailableCrawler{}
	}
	return newDiscoveryService(store, content, selected)
}

type unavailableCrawler struct{}

func (unavailableCrawler) Discover(context.Context, dto.CrawlerRequest) (dto.CrawlerResult, error) {
	return dto.CrawlerResult{}, ErrCrawlerUnavailable
}
func NewDouyinCrawler() *DouyinCrawler { return newDouyinCrawler() }
func NewDouyinCrawlerWithClient(client *douyinclient.Client) *DouyinCrawler {
	return newDouyinCrawlerWithClient(client)
}

func (s *contentService) Scope(actor identityservice.PublicUser, team *identityservice.TeamID) (*identityservice.TeamID, error) {
	return s.scope(actor, team)
}
func (s *contentService) CreateSource(actor identityservice.PublicUser, input dto.SourceInput) (model.SourceContent, error) {
	return s.createSource(actor, input)
}
func (s *contentService) List(actor identityservice.PublicUser, filter dto.Filter) ([]model.SourceContent, error) {
	return s.list(actor, filter)
}
func (s *contentService) Get(actor identityservice.PublicUser, id int64) (model.SourceContent, bool, error) {
	return s.get(actor, id)
}
func (s *contentService) SetStatus(actor identityservice.PublicUser, id int64, status model.Status, reason string) (model.SourceContent, error) {
	return s.setStatus(actor, id, status, reason, "")
}
func (s *contentService) BatchSetStatus(actor identityservice.PublicUser, ids []int64, status model.Status, reason string) ([]model.SourceContent, error) {
	return s.batchSetStatus(actor, ids, status, reason, "")
}
func (s *contentService) Materialize(actor identityservice.PublicUser, id int64) (model.Material, error) {
	return s.materialize(actor, id)
}
func (s *discoveryService) SetCrawler(value Crawler) { s.crawler = value }
func (s *discoveryService) ListStrategies(actor identityservice.PublicUser) ([]model.DiscoveryStrategy, error) {
	return s.listStrategies(actor)
}
func (s *discoveryService) CreateStrategy(actor identityservice.PublicUser, input model.DiscoveryStrategy) (model.DiscoveryStrategy, error) {
	return s.createStrategy(actor, input)
}
func (s *discoveryService) SetStrategyStatus(actor identityservice.PublicUser, id int64, status model.StrategyStatus) (model.DiscoveryStrategy, error) {
	return s.setStrategyStatus(actor, id, status)
}
func (s *discoveryService) UpdateStrategy(actor identityservice.PublicUser, id int64, input model.DiscoveryStrategy) (model.DiscoveryStrategy, error) {
	return s.updateStrategy(actor, id, input)
}
func (s *discoveryService) CreateRun(actor identityservice.PublicUser, id int64) (model.CrawlTask, error) {
	return s.createRun(actor, id, "")
}
func (s *discoveryService) CreateManualRunWithTeam(actor identityservice.PublicUser, requested *identityservice.TeamID, platform, operation string, config map[string]any) (model.CrawlTask, error) {
	return s.createManualRunWithTeam(actor, requested, platform, operation, config)
}

func (s *discoveryService) CreateManualRun(actor identityservice.PublicUser, platform, operation string, config map[string]any) (model.CrawlTask, error) {
	return s.createManualRun(actor, platform, operation, config)
}
func (s *discoveryService) RunDue(now time.Time) int                  { return s.runDue(now) }
func (s *discoveryService) RunNext(ctx context.Context) (bool, error) { return s.runNext(ctx) }
func (s *discoveryService) ListCrawlTasks(actor identityservice.PublicUser, id *int64) ([]model.CrawlTask, error) {
	return s.listCrawlTasks(actor, id)
}
func (s *discoveryService) GetCrawlTask(actor identityservice.PublicUser, id int64) (model.CrawlTask, bool, error) {
	return s.getCrawlTask(actor, id)
}
func (s *discoveryService) ConfirmResults(actor identityservice.PublicUser, id int64, values []string) (model.CrawlTask, error) {
	return s.confirmResults(actor, id, values)
}
