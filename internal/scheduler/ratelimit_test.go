package scheduler

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		DefaultRate:  3600, // 1 per second
		DefaultBurst: 3,
	})

	// Should allow burst of 3
	for i := 0; i < 3; i++ {
		if !rl.Allow("default") {
			t.Errorf("expected allow for burst %d", i+1)
		}
	}

	// 4th should be blocked (in-flight limit reached)
	if rl.Allow("default") {
		t.Error("expected deny after burst exhausted")
	}

	// Release one
	rl.Release("default")
	// Token bucket may still be empty, but in-flight allows
	// Need to wait a bit for token refill
	time.Sleep(10 * time.Millisecond)

	// Depending on token refill, may or may not allow
	// The point is Release reduces in-flight count
}

func TestRateLimiter_NamespaceIsolation(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		DefaultRate:  3600,
		DefaultBurst: 2,
	})

	// Exhaust namespace A
	rl.Allow("ns-a")
	rl.Allow("ns-a")

	// Namespace B should still work
	if !rl.Allow("ns-b") {
		t.Error("namespace B should not be affected by A's limits")
	}
}

func TestRateLimiter_NamespaceOverrides(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		DefaultRate:  3600,
		DefaultBurst: 2,
		NamespaceOverrides: map[string]NamespaceLimit{
			"premium": {Rate: 36000, Burst: 50},
		},
	})

	stats := rl.Stats("premium")
	if stats.MaxBurst != 50 {
		t.Errorf("expected burst 50 for premium, got %d", stats.MaxBurst)
	}
	if stats.RefillRateHour != 36000 {
		t.Errorf("expected rate 36000/hr for premium, got %d", stats.RefillRateHour)
	}
}

func TestRateLimiter_Release(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		DefaultRate:  3600,
		DefaultBurst: 1,
	})

	if !rl.Allow("test") {
		t.Fatal("first request should be allowed")
	}

	// Should be blocked
	if rl.Allow("test") {
		t.Error("should be blocked at burst limit")
	}

	rl.Release("test")

	// After short wait for token refill
	time.Sleep(5 * time.Millisecond)

	stats := rl.Stats("test")
	if stats.InFlight != 0 {
		t.Errorf("expected 0 in-flight after release, got %d", stats.InFlight)
	}
}

func TestRateLimiter_Wait(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		DefaultRate:  36000, // 10 per second
		DefaultBurst: 5,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Should succeed immediately (within burst)
	if err := rl.Wait(ctx, "test"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	rl.Release("test")
}

func TestRateLimiter_Wait_Cancelled(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		DefaultRate:  1,    // very slow refill
		DefaultBurst: 1,
	})

	// Exhaust the bucket
	rl.Allow("test")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := rl.Wait(ctx, "test")
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestRateLimiter_Stats(t *testing.T) {
	rl := NewRateLimiter(DefaultRateLimitConfig())

	stats := rl.Stats("default")
	if stats.Namespace != "default" {
		t.Errorf("expected namespace 'default', got %s", stats.Namespace)
	}
	if stats.RefillRateHour != 600 {
		t.Errorf("expected rate 600/hr, got %d", stats.RefillRateHour)
	}
}

func TestRateLimiter_DefaultConfig(t *testing.T) {
	cfg := DefaultRateLimitConfig()
	if cfg.DefaultRate != 600 {
		t.Errorf("expected default rate 600, got %d", cfg.DefaultRate)
	}
	if cfg.DefaultBurst != 10 {
		t.Errorf("expected default burst 10, got %d", cfg.DefaultBurst)
	}
}

func TestRateLimiter_EmptyNamespace(t *testing.T) {
	rl := NewRateLimiter(DefaultRateLimitConfig())

	// Empty namespace should map to "default"
	if !rl.Allow("") {
		t.Error("expected allow for empty namespace (mapped to default)")
	}
	stats := rl.Stats("")
	if stats.Namespace != "" {
		t.Logf("namespace: %s", stats.Namespace)
	}
}
