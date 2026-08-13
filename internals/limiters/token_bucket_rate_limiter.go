package limiters

import (
	"context"
	"errors"
	"fmt"
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
	// now reads the current time. It is a field rather than a direct call to time.Now
	// so tests can probe the staleness boundary without sleeping.
	now func() time.Time
	mu  sync.Mutex
}

func NewTokenBucketRateLimiter(conf *TokenBucketRateLimiterConfig) (*TokenBucketRateLimiter, error) {
	if conf == nil {
		return nil, errors.New("token bucket rate limiter config must not be nil")
	}
	if conf.Burst <= 0 {
		// rate.Limiter with a zero burst never admits anything, which would stall every
		// call to the chat rather than slow it down.
		return nil, fmt.Errorf("burst must be greater than 0, got %d", conf.Burst)
	}
	if conf.Limit <= 0 {
		return nil, fmt.Errorf("limit must be greater than 0, got %v", conf.Limit)
	}

	return &TokenBucketRateLimiter{
		limiter:  rate.NewLimiter(conf.Limit, conf.Burst),
		lastUsed: time.Now(),
		now:      time.Now,
	}, nil
}

func (c *TokenBucketRateLimiter) IsStale(d time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now().Sub(c.lastUsed) > d
}

func (c *TokenBucketRateLimiter) Wait(ctx context.Context) error {
	c.mu.Lock()
	c.lastUsed = c.now()
	c.mu.Unlock()
	// The lock is released before waiting: rate.Limiter.Wait can block, and we
	// must not hold the mutex (which IsStale also needs) for that duration.
	return c.limiter.Wait(ctx)
}
