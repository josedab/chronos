// Package replay provides execution replay and debugging capabilities.
package replay

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Common errors.
var (
	ErrExecutionNotFound = errors.New("execution not found")
	ErrReplayInProgress  = errors.New("replay already in progress")
	ErrInvalidExecution  = errors.New("invalid execution for replay")
)

// ExecutionSnapshot captures the complete state of an execution.
type ExecutionSnapshot struct {
	ID            string                 `json:"id"`
	JobID         string                 `json:"job_id"`
	JobName       string                 `json:"job_name"`
	ExecutionID   string                 `json:"execution_id"`
	
	// Original request
	Request       RequestSnapshot        `json:"request"`
	
	// Job configuration at time of execution
	JobConfig     JobConfigSnapshot      `json:"job_config"`
	
	// Execution context
	Context       ContextSnapshot        `json:"context"`
	
	// Response and result
	Response      ResponseSnapshot       `json:"response"`
	
	// Timing
	ScheduledAt   time.Time              `json:"scheduled_at"`
	StartedAt     time.Time              `json:"started_at"`
	EndedAt       time.Time              `json:"ended_at"`
	Duration      time.Duration          `json:"duration"`
	
	// Retry info
	Attempt       int                    `json:"attempt"`
	MaxAttempts   int                    `json:"max_attempts"`
	
	// Result
	Status        string                 `json:"status"`
	Error         string                 `json:"error,omitempty"`
	
	// Secrets (redacted)
	RedactedSecrets []string             `json:"redacted_secrets,omitempty"`
	
	// Metadata
	Metadata      map[string]string      `json:"metadata,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
}

// RequestSnapshot captures the HTTP request.
type RequestSnapshot struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body,omitempty"`
}

// JobConfigSnapshot captures the job config at execution time.
type JobConfigSnapshot struct {
	Schedule    string            `json:"schedule"`
	Timezone    string            `json:"timezone"`
	Timeout     string            `json:"timeout"`
	Concurrency string            `json:"concurrency"`
	RetryPolicy *RetryPolicy      `json:"retry_policy,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
	Version     int               `json:"version"`
}

// RetryPolicy is the retry configuration.
type RetryPolicy struct {
	MaxAttempts     int     `json:"max_attempts"`
	InitialInterval string  `json:"initial_interval"`
	MaxInterval     string  `json:"max_interval"`
	Multiplier      float64 `json:"multiplier"`
}

// ContextSnapshot captures execution context.
type ContextSnapshot struct {
	NodeID        string            `json:"node_id"`
	LeaderID      string            `json:"leader_id"`
	Region        string            `json:"region,omitempty"`
	Environment   map[string]string `json:"environment,omitempty"`
	TraceID       string            `json:"trace_id,omitempty"`
	SpanID        string            `json:"span_id,omitempty"`
}

// ResponseSnapshot captures the HTTP response.
type ResponseSnapshot struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
	BodySize   int               `json:"body_size"`
}

// ReplayRequest contains parameters for replaying an execution.
type ReplayRequest struct {
	ExecutionID   string            `json:"execution_id"`
	
	// Modifications for replay
	ModifyRequest *RequestModifier  `json:"modify_request,omitempty"`
	ModifyEnv     map[string]string `json:"modify_env,omitempty"`
	
	// Options
	DryRun        bool              `json:"dry_run"`
	Timeout       time.Duration     `json:"timeout,omitempty"`
	SkipRetries   bool              `json:"skip_retries"`
}

// RequestModifier allows modifying the request for replay.
type RequestModifier struct {
	URL     string            `json:"url,omitempty"`
	Method  string            `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

// ReplayResult contains the result of a replay.
type ReplayResult struct {
	ID              string            `json:"id"`
	OriginalID      string            `json:"original_id"`
	Status          string            `json:"status"`
	DryRun          bool              `json:"dry_run"`
	
	// Original vs Replay comparison
	Original        ExecutionSummary  `json:"original"`
	Replay          ExecutionSummary  `json:"replay"`
	
	// Diff
	Differences     []Difference      `json:"differences,omitempty"`
	
	// Timing
	ReplayedAt      time.Time         `json:"replayed_at"`
	Duration        time.Duration     `json:"duration"`
}

// ExecutionSummary is a summary of an execution.
type ExecutionSummary struct {
	Status     string        `json:"status"`
	StatusCode int           `json:"status_code"`
	Duration   time.Duration `json:"duration"`
	Error      string        `json:"error,omitempty"`
}

// Difference represents a difference between original and replay.
type Difference struct {
	Field    string `json:"field"`
	Original string `json:"original"`
	Replay   string `json:"replay"`
}

// TimelineEvent represents an event in the execution timeline.
type TimelineEvent struct {
	Timestamp   time.Time              `json:"timestamp"`
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	Duration    time.Duration          `json:"duration,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

// ExecutionTimeline provides a detailed timeline view.
type ExecutionTimeline struct {
	ExecutionID string          `json:"execution_id"`
	Events      []TimelineEvent `json:"events"`
	TotalDuration time.Duration `json:"total_duration"`
}

// Debugger provides execution replay and debugging.
type Debugger struct {
	mu        sync.RWMutex
	snapshots map[string]*ExecutionSnapshot
	replays   map[string]*ReplayResult
	executor  ExecutionExecutor
}

// ExecutionExecutor interface for executing jobs.
type ExecutionExecutor interface {
	Execute(ctx context.Context, req *RequestSnapshot) (*ResponseSnapshot, error)
}

// NewDebugger creates a new debugger.
func NewDebugger(executor ExecutionExecutor) *Debugger {
	return &Debugger{
		snapshots: make(map[string]*ExecutionSnapshot),
		replays:   make(map[string]*ReplayResult),
		executor:  executor,
	}
}

// CaptureExecution captures a snapshot of an execution.
func (d *Debugger) CaptureExecution(ctx context.Context, snapshot *ExecutionSnapshot) error {
	if snapshot.ID == "" {
		snapshot.ID = uuid.New().String()
	}
	snapshot.CreatedAt = time.Now()

	d.mu.Lock()
	defer d.mu.Unlock()
	d.snapshots[snapshot.ExecutionID] = snapshot
	return nil
}

// GetSnapshot retrieves an execution snapshot.
func (d *Debugger) GetSnapshot(ctx context.Context, executionID string) (*ExecutionSnapshot, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	snapshot, ok := d.snapshots[executionID]
	if !ok {
		return nil, ErrExecutionNotFound
	}
	return snapshot, nil
}

// Replay replays an execution with optional modifications.
func (d *Debugger) Replay(ctx context.Context, req *ReplayRequest) (*ReplayResult, error) {
	// Get original snapshot
	snapshot, err := d.GetSnapshot(ctx, req.ExecutionID)
	if err != nil {
		return nil, err
	}

	// Build modified request
	request := snapshot.Request
	if req.ModifyRequest != nil {
		if req.ModifyRequest.URL != "" {
			request.URL = req.ModifyRequest.URL
		}
		if req.ModifyRequest.Method != "" {
			request.Method = req.ModifyRequest.Method
		}
		if req.ModifyRequest.Headers != nil {
			for k, v := range req.ModifyRequest.Headers {
				request.Headers[k] = v
			}
		}
		if req.ModifyRequest.Body != "" {
			request.Body = req.ModifyRequest.Body
		}
	}

	result := &ReplayResult{
		ID:         uuid.New().String(),
		OriginalID: req.ExecutionID,
		DryRun:     req.DryRun,
		ReplayedAt: time.Now(),
		Original: ExecutionSummary{
			Status:     snapshot.Status,
			StatusCode: snapshot.Response.StatusCode,
			Duration:   snapshot.Duration,
			Error:      snapshot.Error,
		},
	}

	if req.DryRun {
		// Dry run - just return what would happen
		result.Status = "dry_run"
		result.Replay = ExecutionSummary{
			Status:     "simulated",
			StatusCode: 0,
			Duration:   0,
		}
		
		// Calculate differences
		if req.ModifyRequest != nil {
			if req.ModifyRequest.URL != "" && req.ModifyRequest.URL != snapshot.Request.URL {
				result.Differences = append(result.Differences, Difference{
					Field:    "url",
					Original: snapshot.Request.URL,
					Replay:   req.ModifyRequest.URL,
				})
			}
			if req.ModifyRequest.Body != "" && req.ModifyRequest.Body != snapshot.Request.Body {
				result.Differences = append(result.Differences, Difference{
					Field:    "body",
					Original: snapshot.Request.Body,
					Replay:   req.ModifyRequest.Body,
				})
			}
		}
	} else {
		// Actual replay
		timeout := req.Timeout
		if timeout == 0 {
			timeout = 30 * time.Second
		}
		
		execCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		start := time.Now()
		response, err := d.executor.Execute(execCtx, &request)
		duration := time.Since(start)

		if err != nil {
			result.Status = "failed"
			result.Replay = ExecutionSummary{
				Status:   "failed",
				Duration: duration,
				Error:    err.Error(),
			}
		} else {
			result.Status = "success"
			result.Replay = ExecutionSummary{
				Status:     "success",
				StatusCode: response.StatusCode,
				Duration:   duration,
			}
		}

		result.Duration = duration

		// Calculate differences
		if result.Original.StatusCode != result.Replay.StatusCode {
			result.Differences = append(result.Differences, Difference{
				Field:    "status_code",
				Original: string(rune(result.Original.StatusCode)),
				Replay:   string(rune(result.Replay.StatusCode)),
			})
		}
	}

	// Store replay result
	d.mu.Lock()
	d.replays[result.ID] = result
	d.mu.Unlock()

	return result, nil
}

// GetTimeline returns a detailed timeline of an execution.
func (d *Debugger) GetTimeline(ctx context.Context, executionID string) (*ExecutionTimeline, error) {
	snapshot, err := d.GetSnapshot(ctx, executionID)
	if err != nil {
		return nil, err
	}

	timeline := &ExecutionTimeline{
		ExecutionID:   executionID,
		TotalDuration: snapshot.Duration,
	}

	// Build timeline events
	timeline.Events = []TimelineEvent{
		{
			Timestamp:   snapshot.ScheduledAt,
			Type:        "scheduled",
			Description: "Job scheduled for execution",
		},
		{
			Timestamp:   snapshot.StartedAt,
			Type:        "started",
			Description: "Execution started",
			Details: map[string]interface{}{
				"attempt":      snapshot.Attempt,
				"max_attempts": snapshot.MaxAttempts,
			},
		},
		{
			Timestamp:   snapshot.StartedAt.Add(time.Millisecond),
			Type:        "request_sent",
			Description: "HTTP request sent to " + snapshot.Request.URL,
			Details: map[string]interface{}{
				"method": snapshot.Request.Method,
				"url":    snapshot.Request.URL,
			},
		},
	}

	if snapshot.Response.StatusCode > 0 {
		timeline.Events = append(timeline.Events, TimelineEvent{
			Timestamp:   snapshot.EndedAt.Add(-time.Millisecond),
			Type:        "response_received",
			Description: "HTTP response received",
			Duration:    snapshot.Duration,
			Details: map[string]interface{}{
				"status_code": snapshot.Response.StatusCode,
				"body_size":   snapshot.Response.BodySize,
			},
		})
	}

	timeline.Events = append(timeline.Events, TimelineEvent{
		Timestamp:   snapshot.EndedAt,
		Type:        "completed",
		Description: "Execution " + snapshot.Status,
		Duration:    snapshot.Duration,
		Details: map[string]interface{}{
			"status": snapshot.Status,
			"error":  snapshot.Error,
		},
	})

	return timeline, nil
}

// Compare compares two executions.
func (d *Debugger) Compare(ctx context.Context, executionID1, executionID2 string) ([]Difference, error) {
	snap1, err := d.GetSnapshot(ctx, executionID1)
	if err != nil {
		return nil, err
	}

	snap2, err := d.GetSnapshot(ctx, executionID2)
	if err != nil {
		return nil, err
	}

	var diffs []Difference

	// Compare status
	if snap1.Status != snap2.Status {
		diffs = append(diffs, Difference{
			Field:    "status",
			Original: snap1.Status,
			Replay:   snap2.Status,
		})
	}

	// Compare response
	if snap1.Response.StatusCode != snap2.Response.StatusCode {
		diffs = append(diffs, Difference{
			Field:    "response.status_code",
			Original: jsonString(snap1.Response.StatusCode),
			Replay:   jsonString(snap2.Response.StatusCode),
		})
	}

	// Compare request
	if snap1.Request.URL != snap2.Request.URL {
		diffs = append(diffs, Difference{
			Field:    "request.url",
			Original: snap1.Request.URL,
			Replay:   snap2.Request.URL,
		})
	}

	// Compare job config version
	if snap1.JobConfig.Version != snap2.JobConfig.Version {
		diffs = append(diffs, Difference{
			Field:    "job_config.version",
			Original: jsonString(snap1.JobConfig.Version),
			Replay:   jsonString(snap2.JobConfig.Version),
		})
	}

	return diffs, nil
}

// ListSnapshots lists execution snapshots for a job.
func (d *Debugger) ListSnapshots(ctx context.Context, jobID string, limit int) ([]*ExecutionSnapshot, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var result []*ExecutionSnapshot
	for _, snap := range d.snapshots {
		if jobID == "" || snap.JobID == jobID {
			result = append(result, snap)
		}
	}

	// Sort by created time descending
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].CreatedAt.Before(result[j].CreatedAt) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result, nil
}

func jsonString(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
