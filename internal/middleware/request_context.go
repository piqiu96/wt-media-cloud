package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/wt-media/wt-media-cloud/internal/infra/logger"
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
		c.Next(ctx)
		logger.Access().CtxInfof(
			ctx,
			"request completed method=%s path=%s status=%d duration=%s",
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
