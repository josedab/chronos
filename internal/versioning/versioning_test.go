package versioning

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/chronos/chronos/internal/models"
)

func testJob(id, name string) *models.Job {
	return &models.Job{
		ID:       id,
		Name:     name,
		Schedule: "0 9 * * *",
		Webhook:  &models.WebhookConfig{URL: "http://example.com", Method: "POST"},
		Enabled:  true,
	}
}

func TestManager_CreateVersion(t *testing.T) {
	store := NewMemoryStore()
	mgr := NewManager(store)
	ctx := context.Background()

	job := testJob("job-1", "test-job")
	v, err := mgr.CreateVersion(ctx, job, "admin", "Initial version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Number != 1 {
		t.Errorf("expected version 1, got %d", v.Number)
	}
	if v.Author != "admin" {
		t.Errorf("expected author admin, got %s", v.Author)
	}
	if v.Description != "Initial version" {
		t.Errorf("expected description, got %s", v.Description)
	}
	if v.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestManager_CreateVersion_Increments(t *testing.T) {
	store := NewMemoryStore()
	mgr := NewManager(store)
	ctx := context.Background()

	job := testJob("job-1", "test")
	v1, _ := mgr.CreateVersion(ctx, job, "admin", "v1")
	if v1.Number != 1 {
		t.Fatalf("expected v1, got %d", v1.Number)
	}

	job.Schedule = "*/5 * * * *"
	v2, _ := mgr.CreateVersion(ctx, job, "admin", "v2")
	if v2.Number != 2 {
		t.Fatalf("expected v2, got %d", v2.Number)
	}

	// v2 should have a diff
	if v2.Diff == nil {
		t.Fatal("expected diff in v2")
	}
	if len(v2.Diff.FieldsChanged) == 0 {
		t.Error("expected fields changed in diff")
	}
}

func TestManager_GetVersion(t *testing.T) {
	store := NewMemoryStore()
	mgr := NewManager(store)
	ctx := context.Background()

	job := testJob("job-1", "test")
	mgr.CreateVersion(ctx, job, "admin", "v1")

	v, err := mgr.GetVersion(ctx, "job-1", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Number != 1 {
		t.Errorf("expected version 1, got %d", v.Number)
	}

	_, err = mgr.GetVersion(ctx, "job-1", 999)
	if err == nil {
		t.Error("expected error for nonexistent version")
	}
}

func TestManager_GetLatestVersion(t *testing.T) {
	store := NewMemoryStore()
	mgr := NewManager(store)
	ctx := context.Background()

	_, err := mgr.GetLatestVersion(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for no versions")
	}

	job := testJob("job-1", "test")
	mgr.CreateVersion(ctx, job, "admin", "v1")
	job.Schedule = "changed"
	mgr.CreateVersion(ctx, job, "admin", "v2")

	latest, err := mgr.GetLatestVersion(ctx, "job-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if latest.Number != 2 {
		t.Errorf("expected latest=2, got %d", latest.Number)
	}
}

func TestManager_ListVersions(t *testing.T) {
	store := NewMemoryStore()
	mgr := NewManager(store)
	ctx := context.Background()

	job := testJob("job-1", "test")
	for i := 0; i < 5; i++ {
		job.Schedule = "* * * * *"
		mgr.CreateVersion(ctx, job, "admin", "")
	}

	all, _ := mgr.ListVersions(ctx, "job-1", 10)
	if len(all) != 5 {
		t.Errorf("expected 5 versions, got %d", len(all))
	}

	limited, _ := mgr.ListVersions(ctx, "job-1", 3)
	if len(limited) != 3 {
		t.Errorf("expected 3 versions, got %d", len(limited))
	}

	// Default limit
	defaulted, _ := mgr.ListVersions(ctx, "job-1", 0)
	if len(defaulted) != 5 {
		t.Errorf("expected 5 with default limit, got %d", len(defaulted))
	}
}

func TestManager_GetJobAtVersion(t *testing.T) {
	store := NewMemoryStore()
	mgr := NewManager(store)
	ctx := context.Background()

	job := testJob("job-1", "original")
	mgr.CreateVersion(ctx, job, "admin", "v1")

	job.Name = "modified"
	mgr.CreateVersion(ctx, job, "admin", "v2")

	atV1, err := mgr.GetJobAtVersion(ctx, "job-1", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atV1.Name != "original" {
		t.Errorf("expected name 'original' at v1, got %s", atV1.Name)
	}
}

func TestManager_CompareVersions(t *testing.T) {
	store := NewMemoryStore()
	mgr := NewManager(store)
	ctx := context.Background()

	job := testJob("job-1", "test")
	mgr.CreateVersion(ctx, job, "admin", "v1")

	job.Schedule = "0 0 * * *"
	mgr.CreateVersion(ctx, job, "admin", "v2")

	comp, err := mgr.CompareVersions(ctx, "job-1", 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if comp.Version1 != 1 || comp.Version2 != 2 {
		t.Error("expected version numbers in comparison")
	}
	if comp.Diff == nil {
		t.Fatal("expected diff")
	}
	if len(comp.Diff.FieldsChanged) == 0 {
		t.Error("expected at least 1 changed field")
	}
}

func TestManager_Rollback(t *testing.T) {
	store := NewMemoryStore()
	mgr := NewManager(store)
	ctx := context.Background()

	job := testJob("job-1", "original")
	mgr.CreateVersion(ctx, job, "admin", "v1")

	job.Name = "modified"
	mgr.CreateVersion(ctx, job, "admin", "v2")

	rolled, err := mgr.Rollback(ctx, "job-1", 1, "admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rolled.Number != 3 {
		t.Errorf("expected rollback creates v3, got %d", rolled.Number)
	}

	// Verify content matches v1
	var rolledJob models.Job
	json.Unmarshal(rolled.Config, &rolledJob)
	if rolledJob.Name != "original" {
		t.Errorf("expected rolled back to 'original', got %s", rolledJob.Name)
	}
}

func TestManager_Rollback_CannotRollbackToLatest(t *testing.T) {
	store := NewMemoryStore()
	mgr := NewManager(store)
	ctx := context.Background()

	job := testJob("job-1", "test")
	mgr.CreateVersion(ctx, job, "admin", "v1")

	_, err := mgr.Rollback(ctx, "job-1", 1, "admin")
	if err != ErrCannotRollback {
		t.Errorf("expected ErrCannotRollback, got %v", err)
	}
}

func TestManager_CanaryLifecycle(t *testing.T) {
	store := NewMemoryStore()
	mgr := NewManager(store)
	ctx := context.Background()

	job := testJob("job-1", "test")
	mgr.CreateVersion(ctx, job, "admin", "base")
	job.Schedule = "new-schedule"
	mgr.CreateVersion(ctx, job, "admin", "canary candidate")

	// Start canary
	canary, err := mgr.StartCanary(ctx, "job-1", 2, 10, nil)
	if err != nil {
		t.Fatalf("start canary: %v", err)
	}
	if canary.TrafficPercent != 10 {
		t.Errorf("expected 10%%, got %d%%", canary.TrafficPercent)
	}
	if canary.Status != CanaryStatusRunning {
		t.Errorf("expected running, got %s", canary.Status)
	}

	// Update traffic
	updated, err := mgr.UpdateCanaryTraffic(ctx, "job-1", 50)
	if err != nil {
		t.Fatalf("update traffic: %v", err)
	}
	if updated.TrafficPercent != 50 {
		t.Errorf("expected 50%%, got %d%%", updated.TrafficPercent)
	}

	// Promote
	promoted, err := mgr.PromoteCanary(ctx, "job-1", "admin")
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	if promoted.Number != 3 {
		t.Errorf("expected promoted to v3, got %d", promoted.Number)
	}
}

func TestManager_CanaryRollback(t *testing.T) {
	store := NewMemoryStore()
	mgr := NewManager(store)
	ctx := context.Background()

	job := testJob("job-1", "test")
	mgr.CreateVersion(ctx, job, "admin", "v1")
	mgr.CreateVersion(ctx, job, "admin", "v2")

	mgr.StartCanary(ctx, "job-1", 2, 10, nil)

	err := mgr.RollbackCanary(ctx, "job-1")
	if err != nil {
		t.Fatalf("rollback canary: %v", err)
	}

	canary, _ := mgr.GetCanary(ctx, "job-1")
	if canary.Status != CanaryStatusRolledBack {
		t.Errorf("expected rolled_back, got %s", canary.Status)
	}
}

func TestManager_CanaryDuplicate(t *testing.T) {
	store := NewMemoryStore()
	mgr := NewManager(store)
	ctx := context.Background()

	job := testJob("job-1", "test")
	mgr.CreateVersion(ctx, job, "admin", "v1")
	mgr.CreateVersion(ctx, job, "admin", "v2")

	mgr.StartCanary(ctx, "job-1", 2, 10, nil)

	_, err := mgr.StartCanary(ctx, "job-1", 2, 20, nil)
	if err != ErrCanaryAlreadyActive {
		t.Errorf("expected ErrCanaryAlreadyActive, got %v", err)
	}
}

func TestManager_CanaryInvalidPercent(t *testing.T) {
	store := NewMemoryStore()
	mgr := NewManager(store)
	ctx := context.Background()

	job := testJob("job-1", "test")
	mgr.CreateVersion(ctx, job, "admin", "v1")
	mgr.CreateVersion(ctx, job, "admin", "v2")

	_, err := mgr.StartCanary(ctx, "job-1", 2, 150, nil)
	if err != ErrInvalidPercentage {
		t.Errorf("expected ErrInvalidPercentage, got %v", err)
	}
}

func TestManager_ShouldUseCanary(t *testing.T) {
	store := NewMemoryStore()
	mgr := NewManager(store)
	ctx := context.Background()

	// No canary should return false
	useCanary, _, _ := mgr.ShouldUseCanary(ctx, "job-1", "exec-1")
	if useCanary {
		t.Error("should not use canary when none active")
	}
}

func TestManager_UpdateCanaryMetrics(t *testing.T) {
	store := NewMemoryStore()
	mgr := NewManager(store)
	ctx := context.Background()

	job := testJob("job-1", "test")
	mgr.CreateVersion(ctx, job, "admin", "v1")
	mgr.CreateVersion(ctx, job, "admin", "v2")
	mgr.StartCanary(ctx, "job-1", 2, 50, nil)

	// Record some metrics
	mgr.UpdateCanaryMetrics(ctx, "job-1", true, true, 100*time.Millisecond)
	mgr.UpdateCanaryMetrics(ctx, "job-1", false, true, 200*time.Millisecond)

	canary, _ := mgr.GetCanary(ctx, "job-1")
	if canary.Metrics.CanaryExecutions != 1 {
		t.Errorf("expected 1 canary execution, got %d", canary.Metrics.CanaryExecutions)
	}
	if canary.Metrics.BaseExecutions != 1 {
		t.Errorf("expected 1 base execution, got %d", canary.Metrics.BaseExecutions)
	}
}

func TestMemoryStore_CRUD(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Create
	v := &Version{ID: "v1", JobID: "job-1", Number: 1, Config: json.RawMessage(`{}`)}
	if err := store.CreateVersion(ctx, v); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Get
	got, err := store.GetVersion(ctx, "job-1", 1)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != "v1" {
		t.Errorf("expected v1, got %s", got.ID)
	}

	// Delete
	if err := store.DeleteVersion(ctx, "job-1", 1); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err = store.GetVersion(ctx, "job-1", 1)
	if err != ErrVersionNotFound {
		t.Error("expected not found after delete")
	}
}

func TestMemoryStore_Canary(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	canary := &CanaryDeployment{ID: "c1", JobID: "job-1", Status: CanaryStatusRunning}
	store.CreateCanary(ctx, canary)

	got, err := store.GetCanary(ctx, "job-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != "c1" {
		t.Errorf("expected c1, got %s", got.ID)
	}

	active, _ := store.ListActiveCanaries(ctx)
	if len(active) != 1 {
		t.Errorf("expected 1 active, got %d", len(active))
	}

	store.DeleteCanary(ctx, "job-1")
	_, err = store.GetCanary(ctx, "job-1")
	if err != ErrNoCanaryActive {
		t.Error("expected not found after delete")
	}
}

func TestChangeTypes(t *testing.T) {
	if ChangeTypeAdded != "added" {
		t.Error("bad change type")
	}
	if ChangeTypeRemoved != "removed" {
		t.Error("bad change type")
	}
	if ChangeTypeModified != "modified" {
		t.Error("bad change type")
	}
}

func TestJsonEqual(t *testing.T) {
	if !jsonEqual("hello", "hello") {
		t.Error("same strings should be equal")
	}
	if jsonEqual("a", "b") {
		t.Error("different strings should not be equal")
	}
	if !jsonEqual(42, 42) {
		t.Error("same ints should be equal")
	}
}
