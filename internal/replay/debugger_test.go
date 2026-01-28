package replay

import (
	"context"
	"testing"
	"time"
)

// mockExecutor implements ExecutionExecutor for testing
type mockExecutor struct{}

func (m *mockExecutor) Execute(ctx context.Context, req *RequestSnapshot) (*ResponseSnapshot, error) {
	return &ResponseSnapshot{
		StatusCode: 200,
		Body:       `{"ok": true}`,
	}, nil
}

func TestNewDebugger(t *testing.T) {
	debugger := NewDebugger(&mockExecutor{})

	if debugger == nil {
		t.Fatal("expected debugger, got nil")
	}
	if debugger.snapshots == nil {
		t.Error("expected snapshots map to be initialized")
	}
}

func TestDebugger_CaptureExecution(t *testing.T) {
	debugger := NewDebugger(&mockExecutor{})

	snapshot := &ExecutionSnapshot{
		JobID:       "job-1",
		JobName:     "Test Job",
		ExecutionID: "exec-1",
		Status:      "success",
		StartedAt:   time.Now().Add(-time.Second),
		EndedAt:     time.Now(),
		Duration:    time.Second,
		Request: RequestSnapshot{
			Method: "POST",
			URL:    "https://example.com/webhook",
		},
		Response: ResponseSnapshot{
			StatusCode: 200,
			Body:       `{"ok": true}`,
		},
	}

	err := debugger.CaptureExecution(context.Background(), snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if snapshot.ID == "" {
		t.Error("expected snapshot ID to be set")
	}
}

func TestDebugger_GetSnapshot(t *testing.T) {
	debugger := NewDebugger(&mockExecutor{})

	snapshot := &ExecutionSnapshot{
		JobID:       "job-1",
		ExecutionID: "exec-1",
		Status:      "success",
	}

	debugger.CaptureExecution(context.Background(), snapshot)

	retrieved, err := debugger.GetSnapshot(context.Background(), snapshot.ExecutionID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if retrieved.ExecutionID != "exec-1" {
		t.Errorf("expected execution ID exec-1, got %s", retrieved.ExecutionID)
	}
}

func TestDebugger_GetSnapshot_NotFound(t *testing.T) {
	debugger := NewDebugger(&mockExecutor{})

	_, err := debugger.GetSnapshot(context.Background(), "nonexistent")
	if err != ErrExecutionNotFound {
		t.Errorf("expected ErrExecutionNotFound, got %v", err)
	}
}

func TestDebugger_ListSnapshots(t *testing.T) {
	debugger := NewDebugger(&mockExecutor{})

	for i := 0; i < 5; i++ {
		snapshot := &ExecutionSnapshot{
			JobID:       "job-1",
			ExecutionID: "exec-" + string(rune('0'+i)),
			Status:      "success",
		}
		debugger.CaptureExecution(context.Background(), snapshot)
	}

	snapshots, err := debugger.ListSnapshots(context.Background(), "job-1", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snapshots) != 5 {
		t.Errorf("expected 5 snapshots, got %d", len(snapshots))
	}
}

func TestDebugger_ListSnapshots_WithLimit(t *testing.T) {
	debugger := NewDebugger(&mockExecutor{})

	for i := 0; i < 10; i++ {
		snapshot := &ExecutionSnapshot{
			JobID:       "job-1",
			ExecutionID: "exec-" + string(rune('0'+i)),
			Status:      "success",
		}
		debugger.CaptureExecution(context.Background(), snapshot)
	}

	snapshots, _ := debugger.ListSnapshots(context.Background(), "job-1", 5)
	if len(snapshots) > 5 {
		t.Errorf("expected max 5 snapshots (limited), got %d", len(snapshots))
	}
}

func TestDebugger_GetTimeline(t *testing.T) {
	debugger := NewDebugger(&mockExecutor{})

	now := time.Now()
	snapshot := &ExecutionSnapshot{
		JobID:       "job-1",
		ExecutionID: "exec-1",
		Status:      "success",
		ScheduledAt: now.Add(-2 * time.Second),
		StartedAt:   now.Add(-time.Second),
		EndedAt:     now,
		Duration:    time.Second,
	}

	debugger.CaptureExecution(context.Background(), snapshot)

	timeline, err := debugger.GetTimeline(context.Background(), snapshot.ExecutionID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if timeline == nil {
		t.Error("expected timeline, got nil")
	}
}

func TestDebugger_Compare(t *testing.T) {
	debugger := NewDebugger(&mockExecutor{})

	snapshot1 := &ExecutionSnapshot{
		JobID:       "job-1",
		ExecutionID: "exec-1",
		Status:      "success",
		Response: ResponseSnapshot{
			StatusCode: 200,
			Body:       "OK",
		},
	}

	snapshot2 := &ExecutionSnapshot{
		JobID:       "job-1",
		ExecutionID: "exec-2",
		Status:      "failed",
		Response: ResponseSnapshot{
			StatusCode: 500,
			Body:       "Error",
		},
	}

	debugger.CaptureExecution(context.Background(), snapshot1)
	debugger.CaptureExecution(context.Background(), snapshot2)

	differences, err := debugger.Compare(context.Background(), snapshot1.ExecutionID, snapshot2.ExecutionID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(differences) == 0 {
		t.Error("expected differences between snapshots")
	}
}

func TestExecutionSnapshot_Fields(t *testing.T) {
	snapshot := ExecutionSnapshot{
		ID:          "snap-1",
		JobID:       "job-1",
		JobName:     "Test Job",
		ExecutionID: "exec-1",
		Status:      "success",
		Attempt:     1,
		MaxAttempts: 3,
	}

	if snapshot.Attempt != 1 {
		t.Errorf("expected attempt 1, got %d", snapshot.Attempt)
	}
	if snapshot.MaxAttempts != 3 {
		t.Errorf("expected max attempts 3, got %d", snapshot.MaxAttempts)
	}
}

func TestRequestSnapshot_Fields(t *testing.T) {
	req := RequestSnapshot{
		Method:  "POST",
		URL:     "https://example.com/webhook",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    `{"key": "value"}`,
	}

	if req.Method != "POST" {
		t.Errorf("expected method POST, got %s", req.Method)
	}
}

func TestResponseSnapshot_Fields(t *testing.T) {
	resp := ResponseSnapshot{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       `{"ok": true}`,
		BodySize:   12,
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestReplayRequest_Fields(t *testing.T) {
	req := ReplayRequest{
		ExecutionID: "exec-1",
		DryRun:      true,
		SkipRetries: true,
		Timeout:     30 * time.Second,
	}

	if !req.DryRun {
		t.Error("expected DryRun to be true")
	}
}

func TestDifference_Types(t *testing.T) {
	diff := Difference{
		Field:    "status",
		Original: "success",
		Replay:   "failed",
	}

	if diff.Field != "status" {
		t.Errorf("expected field status, got %s", diff.Field)
	}
}
