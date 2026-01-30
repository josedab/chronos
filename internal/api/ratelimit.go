// Package api provides the REST API router.
package api

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// RateLimitConfig holds rate limiting configuration.
type RateLimitConfig struct {
	// Enabled determines if rate limiting is active.
	Enabled bool
	// RequestsPerSecond is the rate limit per client.
	RequestsPerSecond float64
	// BurstSize is the maximum burst allowed.
	BurstSize int
	// CleanupInterval is how often to clean up old entries.
	CleanupInterval time.Duration
}

// DefaultRateLimitConfig returns sensible defaults for rate limiting.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Enabled:           true,
		RequestsPerSecond: 100,
		BurstSize:         200,
		CleanupInterval:   time.Minute,
	}
}

// EndpointRateLimitConfig holds rate limiting configuration for specific endpoints.
type EndpointRateLimitConfig struct {
	// Pattern is the URL pattern to match (e.g., "/api/v1/jobs/*/trigger")
	Pattern string
	// RequestsPerSecond is the rate limit per client for this endpoint.
	RequestsPerSecond float64
	// BurstSize is the maximum burst allowed for this endpoint.
	BurstSize int
}

// DefaultTriggerEndpointLimit returns the default rate limit for TriggerJob endpoint.
// This is stricter than the global limit to prevent abuse.
func DefaultTriggerEndpointLimit() EndpointRateLimitConfig {
	return EndpointRateLimitConfig{
		Pattern:           "/api/v1/jobs/*/trigger",
		RequestsPerSecond: 10,
		BurstSize:         20,
	}
}

// tokenBucket implements a token bucket rate limiter.
type tokenBucket struct {
	tokens     float64
	maxTokens  float64
	refillRate float64
	lastRefill time.Time
	mu         sync.Mutex
}

func newTokenBucket(rate float64, burst int) *tokenBucket {
	return &tokenBucket{
		tokens:     float64(burst),
		maxTokens:  float64(burst),
		refillRate: rate,
		lastRefill: time.Now(),
	}
}

func (b *tokenBucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.lastRefill = now

	// Refill tokens
	b.tokens += elapsed * b.refillRate
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}

	// Check if we can consume a token
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// RateLimiter manages rate limiting for multiple clients.
type RateLimiter struct {
	config          RateLimitConfig
	clients         map[string]*tokenBucket
	endpointLimits  []EndpointRateLimitConfig
	endpointClients map[string]map[string]*tokenBucket // pattern -> clientID -> bucket
	mu              sync.RWMutex
	stopCh          chan struct{}
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(config RateLimitConfig) *RateLimiter {
	rl := &RateLimiter{
		config:          config,
		clients:         make(map[string]*tokenBucket),
		endpointClients: make(map[string]map[string]*tokenBucket),
		stopCh:          make(chan struct{}),
	}

	// Start cleanup goroutine
	if config.CleanupInterval > 0 {
		go rl.cleanup()
	}

	return rl
}

// WithEndpointLimits adds endpoint-specific rate limits.
func (rl *RateLimiter) WithEndpointLimits(limits []EndpointRateLimitConfig) *RateLimiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.endpointLimits = limits
	for _, limit := range limits {
		if _, exists := rl.endpointClients[limit.Pattern]; !exists {
			rl.endpointClients[limit.Pattern] = make(map[string]*tokenBucket)
		}
	}
	return rl
}

// Allow checks if a request from the given client is allowed.
func (rl *RateLimiter) Allow(clientID string) bool {
	if !rl.config.Enabled {
		return true
	}

	rl.mu.RLock()
	bucket, exists := rl.clients[clientID]
	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()
		// Double-check after acquiring write lock
		if bucket, exists = rl.clients[clientID]; !exists {
			bucket = newTokenBucket(rl.config.RequestsPerSecond, rl.config.BurstSize)
			rl.clients[clientID] = bucket
		}
		rl.mu.Unlock()
	}

	return bucket.allow()
}

// AllowEndpoint checks if a request from the given client is allowed for a specific endpoint.
// Returns the rate limit that was applied (or 0 if no specific limit).
func (rl *RateLimiter) AllowEndpoint(clientID, path string) (allowed bool, rateLimit float64) {
	if !rl.config.Enabled {
		return true, 0
	}

	// Check endpoint-specific limits first
	rl.mu.RLock()
	for _, limit := range rl.endpointLimits {
		if matchEndpointPattern(limit.Pattern, path) {
			clients := rl.endpointClients[limit.Pattern]
			bucket, exists := clients[clientID]
			rl.mu.RUnlock()

			if !exists {
				rl.mu.Lock()
				clients = rl.endpointClients[limit.Pattern]
				if bucket, exists = clients[clientID]; !exists {
					bucket = newTokenBucket(limit.RequestsPerSecond, limit.BurstSize)
					clients[clientID] = bucket
				}
				rl.mu.Unlock()
			}

			return bucket.allow(), limit.RequestsPerSecond
		}
	}
	rl.mu.RUnlock()

	// Fall back to global limit
	return rl.Allow(clientID), rl.config.RequestsPerSecond
}

// matchEndpointPattern checks if a path matches a pattern with wildcard support.
// Patterns use * for single segment wildcards (e.g., "/api/v1/jobs/*/trigger")
func matchEndpointPattern(pattern, path string) bool {
	patternParts := splitPath(pattern)
	pathParts := splitPath(path)

	if len(patternParts) != len(pathParts) {
		return false
	}

	for i, pp := range patternParts {
		if pp == "*" {
			continue
		}
		if pp != pathParts[i] {
			return false
		}
	}
	return true
}

// splitPath splits a URL path into segments.
func splitPath(path string) []string {
	var parts []string
	start := 0
	for i := 0; i <= len(path); i++ {
		if i == len(path) || path[i] == '/' {
			if i > start {
				parts = append(parts, path[start:i])
			}
			start = i + 1
		}
	}
	return parts
}

// cleanup periodically removes old client entries.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rl.stopCh:
			return
		case <-ticker.C:
			rl.mu.Lock()
			threshold := time.Now().Add(-rl.config.CleanupInterval * 2)
			for id, bucket := range rl.clients {
				bucket.mu.Lock()
				if bucket.lastRefill.Before(threshold) && bucket.tokens >= bucket.maxTokens {
					delete(rl.clients, id)
				}
				bucket.mu.Unlock()
			}
			rl.mu.Unlock()
		}
	}
}

// Stop stops the rate limiter cleanup goroutine.
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

// NewRateLimitMiddleware creates a rate limiting middleware.
func NewRateLimitMiddleware(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter == nil || !limiter.config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Use API key or IP as client identifier
			clientID := getClientID(r)

			// Check endpoint-specific and global rate limits
			allowed, rateLimit := limiter.AllowEndpoint(clientID, r.URL.Path)
			if !allowed {
				w.Header().Set("Retry-After", "1")
				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%.0f", rateLimit))
				http.Error(w, `{"error":"rate_limit_exceeded","message":"Too many requests"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// getClientID extracts the client identifier from the request.
func getClientID(r *http.Request) string {
	// Prefer API key if present
	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" && len(apiKey) >= 12 {
		return "key:" + apiKey[:12] // Use prefix for privacy
	}

	// Fall back to IP address
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return "ip:" + ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return "ip:" + ip
	}
	return "ip:" + r.RemoteAddr
}
