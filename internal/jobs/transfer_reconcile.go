package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/logger"
	transferrepo "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/repository"
)

// RunTransferReconcile finishes transfer tasks whose executor stopped reporting.
//
// Cancelling a running transfer only records the request (see `cancelTask`); the
// executor owns the transition out of `running`, and a machine that was closed or
// a process that was killed never makes it. Such a task is not leasable either,
// so nothing else can take it — which is exactly what the two repository
// sweeps are for, and why they were written with their own tests. This job just
// gives them a home in the scheduler, at the worker process's polling interval.
//
// It is silent unless a pass actually took rows out of a state nothing else
// could leave: the sweep runs every interval, and a message about an empty pass
// is noise in the same way a worker saying "not configured" would be.
func RunTransferReconcile(ctx context.Context) error {
	cancelled, err := transferrepo.ReconcileCancelledTasks(time.Now())
	if err != nil {
		return fmt.Errorf("reconcile cancelled transfers: %w", err)
	}
	exhausted, err := transferrepo.ReconcileExhaustedTasks(time.Now())
	if err != nil {
		return fmt.Errorf("reconcile exhausted transfers: %w", err)
	}
	if cancelled+exhausted > 0 {
		logger.Job().Infof("transfer reconcile finished %d cancelled, %d exhausted", cancelled, exhausted)
	}
	return nil
}
