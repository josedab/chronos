// Package plugin provides the plugin registry for Chronos.
package plugin

import (
	"context"
	"fmt"
	"sync"
)

// Registry manages plugin registration and lookup.
type Registry struct {
	mu        sync.RWMutex
	executors map[string]Executor
	triggers  map[string]Trigger
	notifiers map[string]Notifier
	secrets   map[string]SecretProvider
	storage   map[string]Storage
}

// NewRegistry creates a new plugin registry.
func NewRegistry() *Registry {
	return &Registry{
		executors: make(map[string]Executor),
		triggers:  make(map[string]Trigger),
		notifiers: make(map[string]Notifier),
		secrets:   make(map[string]SecretProvider),
		storage:   make(map[string]Storage),
	}
}

// RegisterExecutor registers an executor plugin.
func (r *Registry) RegisterExecutor(name string, executor Executor) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.executors[name]; exists {
		return fmt.Errorf("executor %q already registered", name)
	}
	r.executors[name] = executor
	return nil
}

// GetExecutor returns an executor by name.
func (r *Registry) GetExecutor(name string) (Executor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	executor, exists := r.executors[name]
	if !exists {
		return nil, fmt.Errorf("executor %q not found", name)
	}
	return executor, nil
}

// RegisterTrigger registers a trigger plugin.
func (r *Registry) RegisterTrigger(name string, trigger Trigger) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.triggers[name]; exists {
		return fmt.Errorf("trigger %q already registered", name)
	}
	r.triggers[name] = trigger
	return nil
}

// GetTrigger returns a trigger by name.
func (r *Registry) GetTrigger(name string) (Trigger, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	trigger, exists := r.triggers[name]
	if !exists {
		return nil, fmt.Errorf("trigger %q not found", name)
	}
	return trigger, nil
}

// RegisterNotifier registers a notifier plugin.
func (r *Registry) RegisterNotifier(name string, notifier Notifier) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.notifiers[name]; exists {
		return fmt.Errorf("notifier %q already registered", name)
	}
	r.notifiers[name] = notifier
	return nil
}

// GetNotifier returns a notifier by name.
func (r *Registry) GetNotifier(name string) (Notifier, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	notifier, exists := r.notifiers[name]
	if !exists {
		return nil, fmt.Errorf("notifier %q not found", name)
	}
	return notifier, nil
}

// RegisterSecretProvider registers a secret provider plugin.
func (r *Registry) RegisterSecretProvider(name string, provider SecretProvider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.secrets[name]; exists {
		return fmt.Errorf("secret provider %q already registered", name)
	}
	r.secrets[name] = provider
	return nil
}

// GetSecretProvider returns a secret provider by name.
func (r *Registry) GetSecretProvider(name string) (SecretProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, exists := r.secrets[name]
	if !exists {
		return nil, fmt.Errorf("secret provider %q not found", name)
	}
	return provider, nil
}

// RegisterStorage registers a storage plugin.
func (r *Registry) RegisterStorage(name string, storage Storage) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.storage[name]; exists {
		return fmt.Errorf("storage %q already registered", name)
	}
	r.storage[name] = storage
	return nil
}

// GetStorage returns a storage by name.
func (r *Registry) GetStorage(name string) (Storage, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	storage, exists := r.storage[name]
	if !exists {
		return nil, fmt.Errorf("storage %q not found", name)
	}
	return storage, nil
}

// ListPlugins returns all registered plugins by type.
func (r *Registry) ListPlugins() map[Type][]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[Type][]string)

	if len(r.executors) > 0 {
		names := make([]string, 0, len(r.executors))
		for name := range r.executors {
			names = append(names, name)
		}
		result[TypeExecutor] = names
	}

	if len(r.triggers) > 0 {
		names := make([]string, 0, len(r.triggers))
		for name := range r.triggers {
			names = append(names, name)
		}
		result[TypeTrigger] = names
	}

	if len(r.notifiers) > 0 {
		names := make([]string, 0, len(r.notifiers))
		for name := range r.notifiers {
			names = append(names, name)
		}
		result[TypeNotifier] = names
	}

	if len(r.secrets) > 0 {
		names := make([]string, 0, len(r.secrets))
		for name := range r.secrets {
			names = append(names, name)
		}
		result[TypeSecretProvider] = names
	}

	if len(r.storage) > 0 {
		names := make([]string, 0, len(r.storage))
		for name := range r.storage {
			names = append(names, name)
		}
		result[TypeStorage] = names
	}

	return result
}

// CloseAll closes all registered plugins.
func (r *Registry) CloseAll(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error

	for name, p := range r.executors {
		if err := p.Close(ctx); err != nil {
			errs = append(errs, fmt.Errorf("executor %s: %w", name, err))
		}
	}

	for name, p := range r.triggers {
		if err := p.Close(ctx); err != nil {
			errs = append(errs, fmt.Errorf("trigger %s: %w", name, err))
		}
	}

	for name, p := range r.notifiers {
		if err := p.Close(ctx); err != nil {
			errs = append(errs, fmt.Errorf("notifier %s: %w", name, err))
		}
	}

	for name, p := range r.secrets {
		if err := p.Close(ctx); err != nil {
			errs = append(errs, fmt.Errorf("secret provider %s: %w", name, err))
		}
	}

	for name, p := range r.storage {
		if err := p.Close(ctx); err != nil {
			errs = append(errs, fmt.Errorf("storage %s: %w", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to close plugins: %v", errs)
	}

	return nil
}
