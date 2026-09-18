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
	runner.Start(ctx)
	<-ctx.Done()
	return nil
}
