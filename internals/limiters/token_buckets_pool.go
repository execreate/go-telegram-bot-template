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
	var limiter *TokenBucketRateLimiter
	if l, ok := pool.limiters[limiterID]; ok {
		defer pool.mu.RUnlock()
		limiter = l
	} else {
		pool.mu.RUnlock()
		pool.mu.Lock()
		defer pool.mu.Unlock()
		limiter = NewTokenBucketRateLimiter(pool.limiterRate, pool.limiterBurst)
		pool.limiters[limiterID] = limiter
	}
	return limiter.Wait(ctx)
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
