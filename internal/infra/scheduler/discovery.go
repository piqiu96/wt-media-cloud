package scheduler

import "time"

type DiscoveryTicker interface{ RunDue(time.Time) int }

// StartDiscoveryScheduler provides a small host-controlled trigger loop. It
// intentionally does not parse arbitrary cron or expose a workflow engine.
func StartDiscoveryScheduler(service DiscoveryTicker) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for now := range ticker.C {
			service.RunDue(now.UTC())
		}
	}()
}
