// Package wasm provides WebAssembly plugin runtime for Chronos.
package wasm

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"
)

// Common errors.
var (
	ErrPluginNotFound      = errors.New("plugin not found")
	ErrPluginExists        = errors.New("plugin already exists")
	ErrInvalidPlugin       = errors.New("invalid plugin")
	ErrPluginLoadFailed    = errors.New("failed to load plugin")
	ErrPluginExecutionFailed = errors.New("plugin execution failed")
	ErrPluginTimeout       = errors.New("plugin execution timeout")
	ErrPluginPanic         = errors.New("plugin panicked")
)

// PluginType represents the type of WASM plugin.
type PluginType string

const (
	PluginTypeExecutor   PluginType = "executor"
	PluginTypeTransform  PluginType = "transform"
	PluginTypeFilter     PluginType = "filter"
	PluginTypeValidation PluginType = "validation"
)

// Plugin represents a WASM plugin.
type Plugin struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Type        PluginType        `json:"type"`
	Description string            `json:"description,omitempty"`
	Author      string            `json:"author,omitempty"`
	
	// WASM module
	ModulePath  string            `json:"-"`
	ModuleHash  string            `json:"module_hash"`
	ModuleSize  int64             `json:"module_size"`
	
	// Configuration
	Config      PluginConfig      `json:"config"`
	
	// Metadata
	Enabled     bool              `json:"enabled"`
	LoadedAt    *time.Time        `json:"loaded_at,omitempty"`
	LastUsedAt  *time.Time        `json:"last_used_at,omitempty"`
	UsageCount  int64             `json:"usage_count"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// PluginConfig contains plugin configuration.
type PluginConfig struct {
	// Resource limits
	MemoryLimitMB   int           `json:"memory_limit_mb"`
	TimeoutSeconds  int           `json:"timeout_seconds"`
	MaxInstances    int           `json:"max_instances"`
	
	// Permissions
	AllowNetwork    bool          `json:"allow_network"`
	AllowFileSystem bool          `json:"allow_filesystem"`
	AllowEnv        bool          `json:"allow_env"`
	
	// Environment
	Env             map[string]string `json:"env,omitempty"`
}

// DefaultPluginConfig returns default plugin configuration.
func DefaultPluginConfig() PluginConfig {
	return PluginConfig{
		MemoryLimitMB:  64,
		TimeoutSeconds: 30,
		MaxInstances:   10,
		AllowNetwork:   false,
		AllowFileSystem: false,
		AllowEnv:       false,
	}
}

// ExecutionContext provides context to WASM plugins.
type ExecutionContext struct {
	// Input
	JobID       string                 `json:"job_id"`
	JobName     string                 `json:"job_name"`
	ExecutionID string                 `json:"execution_id"`
	Input       map[string]interface{} `json:"input"`
	
	// Environment
	Env         map[string]string      `json:"env,omitempty"`
	Secrets     map[string]string      `json:"-"`
	
	// Configuration
	Timeout     time.Duration          `json:"timeout"`
	
	// Tracing
	TraceID     string                 `json:"trace_id,omitempty"`
	SpanID      string                 `json:"span_id,omitempty"`
}

// ExecutionResult is the result from WASM plugin execution.
type ExecutionResult struct {
	Success     bool                   `json:"success"`
	Output      map[string]interface{} `json:"output,omitempty"`
	Error       string                 `json:"error,omitempty"`
	
	// Metrics
	Duration    time.Duration          `json:"duration"`
	MemoryUsed  int64                  `json:"memory_used_bytes"`
	
	// Logs
	Logs        []LogEntry             `json:"logs,omitempty"`
}

// LogEntry represents a log entry from plugin execution.
type LogEntry struct {
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// HostFunction represents a function exposed to WASM plugins.
type HostFunction struct {
	Name        string                     `json:"name"`
	Description string                     `json:"description"`
	Handler     func(ctx context.Context, args []interface{}) (interface{}, error)
}

// ModuleInstance represents a running WASM module instance.
type ModuleInstance struct {
	ID          string
	PluginID    string
	StartedAt   time.Time
	MemoryUsed  int64
	Status      string
}

// Runtime manages WASM plugin execution.
type Runtime struct {
	mu            sync.RWMutex
	plugins       map[string]*Plugin
	modules       map[string][]byte // pluginID -> WASM bytes
	instances     map[string]*ModuleInstance
	hostFunctions map[string]HostFunction
	
	// Configuration
	config        RuntimeConfig
}

// RuntimeConfig contains runtime configuration.
type RuntimeConfig struct {
	MaxPlugins          int
	MaxInstances        int
	DefaultMemoryLimit  int64
	DefaultTimeout      time.Duration
	EnableNetworking    bool
	EnableFileSystem    bool
}

// DefaultRuntimeConfig returns default runtime configuration.
func DefaultRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		MaxPlugins:         100,
		MaxInstances:       1000,
		DefaultMemoryLimit: 64 * 1024 * 1024, // 64MB
		DefaultTimeout:     30 * time.Second,
		EnableNetworking:   false,
		EnableFileSystem:   false,
	}
}

// NewRuntime creates a new WASM runtime.
func NewRuntime(config RuntimeConfig) *Runtime {
	r := &Runtime{
		plugins:       make(map[string]*Plugin),
		modules:       make(map[string][]byte),
		instances:     make(map[string]*ModuleInstance),
		hostFunctions: make(map[string]HostFunction),
		config:        config,
	}

	// Register built-in host functions
	r.registerBuiltinFunctions()

	return r
}

// registerBuiltinFunctions registers functions available to WASM plugins.
func (r *Runtime) registerBuiltinFunctions() {
	r.hostFunctions["log"] = HostFunction{
		Name:        "log",
		Description: "Log a message",
		Handler: func(ctx context.Context, args []interface{}) (interface{}, error) {
			// Logging implementation
			return nil, nil
		},
	}

	r.hostFunctions["http_get"] = HostFunction{
		Name:        "http_get",
		Description: "Make an HTTP GET request",
		Handler: func(ctx context.Context, args []interface{}) (interface{}, error) {
			if !r.config.EnableNetworking {
				return nil, errors.New("networking disabled")
			}
			// HTTP implementation would go here
			return nil, nil
		},
	}

	r.hostFunctions["http_post"] = HostFunction{
		Name:        "http_post",
		Description: "Make an HTTP POST request",
		Handler: func(ctx context.Context, args []interface{}) (interface{}, error) {
			if !r.config.EnableNetworking {
				return nil, errors.New("networking disabled")
			}
			// HTTP implementation would go here
			return nil, nil
		},
	}

	r.hostFunctions["env_get"] = HostFunction{
		Name:        "env_get",
		Description: "Get an environment variable",
		Handler: func(ctx context.Context, args []interface{}) (interface{}, error) {
			// Environment variable access
			return nil, nil
		},
	}

	r.hostFunctions["read_file"] = HostFunction{
		Name:        "read_file",
		Description: "Read a file",
		Handler: func(ctx context.Context, args []interface{}) (interface{}, error) {
			if !r.config.EnableFileSystem {
				return nil, errors.New("filesystem access disabled")
			}
			// File read implementation
			return nil, nil
		},
	}
}

// LoadPlugin loads a WASM plugin from bytes.
func (r *Runtime) LoadPlugin(ctx context.Context, plugin *Plugin, wasmBytes []byte) error {
	if plugin.ID == "" || plugin.Name == "" {
		return ErrInvalidPlugin
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.plugins) >= r.config.MaxPlugins {
		return errors.New("maximum plugins reached")
	}

	if _, exists := r.plugins[plugin.ID]; exists {
		return ErrPluginExists
	}

	// Validate WASM bytes (check magic number)
	if len(wasmBytes) < 8 || wasmBytes[0] != 0x00 || wasmBytes[1] != 0x61 || 
		wasmBytes[2] != 0x73 || wasmBytes[3] != 0x6d {
		return ErrInvalidPlugin
	}

	// Store module bytes
	r.modules[plugin.ID] = wasmBytes
	
	// Set defaults
	if plugin.Config.MemoryLimitMB == 0 {
		plugin.Config = DefaultPluginConfig()
	}
	
	now := time.Now()
	plugin.LoadedAt = &now
	plugin.ModuleSize = int64(len(wasmBytes))
	plugin.CreatedAt = now
	plugin.UpdatedAt = now
	plugin.Enabled = true

	r.plugins[plugin.ID] = plugin

	return nil
}

// LoadPluginFromReader loads a WASM plugin from a reader.
func (r *Runtime) LoadPluginFromReader(ctx context.Context, plugin *Plugin, reader io.Reader) error {
	wasmBytes, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	return r.LoadPlugin(ctx, plugin, wasmBytes)
}

// UnloadPlugin unloads a plugin.
func (r *Runtime) UnloadPlugin(ctx context.Context, pluginID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.plugins[pluginID]; !ok {
		return ErrPluginNotFound
	}

	// Stop all instances
	for id, inst := range r.instances {
		if inst.PluginID == pluginID {
			delete(r.instances, id)
		}
	}

	delete(r.modules, pluginID)
	delete(r.plugins, pluginID)

	return nil
}

// GetPlugin retrieves a plugin by ID.
func (r *Runtime) GetPlugin(ctx context.Context, pluginID string) (*Plugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugin, ok := r.plugins[pluginID]
	if !ok {
		return nil, ErrPluginNotFound
	}
	return plugin, nil
}

// ListPlugins lists all plugins.
func (r *Runtime) ListPlugins(ctx context.Context, pluginType PluginType) ([]*Plugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*Plugin
	for _, p := range r.plugins {
		if pluginType == "" || p.Type == pluginType {
			result = append(result, p)
		}
	}
	return result, nil
}

// Execute executes a plugin.
func (r *Runtime) Execute(ctx context.Context, pluginID string, execCtx *ExecutionContext) (*ExecutionResult, error) {
	r.mu.Lock()
	plugin, ok := r.plugins[pluginID]
	if !ok {
		r.mu.Unlock()
		return nil, ErrPluginNotFound
	}

	if !plugin.Enabled {
		r.mu.Unlock()
		return nil, errors.New("plugin is disabled")
	}

	wasmBytes, ok := r.modules[pluginID]
	if !ok {
		r.mu.Unlock()
		return nil, ErrPluginLoadFailed
	}

	// Create instance
	instanceID := generateInstanceID()
	instance := &ModuleInstance{
		ID:        instanceID,
		PluginID:  pluginID,
		StartedAt: time.Now(),
		Status:    "running",
	}
	r.instances[instanceID] = instance

	// Update usage
	now := time.Now()
	plugin.LastUsedAt = &now
	plugin.UsageCount++

	r.mu.Unlock()

	// Execute with timeout
	timeout := time.Duration(plugin.Config.TimeoutSeconds) * time.Second
	if execCtx.Timeout > 0 {
		timeout = execCtx.Timeout
	}

	execCtx2, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result := r.executeWASM(execCtx2, wasmBytes, execCtx, plugin.Config)

	// Clean up instance
	r.mu.Lock()
	delete(r.instances, instanceID)
	r.mu.Unlock()

	return result, nil
}

// executeWASM executes the WASM module.
// In a real implementation, this would use wazero or wasmer.
func (r *Runtime) executeWASM(ctx context.Context, wasmBytes []byte, execCtx *ExecutionContext, config PluginConfig) *ExecutionResult {
	start := time.Now()

	// Simulated WASM execution
	// In production, use wazero:
	// runtime := wazero.NewRuntime(ctx)
	// module, _ := runtime.Instantiate(ctx, wasmBytes)
	// result, _ := module.ExportedFunction("execute").Call(ctx, ...)

	result := &ExecutionResult{
		Success:    true,
		Duration:   time.Since(start),
		MemoryUsed: 1024 * 1024, // 1MB simulated
		Output: map[string]interface{}{
			"status":  "executed",
			"job_id":  execCtx.JobID,
			"message": "WASM plugin executed successfully",
		},
		Logs: []LogEntry{
			{Level: "info", Message: "Plugin started", Timestamp: start},
			{Level: "info", Message: "Plugin completed", Timestamp: time.Now()},
		},
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		result.Success = false
		result.Error = ErrPluginTimeout.Error()
	default:
	}

	return result
}

// GetInstances returns running instances.
func (r *Runtime) GetInstances(ctx context.Context) ([]*ModuleInstance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*ModuleInstance, 0, len(r.instances))
	for _, inst := range r.instances {
		result = append(result, inst)
	}
	return result, nil
}

// StopInstance stops a running instance.
func (r *Runtime) StopInstance(ctx context.Context, instanceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.instances[instanceID]; !ok {
		return errors.New("instance not found")
	}

	delete(r.instances, instanceID)
	return nil
}

// GetHostFunctions returns available host functions.
func (r *Runtime) GetHostFunctions() []HostFunction {
	result := make([]HostFunction, 0, len(r.hostFunctions))
	for _, f := range r.hostFunctions {
		result = append(result, f)
	}
	return result
}

// RegisterHostFunction registers a custom host function.
func (r *Runtime) RegisterHostFunction(name string, fn HostFunction) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hostFunctions[name] = fn
}

// Validate validates a WASM module without loading it.
func (r *Runtime) Validate(ctx context.Context, wasmBytes []byte) error {
	// Check magic number
	if len(wasmBytes) < 8 {
		return errors.New("module too small")
	}

	// WASM magic number: \0asm
	if wasmBytes[0] != 0x00 || wasmBytes[1] != 0x61 || 
		wasmBytes[2] != 0x73 || wasmBytes[3] != 0x6d {
		return errors.New("invalid WASM magic number")
	}

	// Check version (should be 1)
	version := wasmBytes[4]
	if version != 0x01 {
		return errors.New("unsupported WASM version")
	}

	return nil
}

// Stats returns runtime statistics.
func (r *Runtime) Stats() RuntimeStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var totalUsage int64
	var totalMemory int64

	for _, p := range r.plugins {
		totalUsage += p.UsageCount
	}

	for _, inst := range r.instances {
		totalMemory += inst.MemoryUsed
	}

	return RuntimeStats{
		PluginCount:     len(r.plugins),
		InstanceCount:   len(r.instances),
		TotalUsage:      totalUsage,
		TotalMemoryUsed: totalMemory,
	}
}

// RuntimeStats contains runtime statistics.
type RuntimeStats struct {
	PluginCount     int   `json:"plugin_count"`
	InstanceCount   int   `json:"instance_count"`
	TotalUsage      int64 `json:"total_usage"`
	TotalMemoryUsed int64 `json:"total_memory_used"`
}

// generateInstanceID generates a unique instance ID.
func generateInstanceID() string {
	return time.Now().Format("20060102150405.000000")
}
