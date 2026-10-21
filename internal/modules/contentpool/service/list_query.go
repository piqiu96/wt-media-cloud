package service

import (
	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool/model"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
)

// StrategyListQuery and CrawlTaskListQuery are the read filters pushed into the
// repository. They stay here (not in repository) so the service interface does
// not leak the persistence package; the MySQL adapter translates them.
type StrategyListQuery struct {
	Search string
	Type   string
	Status string
}

type CrawlTaskListQuery struct {
	Search     string
	Status     string
	StrategyID *int64
}

func ListStrategiesFiltered(actor identityservice.PublicUser, query StrategyListQuery) ([]model.DiscoveryStrategy, error) {
	return defaultDiscoveryService().listStrategiesFiltered(actor, query)
}

func ListCrawlTasksFiltered(actor identityservice.PublicUser, query CrawlTaskListQuery) ([]model.CrawlTask, error) {
	return defaultDiscoveryService().listCrawlTasksFiltered(actor, query)
}
