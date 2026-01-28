// Package dag provides directed acyclic graph execution for Chronos workflows.
package dag

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Common errors.
var (
	ErrCycleDetected  = errors.New("cycle detected in DAG")
	ErrNodeNotFound   = errors.New("node not found")
	ErrDAGNotFound    = errors.New("DAG not found")
	ErrDAGRunning     = errors.New("DAG is already running")
	ErrInvalidDAG     = errors.New("invalid DAG configuration")
)

// NodeStatus represents the execution status of a node.
type NodeStatus string

const (
	NodePending   NodeStatus = "pending"
	NodeReady     NodeStatus = "ready"
	NodeRunning   NodeStatus = "running"
	NodeSuccess   NodeStatus = "success"
	NodeFailed    NodeStatus = "failed"
	NodeSkipped   NodeStatus = "skipped"
	NodeCancelled NodeStatus = "cancelled"
)

// Node represents a node in the DAG.
type Node struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	JobID        string            `json:"job_id"`
	Dependencies []string          `json:"dependencies"`
	Condition    TriggerCondition  `json:"condition"`
	Timeout      time.Duration     `json:"timeout"`
	RetryCount   int               `json:"retry_count"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// TriggerCondition determines when a node should run.
type TriggerCondition string

const (
	// ConditionAllSuccess runs when all dependencies succeed.
	ConditionAllSuccess TriggerCondition = "all_success"
	// ConditionAllComplete runs when all dependencies complete (success or fail).
	ConditionAllComplete TriggerCondition = "all_complete"
	// ConditionAnySuccess runs when any dependency succeeds.
	ConditionAnySuccess TriggerCondition = "any_success"
	// ConditionNone runs immediately with no conditions.
	ConditionNone TriggerCondition = "none"
)

// DAG represents a directed acyclic graph of jobs.
type DAG struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Nodes       map[string]*Node  `json:"nodes"`
	RootNodes   []string          `json:"root_nodes"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// RunStatus represents the status of a DAG run.
type RunStatus string

const (
	RunPending   RunStatus = "pending"
	RunRunning   RunStatus = "running"
	RunSuccess   RunStatus = "success"
	RunFailed    RunStatus = "failed"
	RunCancelled RunStatus = "cancelled"
)

// Run represents a single execution of a DAG.
type Run struct {
	mu         sync.RWMutex          `json:"-"`
	ID         string                `json:"id"`
	DAGID      string                `json:"dag_id"`
	DAGName    string                `json:"dag_name"`
	Status     RunStatus             `json:"status"`
	NodeStates map[string]*NodeState `json:"node_states"`
	StartedAt  time.Time             `json:"started_at"`
	EndedAt    time.Time             `json:"ended_at,omitempty"`
	Error      string                `json:"error,omitempty"`
}

// GetStatus returns the run status safely.
func (r *Run) GetStatus() RunStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Status
}

// SetStatus sets the run status safely.
func (r *Run) SetStatus(s RunStatus) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Status = s
}

// SetError sets the error safely.
func (r *Run) SetError(err string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Error = err
}

// SetEndedAt sets the ended time safely.
func (r *Run) SetEndedAt(t time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.EndedAt = t
}

// NodeState represents the state of a node during a run.
type NodeState struct {
	mu          sync.Mutex    `json:"-"`
	NodeID      string        `json:"node_id"`
	Status      NodeStatus    `json:"status"`
	ExecutionID string        `json:"execution_id,omitempty"`
	StartedAt   time.Time     `json:"started_at,omitempty"`
	EndedAt     time.Time     `json:"ended_at,omitempty"`
	Duration    time.Duration `json:"duration,omitempty"`
	Attempts    int           `json:"attempts"`
	Error       string        `json:"error,omitempty"`
}

// SetRunning marks the node as running and increments attempts.
func (s *NodeState) SetRunning() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = NodeRunning
	s.StartedAt = time.Now().UTC()
	s.Attempts++
}

// SetCompleted marks the node as completed with the given status.
func (s *NodeState) SetCompleted(status NodeStatus, execID string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = status
	s.ExecutionID = execID
	s.EndedAt = time.Now().UTC()
	s.Duration = s.EndedAt.Sub(s.StartedAt)
	if err != nil {
		s.Error = err.Error()
	}
}

// GetStatus returns the node status safely.
func (s *NodeState) GetStatus() NodeStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Status
}

// NewDAG creates a new DAG.
func NewDAG(id, name string) *DAG {
	now := time.Now().UTC()
	return &DAG{
		ID:        id,
		Name:      name,
		Nodes:     make(map[string]*Node),
		RootNodes: []string{},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// AddNode adds a node to the DAG.
func (d *DAG) AddNode(node *Node) error {
	if node.ID == "" {
		return fmt.Errorf("node ID is required")
	}
	if _, exists := d.Nodes[node.ID]; exists {
		return fmt.Errorf("node %s already exists", node.ID)
	}

	if node.Condition == "" {
		node.Condition = ConditionAllSuccess
	}

	d.Nodes[node.ID] = node
	d.UpdatedAt = time.Now().UTC()

	// Recalculate root nodes
	d.calculateRootNodes()

	return nil
}

// RemoveNode removes a node from the DAG.
func (d *DAG) RemoveNode(nodeID string) error {
	if _, exists := d.Nodes[nodeID]; !exists {
		return ErrNodeNotFound
	}

	delete(d.Nodes, nodeID)
	d.UpdatedAt = time.Now().UTC()
	d.calculateRootNodes()

	return nil
}

// calculateRootNodes finds nodes with no dependencies.
func (d *DAG) calculateRootNodes() {
	d.RootNodes = []string{}
	for id, node := range d.Nodes {
		if len(node.Dependencies) == 0 {
			d.RootNodes = append(d.RootNodes, id)
		}
	}
}

// Validate checks if the DAG is valid (no cycles, all deps exist).
func (d *DAG) Validate() error {
	if len(d.Nodes) == 0 {
		return ErrInvalidDAG
	}

	// Check all dependencies exist
	for _, node := range d.Nodes {
		for _, depID := range node.Dependencies {
			if _, exists := d.Nodes[depID]; !exists {
				return fmt.Errorf("dependency %s not found for node %s", depID, node.ID)
			}
		}
	}

	// Check for cycles using DFS
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var hasCycle func(nodeID string) bool
	hasCycle = func(nodeID string) bool {
		visited[nodeID] = true
		recStack[nodeID] = true

		node := d.Nodes[nodeID]
		for _, depID := range node.Dependencies {
			if !visited[depID] {
				if hasCycle(depID) {
					return true
				}
			} else if recStack[depID] {
				return true
			}
		}

		recStack[nodeID] = false
		return false
	}

	for nodeID := range d.Nodes {
		if !visited[nodeID] {
			if hasCycle(nodeID) {
				return ErrCycleDetected
			}
		}
	}

	return nil
}

// TopologicalSort returns nodes in execution order.
func (d *DAG) TopologicalSort() ([]string, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}

	visited := make(map[string]bool)
	var result []string

	var visit func(nodeID string)
	visit = func(nodeID string) {
		if visited[nodeID] {
			return
		}
		visited[nodeID] = true

		node := d.Nodes[nodeID]
		for _, depID := range node.Dependencies {
			visit(depID)
		}

		result = append(result, nodeID)
	}

	for nodeID := range d.Nodes {
		visit(nodeID)
	}

	return result, nil
}

// GetExecutableNodes returns nodes that are ready to execute.
func (d *DAG) GetExecutableNodes(nodeStates map[string]*NodeState) []string {
	var ready []string

	for nodeID, node := range d.Nodes {
		state := nodeStates[nodeID]
		if state == nil || state.GetStatus() != NodePending {
			continue
		}

		// Check if all dependencies are satisfied
		canRun := true
		for _, depID := range node.Dependencies {
			depState := nodeStates[depID]
			if depState == nil {
				canRun = false
				break
			}

			depStatus := depState.GetStatus()
			switch node.Condition {
			case ConditionAllSuccess:
				if depStatus != NodeSuccess {
					canRun = false
				}
			case ConditionAllComplete:
				if depStatus != NodeSuccess && depStatus != NodeFailed {
					canRun = false
				}
			case ConditionAnySuccess:
				// At least one success is enough
				hasSuccess := false
				for _, dID := range node.Dependencies {
					if s := nodeStates[dID]; s != nil && s.GetStatus() == NodeSuccess {
						hasSuccess = true
						break
					}
				}
				canRun = hasSuccess
			case ConditionNone:
				canRun = true
			}

			if !canRun {
				break
			}
		}

		if canRun {
			ready = append(ready, nodeID)
		}
	}

	return ready
}

// Engine executes DAGs.
type Engine struct {
	mu       sync.RWMutex
	dags     map[string]*DAG
	runs     map[string]*Run
	executor NodeExecutor
}

// NodeExecutor executes individual nodes.
type NodeExecutor interface {
	ExecuteNode(ctx context.Context, node *Node) (string, error)
}

// NewEngine creates a new DAG engine.
func NewEngine(executor NodeExecutor) *Engine {
	return &Engine{
		dags:     make(map[string]*DAG),
		runs:     make(map[string]*Run),
		executor: executor,
	}
}

// RegisterDAG registers a DAG with the engine.
func (e *Engine) RegisterDAG(dag *DAG) error {
	if err := dag.Validate(); err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.dags[dag.ID] = dag
	return nil
}

// GetDAG returns a DAG by ID.
func (e *Engine) GetDAG(id string) (*DAG, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	dag, exists := e.dags[id]
	if !exists {
		return nil, ErrDAGNotFound
	}
	return dag, nil
}

// ListDAGs returns all registered DAGs.
func (e *Engine) ListDAGs() []*DAG {
	e.mu.RLock()
	defer e.mu.RUnlock()

	dags := make([]*DAG, 0, len(e.dags))
	for _, dag := range e.dags {
		dags = append(dags, dag)
	}
	return dags
}

// StartRun starts a new DAG run.
func (e *Engine) StartRun(ctx context.Context, dagID, runID string) (*Run, error) {
	dag, err := e.GetDAG(dagID)
	if err != nil {
		return nil, err
	}

	run := &Run{
		ID:         runID,
		DAGID:      dag.ID,
		DAGName:    dag.Name,
		Status:     RunRunning,
		NodeStates: make(map[string]*NodeState),
		StartedAt:  time.Now().UTC(),
	}

	// Initialize all node states as pending
	for nodeID := range dag.Nodes {
		run.NodeStates[nodeID] = &NodeState{
			NodeID: nodeID,
			Status: NodePending,
		}
	}

	e.mu.Lock()
	e.runs[runID] = run
	e.mu.Unlock()

	// Execute the DAG asynchronously
	go e.executeRun(ctx, dag, run)

	return run, nil
}

// executeRun executes a DAG run.
func (e *Engine) executeRun(ctx context.Context, dag *DAG, run *Run) {
	defer func() {
		run.SetEndedAt(time.Now().UTC())
		if run.GetStatus() == RunRunning {
			run.SetStatus(RunSuccess)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			run.SetStatus(RunCancelled)
			run.SetError(ctx.Err().Error())
			return
		default:
		}

		// Get nodes ready to execute
		ready := dag.GetExecutableNodes(run.NodeStates)
		if len(ready) == 0 {
			// Check if all nodes are complete
			allComplete := true
			for _, state := range run.NodeStates {
				status := state.GetStatus()
				if status == NodePending || status == NodeRunning {
					allComplete = false
					break
				}
			}
			if allComplete {
				return
			}
			// Wait for running nodes
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Execute ready nodes in parallel
		var wg sync.WaitGroup
		for _, nodeID := range ready {
			wg.Add(1)
			go func(nID string) {
				defer wg.Done()
				e.executeNode(ctx, dag, run, nID)
			}(nodeID)
		}
		wg.Wait()
	}
}

// executeNode executes a single node.
func (e *Engine) executeNode(ctx context.Context, dag *DAG, run *Run, nodeID string) {
	node := dag.Nodes[nodeID]
	state := run.NodeStates[nodeID]

	// Mark node as running (thread-safe)
	state.SetRunning()

	// Create node context with timeout
	nodeCtx := ctx
	if node.Timeout > 0 {
		var cancel context.CancelFunc
		nodeCtx, cancel = context.WithTimeout(ctx, node.Timeout)
		defer cancel()
	}

	// Execute the node
	execID, err := e.executor.ExecuteNode(nodeCtx, node)

	// Mark node as completed (thread-safe)
	if err != nil {
		state.SetCompleted(NodeFailed, execID, err)
		run.SetStatus(RunFailed)
		run.SetError(fmt.Sprintf("node %s failed: %v", nodeID, err))
	} else {
		state.SetCompleted(NodeSuccess, execID, nil)
	}
}

// GetRun returns a run by ID.
func (e *Engine) GetRun(runID string) (*Run, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	run, exists := e.runs[runID]
	if !exists {
		return nil, fmt.Errorf("run %s not found", runID)
	}
	return run, nil
}

// ListRuns returns all runs for a DAG.
func (e *Engine) ListRuns(dagID string) []*Run {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var runs []*Run
	for _, run := range e.runs {
		if run.DAGID == dagID {
			runs = append(runs, run)
		}
	}
	return runs
}

// CancelRun cancels a running DAG run.
func (e *Engine) CancelRun(runID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	run, exists := e.runs[runID]
	if !exists {
		return fmt.Errorf("run %s not found", runID)
	}

	if run.GetStatus() != RunRunning {
		return fmt.Errorf("run is not running (status: %s)", run.GetStatus())
	}

	run.SetStatus(RunCancelled)
	run.SetEndedAt(time.Now().UTC())
	return nil
}
