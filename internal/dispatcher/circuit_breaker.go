// Package dispatcher provides circuit breaker functionality for webhook execution.
package dispatcher

import (
	"errors"
	"sync"
	"time"
)

// Circuit breaker errors.
var (
	ErrCircuitOpen    = errors.New("circuit breaker is open")
	ErrTooManyFailures = errors.New("too many consecutive failures")
)

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
	// CircuitClosed allows requests through.
	CircuitClosed CircuitState = iota
	// CircuitOpen blocks all requests.
	CircuitOpen
	// CircuitHalfOpen allows limited requests to test recovery.
	CircuitHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig configures the circuit breaker behavior.
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of consecutive failures before opening.
	FailureThreshold int
	// SuccessThreshold is the number of successes in half-open to close.
	SuccessThreshold int
	// Timeout is how long the circuit stays open before transitioning to half-open.
	Timeout time.Duration
	// MaxHalfOpenRequests is the max concurrent requests allowed in half-open state.
	MaxHalfOpenRequests int
	// OnStateChange is called whenever the circuit state changes (optional).
	OnStateChange func(name string, from, to CircuitState)
}

// DefaultCircuitBreakerConfig returns sensible defaults.
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		FailureThreshold:    5,
		SuccessThreshold:    2,
		Timeout:             30 * time.Second,
		MaxHalfOpenRequests: 1,
	}
}

// CircuitBreaker implements the circuit breaker pattern for a single endpoint.
type CircuitBreaker struct {
	mu     sync.RWMutex
	config *CircuitBreakerConfig
	name   string // identifier for this circuit breaker

	state             CircuitState
	failures          int
	successes         int
	lastFailureTime   time.Time
	halfOpenRequests  int

	// Metrics
	totalTransitions  int64
	totalOpens        int64
	totalCloses       int64
}

// NewCircuitBreaker creates a new circuit breaker with the given config.
func NewCircuitBreaker(config *CircuitBreakerConfig) *CircuitBreaker {
	return NewCircuitBreakerWithName("", config)
}

// NewCircuitBreakerWithName creates a new circuit breaker with a name and config.
func NewCircuitBreakerWithName(name string, config *CircuitBreakerConfig) *CircuitBreaker {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}
	return &CircuitBreaker{
		config: config,
		name:   name,
		state:  CircuitClosed,
	}
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.currentState()
}

// currentState returns the state, checking for timeout transitions.
// Must be called with at least a read lock held.
func (cb *CircuitBreaker) currentState() CircuitState {
	if cb.state == CircuitOpen {
		if time.Since(cb.lastFailureTime) > cb.config.Timeout {
			return CircuitHalfOpen
		}
	}
	return cb.state
}

// Allow checks if a request should be allowed through.
// Returns true if allowed, false if the circuit is open.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	state := cb.currentState()

	switch state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		return false
	case CircuitHalfOpen:
		if cb.halfOpenRequests < cb.config.MaxHalfOpenRequests {
			cb.halfOpenRequests++
			return true
		}
		return false
	}
	return false
}

// RecordSuccess records a successful request.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	state := cb.currentState()

	switch state {
	case CircuitHalfOpen:
		cb.successes++
		cb.halfOpenRequests--
		if cb.successes >= cb.config.SuccessThreshold {
			cb.transitionToFrom(state, CircuitClosed)
		}
	case CircuitClosed:
		cb.failures = 0 // Reset consecutive failure count
	}
}

// RecordFailure records a failed request.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	state := cb.currentState()

	switch state {
	case CircuitClosed:
		cb.failures++
		if cb.failures >= cb.config.FailureThreshold {
			cb.transitionToFrom(state, CircuitOpen)
		}
	case CircuitHalfOpen:
		cb.halfOpenRequests--
		cb.transitionToFrom(state, CircuitOpen)
	}
}

// transitionTo changes the circuit state.
// Must be called with the lock held.
func (cb *CircuitBreaker) transitionTo(state CircuitState) {
	cb.transitionToFrom(cb.currentState(), state)
}

// transitionToFrom changes the circuit state from a known state.
// Must be called with the lock held.
func (cb *CircuitBreaker) transitionToFrom(fromState, state CircuitState) {
	if fromState == state {
		return // No transition
	}

	cb.state = state
	cb.failures = 0
	cb.successes = 0
	cb.halfOpenRequests = 0
	cb.totalTransitions++

	if state == CircuitOpen {
		cb.lastFailureTime = time.Now()
		cb.totalOpens++
	} else if state == CircuitClosed {
		cb.totalCloses++
	}

	// Call the state change callback if configured
	if cb.config.OnStateChange != nil {
		// Call outside the lock context to avoid potential deadlocks
		go cb.config.OnStateChange(cb.name, fromState, state)
	}
}

// Reset forces the circuit breaker back to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.transitionTo(CircuitClosed)
}

// Stats returns current circuit breaker statistics.
func (cb *CircuitBreaker) Stats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return CircuitBreakerStats{
		State:            cb.currentState(),
		Failures:         cb.failures,
		Successes:        cb.successes,
		LastFailureTime:  cb.lastFailureTime,
		TotalTransitions: cb.totalTransitions,
		TotalOpens:       cb.totalOpens,
		TotalCloses:      cb.totalCloses,
	}
}

// CircuitBreakerStats contains circuit breaker statistics.
type CircuitBreakerStats struct {
	State            CircuitState
	Failures         int
	Successes        int
	LastFailureTime  time.Time
	TotalTransitions int64
	TotalOpens       int64
	TotalCloses      int64
}

// CircuitBreakerRegistry manages circuit breakers for multiple endpoints.
type CircuitBreakerRegistry struct {
	mu       sync.RWMutex
	breakers map[string]*CircuitBreaker
	config   *CircuitBreakerConfig
}

// NewCircuitBreakerRegistry creates a registry with the given default config.
func NewCircuitBreakerRegistry(config *CircuitBreakerConfig) *CircuitBreakerRegistry {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}
	return &CircuitBreakerRegistry{
		breakers: make(map[string]*CircuitBreaker),
		config:   config,
	}
}

// Get returns the circuit breaker for a given key (e.g., webhook URL host).
// Creates a new one if it doesn't exist.
func (r *CircuitBreakerRegistry) Get(key string) *CircuitBreaker {
	r.mu.RLock()
	cb, exists := r.breakers[key]
	r.mu.RUnlock()

	if exists {
		return cb
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if cb, exists = r.breakers[key]; exists {
		return cb
	}

	cb = NewCircuitBreakerWithName(key, r.config)
	r.breakers[key] = cb
	return cb
}

// Stats returns stats for all circuit breakers.
func (r *CircuitBreakerRegistry) Stats() map[string]CircuitBreakerStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := make(map[string]CircuitBreakerStats, len(r.breakers))
	for key, cb := range r.breakers {
		stats[key] = cb.Stats()
	}
	return stats
}

// ResetAll resets all circuit breakers to closed state.
func (r *CircuitBreakerRegistry) ResetAll() {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, cb := range r.breakers {
		cb.Reset()
	}
}
