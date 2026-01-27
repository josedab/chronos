package dispatcher

import (
	"testing"
	"time"
)

func TestCircuitBreaker_InitialState(t *testing.T) {
	cb := NewCircuitBreaker(nil)
	if cb.State() != CircuitClosed {
		t.Errorf("expected initial state to be closed, got %v", cb.State())
	}
}

func TestCircuitBreaker_AllowWhenClosed(t *testing.T) {
	cb := NewCircuitBreaker(nil)
	if !cb.Allow() {
		t.Error("expected Allow() to return true when circuit is closed")
	}
}

func TestCircuitBreaker_OpensAfterFailures(t *testing.T) {
	config := &CircuitBreakerConfig{
		FailureThreshold:    3,
		SuccessThreshold:    2,
		Timeout:             100 * time.Millisecond,
		MaxHalfOpenRequests: 1,
	}
	cb := NewCircuitBreaker(config)

	// Record failures up to threshold
	for i := 0; i < 3; i++ {
		if !cb.Allow() {
			t.Errorf("expected Allow() on attempt %d", i)
		}
		cb.RecordFailure()
	}

	// Circuit should now be open
	if cb.State() != CircuitOpen {
		t.Errorf("expected circuit to be open after %d failures, got %v", 3, cb.State())
	}

	// Requests should be blocked
	if cb.Allow() {
		t.Error("expected Allow() to return false when circuit is open")
	}
}

func TestCircuitBreaker_TransitionsToHalfOpen(t *testing.T) {
	config := &CircuitBreakerConfig{
		FailureThreshold:    2,
		SuccessThreshold:    1,
		Timeout:             50 * time.Millisecond,
		MaxHalfOpenRequests: 1,
	}
	cb := NewCircuitBreaker(config)

	// Open the circuit
	cb.Allow()
	cb.RecordFailure()
	cb.Allow()
	cb.RecordFailure()

	if cb.State() != CircuitOpen {
		t.Fatalf("expected circuit to be open, got %v", cb.State())
	}

	// Wait for timeout
	time.Sleep(60 * time.Millisecond)

	// Should be half-open now
	if cb.State() != CircuitHalfOpen {
		t.Errorf("expected circuit to be half-open after timeout, got %v", cb.State())
	}

	// Should allow one request
	if !cb.Allow() {
		t.Error("expected Allow() to return true in half-open state")
	}
}

func TestCircuitBreaker_ClosesAfterSuccessInHalfOpen(t *testing.T) {
	config := &CircuitBreakerConfig{
		FailureThreshold:    2,
		SuccessThreshold:    2,
		Timeout:             50 * time.Millisecond,
		MaxHalfOpenRequests: 2,
	}
	cb := NewCircuitBreaker(config)

	// Open the circuit
	cb.Allow()
	cb.RecordFailure()
	cb.Allow()
	cb.RecordFailure()

	// Wait for half-open
	time.Sleep(60 * time.Millisecond)

	// Record successes in half-open
	cb.Allow()
	cb.RecordSuccess()
	cb.Allow()
	cb.RecordSuccess()

	// Circuit should be closed
	if cb.State() != CircuitClosed {
		t.Errorf("expected circuit to be closed after successes, got %v", cb.State())
	}
}

func TestCircuitBreaker_ReOpensOnFailureInHalfOpen(t *testing.T) {
	config := &CircuitBreakerConfig{
		FailureThreshold:    2,
		SuccessThreshold:    2,
		Timeout:             50 * time.Millisecond,
		MaxHalfOpenRequests: 1,
	}
	cb := NewCircuitBreaker(config)

	// Open the circuit
	cb.Allow()
	cb.RecordFailure()
	cb.Allow()
	cb.RecordFailure()

	// Wait for half-open
	time.Sleep(60 * time.Millisecond)

	// Fail in half-open
	cb.Allow()
	cb.RecordFailure()

	// Should be open again
	if cb.State() != CircuitOpen {
		t.Errorf("expected circuit to be open after half-open failure, got %v", cb.State())
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	config := &CircuitBreakerConfig{
		FailureThreshold:    2,
		SuccessThreshold:    1,
		Timeout:             time.Hour, // Long timeout
		MaxHalfOpenRequests: 1,
	}
	cb := NewCircuitBreaker(config)

	// Open the circuit
	cb.Allow()
	cb.RecordFailure()
	cb.Allow()
	cb.RecordFailure()

	if cb.State() != CircuitOpen {
		t.Fatalf("expected circuit to be open, got %v", cb.State())
	}

	// Reset
	cb.Reset()

	if cb.State() != CircuitClosed {
		t.Errorf("expected circuit to be closed after reset, got %v", cb.State())
	}
}

func TestCircuitBreaker_SuccessResetFailureCount(t *testing.T) {
	config := &CircuitBreakerConfig{
		FailureThreshold:    3,
		SuccessThreshold:    1,
		Timeout:             time.Hour,
		MaxHalfOpenRequests: 1,
	}
	cb := NewCircuitBreaker(config)

	// Two failures
	cb.Allow()
	cb.RecordFailure()
	cb.Allow()
	cb.RecordFailure()

	// One success should reset failure count
	cb.Allow()
	cb.RecordSuccess()

	// Two more failures shouldn't open the circuit (need 3 consecutive)
	cb.Allow()
	cb.RecordFailure()
	cb.Allow()
	cb.RecordFailure()

	if cb.State() != CircuitClosed {
		t.Errorf("expected circuit to still be closed, got %v", cb.State())
	}
}

func TestCircuitBreakerRegistry_Get(t *testing.T) {
	registry := NewCircuitBreakerRegistry(nil)

	cb1 := registry.Get("endpoint-a")
	cb2 := registry.Get("endpoint-a")
	cb3 := registry.Get("endpoint-b")

	if cb1 != cb2 {
		t.Error("expected same circuit breaker for same key")
	}
	if cb1 == cb3 {
		t.Error("expected different circuit breakers for different keys")
	}
}

func TestCircuitBreakerRegistry_Stats(t *testing.T) {
	registry := NewCircuitBreakerRegistry(&CircuitBreakerConfig{
		FailureThreshold:    2,
		SuccessThreshold:    1,
		Timeout:             time.Hour,
		MaxHalfOpenRequests: 1,
	})

	cb := registry.Get("endpoint-a")
	cb.Allow()
	cb.RecordFailure()
	cb.Allow()
	cb.RecordFailure()

	stats := registry.Stats()
	if len(stats) != 1 {
		t.Errorf("expected 1 entry in stats, got %d", len(stats))
	}
	if stats["endpoint-a"].State != CircuitOpen {
		t.Errorf("expected endpoint-a to be open, got %v", stats["endpoint-a"].State)
	}
}

func TestCircuitState_String(t *testing.T) {
	tests := []struct {
		state    CircuitState
		expected string
	}{
		{CircuitClosed, "closed"},
		{CircuitOpen, "open"},
		{CircuitHalfOpen, "half-open"},
		{CircuitState(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.expected {
			t.Errorf("CircuitState(%d).String() = %s, want %s", tt.state, got, tt.expected)
		}
	}
}

func TestCircuitBreaker_Metrics(t *testing.T) {
	config := &CircuitBreakerConfig{
		FailureThreshold:    2,
		SuccessThreshold:    2,
		Timeout:             50 * time.Millisecond,
		MaxHalfOpenRequests: 2,
	}
	cb := NewCircuitBreaker(config)

	// Initial stats should be zero
	stats := cb.Stats()
	if stats.TotalTransitions != 0 {
		t.Errorf("expected 0 transitions initially, got %d", stats.TotalTransitions)
	}
	if stats.TotalOpens != 0 {
		t.Errorf("expected 0 opens initially, got %d", stats.TotalOpens)
	}

	// Open the circuit
	cb.Allow()
	cb.RecordFailure()
	cb.Allow()
	cb.RecordFailure()

	stats = cb.Stats()
	if stats.TotalOpens != 1 {
		t.Errorf("expected 1 open, got %d", stats.TotalOpens)
	}
	if stats.TotalTransitions != 1 {
		t.Errorf("expected 1 transition, got %d", stats.TotalTransitions)
	}

	// Wait for half-open and close the circuit
	time.Sleep(60 * time.Millisecond)
	cb.Allow()
	cb.RecordSuccess()
	cb.Allow()
	cb.RecordSuccess()

	stats = cb.Stats()
	if stats.TotalCloses != 1 {
		t.Errorf("expected 1 close, got %d", stats.TotalCloses)
	}
	// Transitions: closed->open, then half-open->closed (note: half-open is virtual)
	if stats.TotalTransitions != 2 {
		t.Errorf("expected 2 transitions, got %d", stats.TotalTransitions)
	}
}

func TestCircuitBreaker_OnStateChangeCallback(t *testing.T) {
	transitions := make(chan struct {
		name string
		from CircuitState
		to   CircuitState
	}, 10)

	config := &CircuitBreakerConfig{
		FailureThreshold:    2,
		SuccessThreshold:    1,
		Timeout:             50 * time.Millisecond,
		MaxHalfOpenRequests: 1,
		OnStateChange: func(name string, from, to CircuitState) {
			transitions <- struct {
				name string
				from CircuitState
				to   CircuitState
			}{name, from, to}
		},
	}
	cb := NewCircuitBreakerWithName("test-endpoint", config)

	// Open the circuit
	cb.Allow()
	cb.RecordFailure()
	cb.Allow()
	cb.RecordFailure()

	// Wait for callback
	select {
	case t1 := <-transitions:
		if t1.name != "test-endpoint" {
			t.Errorf("expected name 'test-endpoint', got '%s'", t1.name)
		}
		if t1.from != CircuitClosed {
			t.Errorf("expected from=closed, got %v", t1.from)
		}
		if t1.to != CircuitOpen {
			t.Errorf("expected to=open, got %v", t1.to)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for state change callback")
	}

	// Wait for half-open and close
	time.Sleep(60 * time.Millisecond)
	cb.Allow()
	cb.RecordSuccess()

	// Wait for callback
	select {
	case t2 := <-transitions:
		if t2.from != CircuitHalfOpen {
			t.Errorf("expected from=half-open, got %v", t2.from)
		}
		if t2.to != CircuitClosed {
			t.Errorf("expected to=closed, got %v", t2.to)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for state change callback")
	}
}

func TestCircuitBreakerWithName(t *testing.T) {
	cb := NewCircuitBreakerWithName("my-endpoint", nil)
	if cb.name != "my-endpoint" {
		t.Errorf("expected name 'my-endpoint', got '%s'", cb.name)
	}
	if cb.State() != CircuitClosed {
		t.Errorf("expected initial state closed, got %v", cb.State())
	}
}

func TestCircuitBreakerRegistry_NamePropagation(t *testing.T) {
	transitions := make(chan string, 10)

	registry := NewCircuitBreakerRegistry(&CircuitBreakerConfig{
		FailureThreshold:    2,
		SuccessThreshold:    1,
		Timeout:             time.Hour,
		MaxHalfOpenRequests: 1,
		OnStateChange: func(name string, from, to CircuitState) {
			transitions <- name
		},
	})

	cb := registry.Get("endpoint-xyz")
	cb.Allow()
	cb.RecordFailure()
	cb.Allow()
	cb.RecordFailure()

	select {
	case name := <-transitions:
		if name != "endpoint-xyz" {
			t.Errorf("expected name 'endpoint-xyz', got '%s'", name)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for state change callback")
	}
}
