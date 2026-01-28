// Package replay provides enhanced time-travel debugging capabilities.
package replay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Additional errors for time-travel debugging.
var (
	ErrBreakpointNotFound = errors.New("breakpoint not found")
	ErrSessionNotFound    = errors.New("debug session not found")
	ErrStepNotAllowed     = errors.New("step not allowed in current state")
)

// DebugSession represents an interactive debugging session.
type DebugSession struct {
	ID            string                 `json:"id"`
	ExecutionID   string                 `json:"execution_id"`
	Snapshot      *ExecutionSnapshot     `json:"snapshot"`
	State         DebugState             `json:"state"`
	CurrentStep   int                    `json:"current_step"`
	Steps         []*DebugStep           `json:"steps"`
	Breakpoints   map[string]*Breakpoint `json:"breakpoints"`
	WatchExprs    []WatchExpression      `json:"watch_expressions"`
	History       []*StateChange         `json:"history"`
	Variables     map[string]interface{} `json:"variables"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
	mu            sync.RWMutex
}

// DebugState represents the current state of debugging.
type DebugState string

const (
	DebugStateInitialized DebugState = "initialized"
	DebugStateRunning     DebugState = "running"
	DebugStatePaused      DebugState = "paused"
	DebugStateBreakpoint  DebugState = "at_breakpoint"
	DebugStateCompleted   DebugState = "completed"
	DebugStateError       DebugState = "error"
)

// DebugStep represents a step in the execution that can be inspected.
type DebugStep struct {
	Index       int                    `json:"index"`
	Type        StepType               `json:"type"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Timestamp   time.Time              `json:"timestamp"`
	Duration    time.Duration          `json:"duration"`
	Input       map[string]interface{} `json:"input,omitempty"`
	Output      map[string]interface{} `json:"output,omitempty"`
	State       map[string]interface{} `json:"state"`
	Error       string                 `json:"error,omitempty"`
}

// StepType represents the type of debug step.
type StepType string

const (
	StepTypeSchedule   StepType = "schedule"
	StepTypePrepare    StepType = "prepare"
	StepTypeAuth       StepType = "auth"
	StepTypeRequest    StepType = "request"
	StepTypeResponse   StepType = "response"
	StepTypeRetry      StepType = "retry"
	StepTypeComplete   StepType = "complete"
	StepTypeError      StepType = "error"
)

// Breakpoint represents a debugging breakpoint.
type Breakpoint struct {
	ID        string            `json:"id"`
	Type      BreakpointType    `json:"type"`
	Condition string            `json:"condition,omitempty"`
	StepIndex *int              `json:"step_index,omitempty"`
	StepType  *StepType         `json:"step_type,omitempty"`
	Enabled   bool              `json:"enabled"`
	HitCount  int               `json:"hit_count"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// BreakpointType represents the type of breakpoint.
type BreakpointType string

const (
	BreakpointTypeStep       BreakpointType = "step"
	BreakpointTypeStepType   BreakpointType = "step_type"
	BreakpointTypeCondition  BreakpointType = "condition"
	BreakpointTypeError      BreakpointType = "error"
)

// WatchExpression represents a watched variable or expression.
type WatchExpression struct {
	ID         string      `json:"id"`
	Expression string      `json:"expression"`
	Value      interface{} `json:"value"`
	LastUpdate time.Time   `json:"last_update"`
}

// StateChange represents a change in state during debugging.
type StateChange struct {
	Timestamp time.Time              `json:"timestamp"`
	StepIndex int                    `json:"step_index"`
	Changes   []VariableChange       `json:"changes"`
}

// VariableChange represents a single variable change.
type VariableChange struct {
	Name     string      `json:"name"`
	OldValue interface{} `json:"old_value"`
	NewValue interface{} `json:"new_value"`
}

// StepResult contains the result of stepping through execution.
type StepResult struct {
	CurrentStep *DebugStep             `json:"current_step"`
	State       DebugState             `json:"state"`
	Variables   map[string]interface{} `json:"variables"`
	Watches     []WatchExpression      `json:"watches"`
	HitBreakpoint *Breakpoint          `json:"hit_breakpoint,omitempty"`
}

// TimeTravelDebugger provides enhanced debugging with time-travel capabilities.
type TimeTravelDebugger struct {
	debugger *Debugger
	sessions map[string]*DebugSession
	mu       sync.RWMutex
}

// NewTimeTravelDebugger creates a new time-travel debugger.
func NewTimeTravelDebugger(debugger *Debugger) *TimeTravelDebugger {
	return &TimeTravelDebugger{
		debugger: debugger,
		sessions: make(map[string]*DebugSession),
	}
}

// CreateSession creates a new debug session for an execution.
func (t *TimeTravelDebugger) CreateSession(ctx context.Context, executionID string) (*DebugSession, error) {
	snapshot, err := t.debugger.GetSnapshot(ctx, executionID)
	if err != nil {
		return nil, err
	}

	session := &DebugSession{
		ID:          uuid.New().String(),
		ExecutionID: executionID,
		Snapshot:    snapshot,
		State:       DebugStateInitialized,
		CurrentStep: -1,
		Steps:       t.buildSteps(snapshot),
		Breakpoints: make(map[string]*Breakpoint),
		WatchExprs:  make([]WatchExpression, 0),
		History:     make([]*StateChange, 0),
		Variables:   t.initVariables(snapshot),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	t.mu.Lock()
	t.sessions[session.ID] = session
	t.mu.Unlock()

	return session, nil
}

// GetSession retrieves a debug session.
func (t *TimeTravelDebugger) GetSession(sessionID string) (*DebugSession, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	session, exists := t.sessions[sessionID]
	if !exists {
		return nil, ErrSessionNotFound
	}
	return session, nil
}

// buildSteps builds debug steps from an execution snapshot.
func (t *TimeTravelDebugger) buildSteps(snapshot *ExecutionSnapshot) []*DebugStep {
	steps := make([]*DebugStep, 0)
	baseTime := snapshot.ScheduledAt

	// Step 1: Scheduled
	steps = append(steps, &DebugStep{
		Index:       0,
		Type:        StepTypeSchedule,
		Name:        "Scheduled",
		Description: "Job scheduled for execution",
		Timestamp:   baseTime,
		Duration:    snapshot.StartedAt.Sub(baseTime),
		State: map[string]interface{}{
			"job_id":        snapshot.JobID,
			"job_name":      snapshot.JobName,
			"scheduled_at":  baseTime,
			"schedule":      snapshot.JobConfig.Schedule,
		},
	})

	// Step 2: Prepare
	steps = append(steps, &DebugStep{
		Index:       1,
		Type:        StepTypePrepare,
		Name:        "Prepare Request",
		Description: "Building HTTP request",
		Timestamp:   snapshot.StartedAt,
		Duration:    time.Millisecond, // Estimate
		Input: map[string]interface{}{
			"config": snapshot.JobConfig,
		},
		Output: map[string]interface{}{
			"method":  snapshot.Request.Method,
			"url":     snapshot.Request.URL,
			"headers": snapshot.Request.Headers,
		},
		State: map[string]interface{}{
			"attempt":      snapshot.Attempt,
			"max_attempts": snapshot.MaxAttempts,
		},
	})

	// Step 3: Auth (if applicable)
	if len(snapshot.RedactedSecrets) > 0 {
		steps = append(steps, &DebugStep{
			Index:       2,
			Type:        StepTypeAuth,
			Name:        "Apply Authentication",
			Description: "Adding authentication to request",
			Timestamp:   snapshot.StartedAt.Add(time.Millisecond),
			Duration:    time.Millisecond,
			State: map[string]interface{}{
				"auth_applied":     true,
				"redacted_secrets": snapshot.RedactedSecrets,
			},
		})
	}

	// Step 4: Request
	requestStep := &DebugStep{
		Index:       len(steps),
		Type:        StepTypeRequest,
		Name:        "Send Request",
		Description: fmt.Sprintf("%s %s", snapshot.Request.Method, snapshot.Request.URL),
		Timestamp:   snapshot.StartedAt.Add(2 * time.Millisecond),
		Duration:    snapshot.Duration - 2*time.Millisecond,
		Input: map[string]interface{}{
			"method":  snapshot.Request.Method,
			"url":     snapshot.Request.URL,
			"headers": snapshot.Request.Headers,
			"body":    snapshot.Request.Body,
		},
		State: map[string]interface{}{
			"node_id":   snapshot.Context.NodeID,
			"trace_id":  snapshot.Context.TraceID,
		},
	}
	steps = append(steps, requestStep)

	// Step 5: Response
	responseStep := &DebugStep{
		Index:       len(steps),
		Type:        StepTypeResponse,
		Name:        "Receive Response",
		Description: fmt.Sprintf("HTTP %d", snapshot.Response.StatusCode),
		Timestamp:   snapshot.EndedAt.Add(-time.Millisecond),
		Duration:    time.Millisecond,
		Output: map[string]interface{}{
			"status_code": snapshot.Response.StatusCode,
			"headers":     snapshot.Response.Headers,
			"body":        snapshot.Response.Body,
			"body_size":   snapshot.Response.BodySize,
		},
		State: map[string]interface{}{
			"success": snapshot.Status == "success",
		},
	}
	if snapshot.Error != "" {
		responseStep.Error = snapshot.Error
	}
	steps = append(steps, responseStep)

	// Step 6: Complete
	steps = append(steps, &DebugStep{
		Index:       len(steps),
		Type:        StepTypeComplete,
		Name:        "Execution Complete",
		Description: fmt.Sprintf("Status: %s", snapshot.Status),
		Timestamp:   snapshot.EndedAt,
		Duration:    0,
		State: map[string]interface{}{
			"final_status": snapshot.Status,
			"duration":     snapshot.Duration.String(),
			"attempts":     snapshot.Attempt,
		},
		Error: snapshot.Error,
	})

	return steps
}

// initVariables initializes variables from snapshot.
func (t *TimeTravelDebugger) initVariables(snapshot *ExecutionSnapshot) map[string]interface{} {
	return map[string]interface{}{
		"job_id":        snapshot.JobID,
		"job_name":      snapshot.JobName,
		"execution_id":  snapshot.ExecutionID,
		"scheduled_at":  snapshot.ScheduledAt,
		"status":        snapshot.Status,
		"attempt":       snapshot.Attempt,
		"max_attempts":  snapshot.MaxAttempts,
		"request":       snapshot.Request,
		"response":      snapshot.Response,
		"duration":      snapshot.Duration,
		"node_id":       snapshot.Context.NodeID,
	}
}

// StepForward moves to the next step.
func (t *TimeTravelDebugger) StepForward(sessionID string) (*StepResult, error) {
	session, err := t.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.CurrentStep >= len(session.Steps)-1 {
		session.State = DebugStateCompleted
		return &StepResult{
			State:     session.State,
			Variables: session.Variables,
			Watches:   session.WatchExprs,
		}, nil
	}

	session.CurrentStep++
	session.State = DebugStateRunning
	session.UpdatedAt = time.Now()

	currentStep := session.Steps[session.CurrentStep]

	// Check breakpoints
	hitBreakpoint := t.checkBreakpoints(session, currentStep)
	if hitBreakpoint != nil {
		session.State = DebugStateBreakpoint
		hitBreakpoint.HitCount++
	}

	// Update variables based on step state
	t.updateVariables(session, currentStep)

	// Update watches
	t.updateWatches(session)

	return &StepResult{
		CurrentStep:   currentStep,
		State:         session.State,
		Variables:     session.Variables,
		Watches:       session.WatchExprs,
		HitBreakpoint: hitBreakpoint,
	}, nil
}

// StepBackward moves to the previous step (time travel!).
func (t *TimeTravelDebugger) StepBackward(sessionID string) (*StepResult, error) {
	session, err := t.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.CurrentStep <= 0 {
		return &StepResult{
			CurrentStep: session.Steps[0],
			State:       session.State,
			Variables:   session.Variables,
			Watches:     session.WatchExprs,
		}, nil
	}

	session.CurrentStep--
	session.State = DebugStatePaused
	session.UpdatedAt = time.Now()

	currentStep := session.Steps[session.CurrentStep]

	// Restore variables to this step's state
	t.restoreVariables(session, session.CurrentStep)

	// Update watches
	t.updateWatches(session)

	return &StepResult{
		CurrentStep: currentStep,
		State:       session.State,
		Variables:   session.Variables,
		Watches:     session.WatchExprs,
	}, nil
}

// GoToStep jumps to a specific step.
func (t *TimeTravelDebugger) GoToStep(sessionID string, stepIndex int) (*StepResult, error) {
	session, err := t.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if stepIndex < 0 || stepIndex >= len(session.Steps) {
		return nil, ErrStepNotAllowed
	}

	session.CurrentStep = stepIndex
	session.State = DebugStatePaused
	session.UpdatedAt = time.Now()

	currentStep := session.Steps[session.CurrentStep]

	// Restore variables to this step's state
	t.restoreVariables(session, stepIndex)

	// Check breakpoints
	hitBreakpoint := t.checkBreakpoints(session, currentStep)
	if hitBreakpoint != nil {
		session.State = DebugStateBreakpoint
	}

	// Update watches
	t.updateWatches(session)

	return &StepResult{
		CurrentStep:   currentStep,
		State:         session.State,
		Variables:     session.Variables,
		Watches:       session.WatchExprs,
		HitBreakpoint: hitBreakpoint,
	}, nil
}

// Continue runs until a breakpoint or completion.
func (t *TimeTravelDebugger) Continue(sessionID string) (*StepResult, error) {
	session, err := t.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	for session.CurrentStep < len(session.Steps)-1 {
		session.CurrentStep++
		currentStep := session.Steps[session.CurrentStep]

		// Update variables
		t.updateVariables(session, currentStep)

		// Check breakpoints
		hitBreakpoint := t.checkBreakpoints(session, currentStep)
		if hitBreakpoint != nil {
			session.State = DebugStateBreakpoint
			hitBreakpoint.HitCount++
			t.updateWatches(session)
			return &StepResult{
				CurrentStep:   currentStep,
				State:         session.State,
				Variables:     session.Variables,
				Watches:       session.WatchExprs,
				HitBreakpoint: hitBreakpoint,
			}, nil
		}
	}

	session.State = DebugStateCompleted
	session.UpdatedAt = time.Now()
	t.updateWatches(session)

	return &StepResult{
		CurrentStep: session.Steps[session.CurrentStep],
		State:       session.State,
		Variables:   session.Variables,
		Watches:     session.WatchExprs,
	}, nil
}

// AddBreakpoint adds a breakpoint.
func (t *TimeTravelDebugger) AddBreakpoint(sessionID string, bp *Breakpoint) error {
	session, err := t.GetSession(sessionID)
	if err != nil {
		return err
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if bp.ID == "" {
		bp.ID = uuid.New().String()
	}
	bp.Enabled = true
	bp.HitCount = 0

	session.Breakpoints[bp.ID] = bp
	session.UpdatedAt = time.Now()

	return nil
}

// RemoveBreakpoint removes a breakpoint.
func (t *TimeTravelDebugger) RemoveBreakpoint(sessionID, breakpointID string) error {
	session, err := t.GetSession(sessionID)
	if err != nil {
		return err
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if _, exists := session.Breakpoints[breakpointID]; !exists {
		return ErrBreakpointNotFound
	}

	delete(session.Breakpoints, breakpointID)
	session.UpdatedAt = time.Now()

	return nil
}

// AddWatch adds a watch expression.
func (t *TimeTravelDebugger) AddWatch(sessionID, expression string) (*WatchExpression, error) {
	session, err := t.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	watch := WatchExpression{
		ID:         uuid.New().String(),
		Expression: expression,
		LastUpdate: time.Now(),
	}

	// Evaluate the expression
	watch.Value = t.evaluateExpression(session, expression)

	session.WatchExprs = append(session.WatchExprs, watch)
	session.UpdatedAt = time.Now()

	return &watch, nil
}

// GetFullTimeline returns the complete execution timeline with all steps.
func (t *TimeTravelDebugger) GetFullTimeline(sessionID string) ([]*DebugStep, error) {
	session, err := t.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	session.mu.RLock()
	defer session.mu.RUnlock()

	return session.Steps, nil
}

// CompareExecutions compares two executions for debugging.
func (t *TimeTravelDebugger) CompareExecutions(ctx context.Context, executionID1, executionID2 string) (*ExecutionComparison, error) {
	snap1, err := t.debugger.GetSnapshot(ctx, executionID1)
	if err != nil {
		return nil, fmt.Errorf("execution 1: %w", err)
	}

	snap2, err := t.debugger.GetSnapshot(ctx, executionID2)
	if err != nil {
		return nil, fmt.Errorf("execution 2: %w", err)
	}

	comparison := &ExecutionComparison{
		Execution1: ExecutionComparisonItem{
			ExecutionID: executionID1,
			JobID:       snap1.JobID,
			Status:      snap1.Status,
			Duration:    snap1.Duration,
			Timestamp:   snap1.StartedAt,
		},
		Execution2: ExecutionComparisonItem{
			ExecutionID: executionID2,
			JobID:       snap2.JobID,
			Status:      snap2.Status,
			Duration:    snap2.Duration,
			Timestamp:   snap2.StartedAt,
		},
		Differences: make([]ComparisonDiff, 0),
	}

	// Compare request
	if snap1.Request.URL != snap2.Request.URL {
		comparison.Differences = append(comparison.Differences, ComparisonDiff{
			Field:  "request.url",
			Value1: snap1.Request.URL,
			Value2: snap2.Request.URL,
		})
	}

	if snap1.Request.Method != snap2.Request.Method {
		comparison.Differences = append(comparison.Differences, ComparisonDiff{
			Field:  "request.method",
			Value1: snap1.Request.Method,
			Value2: snap2.Request.Method,
		})
	}

	// Compare response
	if snap1.Response.StatusCode != snap2.Response.StatusCode {
		comparison.Differences = append(comparison.Differences, ComparisonDiff{
			Field:  "response.status_code",
			Value1: fmt.Sprintf("%d", snap1.Response.StatusCode),
			Value2: fmt.Sprintf("%d", snap2.Response.StatusCode),
		})
	}

	// Compare status
	if snap1.Status != snap2.Status {
		comparison.Differences = append(comparison.Differences, ComparisonDiff{
			Field:  "status",
			Value1: snap1.Status,
			Value2: snap2.Status,
		})
	}

	// Compare duration (significant if >20% difference)
	durationDiff := float64(snap2.Duration-snap1.Duration) / float64(snap1.Duration)
	if durationDiff > 0.2 || durationDiff < -0.2 {
		comparison.Differences = append(comparison.Differences, ComparisonDiff{
			Field:  "duration",
			Value1: snap1.Duration.String(),
			Value2: snap2.Duration.String(),
		})
	}

	// Compare config version
	if snap1.JobConfig.Version != snap2.JobConfig.Version {
		comparison.Differences = append(comparison.Differences, ComparisonDiff{
			Field:  "job_config.version",
			Value1: fmt.Sprintf("%d", snap1.JobConfig.Version),
			Value2: fmt.Sprintf("%d", snap2.JobConfig.Version),
		})
	}

	return comparison, nil
}

// ExecutionComparison contains the comparison of two executions.
type ExecutionComparison struct {
	Execution1  ExecutionComparisonItem `json:"execution_1"`
	Execution2  ExecutionComparisonItem `json:"execution_2"`
	Differences []ComparisonDiff        `json:"differences"`
}

// ExecutionComparisonItem is a summary for comparison.
type ExecutionComparisonItem struct {
	ExecutionID string        `json:"execution_id"`
	JobID       string        `json:"job_id"`
	Status      string        `json:"status"`
	Duration    time.Duration `json:"duration"`
	Timestamp   time.Time     `json:"timestamp"`
}

// ComparisonDiff represents a difference between executions.
type ComparisonDiff struct {
	Field  string `json:"field"`
	Value1 string `json:"value_1"`
	Value2 string `json:"value_2"`
}

// checkBreakpoints checks if any breakpoints are hit.
func (t *TimeTravelDebugger) checkBreakpoints(session *DebugSession, step *DebugStep) *Breakpoint {
	for _, bp := range session.Breakpoints {
		if !bp.Enabled {
			continue
		}

		hit := false
		switch bp.Type {
		case BreakpointTypeStep:
			if bp.StepIndex != nil && *bp.StepIndex == step.Index {
				hit = true
			}
		case BreakpointTypeStepType:
			if bp.StepType != nil && *bp.StepType == step.Type {
				hit = true
			}
		case BreakpointTypeError:
			if step.Error != "" {
				hit = true
			}
		case BreakpointTypeCondition:
			// Evaluate condition
			if bp.Condition != "" {
				hit = t.evaluateCondition(session, bp.Condition)
			}
		}

		if hit {
			return bp
		}
	}
	return nil
}

// updateVariables updates session variables based on current step.
func (t *TimeTravelDebugger) updateVariables(session *DebugSession, step *DebugStep) {
	// Merge step state into variables
	for k, v := range step.State {
		session.Variables[k] = v
	}

	// Track changes
	session.History = append(session.History, &StateChange{
		Timestamp: time.Now(),
		StepIndex: step.Index,
	})
}

// restoreVariables restores variables to a specific step.
func (t *TimeTravelDebugger) restoreVariables(session *DebugSession, targetStep int) {
	// Reinitialize from snapshot
	session.Variables = t.initVariables(session.Snapshot)

	// Replay variable changes up to target step
	for i := 0; i <= targetStep && i < len(session.Steps); i++ {
		for k, v := range session.Steps[i].State {
			session.Variables[k] = v
		}
	}
}

// updateWatches updates all watch expressions.
func (t *TimeTravelDebugger) updateWatches(session *DebugSession) {
	for i := range session.WatchExprs {
		session.WatchExprs[i].Value = t.evaluateExpression(session, session.WatchExprs[i].Expression)
		session.WatchExprs[i].LastUpdate = time.Now()
	}
}

// evaluateExpression evaluates a simple expression against variables.
func (t *TimeTravelDebugger) evaluateExpression(session *DebugSession, expr string) interface{} {
	// Simple variable lookup
	if val, exists := session.Variables[expr]; exists {
		return val
	}

	// Nested lookup (e.g., "request.url")
	// This is a simplified implementation
	return nil
}

// evaluateCondition evaluates a breakpoint condition.
func (t *TimeTravelDebugger) evaluateCondition(session *DebugSession, condition string) bool {
	// Simplified condition evaluation
	// In a real implementation, this would parse and evaluate the condition
	return false
}

// GetSessionStats returns statistics for a debug session.
func (t *TimeTravelDebugger) GetSessionStats(sessionID string) (*DebugSessionStats, error) {
	session, err := t.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	session.mu.RLock()
	defer session.mu.RUnlock()

	// Count step types
	stepTypeCounts := make(map[StepType]int)
	for _, step := range session.Steps {
		stepTypeCounts[step.Type]++
	}

	// Find slowest step
	var slowestStep *DebugStep
	for _, step := range session.Steps {
		if slowestStep == nil || step.Duration > slowestStep.Duration {
			slowestStep = step
		}
	}

	return &DebugSessionStats{
		SessionID:      sessionID,
		TotalSteps:     len(session.Steps),
		CurrentStep:    session.CurrentStep,
		State:          session.State,
		StepTypeCounts: stepTypeCounts,
		Breakpoints:    len(session.Breakpoints),
		Watches:        len(session.WatchExprs),
		SlowestStep:    slowestStep,
		SessionAge:     time.Since(session.CreatedAt),
	}, nil
}

// DebugSessionStats contains statistics for a debug session.
type DebugSessionStats struct {
	SessionID      string            `json:"session_id"`
	TotalSteps     int               `json:"total_steps"`
	CurrentStep    int               `json:"current_step"`
	State          DebugState        `json:"state"`
	StepTypeCounts map[StepType]int  `json:"step_type_counts"`
	Breakpoints    int               `json:"breakpoints"`
	Watches        int               `json:"watches"`
	SlowestStep    *DebugStep        `json:"slowest_step,omitempty"`
	SessionAge     time.Duration     `json:"session_age"`
}

// CloseSession closes and cleans up a debug session.
func (t *TimeTravelDebugger) CloseSession(sessionID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.sessions[sessionID]; !exists {
		return ErrSessionNotFound
	}

	delete(t.sessions, sessionID)
	return nil
}

// ListSessions returns all active debug sessions.
func (t *TimeTravelDebugger) ListSessions() []*DebugSession {
	t.mu.RLock()
	defer t.mu.RUnlock()

	sessions := make([]*DebugSession, 0, len(t.sessions))
	for _, s := range t.sessions {
		sessions = append(sessions, s)
	}

	// Sort by created time
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.After(sessions[j].CreatedAt)
	})

	return sessions
}

// ExportSession exports a debug session for sharing/analysis.
func (t *TimeTravelDebugger) ExportSession(sessionID string) ([]byte, error) {
	session, err := t.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	session.mu.RLock()
	defer session.mu.RUnlock()

	return json.MarshalIndent(session, "", "  ")
}
