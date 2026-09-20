// Package observe adds external HTTP call telemetry to Hertz client middleware.
package observe

import (
	"context"
	"time"

	hertzclient "github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/wt-media/wt-media-cloud/internal/infra/logger"
)

// External returns Hertz middleware that records external call duration and outcome.
// It never logs request credentials or response bodies.
func External(name string) hertzclient.Middleware {
	return func(next hertzclient.Endpoint) hertzclient.Endpoint {
		return func(ctx context.Context, request *protocol.Request, response *protocol.Response) error {
			started := time.Now()
			err := next(ctx, request, response)
			duration := time.Since(started).String()

			if err != nil {
				logger.External().CtxErrorf(
					ctx,
					"external http failed name=%s method=%s host=%s path=%s error=%s duration=%s",
					name,
					string(request.Header.Method()),
					string(request.URI().Host()),
					string(request.URI().Path()),
					err.Error(),
					duration,
				)
				return err
			}

			logger.External().CtxInfof(
				ctx,
				"external http completed name=%s method=%s host=%s path=%s status=%d duration=%s",
				name,
				string(request.Header.Method()),
				string(request.URI().Host()),
				string(request.URI().Path()),
				response.StatusCode(),
				duration,
			)
			return nil
		}
	}
}
