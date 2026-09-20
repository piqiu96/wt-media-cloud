package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/wt-media/wt-media-cloud/internal/infra/logger"
	"github.com/wt-media/wt-media-cloud/internal/shared/requestctx"
)

func RequestContext() hertzapp.HandlerFunc {
	return func(ctx context.Context, c *hertzapp.RequestContext) {
		started := time.Now()
		traceID := nextID("tr")
		requestID := string(c.Request.Header.Peek("X-Request-ID"))
		if requestID == "" {
			requestID = nextID("rq")
		}
		c.Set("trace_id", traceID)
		c.Set("request_id", requestID)
		c.Set("logid", traceID)
		ctx = requestctx.WithTraceID(ctx, traceID)
		ctx = requestctx.WithRequestID(ctx, requestID)
		c.Next(ctx)
		if value, exists := c.Get("user_id"); exists {
			if userID, valid := value.(int64); valid && userID > 0 {
				ctx = requestctx.WithUserID(ctx, userID)
			}
		}
		logger.Access().CtxInfof(
			ctx,
			"module=http request completed method=%s path=%s status=%d duration=%s",
			string(c.Method()), string(c.Path()), c.Response.StatusCode(), time.Since(started).String(),
		)
	}
}

func nextID(prefix string) string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return prefix + "-unknown"
	}
	return prefix + hex.EncodeToString(bytes)
}
