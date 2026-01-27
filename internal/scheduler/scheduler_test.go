package scheduler

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/chronos/chronos/internal/dispatcher"
	"github.com/chronos/chronos/internal/models"
	"github.com/chronos/chronos/internal/storage"
	"github.com/rs/zerolog"
)

func setupTestScheduler(t *testing.T) (*Scheduler, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "scheduler-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	store, err := storage.NewStore(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create store: %v", err)
	}

	disp := dispatcher.New(dispatcher.DefaultConfig())
	logger := zerolog.Nop()

	sched := New(store, disp, logger, nil)

	cleanup := func() {
		store.Close()
		os.RemoveAll(tmpDir)
	}

	return sched, cleanup
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.TickInterval != time.Second {
		t.Errorf("expected TickInterval 1s, got %v", cfg.TickInterval)
	}
	if cfg.MissedRunPolicy != MissedRunExecuteOne {
		t.Errorf("expected MissedRunPolicy 'execute_one', got %s", cfg.MissedRunPolicy)
	}
}

func TestNew(t *testing.T) {
	sched, cleanup := setupTestScheduler(t)
	defer cleanup()

	if sched == nil {
		t.Fatal("New returned nil")
	}
	if sched.jobs == nil {
		t.Error("jobs map not initialized")
	}
	if sched.nextRuns == nil {
		t.Error("nextRuns queue not initialized")
	}
	if sched.stopCh == nil {
		t.Error("stopCh not initialized")
	}
}

func TestScheduler_AddJob(t *testing.T) {
	sched, cleanup := setupTestScheduler(t)
	defer cleanup()

	job := &models.Job{
		ID:       "job-1",
		Name:     "test-job",
		Schedule: "* * * * *",
		Timezone: "UTC",
		Enabled:  true,
	}

	err := sched.AddJob(job)
	if err != nil {
		t.Fatalf("AddJob failed: %v", err)
	}

	// Verify job was added
	if len(sched.jobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(sched.jobs))
	}

	// Verify metrics updated
	metrics := sched.GetMetrics()
	if metrics.JobsTotal != 1 {
		t.Errorf("expected JobsTotal 1, got %d", metrics.JobsTotal)
	}
	if metrics.JobsEnabled != 1 {
		t.Errorf("expected JobsEnabled 1, got %d", metrics.JobsEnabled)
	}
}

func TestScheduler_AddJob_Disabled(t *testing.T) {
	sched, cleanup := setupTestScheduler(t)
	defer cleanup()

	job := &models.Job{
		ID:       "job-disabled",
		Name:     "disabled-job",
		Schedule: "* * * * *",
		Timezone: "UTC",
		Enabled:  false,
	}

	err := sched.AddJob(job)
	if err != nil {
		t.Fatalf("AddJob failed: %v", err)
	}

	metrics := sched.GetMetrics()
	if metrics.JobsTotal != 1 {
		t.Errorf("expected JobsTotal 1, got %d", metrics.JobsTotal)
	}
	if metrics.JobsEnabled != 0 {
		t.Errorf("expected JobsEnabled 0, got %d", metrics.JobsEnabled)
	}
}

func TestScheduler_RemoveJob(t *testing.T) {
	sched, cleanup := setupTestScheduler(t)
	defer cleanup()

	job := &models.Job{
		ID:       "job-to-remove",
		Name:     "removable",
		Schedule: "* * * * *",
		Timezone: "UTC",
		Enabled:  true,
	}

	_ = sched.AddJob(job)

	// Remove the job
	sched.RemoveJob(job.ID)

	if len(sched.jobs) != 0 {
		t.Errorf("expected 0 jobs after removal, got %d", len(sched.jobs))
	}

	metrics := sched.GetMetrics()
	if metrics.JobsTotal != 0 {
		t.Errorf("expected JobsTotal 0, got %d", metrics.JobsTotal)
	}
}

func TestScheduler_UpdateJob(t *testing.T) {
	sched, cleanup := setupTestScheduler(t)
	defer cleanup()

	job := &models.Job{
		ID:       "job-update",
		Name:     "original",
		Schedule: "* * * * *",
		Timezone: "UTC",
		Enabled:  true,
	}

	_ = sched.AddJob(job)

	// Update the job name
	job.Name = "updated"

	err := sched.UpdateJob(job)
	if err != nil {
		t.Fatalf("UpdateJob failed: %v", err)
	}

	// Verify job was updated
	updated, exists := sched.GetJob(job.ID)
	if !exists {
		t.Fatal("job not found after update")
	}
	if updated.Name != "updated" {
		t.Errorf("expected name 'updated', got %s", updated.Name)
	}
}

func TestScheduler_GetJob(t *testing.T) {
	sched, cleanup := setupTestScheduler(t)
	defer cleanup()

	job := &models.Job{
		ID:       "job-get",
		Name:     "get-test",
		Schedule: "* * * * *",
		Timezone: "UTC",
		Enabled:  true,
	}

	_ = sched.AddJob(job)

	retrieved, exists := sched.GetJob(job.ID)
	if !exists {
		t.Fatal("GetJob returned false")
	}
	if retrieved == nil {
		t.Fatal("GetJob returned nil")
	}
	if retrieved.Name != job.Name {
		t.Errorf("expected name %s, got %s", job.Name, retrieved.Name)
	}
}

func TestScheduler_GetJob_NotFound(t *testing.T) {
	sched, cleanup := setupTestScheduler(t)
	defer cleanup()

	retrieved, exists := sched.GetJob("nonexistent")
	if exists {
		t.Error("expected false for nonexistent job")
	}
	if retrieved != nil {
		t.Error("expected nil for nonexistent job")
	}
}

func TestScheduler_JobCount(t *testing.T) {
	sched, cleanup := setupTestScheduler(t)
	defer cleanup()

	jobs := []*models.Job{
		{ID: "job-1", Name: "one", Schedule: "* * * * *", Timezone: "UTC", Enabled: true},
		{ID: "job-2", Name: "two", Schedule: "* * * * *", Timezone: "UTC", Enabled: true},
		{ID: "job-3", Name: "three", Schedule: "* * * * *", Timezone: "UTC", Enabled: false},
	}

	for _, j := range jobs {
		_ = sched.AddJob(j)
	}

	metrics := sched.GetMetrics()
	if metrics.JobsTotal != 3 {
		t.Errorf("expected 3 jobs, got %d", metrics.JobsTotal)
	}
}

func TestScheduler_SetLeader(t *testing.T) {
	sched, cleanup := setupTestScheduler(t)
	defer cleanup()

	// Initially not leader
	if sched.IsLeader() {
		t.Error("expected not leader initially")
	}

	// Set as leader
	sched.SetLeader(true)
	if !sched.IsLeader() {
		t.Error("expected leader after SetLeader(true)")
	}

	// Remove leadership
	sched.SetLeader(false)
	if sched.IsLeader() {
		t.Error("expected not leader after SetLeader(false)")
	}
}

func TestScheduler_GetMetrics(t *testing.T) {
	sched, cleanup := setupTestScheduler(t)
	defer cleanup()

	metrics := sched.GetMetrics()

	// Initial metrics should be zero
	if metrics.JobsTotal != 0 {
		t.Errorf("expected JobsTotal 0, got %d", metrics.JobsTotal)
	}
	if metrics.JobsEnabled != 0 {
		t.Errorf("expected JobsEnabled 0, got %d", metrics.JobsEnabled)
	}
	if metrics.ScheduledRuns != 0 {
		t.Errorf("expected ScheduledRuns 0, got %d", metrics.ScheduledRuns)
	}
	if metrics.MissedRuns != 0 {
		t.Errorf("expected MissedRuns 0, got %d", metrics.MissedRuns)
	}
	if metrics.ExecutionsTotal != 0 {
		t.Errorf("expected ExecutionsTotal 0, got %d", metrics.ExecutionsTotal)
	}
}

func TestScheduler_GetNextRuns(t *testing.T) {
	sched, cleanup := setupTestScheduler(t)
	defer cleanup()

	// GetNextRuns works on the internal queue which is populated during Start()
	// Here we just verify the method doesn't panic on empty queue
	runs := sched.GetNextRuns(10)
	if runs == nil {
		t.Error("GetNextRuns returned nil")
	}
}

func TestMissedRunPolicy_Values(t *testing.T) {
	tests := []struct {
		policy   MissedRunPolicy
		expected string
	}{
		{MissedRunIgnore, "ignore"},
		{MissedRunExecuteOne, "execute_one"},
		{MissedRunExecuteAll, "execute_all"},
	}

	for _, tt := range tests {
		t.Run(string(tt.policy), func(t *testing.T) {
			if string(tt.policy) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.policy)
			}
		})
	}
}

// TestScheduler_ConcurrentTick tests that tick() doesn't deadlock under concurrent access.
// This is a regression test for the deadlock fix where tick() was holding queueMu
// while calling dispatcher methods that could block.
func TestScheduler_ConcurrentTick(t *testing.T) {
	sched, cleanup := setupTestScheduler(t)
	defer cleanup()

	// Add multiple jobs
	for i := 0; i < 10; i++ {
		job := &models.Job{
			ID:          fmt.Sprintf("job-%d", i),
			Name:        fmt.Sprintf("test-job-%d", i),
			Schedule:    "* * * * *", // Every minute
			Timezone:    "UTC",
			Enabled:     true,
			Concurrency: models.ConcurrencyAllow,
			Webhook: &models.WebhookConfig{
				URL:    "http://localhost:9999/test",
				Method: "GET",
			},
		}
		if err := sched.AddJob(job); err != nil {
			t.Fatalf("failed to add job: %v", err)
		}
	}

	sched.SetLeader(true)

	// Channel to signal completion
	done := make(chan struct{})
	timeout := time.After(5 * time.Second)

	// Run concurrent operations
	go func() {
		var wg sync.WaitGroup

		// Concurrent tick calls
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					sched.tick()
					time.Sleep(time.Millisecond)
				}
			}()
		}

		// Concurrent job operations
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					job := &models.Job{
						ID:       fmt.Sprintf("concurrent-job-%d-%d", idx, j),
						Name:     "concurrent",
						Schedule: "* * * * *",
						Timezone: "UTC",
						Enabled:  true,
					}
					sched.AddJob(job)
					sched.GetJob(job.ID)
					sched.RemoveJob(job.ID)
				}
			}(i)
		}

		// Concurrent metrics access
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 20; j++ {
					sched.GetMetrics()
					sched.GetNextRuns(5)
					sched.ListJobs()
					time.Sleep(time.Millisecond)
				}
			}()
		}

		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - no deadlock
	case <-timeout:
		t.Fatal("test timed out - possible deadlock detected")
	}
}

// TestScheduler_TriggerJobPanicRecovery tests that TriggerJob recovers from panics.
func TestScheduler_TriggerJobPanicRecovery(t *testing.T) {
	sched, cleanup := setupTestScheduler(t)
	defer cleanup()

	job := &models.Job{
		ID:       "panic-job",
		Name:     "panic-test",
		Schedule: "* * * * *",
		Timezone: "UTC",
		Enabled:  true,
		Webhook: &models.WebhookConfig{
			URL:    "http://invalid-url-that-will-cause-issues:99999",
			Method: "GET",
		},
	}

	if err := sched.AddJob(job); err != nil {
		t.Fatalf("failed to add job: %v", err)
	}

	// This should not panic the test
	_, err := sched.TriggerJob(job.ID)
	// Error is expected (connection refused), but no panic
	if err == nil {
		// That's actually fine too - connection might succeed in some environments
		t.Log("TriggerJob succeeded unexpectedly but didn't panic")
	}
}

// TestScheduler_TriggerJob_NotFound tests TriggerJob with non-existent job.
func TestScheduler_TriggerJob_NotFound(t *testing.T) {
	sched, cleanup := setupTestScheduler(t)
	defer cleanup()

	_, err := sched.TriggerJob("nonexistent-job")
	if err != models.ErrJobNotFound {
		t.Errorf("expected ErrJobNotFound, got %v", err)
	}
}
