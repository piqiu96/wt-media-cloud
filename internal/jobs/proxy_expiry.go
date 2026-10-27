package jobs

import (
	"context"
	"fmt"
	"time"

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
		expiresAt := "未配置"
		if proxy.ExpiresAt != nil {
			expiresAt = proxy.ExpiresAt.Format(time.RFC3339)
		}
		jobLogger.Infof(
			"proxy expiry checked module=%s proxy_id=%s host=%s port=%d expires_at=%s",
			"jobs.proxy_expiry",
			proxy.ID,
			proxy.Host,
			proxy.Port,
			expiresAt,
		)
	}
	return nil
}
