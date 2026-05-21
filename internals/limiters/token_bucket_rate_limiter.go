package limiters

import (
	"context"
	"time"

	"golang.org/x/time/rate"
)

type TokenBucketRateLimiterConfig struct {
	Limit rate.Limit
	Burst int
}

type TokenBucketRateLimiter struct {
	limiter  *rate.Limiter
	lastUsed time.Time
}

func NewTokenBucketRateLimiter(conf *TokenBucketRateLimiterConfig) *TokenBucketRateLimiter {
	return &TokenBucketRateLimiter{
		limiter:  rate.NewLimiter(conf.Limit, conf.Burst),
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
