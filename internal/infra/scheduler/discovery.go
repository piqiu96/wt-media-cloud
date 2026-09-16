package scheduler

import (
	"context"
	"log"
	"time"
)

type DiscoveryTicker interface{ RunDue(time.Time) int }

func RunDiscoverySchedulerOnce(service DiscoveryTicker, now time.Time) int {
	return service.RunDue(now.UTC())
}

// StartDiscoveryScheduler starts only the strategy-to-task scheduling loop.
// Task execution belongs to Discovery Worker.
func StartDiscoveryScheduler(ctx context.Context, service DiscoveryTicker, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				if triggered := RunDiscoverySchedulerOnce(service, now); triggered > 0 {
					log.Printf("[discovery-scheduler] queued %d crawl task(s)", triggered)
				}
			}
		}
	}()
}
