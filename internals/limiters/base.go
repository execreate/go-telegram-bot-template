package limiters

import (
	"context"
	"sync"
	"time"
)

type RateLimiter interface {
	Wait(ctx context.Context) error
	IsStale(d time.Duration) bool
}

type RateLimiterPool[T RateLimiter, Conf any] struct {
	limiters map[int64]T
	config   Conf
	create   func(Conf) T
	mu       sync.RWMutex
}

func NewRateLimiterPool[T RateLimiter, Conf any](
	create func(Conf) T,
	config Conf,
	cleanUpInterval time.Duration,
	staleThreshold time.Duration,
) *RateLimiterPool[T, Conf] {
	pool := &RateLimiterPool[T, Conf]{
		limiters: make(map[int64]T),
		create:   create,
		config:   config,
	}
	go pool.watchStaleLimiters(cleanUpInterval, staleThreshold)
	return pool
}

func (pool *RateLimiterPool[T, Conf]) WaitLimiter(ctx context.Context, limiterID int64) error {
	pool.mu.RLock()
	var limiter T
	if l, ok := pool.limiters[limiterID]; ok {
		pool.mu.RUnlock()
		limiter = l
	} else {
		pool.mu.RUnlock()
		limiter = pool.create(pool.config)
		go pool.addLimiter(limiterID, limiter)
	}
	return limiter.Wait(ctx)
}

func (pool *RateLimiterPool[T, Conf]) watchStaleLimiters(cleanUpInterval, staleThreshold time.Duration) {
	ticker := time.NewTicker(cleanUpInterval)
	for range ticker.C {
		pool.removeStaleLimiters(staleThreshold)
	}
}

func (pool *RateLimiterPool[T, Conf]) removeStaleLimiters(staleDuration time.Duration) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	for limiterID, l := range pool.limiters {
		if l.IsStale(staleDuration) {
			delete(pool.limiters, limiterID)
		}
	}
}

func (pool *RateLimiterPool[T, Conf]) addLimiter(limiterID int64, limiter T) {
	pool.mu.Lock()
	pool.limiters[limiterID] = limiter
	pool.mu.Unlock()
}
