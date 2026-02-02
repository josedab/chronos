// Package dag provides DAG storage interfaces and implementations.
package dag

import (
	"context"
	"errors"
	"time"
)

// Common storage errors.
var (
	ErrWorkflowNotFound = errors.New("workflow not found")
	ErrRunNotFound      = errors.New("run not found")
	ErrVersionConflict  = errors.New("version conflict")
)

// Store defines the interface for DAG/workflow persistence.
type Store interface {
	// Workflow operations
	CreateWorkflow(ctx context.Context, workflow *Workflow) error
	GetWorkflow(ctx context.Context, id string) (*Workflow, error)
	UpdateWorkflow(ctx context.Context, workflow *Workflow) error
	DeleteWorkflow(ctx context.Context, id string) error
	ListWorkflows(ctx context.Context, opts ListOptions) ([]*Workflow, int, error)

	// Run operations
	CreateRun(ctx context.Context, run *Run) error
	GetRun(ctx context.Context, id string) (*Run, error)
	UpdateRun(ctx context.Context, run *Run) error
	ListRuns(ctx context.Context, workflowID string, opts ListOptions) ([]*Run, int, error)
	ListRunsByStatus(ctx context.Context, status RunStatus, limit int) ([]*Run, error)

	// DAG operations (legacy support)
	SaveDAG(ctx context.Context, dag *DAG) error
	GetDAG(ctx context.Context, id string) (*DAG, error)
	DeleteDAG(ctx context.Context, id string) error
	ListDAGs(ctx context.Context, opts ListOptions) ([]*DAG, int, error)
}

// ListOptions contains pagination and filtering options.
type ListOptions struct {
	Offset    int
	Limit     int
	SortBy    string
	SortOrder string // "asc" or "desc"
	Filters   map[string]string
}

// DefaultListOptions returns default list options.
func DefaultListOptions() ListOptions {
	return ListOptions{
		Offset:    0,
		Limit:     50,
		SortBy:    "created_at",
		SortOrder: "desc",
	}
}

// MemoryStore provides an in-memory implementation of Store.
type MemoryStore struct {
	workflows map[string]*Workflow
	runs      map[string]*Run
	dags      map[string]*DAG
}

// NewMemoryStore creates a new in-memory DAG store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		workflows: make(map[string]*Workflow),
		runs:      make(map[string]*Run),
		dags:      make(map[string]*DAG),
	}
}

func (s *MemoryStore) CreateWorkflow(ctx context.Context, workflow *Workflow) error {
	if _, exists := s.workflows[workflow.ID]; exists {
		return errors.New("workflow already exists")
	}
	workflow.CreatedAt = time.Now().UTC()
	workflow.UpdatedAt = workflow.CreatedAt
	s.workflows[workflow.ID] = workflow
	return nil
}

func (s *MemoryStore) GetWorkflow(ctx context.Context, id string) (*Workflow, error) {
	workflow, exists := s.workflows[id]
	if !exists {
		return nil, ErrWorkflowNotFound
	}
	return workflow, nil
}

func (s *MemoryStore) UpdateWorkflow(ctx context.Context, workflow *Workflow) error {
	existing, exists := s.workflows[workflow.ID]
	if !exists {
		return ErrWorkflowNotFound
	}
	if existing.Version != workflow.Version-1 {
		return ErrVersionConflict
	}
	workflow.UpdatedAt = time.Now().UTC()
	s.workflows[workflow.ID] = workflow
	return nil
}

func (s *MemoryStore) DeleteWorkflow(ctx context.Context, id string) error {
	if _, exists := s.workflows[id]; !exists {
		return ErrWorkflowNotFound
	}
	delete(s.workflows, id)
	return nil
}

func (s *MemoryStore) ListWorkflows(ctx context.Context, opts ListOptions) ([]*Workflow, int, error) {
	workflows := make([]*Workflow, 0, len(s.workflows))
	for _, w := range s.workflows {
		workflows = append(workflows, w)
	}

	total := len(workflows)

	// Apply pagination
	start := opts.Offset
	if start > len(workflows) {
		return []*Workflow{}, total, nil
	}
	end := start + opts.Limit
	if end > len(workflows) {
		end = len(workflows)
	}

	return workflows[start:end], total, nil
}

func (s *MemoryStore) CreateRun(ctx context.Context, run *Run) error {
	if _, exists := s.runs[run.ID]; exists {
		return errors.New("run already exists")
	}
	s.runs[run.ID] = run
	return nil
}

func (s *MemoryStore) GetRun(ctx context.Context, id string) (*Run, error) {
	run, exists := s.runs[id]
	if !exists {
		return nil, ErrRunNotFound
	}
	return run, nil
}

func (s *MemoryStore) UpdateRun(ctx context.Context, run *Run) error {
	if _, exists := s.runs[run.ID]; !exists {
		return ErrRunNotFound
	}
	s.runs[run.ID] = run
	return nil
}

func (s *MemoryStore) ListRuns(ctx context.Context, workflowID string, opts ListOptions) ([]*Run, int, error) {
	runs := make([]*Run, 0)
	for _, r := range s.runs {
		if r.DAGID == workflowID {
			runs = append(runs, r)
		}
	}

	total := len(runs)
	start := opts.Offset
	if start > len(runs) {
		return []*Run{}, total, nil
	}
	end := start + opts.Limit
	if end > len(runs) {
		end = len(runs)
	}

	return runs[start:end], total, nil
}

func (s *MemoryStore) ListRunsByStatus(ctx context.Context, status RunStatus, limit int) ([]*Run, error) {
	runs := make([]*Run, 0)
	for _, r := range s.runs {
		if r.GetStatus() == status {
			runs = append(runs, r)
			if len(runs) >= limit {
				break
			}
		}
	}
	return runs, nil
}

func (s *MemoryStore) SaveDAG(ctx context.Context, dag *DAG) error {
	s.dags[dag.ID] = dag
	return nil
}

func (s *MemoryStore) GetDAG(ctx context.Context, id string) (*DAG, error) {
	dag, exists := s.dags[id]
	if !exists {
		return nil, ErrDAGNotFound
	}
	return dag, nil
}

func (s *MemoryStore) DeleteDAG(ctx context.Context, id string) error {
	if _, exists := s.dags[id]; !exists {
		return ErrDAGNotFound
	}
	delete(s.dags, id)
	return nil
}

func (s *MemoryStore) ListDAGs(ctx context.Context, opts ListOptions) ([]*DAG, int, error) {
	dags := make([]*DAG, 0, len(s.dags))
	for _, d := range s.dags {
		dags = append(dags, d)
	}

	total := len(dags)
	start := opts.Offset
	if start > len(dags) {
		return []*DAG{}, total, nil
	}
	end := start + opts.Limit
	if end > len(dags) {
		end = len(dags)
	}

	return dags[start:end], total, nil
}
