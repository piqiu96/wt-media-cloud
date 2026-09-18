package jobs

import (
	"context"
	"fmt"

	"github.com/wt-media/wt-media-cloud/internal/infra/logger"
	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/service"
)

func RunProxyExpiry(ctx context.Context) error {
	proxies, err := service.MarkExpiringProxies(ctx)
	if err != nil {
		return fmt.Errorf("mark expiring proxies: %w", err)
	}
	jobLogger := logger.Job()
	for _, proxy := range proxies {
		jobLogger.Info("proxy expiry checked",
			"module", "jobs.proxy_expiry",
			"proxy_id", proxy.ID,
			"host", proxy.Host,
			"port", proxy.Port,
			"status", proxy.LastCheckResult,
		)
	}
	return nil
}
