package limiters

import (
	"context"
	"fmt"
	"my-telegram-bot/internals/logger"
	"sync"
	"time"
)

type SlidingWindowRateLimiter struct {
	maxN     int
	window   time.Duration
	events   []time.Time
	lastUsed time.Time
	mu       sync.Mutex
}

func NewSlidingWindowRateLimiter(window time.Duration, maxN int) *SlidingWindowRateLimiter {
	if maxN <= 0 {
		logger.Log.Fatal().Int("maxN", maxN).Msg("maxN must be greater than 0")
	}
	if window <= 0 {
		logger.Log.Fatal().Dur("window", window).Msg("window must be greater than 0")
	}

	return &SlidingWindowRateLimiter{
		maxN:     maxN,
		window:   window,
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

	windowStart := now.Add(-1 * rl.window)

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

	if len(rl.events)+1 > rl.maxN {
		waitDuration := rl.events[0].Add(rl.window).Sub(now)
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
