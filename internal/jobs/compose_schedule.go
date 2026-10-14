package jobs

import (
	"context"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/production/service"
)

func RunComposeSchedule(_ context.Context) error {
	_, err := service.RunDueComposeSchedules(time.Now())
	return err
}
