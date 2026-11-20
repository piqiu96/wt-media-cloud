package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"
	"io"
	"sync"
	"time"
)

type StreamError struct {
	Message string
	Code    string
}

func (e *StreamError) Error() string { return e.Message }

var generationSlot = make(chan struct{}, 4)
var generationUser = struct {
	sync.Mutex
	count map[int64]int
}{count: map[int64]int{}}

type StreamItem struct {
	UserID        int64  `json:"-"`
	RowKey        string `json:"row_key"`
	InputRevision string `json:"input_revision"`
}
type cancelStream struct {
	*io.PipeReader
	cancel context.CancelFunc
}

func (s cancelStream) Close() error { s.cancel(); return s.PipeReader.Close() }

// StreamOne serves one bounded M6 generation operation. Validation and permission
// errors must be returned before calling it. The operation owns no request context.
func StreamOne(c *app.RequestContext, item StreamItem, run func(context.Context, func(string) error) (any, error)) {
	generationUser.Lock()
	if generationUser.count[item.UserID] >= 2 {
		generationUser.Unlock()
		FailureCode(c, 429, 19004, "已有生成任务，请稍后重试", "generation_busy", nil, true, "retry")
		return
	}
	select {
	case generationSlot <- struct{}{}:
		generationUser.count[item.UserID]++
		generationUser.Unlock()
	default:
		generationUser.Unlock()
		FailureCode(c, 429, 19004, "生成服务繁忙，请稍后重试", "generation_busy", nil, true, "retry")
		return
	}
	release := func() {
		<-generationSlot
		generationUser.Lock()
		generationUser.count[item.UserID]--
		if generationUser.count[item.UserID] == 0 {
			delete(generationUser.count, item.UserID)
		}
		generationUser.Unlock()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	reader, writer := io.Pipe()
	c.SetStatusCode(200)
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("X-Accel-Buffering", "no")
	c.Response.ImmediateHeaderFlush = true
	c.SetBodyStream(cancelStream{reader, cancel}, -1)
	logID := resolveLogID(c)
	go func() {
		defer release()
		stop := context.AfterFunc(ctx, func() { writer.CloseWithError(ctx.Err()) })
		defer func() { stop(); cancel(); writer.Close() }()
		emit := func(event string, data any) error {
			raw, e := json.Marshal(data)
			if e != nil {
				return e
			}
			_, e = fmt.Fprintf(writer, "event: %s\ndata: %s\n\n", event, raw)
			return e
		}
		defer func() {
			if recovered := recover(); recovered != nil {
				zap.L().Error("generation panic", zap.Any("panic", recovered), zap.String("logid", logID))
				_ = emit("error", map[string]any{"message": "生成中断，请保留填写内容后重试", "logid": logID})
			}
		}()
		if e := emit("start", map[string]any{"total": 1, "row_key": item.RowKey, "input_revision": item.InputRevision, "logid": logID}); e != nil {
			return
		}
		result, e := run(ctx, func(stage string) error {
			return emit("progress", map[string]any{"stage": stage, "completed": 0, "total": 1, "row_key": item.RowKey, "input_revision": item.InputRevision})
		})
		success, failed := 0, 0
		if e != nil {
			failed = 1
			if emit("item_error", map[string]any{"row_key": item.RowKey, "input_revision": item.InputRevision, "message": streamErrorMessage(e), "details": result, "logid": logID}) != nil {
				return
			}
		} else {
			success = 1
			if emit("item_result", map[string]any{"row_key": item.RowKey, "input_revision": item.InputRevision, "result": result}) != nil {
				return
			}
		}
		_ = emit("done", map[string]any{"total": 1, "success": success, "failed": failed})
	}()
}

func streamErrorMessage(e error) string {
	var known *StreamError
	if errors.As(e, &known) {
		return known.Message
	}
	if errors.Is(e, context.DeadlineExceeded) {
		return "生成超时，请保留填写内容后重试"
	}
	zap.L().Error("generation failed", zap.Error(e))
	return "生成未完成，请保留填写内容后重试"
}
