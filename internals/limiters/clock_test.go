package limiters

import (
	"sync"
	"time"
)

// fakeClock is a manually advanced clock. Both limiters read the time through a
// function field, so a test can move the window forward instantly instead of
// sleeping through it — wall-clock waits are what make rate limiter tests flaky.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock {
	// A fixed instant, so a failure message reads the same on every run.
	return &fakeClock{t: time.Date(2026, time.August, 13, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}
