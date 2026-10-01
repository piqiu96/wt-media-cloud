package jobs

import (
	"context"
	"testing"
)

func TestRunTransferReconcileUsesPackageServiceBoundary(t *testing.T) {
	var run func(context.Context) error = RunTransferReconcile
	if run == nil {
		t.Fatal("transfer reconcile entry point is unavailable")
	}
}
