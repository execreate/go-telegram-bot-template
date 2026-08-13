package limiters

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type SlidingWindowRateLimiterConfig struct {
	Window time.Duration
	MaxN   int
}

type SlidingWindowRateLimiter struct {
	conf     *SlidingWindowRateLimiterConfig
	events   []time.Time
	lastUsed time.Time
	// now reads the current time. It is a field rather than a direct call to time.Now
	// so tests can drive the window forward without sleeping through it.
	now func() time.Time
	mu  sync.Mutex
}

func NewSlidingWindowRateLimiter(conf *SlidingWindowRateLimiterConfig) (*SlidingWindowRateLimiter, error) {
	if conf == nil {
		return nil, errors.New("sliding window rate limiter config must not be nil")
	}
	if conf.MaxN <= 0 {
		return nil, fmt.Errorf("maxN must be greater than 0, got %d", conf.MaxN)
	}
	if conf.Window <= 0 {
		return nil, fmt.Errorf("window must be greater than 0, got %s", conf.Window)
	}

	return &SlidingWindowRateLimiter{
		conf:     conf,
		lastUsed: time.Now(),
		events:   make([]time.Time, 0),
		now:      time.Now,
	}, nil
}

func (rl *SlidingWindowRateLimiter) IsStale(d time.Duration) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return rl.now().Sub(rl.lastUsed) > d
}

func (rl *SlidingWindowRateLimiter) Wait(ctx context.Context) error {
	// Reserve a slot under the lock, then release it before sleeping. Holding the
	// mutex across the wait would block every other caller for this chat as well as
	// the stale-cleanup goroutine (IsStale) for up to a full window.
	rl.mu.Lock()

	now := rl.now()
	rl.lastUsed = now

	windowStart := now.Add(-1 * rl.conf.Window)
	rl.events = pruneExpiredEvents(rl.events, windowStart)

	// Determine when this event is allowed to fire. Events are stored as the times
	// at which they are scheduled to fire, so the slice stays sorted ascending.
	var eventTime time.Time
	if len(rl.events) < rl.conf.MaxN {
		eventTime = now
	} else {
		// The event MaxN slots back must leave the window before this one may fire,
		// guaranteeing at most MaxN events in any window of length Window.
		eventTime = rl.events[len(rl.events)-rl.conf.MaxN].Add(rl.conf.Window)
	}

	waitDuration := eventTime.Sub(now)
	if waitDuration > 0 {
		if deadline, ok := ctx.Deadline(); ok && deadline.Before(eventTime) {
			rl.mu.Unlock()
			return fmt.Errorf(
				"context deadline would reach before an event is allowed! "+
					"wait duration is %.2fs, but deadline in %.2fs",
				waitDuration.Seconds(),
				deadline.Sub(now).Seconds(),
			)
		}
	}

	rl.events = append(rl.events, eventTime)
	rl.mu.Unlock()

	if waitDuration <= 0 {
		return nil
	}

	timer := time.NewTimer(waitDuration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// pruneExpiredEvents drops the leading events that have fallen out of the window.
// The slice is sorted ascending, so the first event at or after windowStart marks
// the start of the events still in play.
func pruneExpiredEvents(events []time.Time, windowStart time.Time) []time.Time {
	for i := range events {
		if !events[i].Before(windowStart) {
			return events[i:]
		}
	}
	return events[:0]
}
