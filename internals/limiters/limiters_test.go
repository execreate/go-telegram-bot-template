package limiters

import (
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
			got := pruneExpiredEvents(tt.events, tt.windowStart)

			if len(got) != tt.wantLen {
				t.Fatalf("pruneExpiredEvents() kept %d events, want %d", len(got), tt.wantLen)
			}
			// What survives is the tail of the input, so the retained events must be the
			// last wantLen entries, still in order.
			for i, want := range tt.events[len(tt.events)-tt.wantLen:] {
				if !got[i].Equal(want) {
					t.Errorf("kept event %d = %s, want %s", i, got[i], want)
				}
			}
		})
	}
}

func TestPruneExpiredEventsReusesTheBackingArray(t *testing.T) {
	now := time.Now()

	// Both branches return a slice of the input's backing array — events[i:] when
	// something survives, events[:0] when nothing does. The caller reassigns rl.events
	// to the result and appends to it, so the reuse is intended: it keeps a busy chat
	// from allocating a fresh slice on every request.
	t.Run("all expired", func(t *testing.T) {
		events := make([]time.Time, 0, 8)
		events = append(events, now.Add(-3*time.Minute), now.Add(-2*time.Minute))

		pruned := pruneExpiredEvents(events, now)

		if len(pruned) != 0 {
			t.Fatalf("pruneExpiredEvents() kept %d events, want 0", len(pruned))
		}
		if cap(pruned) != cap(events) {
			t.Errorf("cap = %d, want the input's %d — the backing array was not reused", cap(pruned), cap(events))
		}

		// Appending overwrites the expired entries rather than resurrecting them.
		next := now.Add(time.Minute)
		pruned = append(pruned, next)
		if len(pruned) != 1 || !pruned[0].Equal(next) {
			t.Errorf("after append, pruned = %v, want only the new event", pruned)
		}
	})

	t.Run("some expired", func(t *testing.T) {
		survivor := now.Add(time.Minute)
		events := []time.Time{now.Add(-3 * time.Minute), survivor}

		pruned := pruneExpiredEvents(events, now)

		if len(pruned) != 1 || !pruned[0].Equal(survivor) {
			t.Fatalf("pruneExpiredEvents() = %v, want only the surviving event", pruned)
		}

		// The window slides forward, so an event appended after the prune must land
		// after the survivor, never on top of it.
		next := now.Add(2 * time.Minute)
		pruned = append(pruned, next)
		if len(pruned) != 2 || !pruned[0].Equal(survivor) || !pruned[1].Equal(next) {
			t.Errorf("after append, pruned = %v, want [%s %s]", pruned, survivor, next)
		}
	})
}
