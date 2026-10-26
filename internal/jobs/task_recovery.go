package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/logger"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent/service"
)

// RunTaskRecovery 收口超过自身执行超时仍未上报的 running 任务。
// 所有类型共用一次扫描；类型只决定中断后是可重试还是结果不明。
func RunTaskRecovery(ctx context.Context) error {
	recovered, err := service.RecoverTimedOutTasks(time.Now().UTC(), config.Get().Scheduler.TaskRecoveryBatchSize)
	if err != nil {
		return fmt.Errorf("recover timed out tasks: %w", err)
	}
	jobLogger := logger.Job()
	for _, task := range recovered {
		jobLogger.Infof(
			"task recovery settled module=%s task_id=%s error_code=%s",
			"jobs.task_recovery",
			task.TaskID,
			task.ErrorCode,
		)
	}
	return nil
}
