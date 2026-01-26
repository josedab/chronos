package clock

import (
	"testing"
	"time"
)

func TestRealClock_Now(t *testing.T) {
	c := New()
	before := time.Now()
	got := c.Now()
	after := time.Now()

	if got.Before(before) || got.After(after) {
		t.Errorf("Now() = %v, want between %v and %v", got, before, after)
	}
}

func TestRealClock_Since(t *testing.T) {
	c := New()
	start := time.Now()
	time.Sleep(10 * time.Millisecond)
	elapsed := c.Since(start)

	if elapsed < 10*time.Millisecond {
		t.Errorf("Since() = %v, want >= 10ms", elapsed)
	}
}

func TestRealClock_Until(t *testing.T) {
	c := New()
	future := time.Now().Add(100 * time.Millisecond)
	until := c.Until(future)

	if until < 90*time.Millisecond || until > 100*time.Millisecond {
		t.Errorf("Until() = %v, want ~100ms", until)
	}
}

func TestMockClock_Now(t *testing.T) {
	fixed := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	c := NewMock(fixed)

	if got := c.Now(); !got.Equal(fixed) {
		t.Errorf("Now() = %v, want %v", got, fixed)
	}
}

func TestMockClock_Set(t *testing.T) {
	c := NewMock(time.Time{})
	newTime := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	c.Set(newTime)

	if got := c.Now(); !got.Equal(newTime) {
		t.Errorf("Now() after Set = %v, want %v", got, newTime)
	}
}

func TestMockClock_Add(t *testing.T) {
	start := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	c := NewMock(start)
	c.Add(1 * time.Hour)

	expected := start.Add(1 * time.Hour)
	if got := c.Now(); !got.Equal(expected) {
		t.Errorf("Now() after Add = %v, want %v", got, expected)
	}
}

func TestMockClock_Since(t *testing.T) {
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	past := now.Add(-1 * time.Hour)
	c := NewMock(now)

	if got := c.Since(past); got != 1*time.Hour {
		t.Errorf("Since() = %v, want 1h", got)
	}
}

func TestMockClock_Until(t *testing.T) {
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	future := now.Add(1 * time.Hour)
	c := NewMock(now)

	if got := c.Until(future); got != 1*time.Hour {
		t.Errorf("Until() = %v, want 1h", got)
	}
}

func TestMockClock_Timer(t *testing.T) {
	start := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	c := NewMock(start)

	timer := c.NewTimer(1 * time.Hour)

	// Timer shouldn't fire yet
	select {
	case <-timer.C():
		t.Error("timer fired before deadline")
	default:
	}

	// Advance past deadline
	c.Add(1*time.Hour + time.Second)

	// Timer should fire
	select {
	case <-timer.C():
		// Expected
	default:
		t.Error("timer didn't fire after deadline")
	}
}

func TestMockClock_Ticker(t *testing.T) {
	start := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	c := NewMock(start)

	ticker := c.NewTicker(1 * time.Minute)

	// Ticker shouldn't fire yet
	select {
	case <-ticker.C():
		t.Error("ticker fired before interval")
	default:
	}

	// Advance past first interval
	c.Add(1*time.Minute + time.Second)

	// Ticker should fire
	select {
	case <-ticker.C():
		// Expected
	default:
		t.Error("ticker didn't fire after interval")
	}

	// Advance past second interval
	c.Add(1 * time.Minute)

	select {
	case <-ticker.C():
		// Expected - ticker fires again
	default:
		t.Error("ticker didn't fire second time")
	}

	// Stop ticker
	ticker.Stop()
	c.Add(1 * time.Minute)

	select {
	case <-ticker.C():
		t.Error("stopped ticker fired")
	default:
		// Expected
	}
}

func TestMockClock_After(t *testing.T) {
	start := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	c := NewMock(start)

	ch := c.After(30 * time.Second)

	select {
	case <-ch:
		t.Error("After fired before duration")
	default:
	}

	c.Add(30*time.Second + time.Millisecond)

	select {
	case <-ch:
		// Expected
	default:
		t.Error("After didn't fire after duration")
	}
}

func TestMockTimer_Stop(t *testing.T) {
	c := NewMock(time.Now())
	timer := c.NewTimer(1 * time.Hour)

	// Stop returns true if timer was pending
	if !timer.Stop() {
		t.Error("Stop() should return true for pending timer")
	}

	// Stop again returns false
	if timer.Stop() {
		t.Error("Stop() should return false for already stopped timer")
	}
}
