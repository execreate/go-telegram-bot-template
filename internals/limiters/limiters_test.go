package limiters

import (
	"context"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestNewSlidingWindowRateLimiterRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		conf *SlidingWindowRateLimiterConfig
	}{
		{name: "nil config", conf: nil},
		{name: "zero maxN", conf: &SlidingWindowRateLimiterConfig{Window: time.Minute, MaxN: 0}},
		{name: "negative maxN", conf: &SlidingWindowRateLimiterConfig{Window: time.Minute, MaxN: -1}},
		{name: "zero window", conf: &SlidingWindowRateLimiterConfig{Window: 0, MaxN: 20}},
		{name: "negative window", conf: &SlidingWindowRateLimiterConfig{Window: -time.Second, MaxN: 20}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter, err := NewSlidingWindowRateLimiter(tt.conf)
			if err == nil {
				t.Fatal("NewSlidingWindowRateLimiter() returned no error")
			}
			if limiter != nil {
				t.Error("NewSlidingWindowRateLimiter() returned a limiter alongside the error")
			}
		})
	}
}

func TestNewTokenBucketRateLimiterRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		conf *TokenBucketRateLimiterConfig
	}{
		{name: "nil config", conf: nil},
		{name: "zero burst", conf: &TokenBucketRateLimiterConfig{Limit: rate.Every(time.Second), Burst: 0}},
		{name: "negative burst", conf: &TokenBucketRateLimiterConfig{Limit: rate.Every(time.Second), Burst: -1}},
		{name: "zero limit", conf: &TokenBucketRateLimiterConfig{Limit: 0, Burst: 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter, err := NewTokenBucketRateLimiter(tt.conf)
			if err == nil {
				t.Fatal("NewTokenBucketRateLimiter() returned no error")
			}
			if limiter != nil {
				t.Error("NewTokenBucketRateLimiter() returned a limiter alongside the error")
			}
		})
	}
}

func TestNewRateLimiterPoolValidatesEagerly(t *testing.T) {
	// Limiters are built lazily, so an invalid config would otherwise only surface on
	// the first request to a chat.
	pool, err := NewRateLimiterPool[*SlidingWindowRateLimiter, *SlidingWindowRateLimiterConfig](
		NewSlidingWindowRateLimiter,
		&SlidingWindowRateLimiterConfig{Window: time.Minute, MaxN: 0},
		time.Hour,
		time.Hour,
	)

	if err == nil {
		t.Fatal("NewRateLimiterPool() with an invalid config returned no error")
	}
	if pool != nil {
		t.Error("NewRateLimiterPool() returned a pool alongside the error")
	}
}

func TestRateLimiterPoolSharesOneLimiterPerID(t *testing.T) {
	pool, err := NewRateLimiterPool[*TokenBucketRateLimiter, *TokenBucketRateLimiterConfig](
		NewTokenBucketRateLimiter,
		&TokenBucketRateLimiterConfig{Limit: rate.Every(time.Millisecond), Burst: 1},
		time.Hour,
		time.Hour,
	)
	if err != nil {
		t.Fatalf("NewRateLimiterPool() unexpected error: %v", err)
	}

	ctx := t.Context()
	for _, id := range []int64{1, 1, 2} {
		if err := pool.WaitLimiter(ctx, id); err != nil {
			t.Fatalf("WaitLimiter(%d) unexpected error: %v", id, err)
		}
	}

	pool.mu.RLock()
	defer pool.mu.RUnlock()
	if len(pool.limiters) != 2 {
		t.Errorf("pool holds %d limiters, want 2", len(pool.limiters))
	}
}

func TestRemoveStaleLimiters(t *testing.T) {
	pool, err := NewRateLimiterPool[*TokenBucketRateLimiter, *TokenBucketRateLimiterConfig](
		NewTokenBucketRateLimiter,
		&TokenBucketRateLimiterConfig{Limit: rate.Every(time.Millisecond), Burst: 1},
		time.Hour,
		time.Hour,
	)
	if err != nil {
		t.Fatalf("NewRateLimiterPool() unexpected error: %v", err)
	}

	if err := pool.WaitLimiter(t.Context(), 1); err != nil {
		t.Fatalf("WaitLimiter() unexpected error: %v", err)
	}

	// Nothing is stale against an hour-long threshold.
	pool.removeStaleLimiters(time.Hour)
	pool.mu.RLock()
	remaining := len(pool.limiters)
	pool.mu.RUnlock()
	if remaining != 1 {
		t.Fatalf("pool holds %d limiters after a no-op cleanup, want 1", remaining)
	}

	// Everything is stale against a zero threshold.
	pool.removeStaleLimiters(0)
	pool.mu.RLock()
	remaining = len(pool.limiters)
	pool.mu.RUnlock()
	if remaining != 0 {
		t.Errorf("pool holds %d limiters after cleanup, want 0", remaining)
	}
}

func TestPruneExpiredEvents(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		events      []time.Time
		windowStart time.Time
		wantLen     int
	}{
		{
			name:        "empty",
			events:      []time.Time{},
			windowStart: now,
			wantLen:     0,
		},
		{
			name:        "all expired",
			events:      []time.Time{now.Add(-3 * time.Minute), now.Add(-2 * time.Minute)},
			windowStart: now,
			wantLen:     0,
		},
		{
			name:        "some expired",
			events:      []time.Time{now.Add(-3 * time.Minute), now.Add(time.Minute)},
			windowStart: now,
			wantLen:     1,
		},
		{
			name:        "none expired",
			events:      []time.Time{now.Add(time.Minute), now.Add(2 * time.Minute)},
			windowStart: now,
			wantLen:     2,
		},
		{
			name:        "an event exactly at the window start is kept",
			events:      []time.Time{now},
			windowStart: now,
			wantLen:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pruneExpiredEvents(tt.events, tt.windowStart); len(got) != tt.wantLen {
				t.Errorf("pruneExpiredEvents() kept %d events, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestSlidingWindowRateLimiterAdmitsTheFirstMaxN(t *testing.T) {
	limiter, err := NewSlidingWindowRateLimiter(&SlidingWindowRateLimiterConfig{
		Window: time.Hour,
		MaxN:   3,
	})
	if err != nil {
		t.Fatalf("NewSlidingWindowRateLimiter() unexpected error: %v", err)
	}

	// The first MaxN events fire immediately; an hour-long window makes "immediately"
	// unambiguous without depending on how fast the test machine is.
	start := time.Now()
	for i := range 3 {
		if err := limiter.Wait(t.Context()); err != nil {
			t.Fatalf("Wait() %d unexpected error: %v", i, err)
		}
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("the first MaxN waits took %s, want them to return immediately", elapsed)
	}

	// The next one would have to wait out the window, which the context deadline
	// cannot cover — it must fail fast rather than sleep.
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	start = time.Now()
	if err := limiter.Wait(ctx); err == nil {
		t.Error("Wait() past MaxN with a short deadline returned no error")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("Wait() slept for %s before failing, want it to fail immediately", elapsed)
	}
}
