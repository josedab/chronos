// Package dispatcher provides execution event streaming for live logs.
package dispatcher

import (
	"sync"
	"time"

	"github.com/chronos/chronos/internal/models"
)

// ExecutionEvent represents a lifecycle event during job execution.
type ExecutionEvent struct {
	Type        ExecutionEventType `json:"type"`
	ExecutionID string             `json:"execution_id"`
	JobID       string             `json:"job_id"`
	JobName     string             `json:"job_name"`
	Timestamp   time.Time          `json:"timestamp"`

	// Event-specific data
	Status     models.ExecutionStatus `json:"status,omitempty"`
	Attempt    int                    `json:"attempt,omitempty"`
	StatusCode int                    `json:"status_code,omitempty"`
	Duration   time.Duration          `json:"duration,omitempty"`
	Error      string                 `json:"error,omitempty"`
	Response   string                 `json:"response,omitempty"`
	TraceID    string                 `json:"trace_id,omitempty"`
	NodeID     string                 `json:"node_id,omitempty"`
}

// ExecutionEventType identifies the kind of execution event.
type ExecutionEventType string

const (
	EventExecutionQueued    ExecutionEventType = "execution.queued"
	EventExecutionStarted   ExecutionEventType = "execution.started"
	EventExecutionAttempt   ExecutionEventType = "execution.attempt"
	EventExecutionRetry     ExecutionEventType = "execution.retry"
	EventExecutionCompleted ExecutionEventType = "execution.completed"
	EventExecutionFailed    ExecutionEventType = "execution.failed"
)

// EventSubscriber receives execution events.
type EventSubscriber func(event ExecutionEvent)

// EventBus distributes execution events to subscribers.
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[string][]EventSubscriber // jobID -> subscribers; "" for global
	bufferSize  int
}

// NewEventBus creates a new execution event bus.
func NewEventBus(bufferSize int) *EventBus {
	if bufferSize <= 0 {
		bufferSize = 1000
	}
	return &EventBus{
		subscribers: make(map[string][]EventSubscriber),
		bufferSize:  bufferSize,
	}
}

// Subscribe registers a subscriber for events. Use jobID="" for all events.
func (eb *EventBus) Subscribe(jobID string, sub EventSubscriber) func() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	// Use a wrapper that can be disabled
	active := true
	wrapper := func(e ExecutionEvent) {
		if active {
			sub(e)
		}
	}
	eb.subscribers[jobID] = append(eb.subscribers[jobID], wrapper)

	return func() {
		active = false
	}
}

// Publish sends an event to all matching subscribers.
func (eb *EventBus) Publish(event ExecutionEvent) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	// Send to job-specific subscribers
	if event.JobID != "" {
		for _, sub := range eb.subscribers[event.JobID] {
			sub(event)
		}
	}

	// Send to global subscribers
	for _, sub := range eb.subscribers[""] {
		sub(event)
	}
}

// SubscriberCount returns the total number of subscribers.
func (eb *EventBus) SubscriberCount() int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	count := 0
	for _, subs := range eb.subscribers {
		count += len(subs)
	}
	return count
}

// EmitQueued emits an execution.queued event.
func (eb *EventBus) EmitQueued(exec *models.Execution) {
	eb.Publish(ExecutionEvent{
		Type:        EventExecutionQueued,
		ExecutionID: exec.ID,
		JobID:       exec.JobID,
		JobName:     exec.JobName,
		Timestamp:   time.Now(),
		Status:      models.ExecutionPending,
		TraceID:     exec.TraceID,
		NodeID:      exec.NodeID,
	})
}

// EmitStarted emits an execution.started event.
func (eb *EventBus) EmitStarted(exec *models.Execution) {
	eb.Publish(ExecutionEvent{
		Type:        EventExecutionStarted,
		ExecutionID: exec.ID,
		JobID:       exec.JobID,
		JobName:     exec.JobName,
		Timestamp:   time.Now(),
		Status:      models.ExecutionRunning,
		TraceID:     exec.TraceID,
		NodeID:      exec.NodeID,
	})
}

// EmitAttempt emits an execution.attempt event for retry tracking.
func (eb *EventBus) EmitAttempt(exec *models.Execution, attempt int, statusCode int, err string) {
	eb.Publish(ExecutionEvent{
		Type:        EventExecutionAttempt,
		ExecutionID: exec.ID,
		JobID:       exec.JobID,
		JobName:     exec.JobName,
		Timestamp:   time.Now(),
		Attempt:     attempt,
		StatusCode:  statusCode,
		Error:       err,
		TraceID:     exec.TraceID,
	})
}

// EmitCompleted emits an execution.completed event.
func (eb *EventBus) EmitCompleted(exec *models.Execution) {
	eb.Publish(ExecutionEvent{
		Type:        EventExecutionCompleted,
		ExecutionID: exec.ID,
		JobID:       exec.JobID,
		JobName:     exec.JobName,
		Timestamp:   time.Now(),
		Status:      exec.Status,
		StatusCode:  exec.StatusCode,
		Duration:    exec.Duration,
		Response:    truncate(exec.Response, 500),
		TraceID:     exec.TraceID,
	})
}

// EmitFailed emits an execution.failed event.
func (eb *EventBus) EmitFailed(exec *models.Execution) {
	eb.Publish(ExecutionEvent{
		Type:        EventExecutionFailed,
		ExecutionID: exec.ID,
		JobID:       exec.JobID,
		JobName:     exec.JobName,
		Timestamp:   time.Now(),
		Status:      models.ExecutionFailed,
		Error:       exec.Error,
		Duration:    exec.Duration,
		TraceID:     exec.TraceID,
	})
}
