package limiters

import (
	"context"
	"sync"
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
	mu       sync.Mutex
}

func NewTokenBucketRateLimiter(conf *TokenBucketRateLimiterConfig) *TokenBucketRateLimiter {
	return &TokenBucketRateLimiter{
		limiter:  rate.NewLimiter(conf.Limit, conf.Burst),
		lastUsed: time.Now(),
	}
}

func (c *TokenBucketRateLimiter) IsStale(d time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return time.Since(c.lastUsed) > d
}

func (c *TokenBucketRateLimiter) Wait(ctx context.Context) error {
	c.mu.Lock()
	c.lastUsed = time.Now()
	c.mu.Unlock()
	// The lock is released before waiting: rate.Limiter.Wait can block, and we
	// must not hold the mutex (which IsStale also needs) for that duration.
	return c.limiter.Wait(ctx)
}
