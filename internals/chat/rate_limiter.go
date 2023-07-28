package chat

import (
	"context"
	"golang.org/x/time/rate"
	"time"
)

type RateLimiter struct {
	limiter  *rate.Limiter
	lastUsed time.Time
}

func NewRateLimiter(limit rate.Limit, burst int) *RateLimiter {
	return &RateLimiter{
		limiter:  rate.NewLimiter(limit, burst),
		lastUsed: time.Now(),
	}
}

func (c *RateLimiter) IsStale(d time.Duration) bool {
	return time.Since(c.lastUsed) > d
}

func (c *RateLimiter) Wait(ctx context.Context) error {
	c.lastUsed = time.Now()
	c.limiter.Reserve()
	return c.limiter.Wait(ctx)
}

func (c *RateLimiter) GetWaitTime() time.Duration {
	c.lastUsed = time.Now()
	reservation := c.limiter.Reserve()
	defer reservation.Cancel()

	return reservation.Delay()
}
