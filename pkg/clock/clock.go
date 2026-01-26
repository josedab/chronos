// Package clock provides a time abstraction for testability.
package clock

import "time"

// Clock is an interface for time operations, allowing for easy mocking in tests.
type Clock interface {
	// Now returns the current time.
	Now() time.Time
	// Since returns the time elapsed since t.
	Since(t time.Time) time.Duration
	// Until returns the duration until t.
	Until(t time.Time) time.Duration
	// NewTicker returns a new Ticker.
	NewTicker(d time.Duration) Ticker
	// NewTimer returns a new Timer.
	NewTimer(d time.Duration) Timer
	// After waits for the duration to elapse and then sends the current time on the returned channel.
	After(d time.Duration) <-chan time.Time
	// Sleep pauses the current goroutine for at least the duration d.
	Sleep(d time.Duration)
}

// Ticker wraps time.Ticker for mockability.
type Ticker interface {
	C() <-chan time.Time
	Stop()
	Reset(d time.Duration)
}

// Timer wraps time.Timer for mockability.
type Timer interface {
	C() <-chan time.Time
	Stop() bool
	Reset(d time.Duration) bool
}

// RealClock implements Clock using the standard time package.
type RealClock struct{}

// New returns a new RealClock.
func New() Clock {
	return &RealClock{}
}

// Now returns the current time.
func (c *RealClock) Now() time.Time {
	return time.Now()
}

// Since returns the time elapsed since t.
func (c *RealClock) Since(t time.Time) time.Duration {
	return time.Since(t)
}

// Until returns the duration until t.
func (c *RealClock) Until(t time.Time) time.Duration {
	return time.Until(t)
}

// NewTicker returns a new Ticker.
func (c *RealClock) NewTicker(d time.Duration) Ticker {
	return &realTicker{ticker: time.NewTicker(d)}
}

// NewTimer returns a new Timer.
func (c *RealClock) NewTimer(d time.Duration) Timer {
	return &realTimer{timer: time.NewTimer(d)}
}

// After waits for the duration to elapse and then sends the current time.
func (c *RealClock) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}

// Sleep pauses the current goroutine for at least the duration d.
func (c *RealClock) Sleep(d time.Duration) {
	time.Sleep(d)
}

// realTicker wraps time.Ticker.
type realTicker struct {
	ticker *time.Ticker
}

func (t *realTicker) C() <-chan time.Time {
	return t.ticker.C
}

func (t *realTicker) Stop() {
	t.ticker.Stop()
}

func (t *realTicker) Reset(d time.Duration) {
	t.ticker.Reset(d)
}

// realTimer wraps time.Timer.
type realTimer struct {
	timer *time.Timer
}

func (t *realTimer) C() <-chan time.Time {
	return t.timer.C
}

func (t *realTimer) Stop() bool {
	return t.timer.Stop()
}

func (t *realTimer) Reset(d time.Duration) bool {
	return t.timer.Reset(d)
}
