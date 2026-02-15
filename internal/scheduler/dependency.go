// Package scheduler provides job dependency resolution and fan-out/fan-in execution.
package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chronos/chronos/internal/models"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// DependencyResolver manages inter-job dependencies and DAG-based execution.
type DependencyResolver struct {
	store  SchedulerStore
	logger zerolog.Logger
	mu     sync.RWMutex

	// dependencyGraph: jobID -> set of jobs it depends on
	dependencyGraph map[string]map[string]bool
	// reverseDeps: jobID -> set of jobs that depend on it
	reverseDeps map[string]map[string]bool
}

// NewDependencyResolver creates a new DependencyResolver.
func NewDependencyResolver(store SchedulerStore, logger zerolog.Logger) *DependencyResolver {
	return &DependencyResolver{
		store:           store,
		logger:          logger.With().Str("component", "dependency_resolver").Logger(),
		dependencyGraph: make(map[string]map[string]bool),
		reverseDeps:     make(map[string]map[string]bool),
	}
}

// RegisterJob registers a job and its dependencies in the graph.
func (dr *DependencyResolver) RegisterJob(job *models.Job) error {
	if job.Dependencies == nil || len(job.Dependencies.DependsOn) == 0 {
		return nil
	}

	dr.mu.Lock()
	defer dr.mu.Unlock()

	deps := make(map[string]bool)
	for _, depID := range job.Dependencies.DependsOn {
		deps[depID] = true

		// Build reverse index
		if dr.reverseDeps[depID] == nil {
			dr.reverseDeps[depID] = make(map[string]bool)
		}
		dr.reverseDeps[depID][job.ID] = true
	}
	dr.dependencyGraph[job.ID] = deps

	// Check for cycles
	if dr.hasCycle(job.ID, make(map[string]bool), make(map[string]bool)) {
		delete(dr.dependencyGraph, job.ID)
		for _, depID := range job.Dependencies.DependsOn {
			delete(dr.reverseDeps[depID], job.ID)
		}
		return fmt.Errorf("adding job %s would create a dependency cycle", job.ID)
	}

	return nil
}

// UnregisterJob removes a job from the dependency graph.
func (dr *DependencyResolver) UnregisterJob(jobID string) {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	// Remove from dependency graph
	if deps, ok := dr.dependencyGraph[jobID]; ok {
		for depID := range deps {
			if dr.reverseDeps[depID] != nil {
				delete(dr.reverseDeps[depID], jobID)
			}
		}
		delete(dr.dependencyGraph, jobID)
	}

	// Remove from reverse deps
	delete(dr.reverseDeps, jobID)
}

// GetDependents returns jobs that depend on the given job.
func (dr *DependencyResolver) GetDependents(jobID string) []string {
	dr.mu.RLock()
	defer dr.mu.RUnlock()

	deps, ok := dr.reverseDeps[jobID]
	if !ok {
		return nil
	}

	result := make([]string, 0, len(deps))
	for id := range deps {
		result = append(result, id)
	}
	return result
}

// GetDependencies returns jobs that the given job depends on.
func (dr *DependencyResolver) GetDependencies(jobID string) []string {
	dr.mu.RLock()
	defer dr.mu.RUnlock()

	deps, ok := dr.dependencyGraph[jobID]
	if !ok {
		return nil
	}

	result := make([]string, 0, len(deps))
	for id := range deps {
		result = append(result, id)
	}
	return result
}

// AreDependenciesMet checks if all dependencies for a job are satisfied.
func (dr *DependencyResolver) AreDependenciesMet(ctx context.Context, jobID string, since time.Time) (bool, error) {
	dr.mu.RLock()
	deps, ok := dr.dependencyGraph[jobID]
	dr.mu.RUnlock()

	if !ok || len(deps) == 0 {
		return true, nil
	}

	// Look up the job to get the dependency condition
	condition := models.DependencyConditionAllSuccess // default

	for depID := range deps {
		executions, err := dr.store.ListExecutions(depID, 10)
		if err != nil {
			return false, fmt.Errorf("checking dependency %s: %w", depID, err)
		}

		// Find most recent execution after 'since'
		hasRecent := false
		hasSuccess := false
		for _, exec := range executions {
			if exec.StartedAt.After(since) || exec.StartedAt.Equal(since) {
				hasRecent = true
				if exec.Status == models.ExecutionSuccess {
					hasSuccess = true
				}
				break
			}
		}

		switch condition {
		case models.DependencyConditionAllSuccess:
			if !hasSuccess {
				return false, nil
			}
		case models.DependencyConditionAllComplete:
			if !hasRecent {
				return false, nil
			}
		case models.DependencyConditionAnySuccess:
			if hasSuccess {
				return true, nil
			}
		case models.DependencyConditionAnyComplete:
			if hasRecent {
				return true, nil
			}
		}
	}

	// For "any" conditions, if we reach here no dependency was met
	if condition == models.DependencyConditionAnySuccess || condition == models.DependencyConditionAnyComplete {
		return false, nil
	}

	return true, nil
}

// hasCycle detects cycles using DFS. Must be called with mu held.
func (dr *DependencyResolver) hasCycle(nodeID string, visited, inStack map[string]bool) bool {
	visited[nodeID] = true
	inStack[nodeID] = true

	if deps, ok := dr.dependencyGraph[nodeID]; ok {
		for depID := range deps {
			if !visited[depID] {
				if dr.hasCycle(depID, visited, inStack) {
					return true
				}
			} else if inStack[depID] {
				return true
			}
		}
	}

	inStack[nodeID] = false
	return false
}

// TopologicalOrder returns jobs in dependency order (leaves first).
func (dr *DependencyResolver) TopologicalOrder(jobIDs []string) ([]string, error) {
	dr.mu.RLock()
	defer dr.mu.RUnlock()

	// Filter to only requested jobs
	jobSet := make(map[string]bool)
	for _, id := range jobIDs {
		jobSet[id] = true
	}

	inDegree := make(map[string]int)
	for _, id := range jobIDs {
		inDegree[id] = 0
	}

	// Count in-degrees within the job set
	for _, id := range jobIDs {
		if deps, ok := dr.dependencyGraph[id]; ok {
			for depID := range deps {
				if jobSet[depID] {
					inDegree[id]++
				}
			}
		}
	}

	// Kahn's algorithm
	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	var order []string
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		order = append(order, current)

		if dependents, ok := dr.reverseDeps[current]; ok {
			for depID := range dependents {
				if jobSet[depID] {
					inDegree[depID]--
					if inDegree[depID] == 0 {
						queue = append(queue, depID)
					}
				}
			}
		}
	}

	if len(order) != len(jobIDs) {
		return nil, fmt.Errorf("dependency cycle detected among jobs")
	}

	return order, nil
}

// FanOutResult holds results from parallel fan-out execution.
type FanOutResult struct {
	JobID     string
	Execution *models.Execution
	Error     error
}

// ExecuteFanOut runs multiple independent jobs in parallel and collects results.
func (dr *DependencyResolver) ExecuteFanOut(ctx context.Context, jobs []*models.Job, dispatcher JobDispatcher, scheduledTime time.Time) []FanOutResult {
	results := make([]FanOutResult, len(jobs))
	var wg sync.WaitGroup

	for i, job := range jobs {
		wg.Add(1)
		go func(idx int, j *models.Job) {
			defer wg.Done()
			exec, err := dispatcher.Execute(ctx, j, scheduledTime)
			results[idx] = FanOutResult{
				JobID:     j.ID,
				Execution: exec,
				Error:     err,
			}
		}(i, job)
	}

	wg.Wait()
	return results
}

// ExecuteFanIn waits for all specified job executions to complete.
// Returns true if all conditions are met, false otherwise.
func (dr *DependencyResolver) ExecuteFanIn(ctx context.Context, jobIDs []string, condition models.DependencyCondition, since time.Time) (bool, error) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-ticker.C:
			allMet := true
			anyMet := false

			for _, jobID := range jobIDs {
				executions, err := dr.store.ListExecutions(jobID, 10)
				if err != nil {
					return false, fmt.Errorf("checking job %s: %w", jobID, err)
				}

				met := false
				for _, exec := range executions {
					if exec.StartedAt.Before(since) {
						continue
					}
					switch condition {
					case models.DependencyConditionAllSuccess, models.DependencyConditionAnySuccess:
						met = exec.Status == models.ExecutionSuccess
					case models.DependencyConditionAllComplete, models.DependencyConditionAnyComplete:
						met = exec.Status == models.ExecutionSuccess || exec.Status == models.ExecutionFailed
					}
					if met {
						break
					}
				}

				if met {
					anyMet = true
				} else {
					allMet = false
				}
			}

			switch condition {
			case models.DependencyConditionAllSuccess, models.DependencyConditionAllComplete:
				if allMet {
					return true, nil
				}
			case models.DependencyConditionAnySuccess, models.DependencyConditionAnyComplete:
				if anyMet {
					return true, nil
				}
			}
		}
	}
}

// DAGRunManager orchestrates DAG-based job execution.
type DAGRunManager struct {
	resolver   *DependencyResolver
	dispatcher JobDispatcher
	store      SchedulerStore
	logger     zerolog.Logger
}

// JobDispatcher is the interface for executing jobs.
type JobDispatcher interface {
	Execute(ctx context.Context, job *models.Job, scheduledTime time.Time) (*models.Execution, error)
}

// NewDAGRunManager creates a new DAG run manager.
func NewDAGRunManager(resolver *DependencyResolver, dispatcher JobDispatcher, store SchedulerStore, logger zerolog.Logger) *DAGRunManager {
	return &DAGRunManager{
		resolver:   resolver,
		dispatcher: dispatcher,
		store:      store,
		logger:     logger.With().Str("component", "dag_run_manager").Logger(),
	}
}

// ExecuteDAG runs a set of jobs respecting their dependency order.
func (dm *DAGRunManager) ExecuteDAG(ctx context.Context, jobs []*models.Job) (*models.DAGRun, error) {
	jobIDs := make([]string, len(jobs))
	jobMap := make(map[string]*models.Job)
	for i, j := range jobs {
		jobIDs[i] = j.ID
		jobMap[j.ID] = j
	}

	order, err := dm.resolver.TopologicalOrder(jobIDs)
	if err != nil {
		return nil, fmt.Errorf("resolving execution order: %w", err)
	}

	dagRun := &models.DAGRun{
		ID:        uuid.New().String(),
		Jobs:      jobIDs,
		Status:    models.DAGRunRunning,
		StartedAt: time.Now(),
		JobStates: make(map[string]string),
	}

	for _, id := range jobIDs {
		dagRun.JobStates[id] = string(models.ExecutionPending)
	}

	scheduledTime := time.Now()

	// Execute in topological order, parallelizing where possible
	executed := make(map[string]bool)
	for len(executed) < len(order) {
		// Find all ready jobs (dependencies satisfied)
		var ready []*models.Job
		for _, id := range order {
			if executed[id] {
				continue
			}
			deps := dm.resolver.GetDependencies(id)
			allDone := true
			for _, depID := range deps {
				if !executed[depID] {
					allDone = false
					break
				}
			}
			if allDone {
				ready = append(ready, jobMap[id])
			}
		}

		if len(ready) == 0 {
			dagRun.Status = models.DAGRunFailed
			dagRun.EndedAt = time.Now()
			return dagRun, fmt.Errorf("deadlock: no jobs are ready to execute")
		}

		// Fan-out: execute ready jobs in parallel
		results := dm.resolver.ExecuteFanOut(ctx, ready, dm.dispatcher, scheduledTime)

		for _, result := range results {
			executed[result.JobID] = true
			if result.Error != nil || (result.Execution != nil && result.Execution.Status == models.ExecutionFailed) {
				dagRun.JobStates[result.JobID] = string(models.ExecutionFailed)
				dm.logger.Error().
					Str("job_id", result.JobID).
					Err(result.Error).
					Msg("DAG job failed")
			} else {
				dagRun.JobStates[result.JobID] = string(models.ExecutionSuccess)
			}
		}

		if ctx.Err() != nil {
			dagRun.Status = models.DAGRunCancelled
			dagRun.EndedAt = time.Now()
			return dagRun, ctx.Err()
		}
	}

	// Determine final status
	dagRun.Status = models.DAGRunSuccess
	for _, state := range dagRun.JobStates {
		if state == string(models.ExecutionFailed) {
			dagRun.Status = models.DAGRunFailed
			break
		}
	}
	dagRun.EndedAt = time.Now()

	return dagRun, nil
}
