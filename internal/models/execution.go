// Package models defines the core data structures for Chronos.
package models

import "time"

// Execution represents a job execution.
type Execution struct {
	ID            string          `json:"id"`
	JobID         string          `json:"job_id"`
	JobName       string          `json:"job_name,omitempty"`
	ScheduledTime time.Time       `json:"scheduled_time"`
	StartedAt     time.Time       `json:"started_at"`
	CompletedAt   *time.Time      `json:"completed_at,omitempty"`
	Status        ExecutionStatus `json:"status"`
	Attempts      int             `json:"attempts"`
	StatusCode    int             `json:"status_code,omitempty"`
	Response      string          `json:"response,omitempty"`
	Error         string          `json:"error,omitempty"`
	Duration      time.Duration   `json:"duration,omitempty"`
	NodeID        string          `json:"node_id,omitempty"`

	// Debug information for replay
	Request      *ExecutionRequest  `json:"request,omitempty"`
	ReplayOf     string             `json:"replay_of,omitempty"`
	ReplayCount  int                `json:"replay_count,omitempty"`

	// Assertion results
	AssertionResult *AssertionResult `json:"assertion_result,omitempty"`

	// Tracing
	TraceID string `json:"trace_id,omitempty"`
	SpanID  string `json:"span_id,omitempty"`
}

// ExecutionRequest captures the original request for debugging and replay.
type ExecutionRequest struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

// ExecutionStatus represents the status of an execution.
type ExecutionStatus string

const (
	ExecutionPending ExecutionStatus = "pending"
	ExecutionRunning ExecutionStatus = "running"
	ExecutionSuccess ExecutionStatus = "success"
	ExecutionFailed  ExecutionStatus = "failed"
	ExecutionSkipped ExecutionStatus = "skipped"
)

// IsTerminal returns true if the execution is in a terminal state.
func (s ExecutionStatus) IsTerminal() bool {
	return s == ExecutionSuccess || s == ExecutionFailed || s == ExecutionSkipped
}

// ExecutionResult contains the result of a webhook execution.
type ExecutionResult struct {
	Success    bool
	StatusCode int
	Response   string
	Error      string
}

// ScheduleState tracks the scheduling state for a job.
type ScheduleState struct {
	JobID       string    `json:"job_id"`
	NextRun     time.Time `json:"next_run"`
	LastRun     time.Time `json:"last_run,omitempty"`
	MissedCount int       `json:"missed_count"`
}
