package chat

import (
	"context"
	"golang.org/x/time/rate"
	"time"
)

type TokenBucketRateLimiter struct {
	limiter  *rate.Limiter
	lastUsed time.Time
}

func NewTokenBucketRateLimiter(limit rate.Limit, burst int) *TokenBucketRateLimiter {
	return &TokenBucketRateLimiter{
		limiter:  rate.NewLimiter(limit, burst),
		lastUsed: time.Now(),
	}
}

func (c *TokenBucketRateLimiter) IsStale(d time.Duration) bool {
	return time.Since(c.lastUsed) > d
}

func (c *TokenBucketRateLimiter) Wait(ctx context.Context) error {
	c.lastUsed = time.Now()
	return c.limiter.Wait(ctx)
}

func (c *TokenBucketRateLimiter) GetWaitTime() time.Duration {
	c.lastUsed = time.Now()
	reservation := c.limiter.Reserve()
	defer reservation.Cancel()

	return reservation.Delay()
}
