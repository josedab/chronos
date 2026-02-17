// Package cloud provides free tier enforcement middleware for managed Chronos.
package cloud

import (
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// FreeTierConfig defines free tier limits.
type FreeTierConfig struct {
	MaxJobs             int `json:"max_jobs"`
	MaxExecutionsPerDay int `json:"max_executions_per_day"`
	MaxConcurrent       int `json:"max_concurrent"`
	RetentionDays       int `json:"retention_days"`
}

// DefaultFreeTierConfig returns the default free tier limits.
func DefaultFreeTierConfig() FreeTierConfig {
	return FreeTierConfig{
		MaxJobs:             5,
		MaxExecutionsPerDay: 100,
		MaxConcurrent:       2,
		RetentionDays:       7,
	}
}

// FreeTierLimiter enforces free tier usage limits.
type FreeTierLimiter struct {
	config      FreeTierConfig
	jobCount    atomic.Int32
	dailyExecs  atomic.Int32
	concurrent  atomic.Int32
	lastReset   time.Time
	mu          sync.Mutex
}

// NewFreeTierLimiter creates a new free tier limiter.
func NewFreeTierLimiter(config FreeTierConfig) *FreeTierLimiter {
	return &FreeTierLimiter{
		config:    config,
		lastReset: time.Now(),
	}
}

// CanCreateJob checks if job creation is allowed.
func (l *FreeTierLimiter) CanCreateJob() bool {
	return int(l.jobCount.Load()) < l.config.MaxJobs
}

// CanExecute checks if an execution is allowed.
func (l *FreeTierLimiter) CanExecute() bool {
	l.resetDailyIfNeeded()
	return int(l.dailyExecs.Load()) < l.config.MaxExecutionsPerDay &&
		int(l.concurrent.Load()) < l.config.MaxConcurrent
}

// RecordJobCreated increments the job count.
func (l *FreeTierLimiter) RecordJobCreated() { l.jobCount.Add(1) }

// RecordJobDeleted decrements the job count.
func (l *FreeTierLimiter) RecordJobDeleted() { l.jobCount.Add(-1) }

// RecordExecutionStart tracks a new execution.
func (l *FreeTierLimiter) RecordExecutionStart() {
	l.dailyExecs.Add(1)
	l.concurrent.Add(1)
}

// RecordExecutionEnd releases a concurrent slot.
func (l *FreeTierLimiter) RecordExecutionEnd() {
	l.concurrent.Add(-1)
}

// Stats returns current usage stats.
func (l *FreeTierLimiter) Stats() map[string]interface{} {
	return map[string]interface{}{
		"jobs":            l.jobCount.Load(),
		"max_jobs":        l.config.MaxJobs,
		"daily_execs":     l.dailyExecs.Load(),
		"max_daily_execs": l.config.MaxExecutionsPerDay,
		"concurrent":      l.concurrent.Load(),
		"max_concurrent":  l.config.MaxConcurrent,
	}
}

func (l *FreeTierLimiter) resetDailyIfNeeded() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	if now.YearDay() != l.lastReset.YearDay() || now.Year() != l.lastReset.Year() {
		l.dailyExecs.Store(0)
		l.lastReset = now
	}
}

// Middleware returns HTTP middleware that enforces free tier limits on job creation.
func (l *FreeTierLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only enforce on job creation
			if r.Method == "POST" && r.URL.Path == "/api/v1/jobs" {
				if !l.CanCreateJob() {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusTooManyRequests)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"success": false,
						"error": map[string]string{
							"code":    "free_tier_limit",
							"message": "Free tier limit reached. Upgrade to create more jobs.",
						},
					})
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
