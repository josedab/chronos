// Package clock provides a time abstraction for testability.
package clock

import (
	"sync"
	"time"
)

// MockClock is a Clock implementation for testing that allows manual time control.
type MockClock struct {
	mu      sync.RWMutex
	current time.Time
	timers  []*mockTimer
	tickers []*mockTicker
}

// NewMock returns a new MockClock set to the given time.
func NewMock(t time.Time) *MockClock {
	return &MockClock{current: t}
}

// Now returns the mock's current time.
func (c *MockClock) Now() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.current
}

// Since returns the time elapsed since t.
func (c *MockClock) Since(t time.Time) time.Duration {
	return c.Now().Sub(t)
}

// Until returns the duration until t.
func (c *MockClock) Until(t time.Time) time.Duration {
	return t.Sub(c.Now())
}

// Set sets the mock clock's time.
func (c *MockClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current = t
	c.checkTimers()
}

// Add advances the mock clock by the given duration.
func (c *MockClock) Add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current = c.current.Add(d)
	c.checkTimers()
}

// checkTimers fires any timers/tickers that have elapsed.
func (c *MockClock) checkTimers() {
	for _, t := range c.timers {
		if !t.fired && !t.stopped && !c.current.Before(t.deadline) {
			t.fired = true
			select {
			case t.ch <- c.current:
			default:
			}
		}
	}
	for _, t := range c.tickers {
		if t.stopped {
			continue
		}
		for !c.current.Before(t.next) {
			select {
			case t.ch <- c.current:
			default:
			}
			t.next = t.next.Add(t.interval)
		}
	}
}

// NewTicker returns a new mock Ticker.
func (c *MockClock) NewTicker(d time.Duration) Ticker {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &mockTicker{
		ch:       make(chan time.Time, 1),
		interval: d,
		next:     c.current.Add(d),
	}
	c.tickers = append(c.tickers, t)
	return t
}

// NewTimer returns a new mock Timer.
func (c *MockClock) NewTimer(d time.Duration) Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &mockTimer{
		ch:       make(chan time.Time, 1),
		deadline: c.current.Add(d),
	}
	c.timers = append(c.timers, t)
	return t
}

// After returns a channel that receives the current time after duration d.
func (c *MockClock) After(d time.Duration) <-chan time.Time {
	return c.NewTimer(d).C()
}

// Sleep is a no-op for mock clock (instant return).
// Use Add() to advance time in tests.
func (c *MockClock) Sleep(d time.Duration) {
	// In tests, Sleep is typically a no-op or you manually advance time
}

// mockTicker implements Ticker for testing.
type mockTicker struct {
	ch       chan time.Time
	interval time.Duration
	next     time.Time
	stopped  bool
}

func (t *mockTicker) C() <-chan time.Time {
	return t.ch
}

func (t *mockTicker) Stop() {
	t.stopped = true
}

func (t *mockTicker) Reset(d time.Duration) {
	t.interval = d
	t.stopped = false
}

// mockTimer implements Timer for testing.
type mockTimer struct {
	ch       chan time.Time
	deadline time.Time
	fired    bool
	stopped  bool
}

func (t *mockTimer) C() <-chan time.Time {
	return t.ch
}

func (t *mockTimer) Stop() bool {
	wasPending := !t.fired && !t.stopped
	t.stopped = true
	return wasPending
}

func (t *mockTimer) Reset(d time.Duration) bool {
	wasPending := !t.fired && !t.stopped
	t.fired = false
	t.stopped = false
	return wasPending
}
