package worker

import (
	"context"
	"log"
	"time"
)

type DiscoveryExecutor interface {
	RunNext(context.Context) (bool, error)
}

func DrainDiscoveryTasks(ctx context.Context, service DiscoveryExecutor, limit int) (int, error) {
	if limit <= 0 {
		limit = 10
	}
	processed := 0
	for processed < limit {
		claimed, err := service.RunNext(ctx)
		if err != nil {
			return processed, err
		}
		if !claimed {
			return processed, nil
		}
		processed++
	}
	return processed, nil
}

func StartDiscoveryWorker(ctx context.Context, service DiscoveryExecutor, interval time.Duration, batchSize int) {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if batchSize <= 0 {
		batchSize = 10
	}
	go func() {
		run := func() {
			processed, err := DrainDiscoveryTasks(ctx, service, batchSize)
			if err != nil {
				log.Printf("[discovery-worker] execution error: %v", err)
			} else if processed > 0 {
				log.Printf("[discovery-worker] processed %d crawl task(s)", processed)
			}
		}
		run()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run()
			}
		}
	}()
}
