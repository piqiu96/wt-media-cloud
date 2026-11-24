package api

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
)

func TestBatchStreamReturnsIndependentTerminalResults(t *testing.T) {
	c := app.NewContext(0)
	StreamManyWithTimeout(c, 904, time.Second, []StreamJob{
		{Item: StreamItem{RowKey: "first", InputRevision: "3", FieldRevisions: map[string]string{"topics": "2"}}, Stage: "generating", Run: func(context.Context) (any, error) { return map[string]string{"topics": "真实结果"}, nil }},
		{Item: StreamItem{RowKey: "second", InputRevision: "4", FieldRevisions: map[string]string{"description": "5"}}, Stage: "generating", Run: func(context.Context) (any, error) {
			return nil, &StreamError{Code: "item_invalid", Message: "候选已变化"}
		}},
	})
	body := c.Response.BodyStream()
	defer body.(io.Closer).Close()
	raw, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	stream := string(raw)
	if strings.Count(stream, "event: item_result") != 1 || strings.Count(stream, "event: item_error") != 1 ||
		!strings.Contains(stream, `"field_revisions":{"description":"5"}`) || !strings.Contains(stream, `"code":"item_invalid"`) ||
		!strings.Contains(stream, `"failed":1,"success":1,"total":2`) || !strings.Contains(stream, "真实结果") {
		t.Fatal(stream)
	}
}

func TestBatchStreamDeadlineDoesNotStartLaterRows(t *testing.T) {
	c := app.NewContext(0)
	StreamManyWithTimeout(c, 905, 50*time.Millisecond, []StreamJob{
		{Item: StreamItem{RowKey: "first"}, Stage: "generating", Run: func(ctx context.Context) (any, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}},
		{Item: StreamItem{RowKey: "later"}, Stage: "generating", Run: func(context.Context) (any, error) {
			t.Error("next row started after the deadline")
			return nil, nil
		}},
	})
	body := c.Response.BodyStream()
	defer body.(io.Closer).Close()
	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	stream := string(data)
	if !strings.Contains(stream, "event: error") || !strings.Contains(stream, "generation_timeout") ||
		strings.Contains(stream, `"row_key":"later"`) || strings.Contains(stream, "event: done") {
		t.Fatal(stream)
	}
}
