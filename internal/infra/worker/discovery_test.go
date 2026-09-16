package worker

import (
	"context"
	"testing"
)

type workerStub struct {
	remaining int
	calls     int
}

func (w *workerStub) RunNext(context.Context) (bool, error) {
	w.calls++
	if w.remaining == 0 {
		return false, nil
	}
	w.remaining--
	return true, nil
}

func TestDrainDiscoveryTasksStopsWhenQueueIsEmpty(t *testing.T) {
	service := &workerStub{remaining: 2}
	processed, err := DrainDiscoveryTasks(context.Background(), service, 10)
	if err != nil {
		t.Fatal(err)
	}
	if processed != 2 || service.calls != 3 {
		t.Fatalf("processed=%d calls=%d", processed, service.calls)
	}
}

func TestDrainDiscoveryTasksRespectsBatchLimit(t *testing.T) {
	service := &workerStub{remaining: 10}
	processed, err := DrainDiscoveryTasks(context.Background(), service, 3)
	if err != nil {
		t.Fatal(err)
	}
	if processed != 3 || service.calls != 3 {
		t.Fatalf("processed=%d calls=%d", processed, service.calls)
	}
}
