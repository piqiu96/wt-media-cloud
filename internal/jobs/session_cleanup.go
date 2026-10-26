package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/logger"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
)

type deleteSessionBatchFunc func(context.Context, time.Time, int) (int64, error)

// RunSessionCleanup 删除过期超过保留期的会话行。
// 保留期按过期时间算，未过期的失效行不动。
func RunSessionCleanup(ctx context.Context) error {
	schedulerConfig := config.Get().Scheduler
	cutoff := time.Now().UTC().Add(-schedulerConfig.SessionCleanupRetention.Duration)
	deleted, err := cleanupExpiredSessions(ctx, cutoff, schedulerConfig.SessionCleanupBatchSize, service.DeleteExpiredSessionsBefore)
	if err != nil {
		// 调度器会丢弃作业错误，不在这里记就没人看得见。
		logger.Job().Errorf("session cleanup failed module=%s error=%v", "jobs.session_cleanup", err)
		return err
	}
	if deleted > 0 {
		logger.Job().Infof(
			"session cleanup finished module=%s deleted=%d cutoff=%s",
			"jobs.session_cleanup",
			deleted,
			cutoff.Format(time.RFC3339),
		)
	}
	return nil
}

func cleanupExpiredSessions(ctx context.Context, cutoff time.Time, batchSize int, deleteBatch deleteSessionBatchFunc) (int64, error) {
	if batchSize <= 0 {
		return 0, fmt.Errorf("session cleanup batch size must be greater than zero, got %d", batchSize)
	}
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		deleted, err := deleteBatch(ctx, cutoff, batchSize)
		if err != nil {
			return total, fmt.Errorf("delete expired sessions: %w", err)
		}
		total += deleted
		if deleted < int64(batchSize) {
			return total, nil
		}
	}
}
