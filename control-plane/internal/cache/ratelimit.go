package cache

import (
	"context"
	"fmt"
	"time"
)

type RateLimiter struct {
	store *TenantStore
}

func NewRateLimiter(store *TenantStore) *RateLimiter {
	return &RateLimiter{store: store}
}

const slidingWindowScript = `
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])

redis.call('ZREMRANGEBYSCORE', key, 0, now - window)

local count = redis.call('ZCARD', key)
if count < limit then
	redis.call('ZADD', key, now, now .. ':' .. count)
	redis.call('EXPIRE', key, math.ceil(window / 1000) + 1)
	return 1
end
return 0
`

func (r *RateLimiter) Allow(ctx context.Context, maxPerMinute int) (bool, error) {
	key := "ratelimit"
	now := time.Now().UnixMilli()
	window := int64(60 * time.Second / time.Millisecond)

	result, err := r.store.EvalInt(ctx, slidingWindowScript, []string{key}, now, window, maxPerMinute)
	if err != nil {
		return false, fmt.Errorf("rate limit eval: %w", err)
	}

	return result == 1, nil
}
