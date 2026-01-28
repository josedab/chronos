// Package secrets provides secret providers for Chronos.
package secrets

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// EnvProvider provides secrets from environment variables.
type EnvProvider struct {
	prefix string
}

// NewEnvProvider creates a new environment variable provider.
func NewEnvProvider(prefix string) *EnvProvider {
	return &EnvProvider{prefix: prefix}
}

// Name returns the provider name.
func (p *EnvProvider) Name() string {
	return "env"
}

// GetSecret retrieves a secret from environment variables.
func (p *EnvProvider) GetSecret(ctx context.Context, key string) (string, error) {
	envKey := key
	if p.prefix != "" {
		envKey = p.prefix + key
	}

	value := os.Getenv(envKey)
	if value == "" {
		return "", fmt.Errorf("environment variable %s not found", envKey)
	}
	return value, nil
}

// ListSecrets lists environment variables with optional prefix.
func (p *EnvProvider) ListSecrets(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	fullPrefix := p.prefix + prefix

	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		if strings.HasPrefix(key, fullPrefix) {
			// Remove provider prefix before returning
			if p.prefix != "" {
				key = strings.TrimPrefix(key, p.prefix)
			}
			keys = append(keys, key)
		}
	}
	return keys, nil
}

// Close is a no-op for environment provider.
func (p *EnvProvider) Close(ctx context.Context) error {
	return nil
}

// FileProvider provides secrets from files.
type FileProvider struct {
	basePath string
}

// NewFileProvider creates a new file-based secret provider.
func NewFileProvider(basePath string) *FileProvider {
	return &FileProvider{basePath: basePath}
}

// Name returns the provider name.
func (p *FileProvider) Name() string {
	return "file"
}

// GetSecret reads a secret from a file.
func (p *FileProvider) GetSecret(ctx context.Context, key string) (string, error) {
	path := p.basePath + "/" + key
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read secret file %s: %w", path, err)
	}
	// Trim trailing newlines
	return strings.TrimSpace(string(data)), nil
}

// ListSecrets lists secret files in the base path.
func (p *FileProvider) ListSecrets(ctx context.Context, prefix string) ([]string, error) {
	entries, err := os.ReadDir(p.basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets directory: %w", err)
	}

	var keys []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, prefix) {
			keys = append(keys, name)
		}
	}
	return keys, nil
}

// Close is a no-op for file provider.
func (p *FileProvider) Close(ctx context.Context) error {
	return nil
}

// KubernetesSecretConfig configures Kubernetes secret provider.
type KubernetesSecretConfig struct {
	// Namespace is the Kubernetes namespace.
	Namespace string `json:"namespace" yaml:"namespace"`
	// MountPath is where secrets are mounted (default: /var/run/secrets).
	MountPath string `json:"mount_path" yaml:"mount_path"`
}

// KubernetesProvider provides secrets from Kubernetes.
type KubernetesProvider struct {
	namespace string
	mountPath string
}

// NewKubernetesProvider creates a new Kubernetes secret provider.
func NewKubernetesProvider(cfg KubernetesSecretConfig) *KubernetesProvider {
	mountPath := cfg.MountPath
	if mountPath == "" {
		mountPath = "/var/run/secrets"
	}
	return &KubernetesProvider{
		namespace: cfg.Namespace,
		mountPath: mountPath,
	}
}

// Name returns the provider name.
func (p *KubernetesProvider) Name() string {
	return "kubernetes"
}

// GetSecret retrieves a secret from Kubernetes.
// Expects format: secret-name/key
func (p *KubernetesProvider) GetSecret(ctx context.Context, key string) (string, error) {
	path := p.mountPath + "/" + key
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read kubernetes secret %s: %w", key, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// ListSecrets lists Kubernetes secrets.
func (p *KubernetesProvider) ListSecrets(ctx context.Context, prefix string) ([]string, error) {
	entries, err := os.ReadDir(p.mountPath)
	if err != nil {
		return nil, fmt.Errorf("failed to list kubernetes secrets: %w", err)
	}

	var keys []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, prefix) {
			keys = append(keys, name)
		}
	}
	return keys, nil
}

// Close is a no-op for Kubernetes provider.
func (p *KubernetesProvider) Close(ctx context.Context) error {
	return nil
}
