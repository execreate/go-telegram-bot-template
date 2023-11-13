package limiters

import (
	"context"
	"sync"
	"time"
)

type SlidingWindowRateLimiterPool struct {
	window    time.Duration
	maxEvents int
	limiters  map[int64]*SlidingWindowRateLimiter
	mu        *sync.RWMutex
}

func NewSlidingWindowRateLimiterPool(
	window time.Duration,
	maxEvents int,
	cleanUpInterval time.Duration,
	staleThreshold time.Duration,
) *SlidingWindowRateLimiterPool {
	pool := &SlidingWindowRateLimiterPool{
		window:    window,
		maxEvents: maxEvents,
		limiters:  make(map[int64]*SlidingWindowRateLimiter),
		mu:        &sync.RWMutex{},
	}

	go func(
		cleanUpInterval time.Duration,
		staleThreshold time.Duration,
	) {
		ticker := time.NewTicker(cleanUpInterval)
		for range ticker.C {
			pool.removeStaleLimiters(staleThreshold)
		}
	}(cleanUpInterval, staleThreshold)

	return pool
}

func (pool *SlidingWindowRateLimiterPool) WaitLimiter(ctx context.Context, limiterID int64) error {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	l, ok := pool.limiters[limiterID]
	if !ok {
		// If the chat is not in the map, create a new chat and add it to the map.
		l = NewSlidingWindowRateLimiter(pool.window, pool.maxEvents)
		pool.limiters[limiterID] = l
	}
	return l.Wait(ctx)
}

func (pool *SlidingWindowRateLimiterPool) removeStaleLimiters(staleDuration time.Duration) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	for limiterID, l := range pool.limiters {
		if l.IsStale(staleDuration) {
			delete(pool.limiters, limiterID)
		}
	}
}
