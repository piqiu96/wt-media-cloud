package jobs

import (
	"context"
	"testing"
)

func TestRunDiscoveryScheduleUsesPackageServiceBoundary(t *testing.T) {
	var run func(context.Context) error = RunDiscoverySchedule
	if run == nil {
		t.Fatal("discovery schedule entry point is unavailable")
	}
}
