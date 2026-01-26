package plugin

import (
	"context"
	"testing"
)

// mockPlugin is a test plugin implementation.
type mockPlugin struct {
	name    string
	closed  bool
	initErr error
}

func (m *mockPlugin) Metadata() Metadata {
	return Metadata{
		Name:    m.name,
		Version: "1.0.0",
		Type:    TypeExecutor,
	}
}

func (m *mockPlugin) Init(ctx context.Context, config map[string]interface{}) error {
	return m.initErr
}

func (m *mockPlugin) Close(ctx context.Context) error {
	m.closed = true
	return nil
}

func (m *mockPlugin) Health(ctx context.Context) error {
	return nil
}

// mockExecutor is a test executor implementation.
type mockExecutor struct {
	mockPlugin
}

func (m *mockExecutor) Execute(ctx context.Context, req *ExecutionRequest) (*ExecutionResult, error) {
	return &ExecutionResult{Success: true}, nil
}

// mockNotifier is a test notifier implementation.
type mockNotifier struct {
	mockPlugin
}

func (m *mockNotifier) Notify(ctx context.Context, notification *Notification) error {
	return nil
}

func TestRegistry(t *testing.T) {
	t.Run("NewRegistry", func(t *testing.T) {
		r := NewRegistry()
		if r == nil {
			t.Fatal("expected non-nil registry")
		}
	})

	t.Run("RegisterAndGetExecutor", func(t *testing.T) {
		r := NewRegistry()
		executor := &mockExecutor{mockPlugin: mockPlugin{name: "test-executor"}}

		if err := r.RegisterExecutor("test", executor); err != nil {
			t.Fatalf("RegisterExecutor failed: %v", err)
		}

		got, err := r.GetExecutor("test")
		if err != nil {
			t.Fatalf("GetExecutor failed: %v", err)
		}
		if got != executor {
			t.Error("expected same executor instance")
		}
	})

	t.Run("RegisterDuplicateExecutor", func(t *testing.T) {
		r := NewRegistry()
		executor := &mockExecutor{mockPlugin: mockPlugin{name: "test"}}

		if err := r.RegisterExecutor("test", executor); err != nil {
			t.Fatalf("first RegisterExecutor failed: %v", err)
		}

		err := r.RegisterExecutor("test", executor)
		if err == nil {
			t.Error("expected error for duplicate registration")
		}
	})

	t.Run("GetNonExistentExecutor", func(t *testing.T) {
		r := NewRegistry()
		_, err := r.GetExecutor("nonexistent")
		if err == nil {
			t.Error("expected error for non-existent executor")
		}
	})

	t.Run("RegisterAndGetNotifier", func(t *testing.T) {
		r := NewRegistry()
		notifier := &mockNotifier{mockPlugin: mockPlugin{name: "test-notifier"}}

		if err := r.RegisterNotifier("slack", notifier); err != nil {
			t.Fatalf("RegisterNotifier failed: %v", err)
		}

		got, err := r.GetNotifier("slack")
		if err != nil {
			t.Fatalf("GetNotifier failed: %v", err)
		}
		if got != notifier {
			t.Error("expected same notifier instance")
		}
	})

	t.Run("ListPlugins", func(t *testing.T) {
		r := NewRegistry()
		r.RegisterExecutor("exec1", &mockExecutor{mockPlugin: mockPlugin{name: "exec1"}})
		r.RegisterExecutor("exec2", &mockExecutor{mockPlugin: mockPlugin{name: "exec2"}})
		r.RegisterNotifier("notify1", &mockNotifier{mockPlugin: mockPlugin{name: "notify1"}})

		plugins := r.ListPlugins()

		if len(plugins[TypeExecutor]) != 2 {
			t.Errorf("expected 2 executors, got %d", len(plugins[TypeExecutor]))
		}
		if len(plugins[TypeNotifier]) != 1 {
			t.Errorf("expected 1 notifier, got %d", len(plugins[TypeNotifier]))
		}
	})

	t.Run("CloseAll", func(t *testing.T) {
		r := NewRegistry()
		exec := &mockExecutor{mockPlugin: mockPlugin{name: "exec"}}
		r.RegisterExecutor("exec", exec)

		if err := r.CloseAll(context.Background()); err != nil {
			t.Fatalf("CloseAll failed: %v", err)
		}

		if !exec.closed {
			t.Error("expected executor to be closed")
		}
	})
}

func TestExecutionRequest(t *testing.T) {
	req := &ExecutionRequest{
		JobID:       "job-1",
		JobName:     "Test Job",
		ExecutionID: "exec-1",
		Payload:     map[string]interface{}{"key": "value"},
		Metadata:    map[string]string{"trace_id": "abc123"},
	}

	if req.JobID != "job-1" {
		t.Errorf("expected job-1, got %s", req.JobID)
	}
	if req.Payload["key"] != "value" {
		t.Error("expected payload key=value")
	}
}

func TestExecutionResult(t *testing.T) {
	result := &ExecutionResult{
		Success:    true,
		StatusCode: 200,
		Response:   []byte(`{"status":"ok"}`),
	}

	if !result.Success {
		t.Error("expected success=true")
	}
	if result.StatusCode != 200 {
		t.Errorf("expected 200, got %d", result.StatusCode)
	}
}
