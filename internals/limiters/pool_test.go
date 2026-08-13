package limiters

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// stubLimiter is a RateLimiter whose behaviour a test dictates outright, so the pool
// can be exercised without a real limiter's timing.
type stubLimiter struct {
	waitErr error
	stale   bool
	waits   atomic.Int64
	// block, when non-nil, holds Wait until it is closed.
	block chan struct{}
}

func (s *stubLimiter) Wait(ctx context.Context) error {
	s.waits.Add(1)
	if s.block != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.block:
		}
	}
	return s.waitErr
}

func (s *stubLimiter) IsStale(time.Duration) bool { return s.stale }

// stubConfig carries whatever the test wants the factory to do.
type stubConfig struct {
	// created counts factory calls, which is how "exactly once per ID" is checked.
	created  *atomic.Int64
	limiters chan *stubLimiter
	err      error
}

func newStubFactory(conf *stubConfig) func(*stubConfig) (*stubLimiter, error) {
	return func(c *stubConfig) (*stubLimiter, error) {
		c.created.Add(1)
		if c.err != nil {
			return nil, c.err
		}
		select {
		case limiter := <-c.limiters:
			return limiter, nil
		default:
			return &stubLimiter{}, nil
		}
	}
}

// newStubPool builds a pool and returns it alongside a function that queues the
// limiters later WaitLimiter calls should receive. Queuing happens after
// construction on purpose: NewRateLimiterPool validates the config by building one
// limiter and discarding it, which would otherwise swallow the first queued stub.
func newStubPool(
	t *testing.T,
	conf *stubConfig,
) (*RateLimiterPool[*stubLimiter, *stubConfig], func(...*stubLimiter)) {
	t.Helper()

	conf.limiters = make(chan *stubLimiter, 8)

	pool, err := NewRateLimiterPool(newStubFactory(conf), conf, time.Hour, time.Hour)
	if err != nil {
		t.Fatalf("NewRateLimiterPool() unexpected error: %v", err)
	}

	return pool, func(limiters ...*stubLimiter) {
		t.Helper()
		for _, limiter := range limiters {
			conf.limiters <- limiter
		}
	}
}

func TestRateLimiterPoolCreatesOneLimiterPerIDUnderConcurrency(t *testing.T) {
	const goroutines = 50

	created := &atomic.Int64{}
	conf := &stubConfig{created: created}
	pool, _ := newStubPool(t, conf)

	// NewRateLimiterPool validates the config by building one limiter and discarding
	// it, so the factory has already run once before any request arrives.
	baseline := created.Load()
	if baseline != 1 {
		t.Fatalf("the pool constructor called the factory %d times, want 1", baseline)
	}

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			if err := pool.WaitLimiter(t.Context(), 1); err != nil {
				t.Errorf("WaitLimiter() unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	// Every goroutine hit the same ID, so they must all share one limiter — a racing
	// get-or-create would build several and let each caller through its own budget.
	if got := created.Load() - baseline; got != 1 {
		t.Errorf("the factory ran %d times for one ID, want 1", got)
	}

	pool.mu.RLock()
	limiter := pool.limiters[1]
	total := len(pool.limiters)
	pool.mu.RUnlock()

	if total != 1 {
		t.Errorf("pool holds %d limiters, want 1", total)
	}
	if got := limiter.waits.Load(); got != goroutines {
		t.Errorf("the shared limiter saw %d waits, want %d", got, goroutines)
	}
}

func TestRateLimiterPoolCreatesOneLimiterPerDistinctID(t *testing.T) {
	created := &atomic.Int64{}
	conf := &stubConfig{created: created}
	pool, _ := newStubPool(t, conf)
	baseline := created.Load()

	for _, id := range []int64{1, 2, 3, 1, 2} {
		if err := pool.WaitLimiter(t.Context(), id); err != nil {
			t.Fatalf("WaitLimiter(%d) unexpected error: %v", id, err)
		}
	}

	if got := created.Load() - baseline; got != 3 {
		t.Errorf("the factory ran %d times for 3 distinct IDs, want 3", got)
	}
}

func TestRateLimiterPoolPropagatesFactoryFailures(t *testing.T) {
	wantErr := errors.New("bad config")
	created := &atomic.Int64{}

	// The constructor validates eagerly, so a factory that always fails cannot even
	// build a pool. Let the first call through, then start failing.
	conf := &stubConfig{created: created}
	pool, _ := newStubPool(t, conf)
	conf.err = wantErr

	err := pool.WaitLimiter(t.Context(), 1)

	if !errors.Is(err, wantErr) {
		t.Errorf("WaitLimiter() error = %v, want it to wrap %v", err, wantErr)
	}

	// A limiter that could not be built must not be cached, or the ID would be stuck
	// with a broken entry.
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	if len(pool.limiters) != 0 {
		t.Error("the pool cached an entry for a failed creation")
	}
}

func TestRateLimiterPoolPropagatesWaitFailures(t *testing.T) {
	wantErr := errors.New("limiter refused")

	pool, queue := newStubPool(t, &stubConfig{created: &atomic.Int64{}})
	queue(&stubLimiter{waitErr: wantErr})

	if err := pool.WaitLimiter(t.Context(), 1); !errors.Is(err, wantErr) {
		t.Errorf("WaitLimiter() error = %v, want %v", err, wantErr)
	}
}

func TestRateLimiterPoolPropagatesContextCancellation(t *testing.T) {
	blocked := &stubLimiter{block: make(chan struct{})}
	defer close(blocked.block)

	pool, queue := newStubPool(t, &stubConfig{created: &atomic.Int64{}})
	queue(blocked)

	ctx, cancel := context.WithCancel(t.Context())
	errCh := make(chan error, 1)
	go func() { errCh <- pool.WaitLimiter(ctx, 1) }()

	// The lock must be released before Wait blocks, or cancelling would not be enough
	// to get the caller back — the pool would still be holding every other caller.
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("WaitLimiter() error = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("WaitLimiter() did not return after its context was cancelled")
	}
}

func TestRateLimiterPoolDoesNotHoldTheLockWhileWaiting(t *testing.T) {
	blocked := &stubLimiter{block: make(chan struct{})}

	pool, queue := newStubPool(t, &stubConfig{created: &atomic.Int64{}})
	queue(blocked)

	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(started)
		done <- pool.WaitLimiter(t.Context(), 1)
	}()
	<-started

	// Another chat must not be stuck behind the blocked one. Poll rather than sleep:
	// the goroutine above may not have reached Wait yet.
	deadline := time.Now().Add(5 * time.Second)
	for blocked.waits.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the first caller never reached Wait")
		}
		time.Sleep(time.Millisecond)
	}

	secondDone := make(chan error, 1)
	go func() { secondDone <- pool.WaitLimiter(t.Context(), 2) }()

	select {
	case err := <-secondDone:
		if err != nil {
			t.Errorf("WaitLimiter() for the second chat errored: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a second chat was blocked behind the first chat's wait")
	}

	close(blocked.block)
	if err := <-done; err != nil {
		t.Errorf("WaitLimiter() unexpected error: %v", err)
	}
}

func TestRemoveStaleLimitersWithAStub(t *testing.T) {
	fresh := &stubLimiter{stale: false}
	stale := &stubLimiter{stale: true}

	pool, queue := newStubPool(t, &stubConfig{created: &atomic.Int64{}})
	queue(fresh, stale)

	for _, id := range []int64{1, 2} {
		if err := pool.WaitLimiter(t.Context(), id); err != nil {
			t.Fatalf("WaitLimiter(%d) unexpected error: %v", id, err)
		}
	}

	pool.removeStaleLimiters(time.Hour)

	pool.mu.RLock()
	defer pool.mu.RUnlock()

	if _, ok := pool.limiters[1]; !ok {
		t.Error("the fresh limiter was evicted")
	}
	if _, ok := pool.limiters[2]; ok {
		t.Error("the stale limiter was kept")
	}
}

func TestWatchStaleLimitersRunsOnTheInterval(t *testing.T) {
	conf := &stubConfig{created: &atomic.Int64{}, limiters: make(chan *stubLimiter, 1)}

	// A short interval so the background cleanup fires during the test; this is the
	// one place a real ticker is unavoidable.
	pool, err := NewRateLimiterPool(newStubFactory(conf), conf, 10*time.Millisecond, time.Hour)
	if err != nil {
		t.Fatalf("NewRateLimiterPool() unexpected error: %v", err)
	}
	conf.limiters <- &stubLimiter{stale: true}

	if err := pool.WaitLimiter(t.Context(), 1); err != nil {
		t.Fatalf("WaitLimiter() unexpected error: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		pool.mu.RLock()
		remaining := len(pool.limiters)
		pool.mu.RUnlock()

		if remaining == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the cleanup goroutine never evicted the stale limiter")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
