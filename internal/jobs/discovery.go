// Package jobs contains business work invoked by scheduler and worker processes.
package jobs

import (
	"context"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool/service"
)

// RunDiscoverySchedule only creates pending tasks. It does not invoke Douyin.
func RunDiscoverySchedule(_ context.Context) error {
	service.RunDue(time.Now().UTC())
	return nil
}

// RunDiscoveryWorker claims and synchronously executes at most limit tasks.
func RunDiscoveryWorker(ctx context.Context, limit int) error {
	_, err := DrainDiscoveryWorker(ctx, limit)
	return err
}

func DrainDiscoveryWorker(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 10
	}
	for processed := 0; processed < limit; processed++ {
		claimed, err := service.RunNext(ctx)
		if err != nil || !claimed {
			return processed, err
		}
	}
	return limit, nil
}
