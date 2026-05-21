package limiters

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/execreate/go-telegram-bot-template/internals/logger"
	"go.uber.org/zap"
)

type SlidingWindowRateLimiterConfig struct {
	Window time.Duration
	MaxN   int
}

type SlidingWindowRateLimiter struct {
	conf     *SlidingWindowRateLimiterConfig
	events   []time.Time
	lastUsed time.Time
	mu       sync.Mutex
}

func NewSlidingWindowRateLimiter(conf *SlidingWindowRateLimiterConfig) *SlidingWindowRateLimiter {
	if conf.MaxN <= 0 {
		logger.Log.Fatal(
			"maxN must be greater than 0",
			zap.Int("maxN", conf.MaxN),
		)
	}
	if conf.Window <= 0 {
		logger.Log.Fatal(
			"window must be greater than 0",
			zap.Duration("window", conf.Window),
		)
	}

	return &SlidingWindowRateLimiter{
		conf:     conf,
		lastUsed: time.Now(),
		events:   make([]time.Time, 0),
	}
}

func (rl *SlidingWindowRateLimiter) IsStale(d time.Duration) bool {
	return time.Since(rl.lastUsed) > d
}

func (rl *SlidingWindowRateLimiter) Wait(ctx context.Context) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	rl.lastUsed = now

	windowStart := now.Add(-1 * rl.conf.Window)

	if len(rl.events) > 0 {
		if rl.events[len(rl.events)-1].Before(windowStart) {
			rl.events = make([]time.Time, 0)
		} else {
			for i := range rl.events {
				if rl.events[i].Before(windowStart) {
					continue
				} else {
					rl.events = rl.events[i:]
					break
				}
			}
		}
	}

	if len(rl.events)+1 > rl.conf.MaxN {
		waitDuration := rl.events[0].Add(rl.conf.Window).Sub(now)
		if waitDuration > 0 {
			timer := time.NewTimer(waitDuration)
			defer timer.Stop()

			if deadline, ok := ctx.Deadline(); ok {
				if deadline.Sub(now) < waitDuration {
					return fmt.Errorf(
						"context deadline would reach before an event is allowed! "+
							"wait duration is %.2fs, but deadline in %.2fs",
						waitDuration.Seconds(),
						deadline.Sub(now).Seconds(),
					)
				}
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	rl.events = append(rl.events, now)
	return nil
}
