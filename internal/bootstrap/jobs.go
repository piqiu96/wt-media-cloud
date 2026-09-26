package bootstrap

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/jobs"
	"github.com/wt-media/wt-media-cloud/internal/scheduler"
)

func runSchedulerProcess() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.Get()
	runner := scheduler.New()
	runner.Register("discovery-schedule", cfg.Scheduler.DiscoveryInterval.Duration, func(ctx context.Context) error {
		return jobs.RunDiscoverySchedule(ctx)
	})
	runner.Register("proxy-expiry", cfg.Scheduler.ProxyExpiryInterval.Duration, func(ctx context.Context) error {
		return jobs.RunProxyExpiry(ctx)
	})
	runner.Start(ctx)
	<-ctx.Done()
	return nil
}

func runWorkerProcess() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.Get()
	runner := scheduler.New()
	runner.Register("discovery-worker", cfg.Scheduler.WorkerInterval.Duration, func(ctx context.Context) error {
		return jobs.RunDiscoveryWorker(ctx, cfg.Scheduler.WorkerBatchSize)
	})
	// The preparation worker shares the discovery worker's interval rather than
	// taking a key of its own. Both drain a queue by polling, both get their own
	// goroutine here and neither overlaps itself, and the interval that suits one
	// suits the other: it is how long an idle worker waits between looks, not how
	// long a job may take. A second key would have to be added to both config trees
	// and to their validation, and the process would refuse to start until an
	// operator edited two files, to tune a number nothing needs to tune separately.
	runner.Register("material-prepare-worker", cfg.Scheduler.WorkerInterval.Duration, func(ctx context.Context) error {
		return jobs.RunMaterialPrepareWorker(ctx)
	})
	runner.Start(ctx)
	<-ctx.Done()
	return nil
}
