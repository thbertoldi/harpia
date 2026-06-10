package server

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/harpia/control-plane/internal/cache"
)

type RateLimitInterceptor struct {
	limiter      *cache.RateLimiter
	maxPerMinute int
}

func NewRateLimitInterceptor(limiter *cache.RateLimiter, maxPerMinute int) *RateLimitInterceptor {
	return &RateLimitInterceptor{limiter: limiter, maxPerMinute: maxPerMinute}
}

func (i *RateLimitInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if err := i.check(ctx); err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

func (i *RateLimitInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *RateLimitInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		if err := i.check(ctx); err != nil {
			return err
		}
		return next(ctx, conn)
	}
}

func (i *RateLimitInterceptor) check(ctx context.Context) error {
	if i == nil || i.limiter == nil {
		return nil
	}
	allowed, err := i.limiter.Allow(ctx, i.maxPerMinute)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if !allowed {
		return connect.NewError(connect.CodeResourceExhausted, errors.New("rate limit exceeded"))
	}
	return nil
}
