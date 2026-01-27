package raft

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/chronos/chronos/internal/models"
	"github.com/chronos/chronos/internal/storage"
	"github.com/rs/zerolog"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.NodeID != "node-1" {
		t.Errorf("expected NodeID 'node-1', got %s", cfg.NodeID)
	}
	if cfg.RaftDir != "./data/raft" {
		t.Errorf("expected RaftDir './data/raft', got %s", cfg.RaftDir)
	}
	if cfg.RaftAddress != "127.0.0.1:7000" {
		t.Errorf("expected RaftAddress '127.0.0.1:7000', got %s", cfg.RaftAddress)
	}
	if cfg.HeartbeatTimeout != 500*time.Millisecond {
		t.Errorf("expected HeartbeatTimeout 500ms, got %v", cfg.HeartbeatTimeout)
	}
	if cfg.ElectionTimeout != 1*time.Second {
		t.Errorf("expected ElectionTimeout 1s, got %v", cfg.ElectionTimeout)
	}
}

func TestNewCreateJobCommand(t *testing.T) {
	job := &models.Job{
		ID:       "job-123",
		Name:     "test-job",
		Schedule: "* * * * *",
		Enabled:  true,
	}

	cmd, err := NewCreateJobCommand(job)
	if err != nil {
		t.Fatalf("NewCreateJobCommand failed: %v", err)
	}

	if cmd.Type != CmdCreateJob {
		t.Errorf("expected type CmdCreateJob, got %s", cmd.Type)
	}

	var decoded models.Job
	if err := json.Unmarshal(cmd.Payload, &decoded); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if decoded.ID != job.ID {
		t.Errorf("expected job ID %s, got %s", job.ID, decoded.ID)
	}
	if decoded.Name != job.Name {
		t.Errorf("expected job name %s, got %s", job.Name, decoded.Name)
	}
}

func TestNewUpdateJobCommand(t *testing.T) {
	job := &models.Job{
		ID:       "job-456",
		Name:     "updated-job",
		Schedule: "0 * * * *",
		Enabled:  false,
	}

	cmd, err := NewUpdateJobCommand(job)
	if err != nil {
		t.Fatalf("NewUpdateJobCommand failed: %v", err)
	}

	if cmd.Type != CmdUpdateJob {
		t.Errorf("expected type CmdUpdateJob, got %s", cmd.Type)
	}
}

func TestNewDeleteJobCommand(t *testing.T) {
	jobID := "job-789"

	cmd, err := NewDeleteJobCommand(jobID)
	if err != nil {
		t.Fatalf("NewDeleteJobCommand failed: %v", err)
	}

	if cmd.Type != CmdDeleteJob {
		t.Errorf("expected type CmdDeleteJob, got %s", cmd.Type)
	}

	var payload struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if payload.ID != jobID {
		t.Errorf("expected job ID %s, got %s", jobID, payload.ID)
	}
}

func TestNewRecordExecutionCommand(t *testing.T) {
	exec := &models.Execution{
		ID:        "exec-123",
		JobID:     "job-123",
		Status:    models.ExecutionSuccess,
		StartedAt: time.Now(),
	}

	cmd, err := NewRecordExecutionCommand(exec)
	if err != nil {
		t.Fatalf("NewRecordExecutionCommand failed: %v", err)
	}

	if cmd.Type != CmdRecordExecution {
		t.Errorf("expected type CmdRecordExecution, got %s", cmd.Type)
	}
}

func TestNewAcquireLockCommand(t *testing.T) {
	cmd, err := NewAcquireLockCommand("job-123", "node-1", 30*time.Second)
	if err != nil {
		t.Fatalf("NewAcquireLockCommand failed: %v", err)
	}

	if cmd.Type != CmdAcquireLock {
		t.Errorf("expected type CmdAcquireLock, got %s", cmd.Type)
	}

	var payload struct {
		JobID  string        `json:"job_id"`
		NodeID string        `json:"node_id"`
		TTL    time.Duration `json:"ttl"`
	}
	if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if payload.JobID != "job-123" {
		t.Errorf("expected job ID 'job-123', got %s", payload.JobID)
	}
	if payload.NodeID != "node-1" {
		t.Errorf("expected node ID 'node-1', got %s", payload.NodeID)
	}
	if payload.TTL != 30*time.Second {
		t.Errorf("expected TTL 30s, got %v", payload.TTL)
	}
}

func TestNewReleaseLockCommand(t *testing.T) {
	cmd, err := NewReleaseLockCommand("job-123", "node-1")
	if err != nil {
		t.Fatalf("NewReleaseLockCommand failed: %v", err)
	}

	if cmd.Type != CmdReleaseLock {
		t.Errorf("expected type CmdReleaseLock, got %s", cmd.Type)
	}
}

func setupTestStore(t *testing.T) (*storage.BadgerStore, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "raft-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	store, err := storage.NewStore(tmpDir)
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

func TestNewFSM(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	logger := zerolog.Nop()
	fsm := NewFSM(store, logger)

	if fsm == nil {
		t.Fatal("NewFSM returned nil")
	}
	if fsm.store != store {
		t.Error("FSM store not set correctly")
	}
}

func TestFSM_Snapshot(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	logger := zerolog.Nop()
	fsm := NewFSM(store, logger)

	// Create some test data
	job := &models.Job{
		ID:       "job-snap-1",
		Name:     "snapshot-test",
		Schedule: "* * * * *",
		Enabled:  true,
	}
	if err := store.CreateJob(job); err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	// Take snapshot
	snapshot, err := fsm.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	if snapshot == nil {
		t.Fatal("Snapshot returned nil")
	}
}

func TestFSMSnapshot_Release(t *testing.T) {
	snapshot := &FSMSnapshot{data: []byte("test data")}
	// Should not panic
	snapshot.Release()
}

func TestCommand_MarshalUnmarshal(t *testing.T) {
	original := &Command{
		Type:    CmdCreateJob,
		Payload: json.RawMessage(`{"id":"test"}`),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal command: %v", err)
	}

	var decoded Command
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal command: %v", err)
	}

	if decoded.Type != original.Type {
		t.Errorf("expected type %s, got %s", original.Type, decoded.Type)
	}
}

func TestCommandTypes(t *testing.T) {
	tests := []struct {
		cmdType  CommandType
		expected string
	}{
		{CmdCreateJob, "create_job"},
		{CmdUpdateJob, "update_job"},
		{CmdDeleteJob, "delete_job"},
		{CmdRecordExecution, "record_execution"},
		{CmdAcquireLock, "acquire_lock"},
		{CmdReleaseLock, "release_lock"},
	}

	for _, tt := range tests {
		t.Run(string(tt.cmdType), func(t *testing.T) {
			if string(tt.cmdType) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.cmdType)
			}
		})
	}
}
