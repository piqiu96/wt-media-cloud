package jobs

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCleanupExpiredSessionsDeletesInBatchesUntilShort(t *testing.T) {
	cutoff := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	results := []int64{2, 2, 1}
	var cutoffs []time.Time
	var limits []int
	calls := 0
	deleted, err := cleanupExpiredSessions(context.Background(), cutoff, 2, func(_ context.Context, gotCutoff time.Time, limit int) (int64, error) {
		cutoffs = append(cutoffs, gotCutoff)
		limits = append(limits, limit)
		result := results[calls]
		calls++
		return result, nil
	})
	if err != nil {
		t.Fatalf("cleanupExpiredSessions() error = %v", err)
	}
	if deleted != 5 {
		t.Fatalf("deleted = %d, want 5", deleted)
	}
	if calls != 3 {
		t.Fatalf("batch calls = %d, want 3", calls)
	}
	for _, got := range cutoffs {
		if !got.Equal(cutoff) {
			t.Fatalf("cutoff changed between batches: %s", got)
		}
	}
	for _, limit := range limits {
		if limit != 2 {
			t.Fatalf("limit = %d, want 2", limit)
		}
	}
}

func TestCleanupExpiredSessionsStopsWhenBatchIsEmpty(t *testing.T) {
	results := []int64{500, 0}
	calls := 0
	deleted, err := cleanupExpiredSessions(context.Background(), time.Now(), 500, func(context.Context, time.Time, int) (int64, error) {
		result := results[calls]
		calls++
		return result, nil
	})
	if err != nil {
		t.Fatalf("cleanupExpiredSessions() error = %v", err)
	}
	if deleted != 500 {
		t.Fatalf("deleted = %d, want 500", deleted)
	}
	if calls != 2 {
		t.Fatalf("batch calls = %d, want 2", calls)
	}
}

func TestCleanupExpiredSessionsReturnsProgressAndError(t *testing.T) {
	boom := errors.New("database is gone")
	calls := 0
	deleted, err := cleanupExpiredSessions(context.Background(), time.Now(), 2, func(context.Context, time.Time, int) (int64, error) {
		calls++
		if calls == 1 {
			return 2, nil
		}
		return 0, boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("cleanupExpiredSessions() error = %v, want %v", err, boom)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2", deleted)
	}
	if calls != 2 {
		t.Fatalf("batch calls = %d, want 2", calls)
	}
}

func TestCleanupExpiredSessionsStopsOnCanceledContextBetweenBatches(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	deleted, err := cleanupExpiredSessions(ctx, time.Now(), 2, func(context.Context, time.Time, int) (int64, error) {
		calls++
		cancel()
		return 2, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cleanupExpiredSessions() error = %v, want context.Canceled", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2", deleted)
	}
	if calls != 1 {
		t.Fatalf("batch calls = %d, want 1", calls)
	}
}

func TestCleanupExpiredSessionsRejectsNonPositiveBatchSize(t *testing.T) {
	for _, batchSize := range []int{0, -1} {
		called := false
		_, err := cleanupExpiredSessions(context.Background(), time.Now(), batchSize, func(context.Context, time.Time, int) (int64, error) {
			called = true
			return 0, nil
		})
		if err == nil {
			t.Fatalf("cleanupExpiredSessions(batchSize=%d) error = nil, want a validation error", batchSize)
		}
		if called {
			t.Fatalf("cleanupExpiredSessions(batchSize=%d) called the batch deleter", batchSize)
		}
	}
}
