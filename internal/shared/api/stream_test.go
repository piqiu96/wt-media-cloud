package api

import (
	"bufio"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
)

func TestStreamProducesProgressBeforeWorkCompletesAndCancelsOnDisconnect(t *testing.T) {
	c := app.NewContext(0)
	cancelled := make(chan struct{})
	StreamOne(c, StreamItem{UserID: 901, RowKey: "row", InputRevision: "r1"}, func(ctx context.Context, progress func(string) error) (any, error) {
		defer close(cancelled)
		if err := progress("rendering"); err != nil {
			return nil, err
		}
		<-ctx.Done()
		return nil, ctx.Err()
	})
	body := c.Response.BodyStream()
	reader := bufio.NewReader(body)
	first, err := reader.ReadString('\n')
	if err != nil || first != "event: start\n" {
		t.Fatalf("start: %q %v", first, err)
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(line) == "event: progress" {
			break
		}
	}
	if err := body.(io.Closer).Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("disconnected work was not cancelled")
	}
}

func TestStreamCompletesWithSingleItemResult(t *testing.T) {
	c := app.NewContext(0)
	StreamOne(c, StreamItem{UserID: 902, RowKey: "row", InputRevision: "r2"}, func(ctx context.Context, progress func(string) error) (any, error) {
		return map[string]string{"title": "真实标题"}, nil
	})
	body := c.Response.BodyStream()
	defer body.(io.Closer).Close()
	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Count(text, "event: item_result") != 1 || !strings.Contains(text, "event: done") || !strings.Contains(text, "真实标题") {
		t.Fatal(text)
	}
}

func TestGenerationCanUseAIWindowWithoutExtendingCoverDeadline(t *testing.T) {
	for _, duration := range []time.Duration{60 * time.Second, 180 * time.Second} {
		c := app.NewContext(0)
		run := func(ctx context.Context, _ func(string) error) (any, error) {
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) > duration || time.Until(deadline) < duration-time.Second {
				t.Errorf("wrong operation deadline for %s", duration)
			}
			return map[string]string{"ok": "complete"}, nil
		}
		if duration == 60*time.Second {
			StreamOne(c, StreamItem{UserID: 999}, run)
		} else {
			StreamOneWithTimeout(c, StreamItem{UserID: 999}, duration, run)
		}
		body := c.Response.BodyStream()
		_, err := io.ReadAll(body)
		_ = body.(io.Closer).Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}
