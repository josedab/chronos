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

// legacySecretPattern matches ${provider:key} format for backward compatibility
// Supports: vault, aws, gcp, azure, env, file
var legacySecretPattern = regexp.MustCompile(`\$\{(vault|aws|gcp|azure|env|file):([^}]+)\}`)

// InjectSecrets replaces secret references in a string with actual values.
// Supports two formats:
//   - ${secret:provider:key} or ${secret:provider:key:field}
//   - ${provider:key} (legacy format for vault, aws, gcp, azure)
func (m *Manager) InjectSecrets(ctx context.Context, input string) (string, error) {
	var lastErr error
	
	// First, handle standard format
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

	// Then, handle legacy format
	result = legacySecretPattern.ReplaceAllStringFunc(result, func(match string) string {
		parts := legacySecretPattern.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}

		ref := SecretRef{
			Provider: parts[1],
			Key:      parts[2],
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

// InjectSecretsInWebhookConfig injects secrets into a webhook configuration.
func (m *Manager) InjectSecretsInWebhookConfig(ctx context.Context, config *WebhookSecretConfig) (*WebhookSecretConfig, error) {
	result := &WebhookSecretConfig{
		URL:     config.URL,
		Method:  config.Method,
		Headers: make(map[string]string),
		Body:    config.Body,
	}

	// Inject in URL
	var err error
	result.URL, err = m.InjectSecrets(ctx, config.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to inject secrets in URL: %w", err)
	}

	// Inject in headers
	for k, v := range config.Headers {
		result.Headers[k], err = m.InjectSecrets(ctx, v)
		if err != nil {
			return nil, fmt.Errorf("failed to inject secrets in header %s: %w", k, err)
		}
	}

	// Inject in body
	result.Body, err = m.InjectSecrets(ctx, config.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to inject secrets in body: %w", err)
	}

	// Inject auth credentials
	if config.Auth != nil {
		result.Auth = &AuthSecretConfig{
			Type: config.Auth.Type,
		}
		if config.Auth.Token != "" {
			result.Auth.Token, err = m.InjectSecrets(ctx, config.Auth.Token)
			if err != nil {
				return nil, fmt.Errorf("failed to inject secrets in auth token: %w", err)
			}
		}
		if config.Auth.APIKey != "" {
			result.Auth.APIKey, err = m.InjectSecrets(ctx, config.Auth.APIKey)
			if err != nil {
				return nil, fmt.Errorf("failed to inject secrets in API key: %w", err)
			}
		}
		if config.Auth.Username != "" {
			result.Auth.Username, err = m.InjectSecrets(ctx, config.Auth.Username)
			if err != nil {
				return nil, fmt.Errorf("failed to inject secrets in username: %w", err)
			}
		}
		if config.Auth.Password != "" {
			result.Auth.Password, err = m.InjectSecrets(ctx, config.Auth.Password)
			if err != nil {
				return nil, fmt.Errorf("failed to inject secrets in password: %w", err)
			}
		}
	}

	return result, nil
}

// WebhookSecretConfig is a webhook configuration that may contain secret references.
type WebhookSecretConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
	Auth    *AuthSecretConfig `json:"auth,omitempty"`
}

// AuthSecretConfig is an auth configuration that may contain secret references.
type AuthSecretConfig struct {
	Type     string `json:"type"`
	Token    string `json:"token,omitempty"`
	APIKey   string `json:"api_key,omitempty"`
	Header   string `json:"header,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// ValidateSecretReferences validates that all secret references in a string can be resolved.
func (m *Manager) ValidateSecretReferences(ctx context.Context, input string) []error {
	var errors []error

	// Find all secret references
	matches := secretPattern.FindAllStringSubmatch(input, -1)
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		ref := SecretRef{
			Provider: match[1],
			Key:      match[2],
		}
		if len(match) > 3 {
			ref.Field = match[3]
		}

		if _, err := m.ResolveSecret(ctx, ref); err != nil {
			errors = append(errors, fmt.Errorf("secret %s:%s: %w", ref.Provider, ref.Key, err))
		}
	}

	// Also check legacy format
	legacyMatches := legacySecretPattern.FindAllStringSubmatch(input, -1)
	for _, match := range legacyMatches {
		if len(match) < 3 {
			continue
		}
		ref := SecretRef{
			Provider: match[1],
			Key:      match[2],
		}

		if _, err := m.ResolveSecret(ctx, ref); err != nil {
			errors = append(errors, fmt.Errorf("secret %s:%s: %w", ref.Provider, ref.Key, err))
		}
	}

	return errors
}

// ListProviders returns the names of all registered providers.
func (m *Manager) ListProviders() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	providers := make([]string, 0, len(m.providers))
	for name := range m.providers {
		providers = append(providers, name)
	}
	return providers
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
