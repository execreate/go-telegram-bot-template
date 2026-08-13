package limiters

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

// newTestSlidingWindow builds a limiter driven by a manually advanced clock.
func newTestSlidingWindow(t *testing.T, window time.Duration, maxN int) (*SlidingWindowRateLimiter, *fakeClock) {
	t.Helper()

	limiter, err := NewSlidingWindowRateLimiter(&SlidingWindowRateLimiterConfig{
		Window: window,
		MaxN:   maxN,
	})
	if err != nil {
		t.Fatalf("NewSlidingWindowRateLimiter() unexpected error: %v", err)
	}

	clock := newFakeClock()
	limiter.now = clock.Now
	limiter.lastUsed = clock.Now()

	return limiter, clock
}

func TestSlidingWindowAdmitsTheFirstMaxNWithoutWaiting(t *testing.T) {
	limiter, _ := newTestSlidingWindow(t, time.Minute, 3)

	// The clock never advances, so anything that returned had to return without
	// waiting — there is no tolerance to tune.
	for i := range 3 {
		if err := limiter.Wait(t.Context()); err != nil {
			t.Fatalf("Wait() %d unexpected error: %v", i, err)
		}
	}

	if len(limiter.events) != 3 {
		t.Errorf("limiter recorded %d events, want 3", len(limiter.events))
	}
}

func TestSlidingWindowMakesTheNextCallWaitAWholeWindow(t *testing.T) {
	const window = time.Minute

	limiter, clock := newTestSlidingWindow(t, window, 2)

	start := clock.Now()
	for i := range 2 {
		if err := limiter.Wait(t.Context()); err != nil {
			t.Fatalf("Wait() %d unexpected error: %v", i, err)
		}
	}

	// The budget is spent, so the next call has to wait for the first event to leave
	// the window. The wait is pinned from both sides without sleeping through it.

	// Lower bound: half a window is not enough, so the deadline check rejects it.
	ctx, cancel := context.WithDeadline(t.Context(), start.Add(window/2))
	defer cancel()

	if err := limiter.Wait(ctx); err == nil {
		t.Fatal("Wait() past MaxN with a deadline half a window out returned no error")
	}

	// A rejected call must not book a slot, or the budget would leak away on every
	// caller that gave up.
	limiter.mu.Lock()
	booked := len(limiter.events)
	limiter.mu.Unlock()
	if booked != 2 {
		t.Errorf("the rejected call left %d events booked, want 2", booked)
	}

	// Upper bound: exactly one window later the call is admitted with no wait, and is
	// scheduled for that instant. So the required wait was exactly one window.
	clock.Advance(window)

	if err := limiter.Wait(t.Context()); err != nil {
		t.Fatalf("Wait() one window later returned an error: %v", err)
	}

	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if scheduled := limiter.events[len(limiter.events)-1]; !scheduled.Equal(start.Add(window)) {
		t.Errorf("the admitted event is scheduled for %s, want %s", scheduled, start.Add(window))
	}
}

func TestSlidingWindowAdmitsAgainOnceTheWindowSlides(t *testing.T) {
	const window = time.Minute

	limiter, clock := newTestSlidingWindow(t, window, 2)

	for i := range 2 {
		if err := limiter.Wait(t.Context()); err != nil {
			t.Fatalf("Wait() %d unexpected error: %v", i, err)
		}
	}

	// Move past the window so both events expire. The next call is admitted with no
	// wait at all, which is the whole point of the window sliding.
	clock.Advance(window + time.Second)

	if err := limiter.Wait(t.Context()); err != nil {
		t.Fatalf("Wait() after the window slid returned an error: %v", err)
	}

	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if len(limiter.events) != 1 {
		t.Errorf("limiter holds %d events, want the expired ones pruned", len(limiter.events))
	}
	if !limiter.events[0].Equal(clock.Now()) {
		t.Errorf("the admitted event is scheduled for %s, want now (%s)", limiter.events[0], clock.Now())
	}
}

func TestSlidingWindowFailsFastWhenTheDeadlineIsTooShort(t *testing.T) {
	const window = time.Hour

	limiter, clock := newTestSlidingWindow(t, window, 1)

	if err := limiter.Wait(t.Context()); err != nil {
		t.Fatalf("Wait() unexpected error: %v", err)
	}

	ctx, cancel := context.WithDeadline(t.Context(), clock.Now().Add(time.Minute))
	defer cancel()

	// A real sleep here would be an hour long, so returning at all proves it did not
	// start one.
	done := make(chan error, 1)
	go func() { done <- limiter.Wait(ctx) }()

	select {
	case err := <-done:
		if err == nil {
			t.Error("Wait() returned no error for a deadline inside the wait")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Wait() slept instead of failing fast on a short deadline")
	}
}

func TestSlidingWindowReturnsContextErrWhenCancelledMidWait(t *testing.T) {
	// A real (short) window: this is the one path that genuinely sleeps, and the point
	// is that cancelling cuts the sleep short.
	limiter, err := NewSlidingWindowRateLimiter(&SlidingWindowRateLimiterConfig{
		Window: 30 * time.Second,
		MaxN:   1,
	})
	if err != nil {
		t.Fatalf("NewSlidingWindowRateLimiter() unexpected error: %v", err)
	}

	if err := limiter.Wait(t.Context()); err != nil {
		t.Fatalf("Wait() unexpected error: %v", err)
	}

	// No deadline, so the fail-fast branch does not apply and the call sleeps.
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- limiter.Wait(ctx) }()

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Wait() error = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Wait() ignored its cancelled context and kept sleeping")
	}
}

func TestSlidingWindowIsStale(t *testing.T) {
	limiter, clock := newTestSlidingWindow(t, time.Minute, 1)

	if limiter.IsStale(time.Hour) {
		t.Error("IsStale() = true for a limiter that was just created")
	}

	// Exactly at the threshold is not yet stale — the comparison is strictly greater.
	clock.Advance(time.Hour)
	if limiter.IsStale(time.Hour) {
		t.Error("IsStale() = true exactly at the threshold, want false")
	}

	clock.Advance(time.Nanosecond)
	if !limiter.IsStale(time.Hour) {
		t.Error("IsStale() = false past the threshold")
	}

	// Any use refreshes it.
	if err := limiter.Wait(t.Context()); err != nil {
		t.Fatalf("Wait() unexpected error: %v", err)
	}
	if limiter.IsStale(time.Hour) {
		t.Error("IsStale() = true right after Wait refreshed lastUsed")
	}
}

func newTestTokenBucket(t *testing.T, limit rate.Limit, burst int) (*TokenBucketRateLimiter, *fakeClock) {
	t.Helper()

	limiter, err := NewTokenBucketRateLimiter(&TokenBucketRateLimiterConfig{Limit: limit, Burst: burst})
	if err != nil {
		t.Fatalf("NewTokenBucketRateLimiter() unexpected error: %v", err)
	}

	clock := newFakeClock()
	limiter.now = clock.Now
	limiter.lastUsed = clock.Now()

	return limiter, clock
}

func TestTokenBucketAdmitsTheBurstThenRateLimits(t *testing.T) {
	// One token per hour with a burst of one: the first call spends the burst, the
	// second would have to wait an hour.
	limiter, _ := newTestTokenBucket(t, rate.Every(time.Hour), 1)

	if err := limiter.Wait(t.Context()); err != nil {
		t.Fatalf("the first Wait() spent the burst but errored: %v", err)
	}

	// rate.Limiter keeps its own clock, so this asserts the refusal rather than a
	// duration: with a deadline sooner than the next token, Wait must not sleep.
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	if err := limiter.Wait(ctx); err == nil {
		t.Error("the second Wait() was admitted, want it rate limited")
	}
}

func TestTokenBucketWaitRefreshesLastUsed(t *testing.T) {
	limiter, clock := newTestTokenBucket(t, rate.Every(time.Millisecond), 10)

	clock.Advance(2 * time.Hour)
	if !limiter.IsStale(time.Hour) {
		t.Fatal("IsStale() = false after two idle hours")
	}

	if err := limiter.Wait(t.Context()); err != nil {
		t.Fatalf("Wait() unexpected error: %v", err)
	}

	// Wait stamps lastUsed, so the limiter is no longer a cleanup candidate.
	if limiter.IsStale(time.Hour) {
		t.Error("IsStale() = true right after Wait refreshed lastUsed")
	}
}

func TestTokenBucketIsStaleBoundary(t *testing.T) {
	limiter, clock := newTestTokenBucket(t, rate.Every(time.Millisecond), 1)

	if limiter.IsStale(time.Hour) {
		t.Error("IsStale() = true for a limiter that was just created")
	}

	clock.Advance(time.Hour)
	if limiter.IsStale(time.Hour) {
		t.Error("IsStale() = true exactly at the threshold, want false")
	}

	clock.Advance(time.Nanosecond)
	if !limiter.IsStale(time.Hour) {
		t.Error("IsStale() = false past the threshold")
	}
}
