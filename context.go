package queryrunner

import (
	"context"
	"time"
)

type contextKey string

var timeoutExtenderContextKey contextKey = "__queryrunner_timeout_extender"

type TimeoutExtender interface {
	ExtendTimeout(ctx context.Context, timeout time.Duration) error
}

type TimeoutExtenderFunc func(ctx context.Context, timeout time.Duration) error

func (f TimeoutExtenderFunc) ExtendTimeout(ctx context.Context, timeout time.Duration) error {
	return f(ctx, timeout)
}

func WithTimeoutExtender(ctx context.Context, extender TimeoutExtender) context.Context {
	return context.WithValue(ctx, timeoutExtenderContextKey, extender)
}

func GetTimeoutExtender(ctx context.Context) TimeoutExtender {
	if extender, ok := ctx.Value(timeoutExtenderContextKey).(TimeoutExtender); ok {
		return extender
	}
	return TimeoutExtenderFunc(func(ctx context.Context, timeout time.Duration) error { return nil })
}

var requestIDContextKey contextKey = "__queryrunner_request_id"

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey, requestID)
}

func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(requestIDContextKey).(string); ok {
		return requestID
	}
	return "-"
}
