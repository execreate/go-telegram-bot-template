package chat

import (
	"context"
	"golang.org/x/time/rate"
	"sync"
	"time"
)

type RateLimiterPool struct {
	limiterRate  rate.Limit
	limiterBurst int
	limiters     map[int64]*RateLimiter
	mu           *sync.RWMutex
}

func NewRateLimiterPool(
	limiterRate rate.Limit,
	limiterBurst int,
	staleDuration time.Duration,
	cleanUpInterval time.Duration,
) *RateLimiterPool {
	pool := &RateLimiterPool{
		limiterRate:  limiterRate,
		limiterBurst: limiterBurst,
		limiters:     make(map[int64]*RateLimiter),
		mu:           &sync.RWMutex{},
	}

	go func(
		staleDuration time.Duration,
		cleanUpInterval time.Duration,
	) {
		// Every 24 hours, check for stale chats and remove them from the map.
		ticker := time.NewTicker(cleanUpInterval)
		for range ticker.C {
			pool.removeStaleLimiters(staleDuration)
		}
	}(staleDuration, cleanUpInterval)

	return pool
}

func (pool *RateLimiterPool) WaitLimiter(ctx context.Context, limiterID int64) error {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	l, ok := pool.limiters[limiterID]
	if !ok {
		// If the chat is not in the map, create a new chat and add it to the map.
		l = NewRateLimiter(pool.limiterRate, pool.limiterBurst)
		pool.limiters[limiterID] = l
	}
	return l.Wait(ctx)
}

func (pool *RateLimiterPool) GetLimiterWaitTime(limiterID int64) time.Duration {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	l, ok := pool.limiters[limiterID]
	if !ok {
		// If the chat is not in the map, create a new chat and add it to the map.
		l = NewRateLimiter(pool.limiterRate, pool.limiterBurst)
		pool.limiters[limiterID] = l
	}
	return l.GetWaitTime()
}

func (pool *RateLimiterPool) removeStaleLimiters(staleDuration time.Duration) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	for limiterID, l := range pool.limiters {
		if l.IsStale(staleDuration) {
			delete(pool.limiters, limiterID)
		}
	}
}
