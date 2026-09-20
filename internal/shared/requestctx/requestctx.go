// Package requestctx owns the generic request identity values carried by Go context.
package requestctx

import "context"

// String context keys are required by the selected Hertz Zap adapter.
// Application code must use this package instead of calling context.WithValue.
const (
	traceIDKey   = "trace_id"
	requestIDKey = "request_id"
	userIDKey    = "user_id"
)

// WithTraceID returns a context carrying trace_id.
func WithTraceID(ctx context.Context, value string) context.Context {
	return context.WithValue(ctx, traceIDKey, value)
}

// WithRequestID returns a context carrying request_id.
func WithRequestID(ctx context.Context, value string) context.Context {
	return context.WithValue(ctx, requestIDKey, value)
}

// WithUserID returns a context carrying user_id.
func WithUserID(ctx context.Context, value int64) context.Context {
	return context.WithValue(ctx, userIDKey, value)
}

// TraceID returns the trace_id in ctx, or an empty string.
func TraceID(ctx context.Context) string {
	if value, ok := ctx.Value(traceIDKey).(string); ok {
		return value
	}
	return ""
}

// RequestID returns the request_id in ctx, or an empty string.
func RequestID(ctx context.Context) string {
	if value, ok := ctx.Value(requestIDKey).(string); ok {
		return value
	}
	return ""
}

// UserID returns the user_id in ctx, or zero.
func UserID(ctx context.Context) int64 {
	if value, ok := ctx.Value(userIDKey).(int64); ok {
		return value
	}
	return 0
}

// TaskID is intentionally absent from generic request context. Task identity
// remains a business-specific concept and must not be added here.
func TaskID(_ context.Context) string { return "" }
