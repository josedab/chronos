package chaos

import (
	"context"
	"testing"
	"time"
)

func TestNewEngine(t *testing.T) {
	e := NewEngine()
	if e == nil {
		t.Fatal("expected non-nil engine")
	}
	if e.IsEnabled() {
		t.Error("engine should be disabled by default")
	}
}

func TestEnableDisable(t *testing.T) {
	e := NewEngine()

	e.Enable()
	if !e.IsEnabled() {
		t.Error("engine should be enabled")
	}

	e.Disable()
	if e.IsEnabled() {
		t.Error("engine should be disabled")
	}
}

func TestCreateExperiment(t *testing.T) {
	e := NewEngine()

	exp := &Experiment{
		ID:        "exp-1",
		Name:      "Test Experiment",
		FaultType: FaultLatency,
		Target:    Target{All: true},
		Config: FaultConfig{
			Latency: 100 * time.Millisecond,
		},
	}

	err := e.CreateExperiment(exp)
	if err != nil {
		t.Fatalf("CreateExperiment failed: %v", err)
	}

	// Duplicate should fail
	err = e.CreateExperiment(exp)
	if err != ErrExperimentExists {
		t.Errorf("expected ErrExperimentExists, got %v", err)
	}
}

func TestGetExperiment(t *testing.T) {
	e := NewEngine()

	e.CreateExperiment(&Experiment{
		ID:   "exp-1",
		Name: "Test",
	})

	exp, err := e.GetExperiment("exp-1")
	if err != nil {
		t.Fatalf("GetExperiment failed: %v", err)
	}
	if exp.Name != "Test" {
		t.Errorf("expected name 'Test', got %s", exp.Name)
	}

	_, err = e.GetExperiment("nonexistent")
	if err != ErrExperimentNotFound {
		t.Errorf("expected ErrExperimentNotFound, got %v", err)
	}
}

func TestListExperiments(t *testing.T) {
	e := NewEngine()

	e.CreateExperiment(&Experiment{ID: "exp-1"})
	e.CreateExperiment(&Experiment{ID: "exp-2"})
	e.CreateExperiment(&Experiment{ID: "exp-3"})

	exps := e.ListExperiments()
	if len(exps) != 3 {
		t.Errorf("expected 3 experiments, got %d", len(exps))
	}
}

func TestStartStopExperiment(t *testing.T) {
	e := NewEngine()

	e.CreateExperiment(&Experiment{
		ID: "exp-1",
		Config: FaultConfig{
			Duration: 1 * time.Second,
		},
	})

	ctx := context.Background()
	err := e.StartExperiment(ctx, "exp-1")
	if err != nil {
		t.Fatalf("StartExperiment failed: %v", err)
	}

	exp, _ := e.GetExperiment("exp-1")
	if exp.Status != StatusRunning {
		t.Errorf("expected status running, got %s", exp.Status)
	}

	err = e.StopExperiment("exp-1")
	if err != nil {
		t.Fatalf("StopExperiment failed: %v", err)
	}

	exp, _ = e.GetExperiment("exp-1")
	if exp.Status != StatusAborted {
		t.Errorf("expected status aborted, got %s", exp.Status)
	}
}

func TestDeleteExperiment(t *testing.T) {
	e := NewEngine()

	e.CreateExperiment(&Experiment{ID: "exp-1"})

	err := e.DeleteExperiment("exp-1")
	if err != nil {
		t.Fatalf("DeleteExperiment failed: %v", err)
	}

	_, err = e.GetExperiment("exp-1")
	if err != ErrExperimentNotFound {
		t.Error("experiment should be deleted")
	}
}

func TestShouldInjectFault(t *testing.T) {
	e := NewEngine()

	// Should not inject when disabled
	_, should := e.ShouldInjectFault("job-1")
	if should {
		t.Error("should not inject when disabled")
	}

	e.Enable()

	// Create and start experiment
	e.CreateExperiment(&Experiment{
		ID:        "exp-1",
		FaultType: FaultError,
		Target:    Target{JobIDs: []string{"job-1"}},
		Config: FaultConfig{
			ErrorRate: 1.0, // 100% error rate
		},
	})
	e.StartExperiment(context.Background(), "exp-1")

	exp, should := e.ShouldInjectFault("job-1")
	if !should {
		t.Error("should inject for targeted job")
	}
	if exp.ID != "exp-1" {
		t.Errorf("expected exp-1, got %s", exp.ID)
	}

	// Non-targeted job
	_, should = e.ShouldInjectFault("job-2")
	if should {
		t.Error("should not inject for non-targeted job")
	}
}

func TestInjectError(t *testing.T) {
	e := NewEngine()

	exp := &Experiment{
		ID: "exp-1",
		Config: FaultConfig{
			ErrorMessage: "test error",
		},
	}

	err := e.InjectError(exp)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "test error" {
		t.Errorf("expected 'test error', got %s", err.Error())
	}

	if !IsInjectedError(err) {
		t.Error("should be identified as injected error")
	}
}

func TestAddHook(t *testing.T) {
	e := NewEngine()

	events := make([]Event, 0)
	e.AddHook(func(event Event) {
		events = append(events, event)
	})

	e.CreateExperiment(&Experiment{ID: "exp-1"})
	ctx := context.Background()
	e.StartExperiment(ctx, "exp-1")

	// Give time for event to fire
	time.Sleep(10 * time.Millisecond)

	if len(events) < 1 {
		t.Error("expected at least 1 event")
	}
}

func TestGetStats(t *testing.T) {
	e := NewEngine()

	e.CreateExperiment(&Experiment{ID: "exp-1"})
	e.CreateExperiment(&Experiment{ID: "exp-2"})

	stats := e.GetStats()
	if stats.TotalExperiments != 2 {
		t.Errorf("expected 2 total, got %d", stats.TotalExperiments)
	}
	if stats.Enabled {
		t.Error("should not be enabled")
	}
}
