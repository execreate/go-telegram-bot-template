package limiters

import (
	"context"
	"golang.org/x/time/rate"
	"sync"
	"time"
)

type TokenBucketRateLimiterPool struct {
	limiterRate  rate.Limit
	limiterBurst int
	limiters     map[int64]*TokenBucketRateLimiter
	mu           *sync.RWMutex
}

func NewTokenBucketRateLimiterPool(
	limiterRate rate.Limit,
	limiterBurst int,
	cleanUpInterval time.Duration,
	staleThreshold time.Duration,
) *TokenBucketRateLimiterPool {
	pool := &TokenBucketRateLimiterPool{
		limiterRate:  limiterRate,
		limiterBurst: limiterBurst,
		limiters:     make(map[int64]*TokenBucketRateLimiter),
		mu:           &sync.RWMutex{},
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

func (pool *TokenBucketRateLimiterPool) WaitLimiter(ctx context.Context, limiterID int64) error {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	l, ok := pool.limiters[limiterID]
	if !ok {
		// If the chat is not in the map, create a new chat and add it to the map.
		l = NewTokenBucketRateLimiter(pool.limiterRate, pool.limiterBurst)
		pool.limiters[limiterID] = l
	}
	return l.Wait(ctx)
}

func (pool *TokenBucketRateLimiterPool) GetLimiterWaitTime(limiterID int64) time.Duration {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	l, ok := pool.limiters[limiterID]
	if !ok {
		// If the chat is not in the map, create a new chat and add it to the map.
		l = NewTokenBucketRateLimiter(pool.limiterRate, pool.limiterBurst)
		pool.limiters[limiterID] = l
	}
	return l.GetWaitTime()
}

func (pool *TokenBucketRateLimiterPool) removeStaleLimiters(staleDuration time.Duration) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	for limiterID, l := range pool.limiters {
		if l.IsStale(staleDuration) {
			delete(pool.limiters, limiterID)
		}
	}
}
