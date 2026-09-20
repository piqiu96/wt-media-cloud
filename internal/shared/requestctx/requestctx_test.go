package requestctx_test

import (
	"context"
	"testing"

	"github.com/wt-media/wt-media-cloud/internal/shared/requestctx"
)

func TestContextCarriesTraceRequestAndUserIDs(t *testing.T) {
	ctx := context.Background()
	ctx = requestctx.WithTraceID(ctx, "tr-1")
	ctx = requestctx.WithRequestID(ctx, "rq-1")
	ctx = requestctx.WithUserID(ctx, 7)

	if got := requestctx.TraceID(ctx); got != "tr-1" {
		t.Fatalf("TraceID() = %q", got)
	}
	if got := requestctx.RequestID(ctx); got != "rq-1" {
		t.Fatalf("RequestID() = %q", got)
	}
	if got := requestctx.UserID(ctx); got != 7 {
		t.Fatalf("UserID() = %d", got)
	}
}

func TestContextReturnsZeroValuesWhenAbsent(t *testing.T) {
	ctx := context.Background()
	if requestctx.TraceID(ctx) != "" || requestctx.RequestID(ctx) != "" || requestctx.UserID(ctx) != 0 {
		t.Fatal("absent context values are not zero values")
	}
}

func TestContextDoesNotCarryTaskID(t *testing.T) {
	if requestctx.TaskID(context.Background()) != "" {
		t.Fatal("generic request context unexpectedly exposes task_id")
	}
}
