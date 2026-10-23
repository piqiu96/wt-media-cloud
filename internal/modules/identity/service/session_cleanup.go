package service

import (
	"context"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/identity/repository"
)

func DeleteExpiredSessionsBefore(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	return repository.DeleteExpiredSessionsBefore(ctx, cutoff, limit)
}
