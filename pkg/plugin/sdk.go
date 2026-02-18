// Package plugin provides the Plugin SDK for extending Chronos.
//
// # Plugin Types
//
// Chronos supports five plugin types:
//   - Executor: Execute jobs via custom protocols (gRPC, Lambda, etc.)
//   - Trigger: Trigger jobs from external events (Kafka, SQS, webhooks)
//   - Notifier: Send notifications (Slack, PagerDuty, email)
//   - SecretProvider: Provide secrets (Vault, AWS SM, GCP SM)
//   - Storage: Provide custom storage backends
//
// # Creating a Plugin
//
// Implement the appropriate interface and register it:
//
//	type MyNotifier struct { plugin.BasePlugin }
//	func (n *MyNotifier) Notify(ctx context.Context, notif *plugin.Notification) error { ... }
//
//	func init() {
//	    plugin.MustRegister(&MyNotifier{
//	        BasePlugin: plugin.NewBasePlugin(plugin.Metadata{
//	            Name: "my-notifier", Version: "1.0.0", Type: plugin.TypeNotifier,
//	        }),
//	    })
//	}
package plugin

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// BasePlugin provides a default implementation of the Plugin interface.
// Embed this in your plugin struct to satisfy the interface.
type BasePlugin struct {
	meta   Metadata
	config map[string]interface{}
}

// NewBasePlugin creates a new BasePlugin with the given metadata.
func NewBasePlugin(meta Metadata) BasePlugin {
	return BasePlugin{meta: meta}
}

func (b *BasePlugin) Metadata() Metadata                                       { return b.meta }
func (b *BasePlugin) Init(_ context.Context, config map[string]interface{}) error {
	b.config = config
	return nil
}
func (b *BasePlugin) Close(_ context.Context) error                            { return nil }
func (b *BasePlugin) Health(_ context.Context) error                           { return nil }

// Config returns the plugin configuration.
func (b *BasePlugin) Config() map[string]interface{} { return b.config }

// ConfigString returns a string config value or default.
func (b *BasePlugin) ConfigString(key, defaultVal string) string {
	if v, ok := b.config[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultVal
}

// ConfigInt returns an int config value or default.
func (b *BasePlugin) ConfigInt(key string, defaultVal int) int {
	if v, ok := b.config[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case float64:
			return int(n)
		}
	}
	return defaultVal
}

// ConfigBool returns a bool config value or default.
func (b *BasePlugin) ConfigBool(key string, defaultVal bool) bool {
	if v, ok := b.config[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return defaultVal
}

// PluginManager manages the lifecycle of all registered plugins.
type PluginManager struct {
	mu        sync.RWMutex
	plugins   map[string]Plugin
	executors map[string]Executor
	triggers  map[string]Trigger
	notifiers map[string]Notifier
	secrets   map[string]SecretProvider
	storages  map[string]Storage
}

// NewPluginManager creates a new plugin manager.
func NewPluginManager() *PluginManager {
	return &PluginManager{
		plugins:   make(map[string]Plugin),
		executors: make(map[string]Executor),
		triggers:  make(map[string]Trigger),
		notifiers: make(map[string]Notifier),
		secrets:   make(map[string]SecretProvider),
		storages:  make(map[string]Storage),
	}
}

// Register adds a plugin to the manager.
func (pm *PluginManager) Register(p Plugin) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	meta := p.Metadata()
	if meta.Name == "" {
		return fmt.Errorf("plugin name is required")
	}

	key := meta.Name
	if _, exists := pm.plugins[key]; exists {
		return fmt.Errorf("plugin already registered: %s", key)
	}

	pm.plugins[key] = p

	// Type-specific registration
	switch meta.Type {
	case TypeExecutor:
		if e, ok := p.(Executor); ok {
			pm.executors[key] = e
		} else {
			return fmt.Errorf("plugin %s declares type executor but doesn't implement Executor interface", key)
		}
	case TypeTrigger:
		if t, ok := p.(Trigger); ok {
			pm.triggers[key] = t
		} else {
			return fmt.Errorf("plugin %s declares type trigger but doesn't implement Trigger interface", key)
		}
	case TypeNotifier:
		if n, ok := p.(Notifier); ok {
			pm.notifiers[key] = n
		} else {
			return fmt.Errorf("plugin %s declares type notifier but doesn't implement Notifier interface", key)
		}
	case TypeSecretProvider:
		if s, ok := p.(SecretProvider); ok {
			pm.secrets[key] = s
		} else {
			return fmt.Errorf("plugin %s declares type secret_provider but doesn't implement SecretProvider", key)
		}
	case TypeStorage:
		if s, ok := p.(Storage); ok {
			pm.storages[key] = s
		} else {
			return fmt.Errorf("plugin %s declares type storage but doesn't implement Storage", key)
		}
	}

	return nil
}

// InitAll initializes all registered plugins with their configs.
func (pm *PluginManager) InitAll(ctx context.Context, configs map[string]map[string]interface{}) error {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for name, p := range pm.plugins {
		cfg := configs[name]
		if err := p.Init(ctx, cfg); err != nil {
			return fmt.Errorf("initializing plugin %s: %w", name, err)
		}
	}
	return nil
}

// CloseAll closes all registered plugins.
func (pm *PluginManager) CloseAll(ctx context.Context) error {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var firstErr error
	for name, p := range pm.plugins {
		if err := p.Close(ctx); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("closing plugin %s: %w", name, err)
		}
	}
	return firstErr
}

// GetExecutor returns a registered executor plugin.
func (pm *PluginManager) GetExecutor(name string) (Executor, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	e, ok := pm.executors[name]
	return e, ok
}

// GetNotifier returns a registered notifier plugin.
func (pm *PluginManager) GetNotifier(name string) (Notifier, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	n, ok := pm.notifiers[name]
	return n, ok
}

// GetTrigger returns a registered trigger plugin.
func (pm *PluginManager) GetTrigger(name string) (Trigger, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	t, ok := pm.triggers[name]
	return t, ok
}

// GetSecretProvider returns a registered secret provider plugin.
func (pm *PluginManager) GetSecretProvider(name string) (SecretProvider, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	s, ok := pm.secrets[name]
	return s, ok
}

// ListPlugins returns metadata for all registered plugins.
func (pm *PluginManager) ListPlugins() []Metadata {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	metas := make([]Metadata, 0, len(pm.plugins))
	for _, p := range pm.plugins {
		metas = append(metas, p.Metadata())
	}
	return metas
}

// HealthCheckAll runs health checks on all plugins.
func (pm *PluginManager) HealthCheckAll(ctx context.Context) map[string]error {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	results := make(map[string]error)
	for name, p := range pm.plugins {
		results[name] = p.Health(ctx)
	}
	return results
}

// --- Example Plugins for SDK Reference ---

// LogNotifier is an example notifier plugin that logs notifications.
type LogNotifier struct {
	BasePlugin
	notifications []Notification
	mu            sync.Mutex
}

// NewLogNotifier creates a new log notifier for testing/development.
func NewLogNotifier() *LogNotifier {
	return &LogNotifier{
		BasePlugin: NewBasePlugin(Metadata{
			Name:        "log-notifier",
			Version:     "1.0.0",
			Type:        TypeNotifier,
			Description: "Logs notifications to memory (development/testing)",
			Author:      "Chronos",
		}),
	}
}

func (l *LogNotifier) Notify(_ context.Context, notification *Notification) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if notification.Timestamp.IsZero() {
		notification.Timestamp = time.Now()
	}
	l.notifications = append(l.notifications, *notification)
	return nil
}

// Notifications returns all logged notifications.
func (l *LogNotifier) Notifications() []Notification {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Notification, len(l.notifications))
	copy(out, l.notifications)
	return out
}

// NoopExecutor is an example executor that always succeeds.
type NoopExecutor struct {
	BasePlugin
}

// NewNoopExecutor creates a no-op executor for testing.
func NewNoopExecutor() *NoopExecutor {
	return &NoopExecutor{
		BasePlugin: NewBasePlugin(Metadata{
			Name:        "noop-executor",
			Version:     "1.0.0",
			Type:        TypeExecutor,
			Description: "No-op executor that always succeeds (testing)",
			Author:      "Chronos",
		}),
	}
}

func (n *NoopExecutor) Execute(_ context.Context, req *ExecutionRequest) (*ExecutionResult, error) {
	return &ExecutionResult{
		Success:    true,
		StatusCode: 200,
		Duration:   time.Millisecond,
		Metadata: map[string]interface{}{
			"executor": "noop",
			"job_id":   req.JobID,
		},
	}, nil
}
