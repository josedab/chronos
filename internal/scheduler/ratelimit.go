// Package scheduler provides per-namespace rate-limited job queuing.
package scheduler

import (
	"context"
	"sync"
	"time"
)

// RateLimiter implements per-namespace token bucket rate limiting.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	config  RateLimitConfig
}

// RateLimitConfig configures rate limiting behavior.
type RateLimitConfig struct {
	// DefaultRate is the max executions per hour for namespaces without explicit config.
	DefaultRate int `json:"default_rate" yaml:"default_rate"`
	// DefaultBurst is the max concurrent executions per namespace.
	DefaultBurst int `json:"default_burst" yaml:"default_burst"`
	// NamespaceOverrides maps namespace to custom limits.
	NamespaceOverrides map[string]NamespaceLimit `json:"namespace_overrides,omitempty" yaml:"namespace_overrides,omitempty"`
}

// NamespaceLimit defines rate limits for a specific namespace.
type NamespaceLimit struct {
	Rate  int `json:"rate"`  // Max executions per hour
	Burst int `json:"burst"` // Max concurrent executions
}

// DefaultRateLimitConfig returns sensible defaults.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		DefaultRate:  600, // 10 per minute
		DefaultBurst: 10,
	}
}

type tokenBucket struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
	inFlight   int
	maxBurst   int
}

// NewRateLimiter creates a new per-namespace rate limiter.
func NewRateLimiter(config RateLimitConfig) *RateLimiter {
	if config.DefaultRate <= 0 {
		config.DefaultRate = 600
	}
	if config.DefaultBurst <= 0 {
		config.DefaultBurst = 10
	}
	return &RateLimiter{
		buckets: make(map[string]*tokenBucket),
		config:  config,
	}
}

// Allow checks if an execution is allowed for the given namespace.
// Returns true if allowed, false if rate limited.
func (rl *RateLimiter) Allow(namespace string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket := rl.getOrCreateBucket(namespace)
	bucket.refill()

	if bucket.tokens < 1 {
		return false
	}
	if bucket.inFlight >= bucket.maxBurst {
		return false
	}

	bucket.tokens--
	bucket.inFlight++
	return true
}

// Release signals that an execution has completed for the namespace.
func (rl *RateLimiter) Release(namespace string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if bucket, ok := rl.buckets[namespace]; ok {
		if bucket.inFlight > 0 {
			bucket.inFlight--
		}
	}
}

// Wait blocks until an execution is allowed or context is cancelled.
func (rl *RateLimiter) Wait(ctx context.Context, namespace string) error {
	for {
		if rl.Allow(namespace) {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			// Retry
		}
	}
}

// Stats returns rate limit stats for a namespace.
func (rl *RateLimiter) Stats(namespace string) RateLimitStats {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket := rl.getOrCreateBucket(namespace)
	bucket.refill()

	return RateLimitStats{
		Namespace:      namespace,
		TokensAvail:    int(bucket.tokens),
		MaxTokens:      int(bucket.maxTokens),
		InFlight:       bucket.inFlight,
		MaxBurst:       bucket.maxBurst,
		RefillRateHour: int(bucket.refillRate * 3600),
	}
}

// RateLimitStats contains rate limit statistics for a namespace.
type RateLimitStats struct {
	Namespace      string `json:"namespace"`
	TokensAvail    int    `json:"tokens_available"`
	MaxTokens      int    `json:"max_tokens"`
	InFlight       int    `json:"in_flight"`
	MaxBurst       int    `json:"max_burst"`
	RefillRateHour int    `json:"refill_rate_per_hour"`
}

func (rl *RateLimiter) getOrCreateBucket(namespace string) *tokenBucket {
	if namespace == "" {
		namespace = "default"
	}

	bucket, ok := rl.buckets[namespace]
	if ok {
		return bucket
	}

	rate := rl.config.DefaultRate
	burst := rl.config.DefaultBurst
	if override, ok := rl.config.NamespaceOverrides[namespace]; ok {
		rate = override.Rate
		burst = override.Burst
	}

	refillRate := float64(rate) / 3600.0 // convert per-hour to per-second
	bucket = &tokenBucket{
		tokens:     float64(burst), // Start with full burst capacity
		maxTokens:  float64(burst),
		refillRate: refillRate,
		lastRefill: time.Now(),
		maxBurst:   burst,
	}
	rl.buckets[namespace] = bucket
	return bucket
}

func (b *tokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * b.refillRate
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}
	b.lastRefill = now
}
