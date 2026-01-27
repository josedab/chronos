package models

import (
	"testing"
	"time"
)

func TestExecutionStatus_IsTerminal(t *testing.T) {
	tests := []struct {
		status   ExecutionStatus
		terminal bool
	}{
		{ExecutionPending, false},
		{ExecutionRunning, false},
		{ExecutionSuccess, true},
		{ExecutionFailed, true},
		{ExecutionSkipped, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if tt.status.IsTerminal() != tt.terminal {
				t.Errorf("expected IsTerminal=%v for %s", tt.terminal, tt.status)
			}
		})
	}
}

func TestExecutionStatus_Values(t *testing.T) {
	tests := []struct {
		status   ExecutionStatus
		expected string
	}{
		{ExecutionPending, "pending"},
		{ExecutionRunning, "running"},
		{ExecutionSuccess, "success"},
		{ExecutionFailed, "failed"},
		{ExecutionSkipped, "skipped"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.status)
		}
	}
}

func TestExecution_Fields(t *testing.T) {
	now := time.Now()
	completed := now.Add(time.Second)

	exec := &Execution{
		ID:            "exec-123",
		JobID:         "job-456",
		JobName:       "test-job",
		ScheduledTime: now,
		StartedAt:     now,
		CompletedAt:   &completed,
		Status:        ExecutionSuccess,
		Attempts:      1,
		StatusCode:    200,
		Response:      "OK",
		Duration:      time.Second,
		NodeID:        "node-1",
	}

	if exec.ID != "exec-123" {
		t.Errorf("ID mismatch")
	}
	if exec.JobID != "job-456" {
		t.Errorf("JobID mismatch")
	}
	if exec.Status != ExecutionSuccess {
		t.Errorf("Status mismatch")
	}
	if !exec.Status.IsTerminal() {
		t.Errorf("expected terminal status")
	}
	if exec.Attempts != 1 {
		t.Errorf("Attempts mismatch")
	}
	if exec.StatusCode != 200 {
		t.Errorf("StatusCode mismatch")
	}
}

func TestExecutionResult(t *testing.T) {
	result := &ExecutionResult{
		Success:    true,
		StatusCode: 200,
		Response:   "OK",
		Error:      "",
	}

	if !result.Success {
		t.Error("expected success")
	}
	if result.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", result.StatusCode)
	}

	failedResult := &ExecutionResult{
		Success:    false,
		StatusCode: 500,
		Error:      "Internal Server Error",
	}

	if failedResult.Success {
		t.Error("expected failure")
	}
}

func TestScheduleState(t *testing.T) {
	now := time.Now()
	state := &ScheduleState{
		JobID:       "job-123",
		NextRun:     now.Add(time.Hour),
		LastRun:     now,
		MissedCount: 0,
	}

	if state.JobID != "job-123" {
		t.Errorf("JobID mismatch")
	}
	if state.NextRun.Before(now) {
		t.Error("NextRun should be in the future")
	}
	if state.MissedCount != 0 {
		t.Errorf("expected 0 missed, got %d", state.MissedCount)
	}
}
