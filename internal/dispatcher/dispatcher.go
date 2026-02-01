// Package dispatcher handles job execution via HTTP webhooks.
package dispatcher

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chronos/chronos/internal/models"
	"github.com/google/uuid"
)

// Dispatcher executes jobs via HTTP webhooks.
type Dispatcher struct {
	httpClient *http.Client
	nodeID     string

	// Track running executions
	running   map[string]*runningExecution
	runningMu sync.RWMutex

	// Configuration
	maxConcurrent   int
	maxResponseSize int64
	semaphore       chan struct{}

	// Circuit breakers for endpoint resilience
	circuitBreakers *CircuitBreakerRegistry

	// Metrics
	metrics *Metrics
}

// runningExecution tracks an in-progress execution.
type runningExecution struct {
	Execution *models.Execution
	Cancel    context.CancelFunc
	StartedAt time.Time
}

// Metrics tracks dispatcher metrics using atomic counters for thread safety.
type Metrics struct {
	ExecutionsTotal   atomic.Int64
	ExecutionsSuccess atomic.Int64
	ExecutionsFailed  atomic.Int64
}

// Config holds dispatcher configuration.
type Config struct {
	NodeID          string
	Timeout         time.Duration
	MaxIdleConns    int
	IdleConnTimeout time.Duration
	MaxConcurrent   int
	// MaxResponseSize is the maximum response body size to read (default: 1MB)
	MaxResponseSize int64
	// CircuitBreaker configures circuit breaker behavior (optional).
	CircuitBreaker *CircuitBreakerConfig
}

// DefaultConfig returns the default dispatcher configuration.
func DefaultConfig() *Config {
	return &Config{
		NodeID:          "chronos-1",
		Timeout:         30 * time.Second,
		MaxIdleConns:    100,
		IdleConnTimeout: 90 * time.Second,
		MaxConcurrent:   100,
		MaxResponseSize: 1024 * 1024, // 1MB default
	}
}

// New creates a new Dispatcher.
func New(cfg *Config) *Dispatcher {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	maxRespSize := cfg.MaxResponseSize
	if maxRespSize == 0 {
		maxRespSize = 1024 * 1024 // 1MB default
	}

	transport := &http.Transport{
		MaxIdleConns:        cfg.MaxIdleConns,
		IdleConnTimeout:     cfg.IdleConnTimeout,
		DisableCompression:  false,
		DisableKeepAlives:   false,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   cfg.Timeout,
	}

	return &Dispatcher{
		httpClient:      client,
		nodeID:          cfg.NodeID,
		running:         make(map[string]*runningExecution),
		maxConcurrent:   cfg.MaxConcurrent,
		maxResponseSize: maxRespSize,
		semaphore:       make(chan struct{}, cfg.MaxConcurrent),
		circuitBreakers: NewCircuitBreakerRegistry(cfg.CircuitBreaker),
		metrics:         &Metrics{},
	}
}

// Execute runs a job and returns the execution result.
func (d *Dispatcher) Execute(ctx context.Context, job *models.Job, scheduledTime time.Time) (*models.Execution, error) {
	// Acquire semaphore
	select {
	case d.semaphore <- struct{}{}:
		defer func() { <-d.semaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	execution := &models.Execution{
		ID:            uuid.New().String(),
		JobID:         job.ID,
		JobName:       job.Name,
		ScheduledTime: scheduledTime,
		StartedAt:     time.Now(),
		Status:        models.ExecutionRunning,
		Attempts:      0,
		NodeID:        d.nodeID,
	}

	// Track running execution
	execCtx, cancel := context.WithCancel(ctx)
	d.trackExecution(execution, cancel)
	defer d.untrackExecution(execution.ID)

	// Get retry policy
	policy := job.RetryPolicy
	if policy == nil {
		policy = models.DefaultRetryPolicy()
	}

	// Get timeout
	timeout := job.Timeout.Duration()
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	var lastErr error
	interval := policy.InitialInterval.Duration()

	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		execution.Attempts = attempt

		// Create context with timeout for this attempt
		attemptCtx, attemptCancel := context.WithTimeout(execCtx, timeout)

		var result *models.ExecutionResult
		var err error

		if job.Webhook != nil {
			result, err = d.executeWebhook(attemptCtx, job.Webhook, execution)
		} else {
			err = fmt.Errorf("no execution target configured")
		}
		attemptCancel()

		if err == nil && result != nil && result.Success {
			execution.Status = models.ExecutionSuccess
			completedAt := time.Now()
			execution.CompletedAt = &completedAt
			execution.StatusCode = result.StatusCode
			execution.Response = result.Response
			execution.Duration = time.Since(execution.StartedAt)

			d.metrics.ExecutionsTotal.Add(1)
			d.metrics.ExecutionsSuccess.Add(1)

			return execution, nil
		}

		lastErr = err
		if result != nil {
			execution.StatusCode = result.StatusCode
			execution.Response = result.Response
			execution.Error = result.Error
		}

		// Check if we should retry
		if !d.isRetriable(err, result) {
			break
		}

		// Don't retry if context is cancelled
		if execCtx.Err() != nil {
			break
		}

		// Wait before retry
		if attempt < policy.MaxAttempts {
			select {
			case <-time.After(interval):
			case <-execCtx.Done():
				break
			}
			interval = time.Duration(float64(interval) * policy.Multiplier)
			if interval > policy.MaxInterval.Duration() {
				interval = policy.MaxInterval.Duration()
			}
		}
	}

	execution.Status = models.ExecutionFailed
	completedAt := time.Now()
	execution.CompletedAt = &completedAt
	execution.Duration = time.Since(execution.StartedAt)
	if lastErr != nil && execution.Error == "" {
		execution.Error = lastErr.Error()
	}

	d.metrics.ExecutionsTotal.Add(1)
	d.metrics.ExecutionsFailed.Add(1)

	return execution, lastErr
}

// executeWebhook sends an HTTP request to the webhook.
func (d *Dispatcher) executeWebhook(ctx context.Context, config *models.WebhookConfig, execution *models.Execution) (*models.ExecutionResult, error) {
	// Extract host for circuit breaker key
	parsedURL, err := url.Parse(config.URL)
	if err != nil {
		return &models.ExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("invalid URL: %v", err),
		}, err
	}
	cbKey := parsedURL.Host

	// Check circuit breaker
	cb := d.circuitBreakers.Get(cbKey)
	if !cb.Allow() {
		return &models.ExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("circuit breaker open for %s", cbKey),
		}, ErrCircuitOpen
	}

	var body io.Reader
	if config.Body != "" {
		body = strings.NewReader(config.Body)
	}

	method := config.Method
	if method == "" {
		method = "GET"
	}

	req, err := http.NewRequestWithContext(ctx, method, config.URL, body)
	if err != nil {
		cb.RecordFailure()
		return &models.ExecutionResult{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	// Add custom headers
	for k, v := range config.Headers {
		req.Header.Set(k, v)
	}

	// Add Chronos metadata headers
	req.Header.Set("X-Chronos-Job-ID", execution.JobID)
	req.Header.Set("X-Chronos-Execution-ID", execution.ID)
	req.Header.Set("X-Chronos-Scheduled-Time", execution.ScheduledTime.Format(time.RFC3339))
	req.Header.Set("X-Chronos-Attempt", fmt.Sprintf("%d", execution.Attempts))

	// Apply authentication
	if config.Auth != nil {
		if err := d.applyAuth(req, config.Auth); err != nil {
			cb.RecordFailure()
			return &models.ExecutionResult{
				Success: false,
				Error:   fmt.Sprintf("auth error: %v", err),
			}, err
		}
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		cb.RecordFailure()
		return &models.ExecutionResult{
			Success: false,
			Error:   err.Error(),
		}, err
	}
	defer resp.Body.Close()

	// Read response body (limited to configured max size)
	respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, d.maxResponseSize))
	if readErr != nil {
		// Log read error but don't fail the request - the HTTP call itself succeeded
		respBody = []byte(fmt.Sprintf("[error reading response: %v]", readErr))
	}

	success := d.isSuccessCode(resp.StatusCode, config.SuccessCodes)

	result := &models.ExecutionResult{
		Success:    success,
		StatusCode: resp.StatusCode,
		Response:   string(respBody),
	}

	if !success {
		result.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, http.StatusText(resp.StatusCode))
		// Record failure for 5xx errors (server errors) - don't trip on 4xx (client errors)
		if resp.StatusCode >= 500 || resp.StatusCode == 429 {
			cb.RecordFailure()
		}
	} else {
		cb.RecordSuccess()
	}

	return result, nil
}

// applyAuth applies authentication to the request.
func (d *Dispatcher) applyAuth(req *http.Request, auth *models.AuthConfig) error {
	switch auth.Type {
	case models.AuthTypeBasic:
		credentials := base64.StdEncoding.EncodeToString(
			[]byte(auth.Username + ":" + auth.Password),
		)
		req.Header.Set("Authorization", "Basic "+credentials)

	case models.AuthTypeBearer:
		req.Header.Set("Authorization", "Bearer "+auth.Token)

	case models.AuthTypeAPIKey:
		header := auth.Header
		if header == "" {
			header = "X-API-Key"
		}
		req.Header.Set(header, auth.APIKey)
	}

	return nil
}

// isSuccessCode checks if a status code indicates success.
func (d *Dispatcher) isSuccessCode(code int, successCodes []int) bool {
	if len(successCodes) == 0 {
		// Default: 2xx codes are success
		return code >= 200 && code < 300
	}

	for _, c := range successCodes {
		if c == code {
			return true
		}
	}
	return false
}

// isRetriable checks if an error or result is retriable.
func (d *Dispatcher) isRetriable(err error, result *models.ExecutionResult) bool {
	// Context cancellation/deadline is not retriable
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false
		}
		// Other network errors are retriable
		return true
	}

	if result == nil {
		return false
	}

	// 4xx errors (client errors) are not retriable (except 429)
	if result.StatusCode >= 400 && result.StatusCode < 500 && result.StatusCode != 429 {
		return false
	}

	// 5xx errors and 429 are retriable
	return result.StatusCode >= 500 || result.StatusCode == 429
}

// trackExecution adds an execution to the running map.
func (d *Dispatcher) trackExecution(exec *models.Execution, cancel context.CancelFunc) {
	d.runningMu.Lock()
	defer d.runningMu.Unlock()

	d.running[exec.ID] = &runningExecution{
		Execution: exec,
		Cancel:    cancel,
		StartedAt: time.Now(),
	}
}

// untrackExecution removes an execution from the running map.
func (d *Dispatcher) untrackExecution(execID string) {
	d.runningMu.Lock()
	defer d.runningMu.Unlock()

	delete(d.running, execID)
}

// IsJobRunning checks if a job has an active execution.
func (d *Dispatcher) IsJobRunning(jobID string) bool {
	d.runningMu.RLock()
	defer d.runningMu.RUnlock()

	for _, r := range d.running {
		if r.Execution.JobID == jobID {
			return true
		}
	}
	return false
}

// CancelJobExecutions cancels all running executions for a job.
func (d *Dispatcher) CancelJobExecutions(jobID string) {
	d.runningMu.Lock()
	defer d.runningMu.Unlock()

	for _, r := range d.running {
		if r.Execution.JobID == jobID {
			r.Cancel()
		}
	}
}

// RunningCount returns the number of currently running executions.
func (d *Dispatcher) RunningCount() int {
	d.runningMu.RLock()
	defer d.runningMu.RUnlock()
	return len(d.running)
}

// MetricsSnapshot is a point-in-time snapshot of dispatcher metrics.
type MetricsSnapshot struct {
	ExecutionsTotal   int64
	ExecutionsSuccess int64
	ExecutionsFailed  int64
}

// GetMetrics returns a snapshot of the current metrics.
func (d *Dispatcher) GetMetrics() MetricsSnapshot {
	return MetricsSnapshot{
		ExecutionsTotal:   d.metrics.ExecutionsTotal.Load(),
		ExecutionsSuccess: d.metrics.ExecutionsSuccess.Load(),
		ExecutionsFailed:  d.metrics.ExecutionsFailed.Load(),
	}
}

// GetCircuitBreakerStats returns statistics for all circuit breakers.
func (d *Dispatcher) GetCircuitBreakerStats() map[string]CircuitBreakerStats {
	return d.circuitBreakers.Stats()
}

// ResetCircuitBreakers resets all circuit breakers to closed state.
func (d *Dispatcher) ResetCircuitBreakers() {
	d.circuitBreakers.ResetAll()
}
