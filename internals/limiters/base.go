package limiters

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type RateLimiter interface {
	Wait(ctx context.Context) error
	IsStale(d time.Duration) bool
}

type RateLimiterPool[T RateLimiter, Conf any] struct {
	limiterConfig Conf
	createLimiter func(Conf) (T, error)
	limiters      map[int64]T
	mu            sync.RWMutex
}

func NewRateLimiterPool[T RateLimiter, Conf any](
	createLimiter func(Conf) (T, error),
	limiterConfig Conf,
	cleanUpInterval time.Duration,
	staleThreshold time.Duration,
) (*RateLimiterPool[T, Conf], error) {
	// Limiters are created lazily, on the first request for a chat. Build one here and
	// throw it away so a bad config fails at startup rather than mid-request, hours in.
	if _, err := createLimiter(limiterConfig); err != nil {
		return nil, fmt.Errorf("invalid rate limiter config: %w", err)
	}

	pool := &RateLimiterPool[T, Conf]{
		limiters:      make(map[int64]T),
		createLimiter: createLimiter,
		limiterConfig: limiterConfig,
	}
	go pool.watchStaleLimiters(cleanUpInterval, staleThreshold)
	return pool, nil
}

func (pool *RateLimiterPool[T, Conf]) WaitLimiter(ctx context.Context, limiterID int64) error {
	// Get-or-create atomically under a single write lock so concurrent first
	// requests for the same ID share one limiter instead of racing to create
	// (and overwrite) separate ones. The lock is released before Wait, which blocks.
	pool.mu.Lock()
	limiter, ok := pool.limiters[limiterID]
	if !ok {
		newLimiter, err := pool.createLimiter(pool.limiterConfig)
		if err != nil {
			pool.mu.Unlock()
			return fmt.Errorf("failed to create rate limiter for %d: %w", limiterID, err)
		}
		limiter = newLimiter
		pool.limiters[limiterID] = limiter
	}
	pool.mu.Unlock()
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
