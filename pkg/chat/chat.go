package chat

import (
	"context"
	"golang.org/x/time/rate"
	"time"
)

type Chat struct {
	limiter  *rate.Limiter
	lastUsed time.Time
}

func NewChat() *Chat {
	return &Chat{
		limiter:  rate.NewLimiter(rate.Every(time.Second), 1),
		lastUsed: time.Now(),
	}
}

func (c *Chat) IsStale() bool {
	return time.Since(c.lastUsed) > time.Hour*24
}

func (c *Chat) WaitLimiter(ctx context.Context) error {
	c.lastUsed = time.Now()
	return c.limiter.Wait(ctx)
}
