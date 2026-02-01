package storage

import (
	"testing"
	"time"

	"github.com/chronos/chronos/internal/models"
)

func TestMemoryStore_JobCRUD(t *testing.T) {
	store := NewMemoryStore()

	job := &models.Job{
		ID:       "test-job-1",
		Name:     "Test Job",
		Schedule: "*/5 * * * *",
		Webhook: &models.WebhookConfig{
			URL:    "http://example.com/hook",
			Method: "POST",
		},
		Enabled:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Create
	if err := store.CreateJob(job); err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}

	// Create duplicate should fail
	if err := store.CreateJob(job); err != models.ErrJobAlreadyExists {
		t.Fatalf("Expected ErrJobAlreadyExists, got: %v", err)
	}

	// Get
	retrieved, err := store.GetJob(job.ID)
	if err != nil {
		t.Fatalf("GetJob failed: %v", err)
	}
	if retrieved.Name != job.Name {
		t.Errorf("Expected name %s, got %s", job.Name, retrieved.Name)
	}

	// Update
	retrieved.Name = "Updated Job"
	if err := store.UpdateJob(retrieved); err != nil {
		t.Fatalf("UpdateJob failed: %v", err)
	}

	updated, _ := store.GetJob(job.ID)
	if updated.Name != "Updated Job" {
		t.Errorf("Expected updated name, got %s", updated.Name)
	}

	// List
	jobs, err := store.ListJobs()
	if err != nil {
		t.Fatalf("ListJobs failed: %v", err)
	}
	if len(jobs) != 1 {
		t.Errorf("Expected 1 job, got %d", len(jobs))
	}

	// Delete
	if err := store.DeleteJob(job.ID); err != nil {
		t.Fatalf("DeleteJob failed: %v", err)
	}

	// Get deleted should fail
	if _, err := store.GetJob(job.ID); err != models.ErrJobNotFound {
		t.Fatalf("Expected ErrJobNotFound, got: %v", err)
	}
}

func TestMemoryStore_ExecutionCRUD(t *testing.T) {
	store := NewMemoryStore()

	exec := &models.Execution{
		ID:            "exec-1",
		JobID:         "job-1",
		Status:        models.ExecutionRunning,
		ScheduledTime: time.Now(),
		StartedAt:     time.Now(),
	}

	// Record
	if err := store.RecordExecution(exec); err != nil {
		t.Fatalf("RecordExecution failed: %v", err)
	}

	// Get
	retrieved, err := store.GetExecution(exec.JobID, exec.ID)
	if err != nil {
		t.Fatalf("GetExecution failed: %v", err)
	}
	if retrieved.Status != models.ExecutionRunning {
		t.Errorf("Expected status %s, got %s", models.ExecutionRunning, retrieved.Status)
	}

	// Update
	exec.Status = models.ExecutionSuccess
	completedAt := time.Now()
	exec.CompletedAt = &completedAt
	if err := store.UpdateExecution(exec); err != nil {
		t.Fatalf("UpdateExecution failed: %v", err)
	}

	updated, _ := store.GetExecution(exec.JobID, exec.ID)
	if updated.Status != models.ExecutionSuccess {
		t.Errorf("Expected status %s, got %s", models.ExecutionSuccess, updated.Status)
	}

	// List
	execs, err := store.ListExecutions(exec.JobID, 10)
	if err != nil {
		t.Fatalf("ListExecutions failed: %v", err)
	}
	if len(execs) != 1 {
		t.Errorf("Expected 1 execution, got %d", len(execs))
	}

	// Delete
	if err := store.DeleteExecutions(exec.JobID); err != nil {
		t.Fatalf("DeleteExecutions failed: %v", err)
	}

	execs, _ = store.ListExecutions(exec.JobID, 10)
	if len(execs) != 0 {
		t.Errorf("Expected 0 executions after delete, got %d", len(execs))
	}
}

func TestMemoryStore_Lock(t *testing.T) {
	store := NewMemoryStore()

	jobID := "job-1"
	node1 := "node-1"
	node2 := "node-2"
	ttl := 5 * time.Second

	// Acquire lock
	acquired, err := store.AcquireLock(jobID, node1, ttl)
	if err != nil {
		t.Fatalf("AcquireLock failed: %v", err)
	}
	if !acquired {
		t.Error("Expected lock to be acquired")
	}

	// Check locked
	locked, holder, err := store.IsLocked(jobID)
	if err != nil {
		t.Fatalf("IsLocked failed: %v", err)
	}
	if !locked {
		t.Error("Expected job to be locked")
	}
	if holder != node1 {
		t.Errorf("Expected holder %s, got %s", node1, holder)
	}

	// Second node cannot acquire
	acquired, _ = store.AcquireLock(jobID, node2, ttl)
	if acquired {
		t.Error("Expected second node to fail acquiring lock")
	}

	// Release lock
	if err := store.ReleaseLock(jobID, node1); err != nil {
		t.Fatalf("ReleaseLock failed: %v", err)
	}

	// Now second node can acquire
	acquired, _ = store.AcquireLock(jobID, node2, ttl)
	if !acquired {
		t.Error("Expected second node to acquire lock after release")
	}
}

func TestMemoryStore_ScheduleState(t *testing.T) {
	store := NewMemoryStore()

	state := &models.ScheduleState{
		JobID:   "job-1",
		LastRun: time.Now(),
		NextRun: time.Now().Add(time.Hour),
	}

	// Save
	if err := store.SaveScheduleState(state); err != nil {
		t.Fatalf("SaveScheduleState failed: %v", err)
	}

	// Get
	retrieved, err := store.GetScheduleState(state.JobID)
	if err != nil {
		t.Fatalf("GetScheduleState failed: %v", err)
	}
	if !retrieved.LastRun.Equal(state.LastRun) {
		t.Error("Schedule state mismatch")
	}

	// Get non-existent
	_, err = store.GetScheduleState("nonexistent")
	if err != models.ErrScheduleStateNotFound {
		t.Errorf("Expected ErrScheduleStateNotFound, got: %v", err)
	}
}

func TestMemoryStore_JobVersioning(t *testing.T) {
	store := NewMemoryStore()

	jobID := "job-1"
	v1 := &models.JobVersion{
		JobID:     jobID,
		Version:   1,
		Config:    models.Job{ID: jobID, Name: "v1"},
		CreatedAt: time.Now(),
	}
	v2 := &models.JobVersion{
		JobID:     jobID,
		Version:   2,
		Config:    models.Job{ID: jobID, Name: "v2"},
		CreatedAt: time.Now(),
	}

	// Save versions
	if err := store.SaveJobVersion(v1); err != nil {
		t.Fatalf("SaveJobVersion v1 failed: %v", err)
	}
	if err := store.SaveJobVersion(v2); err != nil {
		t.Fatalf("SaveJobVersion v2 failed: %v", err)
	}

	// List versions (should be in descending order)
	versions, err := store.ListJobVersions(jobID)
	if err != nil {
		t.Fatalf("ListJobVersions failed: %v", err)
	}
	if len(versions) != 2 {
		t.Errorf("Expected 2 versions, got %d", len(versions))
	}
	if versions[0].Version != 2 {
		t.Error("Expected newest version first")
	}

	// Get specific version
	retrieved, err := store.GetJobVersion(jobID, 1)
	if err != nil {
		t.Fatalf("GetJobVersion failed: %v", err)
	}
	if retrieved.Config.Name != "v1" {
		t.Error("Version config mismatch")
	}

	// Get non-existent version
	_, err = store.GetJobVersion(jobID, 99)
	if err != models.ErrVersionNotFound {
		t.Errorf("Expected ErrVersionNotFound, got: %v", err)
	}
}

func TestMemoryStore_SnapshotRestore(t *testing.T) {
	store := NewMemoryStore()

	// Add some data
	job := &models.Job{
		ID:       "job-1",
		Name:     "Test Job",
		Schedule: "* * * * *",
		Webhook: &models.WebhookConfig{
			URL:    "http://example.com",
			Method: "POST",
		},
		Enabled:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.CreateJob(job)

	// Create snapshot
	snapshot, err := store.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	// Clear and restore
	store.Reset()

	// Verify data is cleared
	jobs, _ := store.ListJobs()
	if len(jobs) != 0 {
		t.Error("Expected empty store after reset")
	}

	// Restore
	if err := store.Restore(snapshot); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// Verify data is restored
	jobs, _ = store.ListJobs()
	if len(jobs) != 1 {
		t.Errorf("Expected 1 job after restore, got %d", len(jobs))
	}

	restored, _ := store.GetJob("job-1")
	if restored.Name != "Test Job" {
		t.Errorf("Expected restored job name 'Test Job', got %s", restored.Name)
	}
}

func TestMemoryStore_Pagination(t *testing.T) {
	store := NewMemoryStore()

	// Create multiple jobs
	for i := 0; i < 10; i++ {
		job := &models.Job{
			ID:       "job-" + string(rune('a'+i)),
			Name:     "Test Job",
			Schedule: "* * * * *",
			Webhook: &models.WebhookConfig{
				URL:    "http://example.com",
				Method: "POST",
			},
			Enabled:   true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		store.CreateJob(job)
	}

	// Test pagination
	jobs, total, err := store.ListJobsPaginated(0, 3)
	if err != nil {
		t.Fatalf("ListJobsPaginated failed: %v", err)
	}
	if total != 10 {
		t.Errorf("Expected total 10, got %d", total)
	}
	if len(jobs) != 3 {
		t.Errorf("Expected 3 jobs, got %d", len(jobs))
	}

	// Second page
	jobs, _, _ = store.ListJobsPaginated(3, 3)
	if len(jobs) != 3 {
		t.Errorf("Expected 3 jobs on second page, got %d", len(jobs))
	}

	// Beyond range
	jobs, _, _ = store.ListJobsPaginated(100, 10)
	if len(jobs) != 0 {
		t.Errorf("Expected 0 jobs beyond range, got %d", len(jobs))
	}
}
