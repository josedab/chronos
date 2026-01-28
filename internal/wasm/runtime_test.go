package wasm

import (
	"context"
	"testing"
	"time"
)

func TestNewRuntime(t *testing.T) {
	runtime := NewRuntime(DefaultRuntimeConfig())

	if runtime == nil {
		t.Fatal("expected runtime, got nil")
	}
	if runtime.plugins == nil {
		t.Error("expected plugins map to be initialized")
	}
}

func TestDefaultPluginConfig(t *testing.T) {
	cfg := DefaultPluginConfig()

	if cfg.MemoryLimitMB != 64 {
		t.Errorf("expected memory limit 64MB, got %d", cfg.MemoryLimitMB)
	}
	if cfg.TimeoutSeconds != 30 {
		t.Errorf("expected timeout 30s, got %d", cfg.TimeoutSeconds)
	}
	if cfg.MaxInstances != 10 {
		t.Errorf("expected max instances 10, got %d", cfg.MaxInstances)
	}
	if cfg.AllowNetwork {
		t.Error("expected AllowNetwork to be false by default")
	}
}

func TestDefaultRuntimeConfig(t *testing.T) {
	cfg := DefaultRuntimeConfig()

	if cfg.MaxPlugins != 100 {
		t.Errorf("expected max plugins 100, got %d", cfg.MaxPlugins)
	}
	if cfg.DefaultTimeout != 30*time.Second {
		t.Errorf("expected default timeout 30s, got %v", cfg.DefaultTimeout)
	}
}

func TestRuntime_LoadPlugin(t *testing.T) {
	runtime := NewRuntime(DefaultRuntimeConfig())

	plugin := &Plugin{
		ID:          "plugin-1",
		Name:        "Test Plugin",
		Version:     "1.0.0",
		Type:        PluginTypeExecutor,
		Description: "A test plugin",
		Config:      DefaultPluginConfig(),
	}

	// Valid WASM magic number: 0x00 0x61 0x73 0x6D (followed by version 0x01 0x00 0x00 0x00)
	wasmBytes := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}

	err := runtime.LoadPlugin(context.Background(), plugin, wasmBytes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !plugin.Enabled {
		t.Error("expected plugin to be enabled")
	}
	if plugin.LoadedAt == nil {
		t.Error("expected LoadedAt to be set")
	}
}

func TestRuntime_LoadPlugin_Duplicate(t *testing.T) {
	runtime := NewRuntime(DefaultRuntimeConfig())

	plugin := &Plugin{
		ID:   "plugin-1",
		Name: "Test Plugin",
		Type: PluginTypeExecutor,
	}

	wasmBytes := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}

	runtime.LoadPlugin(context.Background(), plugin, wasmBytes)
	err := runtime.LoadPlugin(context.Background(), plugin, wasmBytes)

	if err != ErrPluginExists {
		t.Errorf("expected ErrPluginExists, got %v", err)
	}
}

func TestRuntime_LoadPlugin_Invalid(t *testing.T) {
	runtime := NewRuntime(DefaultRuntimeConfig())

	tests := []struct {
		name   string
		plugin *Plugin
	}{
		{"missing ID", &Plugin{Name: "test", Type: PluginTypeExecutor}},
		{"missing Name", &Plugin{ID: "p-1", Type: PluginTypeExecutor}},
	}

	wasmBytes := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runtime.LoadPlugin(context.Background(), tt.plugin, wasmBytes)
			if err != ErrInvalidPlugin {
				t.Errorf("expected ErrInvalidPlugin, got %v", err)
			}
		})
	}
}

func TestRuntime_LoadPlugin_InvalidWASM(t *testing.T) {
	runtime := NewRuntime(DefaultRuntimeConfig())

	plugin := &Plugin{
		ID:   "plugin-1",
		Name: "Test Plugin",
		Type: PluginTypeExecutor,
	}

	// Invalid WASM bytes
	invalidWasm := []byte{0xFF, 0xFF, 0xFF, 0xFF}

	err := runtime.LoadPlugin(context.Background(), plugin, invalidWasm)
	if err != ErrInvalidPlugin {
		t.Errorf("expected ErrInvalidPlugin, got %v", err)
	}
}

func TestRuntime_GetPlugin(t *testing.T) {
	runtime := NewRuntime(DefaultRuntimeConfig())

	plugin := &Plugin{
		ID:   "plugin-1",
		Name: "Test Plugin",
		Type: PluginTypeExecutor,
	}

	wasmBytes := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	runtime.LoadPlugin(context.Background(), plugin, wasmBytes)

	retrieved, err := runtime.GetPlugin(context.Background(), "plugin-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if retrieved.Name != "Test Plugin" {
		t.Errorf("expected name 'Test Plugin', got %s", retrieved.Name)
	}
}

func TestRuntime_GetPlugin_NotFound(t *testing.T) {
	runtime := NewRuntime(DefaultRuntimeConfig())

	_, err := runtime.GetPlugin(context.Background(), "nonexistent")
	if err != ErrPluginNotFound {
		t.Errorf("expected ErrPluginNotFound, got %v", err)
	}
}

func TestRuntime_ListPlugins(t *testing.T) {
	runtime := NewRuntime(DefaultRuntimeConfig())

	wasmBytes := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}

	for i := 0; i < 5; i++ {
		plugin := &Plugin{
			ID:   "plugin-" + string(rune('0'+i)),
			Name: "Plugin " + string(rune('A'+i)),
			Type: PluginTypeExecutor,
		}
		runtime.LoadPlugin(context.Background(), plugin, wasmBytes)
	}

	plugins, err := runtime.ListPlugins(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plugins) != 5 {
		t.Errorf("expected 5 plugins, got %d", len(plugins))
	}
}

func TestRuntime_ListPlugins_ByType(t *testing.T) {
	runtime := NewRuntime(DefaultRuntimeConfig())

	wasmBytes := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}

	runtime.LoadPlugin(context.Background(), &Plugin{
		ID: "p-1", Name: "Executor 1", Type: PluginTypeExecutor,
	}, wasmBytes)
	runtime.LoadPlugin(context.Background(), &Plugin{
		ID: "p-2", Name: "Transform 1", Type: PluginTypeTransform,
	}, wasmBytes)
	runtime.LoadPlugin(context.Background(), &Plugin{
		ID: "p-3", Name: "Executor 2", Type: PluginTypeExecutor,
	}, wasmBytes)

	executors, err := runtime.ListPlugins(context.Background(), PluginTypeExecutor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(executors) != 2 {
		t.Errorf("expected 2 executors, got %d", len(executors))
	}
}

func TestRuntime_UnloadPlugin(t *testing.T) {
	runtime := NewRuntime(DefaultRuntimeConfig())

	plugin := &Plugin{
		ID:   "plugin-1",
		Name: "Test Plugin",
		Type: PluginTypeExecutor,
	}

	wasmBytes := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	runtime.LoadPlugin(context.Background(), plugin, wasmBytes)

	err := runtime.UnloadPlugin(context.Background(), "plugin-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = runtime.GetPlugin(context.Background(), "plugin-1")
	if err != ErrPluginNotFound {
		t.Error("expected plugin to be unloaded")
	}
}

func TestPluginTypes(t *testing.T) {
	types := []PluginType{
		PluginTypeExecutor,
		PluginTypeTransform,
		PluginTypeFilter,
		PluginTypeValidation,
	}

	for _, pt := range types {
		if pt == "" {
			t.Error("plugin type should not be empty")
		}
	}
}

func TestPlugin_Fields(t *testing.T) {
	now := time.Now()
	plugin := Plugin{
		ID:          "plugin-1",
		Name:        "Test Plugin",
		Version:     "1.0.0",
		Type:        PluginTypeExecutor,
		Description: "A test plugin",
		Author:      "Test Author",
		ModuleHash:  "sha256:abc123",
		ModuleSize:  1024,
		Enabled:     true,
		UsageCount:  100,
		LoadedAt:    &now,
		LastUsedAt:  &now,
	}

	if plugin.ModuleSize != 1024 {
		t.Errorf("expected module size 1024, got %d", plugin.ModuleSize)
	}
	if plugin.UsageCount != 100 {
		t.Errorf("expected usage count 100, got %d", plugin.UsageCount)
	}
}

func TestPluginConfig_Fields(t *testing.T) {
	config := PluginConfig{
		MemoryLimitMB:   128,
		TimeoutSeconds:  60,
		MaxInstances:    20,
		AllowNetwork:    true,
		AllowFileSystem: false,
		AllowEnv:        true,
		Env:             map[string]string{"DEBUG": "true"},
	}

	if config.MemoryLimitMB != 128 {
		t.Errorf("expected memory limit 128MB, got %d", config.MemoryLimitMB)
	}
	if !config.AllowNetwork {
		t.Error("expected AllowNetwork to be true")
	}
}

func TestExecutionContext_Fields(t *testing.T) {
	ctx := ExecutionContext{
		JobID:       "job-1",
		JobName:     "Test Job",
		ExecutionID: "exec-1",
		Input:       map[string]interface{}{"key": "value"},
		Timeout:     30 * time.Second,
	}

	if ctx.JobID != "job-1" {
		t.Errorf("expected job ID job-1, got %s", ctx.JobID)
	}
}

func TestExecutionResult_Fields(t *testing.T) {
	result := ExecutionResult{
		Output:     map[string]interface{}{"result": "ok"},
		Logs:       []LogEntry{{Level: "info", Message: "log1"}, {Level: "info", Message: "log2"}},
		Duration:   time.Second,
		MemoryUsed: 1024 * 1024,
	}

	if result.Duration != time.Second {
		t.Errorf("expected duration 1s, got %v", result.Duration)
	}
}

func TestHostFunction_Fields(t *testing.T) {
	hf := HostFunction{
		Name:        "log",
		Description: "Logs a message",
	}

	if hf.Name != "log" {
		t.Errorf("expected name log, got %s", hf.Name)
	}
}
