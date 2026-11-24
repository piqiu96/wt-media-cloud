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
	UserID         int64             `json:"-"`
	RowKey         string            `json:"row_key"`
	InputRevision  string            `json:"input_revision"`
	FieldRevisions map[string]string `json:"field_revisions,omitempty"`
}
type cancelStream struct {
	*io.PipeReader
	cancel context.CancelFunc
}

func (s cancelStream) Close() error { s.cancel(); return s.PipeReader.Close() }

// StreamOne serves one bounded M6 generation operation. Validation and permission
// errors must be returned before calling it. The operation owns no request context.
func StreamOne(c *app.RequestContext, item StreamItem, run func(context.Context, func(string) error) (any, error)) {
	StreamOneWithTimeout(c, item, 60*time.Second, run)
}

// StreamOneWithTimeout preserves the shared stream lifecycle with an operation deadline.
func StreamOneWithTimeout(c *app.RequestContext, item StreamItem, timeout time.Duration, run func(context.Context, func(string) error) (any, error)) {
	release := acquireGenerationSlot(c, item.UserID)
	if release == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	writer, logID := openGenerationStream(c, cancel)
	go func() {
		defer release()
		stop := context.AfterFunc(ctx, func() { writer.CloseWithError(ctx.Err()) })
		defer func() { stop(); cancel(); writer.Close() }()
		emit := func(event string, data any) error { return writeGenerationEvent(writer, event, data) }
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

func acquireGenerationSlot(c *app.RequestContext, userID int64) func() {
	generationUser.Lock()
	if generationUser.count[userID] >= 2 {
		generationUser.Unlock()
		FailureCode(c, 429, 19004, "已有生成任务，请稍后重试", "generation_busy", nil, true, "retry")
		return nil
	}
	select {
	case generationSlot <- struct{}{}:
		generationUser.count[userID]++
		generationUser.Unlock()
	default:
		generationUser.Unlock()
		FailureCode(c, 429, 19004, "生成服务繁忙，请稍后重试", "generation_busy", nil, true, "retry")
		return nil
	}
	return func() {
		<-generationSlot
		generationUser.Lock()
		generationUser.count[userID]--
		if generationUser.count[userID] == 0 {
			delete(generationUser.count, userID)
		}
		generationUser.Unlock()
	}
}

func openGenerationStream(c *app.RequestContext, cancel context.CancelFunc) (*io.PipeWriter, string) {
	reader, writer := io.Pipe()
	c.SetStatusCode(200)
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("X-Accel-Buffering", "no")
	c.Response.ImmediateHeaderFlush = true
	c.SetBodyStream(cancelStream{reader, cancel}, -1)
	return writer, resolveLogID(c)
}

func writeGenerationEvent(writer *io.PipeWriter, event string, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, "event: %s\ndata: %s\n\n", event, raw)
	return err
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
