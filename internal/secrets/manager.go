// Package secrets provides secret injection for Chronos jobs.
package secrets

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// Provider is a source of secrets.
type Provider interface {
	// Name returns the provider name.
	Name() string
	// GetSecret retrieves a secret by key.
	GetSecret(ctx context.Context, key string) (string, error)
	// ListSecrets lists available secret keys.
	ListSecrets(ctx context.Context, prefix string) ([]string, error)
	// Close cleans up provider resources.
	Close(ctx context.Context) error
}

// SecretRef references a secret to be injected.
type SecretRef struct {
	// Provider is the secret provider name (e.g., "kubernetes", "vault", "env").
	Provider string `json:"provider" yaml:"provider"`
	// Key is the secret key/path in the provider.
	Key string `json:"key" yaml:"key"`
	// Field is the specific field within the secret (for structured secrets).
	Field string `json:"field,omitempty" yaml:"field,omitempty"`
}

// Manager manages secret providers and injection.
type Manager struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewManager creates a new secret manager.
func NewManager() *Manager {
	return &Manager{
		providers: make(map[string]Provider),
	}
}

// RegisterProvider registers a secret provider.
func (m *Manager) RegisterProvider(provider Provider) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := provider.Name()
	if _, exists := m.providers[name]; exists {
		return fmt.Errorf("provider %q already registered", name)
	}
	m.providers[name] = provider
	return nil
}

// GetProvider returns a provider by name.
func (m *Manager) GetProvider(name string) (Provider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	provider, exists := m.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider %q not found", name)
	}
	return provider, nil
}

// ResolveSecret resolves a secret reference to its value.
func (m *Manager) ResolveSecret(ctx context.Context, ref SecretRef) (string, error) {
	provider, err := m.GetProvider(ref.Provider)
	if err != nil {
		return "", err
	}

	value, err := provider.GetSecret(ctx, ref.Key)
	if err != nil {
		return "", fmt.Errorf("failed to get secret %s from %s: %w", ref.Key, ref.Provider, err)
	}

	return value, nil
}

// secretPattern matches ${secret:provider:key} or ${secret:provider:key:field}
var secretPattern = regexp.MustCompile(`\$\{secret:([^:}]+):([^:}]+)(?::([^}]+))?\}`)

// InjectSecrets replaces secret references in a string with actual values.
func (m *Manager) InjectSecrets(ctx context.Context, input string) (string, error) {
	var lastErr error
	result := secretPattern.ReplaceAllStringFunc(input, func(match string) string {
		parts := secretPattern.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}

		ref := SecretRef{
			Provider: parts[1],
			Key:      parts[2],
		}
		if len(parts) > 3 {
			ref.Field = parts[3]
		}

		value, err := m.ResolveSecret(ctx, ref)
		if err != nil {
			lastErr = err
			return match
		}
		return value
	})

	return result, lastErr
}

// InjectSecretsInMap injects secrets into all string values in a map.
func (m *Manager) InjectSecretsInMap(ctx context.Context, input map[string]string) (map[string]string, error) {
	result := make(map[string]string, len(input))
	for k, v := range input {
		injected, err := m.InjectSecrets(ctx, v)
		if err != nil {
			return nil, fmt.Errorf("failed to inject secret for key %s: %w", k, err)
		}
		result[k] = injected
	}
	return result, nil
}

// Close closes all providers.
func (m *Manager) Close(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []string
	for name, provider := range m.providers {
		if err := provider.Close(ctx); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to close providers: %s", strings.Join(errs, "; "))
	}
	return nil
}
