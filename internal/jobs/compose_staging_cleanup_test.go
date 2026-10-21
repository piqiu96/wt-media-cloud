package jobs

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/wt-media/wt-media-cloud/internal/modules/production/compose"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	productionservice "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
)

type stagingCleanupFake struct {
	items  []model.ComposeStagingCleanupCandidate
	marked []model.ComposeStagingCleanupCandidate
}

func TestComposeWorkerRunsStagingCleanupAtStartup(t *testing.T) {
	workspace, err := compose.NewWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	called := 0
	deps := ComposeWorkerDeps{
		Tasks:     &workerTasksFake{},
		Workspace: workspace,
		Attempt: func(context.Context, model.ComposeTask, *compose.RunDir, *productionservice.ComposeInputGate) productionservice.ComposeAttemptOutcome {
			t.Fatal("no task should be claimed")
			return ""
		},
		DiskFree: func(string) (int64, error) { return 1 << 40, nil },
		CleanStaging: func(context.Context) error {
			called++
			cancel()
			return nil
		},
	}
	if err := RunComposeWorker(ctx, deps, workerSettings()); err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatalf("startup cleanup calls=%d", called)
	}
}

func (f *stagingCleanupFake) FindStagingCleanupCandidates(int) ([]model.ComposeStagingCleanupCandidate, error) {
	return f.items, nil
}

func (f *stagingCleanupFake) MarkStagingCleaned(taskID int64, attempt int) (bool, error) {
	f.marked = append(f.marked, model.ComposeStagingCleanupCandidate{TaskID: taskID, Attempt: attempt})
	return true, nil
}

func TestCleanComposeStagingUsesAttemptKeyAndMarksOnlyAfterRemoval(t *testing.T) {
	items := []model.ComposeStagingCleanupCandidate{{TaskID: 77, Attempt: 1}, {TaskID: 77, Attempt: 2}}
	store := &stagingCleanupFake{items: items}
	var keys []string
	err := CleanComposeStaging(context.Background(), store, func(_ context.Context, key string) error {
		keys = append(keys, key)
		if key == "compose-staging/77/1/output.mp4" {
			return errors.New("storage unavailable")
		}
		return nil
	})
	if err == nil {
		t.Fatal("failed deletion must remain observable for the next scan")
	}
	if !reflect.DeepEqual(keys, []string{"compose-staging/77/1/output.mp4", "compose-staging/77/2/output.mp4"}) {
		t.Fatalf("keys=%v", keys)
	}
	if !reflect.DeepEqual(store.marked, items[1:]) {
		t.Fatalf("marked=%v", store.marked)
	}
}
