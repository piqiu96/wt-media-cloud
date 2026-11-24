package api

import (
	"context"
	"errors"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"
)

// MaxStreamItems caps work started by one connection. Clients can send additional
// requests for selections larger than this bound.
const MaxStreamItems = 50

type StreamJob struct {
	Item  StreamItem
	Stage string
	Run   func(context.Context) (any, error)
}
type streamJobResult struct {
	result any
	err    error
}

func runStreamJob(ctx context.Context, job StreamJob, logID string) <-chan streamJobResult {
	result := make(chan streamJobResult, 1)
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				zap.L().Error("batch generation panic", zap.Any("panic", recovered), zap.String("logid", logID))
				result <- streamJobResult{err: errors.New("generation interrupted")}
			}
		}()
		value, err := job.Run(ctx)
		result <- streamJobResult{result: value, err: err}
	}()
	return result
}

func StreamManyWithTimeout(c *app.RequestContext, userID int64, timeout time.Duration, jobs []StreamJob) {
	release := acquireGenerationSlot(c, userID)
	if release == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	writer, logID := openGenerationStream(c, cancel)
	go func() {
		var running <-chan streamJobResult
		defer func() {
			if running != nil {
				<-running
			}
			release()
		}()
		// Preserve an opportunity to emit a terminal timeout even if a dependency
		// does not return promptly. Close the pipe if the client has stopped reading.
		stop := context.AfterFunc(ctx, func() {
			timer := time.NewTimer(time.Second)
			defer timer.Stop()
			<-timer.C
			_ = writer.Close()
		})
		defer func() { stop(); cancel(); _ = writer.Close() }()
		emit := func(event string, data any) error { return writeGenerationEvent(writer, event, data) }
		total := len(jobs)
		if err := emit("start", map[string]any{"total": total, "logid": logID}); err != nil {
			return
		}
		success, failed := 0, 0
		heartbeat := time.NewTicker(10 * time.Second)
		defer heartbeat.Stop()
		finishTimeout := func() {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				_ = emit("error", map[string]any{"total": total, "success": success, "failed": failed, "code": "generation_timeout", "message": "生成超时，请重试未完成的条目", "logid": logID})
			}
		}
		for _, job := range jobs {
			if ctx.Err() != nil {
				finishTimeout()
				return
			}
			event := func() map[string]any {
				return map[string]any{"row_key": job.Item.RowKey, "input_revision": job.Item.InputRevision, "field_revisions": job.Item.FieldRevisions}
			}
			progress := event()
			progress["stage"], progress["completed"], progress["total"] = job.Stage, success+failed, total
			if err := emit("progress", progress); err != nil {
				return
			}
			if ctx.Err() != nil {
				finishTimeout()
				return
			}
			running = runStreamJob(ctx, job, logID)
			var row streamJobResult
			waiting := true
			for waiting {
				select {
				case <-ctx.Done():
					finishTimeout()
					return
				case <-heartbeat.C:
					if _, err := writer.Write([]byte(": keep-alive\n\n")); err != nil {
						return
					}
				case row = <-running:
					running = nil
					waiting = false
				}
			}
			if ctx.Err() != nil {
				finishTimeout()
				return
			}
			payload := event()
			if row.err != nil {
				failed++
				payload["message"], payload["details"], payload["logid"] = streamErrorMessage(row.err), row.result, logID
				var known *StreamError
				if errors.As(row.err, &known) {
					payload["code"] = known.Code
				}
				if err := emit("item_error", payload); err != nil {
					return
				}
			} else {
				success++
				payload["result"] = row.result
				if err := emit("item_result", payload); err != nil {
					return
				}
			}
		}
		_ = emit("done", map[string]any{"total": total, "success": success, "failed": failed})
	}()
}
