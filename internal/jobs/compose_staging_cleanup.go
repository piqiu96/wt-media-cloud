package jobs

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
)

type composeStagingCleanupStore interface {
	FindStagingCleanupCandidates(limit int) ([]model.ComposeStagingCleanupCandidate, error)
	MarkStagingCleaned(taskID int64, attempt int) (bool, error)
}

// CleanComposeStaging removes only the staging key for a known finished
// attempt. The repository excludes execution_uncertain until an administrator
// records verification that the old process stopped. A deletion failure
// remains unmarked for the next scan.
func CleanComposeStaging(ctx context.Context, tasks composeStagingCleanupStore, remove func(context.Context, string) error) error {
	items, err := tasks.FindStagingCleanupCandidates(100)
	if err != nil {
		return err
	}
	var failures error
	for _, item := range items {
		key := "compose-staging/" + strconv.FormatInt(item.TaskID, 10) + "/" + strconv.Itoa(item.Attempt) + "/output.mp4"
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		err := remove(cleanupCtx, key)
		cancel()
		if err != nil {
			failures = errors.Join(failures, fmt.Errorf("remove %s: %w", key, err))
			continue
		}
		if _, err := tasks.MarkStagingCleaned(item.TaskID, item.Attempt); err != nil {
			failures = errors.Join(failures, fmt.Errorf("mark %s cleaned: %w", key, err))
		}
	}
	return failures
}
