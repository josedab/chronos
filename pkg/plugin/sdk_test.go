package plugin

import (
	"context"
	"testing"
	"time"
)

func TestBasePlugin(t *testing.T) {
	bp := NewBasePlugin(Metadata{
		Name:    "test-plugin",
		Version: "1.0.0",
		Type:    TypeNotifier,
	})

	meta := bp.Metadata()
	if meta.Name != "test-plugin" {
		t.Errorf("expected name test-plugin, got %s", meta.Name)
	}

	err := bp.Init(context.Background(), map[string]interface{}{
		"key": "value",
		"num": 42,
		"flag": true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if bp.ConfigString("key", "") != "value" {
		t.Error("expected config string 'value'")
	}
	if bp.ConfigString("missing", "default") != "default" {
		t.Error("expected default string")
	}
	if bp.ConfigInt("num", 0) != 42 {
		t.Error("expected config int 42")
	}
	if bp.ConfigBool("flag", false) != true {
		t.Error("expected config bool true")
	}

	if err := bp.Health(context.Background()); err != nil {
		t.Errorf("unexpected health error: %v", err)
	}
	if err := bp.Close(context.Background()); err != nil {
		t.Errorf("unexpected close error: %v", err)
	}
}

func TestPluginManager_Register(t *testing.T) {
	pm := NewPluginManager()

	notifier := NewLogNotifier()
	err := pm.Register(notifier)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Duplicate should fail
	err = pm.Register(notifier)
	if err == nil {
		t.Error("expected error for duplicate registration")
	}

	// List plugins
	metas := pm.ListPlugins()
	if len(metas) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(metas))
	}
	if metas[0].Name != "log-notifier" {
		t.Errorf("expected log-notifier, got %s", metas[0].Name)
	}
}

func TestPluginManager_TypeSpecificLookup(t *testing.T) {
	pm := NewPluginManager()

	pm.Register(NewLogNotifier())
	pm.Register(NewNoopExecutor())

	// Notifier lookup
	n, ok := pm.GetNotifier("log-notifier")
	if !ok || n == nil {
		t.Fatal("expected log-notifier to be found")
	}

	// Executor lookup
	e, ok := pm.GetExecutor("noop-executor")
	if !ok || e == nil {
		t.Fatal("expected noop-executor to be found")
	}

	// Wrong type lookup
	_, ok = pm.GetExecutor("log-notifier")
	if ok {
		t.Error("log-notifier should not be found as executor")
	}
}

func TestPluginManager_InitAll(t *testing.T) {
	pm := NewPluginManager()

	pm.Register(NewLogNotifier())
	pm.Register(NewNoopExecutor())

	configs := map[string]map[string]interface{}{
		"log-notifier":  {"level": "debug"},
		"noop-executor": {"timeout": 30},
	}

	err := pm.InitAll(context.Background(), configs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPluginManager_CloseAll(t *testing.T) {
	pm := NewPluginManager()
	pm.Register(NewLogNotifier())

	err := pm.CloseAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPluginManager_HealthCheckAll(t *testing.T) {
	pm := NewPluginManager()
	pm.Register(NewLogNotifier())
	pm.Register(NewNoopExecutor())

	results := pm.HealthCheckAll(context.Background())
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for name, err := range results {
		if err != nil {
			t.Errorf("plugin %s health check failed: %v", name, err)
		}
	}
}

func TestLogNotifier(t *testing.T) {
	notifier := NewLogNotifier()

	err := notifier.Notify(context.Background(), &Notification{
		JobID:   "job-1",
		JobName: "Test Job",
		Status:  "failed",
		Message: "Job execution failed",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	notifications := notifier.Notifications()
	if len(notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifications))
	}
	if notifications[0].Status != "failed" {
		t.Errorf("expected status failed, got %s", notifications[0].Status)
	}
	if notifications[0].Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestNoopExecutor(t *testing.T) {
	executor := NewNoopExecutor()

	result, err := executor.Execute(context.Background(), &ExecutionRequest{
		JobID:       "job-1",
		JobName:     "Test Job",
		ExecutionID: "exec-1",
		Timeout:     5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if result.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", result.StatusCode)
	}
}

func TestPluginManager_EmptyName(t *testing.T) {
	pm := NewPluginManager()
	bp := &NoopExecutor{
		BasePlugin: NewBasePlugin(Metadata{
			Version: "1.0.0",
			Type:    TypeExecutor,
		}),
	}

	err := pm.Register(bp)
	if err == nil {
		t.Error("expected error for empty plugin name")
	}
}
