package service

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool/repository"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
)

type mysqlContentStore struct{}

func (mysqlContentStore) CreateSource(v model.SourceContent, raw json.RawMessage) (model.SourceContent, error) {
	item, err := repository.CreateSource(v, raw)
	if err != nil && strings.HasPrefix(err.Error(), ErrDuplicate.Error()) {
		return model.SourceContent{}, ErrDuplicate
	}
	return item, err
}
func (mysqlContentStore) ListSources(v dto.Filter) ([]model.SourceContentView, error) {
	return repository.ListSources(repository.Filter{
		TeamID:      v.TeamID,
		Platform:    v.Platform,
		Status:      v.Status,
		SourceType:  v.SourceType,
		Search:      v.Search,
		StrategyID:  v.StrategyID,
		CrawlTaskID: v.CrawlTaskID,
		MaterialID:  v.MaterialID,
	})
}
func (mysqlContentStore) FindSource(id int64) (model.SourceContentView, bool, error) {
	return repository.FindSource(id)
}
func (mysqlContentStore) UpdateStatus(id int64, status model.Status, reason string, auditNote string, actorID identityservice.UserID, now time.Time) (model.SourceContent, error) {
	return repository.UpdateStatus(id, status, reason, auditNote, actorID, now)
}
func (mysqlContentStore) RecordMaterialFailure(id int64, reason string) (model.SourceContent, error) {
	return repository.RecordMaterialFailure(id, reason)
}
func (mysqlContentStore) Materialize(id int64, userID identityservice.UserID, now time.Time) (model.Material, error) {
	return repository.Materialize(id, int64(userID), now)
}

type mysqlDiscoveryStore struct{}

func (mysqlDiscoveryStore) CreateStrategy(v model.DiscoveryStrategy) (model.DiscoveryStrategy, error) {
	return repository.CreateStrategy(v)
}
func (mysqlDiscoveryStore) ListStrategies(team *identityservice.TeamID) ([]model.DiscoveryStrategy, error) {
	return repository.ListStrategies(team)
}
func (mysqlDiscoveryStore) FindStrategy(id int64) (model.DiscoveryStrategy, bool, error) {
	return repository.FindStrategy(id)
}
func (mysqlDiscoveryStore) UpdateStrategy(v model.DiscoveryStrategy) (model.DiscoveryStrategy, error) {
	return repository.UpdateStrategy(v)
}
func (mysqlDiscoveryStore) DeleteStrategy(id int64) error {
	return repository.DeleteStrategy(id)
}
func (mysqlDiscoveryStore) CreateCrawlTask(v model.CrawlTask) (model.CrawlTask, error) {
	return repository.CreateCrawlTask(v)
}
func (mysqlDiscoveryStore) ListCrawlTasks(team *identityservice.TeamID, strategyID *int64) ([]model.CrawlTask, error) {
	return repository.ListCrawlTasks(team, strategyID)
}
func (mysqlDiscoveryStore) FindCrawlTask(id int64) (model.CrawlTask, bool, error) {
	return repository.FindCrawlTask(id)
}
func (mysqlDiscoveryStore) UpdateCrawlTask(v model.CrawlTask) (model.CrawlTask, error) {
	return repository.UpdateCrawlTask(v)
}
func (mysqlDiscoveryStore) ClaimPendingCrawlTask(now time.Time) (model.CrawlTask, bool, error) {
	return repository.ClaimPendingCrawlTask(now)
}

func defaultContentService() *contentService { return newContentService(mysqlContentStore{}) }

// defaultDiscoveryService serves the server and worker processes: both assemble
// the Douyin client in their resource plan.
func defaultDiscoveryService() *discoveryService {
	return newDiscoveryService(mysqlDiscoveryStore{}, defaultContentService(), newDouyinCrawler())
}

// defaultSchedulerDiscoveryService serves the scheduling path only. runDue reads
// the database and enqueues pending tasks without ever crawling, so the service
// must not carry a crawler: cmd/discovery-scheduler's resource plan deliberately
// assembles no clients, and douyinclient.Get() panics before Initialize.
func defaultSchedulerDiscoveryService() *discoveryService {
	return newDiscoveryService(mysqlDiscoveryStore{}, defaultContentService(), nil)
}
