package limiters

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
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
	if l, ok := pool.limiters[limiterID]; ok {
		defer pool.mu.RUnlock()
		return l.Wait(ctx)
	} else {
		// limiter for the given ID is not found, let's create a new one,
		// unlock read lock and acquire write lock to create a new limiter
		pool.mu.RUnlock()
		pool.mu.Lock()
		defer pool.mu.Unlock()
		l = NewTokenBucketRateLimiter(pool.limiterRate, pool.limiterBurst)
		pool.limiters[limiterID] = l
		return l.Wait(ctx)
	}
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
