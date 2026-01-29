// Package chaos provides chaos engineering capabilities for testing Chronos resilience.
package chaos

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"
)

// Common errors.
var (
	ErrChaosInjected     = errors.New("chaos: injected failure")
	ErrExperimentExists  = errors.New("experiment already exists")
	ErrExperimentNotFound = errors.New("experiment not found")
)

// FaultType represents the type of fault to inject.
type FaultType string

const (
	// FaultLatency adds artificial latency.
	FaultLatency FaultType = "latency"
	// FaultError returns an error.
	FaultError FaultType = "error"
	// FaultTimeout simulates a timeout.
	FaultTimeout FaultType = "timeout"
	// FaultPartition simulates network partition.
	FaultPartition FaultType = "partition"
	// FaultCPU simulates CPU stress.
	FaultCPU FaultType = "cpu"
	// FaultMemory simulates memory pressure.
	FaultMemory FaultType = "memory"
	// FaultDisk simulates disk slowdown.
	FaultDisk FaultType = "disk"
)

// Experiment defines a chaos experiment.
type Experiment struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	FaultType   FaultType     `json:"fault_type"`
	Target      Target        `json:"target"`
	Config      FaultConfig   `json:"config"`
	Schedule    *Schedule     `json:"schedule,omitempty"`
	Status      Status        `json:"status"`
	StartTime   time.Time     `json:"start_time,omitempty"`
	EndTime     time.Time     `json:"end_time,omitempty"`
	Results     *Results      `json:"results,omitempty"`
}

// Target specifies what to target with the experiment.
type Target struct {
	// JobIDs targets specific jobs.
	JobIDs []string `json:"job_ids,omitempty"`
	// JobTags targets jobs with specific tags.
	JobTags map[string]string `json:"job_tags,omitempty"`
	// Percentage of targets to affect.
	Percentage float64 `json:"percentage,omitempty"`
	// Nodes targets specific nodes.
	Nodes []string `json:"nodes,omitempty"`
	// All targets all matching resources.
	All bool `json:"all,omitempty"`
}

// FaultConfig configures fault injection parameters.
type FaultConfig struct {
	// Duration of the fault.
	Duration time.Duration `json:"duration,omitempty"`
	// Latency to add (for latency fault).
	Latency time.Duration `json:"latency,omitempty"`
	// LatencyJitter adds randomness to latency.
	LatencyJitter time.Duration `json:"latency_jitter,omitempty"`
	// ErrorMessage for error faults.
	ErrorMessage string `json:"error_message,omitempty"`
	// ErrorRate is the probability of error (0.0-1.0).
	ErrorRate float64 `json:"error_rate,omitempty"`
	// CPUPercent for CPU faults.
	CPUPercent int `json:"cpu_percent,omitempty"`
	// MemoryMB for memory faults.
	MemoryMB int `json:"memory_mb,omitempty"`
}

// Schedule defines when the experiment runs.
type Schedule struct {
	// Cron expression for periodic experiments.
	Cron string `json:"cron,omitempty"`
	// Once runs the experiment once immediately.
	Once bool `json:"once,omitempty"`
	// Duration of the experiment.
	Duration time.Duration `json:"duration,omitempty"`
}

// Status represents experiment status.
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusAborted   Status = "aborted"
)

// Results contains experiment results.
type Results struct {
	AffectedJobs     []string      `json:"affected_jobs"`
	TotalExecutions  int           `json:"total_executions"`
	FailedExecutions int           `json:"failed_executions"`
	AvgLatencyAdded  time.Duration `json:"avg_latency_added"`
	Observations     []string      `json:"observations"`
}

// Engine runs chaos experiments.
type Engine struct {
	mu          sync.RWMutex
	experiments map[string]*Experiment
	active      map[string]context.CancelFunc
	hooks       []Hook
	rng         *rand.Rand
	enabled     bool
}

// Hook is called during chaos events.
type Hook func(event Event)

// Event represents a chaos event.
type Event struct {
	Type       string
	Experiment *Experiment
	JobID      string
	Message    string
	Timestamp  time.Time
}

// NewEngine creates a new chaos engine.
func NewEngine() *Engine {
	return &Engine{
		experiments: make(map[string]*Experiment),
		active:      make(map[string]context.CancelFunc),
		hooks:       make([]Hook, 0),
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
		enabled:     false,
	}
}

// Enable enables chaos mode.
func (e *Engine) Enable() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.enabled = true
}

// Disable disables chaos mode.
func (e *Engine) Disable() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.enabled = false
}

// IsEnabled returns whether chaos mode is enabled.
func (e *Engine) IsEnabled() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.enabled
}

// AddHook adds an event hook.
func (e *Engine) AddHook(hook Hook) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.hooks = append(e.hooks, hook)
}

// CreateExperiment creates a new experiment.
func (e *Engine) CreateExperiment(exp *Experiment) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.experiments[exp.ID]; exists {
		return ErrExperimentExists
	}

	exp.Status = StatusPending
	e.experiments[exp.ID] = exp
	return nil
}

// GetExperiment returns an experiment by ID.
func (e *Engine) GetExperiment(id string) (*Experiment, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	exp, exists := e.experiments[id]
	if !exists {
		return nil, ErrExperimentNotFound
	}
	return exp, nil
}

// ListExperiments returns all experiments.
func (e *Engine) ListExperiments() []*Experiment {
	e.mu.RLock()
	defer e.mu.RUnlock()

	exps := make([]*Experiment, 0, len(e.experiments))
	for _, exp := range e.experiments {
		exps = append(exps, exp)
	}
	return exps
}

// StartExperiment starts an experiment.
func (e *Engine) StartExperiment(ctx context.Context, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	exp, exists := e.experiments[id]
	if !exists {
		return ErrExperimentNotFound
	}

	if exp.Status == StatusRunning {
		return nil
	}

	exp.Status = StatusRunning
	exp.StartTime = time.Now()

	expCtx, cancel := context.WithCancel(ctx)
	e.active[id] = cancel

	go e.runExperiment(expCtx, exp)

	e.fireEvent(Event{
		Type:       "experiment_started",
		Experiment: exp,
		Timestamp:  time.Now(),
	})

	return nil
}

// StopExperiment stops a running experiment.
func (e *Engine) StopExperiment(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	exp, exists := e.experiments[id]
	if !exists {
		return ErrExperimentNotFound
	}

	if cancel, ok := e.active[id]; ok {
		cancel()
		delete(e.active, id)
	}

	exp.Status = StatusAborted
	exp.EndTime = time.Now()

	e.fireEvent(Event{
		Type:       "experiment_stopped",
		Experiment: exp,
		Timestamp:  time.Now(),
	})

	return nil
}

// DeleteExperiment deletes an experiment.
func (e *Engine) DeleteExperiment(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.experiments[id]; !exists {
		return ErrExperimentNotFound
	}

	if cancel, ok := e.active[id]; ok {
		cancel()
		delete(e.active, id)
	}

	delete(e.experiments, id)
	return nil
}

// runExperiment executes an experiment.
func (e *Engine) runExperiment(ctx context.Context, exp *Experiment) {
	defer func() {
		e.mu.Lock()
		exp.EndTime = time.Now()
		if exp.Status == StatusRunning {
			exp.Status = StatusCompleted
		}
		delete(e.active, exp.ID)
		e.mu.Unlock()

		e.fireEvent(Event{
			Type:       "experiment_completed",
			Experiment: exp,
			Timestamp:  time.Now(),
		})
	}()

	duration := exp.Config.Duration
	if duration == 0 && exp.Schedule != nil {
		duration = exp.Schedule.Duration
	}
	if duration == 0 {
		duration = 5 * time.Minute
	}

	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return
	case <-timer.C:
		return
	}
}

// ShouldInjectFault checks if a fault should be injected for a job.
func (e *Engine) ShouldInjectFault(jobID string) (*Experiment, bool) {
	if !e.IsEnabled() {
		return nil, false
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, exp := range e.experiments {
		if exp.Status != StatusRunning {
			continue
		}

		// Check if job is targeted
		if e.isJobTargeted(jobID, exp.Target) {
			// Check error rate
			if exp.Config.ErrorRate > 0 {
				if e.rng.Float64() > exp.Config.ErrorRate {
					continue
				}
			}
			return exp, true
		}
	}

	return nil, false
}

// isJobTargeted checks if a job is targeted by the experiment.
func (e *Engine) isJobTargeted(jobID string, target Target) bool {
	if target.All {
		return true
	}

	for _, id := range target.JobIDs {
		if id == jobID {
			return true
		}
	}

	return false
}

// InjectLatency injects latency for an experiment.
func (e *Engine) InjectLatency(exp *Experiment) {
	latency := exp.Config.Latency
	if exp.Config.LatencyJitter > 0 {
		jitter := time.Duration(e.rng.Int63n(int64(exp.Config.LatencyJitter)))
		latency += jitter
	}
	time.Sleep(latency)
}

// InjectError returns an injected error.
func (e *Engine) InjectError(exp *Experiment) error {
	msg := exp.Config.ErrorMessage
	if msg == "" {
		msg = "chaos: injected failure"
	}
	return &InjectedError{Message: msg, ExperimentID: exp.ID}
}

// InjectedError represents an intentionally injected error.
type InjectedError struct {
	Message      string
	ExperimentID string
}

func (e *InjectedError) Error() string {
	return e.Message
}

// IsInjectedError checks if an error was injected by chaos.
func IsInjectedError(err error) bool {
	_, ok := err.(*InjectedError)
	return ok
}

// fireEvent fires an event to all hooks.
func (e *Engine) fireEvent(event Event) {
	for _, hook := range e.hooks {
		hook(event)
	}
}

// Stats returns chaos engine statistics.
type Stats struct {
	Enabled            bool `json:"enabled"`
	TotalExperiments   int  `json:"total_experiments"`
	ActiveExperiments  int  `json:"active_experiments"`
	CompletedExperiments int `json:"completed_experiments"`
}

// GetStats returns engine statistics.
func (e *Engine) GetStats() Stats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := Stats{
		Enabled:          e.enabled,
		TotalExperiments: len(e.experiments),
		ActiveExperiments: len(e.active),
	}

	for _, exp := range e.experiments {
		if exp.Status == StatusCompleted {
			stats.CompletedExperiments++
		}
	}

	return stats
}
