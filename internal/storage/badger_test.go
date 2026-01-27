package storage

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/chronos/chronos/internal/models"
)

func setupTestStore(t *testing.T) (*BadgerStore, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "chronos-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	store, err := NewStore(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create store: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.RemoveAll(tmpDir)
	}

	return store, cleanup
}

func TestStore_JobCRUD(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create job
	job := &models.Job{
		ID:        "test-job-1",
		Name:      "Test Job",
		Schedule:  "* * * * *",
		Enabled:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Webhook: &models.WebhookConfig{
			URL:    "https://example.com/webhook",
			Method: "POST",
		},
	}

	err := store.CreateJob(job)
	if err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}

	// Get job
	retrieved, err := store.GetJob(job.ID)
	if err != nil {
		t.Fatalf("GetJob failed: %v", err)
	}
	if retrieved.Name != job.Name {
		t.Errorf("expected name %q, got %q", job.Name, retrieved.Name)
	}
	if retrieved.Schedule != job.Schedule {
		t.Errorf("expected schedule %q, got %q", job.Schedule, retrieved.Schedule)
	}

	// Update job
	job.Name = "Updated Job"
	job.UpdatedAt = time.Now()
	err = store.UpdateJob(job)
	if err != nil {
		t.Fatalf("UpdateJob failed: %v", err)
	}

	retrieved, err = store.GetJob(job.ID)
	if err != nil {
		t.Fatalf("GetJob after update failed: %v", err)
	}
	if retrieved.Name != "Updated Job" {
		t.Errorf("expected name %q, got %q", "Updated Job", retrieved.Name)
	}

	// List jobs
	jobs, err := store.ListJobs()
	if err != nil {
		t.Fatalf("ListJobs failed: %v", err)
	}
	if len(jobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(jobs))
	}

	// Delete job
	err = store.DeleteJob(job.ID)
	if err != nil {
		t.Fatalf("DeleteJob failed: %v", err)
	}

	// Verify deletion
	_, err = store.GetJob(job.ID)
	if err != models.ErrJobNotFound {
		t.Errorf("expected ErrJobNotFound, got %v", err)
	}
}

func TestStore_JobNotFound(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	_, err := store.GetJob("nonexistent")
	if err != models.ErrJobNotFound {
		t.Errorf("expected ErrJobNotFound, got %v", err)
	}
}

func TestStore_JobAlreadyExists(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	job := &models.Job{
		ID:        "test-job-1",
		Name:      "Test Job",
		Schedule:  "* * * * *",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := store.CreateJob(job)
	if err != nil {
		t.Fatalf("first CreateJob failed: %v", err)
	}

	err = store.CreateJob(job)
	if err != models.ErrJobAlreadyExists {
		t.Errorf("expected ErrJobAlreadyExists, got %v", err)
	}
}

func TestStore_ExecutionCRUD(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	jobID := "test-job-1"

	// Create executions
	for i := 1; i <= 5; i++ {
		exec := &models.Execution{
			ID:            fmt.Sprintf("exec-%d", i),
			JobID:         jobID,
			ScheduledTime: time.Now().Add(time.Duration(-i) * time.Hour),
			StartedAt:     time.Now().Add(time.Duration(-i) * time.Hour),
			Status:        models.ExecutionSuccess,
			Attempts:      1,
		}
		err := store.RecordExecution(exec)
		if err != nil {
			t.Fatalf("RecordExecution failed: %v", err)
		}
	}

	// List executions
	executions, err := store.ListExecutions(jobID, 10)
	if err != nil {
		t.Fatalf("ListExecutions failed: %v", err)
	}
	if len(executions) != 5 {
		t.Errorf("expected 5 executions, got %d", len(executions))
	}

	// List with limit
	executions, err = store.ListExecutions(jobID, 3)
	if err != nil {
		t.Fatalf("ListExecutions with limit failed: %v", err)
	}
	if len(executions) != 3 {
		t.Errorf("expected 3 executions, got %d", len(executions))
	}

	// Get single execution
	exec, err := store.GetExecution(jobID, "exec-1")
	if err != nil {
		t.Fatalf("GetExecution failed: %v", err)
	}
	if exec.ID != "exec-1" {
		t.Errorf("expected exec-1, got %s", exec.ID)
	}

	// Delete executions
	err = store.DeleteExecutions(jobID)
	if err != nil {
		t.Fatalf("DeleteExecutions failed: %v", err)
	}

	executions, err = store.ListExecutions(jobID, 10)
	if err != nil {
		t.Fatalf("ListExecutions after delete failed: %v", err)
	}
	if len(executions) != 0 {
		t.Errorf("expected 0 executions after delete, got %d", len(executions))
	}
}

func TestStore_Lock(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	jobID := "test-job-1"
	nodeID := "node-1"
	ttl := 5 * time.Second

	// Acquire lock
	acquired, err := store.AcquireLock(jobID, nodeID, ttl)
	if err != nil {
		t.Fatalf("AcquireLock failed: %v", err)
	}
	if !acquired {
		t.Error("expected lock to be acquired")
	}

	// Check if locked
	locked, owner, err := store.IsLocked(jobID)
	if err != nil {
		t.Fatalf("IsLocked failed: %v", err)
	}
	if !locked {
		t.Error("expected job to be locked")
	}
	if owner != nodeID {
		t.Errorf("expected owner %q, got %q", nodeID, owner)
	}

	// Try to acquire same lock from different node
	acquired, err = store.AcquireLock(jobID, "node-2", ttl)
	if err != nil {
		t.Fatalf("AcquireLock from node-2 failed: %v", err)
	}
	if acquired {
		t.Error("expected lock acquisition to fail for node-2")
	}

	// Release lock
	err = store.ReleaseLock(jobID, nodeID)
	if err != nil {
		t.Fatalf("ReleaseLock failed: %v", err)
	}

	// Check if unlocked
	locked, _, err = store.IsLocked(jobID)
	if err != nil {
		t.Fatalf("IsLocked after release failed: %v", err)
	}
	if locked {
		t.Error("expected job to be unlocked")
	}

	// Acquire lock again
	acquired, err = store.AcquireLock(jobID, "node-2", ttl)
	if err != nil {
		t.Fatalf("AcquireLock after release failed: %v", err)
	}
	if !acquired {
		t.Error("expected lock to be acquired after release")
	}
}

func TestStore_ScheduleState(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	jobID := "test-job-1"
	nextRun := time.Now().Add(1 * time.Hour)
	lastRun := time.Now()

	state := &models.ScheduleState{
		JobID:       jobID,
		NextRun:     nextRun,
		LastRun:     lastRun,
		MissedCount: 0,
	}

	err := store.SaveScheduleState(state)
	if err != nil {
		t.Fatalf("SaveScheduleState failed: %v", err)
	}

	retrieved, err := store.GetScheduleState(jobID)
	if err != nil {
		t.Fatalf("GetScheduleState failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("expected schedule state, got nil")
	}
	if retrieved.JobID != jobID {
		t.Errorf("expected job ID %q, got %q", jobID, retrieved.JobID)
	}
}

func TestStore_SnapshotRestore(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create some data
	job := &models.Job{
		ID:        "test-job-1",
		Name:      "Test Job",
		Schedule:  "* * * * *",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.CreateJob(job)

	// Take snapshot
	snapshot, err := store.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if len(snapshot) == 0 {
		t.Error("expected non-empty snapshot")
	}

	// Delete the job
	store.DeleteJob(job.ID)

	// Verify job is gone
	_, err = store.GetJob(job.ID)
	if err != models.ErrJobNotFound {
		t.Error("expected job to be deleted")
	}

	// Restore from snapshot
	err = store.Restore(snapshot)
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// Verify job is back
	retrieved, err := store.GetJob(job.ID)
	if err != nil {
		t.Fatalf("GetJob after restore failed: %v", err)
	}
	if retrieved.Name != job.Name {
		t.Errorf("expected name %q, got %q", job.Name, retrieved.Name)
	}
}

func TestStore_JobVersioning(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	job := &models.Job{
		ID:        "test-job-1",
		Name:      "Test Job",
		Schedule:  "* * * * *",
		Enabled:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Save version 1
	v1 := &models.JobVersion{
		JobID:     job.ID,
		Version:   1,
		Config:    *job,
		CreatedAt: time.Now(),
	}
	if err := store.SaveJobVersion(v1); err != nil {
		t.Fatalf("SaveJobVersion v1 failed: %v", err)
	}

	// Modify job and save version 2
	job.Name = "Updated Test Job"
	job.Version = 2
	v2 := &models.JobVersion{
		JobID:     job.ID,
		Version:   2,
		Config:    *job,
		CreatedAt: time.Now(),
	}
	if err := store.SaveJobVersion(v2); err != nil {
		t.Fatalf("SaveJobVersion v2 failed: %v", err)
	}

	// List versions
	versions, err := store.ListJobVersions(job.ID)
	if err != nil {
		t.Fatalf("ListJobVersions failed: %v", err)
	}
	if len(versions) != 2 {
		t.Errorf("expected 2 versions, got %d", len(versions))
	}

	// Get specific version
	retrieved, err := store.GetJobVersion(job.ID, 1)
	if err != nil {
		t.Fatalf("GetJobVersion failed: %v", err)
	}
	if retrieved.Config.Name != "Test Job" {
		t.Errorf("expected name 'Test Job', got %q", retrieved.Config.Name)
	}

	// Get version 2
	retrieved, err = store.GetJobVersion(job.ID, 2)
	if err != nil {
		t.Fatalf("GetJobVersion v2 failed: %v", err)
	}
	if retrieved.Config.Name != "Updated Test Job" {
		t.Errorf("expected name 'Updated Test Job', got %q", retrieved.Config.Name)
	}

	// Get non-existent version
	_, err = store.GetJobVersion(job.ID, 99)
	if err != models.ErrJobVersionNotFound {
		t.Errorf("expected ErrJobVersionNotFound, got %v", err)
	}
}

// Need to add fmt import
func init() {}
