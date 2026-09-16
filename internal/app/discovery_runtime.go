package app

import (
	"database/sql"
	"fmt"

	"github.com/wt-media/wt-media-cloud/internal/infra/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool"
)

// DiscoveryRuntime is the shared Cloud-owned assembly used by the HTTP
// server and the standalone scheduler/worker commands.
type DiscoveryRuntime struct {
	DB      *sql.DB
	Service *contentpool.DiscoveryService
}

func NewDiscoveryRuntime() (*DiscoveryRuntime, error) {
	cfg := config.Load()
	if cfg.MySQLDSN == "" {
		return nil, database.ErrMissingMySQLDSN
	}
	db, err := database.OpenMySQL(cfg.MySQLDSN)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	contentService := contentpool.NewService(contentpool.NewMySQLStore(db))
	service := contentpool.NewDiscoveryService(contentpool.NewMySQLDiscoveryStore(db), contentService, contentpool.NewDouyinCrawlerFromEnv())
	return &DiscoveryRuntime{DB: db, Service: service}, nil
}

func (r *DiscoveryRuntime) Close() error {
	if r == nil || r.DB == nil {
		return nil
	}
	return r.DB.Close()
}
