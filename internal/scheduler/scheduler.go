// Package scheduler owns periodic job timing and cancellation.
package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type JobFunc func(context.Context) error

type Scheduler struct {
	mu   sync.RWMutex
	jobs []job
}

type job struct {
	name     string
	interval time.Duration
	run      JobFunc
}

func New() *Scheduler { return &Scheduler{} }

func (s *Scheduler) Register(name string, interval time.Duration, run JobFunc) {
	if s == nil || run == nil || interval <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs = append(s.jobs, job{name: name, interval: interval, run: run})
}

func (s *Scheduler) RunOnce(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	jobs := append([]job(nil), s.jobs...)
	s.mu.RUnlock()
	for _, item := range jobs {
		if err := item.run(ctx); err != nil {
			return fmt.Errorf("run job %s: %w", item.name, err)
		}
	}
	return nil
}

func (s *Scheduler) Start(ctx context.Context) {
	if s == nil {
		return
	}
	s.mu.RLock()
	jobs := append([]job(nil), s.jobs...)
	s.mu.RUnlock()
	for _, item := range jobs {
		go runPeriodically(ctx, item)
	}
}

func runPeriodically(ctx context.Context, item job) {
	_ = item.run(ctx)
	ticker := time.NewTicker(item.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = item.run(ctx)
		}
	}
}
