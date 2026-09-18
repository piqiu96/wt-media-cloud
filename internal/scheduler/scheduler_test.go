package scheduler

import (
	"context"
	"testing"
	"time"
)

func TestRunOnceInvokesRegisteredJobWithContext(t *testing.T) {
	s := New()
	called := false
	s.Register("discovery", time.Minute, func(ctx context.Context) error {
		called = ctx != nil
		return nil
	})
	if err := s.RunOnce(context.Background()); err != nil {
		t.Fatalf("run once: %v", err)
	}
	if !called {
		t.Fatal("registered job was not invoked with a context")
	}
}
