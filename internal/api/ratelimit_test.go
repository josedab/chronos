package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTokenBucket_Allow(t *testing.T) {
	bucket := newTokenBucket(10, 5) // 10 req/s, burst 5

	// Should allow burst
	for i := 0; i < 5; i++ {
		if !bucket.allow() {
			t.Errorf("expected bucket to allow request %d", i)
		}
	}

	// Should deny next request (bucket empty)
	if bucket.allow() {
		t.Error("expected bucket to deny request after burst")
	}
}

func TestTokenBucket_Refill(t *testing.T) {
	bucket := newTokenBucket(100, 1) // 100 req/s, burst 1

	// Consume the token
	if !bucket.allow() {
		t.Error("expected bucket to allow first request")
	}

	// Should be empty
	if bucket.allow() {
		t.Error("expected bucket to deny after consuming token")
	}

	// Wait for refill (10ms should add ~1 token at 100 req/s)
	time.Sleep(15 * time.Millisecond)

	// Should allow again
	if !bucket.allow() {
		t.Error("expected bucket to allow after refill")
	}
}

func TestRateLimiter_Allow(t *testing.T) {
	config := RateLimitConfig{
		Enabled:           true,
		RequestsPerSecond: 10,
		BurstSize:         2,
		CleanupInterval:   0, // Disable cleanup for test
	}
	limiter := NewRateLimiter(config)

	// Different clients should have separate limits
	if !limiter.Allow("client1") {
		t.Error("expected client1 to be allowed")
	}
	if !limiter.Allow("client2") {
		t.Error("expected client2 to be allowed")
	}

	// Same client should be rate limited
	limiter.Allow("client1") // Use second token
	if limiter.Allow("client1") {
		t.Error("expected client1 to be rate limited after burst")
	}

	// Client2 should still have tokens
	if !limiter.Allow("client2") {
		t.Error("expected client2 to still have tokens")
	}
}

func TestRateLimiter_Disabled(t *testing.T) {
	config := RateLimitConfig{
		Enabled:           false,
		RequestsPerSecond: 1,
		BurstSize:         1,
	}
	limiter := NewRateLimiter(config)

	// Should always allow when disabled
	for i := 0; i < 100; i++ {
		if !limiter.Allow("client") {
			t.Error("expected all requests to be allowed when disabled")
		}
	}
}

func TestRateLimiter_EndpointSpecificLimits(t *testing.T) {
	config := RateLimitConfig{
		Enabled:           true,
		RequestsPerSecond: 100, // Global limit
		BurstSize:         100,
		CleanupInterval:   0,
	}
	limiter := NewRateLimiter(config).WithEndpointLimits([]EndpointRateLimitConfig{
		{
			Pattern:           "/api/v1/jobs/*/trigger",
			RequestsPerSecond: 5,
			BurstSize:         2,
		},
	})

	clientID := "test-client"

	// Trigger endpoint should use stricter limit
	allowed, rate := limiter.AllowEndpoint(clientID, "/api/v1/jobs/abc123/trigger")
	if !allowed {
		t.Error("expected first trigger request to be allowed")
	}
	if rate != 5 {
		t.Errorf("expected rate 5, got %f", rate)
	}

	// Use up the burst
	limiter.AllowEndpoint(clientID, "/api/v1/jobs/abc123/trigger")
	allowed, _ = limiter.AllowEndpoint(clientID, "/api/v1/jobs/abc123/trigger")
	if allowed {
		t.Error("expected trigger request to be denied after burst")
	}

	// Other endpoints should use global limit
	allowed, rate = limiter.AllowEndpoint(clientID, "/api/v1/jobs")
	if !allowed {
		t.Error("expected other endpoint to be allowed")
	}
	if rate != 100 {
		t.Errorf("expected global rate 100, got %f", rate)
	}
}

func TestMatchEndpointPattern(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		match   bool
	}{
		{"/api/v1/jobs/*/trigger", "/api/v1/jobs/abc123/trigger", true},
		{"/api/v1/jobs/*/trigger", "/api/v1/jobs/xyz789/trigger", true},
		{"/api/v1/jobs/*/trigger", "/api/v1/jobs/trigger", false},
		{"/api/v1/jobs/*/trigger", "/api/v1/jobs/abc/def/trigger", false},
		{"/api/v1/*", "/api/v1/jobs", true},
		{"/api/v1/*", "/api/v1/users", true},
		{"/api/v1/*", "/api/v1/jobs/123", false},
		{"/exact/path", "/exact/path", true},
		{"/exact/path", "/exact/other", false},
	}

	for _, tt := range tests {
		got := matchEndpointPattern(tt.pattern, tt.path)
		if got != tt.match {
			t.Errorf("matchEndpointPattern(%q, %q) = %v, want %v",
				tt.pattern, tt.path, got, tt.match)
		}
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		path     string
		expected []string
	}{
		{"/api/v1/jobs", []string{"api", "v1", "jobs"}},
		{"/api/v1/jobs/", []string{"api", "v1", "jobs"}},
		{"api/v1/jobs", []string{"api", "v1", "jobs"}},
		{"/", nil},
		{"", nil},
	}

	for _, tt := range tests {
		got := splitPath(tt.path)
		if len(got) != len(tt.expected) {
			t.Errorf("splitPath(%q) = %v, want %v", tt.path, got, tt.expected)
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("splitPath(%q)[%d] = %q, want %q", tt.path, i, got[i], tt.expected[i])
			}
		}
	}
}

func TestGetClientID(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		addr     string
		expected string
	}{
		{
			name:     "API key present",
			headers:  map[string]string{"X-API-Key": "sk-test123456789"},
			expected: "key:sk-test12345",
		},
		{
			name:     "X-Real-IP present",
			headers:  map[string]string{"X-Real-IP": "192.168.1.1"},
			expected: "ip:192.168.1.1",
		},
		{
			name:     "X-Forwarded-For present",
			headers:  map[string]string{"X-Forwarded-For": "10.0.0.1"},
			expected: "ip:10.0.0.1",
		},
		{
			name:     "Falls back to RemoteAddr",
			headers:  map[string]string{},
			addr:     "127.0.0.1:8080",
			expected: "ip:127.0.0.1:8080",
		},
		{
			name:     "API key too short",
			headers:  map[string]string{"X-API-Key": "short"},
			addr:     "127.0.0.1:8080",
			expected: "ip:127.0.0.1:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			if tt.addr != "" {
				req.RemoteAddr = tt.addr
			}

			got := getClientID(req)
			if got != tt.expected {
				t.Errorf("getClientID() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	config := RateLimitConfig{
		Enabled:           true,
		RequestsPerSecond: 10,
		BurstSize:         2,
		CleanupInterval:   0,
	}
	limiter := NewRateLimiter(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := NewRateLimitMiddleware(limiter)(handler)

	// First two requests should succeed (burst)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()
		middleware.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i, rr.Code)
		}
	}

	// Third request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rr.Code)
	}
	if rr.Header().Get("Retry-After") != "1" {
		t.Error("expected Retry-After header")
	}
}

func TestRateLimitMiddleware_Disabled(t *testing.T) {
	limiter := NewRateLimiter(RateLimitConfig{Enabled: false})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := NewRateLimitMiddleware(limiter)(handler)

	// All requests should succeed when disabled
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()
		middleware.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i, rr.Code)
		}
	}
}

func TestRateLimitMiddleware_EndpointSpecific(t *testing.T) {
	config := RateLimitConfig{
		Enabled:           true,
		RequestsPerSecond: 100,
		BurstSize:         100,
		CleanupInterval:   0,
	}
	limiter := NewRateLimiter(config).WithEndpointLimits([]EndpointRateLimitConfig{
		{
			Pattern:           "/api/v1/jobs/*/trigger",
			RequestsPerSecond: 5,
			BurstSize:         1, // Only 1 allowed before rate limit
		},
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := NewRateLimitMiddleware(limiter)(handler)

	// First trigger request should succeed
	req := httptest.NewRequest("POST", "/api/v1/jobs/abc/trigger", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("first trigger: expected 200, got %d", rr.Code)
	}

	// Second trigger request should be rate limited
	req = httptest.NewRequest("POST", "/api/v1/jobs/abc/trigger", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr = httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("second trigger: expected 429, got %d", rr.Code)
	}

	// Other endpoint should still work (uses global limit)
	for i := 0; i < 10; i++ {
		req = httptest.NewRequest("GET", "/api/v1/jobs", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr = httptest.NewRecorder()
		middleware.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("list jobs request %d: expected 200, got %d", i, rr.Code)
		}
	}
}

func TestDefaultTriggerEndpointLimit(t *testing.T) {
	limit := DefaultTriggerEndpointLimit()

	if limit.Pattern != "/api/v1/jobs/*/trigger" {
		t.Errorf("unexpected pattern: %s", limit.Pattern)
	}
	if limit.RequestsPerSecond != 10 {
		t.Errorf("expected 10 req/s, got %f", limit.RequestsPerSecond)
	}
	if limit.BurstSize != 20 {
		t.Errorf("expected burst 20, got %d", limit.BurstSize)
	}
}
