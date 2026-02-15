package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/chronos/chronos/internal/models"
	"github.com/rs/zerolog"
)

func TestDependencyResolver_RegisterJob(t *testing.T) {
	dr := NewDependencyResolver(nil, testLogger())

	// Job with no dependencies is fine
	err := dr.RegisterJob(&models.Job{ID: "job-a"})
	if err != nil {
		t.Fatalf("unexpected error for job without deps: %v", err)
	}

	// Job with dependencies
	err = dr.RegisterJob(&models.Job{
		ID: "job-b",
		Dependencies: &models.DependencyConfig{
			DependsOn: []string{"job-a"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deps := dr.GetDependencies("job-b")
	if len(deps) != 1 || deps[0] != "job-a" {
		t.Errorf("expected deps [job-a], got %v", deps)
	}

	dependents := dr.GetDependents("job-a")
	if len(dependents) != 1 || dependents[0] != "job-b" {
		t.Errorf("expected dependents [job-b], got %v", dependents)
	}
}

func TestDependencyResolver_CycleDetection(t *testing.T) {
	dr := NewDependencyResolver(nil, testLogger())

	dr.RegisterJob(&models.Job{
		ID: "job-a",
		Dependencies: &models.DependencyConfig{
			DependsOn: []string{"job-b"},
		},
	})

	// This should create a cycle: b -> a -> b
	err := dr.RegisterJob(&models.Job{
		ID: "job-b",
		Dependencies: &models.DependencyConfig{
			DependsOn: []string{"job-a"},
		},
	})
	if err == nil {
		t.Fatal("expected cycle detection error")
	}
}

func TestDependencyResolver_UnregisterJob(t *testing.T) {
	dr := NewDependencyResolver(nil, testLogger())

	dr.RegisterJob(&models.Job{
		ID: "job-b",
		Dependencies: &models.DependencyConfig{
			DependsOn: []string{"job-a"},
		},
	})

	dr.UnregisterJob("job-b")

	deps := dr.GetDependencies("job-b")
	if len(deps) != 0 {
		t.Errorf("expected no deps after unregister, got %v", deps)
	}

	dependents := dr.GetDependents("job-a")
	if len(dependents) != 0 {
		t.Errorf("expected no dependents after unregister, got %v", dependents)
	}
}

func TestDependencyResolver_TopologicalOrder(t *testing.T) {
	dr := NewDependencyResolver(nil, testLogger())

	// Create: C depends on A and B
	dr.RegisterJob(&models.Job{
		ID: "job-c",
		Dependencies: &models.DependencyConfig{
			DependsOn: []string{"job-a", "job-b"},
		},
	})

	order, err := dr.TopologicalOrder([]string{"job-a", "job-b", "job-c"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// job-a and job-b should come before job-c
	if len(order) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(order))
	}
	if order[2] != "job-c" {
		t.Errorf("expected job-c last, got %s", order[2])
	}
}

func TestDependencyResolver_FanOut(t *testing.T) {
	dr := NewDependencyResolver(nil, testLogger())

	jobs := []*models.Job{
		{ID: "job-1", Name: "Job 1"},
		{ID: "job-2", Name: "Job 2"},
		{ID: "job-3", Name: "Job 3"},
	}

	mock := &mockDispatcher{
		results: map[string]*models.Execution{
			"job-1": {ID: "exec-1", Status: models.ExecutionSuccess},
			"job-2": {ID: "exec-2", Status: models.ExecutionSuccess},
			"job-3": {ID: "exec-3", Status: models.ExecutionFailed},
		},
	}

	results := dr.ExecuteFanOut(context.Background(), jobs, mock, time.Now())

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	successCount := 0
	for _, r := range results {
		if r.Execution != nil && r.Execution.Status == models.ExecutionSuccess {
			successCount++
		}
	}
	if successCount != 2 {
		t.Errorf("expected 2 successes, got %d", successCount)
	}
}

func TestDAGRunManager_ExecuteDAG(t *testing.T) {
	dr := NewDependencyResolver(nil, testLogger())

	// Register: C depends on A and B
	dr.RegisterJob(&models.Job{
		ID: "job-c",
		Dependencies: &models.DependencyConfig{
			DependsOn: []string{"job-a", "job-b"},
		},
	})

	mock := &mockDispatcher{
		results: map[string]*models.Execution{
			"job-a": {ID: "exec-a", Status: models.ExecutionSuccess},
			"job-b": {ID: "exec-b", Status: models.ExecutionSuccess},
			"job-c": {ID: "exec-c", Status: models.ExecutionSuccess},
		},
	}

	dm := NewDAGRunManager(dr, mock, nil, testLogger())

	jobs := []*models.Job{
		{ID: "job-a", Name: "A"},
		{ID: "job-b", Name: "B"},
		{ID: "job-c", Name: "C"},
	}

	dagRun, err := dm.ExecuteDAG(context.Background(), jobs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if dagRun.Status != models.DAGRunSuccess {
		t.Errorf("expected DAG success, got %s", dagRun.Status)
	}

	for _, state := range dagRun.JobStates {
		if state != string(models.ExecutionSuccess) {
			t.Errorf("expected all jobs success, got %s", state)
		}
	}
}

func TestDAGRunManager_ExecuteDAG_Failure(t *testing.T) {
	dr := NewDependencyResolver(nil, testLogger())

	dr.RegisterJob(&models.Job{
		ID: "job-b",
		Dependencies: &models.DependencyConfig{
			DependsOn: []string{"job-a"},
		},
	})

	mock := &mockDispatcher{
		results: map[string]*models.Execution{
			"job-a": {ID: "exec-a", Status: models.ExecutionFailed},
			"job-b": {ID: "exec-b", Status: models.ExecutionSuccess},
		},
	}

	dm := NewDAGRunManager(dr, mock, nil, testLogger())

	jobs := []*models.Job{
		{ID: "job-a", Name: "A"},
		{ID: "job-b", Name: "B"},
	}

	dagRun, _ := dm.ExecuteDAG(context.Background(), jobs)

	if dagRun.Status != models.DAGRunFailed {
		t.Errorf("expected DAG failed, got %s", dagRun.Status)
	}
}

// mockDispatcher implements JobDispatcher for testing.
type mockDispatcher struct {
	results map[string]*models.Execution
}

func (m *mockDispatcher) Execute(_ context.Context, job *models.Job, _ time.Time) (*models.Execution, error) {
	if exec, ok := m.results[job.ID]; ok {
		return exec, nil
	}
	return &models.Execution{Status: models.ExecutionSuccess}, nil
}

func testLogger() zerolog.Logger {
	return zerolog.Nop()
}
