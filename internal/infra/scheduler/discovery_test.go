package scheduler

import (
	"testing"
	"time"
)

type schedulerStub struct {
	called int
	at     time.Time
}

func (s *schedulerStub) RunDue(at time.Time) int {
	s.called++
	s.at = at
	return 3
}

func TestRunDiscoverySchedulerOnceOnlyEnqueuesDueTasks(t *testing.T) {
	service := &schedulerStub{}
	at := time.Date(2026, 9, 16, 1, 0, 0, 0, time.UTC)
	if got := RunDiscoverySchedulerOnce(service, at); got != 3 {
		t.Fatalf("triggered=%d, want 3", got)
	}
	if service.called != 1 || !service.at.Equal(at) {
		t.Fatalf("called=%d at=%s", service.called, service.at)
	}
}
